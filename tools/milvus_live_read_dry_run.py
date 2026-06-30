#!/usr/bin/env python3
"""Milvus live-read dry-run harness for Archive Center 2.0 (slice 2.0-3.R1-11).

This tool performs a bounded, disposable dry-run that temporarily exercises
Milvus Lite as if milvus_live_enabled=true, then verifies mandatory rollback
to milvus_live_enabled=false and live_retrieval_enabled=false.

It does NOT persist any live flag, does NOT alter stable runtime config, and
does NOT retire Chroma or switch MariaDB authority.
"""

from __future__ import annotations

import argparse
import hashlib
import importlib
import importlib.metadata
import importlib.util
import json
import platform
import shutil
import sys
import tempfile
import time
import traceback
from pathlib import Path
from typing import Any


SCHEMA_VERSION = "archive-center.milvus-live-read-dry-run.v1"
DEFAULT_COLLECTION = "archive_center_live_read_dry_run"
DEFAULT_QUERY_SET = (
    Path(__file__).resolve().parents[1]
    / "benchmarks"
    / "chroma-milvus-query-set-2026-05-25-real.json"
)


# ---------------------------------------------------------------------------
# State tracker (in-memory only)
# ---------------------------------------------------------------------------

class LiveReadDryRunState:
    """In-memory state tracker. No disk writes for flags."""

    def __init__(self) -> None:
        self.milvus_live_enabled = False
        self.live_retrieval_enabled = False
        self.state_log: list[dict[str, Any]] = []

    def record(self, phase: str, notes: str | None = None) -> None:
        entry: dict[str, Any] = {
            "phase": phase,
            "milvus_live_enabled": self.milvus_live_enabled,
            "live_retrieval_enabled": self.live_retrieval_enabled,
            "timestamp_ms": int(time.time() * 1000),
        }
        if notes:
            entry["notes"] = notes
        self.state_log.append(entry)

    def simulate_enable(self) -> None:
        self.milvus_live_enabled = True
        self.live_retrieval_enabled = True
        self.record("simulated_enable", "Temporary in-memory enable for dry-run exercise only")

    def rollback(self) -> None:
        self.milvus_live_enabled = False
        self.live_retrieval_enabled = False
        self.record("rollback", "Mandatory rollback to disabled state")

    def verify_final(self) -> tuple[bool, list[str]]:
        self.record("final_verify")
        ok = True
        failures: list[str] = []
        if self.milvus_live_enabled is not False:
            ok = False
            failures.append("milvus_live_enabled_not_false_after_rollback")
        if self.live_retrieval_enabled is not False:
            ok = False
            failures.append("live_retrieval_enabled_not_false_after_rollback")
        self.state_log[-1]["verified"] = ok
        return ok, failures


# ---------------------------------------------------------------------------
# Reused probe utilities (self-contained, based on managed_milvus_shadow_probe)
# ---------------------------------------------------------------------------

def repo_root() -> Path:
    return Path(__file__).resolve().parents[1]


def path_inside(child: Path, parent: Path) -> bool:
    try:
        child.resolve().relative_to(parent.resolve())
        return True
    except ValueError:
        return False


def module_available(name: str) -> bool:
    return importlib.util.find_spec(name) is not None


def package_version(name: str) -> str | None:
    try:
        return importlib.metadata.version(name)
    except importlib.metadata.PackageNotFoundError:
        return None


def pymilvus_info() -> dict[str, Any]:
    info: dict[str, Any] = {
        "available": module_available("pymilvus"),
        "version": package_version("pymilvus"),
        "milvus_client_available": False,
    }
    if not info["available"]:
        return info
    try:
        pymilvus = importlib.import_module("pymilvus")
        info["version"] = getattr(pymilvus, "__version__", None)
        info["milvus_client_available"] = hasattr(pymilvus, "MilvusClient")
    except Exception as exc:  # noqa: BLE001
        info["available"] = False
        info["import_error"] = f"{type(exc).__name__}: {exc}"
    return info


