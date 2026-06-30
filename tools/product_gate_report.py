#!/usr/bin/env python3
"""Summarize Archive Center 2.0 product gate readiness from evidence files.

This is a report-only validation tool. It never starts services, writes runtime
data, changes authority, or edits the 0.8 reference tree.
"""

from __future__ import annotations

import argparse
import json
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


@dataclass(frozen=True)
class Gate:
    gate_id: str
    name: str
    evidence: tuple[str, ...]
    green_when: str
    blocked_when: str


GATES: tuple[Gate, ...] = (
    Gate(
        "2.0-0",
        "readiness floor",
        (
            "docs/architecture-runtime-handoff-freeze.md",
            "benchmarks/runtime-startup-rss-r2-110-architecture-handoff.json",
            "benchmarks/low-resource-go-only-r2-114.json",
            "benchmarks/combined-stack-low-resource-r2-115.json",
        ),
        "current-runtime safety, Go-only, and combined MariaDB/Milvus/Go low-resource evidence all pass",
        "low-resource shakedown/current-runtime readiness closure is still open",
    ),
    Gate(
        "2.0-1",
        "architecture/runtime framing",
        (
            "docs/architecture-runtime-handoff-freeze.md",
            "benchmarks/runtime-startup-rss-r2-110-architecture-handoff.json",
            "benchmarks/baseline-capture-r2-110-architecture-handoff.json",
        ),
        "architecture/runtime handoff freeze evidence is present and startup/RSS probe is ok",
        "architecture/runtime handoff evidence is missing or degraded",
    ),
    Gate(
        "2.0-2",
        "MariaDB truth migration",
        (
            "benchmarks/managed-mariadb-e2e-r2-108-authority-cutover-replay.json",
            "benchmarks/windows-live-write-smoke-r2-124.json",
            "benchmarks/windows-live-stack-smoke-r2-125.json",
        ),
        "MariaDB authority replay plus managed live stack write/read smoke pass with product authority enabled",
        "authority replay, managed live stack write/read smoke, or rollback evidence is missing or not ok",
    ),
    Gate(
        "2.0-3",
        "Milvus vector migration",
        (
            "benchmarks/milvus-r2-product-read-rollback-decommission-2026-05-27-r2-97.json",
            "benchmarks/windows-live-stack-smoke-r2-125.json",
        ),
        "Milvus selected recall product-read plus managed live stack product-read proof pass",
        "Milvus product-read, rollback, Chroma decommission, or managed live stack proof is missing or not ok",
    ),
    Gate(
        "2.0-4",
        "Go runtime migration",
        (
            "benchmarks/managed-mariadb-e2e-r2-107-default-runtime-actual-switch.json",
            "benchmarks/windows-live-stack-smoke-r2-125.json",
        ),
        "Go default-runtime replay plus managed live stack runtime proof pass",
        "default switch replay, managed live stack runtime proof, Python fallback, or rollback evidence is missing or not ok",
    ),
    Gate(
        "2.0-4m",
        "backend decomposition",
        ("benchmarks/backend-surface-smoke-r2-111-decomposition-phase3-phase4-summary.json",),
        "all 2.0-side decomposition route families are mapped and smoke-tested",
        "Phase 3/4 decomposition smoke evidence is missing or not ok",
    ),
    Gate(
        "2.0-5",
        "packaging/ops",
        (
            "benchmarks/windows-single-file-bundle-r1-102.json",
            "benchmarks/platform-adoption-smoke-r2-121.json",
        ),
        "Windows plus accepted Linux, Termux, and macOS one-file/bootstrap adoption proofs pass",
        "At least one platform proof or update/repair/uninstall behavior remains unaccepted",
    ),
    Gate(
        "2.0-6",
        "validation/adoption",
        ("benchmarks/final-adoption-replay-r2-126.json",),
        "all product gates are green and final adoption replay is signed off",
        "this report is not a final adoption replay and at least one product gate remains open",
    ),
)


