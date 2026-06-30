#!/usr/bin/env python3
"""Run report-only platform adoption smoke checks for installer scripts.

This tool is intentionally non-mutating. It checks that Linux/Termux/macOS
bootstrap entrypoints can emit structured preflight JSON and that their reports
keep the normal-user contract: no manual MariaDB or ChromaDB setup in the
normal path.
"""

from __future__ import annotations

import argparse
import json
import platform
import shutil
import shlex
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any


TARGET_SCRIPTS = {
    "linux": Path("ops/install-linux.sh"),
    "termux": Path("ops/install-termux.sh"),
    "macos": Path("ops/install-macos.sh"),
}

PROOF_TARGETS = ("linux", "termux", "macos")
PROOF_FILENAMES = {
    "linux": ("linux-adoption-proof.json", "linux-preflight.json"),
    "termux": ("termux-adoption-proof.json", "termux-preflight.json"),
    "macos": ("macos-adoption-proof.json", "macos-preflight.json"),
}
ASSUMPTION_FILENAMES = {
    "linux": ("linux-assumption-profile.json",),
    "termux": ("termux-assumption-profile.json",),
    "macos": ("macos-assumption-profile.json",),
}
REQUIRED_LIFECYCLE_CHECKS = ("bootstrap", "install", "update", "repair", "uninstall", "rollback")


def run_command(args: list[str], timeout: int = 45, cwd: Path | None = None) -> dict[str, Any]:
    try:
        proc = subprocess.run(
            args,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=timeout,
            check=False,
            cwd=str(cwd) if cwd else None,
        )
        return {
            "ran": True,
            "exit_code": proc.returncode,
            "stdout": proc.stdout.strip(),
            "stderr": proc.stderr.strip(),
            "timed_out": False,
        }
    except subprocess.TimeoutExpired as exc:
        return {
            "ran": True,
            "exit_code": None,
            "stdout": (exc.stdout or "").strip() if isinstance(exc.stdout, str) else "",
            "stderr": (exc.stderr or "").strip() if isinstance(exc.stderr, str) else "",
            "timed_out": True,
        }
    except OSError as exc:
        return {
            "ran": False,
            "exit_code": None,
            "stdout": "",
            "stderr": str(exc),
            "timed_out": False,
        }


def windows_path_to_msys(path: Path) -> str:
    raw = str(path.resolve()).replace("\\", "/")
    if len(raw) >= 2 and raw[1] == ":":
        return f"/{raw[0].lower()}{raw[2:]}"
    return raw


def find_usable_bash(explicit: str = "") -> str | None:
    candidates: list[str] = []
    if explicit:
        candidates.append(explicit)
    for candidate in (
        r"C:\Program Files\Git\bin\bash.exe",
        r"C:\Program Files\Git\usr\bin\bash.exe",
        shutil.which("bash") or "",
    ):
        if candidate and candidate not in candidates:
            candidates.append(candidate)
    for candidate in candidates:
        if not Path(candidate).is_file() and shutil.which(candidate) is None:
            continue
        probe = run_command(
            [candidate, "-lc", "command -v dirname >/dev/null && command -v grep >/dev/null && command -v sed >/dev/null"],
            timeout=10,
        )
        if probe["ran"] and probe["exit_code"] == 0:
            return candidate
    return None


def read_json_report(path: Path) -> tuple[dict[str, Any] | None, str]:
    try:
        return json.loads(path.read_text(encoding="utf-8")), ""
    except Exception as exc:  # noqa: BLE001 - report-only diagnostic path
        return None, str(exc)