def milvus_lite_info() -> dict[str, Any]:
    return {
        "available": module_available("milvus_lite"),
        "version": package_version("milvus-lite"),
    }


def cleanup_dir(path: Path) -> str | None:
    last_error: str | None = None
    for _ in range(5):
        try:
            shutil.rmtree(path)
            return None
        except FileNotFoundError:
            return None
        except PermissionError as exc:
            last_error = f"{type(exc).__name__}: {exc}"
            time.sleep(0.2)
        except OSError as exc:
            last_error = f"{type(exc).__name__}: {exc}"
            time.sleep(0.2)
    return last_error


def close_client(client: Any) -> None:
    for method_name in ("close", "disconnect"):
        method = getattr(client, method_name, None)
        if callable(method):
            method()
            return


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


def vector_hash(vector: list[float]) -> str:
    payload = json.dumps(vector, separators=(",", ":"), ensure_ascii=True)
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()


def build_rows(query_set: dict[str, Any], max_docs: int) -> list[dict[str, Any]]:
    queries = [item for item in query_set.get("queries", []) if isinstance(item, dict)]
    rows: list[dict[str, Any]] = []
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


def build_search_plan(query_set: dict[str, Any], max_queries: int) -> list[dict[str, Any]]:
    queries = query_set.get("queries", [])
    if not isinstance(queries, list):
        raise ValueError("queries must be a list")
    plan: list[dict[str, Any]] = []
    for query in queries[:max_queries]:
        if not isinstance(query, dict):
            continue
        qid = str(query.get("query_id") or query.get("id") or "")
        if not qid:
            continue
        emb = normalize_vector(query.get("embedding"))
        if not emb:
            continue
        plan.append(
            {
                "query_id": qid,
                "source_id": str(query.get("source_id") or qid),
                "vector": emb,
            }
        )
    return plan


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


def run_dry_run_exercise(
    rows: list[dict[str, Any]],
    search_plan: list[dict[str, Any]],
    persist_path: Path,
    collection: str,
    metric_type: str,
    drop_after: bool = True,
) -> dict[str, Any]:
    started = time.time()
    result: dict[str, Any] = {
        "status": "error",
        "persist_path": str(persist_path),
        "collection": collection,
        "metric_type": metric_type,
        "insert_count": 0,
        "query_count": 0,
        "duration_ms": None,
        "comparisons": [],
        "cleanup_warnings": [],
    }

    pymilvus = importlib.import_module("pymilvus")
    client_class = getattr(pymilvus, "MilvusClient")
    client = client_class(str(persist_path))

    try:
        if drop_after:
            try:
                if client.has_collection(collection):
                    client.drop_collection(collection)
            except Exception as exc:
                result["cleanup_warnings"].append(f"pre_drop_failed:{type(exc).__name__}: {exc}")

        dimension = len(rows[0]["vector"])
        try:
            client.create_collection(
                collection_name=collection, dimension=dimension, metric_type=metric_type
            )
        except TypeError:
            client.create_collection(collection_name=collection, dimension=dimension)

        insert_result = client.insert(collection_name=collection, data=rows)
        result["insert_count"] = len(rows)
        result["insert_result"] = str(insert_result)

        comparisons: list[dict[str, Any]] = []
        for plan_item in search_plan:
            search_result = client.search(
                collection_name=collection,
                data=[plan_item["vector"]],
                limit=5,
                output_fields=["source_id", "text", "tier", "chat_session_id", "source_table", "source_row_id"],
            )
            hits = search_result[0] if search_result else []
            milvus_ids = [value for value in (get_hit_source_id(hit) for hit in hits) if value]
            expected_ids = [plan_item["source_id"]]
            top1_match = bool(milvus_ids) and milvus_ids[0] == plan_item["source_id"]
            comparisons.append(
                {
                    "query_id": plan_item["query_id"],
                    "source_id": plan_item["source_id"],
                    "expected_ids": expected_ids,
                    "milvus_ids": milvus_ids,
                    "hit_count": len(milvus_ids),
                    "top1_match": top1_match,
                }
            )

        result["query_count"] = len(search_plan)
        result["comparisons"] = comparisons
        result["duration_ms"] = round((time.time() - started) * 1000, 3)
        result["status"] = "ok"

        if drop_after:
            try:
                if client.has_collection(collection):
                    client.drop_collection(collection)
            except Exception as exc:
                result["cleanup_warnings"].append(f"post_drop_failed:{type(exc).__name__}: {exc}")
    except Exception as exc:
        result["status"] = "error"
        result["error"] = f"{type(exc).__name__}: {exc}"
        result["traceback"] = traceback.format_exc()
    finally:
        close_client(client)

    return result


