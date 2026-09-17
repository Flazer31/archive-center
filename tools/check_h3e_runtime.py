#!/usr/bin/env python3
"""Check H-3e initiative runtime behavior against a running 2.0 backend.

This helper is diagnostic only. It sends the same supervisor context with
reactive, balanced, and proactive narrative stance settings, records each mode
independently, and never treats one mode timeout as a reason to hide the other
results.
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


MODES = ("reactive", "balanced", "proactive")


def as_dict(value: Any) -> dict[str, Any]:
    return value if isinstance(value, dict) else {}


def as_list(value: Any) -> list[Any]:
    return value if isinstance(value, list) else []


def text_value(value: Any, default: str = "") -> str:
    return value if isinstance(value, str) else default


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
    except Exception as err:  # pragma: no cover - manual runtime branch
        return {"ok": False, "status_code": None, "payload": {}, "error": str(err)}


def build_initiative_mode_suffix(mode: str) -> str:
    normalized = normalize_mode(mode)
    if normalized == "reactive":
        return "\n".join(
            [
                "",
                "[Story Initiative - Reactive]",
                "Stay close to the user's immediate lead and the current scene.",
                "If the user expresses caution, hesitation, or uncertainty, remain in observation, clarification, or low-risk preparation rather than pushing action.",
                "Do not initiate entry, unlock barriers, assign a plan, or commit companions to a risky move unless the user explicitly asks for that step.",
                "Advance existing threads only when the current exchange clearly opens space for it, and keep any suggestion small and reversible.",
            ]
        )
    if normalized == "proactive":
        return "\n".join(
            [
                "",
                "[Story Initiative - Proactive]",
                "You may introduce one plausible next beat or complication when continuity supports it.",
                "Initiative must grow from existing tensions, hooks, promises, or scene context.",
                "When the user is still deciding, propose the next beat rather than executing the decision on the user's behalf.",
                "Do not override the user's intent, skip causal steps, or force abrupt scene changes.",
            ]
        )
    return "\n".join(
        [
            "",
            "[Story Initiative - Balanced]",
            "You may add one gentle next-beat nudge when it naturally fits the current scene.",
            "Keep the response anchored to the user's immediate intent and the current arc, and suggest rather than execute the next step.",
            "Avoid abrupt escalation, forced twists, or hard scene jumps.",
        ]
    )


def build_initiative_mode_bounds(mode: str) -> dict[str, Any]:
    normalized = normalize_mode(mode)
    if normalized == "reactive":
        return {
            "emphasis": ["user-led follow-through", "observation before action", "low-risk option framing"],
            "forbidden_moves": [
                "unlocking barriers or initiating entry without explicit user intent",
                "committing the group to a risky plan on the user's behalf",
                "inventing urgent danger to force motion",
            ],
            "max_new_beats": 0,
            "allow_scene_jump": False,
        }
    if normalized == "proactive":
        return {
            "emphasis": ["causal next-beat proposal", "continuity-aware tension increase", "bounded steering"],
            "forbidden_moves": [
                "forcing irreversible turns without buildup",
                "overwriting the user's immediate intent",
                "turning a cautious pause into immediate entry or confrontation without buy-in",
            ],
            "max_new_beats": 1,
            "allow_scene_jump": False,
        }
    return {
        "emphasis": ["gentle next-beat nudges", "continuity-aware escalation", "conversation momentum"],
        "forbidden_moves": [
            "hard scene cut without setup",
            "forcing a dramatic turn too early",
            "executing a risky step before the user agrees to it",
        ],
        "max_new_beats": 1,
        "allow_scene_jump": False,
    }


def normalize_mode(mode: str) -> str:
    mode = (mode or "").strip().lower()
    return mode if mode in MODES else "balanced"


def build_payload(session_id: str, mode: str, user_input: str, guide_mode: str) -> dict[str, Any]:
    normalized = normalize_mode(mode)
    return {
        "chat_session_id": session_id,
        "context_messages": [{"role": "user", "content": user_input}],
        "guide_mode": guide_mode,
        "narrative_stance": normalized,
        "narrative_stance_suffix": build_initiative_mode_suffix(normalized),
        "narrative_stance_bounds": build_initiative_mode_bounds(normalized),
        "auto_advance_trigger": "none",
        "wake_up_context": "",
        "persistent_guidance": "",
    }


def summarize_directive(payload: dict[str, Any]) -> dict[str, Any]:
    result = as_dict(payload.get("supervisor_result"))
    directive = as_dict(result.get("directive") or result)
    story_author = as_dict(directive.get("story_author") or directive.get("book_author"))
    director = as_dict(directive.get("director"))
    section_world = as_dict(directive.get("section_world"))
    return {
        "current_arc": text_value(story_author.get("current_arc")),
        "narrative_goal": text_value(story_author.get("narrative_goal")),
        "pressure_level": text_value(director.get("pressure_level")),
        "required_outcomes": [str(v) for v in as_list(director.get("required_outcomes"))[:3]],
        "forbidden_moves": [str(v) for v in as_list(director.get("forbidden_moves"))[:3]],
        "section_world_keys": sorted(section_world.keys())[:8],
        "raw_text": text_value(as_dict(directive.get("directive")).get("raw_text") or directive.get("raw_text"))[:240],
    }


def summarize_mode(mode: str, request_payload: dict[str, Any], probe: dict[str, Any]) -> dict[str, Any]:
    payload = as_dict(probe.get("payload"))
    pack = as_dict(payload.get("supervisor_input_pack"))
    trace = as_dict(payload.get("trace_summary"))
    bounds = as_dict(request_payload.get("narrative_stance_bounds"))
    pack_bounds = as_dict(pack.get("narrative_stance_bounds"))
    trace_summary = as_dict(trace.get("narrative_stance_summary"))
    return {
        "mode": mode,
        "ok": probe.get("ok") is True,
        "status_code": probe.get("status_code"),
        "error": probe.get("error"),
        "response_status": payload.get("status"),
        "source": payload.get("source"),
        "would_call_llm": payload.get("would_call_llm"),
        "request_bounds": {
            "max_new_beats": bounds.get("max_new_beats"),
            "allow_scene_jump": bounds.get("allow_scene_jump"),
            "emphasis": as_list(bounds.get("emphasis"))[:3],
            "forbidden_moves": as_list(bounds.get("forbidden_moves"))[:3],
        },
        "pack_bounds": {
            "max_new_beats": pack_bounds.get("max_new_beats"),
            "allow_scene_jump": pack_bounds.get("allow_scene_jump"),
            "emphasis": as_list(pack_bounds.get("emphasis"))[:3],
            "forbidden_moves": as_list(pack_bounds.get("forbidden_moves"))[:3],
        },
        "trace": {
            "narrative_stance": trace.get("narrative_stance"),
            "suffix_present": trace.get("narrative_stance_suffix_present"),
            "bounds_present": trace.get("narrative_stance_bounds_present"),
            "summary_mode": trace_summary.get("mode"),
            "summary_max_new_beats": trace_summary.get("max_new_beats"),
            "summary_allow_scene_jump": trace_summary.get("allow_scene_jump"),
        },
        "directive_summary": summarize_directive(payload),
    }


def compare_modes(mode_reports: dict[str, dict[str, Any]]) -> dict[str, Any]:
    ok_modes = [mode for mode in MODES if mode_reports.get(mode, {}).get("ok")]
    directive_fingerprints = {
        mode: json.dumps(mode_reports[mode].get("directive_summary", {}), ensure_ascii=False, sort_keys=True)
        for mode in ok_modes
    }
    bounds_fingerprints = {
        mode: json.dumps(mode_reports[mode].get("request_bounds", {}), ensure_ascii=False, sort_keys=True)
        for mode in MODES
        if mode in mode_reports
    }
    return {
        "ok_modes": ok_modes,
        "failed_modes": [mode for mode in MODES if mode not in ok_modes],
        "all_modes_returned": len(ok_modes) == len(MODES),
        "request_bounds_differ": len(set(bounds_fingerprints.values())) > 1,
        "directive_diff_observable": len(ok_modes) >= 2 and len(set(directive_fingerprints.values())) > 1,
        "mode_comparison_available": len(ok_modes) == len(MODES),
    }


def build_h3e_checks(mode_reports: dict[str, dict[str, Any]], comparison: dict[str, Any]) -> dict[str, Any]:
    successful_modes = [mode for mode in MODES if mode_reports.get(mode, {}).get("ok")]
    directive_fingerprints = [
        json.dumps(mode_reports[mode].get("directive_summary", {}), ensure_ascii=False, sort_keys=True)
        for mode in successful_modes
    ]
    fallback_suspected_modes = []
    for mode in successful_modes:
        report = mode_reports[mode]
        source = text_value(report.get("source")).lower()
        would_call_llm = report.get("would_call_llm")
        if source in {"fallback", "default", "shadow", "stub"} or would_call_llm is False:
            fallback_suspected_modes.append(mode)
    exact_identical = len(successful_modes) >= 2 and len(set(directive_fingerprints)) == 1
    return {
        "successful_modes": successful_modes,
        "failed_modes": comparison.get("failed_modes", []),
        "fallback_suspected_modes": fallback_suspected_modes,
        "exact_identical": exact_identical,
        "request_bounds_differ": comparison.get("request_bounds_differ") is True,
        "directive_diff_observable": comparison.get("directive_diff_observable") is True,
        "mode_comparison_available": comparison.get("mode_comparison_available") is True,
    }


def build_report(
    session_id: str,
    user_input: str,
    guide_mode: str,
    timeout: float,
    health_probe: dict[str, Any],
    mode_probes: dict[str, dict[str, Any]],
) -> dict[str, Any]:
    modes: dict[str, dict[str, Any]] = {}
    for mode in MODES:
        payload = build_payload(session_id, mode, user_input, guide_mode)
        modes[mode] = summarize_mode(mode, payload, mode_probes.get(mode, {"ok": False, "error": "not_run"}))
    comparison = compare_modes(modes)
    health_ok = health_probe.get("ok") is True
    status = "ok"
    if not health_ok:
        status = "error"
    elif not comparison["all_modes_returned"]:
        status = "partial"
    elif not comparison["directive_diff_observable"]:
        status = "degraded"
    checks = {
        "health_ok": health_ok,
        "request_bounds_differ": comparison["request_bounds_differ"],
        "mode_comparison_available": comparison["mode_comparison_available"],
        "directive_diff_observable": comparison["directive_diff_observable"],
        "no_mode_timeout_blocks_report": True,
    }
    h3e_checks = build_h3e_checks(modes, comparison)
    return {
        "status": status,
        "checked_at": datetime.now(timezone.utc).isoformat(),
        "session_id": session_id,
        "guide_mode": guide_mode,
        "timeout_seconds": timeout,
        "user_input_preview": user_input[:240],
        "health": {
            "ok": health_probe.get("ok"),
            "status_code": health_probe.get("status_code"),
            "payload": as_dict(health_probe.get("payload")),
            "error": health_probe.get("error"),
        },
        "modes": modes,
        "comparison": comparison,
        "checks": checks,
        "h3e_checks": h3e_checks,
    }


def run(base_url: str, session_id: str, timeout: float, user_input: str, guide_mode: str) -> dict[str, Any]:
    health = fetch_json(base_url, "GET", "/health", None, timeout)
    mode_probes: dict[str, dict[str, Any]] = {}
    for mode in MODES:
        mode_probes[mode] = fetch_json(
            base_url,
            "POST",
            "/supervisor",
            build_payload(session_id, mode, user_input, guide_mode),
            timeout,
        )
    return build_report(session_id, user_input, guide_mode, timeout, health, mode_probes)


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Check H-3e initiative runtime mode differentiation.")
    parser.add_argument("--base-url", default="http://127.0.0.1:28080", help="Archive Center 2.0 backend base URL")
    parser.add_argument("--session-id", default="h3e-runtime-smoke", help="chat_session_id to use for supervisor probes")
    parser.add_argument("--timeout", type=float, default=60.0, help="per-request HTTP timeout in seconds")
    parser.add_argument("--guide-mode", default="off", help="narrative guide mode to hold constant during comparison")
    parser.add_argument(
        "--input",
        default="The user hesitates before a locked archive door and asks what to do next.",
        help="same user input sent to all three initiative modes",
    )
    parser.add_argument("--out", type=Path, help="optional JSON output path")
    parser.add_argument("--append-jsonl", type=Path, help="optional JSONL path to append one compact report record")
    parser.add_argument("--json-only", action="store_true", help="print only JSON, without the [H-3e Checks] summary")
    return parser.parse_args(argv)


def format_h3e_checks(report: dict[str, Any]) -> str:
    checks = as_dict(report.get("h3e_checks"))
    lines = [
        "[H-3e Checks]",
        f"status={report.get('status')}",
        f"successful_modes={','.join(as_list(checks.get('successful_modes'))) or '(none)'}",
        f"failed_modes={','.join(as_list(checks.get('failed_modes'))) or '(none)'}",
        f"fallback_suspected_modes={','.join(as_list(checks.get('fallback_suspected_modes'))) or '(none)'}",
        f"exact_identical={checks.get('exact_identical') is True}",
        f"request_bounds_differ={checks.get('request_bounds_differ') is True}",
        f"directive_diff_observable={checks.get('directive_diff_observable') is True}",
        f"mode_comparison_available={checks.get('mode_comparison_available') is True}",
    ]
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv or sys.argv[1:])
    report = run(args.base_url, args.session_id, args.timeout, args.input, args.guide_mode)
    text = json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True)
    if args.out:
        args.out.parent.mkdir(parents=True, exist_ok=True)
        args.out.write_text(text + "\n", encoding="utf-8")
    if args.append_jsonl:
        args.append_jsonl.parent.mkdir(parents=True, exist_ok=True)
        with args.append_jsonl.open("a", encoding="utf-8") as fh:
            fh.write(json.dumps(report, ensure_ascii=False, sort_keys=True) + "\n")
    if not args.json_only:
        print(format_h3e_checks(report))
    print(text)
    return 0 if report["status"] in {"ok", "degraded", "partial"} else 1


if __name__ == "__main__":
    raise SystemExit(main())
