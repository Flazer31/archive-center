#!/usr/bin/env python3
"""Probe the Go Milvus shadow boundary with existing vector parity evidence.

This is an R1 evidence tool. It reads a Chroma/Milvus query-set JSON artifact,
builds a dry-run payload for the Go shadow boundary, posts it to
`/milvus-shadow/backfill-compare`, and writes a JSON report. It does not
connect to Chroma, Milvus, MariaDB, SQLite, or the 0.8 runtime.
"""

from __future__ import annotations

import argparse
import json
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any


SCHEMA_VERSION = "archive-center.milvus-shadow-boundary-probe.v1"
DEFAULT_QUERY_SET = Path(__file__).resolve().parents[1] / "benchmarks" / "chroma-milvus-query-set-2026-05-25-real.json"


def json_dump(path: Path, payload: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2, ensure_ascii=True) + "\n", encoding="utf-8")


def load_query_set(path: Path) -> dict[str, Any]:
    payload = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(payload, dict):
        raise ValueError("query set JSON must be an object")
    queries = payload.get("queries")
    if not isinstance(queries, list) or not queries:
        raise ValueError("query set JSON must contain a non-empty queries array")
    return payload


def normalize_vector(value: Any) -> list[float]:
    if not isinstance(value, list):
        return []
    out: list[float] = []
    for item in value:
        try:
            out.append(float(item))
        except (TypeError, ValueError):
            return []
    return out


def build_doc(query: dict[str, Any]) -> dict[str, Any]:
    source_id = str(query.get("source_id") or query.get("id") or "")
    return {
        "id": source_id,
        "embedding": normalize_vector(query.get("embedding")),
        "tier": str(query.get("tier") or ""),
        "source_table": str(query.get("source_table") or ""),
        "source_row_id": str(query.get("source_row_id") or ""),
        "schema_version": "milvus_shadow_boundary_probe_from_query_set.v1",
        "document_text": str(query.get("document_excerpt") or source_id),
    }


def build_boundary_payload(query_set: dict[str, Any], *, max_docs: int, query_index: int, limit: int) -> dict[str, Any]:
    queries = [item for item in query_set.get("queries", []) if isinstance(item, dict)]
    if not queries:
        raise ValueError("query set contains no object queries")
    if query_index < 0 or query_index >= len(queries):
        raise ValueError(f"query_index {query_index} is outside the query set")

    query = queries[query_index]
    docs = [build_doc(item) for item in queries[:max_docs]]
    docs = [doc for doc in docs if doc["id"] and doc["embedding"]]
    if not docs:
        raise ValueError("query set produced no docs with both id and embedding")

    query_vector = normalize_vector(query.get("embedding"))
    if not query_vector:
        raise ValueError("selected query has no usable embedding")

    chat_session_id = str(query.get("chat_session_id") or docs[0].get("chat_session_id") or "default")
    return {
        "chat_session_id": chat_session_id,
        "dry_run": True,
        "allow_shadow_boundary": True,
        "docs": docs,
        "query_vector": query_vector,
        "limit": limit,
        "filter": "tier=memory",
    }


def post_json(url: str, payload: dict[str, Any], timeout: float) -> tuple[int | None, dict[str, Any] | None, str | None]:
    data = json.dumps(payload, ensure_ascii=True).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = resp.read().decode("utf-8", errors="replace")
            return resp.status, json.loads(body), None
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        try:
            parsed = json.loads(body)
        except json.JSONDecodeError:
            parsed = {"raw_body": body}
        return exc.code, parsed, None
    except Exception as exc:  # noqa: BLE001 - report probe transport failure as JSON evidence.
        return None, None, f"{type(exc).__name__}: {exc}"