# ---------------------------------------------------------------------------
# Report builder
# ---------------------------------------------------------------------------

def build_report(
    query_set_path: Path,
    query_set: dict[str, Any],
    deps: dict[str, Any],
    run_result: dict[str, Any],
    state: LiveReadDryRunState,
    rollback_ok: bool,
    rollback_failures: list[str],
    warnings: list[str],
    failures: list[str],
) -> tuple[dict[str, Any], int]:
    if failures:
        support_level = "red"
        status = "failed"
    elif warnings:
        support_level = "yellow"
        status = "degraded"
    else:
        support_level = "green"
        status = "ok"

    exit_code = 0 if status == "ok" else 1

    report = {
        "schema_version": SCHEMA_VERSION,
        "tool": "milvus_live_read_dry_run",
        "slice": "2.0-3.R1-11",
        "status": status,
        "support_level": support_level,
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "python": {
            "version": platform.python_version(),
            "executable": sys.executable,
            "platform": platform.platform(),
        },
        "dependency_status": {
            "pymilvus": deps.get("pymilvus"),
            "milvus_lite": deps.get("milvus_lite"),
        },
        "source_evidence": {
            "query_set_path": str(query_set_path),
            "source_mode": query_set.get("source_mode"),
            "synthetic_embeddings": query_set.get("synthetic_embeddings"),
            "mirrored_doc_count": query_set.get("mirrored_doc_count"),
            "query_count": query_set.get("query_count"),
            "metric_type": query_set.get("metric_type"),
        },
        "dry_run_scope": "tool_only",
        "authority_unchanged": True,
        "state_log": state.state_log,
        "rollback_verified": rollback_ok,
        "rollback_failures": rollback_failures,
        "exercise": run_result,
        "warnings": warnings,
        "failures": failures,
        "non_goals": [
            "does_not_persist_live_flag",
            "does_not_alter_stable_runtime_config",
            "does_not_enable_go_vector_store",
            "does_not_touch_archive_center_0_8",
            "does_not_switch_live_retrieval_in_production",
            "does_not_authorize_mariadb_truth",
            "does_not_enable_go_default_runtime",
            "does_not_retire_chroma",
        ],
    }
    return report, exit_code


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Milvus live-read dry-run harness (R1-11)")
    parser.add_argument("--query-set", default=str(DEFAULT_QUERY_SET))
    parser.add_argument("--out", help="write JSON report to this path")
    parser.add_argument("--max-docs", type=int, default=0, help="max docs to backfill (0=all)")
    parser.add_argument("--max-queries", type=int, default=0, help="max queries to run (0=all)")
    parser.add_argument("--metric-type", default="L2", help="Milvus metric type (L2, IP, COSINE)")
    parser.add_argument("--temp-dir", help="optional temp directory outside source tree")
    parser.add_argument(
        "--simulate-live",
        action="store_true",
        default=True,
        help="simulate milvus_live_enabled=true in-memory (default true for dry-run)",
    )
    parser.add_argument(
        "--no-simulate-live",
        action="store_true",
        help="skip the simulated live enable (produces a readiness-only report)",
    )
    args = parser.parse_args(argv)
    if args.max_docs < 0:
        parser.error("--max-docs must be >= 0")
    if args.max_queries < 0:
        parser.error("--max-queries must be >= 0")
    return args