def contract_from_report(report: dict[str, Any] | None) -> dict[str, Any]:
    if not report:
        return {
            "status": "missing_report",
            "normal_user_manual_mariadb_required": None,
            "normal_user_manual_chromadb_required": None,
            "failures": ["preflight_json_missing"],
        }

    failures: list[str] = []
    mariadb = report.get("mariadb")
    chromadb = report.get("chromadb")
    if not isinstance(mariadb, dict):
        failures.append("mariadb_section_missing")
        mariadb = {}
    if not isinstance(chromadb, dict):
        failures.append("chromadb_section_missing")
        chromadb = {}

    manual_mariadb = mariadb.get("normal_user_manual_mariadb_required")
    manual_chromadb = chromadb.get("normal_user_manual_chromadb_required")
    if manual_mariadb is not False:
        failures.append("normal_user_manual_mariadb_required_not_false")
    if manual_chromadb is not False:
        failures.append("normal_user_manual_chromadb_required_not_false")
    for section_name, section in (("mariadb", mariadb), ("chromadb", chromadb)):
        if not section.get("provider_mode"):
            failures.append(f"{section_name}_provider_mode_missing")
        if "installer_managed_required" not in section:
            failures.append(f"{section_name}_installer_managed_required_missing")
        if not section.get("required_action"):
            failures.append(f"{section_name}_required_action_missing")

    return {
        "status": "ok" if not failures else "failed",
        "normal_user_manual_mariadb_required": manual_mariadb,
        "normal_user_manual_chromadb_required": manual_chromadb,
        "mariadb_provider_mode": mariadb.get("provider_mode"),
        "mariadb_installer_managed_required": mariadb.get("installer_managed_required"),
        "chromadb_provider_mode": chromadb.get("provider_mode"),
        "chromadb_installer_managed_required": chromadb.get("installer_managed_required"),
        "failures": failures,
    }


def value_ok(value: Any) -> bool:
    return value is True or str(value).lower() in {"ok", "pass", "passed", "green", "true"}


def lifecycle_from_report(report: dict[str, Any]) -> dict[str, Any]:
    lifecycle = report.get("lifecycle")
    if not isinstance(lifecycle, dict):
        lifecycle = report.get("adoption_lifecycle")
    if not isinstance(lifecycle, dict):
        lifecycle = {}

    failures: list[str] = []
    checks: dict[str, bool] = {}
    for name in REQUIRED_LIFECYCLE_CHECKS:
        ok = value_ok(lifecycle.get(name))
        checks[name] = ok
        if not ok:
            failures.append(f"{name}_proof_missing_or_not_ok")
    return {
        "status": "ok" if not failures else "failed",
        "checks": checks,
        "failures": failures,
    }


def platform_match(target: str, report: dict[str, Any]) -> bool:
    platform_name = str(report.get("platform", "")).lower()
    detail = str(report.get("platform_detail", "")).lower()
    if target == "linux":
        return "linux" in platform_name and "android" not in detail
    if target == "termux":
        termux = report.get("termux")
        return bool(isinstance(termux, dict) and termux.get("detected") is True) or "android" in detail
    if target == "macos":
        return platform_name in {"darwin", "macos"} or "darwin" in platform_name
    return False


def load_external_proof(proof_dir: Path, target: str) -> dict[str, Any]:
    candidates = [proof_dir / name for name in PROOF_FILENAMES[target]]
    existing = next((path for path in candidates if path.exists()), None)
    if existing is None:
        assumption = load_assumption_profile(proof_dir, target, real_failures=["real_platform_proof_missing"])
        if assumption:
            return assumption
        return {
            "target": target,
            "present": False,
            "path": str(candidates[0]),
            "status": "missing",
            "gate_accepted": False,
            "failures": ["real_platform_proof_missing"],
        }

    report, error = read_json_report(existing)
    failures: list[str] = []
    if report is None:
        assumption = load_assumption_profile(proof_dir, target, real_failures=[f"proof_json_invalid:{error}"])
        if assumption:
            return assumption
        return {
            "target": target,
            "present": True,
            "path": str(existing),
            "status": "failed",
            "gate_accepted": False,
            "failures": [f"proof_json_invalid:{error}"],
        }

    if report.get("target") != target:
        failures.append("target_mismatch")
    if report.get("support_level") != "green":
        failures.append("support_level_not_green")
    if report.get("preflight_status") != "ok":
        failures.append("preflight_status_not_ok")
    if not platform_match(target, report):
        failures.append("platform_identity_not_proven")

    contract = contract_from_report(report)
    lifecycle = lifecycle_from_report(report)
    failures.extend(contract["failures"])
    failures.extend(lifecycle["failures"])

    if failures:
        assumption = load_assumption_profile(proof_dir, target, real_failures=failures)
        if assumption:
            return assumption

    return {
        "target": target,
        "present": True,
        "path": str(existing),
        "status": "ok" if not failures else "failed",
        "gate_accepted": not failures,
        "support_level": report.get("support_level"),
        "preflight_status": report.get("preflight_status"),
        "contract": contract,
        "lifecycle": lifecycle,
        "failures": failures,
    }


def load_external_proofs(proof_dir: Path) -> list[dict[str, Any]]:
    return [load_external_proof(proof_dir, target) for target in PROOF_TARGETS]