def load_json(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {}
    try:
        return json.loads(path.read_text(encoding="utf-8-sig"))
    except json.JSONDecodeError:
        return {"_decode_error": True}


def is_ok_status(payload: dict[str, Any]) -> bool:
    return payload.get("status") == "ok"


def gate_status(root: Path, gate: Gate) -> dict[str, Any]:
    evidence_paths = [root / item for item in gate.evidence]
    present = [path.exists() for path in evidence_paths]
    payloads = [load_json(path) if path.suffix.lower() == ".json" else {} for path in evidence_paths]

    green = False
    notes: list[str] = []

    if gate.gate_id == "2.0-0":
        # Low-resource readiness needs both Go-only and combined managed-stack evidence.
        startup = payloads[1] if len(payloads) > 1 else {}
        go_only = payloads[2] if len(payloads) > 2 else {}
        combined = payloads[3] if len(payloads) > 3 else {}
        go_only_ok = (
            is_ok_status(go_only)
            and go_only.get("checks", {}).get("startup_ok") is True
            and go_only.get("checks", {}).get("idle_rss_ok") is True
        )
        combined_ok = (
            is_ok_status(combined)
            and combined.get("checks", {}).get("mariadb_ready") is True
            and combined.get("checks", {}).get("milvus_ready") is True
            and combined.get("checks", {}).get("go_ready") is True
            and combined.get("checks", {}).get("go_rss_ok") is True
            and combined.get("checks", {}).get("combined_rss_ok") is True
            and combined.get("checks", {}).get("startup_ok") is True
            and combined.get("checks", {}).get("read_only_latency_ok") is True
        )
        green = all(present) and is_ok_status(startup) and go_only_ok and combined_ok
        if is_ok_status(startup):
            notes.append("startup/RSS handoff evidence is present")
        if go_only_ok:
            notes.append("Go-only low-resource shakedown evidence is present")
        if combined_ok:
            notes.append("combined MariaDB/Milvus/Go low-resource shakedown evidence is present")
        if not green:
            notes.append("combined-stack low-resource readiness is not fully green")
    elif gate.gate_id == "2.0-1":
        startup = payloads[1] if len(payloads) > 1 else {}
        baseline = payloads[2] if len(payloads) > 2 else {}
        green = all(present) and is_ok_status(startup) and is_ok_status(baseline)
    elif gate.gate_id == "2.0-2":
        payload = payloads[0] if payloads else {}
        live = payloads[1] if len(payloads) > 1 else {}
        live_stack = payloads[2] if len(payloads) > 2 else {}
        authority = payload.get("authority_cutover_replay", {})
        rollback = payload.get("rollback_proof", {})
        safety = payload.get("safety_flags", {})
        route_write = payload.get("route_write_smoke", {})
        live_ready = live.get("ready", {}).get("checks", {})
        live_write = live.get("write_smoke", {})
        stack_ready = live_stack.get("ready", {}).get("checks", {})
        stack_authority_ok = (
            is_ok_status(live_stack)
            and stack_ready.get("store_mode") == "mariadb_authority"
            and stack_ready.get("mariadb_authority") == "enabled"
            and live_stack.get("write_smoke", {}).get("status") == "ok"
        )
        live_write_ok = (
            is_ok_status(live)
            and live_ready.get("store_mode") == "mariadb_authority"
            and live_ready.get("mariadb_authority") == "enabled"
            and live_write.get("status") == "ok"
        )
        green = (
            is_ok_status(payload)
            and authority.get("status") == "ok"
            and authority.get("authority_switch") is True
            and live_write_ok
            and stack_authority_ok
            and (
                safety.get("authority_switch") is True
                or safety.get("mariadb_authority_default_enabled") is True
                or stack_authority_ok
            )
            and (safety.get("mariadb_product_read_persisted") is True or stack_authority_ok)
            and route_write.get("status") == "ok"
            and authority.get("fallback_available") is True
            and rollback.get("status") == "ok"
            and rollback.get("rolled_back") is True
        )
        if stack_authority_ok:
            notes.append("Windows managed live stack MariaDB authority write/read smoke is green")
        if authority.get("status") == "ok" and not green:
            notes.append(
                "MariaDB authority evidence is managed/report-only; persisted product authority is not proven"
            )
        if live_write_ok:
            notes.append("Windows live source MariaDB authority write/read smoke is green")
    elif gate.gate_id == "2.0-3":
        payload = payloads[0] if payloads else {}
        live_stack = payloads[1] if len(payloads) > 1 else {}
        summary = payload.get("summary", {})
        rollback = payload.get("rollback_proof", {})
        chroma = payload.get("chroma_decommission_proof", {})
        stack_ready = live_stack.get("ready", {}).get("checks", {})
        stack_milvus = live_stack.get("milvus_smoke", {})
        stack_summary = stack_milvus.get("summary", {})
        stack_milvus_ok = (
            is_ok_status(live_stack)
            and stack_ready.get("milvus") == "configured"
            and stack_ready.get("milvus_live_enabled") == "enabled"
            and stack_milvus.get("status") == "ok"
            and stack_summary.get("persisted_milvus_live_enabled") is True
            and stack_summary.get("persisted_live_retrieval_enabled") is True
            and stack_summary.get("bounded_shadow_route_only") is False
            and stack_summary.get("search_result") == "ok"
        )
        green = (
            is_ok_status(payload)
            and summary.get("search_result") == "ok"
            and summary.get("rollback_proof_ok") is True
            and (
                (
                    summary.get("persisted_milvus_live_enabled") is True
                    and summary.get("persisted_live_retrieval_enabled") is True
                    and summary.get("bounded_shadow_route_only") is not True
                )
                or stack_milvus_ok
            )
            and chroma.get("status") == "ok"
            and rollback.get("rolled_back") is True
            and stack_milvus_ok
        )
        if stack_milvus_ok:
            notes.append("Windows managed live stack Milvus product-read smoke is green")
        if summary.get("search_result") == "ok" and not green:
            notes.append(
                "Milvus evidence is selected-surface/bounded; persisted product live retrieval is not proven"
            )
    elif gate.gate_id == "2.0-4":
        payload = payloads[0] if payloads else {}
        live_stack = payloads[1] if len(payloads) > 1 else {}
        switch = payload.get("default_runtime_switch", {})
        rollback = payload.get("rollback_proof", {})
        safety = payload.get("safety_flags", {})
        stack_ready = live_stack.get("ready", {}).get("checks", {})
        stack_runtime_ok = (
            is_ok_status(live_stack)
            and stack_ready.get("shadow_mode") == "active"
            and stack_ready.get("store_mode") == "mariadb_authority"
            and live_stack.get("write_smoke", {}).get("status") == "ok"
            and live_stack.get("milvus_smoke", {}).get("status") == "ok"
        )
        green = (
            is_ok_status(payload)
            and switch.get("status") == "ok"
            and switch.get("go_default_switch") is True
            and (
                switch.get("persistent_switch") is True
                or safety.get("go_default_switch") is True
                or stack_runtime_ok
            )
            and switch.get("fallback_available") is True
            and rollback.get("status") == "ok"
            and rollback.get("rolled_back") is True
            and stack_runtime_ok
        )
        if stack_runtime_ok:
            notes.append("Windows managed live stack Go runtime proof is green")
        if switch.get("status") == "ok" and not green:
            notes.append(
                "Go default-runtime evidence is disposable/report-only; persisted product default is not proven"
            )
    elif gate.gate_id == "2.0-4m":
        payload = payloads[0] if payloads else {}
        phase_counts = {}
        for item in payload.get("phase_counts", []):
            if isinstance(item, dict):
                phase_counts.update(item)
        green = (
            is_ok_status(payload)
            and payload.get("total_checks", 0) >= 56
            and phase_counts.get("3", 0) >= 19
            and phase_counts.get("4", 0) >= 4
        )
    elif gate.gate_id == "2.0-5":
        bundle = payloads[0] if payloads else {}
        platform = payloads[1] if len(payloads) > 1 else {}
        scope = platform.get("scope", {})
        summary = platform.get("summary", {})
        real_platform_proofs = (
            summary.get("all_external_proofs_ok") is True
            and summary.get("external_proof_conditional", 0) == 0
            and scope.get("runs_installation") is True
        )
        accepted_conditional_proofs = (
            summary.get("all_platform_requirements_ok") is True
            and summary.get("external_proof_accepted", 0) == summary.get("external_proof_total", -1)
            and scope.get("normal_user_manual_mariadb_required") is False
            and scope.get("normal_user_manual_milvus_required") is False
        )
        green = (
            is_ok_status(bundle)
            and is_ok_status(platform)
            and platform.get("product_gate_green") is True
            and (real_platform_proofs or accepted_conditional_proofs)
        )
        if is_ok_status(bundle):
            notes.append("Windows single-file bundle proof is present")
        if platform.get("product_gate_green") is True and real_platform_proofs:
            notes.append("Linux/Termux/macOS adoption proof is accepted with real install execution")
        elif platform.get("product_gate_green") is True and accepted_conditional_proofs:
            notes.append("Linux/Termux/macOS adoption profiles are conditionally accepted by operator scope")
        elif platform.get("product_gate_green") is True:
            notes.append(
                "Platform adoption evidence is conditional/report-only; real install/update/repair/uninstall proof is not complete"
            )
        else:
            proof_summary = platform.get("summary", {})
            proofs = platform.get("external_proofs", [])
            accepted_targets = [
                proof.get("target")
                for proof in proofs
                if isinstance(proof, dict) and proof.get("gate_accepted") is True
            ]
            open_targets = [
                proof.get("target")
                for proof in proofs
                if isinstance(proof, dict) and proof.get("gate_accepted") is not True
            ]
            if accepted_targets:
                accepted = proof_summary.get("external_proof_accepted", len(accepted_targets))
                total = proof_summary.get("external_proof_total", len(proofs))
                notes.append(
                    "Accepted platform proofs: "
                    + ", ".join(str(target) for target in accepted_targets)
                    + f" ({accepted}/{total})"
                )
            if open_targets:
                notes.append(
                    "Open platform proof targets: "
                    + ", ".join(str(target) for target in open_targets)
                )
            if not accepted_targets and not open_targets:
                notes.append("Platform adoption proof is still missing or incomplete")
    elif gate.gate_id == "2.0-6":
        payload = payloads[0] if payloads else {}
        scope = payload.get("scope", {})
        green = (
            is_ok_status(payload)
            and payload.get("final_adoption_green") is True
            and payload.get("prerequisite_open_gates") == []
            and scope.get("actual_scope_accepted") is True
            and scope.get("authority_switch_evidence_green") is True
            and scope.get("go_default_switch_evidence_green") is True
            and scope.get("milvus_live_switch_evidence_green") is True
            and scope.get("python_fallback_retained") is True
        )
        if not green:
            blockers = payload.get("open_blockers")
            if isinstance(blockers, list) and blockers:
                notes.extend(str(item) for item in blockers[:8])
            else:
                notes.append("final adoption replay is report-only or product switches are still unproven")

    status = "green" if green else ("yellow" if any(present) else "red")
    if not green and not notes:
        notes.append(gate.blocked_when)

    return {
        "gate_id": gate.gate_id,
        "name": gate.name,
        "status": status,
        "green": green,
        "green_when": gate.green_when,
        "blocked_when": gate.blocked_when,
        "evidence": [
            {"path": item, "present": exists}
            for item, exists in zip(gate.evidence, present, strict=True)
        ],
        "notes": notes,
    }


def build_report(root: Path) -> dict[str, Any]:
    gates = [gate_status(root, gate) for gate in GATES]
    green_count = sum(1 for gate in gates if gate["green"])
    total = len(gates)
    readiness_percent = int((green_count / total) * 100)
    open_gates = [gate["gate_id"] for gate in gates if not gate["green"]]

    return {
        "schema_version": "archive-center.product_gate_report.v1",
        "generated_at": datetime.now(timezone.utc).replace(microsecond=0).isoformat(),
        "status": "ok" if green_count == total else "degraded",
        "product_gate_progress": f"{green_count} / {total}",
        "product_cutover_readiness_percent": readiness_percent,
        "green_count": green_count,
        "total_gates": total,
        "open_gates": open_gates,
        "gates": gates,
        "manager_note": (
            "Readiness summary for the accepted small-group scope. Conditional platform evidence is "
            "counted only when an explicit operator assumption profile accepts it and normal users do "
            "not manually install MariaDB or Milvus. Runtime/vector/authority gates require managed "
            "live-stack evidence, not report-only claims."
        ),
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--out", default="")
    args = parser.parse_args()

    report = build_report(Path(args.root).resolve())
    data = json.dumps(report, ensure_ascii=False, indent=2) + "\n"
    if args.out:
        Path(args.out).write_text(data, encoding="utf-8")
    else:
        print(data, end="")
    return 0 if report["status"] == "ok" else 2


if __name__ == "__main__":
    raise SystemExit(main())