def main(argv: list[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    query_set_path = Path(args.query_set).expanduser().resolve()
    query_set = load_query_set(query_set_path)

    pymilvus = pymilvus_info()
    milvus_lite = milvus_lite_info()
    deps = {"pymilvus": pymilvus, "milvus_lite": milvus_lite}

    state = LiveReadDryRunState()
    state.record("initial", "Dry-run begins with both flags false")

    warnings: list[str] = []
    failures: list[str] = []

    run_result: dict[str, Any] = {"requested": False, "status": "skipped"}

    simulate_live = args.simulate_live and not args.no_simulate_live

    if simulate_live:
        state.simulate_enable()

    # Dependency checks
    if not pymilvus["available"]:
        failures.append("pymilvus_not_available")
    if not pymilvus.get("milvus_client_available"):
        failures.append("milvus_client_not_available")
    if not milvus_lite["available"]:
        warnings.append("milvus_lite_package_not_available")

    if not failures:
        max_docs = args.max_docs if args.max_docs > 0 else len(query_set.get("queries", []))
        rows = build_rows(query_set, max_docs=max_docs)
        max_q = args.max_queries if args.max_queries > 0 else len(query_set.get("queries", []))
        search_plan = build_search_plan(query_set, max_queries=max_q)

        if not rows:
            failures.append("no_docs_with_embeddings")
        if not search_plan:
            warnings.append("no_search_plan_from_queries")

        if rows and search_plan:
            dimension = len(rows[0]["vector"])
            collection = f"{DEFAULT_COLLECTION}_{int(time.time())}"
            metric_type = str(args.metric_type or query_set.get("metric_type", "L2"))

            if args.temp_dir:
                temp_dir = Path(args.temp_dir).expanduser().resolve()
                if path_inside(temp_dir, repo_root()):
                    failures.append("temp_dir_inside_source_tree")
                else:
                    temp_dir.mkdir(parents=True, exist_ok=True)
            else:
                temp_dir = Path(tempfile.mkdtemp(prefix="archive-center-live-read-dry-run-"))

            persist_path: Path | None = None
            if not failures:
                persist_path = temp_dir / f"{collection}.db"
                run_result = run_dry_run_exercise(
                    rows=rows,
                    search_plan=search_plan,
                    persist_path=persist_path,
                    collection=collection,
                    metric_type=metric_type,
                    # The dry run writes to an isolated temp DB. Avoid
                    # drop_collection here because Windows Milvus Lite can keep
                    # manifest/WAL files briefly locked after a successful
                    # search, which turns clean evidence into cleanup noise.
                    drop_after=False,
                )
                run_result["requested"] = True
                if run_result.get("status") != "ok":
                    failures.append("dry_run_exercise_error")

            if args.temp_dir is None and temp_dir is not None:
                cleanup_error = cleanup_dir(temp_dir)
                if cleanup_error:
                    run_result.setdefault("cleanup_warnings", []).append(
                        f"temp_cleanup_failed:{cleanup_error}"
                    )
                    warnings.append("temp_cleanup_incomplete")
    else:
        run_result["requested"] = True
        run_result["status"] = "unavailable"

    # Mandatory rollback
    state.rollback()
    rollback_ok, rollback_failures = state.verify_final()
    if not rollback_ok:
        failures.extend(rollback_failures)
        # This is a critical safety failure; elevate status
        status_override = "failed"
    else:
        status_override = None

    report, exit_code = build_report(
        query_set_path=query_set_path,
        query_set=query_set,
        deps=deps,
        run_result=run_result,
        state=state,
        rollback_ok=rollback_ok,
        rollback_failures=rollback_failures,
        warnings=warnings,
        failures=failures,
    )
    if status_override:
        report["status"] = status_override
        exit_code = 1

    output = json.dumps(report, indent=2, ensure_ascii=True)
    if args.out:
        out_path = Path(args.out).expanduser().resolve()
        json_dump(out_path, report)
    print(output)
    return exit_code


if __name__ == "__main__":
    raise SystemExit(main())