def build_report(
    *,
    query_set_path: Path,
    query_set: dict[str, Any],
    endpoint: str,
    request_payload: dict[str, Any],
    http_status: int | None,
    response_payload: dict[str, Any] | None,
    transport_error: str | None,
    elapsed_ms: float,
) -> tuple[dict[str, Any], int]:
    failures: list[str] = []
    warnings: list[str] = []
    if transport_error:
        failures.append("transport_error")
    if http_status != 200:
        failures.append(f"http_status_{http_status}")
    if response_payload is None:
        failures.append("missing_response_json")
    else:
        if response_payload.get("status") != "ok":
            failures.append("response_status_not_ok")
        if response_payload.get("live_retrieval_enabled") is not False:
            failures.append("live_retrieval_not_false")
        if response_payload.get("milvus_live_enabled") is not False:
            failures.append("milvus_live_not_false")
        if response_payload.get("boundary_exercised") is not True:
            warnings.append("boundary_exercised_not_true")
        if response_payload.get("backfill_doc_count") != len(request_payload["docs"]):
            warnings.append("backfill_doc_count_mismatch")

    status = "ok" if not failures else "fail"
    exit_code = 0 if status == "ok" else 1
    report = {
        "schema_version": SCHEMA_VERSION,
        "tool": "milvus_shadow_boundary_probe",
        "status": status,
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "endpoint": endpoint,
        "http_status": http_status,
        "elapsed_ms": round(elapsed_ms, 3),
        "transport_error": transport_error,
        "source_evidence": {
            "query_set_path": str(query_set_path),
            "source_mode": query_set.get("source_mode"),
            "synthetic_embeddings": query_set.get("synthetic_embeddings"),
            "mirrored_doc_count": query_set.get("mirrored_doc_count"),
            "query_count": query_set.get("query_count"),
            "metric_type": query_set.get("metric_type"),
        },
        "request_summary": {
            "chat_session_id": request_payload["chat_session_id"],
            "doc_count": len(request_payload["docs"]),
            "query_vector_dim": len(request_payload["query_vector"]),
            "limit": request_payload["limit"],
            "filter": request_payload["filter"],
        },
        "response": response_payload,
        "warnings": warnings,
        "failures": failures,
        "non_goals": [
            "does_not_connect_to_chroma",
            "does_not_connect_to_milvus",
            "does_not_connect_to_mariadb",
            "does_not_connect_to_sqlite",
            "does_not_touch_archive_center_0_8",
            "does_not_switch_live_retrieval",
            "does_not_enable_go_default_runtime",
        ],
    }
    return report, exit_code


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Probe the Go Milvus shadow boundary")
    parser.add_argument("--query-set", default=str(DEFAULT_QUERY_SET))
    parser.add_argument("--go-base", default="http://127.0.0.1:28080")
    parser.add_argument("--out", help="write JSON report to this path")
    parser.add_argument("--max-docs", type=int, default=5)
    parser.add_argument("--query-index", type=int, default=0)
    parser.add_argument("--limit", type=int, default=5)
    parser.add_argument("--timeout", type=float, default=5.0)
    args = parser.parse_args(argv)
    if args.max_docs <= 0:
        parser.error("--max-docs must be > 0")
    if args.limit <= 0:
        parser.error("--limit must be > 0")
    if args.timeout <= 0:
        parser.error("--timeout must be > 0")
    return args


def main(argv: list[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    query_set_path = Path(args.query_set).expanduser().resolve()
    query_set = load_query_set(query_set_path)
    request_payload = build_boundary_payload(
        query_set,
        max_docs=args.max_docs,
        query_index=args.query_index,
        limit=args.limit,
    )
    endpoint = args.go_base.rstrip("/") + "/milvus-shadow/backfill-compare"
    start = time.perf_counter()
    http_status, response_payload, transport_error = post_json(endpoint, request_payload, args.timeout)
    elapsed_ms = (time.perf_counter() - start) * 1000.0
    report, exit_code = build_report(
        query_set_path=query_set_path,
        query_set=query_set,
        endpoint=endpoint,
        request_payload=request_payload,
        http_status=http_status,
        response_payload=response_payload,
        transport_error=transport_error,
        elapsed_ms=elapsed_ms,
    )
    output = json.dumps(report, indent=2, ensure_ascii=True)
    if args.out:
        json_dump(Path(args.out).expanduser().resolve(), report)
    print(output)
    return exit_code


if __name__ == "__main__":
    raise SystemExit(main())
