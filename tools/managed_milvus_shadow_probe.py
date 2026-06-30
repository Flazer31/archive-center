#!/usr/bin/env python3
"""Run a managed temp Milvus Lite shadow backfill/read-compare probe.

This R1 evidence tool reads an existing query-set JSON artifact, opens a
temporary Milvus Lite database outside the source tree, inserts the query-set
documents, searches the same query vectors, and writes a JSON report. It does
not connect to Chroma, MariaDB, SQLite, the 0.8 runtime, or the Go live vector
store.
"""

from __future__ import annotations

import argparse
import importlib
import importlib.metadata
import importlib.util
import json
import shutil
import sys
import tempfile
import time
from pathlib import Path
from typing import Any


SCHEMA_VERSION = "archive-center.managed-milvus-shadow-probe.v1"
DEFAULT_COLLECTION = "archive_center_managed_shadow_probe"
DEFAULT_QUERY_SET = Path(__file__).resolve().parents[1] / "benchmarks" / "chroma-milvus-query-set-2026-05-25-real.json"
SAFE_FAIL_OPEN = {
    "live_retrieval_enabled": False,
    "go_vector_store_enabled": False,
    "mariadb_authority_enabled": False,
    "fallback_action": "emit_json_evidence_without_switching_live_paths",
}


def repo_root() -> Path:
    return Path(__file__).resolve().parents[1]


def module_available(name: str) -> bool:
    return importlib.util.find_spec(name) is not None


def package_version(name: str) -> str | None:
    try:
        return importlib.metadata.version(name)
    except importlib.metadata.PackageNotFoundError:
        return None


def path_inside(child: Path, parent: Path) -> bool:
    try:
        child.resolve().relative_to(parent.resolve())
        return True
    except ValueError:
        return False


def json_dump(path: Path, payload: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2, ensure_ascii=True) + "\n", encoding="utf-8")


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


def load_query_set(path: Path) -> dict[str, Any]:
    payload = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(payload, dict):
        raise ValueError("query set JSON must be an object")
    queries = payload.get("queries")
    if not isinstance(queries, list) or not queries:
        raise ValueError("query set JSON must contain a non-empty queries array")
    return payload


def build_rows(query_set: dict[str, Any], max_docs: int) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    queries = [item for item in query_set.get("queries", []) if isinstance(item, dict)]
    for index, query in enumerate(queries[:max_docs], start=1):
        vector = normalize_vector(query.get("embedding"))
        source_id = str(query.get("source_id") or "")
        if not source_id or not vector:
            continue
        rows.append(
            {
                "id": index,
                "vector": vector,
                "source_id": source_id,
                "text": str(query.get("document_excerpt") or source_id),
                "tier": str(query.get("tier") or ""),
                "chat_session_id": str(query.get("chat_session_id") or ""),
                "source_table": str(query.get("source_table") or ""),
                "source_row_id": str(query.get("source_row_id") or ""),
            }
        )
    return rows


def build_search_queries(query_set: dict[str, Any], query_limit: int) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    queries = [item for item in query_set.get("queries", []) if isinstance(item, dict)]
    for index, query in enumerate(queries[:query_limit], start=1):
        vector = normalize_vector(query.get("embedding"))
        source_id = str(query.get("source_id") or "")
        if not source_id or not vector:
            continue
        out.append(
            {
                "query_id": str(query.get("query_id") or f"q{index}"),
                "source_id": source_id,
                "vector": vector,
            }
        )
    return out


def get_hit_source_id(hit: Any) -> str | None:
    if isinstance(hit, dict):
        entity = hit.get("entity")
        if isinstance(entity, dict):
            for key in ("source_id", "id"):
                value = entity.get(key)
                if value is not None:
                    return str(value)
        for key in ("source_id", "id"):
            value = hit.get(key)
            if value is not None:
                return str(value)
    return None


def cleanup_dir(path: Path) -> str | None:
    last_error: str | None = None
    for _ in range(5):
        try:
            shutil.rmtree(path)
            return None
        except FileNotFoundError:
            return None
        except (PermissionError, OSError) as exc:
            last_error = f"{type(exc).__name__}: {exc}"
            time.sleep(0.2)
    return last_error


