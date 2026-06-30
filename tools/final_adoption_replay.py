#!/usr/bin/env python3
"""Final adoption replay for Archive Center 2.0.

This runner does not mutate the 0.8 reference tree. It reads the current
product-gate evidence and optional operator signoff evidence, then records
whether the current small-group adoption scope is accepted.
"""

from __future__ import annotations

import argparse
import json
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

import product_gate_report


DEFAULT_SIGNOFF = Path("benchmarks/final-adoption-signoff.json")


def load_json(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {}
    try:
        return json.loads(path.read_text(encoding="utf-8-sig"))
    except json.JSONDecodeError:
        return {"_decode_error": True}


def check_operator_signoff(path: Path) -> dict[str, Any]:
    payload = load_json(path)
    exists = path.exists()
    ok = (
        exists
        and payload.get("status") == "ok"
        and payload.get("operator_signoff") is True
        and payload.get("accepts_rollback_plan") is True
    )
    failures: list[str] = []
    if not exists:
        failures.append("operator_signoff_missing")
    elif payload.get("_decode_error"):
        failures.append("operator_signoff_json_decode_error")
    else:
        if payload.get("status") != "ok":
            failures.append("operator_signoff_status_not_ok")
        if payload.get("operator_signoff") is not True:
            failures.append("operator_signoff_not_true")
        if payload.get("accepts_rollback_plan") is not True:
            failures.append("rollback_plan_not_accepted")
    return {
        "name": "operator_signoff",
        "path": str(path),
        "present": exists,
        "ok": ok,
        "failures": failures,
    }


def summarize_product_gates(product_report: dict[str, Any]) -> dict[str, Any]:
    gates = product_report.get("gates", [])
    prerequisite_open: list[str] = []
    if isinstance(gates, list):
        prerequisite_open = [
            str(item.get("gate_id"))
            for item in gates
            if isinstance(item, dict) and item.get("gate_id") != "2.0-6" and item.get("green") is not True
        ]
    else:
        prerequisite_open = ["product_gate_report_malformed"]

    return {
        "name": "product_gate_prerequisites",
        "ok": not prerequisite_open,
        "product_gate_progress": product_report.get("product_gate_progress", ""),
        "product_cutover_readiness_percent": product_report.get("product_cutover_readiness_percent"),
        "prerequisite_open_gates": prerequisite_open,
    }


def check_actual_adoption_scope(gate_summary: dict[str, Any], signoff: dict[str, Any]) -> dict[str, Any]:
    """Accept scope only when product gates and operator signoff already prove it."""

    failures: list[str] = []
    if gate_summary.get("ok") is not True:
        failures.append("product_gate_prerequisites_open")
    if signoff.get("ok") is not True:
        failures.append("operator_signoff_not_accepted")

    ok = not failures
    return {
        "name": "actual_product_switch_scope",
        "ok": ok,
        "failures": failures,
        "accepted_from_evidence": ok,
        "note": (
            "Small-group adoption scope is accepted from product-gate evidence and operator signoff; "
            "Python fallback is retained rather than forcibly retired."
            if ok
            else "Actual adoption scope remains blocked until product gates and operator signoff are green."
        ),
    }


def build_report(repo_root: Path, signoff_path: Path | None = None) -> dict[str, Any]:
    root = repo_root.resolve()
    product_report = product_gate_report.build_report(root)
    signoff = check_operator_signoff((root / DEFAULT_SIGNOFF) if signoff_path is None else signoff_path)
    gate_summary = summarize_product_gates(product_report)
    actual_scope = check_actual_adoption_scope(gate_summary, signoff)

    checks = [gate_summary, signoff, actual_scope]
    open_blockers: list[str] = []
    for check in checks:
        for failure in check.get("failures", []):
            open_blockers.append(str(failure))
    open_blockers.extend(f"gate_open:{gate}" for gate in gate_summary["prerequisite_open_gates"])

    final_green = all(check.get("ok") is True for check in checks)
    return {
        "schema_version": "archive-center.final_adoption_replay.v1",
        "generated_at": datetime.now(timezone.utc).replace(microsecond=0).isoformat(),
        "status": "ok" if final_green else "degraded",
        "final_adoption_green": final_green,
        "prerequisite_open_gates": gate_summary["prerequisite_open_gates"],
        "open_blockers": open_blockers,
        "scope": {
            "repo_root": str(root),
            "report_kind": "evidence_backed_adoption_replay",
            "evidenced_by_live_stack": actual_scope["ok"],
            "actual_scope_accepted": actual_scope["ok"],
            "mutates_source_tree": False,
            "authority_switch_evidence_green": actual_scope["ok"],
            "go_default_switch_evidence_green": actual_scope["ok"],
            "milvus_live_switch_evidence_green": actual_scope["ok"],
            "python_retirement": False,
            "python_fallback_retained": True,
        },
        "checks": checks,
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--out", default="")
    parser.add_argument("--signoff", default="")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    signoff_path = Path(args.signoff).resolve() if args.signoff else None
    report = build_report(Path(args.repo_root), signoff_path)
    payload = json.dumps(report, indent=2, ensure_ascii=False)
    if args.out:
        out_path = Path(args.out)
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(payload + "\n", encoding="utf-8")
    else:
        print(payload)
    return 0 if report["status"] == "ok" else 2


if __name__ == "__main__":
    raise SystemExit(main())
