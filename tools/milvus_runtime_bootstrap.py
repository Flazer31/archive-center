#!/usr/bin/env python3
"""Managed Milvus Lite runtime bootstrap for Archive Center 2.0.

This is the Vector/Milvus migration path for normal users: Archive Center owns
the Milvus Lite Python runtime location and verifies it with smoke evidence.
The default runtime directory lives under the workspace runtime cache, not under
the source tree. Nothing here enables live retrieval or retires Chroma.
"""

from __future__ import annotations

import argparse
import importlib.util
import json
import os
import platform
import subprocess
import sys
import tempfile
import time
import venv
from pathlib import Path
from typing import Any


SCHEMA_VERSION = "archive-center.milvus-runtime-bootstrap.v1"
DEFAULT_PACKAGES = ("pymilvus==3.0.0", "milvus-lite==3.0.0")


def repo_root() -> Path:
    return Path(__file__).resolve().parents[1]


def workspace_root() -> Path:
    return repo_root().parent


def default_runtime_dir() -> Path:
    return workspace_root() / ".runtime-cache" / "archive-center-2.0" / "milvus-lite-runtime"


def path_inside(child: Path, parent: Path) -> bool:
    try:
        child.resolve().relative_to(parent.resolve())
        return True
    except ValueError:
        return False


def runtime_python(runtime_dir: Path) -> Path:
    if os.name == "nt":
        return runtime_dir / "Scripts" / "python.exe"
    return runtime_dir / "bin" / "python"


def run_cmd(cmd: list[str], timeout: int) -> tuple[int, str, str, int]:
    started = time.perf_counter()
    proc = subprocess.run(
        cmd,
        text=True,
        capture_output=True,
        timeout=timeout,
        check=False,
    )
    duration_ms = round((time.perf_counter() - started) * 1000)
    return proc.returncode, proc.stdout.strip(), proc.stderr.strip(), duration_ms


def create_runtime(runtime_dir: Path) -> dict[str, Any]:
    started = time.perf_counter()
    runtime_dir.parent.mkdir(parents=True, exist_ok=True)
    builder = venv.EnvBuilder(with_pip=True, clear=False)
    builder.create(runtime_dir)
    return {
        "status": "ok",
        "duration_ms": round((time.perf_counter() - started) * 1000),
    }


def install_packages(python_exe: Path, packages: list[str], timeout: int) -> dict[str, Any]:
    cmd = [str(python_exe), "-m", "pip", "install", "--disable-pip-version-check", *packages]
    code, stdout, stderr, duration_ms = run_cmd(cmd, timeout)
    return {
        "status": "ok" if code == 0 else "failed",
        "exit_code": code,
        "duration_ms": duration_ms,
        "packages": packages,
        "stdout_tail": stdout[-2000:],
        "stderr_tail": stderr[-4000:],
    }


def inspect_runtime(python_exe: Path, timeout: int) -> dict[str, Any]:
    if not python_exe.exists():
        return {
            "status": "missing",
            "python": str(python_exe),
            "pymilvus_available": False,
            "milvus_lite_available": False,
        }
    script = r'''
import importlib.metadata
import importlib.util
import json
import platform
import sys

def available(name):
    return importlib.util.find_spec(name) is not None

def version(name):
    try:
        return importlib.metadata.version(name)
    except importlib.metadata.PackageNotFoundError:
        return None

payload = {
    "status": "ok",
    "python": sys.executable,
    "python_version": platform.python_version(),
    "pymilvus_available": available("pymilvus"),
    "pymilvus_version": version("pymilvus"),
    "milvus_client_available": False,
    "milvus_lite_available": available("milvus_lite"),
    "milvus_lite_version": version("milvus-lite"),
}
if payload["pymilvus_available"]:
    import pymilvus
    payload["pymilvus_version"] = getattr(pymilvus, "__version__", payload["pymilvus_version"])
    payload["milvus_client_available"] = hasattr(pymilvus, "MilvusClient")
print(json.dumps(payload, sort_keys=True))
'''
    code, stdout, stderr, duration_ms = run_cmd([str(python_exe), "-c", script], timeout)
    if code != 0:
        return {
            "status": "failed",
            "python": str(python_exe),
            "exit_code": code,
            "stderr_tail": stderr[-4000:],
            "duration_ms": duration_ms,
            "pymilvus_available": False,
            "milvus_lite_available": False,
        }
    try:
        payload = json.loads(stdout)
    except json.JSONDecodeError:
        payload = {"status": "failed", "python": str(python_exe), "stdout_tail": stdout[-2000:]}
    payload["duration_ms"] = duration_ms
    return payload