def load_assumption_profile(proof_dir: Path, target: str, real_failures: list[str]) -> dict[str, Any] | None:
    if target not in ASSUMPTION_FILENAMES:
        return None

    candidates = [proof_dir / name for name in ASSUMPTION_FILENAMES[target]]
    existing = next((path for path in candidates if path.exists()), None)
    if existing is None:
        return None

    report, error = read_json_report(existing)
    failures: list[str] = []
    if report is None:
        failures.append(f"assumption_json_invalid:{error}")
        report = {}

    if report.get("target") != target:
        failures.append("assumption_target_mismatch")
    if report.get("status") != "ok":
        failures.append("assumption_status_not_ok")
    if report.get("proof_kind") != "assumption_profile":
        failures.append("assumption_proof_kind_invalid")
    if report.get("conditional_support") is not True:
        failures.append("conditional_support_not_true")
    if report.get("accepted_without_real_device") is not True:
        failures.append("accepted_without_real_device_not_true")

    contract = contract_from_report(report)
    failures.extend(contract["failures"])
    assumptions = report.get("assumptions")
    if not isinstance(assumptions, list) or not assumptions:
        failures.append("assumptions_missing")
    limitations = report.get("limitations")
    if not isinstance(limitations, list) or not limitations:
        failures.append("limitations_missing")

    return {
        "target": target,
        "present": True,
        "path": str(existing),
        "status": "conditional" if not failures else "failed",
        "gate_accepted": not failures,
        "proof_kind": "assumption_profile",
        "support_level": report.get("support_level", "conditional"),
        "preflight_status": report.get("preflight_status", "assumed"),
        "contract": contract,
        "lifecycle": {
            "status": "conditional" if not failures else "failed",
            "checks": {name: "assumed" for name in REQUIRED_LIFECYCLE_CHECKS},
            "failures": [],
        },
        "real_platform_proof_failures": real_failures,
        "failures": failures,
    }


def run_target(repo_root: Path, target: str, bash_path: str | None, temp_root: Path) -> dict[str, Any]:
    rel_script = TARGET_SCRIPTS[target]
    script = repo_root / rel_script
    result: dict[str, Any] = {
        "target": target,
        "script": str(script),
        "script_exists": script.is_file(),
    }
    if not script.is_file():
        result.update(
            {
                "syntax_status": "missing_script",
                "preflight_run_status": "not_run",
                "contract": contract_from_report(None),
            }
        )
        return result

    if not bash_path:
        result["syntax_status"] = "not_run_bash_missing"
        result["syntax_exit_code"] = None
        result["preflight_run_status"] = "not_run_bash_missing"
        result["contract"] = contract_from_report(None)
        return result

    script_arg = shlex.quote(windows_path_to_msys(script))
    syntax = run_command([bash_path, "-lc", f"bash -n {script_arg}"], cwd=repo_root)
    result["syntax_status"] = "ok" if syntax["ran"] and syntax["exit_code"] == 0 else "failed"
    result["syntax_exit_code"] = syntax["exit_code"]
    result["syntax_stderr"] = syntax["stderr"]

    report_path = temp_root / f"{target}-preflight.json"
    data_dir = temp_root / f"{target}-data"
    data_dir.mkdir(parents=True, exist_ok=True)
    data_arg = shlex.quote(windows_path_to_msys(data_dir))
    report_arg = shlex.quote(windows_path_to_msys(report_path))
    preflight = run_command(
        [bash_path, "-lc", f"bash {script_arg} --preflight --data-dir {data_arg} --out {report_arg}"],
        timeout=60,
        cwd=repo_root,
    )
    report, report_error = read_json_report(report_path) if report_path.exists() else (None, "report_not_created")
    result["preflight_run_status"] = "ran" if preflight["ran"] else "failed_to_start"
    result["preflight_exit_code"] = preflight["exit_code"]
    result["preflight_timed_out"] = preflight["timed_out"]
    result["preflight_stderr"] = preflight["stderr"]
    result["preflight_json_valid"] = report is not None
    result["preflight_json_error"] = report_error
    if report is not None:
        result["preflight_support_level"] = report.get("support_level")
        result["preflight_status"] = report.get("preflight_status")
        result["warnings"] = report.get("warnings", [])
        result["failures"] = report.get("failures", [])
    result["contract"] = contract_from_report(report)
    return result