def close_client(client: Any) -> None:
    for method_name in ("close", "disconnect"):
        method = getattr(client, method_name, None)
        if callable(method):
            method()
            return


def jaccard(left: list[str], right: list[str]) -> float:
    left_set = set(left)
    right_set = set(right)
    if not left_set and not right_set:
        return 1.0
    union = left_set | right_set
    if not union:
        return 1.0
    return len(left_set & right_set) / len(union)


def comparison_ids_by_query(probe: dict[str, Any]) -> dict[str, list[str]]:
    out: dict[str, list[str]] = {}
    for item in probe.get("comparisons", []):
        if not isinstance(item, dict):
            continue
        query_id = str(item.get("query_id") or "")
        ids = item.get("milvus_ids")
        if query_id and isinstance(ids, list):
            out[query_id] = [str(value) for value in ids]
    return out


def build_rebuild_parity(runs: list[dict[str, Any]]) -> dict[str, Any]:
    if len(runs) < 2:
        return {
            "requested": False,
            "status": "not_run",
            "run_count": len(runs),
            "mismatches": [],
        }

    baseline = comparison_ids_by_query(runs[0])
    mismatches: list[dict[str, Any]] = []
    for run_index, run in enumerate(runs[1:], start=2):
        current = comparison_ids_by_query(run)
        for query_id, baseline_ids in baseline.items():
            current_ids = current.get(query_id, [])
            if current_ids != baseline_ids:
                mismatches.append(
                    {
                        "run": run_index,
                        "query_id": query_id,
                        "baseline_ids": baseline_ids,
                        "current_ids": current_ids,
                    }
                )

    return {
        "requested": True,
        "status": "ok" if not mismatches else "mismatch",
        "run_count": len(runs),
        "baseline_query_count": len(baseline),
        "mismatch_count": len(mismatches),
        "mismatches": mismatches,
    }


def run_temp_milvus_probe(
    *,
    rows: list[dict[str, Any]],
    queries: list[dict[str, Any]],
    collection: str,
    result_limit: int,
    metric_type: str,
    persist_path: Path | None = None,
) -> dict[str, Any]:
    from pymilvus import MilvusClient  # type: ignore

    temp_dir: Path | None = None
    if persist_path is None:
        temp_dir = Path(tempfile.mkdtemp(prefix="archive-center-managed-milvus-"))
        db_path = temp_dir / "managed_shadow.db"
    else:
        db_path = persist_path
        db_path.parent.mkdir(parents=True, exist_ok=True)
    client = MilvusClient(str(db_path))
    dimension = len(rows[0]["vector"])
    cleanup_warning: str | None = None
    started = time.perf_counter()
    result: dict[str, Any] | None = None
    try:
        try:
            client.create_collection(collection_name=collection, dimension=dimension, metric_type=metric_type)
        except TypeError:
            client.create_collection(collection_name=collection, dimension=dimension)
        insert_result = client.insert(collection_name=collection, data=rows)

        comparisons: list[dict[str, Any]] = []
        for query in queries:
            raw_hits = client.search(
                collection_name=collection,
                data=[query["vector"]],
                limit=result_limit,
                output_fields=["source_id", "text", "tier", "chat_session_id", "source_table", "source_row_id"],
            )
            hits = raw_hits[0] if raw_hits else []
            milvus_ids = [value for value in (get_hit_source_id(hit) for hit in hits) if value]
            expected_ids = [query["source_id"]]
            comparisons.append(
                {
                    "query_id": query["query_id"],
                    "source_id": query["source_id"],
                    "expected_ids": expected_ids,
                    "milvus_ids": milvus_ids,
                    "hit_count": len(milvus_ids),
                    "top1_match": bool(milvus_ids) and milvus_ids[0] == query["source_id"],
                    "jaccard_vs_expected": round(jaccard(expected_ids, milvus_ids), 6),
                }
            )
        duration_ms = round((time.perf_counter() - started) * 1000.0, 3)
        result = {
            "status": "ok",
            "db_path": str(db_path),
            "collection": collection,
            "dimension": dimension,
            "metric_type": metric_type,
            "insert_result": str(insert_result),
            "insert_count": len(rows),
            "query_count": len(queries),
            "duration_ms": duration_ms,
            "comparisons": comparisons,
            "cleanup_warning": cleanup_warning,
        }
    finally:
        try:
            close_client(client)
        except Exception:
            pass
        if temp_dir is not None:
            cleanup_warning = cleanup_dir(temp_dir)
            if cleanup_warning:
                print(f"warning: temp cleanup incomplete: {cleanup_warning}", file=sys.stderr)
    if result is None:
        raise RuntimeError("managed Milvus probe did not produce a result")
    result["cleanup_warning"] = cleanup_warning
    result["temp_dir_removed"] = cleanup_warning is None
    return result


