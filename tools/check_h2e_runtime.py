#!/usr/bin/env python3
"""Check H-2e storyline runtime observability against a running 2.0 backend.

This helper is diagnostic only. It performs read-only GET requests and one
read-only supervisor probe, then prints a compact JSON summary. It does not
write rows, change authority, or treat an empty session as a pass.
"""

from __future__ import annotations

import argparse
import json
import sys
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


def as_dict(value: Any) -> dict[str, Any]:
    return value if isinstance(value, dict) else {}


def as_list(value: Any) -> list[Any]:
    return value if isinstance(value, list) else []


def int_value(value: Any, default: int = 0) -> int:
    if isinstance(value, bool):
        return default
    if isinstance(value, int):
        return value
    if isinstance(value, float):
        return int(value)
    if isinstance(value, str):
        try:
            return int(value)
        except ValueError:
            return default
    return default


def bool_value(value: Any) -> bool:
    return value is True or (isinstance(value, str) and value.lower() == "true")


def fetch_json(base_url: str, method: str, path: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    url = base_url.rstrip("/") + path
    data = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, method=method.upper(), headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode("utf-8")
            decoded = json.loads(raw) if raw.strip() else {}
            return {"ok": 200 <= resp.status < 300, "status_code": resp.status, "payload": decoded}
    except urllib.error.HTTPError as err:
        body = err.read().decode("utf-8", errors="replace")
        try:
            decoded = json.loads(body) if body.strip() else {}
        except json.JSONDecodeError:
            decoded = {"raw_body": body[:500]}
        return {"ok": False, "status_code": err.code, "payload": decoded, "error": str(err)}
    except Exception as err:  # pragma: no cover - exercised by manual runtime use
        return {"ok": False, "status_code": None, "payload": {}, "error": str(err)}


def summarize_storylines(payload: dict[str, Any]) -> dict[str, Any]:
    rows = as_list(payload.get("storylines") or payload.get("items") or payload.get("rows"))
    active = [row for row in rows if as_dict(row).get("status") == "active" and not bool_value(as_dict(row).get("suppressed"))]
    stale = [row for row in active if bool_value(as_dict(row).get("is_stale"))]
    resolved = [row for row in rows if as_dict(row).get("status") == "resolved"]
    return {
        "status": payload.get("status", "unknown"),
        "count": int_value(payload.get("count"), len(rows)),
        "active_count": len(active),
        "stale_active_count": len(stale),
        "resolved_count": len(resolved),
        "reference_turn": payload.get("reference_turn"),
        "preview_names": [str(as_dict(row).get("name", "")) for row in rows[:5]],
    }


def summarize_export(payload: dict[str, Any]) -> dict[str, Any]:
    summary = as_dict(payload.get("summary"))
    chat_logs = as_list(payload.get("chat_logs"))
    storylines = as_list(payload.get("storylines"))
    return {
        "status": payload.get("status", "unknown"),
        "chat_logs_count": int_value(summary.get("chat_logs_count"), len(chat_logs)),
        "storylines_count": int_value(summary.get("storylines_count"), len(storylines)),
        "raw_chat_logs_present": len(chat_logs) > 0 or int_value(summary.get("chat_logs_count")) > 0,
        "storyline_rows_present": len(storylines) > 0 or int_value(summary.get("storylines_count")) > 0,
    }


def summarize_selection(selection: dict[str, Any]) -> dict[str, Any]:
    selected = [as_dict(item) for item in as_list(selection.get("selected"))]
    dropped = [as_dict(item) for item in as_list(selection.get("dropped"))]
    stale_selected = int_value(selection.get("stale_selected_count"))
    if stale_selected == 0:
        stale_selected = sum(1 for item in selected if bool_value(item.get("is_stale")))
    stale_dropped = int_value(selection.get("stale_dropped_count"))
    if stale_dropped == 0:
        stale_dropped = sum(1 for item in dropped if bool_value(item.get("is_stale")))
    selected_count = int_value(selection.get("selected_count"), len(selected))
    dropped_count = int_value(selection.get("dropped_count"), len(dropped))
    total_active = int_value(selection.get("total_active_count"), selected_count + dropped_count)
    return {
        "policy_version": selection.get("policy_version"),
        "reference_turn": selection.get("reference_turn"),
        "total_active_count": total_active,
        "selected_count": selected_count,
        "dropped_count": dropped_count,
        "stale_selected_count": stale_selected,
        "stale_dropped_count": stale_dropped,
        "fresh_rows_take_priority": selection.get("fresh_rows_take_priority"),
        "selected_preview": [str(item.get("name", "")) for item in selected[:5]],
        "dropped_preview": [str(item.get("name", "")) for item in dropped[:5]],
    }


def build_report(
    session_id: str,
    storylines_probe: dict[str, Any],
    supervisor_probe: dict[str, Any],
    export_probe: dict[str, Any],
) -> dict[str, Any]:
    storylines_payload = as_dict(storylines_probe.get("payload"))
    supervisor_payload = as_dict(supervisor_probe.get("payload"))
    export_payload = as_dict(export_probe.get("payload"))
    supervisor_pack = as_dict(supervisor_payload.get("supervisor_input_pack"))
    selection = summarize_selection(as_dict(supervisor_pack.get("storyline_selection") or as_dict(supervisor_payload.get("trace_summary")).get("storyline_selection")))
    storylines = summarize_storylines(storylines_payload)
    export = summarize_export(export_payload)

    active_storyline_path_exercised = storylines["active_count"] > 0 or selection["total_active_count"] > 0
    selection_pressure_exercised = selection["total_active_count"] > 0 and (
        selection["selected_count"] > 0 or selection["dropped_count"] > 0
    )
    stale_selected = selection["stale_selected_count"] > 0
    stale_filter_effective = selection["stale_dropped_count"] > 0 and not stale_selected
    stale_contamination_risk = stale_selected
    upstream_sync_gap = export["raw_chat_logs_present"] and storylines["count"] == 0

    checks = {
        "active_storyline_path_exercised": active_storyline_path_exercised,
        "selection_pressure_exercised": selection_pressure_exercised,
        "stale_selected": stale_selected,
        "stale_filter_effective": stale_filter_effective,
        "stale_contamination_risk": stale_contamination_risk,
        "raw_chat_without_storyline_rows": upstream_sync_gap,
    }
    status = "ok" if all(probe.get("ok") for probe in (storylines_probe, supervisor_probe, export_probe)) else "error"
    if status == "ok" and (not active_storyline_path_exercised or stale_contamination_risk or upstream_sync_gap):
        status = "degraded"

    return {
        "status": status,
        "checked_at": datetime.now(timezone.utc).isoformat(),
        "session_id": session_id,
        "timeout_seconds": None,
        "http": {
            "storylines": {"ok": storylines_probe.get("ok"), "status_code": storylines_probe.get("status_code")},
            "supervisor": {"ok": supervisor_probe.get("ok"), "status_code": supervisor_probe.get("status_code")},
            "export": {"ok": export_probe.get("ok"), "status_code": export_probe.get("status_code")},
        },
        "storylines": storylines,
        "supervisor": {
            "status": supervisor_payload.get("status", "unknown"),
            "source": supervisor_payload.get("source"),
            "selection": selection,
        },
        "export": export,
        "checks": checks,
    }


def run(base_url: str, session_id: str, timeout: float) -> dict[str, Any]:
    storylines = fetch_json(base_url, "GET", f"/storylines/{session_id}", None, timeout)
    supervisor = fetch_json(
        base_url,
        "POST",
        "/supervisor",
        {
            "chat_session_id": session_id,
            "context_messages": [{"role": "user", "content": "H-2e runtime contamination check"}],
            "guide_mode": "strict",
        },
        timeout,
    )
    export = fetch_json(base_url, "GET", f"/sessions/{session_id}/export", None, timeout)
    report = build_report(session_id, storylines, supervisor, export)
    report["timeout_seconds"] = timeout
    return report


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Check H-2e storyline runtime observability.")
    parser.add_argument("--base-url", default="http://127.0.0.1:28080", help="Archive Center 2.0 backend base URL")
    parser.add_argument("--session-id", required=True, help="chat_session_id to inspect")
    parser.add_argument("--timeout", type=float, default=60.0, help="HTTP timeout in seconds")
    parser.add_argument("--out", type=Path, help="optional JSON output path")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv or sys.argv[1:])
    report = run(args.base_url, args.session_id, args.timeout)
    text = json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True)
    if args.out:
        args.out.parent.mkdir(parents=True, exist_ok=True)
        args.out.write_text(text + "\n", encoding="utf-8")
    print(text)
    return 0 if report["status"] in {"ok", "degraded"} else 1


if __name__ == "__main__":
    raise SystemExit(main())
