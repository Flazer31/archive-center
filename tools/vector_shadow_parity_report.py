#!/usr/bin/env python3
"""Compare Chroma-vs-Milvus shadow result IDs for Archive Center 2.0.

The tool consumes JSON evidence only. It does not connect to Chroma, Milvus,
MariaDB, SQLite, or the 0.8 runtime.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any


SCHEMA_VERSION = "archive-center.vector-shadow-parity-report.v1"
DEFAULT_THRESHOLD = 0.8


def extract_ids(value: Any) -> list[str]:
    ids: list[str] = []
    if value is None:
        return ids
    if isinstance(value, str):
        return [value]
    if isinstance(value, list):
        for item in value:
            ids.extend(extract_ids(item))
        return ids
    if isinstance(value, dict):
        if "id" in value:
            ids.extend(extract_ids(value["id"]))
        if "ids" in value:
            ids.extend(extract_ids(value["ids"]))
        if "results" in value:
            ids.extend(extract_ids(value["results"]))
        if "documents" in value and "ids" not in value:
            ids.extend(extract_ids(value["documents"]))
    return ids


def unique_ordered(values: list[str]) -> list[str]:
    seen: set[str] = set()
    out: list[str] = []
    for value in values:
        if value not in seen:
            seen.add(value)
            out.append(value)
    return out


def duplicate_ids(values: list[str]) -> list[str]:
    seen: set[str] = set()
    dupes: set[str] = set()
    for value in values:
        if value in seen:
            dupes.add(value)
        seen.add(value)
    return sorted(dupes)


def jaccard(left: list[str], right: list[str]) -> float:
    left_set = set(left)
    right_set = set(right)
    if not left_set and not right_set:
        return 1.0
    union = left_set | right_set
    if not union:
        return 1.0
    return len(left_set & right_set) / len(union)


def first_present(source: dict[str, Any], keys: list[str]) -> Any:
    for key in keys:
        if key in source:
            return source[key]
    return None


def normalize_queries(payload: Any) -> list[dict[str, Any]]:
    if isinstance(payload, dict) and isinstance(payload.get("queries"), list):
        return payload["queries"]
    if isinstance(payload, list):
        return payload
    if isinstance(payload, dict):
        return [payload]
    raise ValueError("input JSON must be an object or list")


def compare_query(query: dict[str, Any], index: int, threshold: float, max_list: int) -> dict[str, Any]:
    query_id = str(query.get("query_id") or query.get("name") or f"query-{index + 1}")
    chroma_raw = first_present(query, ["chroma_ids", "chroma", "chroma_results", "left"])
    milvus_raw = first_present(query, ["milvus_ids", "milvus", "milvus_results", "right"])

    chroma_ids_raw = extract_ids(chroma_raw)
    milvus_ids_raw = extract_ids(milvus_raw)
    chroma_ids = unique_ordered(chroma_ids_raw)
    milvus_ids = unique_ordered(milvus_ids_raw)

    chroma_set = set(chroma_ids)
    milvus_set = set(milvus_ids)
    overlap = sorted(chroma_set & milvus_set)
    missing = [value for value in chroma_ids if value not in milvus_set]
    extra = [value for value in milvus_ids if value not in chroma_set]
    score = jaccard(chroma_ids, milvus_ids)

    status = "pass" if score >= threshold else "fail"
    if not chroma_ids and not milvus_ids:
        status = "warn_empty_both"
    elif not chroma_ids or not milvus_ids:
        status = "fail_empty_side"

    return {
        "query_id": query_id,
        "status": status,
        "threshold": threshold,
        "jaccard": round(score, 6),
        "chroma_count": len(chroma_ids),
        "milvus_count": len(milvus_ids),
        "overlap_count": len(overlap),
        "overlap_ids": overlap[:max_list],
        "missing_in_milvus": missing[:max_list],
        "extra_in_milvus": extra[:max_list],
        "chroma_duplicate_ids": duplicate_ids(chroma_ids_raw)[:max_list],
        "milvus_duplicate_ids": duplicate_ids(milvus_ids_raw)[:max_list],
    }


def build_report(payload: Any, threshold: float, max_list: int) -> tuple[dict[str, Any], int]:
    queries = normalize_queries(payload)
    failures: list[str] = []
    warnings: list[str] = []
    results: list[dict[str, Any]] = []

    if not queries:
        failures.append("no_queries")

    for index, query in enumerate(queries):
        if not isinstance(query, dict):
            failures.append(f"query_{index + 1}_not_object")
            continue
        result = compare_query(query, index, threshold, max_list)
        results.append(result)
        if result["status"].startswith("fail"):
            failures.append(f"{result['query_id']}:{result['status']}")
        elif result["status"].startswith("warn"):
            warnings.append(f"{result['query_id']}:{result['status']}")

    pass_count = sum(1 for item in results if item["status"] == "pass")
    avg_jaccard = 0.0
    if results:
        avg_jaccard = sum(float(item["jaccard"]) for item in results) / len(results)

    if failures:
        status = "fail"
        exit_code = 1
    elif warnings:
        status = "warn"
        exit_code = 0
    else:
        status = "ok"
        exit_code = 0

    return (
        {
            "schema_version": SCHEMA_VERSION,
            "tool": "vector_shadow_parity_report",
            "status": status,
            "threshold": threshold,
            "query_count": len(queries),
            "pass_count": pass_count,
            "warning_count": len(warnings),
            "failure_count": len(failures),
            "average_jaccard": round(avg_jaccard, 6),
            "queries": results,
            "warnings": warnings,
            "failures": failures,
            "non_goals": [
                "does_not_connect_to_chroma",
                "does_not_connect_to_milvus",
                "does_not_touch_archive_center_0_8",
                "does_not_switch_live_retrieval",
            ],
        },
        exit_code,
    )


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Compare Chroma/Milvus shadow result JSON")
    parser.add_argument("--input", required=True, help="JSON file containing query comparison evidence")
    parser.add_argument("--out", help="write JSON report to this path")
    parser.add_argument("--threshold", type=float, default=DEFAULT_THRESHOLD)
    parser.add_argument("--max-list", type=int, default=20)
    args = parser.parse_args(argv)
    if args.threshold < 0.0 or args.threshold > 1.0:
        parser.error("--threshold must be between 0 and 1")
    if args.max_list < 0:
        parser.error("--max-list must be >= 0")
    return args


def main(argv: list[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    payload = json.loads(Path(args.input).read_text(encoding="utf-8"))
    report, exit_code = build_report(payload, args.threshold, args.max_list)
    output = json.dumps(report, indent=2, ensure_ascii=True)
    if args.out:
        out_path = Path(args.out).expanduser().resolve()
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(output + "\n", encoding="utf-8")
    print(output)
    return exit_code


if __name__ == "__main__":
    raise SystemExit(main())