def dependency_status() -> dict[str, Any]:
    return {
        "pymilvus_available": module_available("pymilvus"),
        "pymilvus_version": package_version("pymilvus"),
        "milvus_lite_available": module_available("milvus_lite"),
        "milvus_lite_version": package_version("milvus-lite"),
    }


def build_report(args: argparse.Namespace) -> tuple[dict[str, Any], int]:
    query_set_path = Path(args.query_set).expanduser().resolve()
    query_set = load_query_set(query_set_path)
    rows = build_rows(query_set, args.max_docs)
    queries = build_search_queries(query_set, args.query_limit)
    deps = dependency_status()
    failures: list[str] = []
    warnings: list[str] = []
    probe: dict[str, Any]
    rebuild_runs: list[dict[str, Any]] = []
    persist_path: Path | None = None

    if not rows:
        failures.append("no_rows")
    if not queries:
        failures.append("no_queries")
    if args.persist_path:
        persist_path = Path(args.persist_path).expanduser().resolve()
        if path_inside(persist_path, repo_root()):
            failures.append("persist_path_inside_source_tree")

    if not deps["pymilvus_available"]:
        failures.append("pymilvus_not_available")
    if not deps["milvus_lite_available"]:
        failures.append("milvus_lite_package_not_available")

    if failures:
        probe = {"status": "not_run"}
        status = "failed"
        exit_code = 1
    else:
        try:
            run_count = max(1, int(args.rebuild_runs))
            for run_index in range(run_count):
                rebuild_runs.append(
                    run_temp_milvus_probe(
                        rows=rows,
                        queries=queries,
                        collection=f"{args.collection}_{run_index + 1}" if run_count > 1 else args.collection,
                        result_limit=args.result_limit,
                        metric_type=args.metric_type,
                        persist_path=persist_path,
                    )
                )
            probe = rebuild_runs[0]
            if probe.get("cleanup_warning"):
                warnings.append("temp_cleanup_incomplete")
            for run in rebuild_runs[1:]:
                if run.get("cleanup_warning"):
                    warnings.append("temp_cleanup_incomplete")
            failed_top1 = sorted(
                {
                    item["query_id"]
                    for run in rebuild_runs
                    for item in run.get("comparisons", [])
                    if not item.get("top1_match")
                }
            )
            if failed_top1:
                failures.append("top1_mismatch:" + ",".join(failed_top1))
            rebuild_parity = build_rebuild_parity(rebuild_runs)
            if rebuild_parity.get("status") == "mismatch":
                failures.append("rebuild_parity_mismatch")
        except Exception as exc:  # pragma: no cover - depends on local package state.
            probe = {"status": "error", "error": f"{type(exc).__name__}: {exc}"}
            failures.append("milvus_probe_error")
            rebuild_parity = {
                "requested": args.rebuild_runs > 1,
                "status": "not_run",
                "run_count": 0,
                "mismatches": [],
            }

        if failures:
            status = "failed"
            exit_code = 1
        elif warnings:
            status = "degraded"
            exit_code = 0
        else:
            status = "ok"
            exit_code = 0

    report = {
        "schema_version": SCHEMA_VERSION,
        "tool": "managed_milvus_shadow_probe",
        "status": status,
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "dependency_status": deps,
        "source_evidence": {
            "query_set_path": str(query_set_path),
            "source_mode": query_set.get("source_mode"),
            "synthetic_embeddings": query_set.get("synthetic_embeddings"),
            "mirrored_doc_count": query_set.get("mirrored_doc_count"),
            "query_count": query_set.get("query_count"),
            "metric_type": query_set.get("metric_type"),
        },
        "request_summary": {
            "row_count": len(rows),
            "query_count": len(queries),
            "result_limit": args.result_limit,
            "metric_type": args.metric_type,
        },
        "managed_milvus_probe": probe,
        "rebuild_runs": rebuild_runs,
        "rebuild_parity": rebuild_parity if "rebuild_parity" in locals() else build_rebuild_parity(rebuild_runs),
        "fail_open_checks": {
            **SAFE_FAIL_OPEN,
            "dependency_failure_safe": bool(
                "pymilvus_not_available" in failures
                or "milvus_lite_package_not_available" in failures
            ),
            "probe_error_safe": "milvus_probe_error" in failures,
            "invalid_input_safe": False,
        },
        "warnings": warnings,
        "failures": failures,
        "non_goals": [
            "does_not_connect_to_chroma",
            "does_not_connect_to_mariadb",
            "does_not_connect_to_sqlite",
            "does_not_touch_archive_center_0_8",
            "does_not_enable_go_vector_store",
            "does_not_switch_live_retrieval",
            "does_not_enable_go_default_runtime",
        ],
    }
    return report, exit_code


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run managed temp Milvus Lite shadow probe")
    parser.add_argument("--query-set", default=str(DEFAULT_QUERY_SET))
    parser.add_argument("--out", help="write JSON report to this path")
    parser.add_argument("--collection", default=DEFAULT_COLLECTION)
    parser.add_argument("--max-docs", type=int, default=16)
    parser.add_argument("--query-limit", type=int, default=5)
    parser.add_argument("--result-limit", type=int, default=5)
    parser.add_argument("--metric-type", default="L2", choices=["L2", "COSINE", "IP"])
    parser.add_argument("--persist-path", help="blocked when inside the source tree; reserved for external smoke")
    parser.add_argument("--rebuild-runs", type=int, default=1)
    args = parser.parse_args(argv)
    if args.max_docs <= 0:
        parser.error("--max-docs must be > 0")
    if args.query_limit <= 0:
        parser.error("--query-limit must be > 0")
    if args.result_limit <= 0:
        parser.error("--result-limit must be > 0")
    if args.rebuild_runs <= 0:
        parser.error("--rebuild-runs must be > 0")
    return args