def summarize(targets: list[dict[str, Any]], external_proofs: list[dict[str, Any]]) -> dict[str, Any]:
    total = len(targets)
    syntax_ok = sum(1 for item in targets if item.get("syntax_status") == "ok")
    json_valid = sum(1 for item in targets if item.get("preflight_json_valid") is True)
    contract_ok = sum(1 for item in targets if item.get("contract", {}).get("status") == "ok")
    platform_green = sum(1 for item in targets if item.get("preflight_support_level") == "green")
    platform_yellow = sum(1 for item in targets if item.get("preflight_support_level") == "yellow")
    platform_red = sum(1 for item in targets if item.get("preflight_support_level") == "red")
    proof_total = len(external_proofs)
    proof_ok = sum(1 for item in external_proofs if item.get("status") == "ok")
    proof_conditional = sum(1 for item in external_proofs if item.get("status") == "conditional")
    proof_accepted = sum(1 for item in external_proofs if item.get("gate_accepted") is True)
    return {
        "targets_total": total,
        "syntax_ok": syntax_ok,
        "preflight_json_valid": json_valid,
        "contract_ok": contract_ok,
        "platform_green": platform_green,
        "platform_yellow": platform_yellow,
        "platform_red": platform_red,
        "all_contracts_ok": contract_ok == total,
        "all_syntax_ok": syntax_ok == total,
        "all_preflight_json_valid": json_valid == total,
        "external_proof_total": proof_total,
        "external_proof_ok": proof_ok,
        "external_proof_conditional": proof_conditional,
        "external_proof_accepted": proof_accepted,
        "all_external_proofs_ok": proof_total == len(PROOF_TARGETS) and proof_ok == proof_total,
        "all_platform_requirements_ok": proof_total == len(PROOF_TARGETS) and proof_accepted == proof_total,
    }


def determine_status(summary: dict[str, Any]) -> str:
    if not summary["all_syntax_ok"] or not summary["all_preflight_json_valid"] or not summary["all_contracts_ok"]:
        return "failed"
    if not summary["all_platform_requirements_ok"]:
        return "degraded"
    return "ok"


def build_report(repo_root: Path, targets: list[str], bash_path: str | None, proof_dir: Path) -> dict[str, Any]:
    with tempfile.TemporaryDirectory(prefix="archive-center-platform-adoption-") as tmp:
        temp_root = Path(tmp)
        target_reports = [run_target(repo_root, target, bash_path, temp_root) for target in targets]
    external_proofs = load_external_proofs(proof_dir)
    summary = summarize(target_reports, external_proofs)
    status = determine_status(summary)
    product_gate_green = status == "ok" and summary["all_platform_requirements_ok"]
    return {
        "schema_version": "archive-center.platform_adoption_smoke.v1",
        "status": status,
        "product_gate_green": product_gate_green,
        "interpretation": (
            "report-only adoption smoke; packaging/adoption closes when Linux, Termux, and macOS proofs or explicit conditional profiles are accepted"
        ),
        "host": {
            "system": platform.system(),
            "release": platform.release(),
            "machine": platform.machine(),
            "python": sys.version.split()[0],
            "bash_path": bash_path or "",
        },
        "scope": {
            "repo_root": str(repo_root),
            "proof_dir": str(proof_dir),
            "mutates_source_tree": False,
            "runs_installation": False,
            "normal_user_manual_mariadb_required": False,
            "normal_user_manual_chromadb_required": False,
        },
        "summary": summary,
        "targets": target_reports,
        "external_proofs": external_proofs,
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--out", default="")
    parser.add_argument("--targets", default="linux,termux")
    parser.add_argument("--bash", default="")
    parser.add_argument("--proof-dir", default="benchmarks/platform-proofs")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    repo_root = Path(args.repo_root).resolve()
    targets = [item.strip() for item in args.targets.split(",") if item.strip()]
    unknown = sorted(set(targets) - set(TARGET_SCRIPTS))
    if unknown:
        raise SystemExit(f"unknown targets: {', '.join(unknown)}")
    bash_path = find_usable_bash(args.bash)
    proof_dir = (repo_root / args.proof_dir).resolve()
    report = build_report(repo_root, targets, bash_path, proof_dir)
    payload = json.dumps(report, indent=2, ensure_ascii=False)
    if args.out:
        out_path = Path(args.out)
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(payload + "\n", encoding="utf-8")
    else:
        print(payload)
    return 0 if report["status"] in {"ok", "degraded"} else 1


if __name__ == "__main__":
    raise SystemExit(main())