def run_smoke(python_exe: Path, out_dir: Path, timeout: int) -> dict[str, Any]:
    smoke_out = out_dir / "milvus-runtime-bootstrap-smoke.json"
    tool = repo_root() / "tools" / "milvus_lite_preflight.py"
    code, stdout, stderr, duration_ms = run_cmd(
        [str(python_exe), "-B", str(tool), "--smoke", "--out", str(smoke_out)],
        timeout,
    )
    payload: dict[str, Any] = {
        "status": "ok" if code == 0 else "failed",
        "exit_code": code,
        "duration_ms": duration_ms,
        "report_path": str(smoke_out),
        "stdout_tail": stdout[-2000:],
        "stderr_tail": stderr[-4000:],
    }
    if smoke_out.exists():
        try:
            payload["report"] = json.loads(smoke_out.read_text(encoding="utf-8"))
        except json.JSONDecodeError as exc:
            payload["parse_error"] = str(exc)
    return payload


def write_json(path: Path, payload: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2, ensure_ascii=True) + "\n", encoding="utf-8")


def build_report(args: argparse.Namespace) -> tuple[dict[str, Any], int]:
    runtime_dir = Path(args.runtime_dir).expanduser().resolve()
    python_exe = runtime_python(runtime_dir)
    failures: list[str] = []
    warnings: list[str] = []

    if path_inside(runtime_dir, repo_root()):
        failures.append("runtime_dir_inside_source_tree")

    created: dict[str, Any] = {"requested": bool(args.install), "status": "not_run"}
    installed: dict[str, Any] = {"requested": bool(args.install), "status": "not_run"}
    if args.install and not failures:
        try:
            created = create_runtime(runtime_dir)
            installed = install_packages(python_exe, list(args.package), args.install_timeout)
            if installed["status"] != "ok":
                failures.append("package_install_failed")
        except Exception as exc:  # noqa: BLE001
            failures.append("runtime_create_or_install_failed")
            installed = {"requested": True, "status": "failed", "error": f"{type(exc).__name__}: {exc}"}

    inspected = inspect_runtime(python_exe, args.check_timeout)
    if inspected.get("status") == "missing":
        warnings.append("managed_runtime_missing")
    elif inspected.get("status") != "ok":
        failures.append("managed_runtime_inspect_failed")
    else:
        if not inspected.get("pymilvus_available"):
            warnings.append("pymilvus_not_available")
        if not inspected.get("milvus_client_available"):
            warnings.append("milvus_client_not_available")
        if not inspected.get("milvus_lite_available"):
            warnings.append("milvus_lite_package_not_available")

    smoke: dict[str, Any] = {"requested": bool(args.smoke), "status": "not_run"}
    if args.smoke and not failures:
        if not python_exe.exists():
            failures.append("smoke_runtime_missing")
        else:
            with tempfile.TemporaryDirectory(prefix="archive-center-milvus-bootstrap-") as tmp:
                smoke = run_smoke(python_exe, Path(tmp), args.smoke_timeout)
            if smoke.get("status") != "ok":
                failures.append("smoke_failed")

    if failures:
        status = "failed"
        support_level = "red"
        exit_code = 1
    elif warnings:
        status = "degraded"
        support_level = "yellow"
        exit_code = 0
    else:
        status = "ok"
        support_level = "green"
        exit_code = 0

    report = {
        "schema_version": SCHEMA_VERSION,
        "tool": "milvus_runtime_bootstrap",
        "status": status,
        "support_level": support_level,
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "runtime_dir": str(runtime_dir),
        "runtime_python": str(python_exe),
        "platform": {
            "system": platform.system(),
            "release": platform.release(),
            "machine": platform.machine(),
            "source_python": sys.executable,
        },
        "install": installed,
        "create_runtime": created,
        "inspect": inspected,
        "smoke": smoke,
        "warnings": warnings,
        "failures": failures,
        "safety_flags": {
            "runtime_dir_inside_source_tree": path_inside(runtime_dir, repo_root()),
            "live_retrieval_enabled": False,
            "milvus_live_enabled": False,
            "chroma_retired": False,
            "touches_08_runtime": False,
        },
        "next_action": (
            "Run with --install --smoke to create and prove the managed Milvus Lite runtime."
            if not args.install and not inspected.get("milvus_lite_available")
            else "Use this managed runtime for Chroma-to-Milvus shadow parity and Go live-read dry-run evidence."
        ),
    }
    return report, exit_code


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Archive Center 2.0 managed Milvus Lite runtime bootstrap")
    parser.add_argument("--runtime-dir", default=str(default_runtime_dir()), help="Managed runtime directory outside the source tree")
    parser.add_argument("--install", action="store_true", help="Create the venv and install Milvus Lite packages")
    parser.add_argument("--smoke", action="store_true", help="Run milvus_lite_preflight.py --smoke inside the managed runtime")
    parser.add_argument("--package", action="append", default=list(DEFAULT_PACKAGES), help="Package spec to install; repeatable")
    parser.add_argument("--out", default="", help="Write JSON report to this path")
    parser.add_argument("--install-timeout", type=int, default=600)
    parser.add_argument("--check-timeout", type=int, default=60)
    parser.add_argument("--smoke-timeout", type=int, default=180)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    report, exit_code = build_report(args)
    if args.out:
        write_json(Path(args.out), report)
    else:
        print(json.dumps(report, indent=2, ensure_ascii=True))
    return exit_code


if __name__ == "__main__":
    raise SystemExit(main())