def build_exception_report(args: argparse.Namespace, exc: Exception) -> dict[str, Any]:
    query_set = getattr(args, "query_set", "")
    return {
        "schema_version": SCHEMA_VERSION,
        "tool": "managed_milvus_shadow_probe",
        "status": "failed",
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "dependency_status": dependency_status(),
        "source_evidence": {
            "query_set_path": str(Path(query_set).expanduser()) if query_set else "",
        },
        "request_summary": {
            "row_count": 0,
            "query_count": 0,
            "result_limit": getattr(args, "result_limit", None),
            "metric_type": getattr(args, "metric_type", None),
        },
        "managed_milvus_probe": {"status": "not_run"},
        "rebuild_runs": [],
        "rebuild_parity": {
            "requested": getattr(args, "rebuild_runs", 1) > 1,
            "status": "not_run",
            "run_count": 0,
            "mismatches": [],
        },
        "fail_open_checks": {
            **SAFE_FAIL_OPEN,
            "dependency_failure_safe": False,
            "probe_error_safe": False,
            "invalid_input_safe": True,
            "error_type": type(exc).__name__,
        },
        "warnings": [],
        "failures": [f"invalid_input:{type(exc).__name__}: {exc}"],
        "non_goals": [
            "does_not_connect_to_chroma",
            "does_not_connect_to_mariadb",
            "does_not_connect_to_sqlite",
            "does_not_touch_archive_center_0_8",
            "does_not_enable_go_vector_store",
            "does_not_switch_live_retrieval",
            "does_not_enable_go_default_runtime",
        ],
    }


def main(argv: list[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    try:
        report, exit_code = build_report(args)
    except Exception as exc:
        report = build_exception_report(args, exc)
        exit_code = 1
    output = json.dumps(report, indent=2, ensure_ascii=True)
    if args.out:
        json_dump(Path(args.out).expanduser().resolve(), report)
    print(output)
    return exit_code


if __name__ == "__main__":
    raise SystemExit(main())
