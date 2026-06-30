#!/usr/bin/env python3
"""Managed Milvus Lite shadow adapter/probe for Archive Center 2.0.

This tool reads a Chroma/Milvus query-set JSON artifact, backfills the
extracted documents into a temporary managed Milvus Lite collection,
performs vector search with the query embeddings, and writes JSON evidence.

It is R1-safe: it does not enable the Go vector store, does not touch the
0.8 runtime, and does not create a persist dir inside the source tree.
If pymilvus/milvus-lite are unavailable, it produces a graceful red/yellow
JSON report instead of failing hard.
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


SCHEMA_VERSION = "archive-center.milvus-lite-managed-probe.v1"
DEFAULT_COLLECTION = "archive_center_lite_probe"
DEFAULT_QUERY_SET = Path(__file__).resolve().parents[1] / "benchmarks" / "chroma-milvus-query-set-2026-05-25-real.json"


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


def build_docs(query_set: dict[str, Any]) -> list[dict[str, Any]]:
    queries = query_set.get("queries", [])
    if not isinstance(queries, list):
        raise ValueError("queries must be a list")
    docs: list[dict[str, Any]] = []
    seen_ids: set[str] = set()
    for query in queries:
        if not isinstance(query, dict):
            continue
        source_id = str(query.get("source_id") or query.get("id") or "")
        if not source_id:
            continue
        if source_id in seen_ids:
            continue
        seen_ids.add(source_id)
        emb = normalize_vector(query.get("embedding"))
        if not emb:
            continue
        docs.append({
            "id": source_id,
            "source_id": source_id,
            "embedding": emb,
            "tier": str(query.get("tier") or ""),
            "chat_session_id": str(query.get("chat_session_id") or ""),
            "source_table": str(query.get("source_table") or ""),
            "source_row_id": str(query.get("source_row_id") or ""),
            "document_text": str(query.get("document_excerpt") or source_id),
        })
    return docs


def build_search_plan(query_set: dict[str, Any], *, max_queries: int) -> list[dict[str, Any]]:
    queries = query_set.get("queries", [])
    if not isinstance(queries, list):
        raise ValueError("queries must be a list")
    plan: list[dict[str, Any]] = []
    for index, query in enumerate(queries):
        if index >= max_queries:
            break
        if not isinstance(query, dict):
            continue
        emb = normalize_vector(query.get("embedding"))
        if not emb:
            continue
        plan.append({
            "query_id": str(query.get("query_id") or f"q{index + 1}"),
            "source_id": str(query.get("source_id") or query.get("id") or ""),
            "vector": emb,
            "chat_session_id": str(query.get("chat_session_id") or ""),
            "limit": int(query_set.get("result_limit", 5)),
        })
    return plan


def get_hit_source_id(hit: dict[str, Any]) -> str | None:
    entity = hit.get("entity", {})
    if isinstance(entity, dict):
        hit_id = entity.get("source_id") or entity.get("id") or hit.get("source_id") or hit.get("id")
    else:
        hit_id = hit.get("source_id") or hit.get("id")
    return str(hit_id) if hit_id is not None else None


def run_temp_milvus(
    docs: list[dict[str, Any]],
    search_plan: list[dict[str, Any]],
    persist_path: Path,
    collection: str,
    metric_type: str,
    drop_after: bool,
) -> dict[str, Any]:
    started = time.time()
    result: dict[str, Any] = {
        "status": "error",
        "persist_path": str(persist_path),
        "collection": collection,
        "dimension": len(docs[0]["embedding"]) if docs else 0,
        "metric_type": metric_type,
        "insert_count": 0,
        "search_count": 0,
        "duration_ms": None,
        "queries": [],
        "errors": [],
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

        dimension = result["dimension"]
        if dimension <= 0:
            raise ValueError("cannot infer dimension from empty docs")

        client.create_collection(
            collection_name=collection,
            dimension=dimension,
            metric_type=metric_type,
        )

        rows = [
            {
                "id": index,
                "source_id": doc["source_id"],
                "vector": doc["embedding"],
                "tier": doc["tier"],
                "chat_session_id": doc["chat_session_id"],
                "source_table": doc["source_table"],
                "source_row_id": doc["source_row_id"],
                "document_text": doc["document_text"],
            }
            for index, doc in enumerate(docs, start=1)
        ]
        insert_result = client.insert(collection_name=collection, data=rows)
        result["insert_count"] = len(rows)
        result["insert_result"] = str(insert_result)

        for plan_item in search_plan:
            sid = plan_item["chat_session_id"]
            filter_expr = f'chat_session_id == "{sid}"' if sid else ""
            search_kwargs: dict[str, Any] = {
                "collection_name": collection,
                "data": [plan_item["vector"]],
                "limit": plan_item["limit"],
                "output_fields": [
                    "id",
                    "source_id",
                    "tier",
                    "chat_session_id",
                    "source_table",
                    "source_row_id",
                    "document_text",
                ],
            }
            if filter_expr:
                search_kwargs["filter"] = filter_expr

            search_result = client.search(**search_kwargs)
            result["search_count"] += 1

            ids: list[str] = []
            distances: list[float] = []
            if search_result and isinstance(search_result, list) and len(search_result) > 0:
                first_hits = search_result[0]
                if isinstance(first_hits, list):
                    for hit in first_hits:
                        if isinstance(hit, dict):
                            hit_id = get_hit_source_id(hit)
                            if hit_id is not None:
                                ids.append(hit_id)
                            distances.append(float(hit.get("distance", 0.0)))

            self_found = plan_item["source_id"] in ids if plan_item["source_id"] else None
            result["queries"].append({
                "query_id": plan_item["query_id"],
                "source_id": plan_item["source_id"],
                "milvus_ids": ids,
                "raw_hit_count": len(ids),
                "self_found": self_found,
                "distances": [round(d, 6) for d in distances],
            })

        result["status"] = "ok"
        result["duration_ms"] = round((time.time() - started) * 1000.0, 3)
    except Exception as exc:
        result["errors"].append(f"{type(exc).__name__}: {exc}")
        result["status"] = "error"
    finally:
        if drop_after:
            try:
                if client.has_collection(collection):
                    client.drop_collection(collection)
            except Exception as exc:
                result["cleanup_warnings"].append(f"drop_failed:{type(exc).__name__}: {exc}")
        close_client(client)

    return result


def build_report(
    *,
    query_set_path: Path,
    query_set: dict[str, Any],
    deps: dict[str, Any],
    run_result: dict[str, Any],
    warnings: list[str],
    failures: list[str],
) -> tuple[dict[str, Any], int]:
    status = "ok"
    exit_code = 0
    support_level = "green"

    if failures:
        status = "fail"
        support_level = "red"
        exit_code = 1
    elif warnings:
        status = "degraded"
        support_level = "yellow"

    if run_result.get("status") != "ok":
        if run_result.get("requested"):
            status = "fail"
            support_level = "red"
            exit_code = 1
        else:
            if support_level == "green":
                support_level = "yellow"
                status = "unavailable"

    report = {
        "schema_version": SCHEMA_VERSION,
        "tool": "milvus_lite_managed_probe",
        "status": status,
        "support_level": support_level,
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "query_set_path": str(query_set_path),
        "source_evidence": {
            "source_mode": query_set.get("source_mode"),
            "synthetic_embeddings": query_set.get("synthetic_embeddings"),
            "mirrored_doc_count": query_set.get("mirrored_doc_count"),
            "query_count": query_set.get("query_count"),
            "metric_type": query_set.get("metric_type"),
            "embedding_dim": query_set.get("queries", [{}])[0].get("embedding_dim") if query_set.get("queries") else None,
        },
        "environment": {
            "python_version": platform.python_version(),
            "python_executable": sys.executable,
            "platform": platform.platform(),
            "pymilvus": deps.get("pymilvus"),
            "milvus_lite": deps.get("milvus_lite"),
        },
        "run": run_result,
        "warnings": warnings,
        "failures": failures,
        "non_goals": [
            "does_not_enable_go_vector_store",
            "does_not_touch_archive_center_0_8",
            "does_not_create_source_tree_persist_dir",
            "does_not_switch_live_retrieval",
            "does_not_authorize_mariadb_truth",
            "does_not_enable_go_default_runtime",
        ],
    }
    return report, exit_code


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Managed Milvus Lite shadow adapter/probe")
    parser.add_argument("--query-set", default=str(DEFAULT_QUERY_SET))
    parser.add_argument("--out", help="write JSON report to this path")
    parser.add_argument("--max-docs", type=int, default=0, help="max docs to backfill (0=all)")
    parser.add_argument("--max-queries", type=int, default=0, help="max queries to run (0=all)")
    parser.add_argument("--metric-type", default="L2", help="Milvus metric type (L2, IP, COSINE)")
    parser.add_argument("--temp-dir", help="optional temp directory outside source tree")
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

    warnings: list[str] = []
    failures: list[str] = []

    if not pymilvus["available"]:
        failures.append("pymilvus_not_available")
    if not pymilvus.get("milvus_client_available"):
        failures.append("milvus_client_not_available")
    if not milvus_lite["available"]:
        warnings.append("milvus_lite_package_not_available")

    run_result: dict[str, Any] = {"requested": False, "status": "unavailable"}

    if not failures:
        docs = build_docs(query_set)
        if args.max_docs > 0:
            docs = docs[:args.max_docs]
        search_plan = build_search_plan(query_set, max_queries=args.max_queries if args.max_queries > 0 else len(query_set.get("queries", [])))

        if not docs:
            failures.append("no_docs_with_embeddings")
        if not search_plan:
            warnings.append("no_search_plan_from_queries")

        if docs and search_plan:
            dimension = len(docs[0]["embedding"])
            collection = f"{DEFAULT_COLLECTION}_{int(time.time())}"
            metric_type = str(args.metric_type or query_set.get("metric_type", "L2"))

            if args.temp_dir:
                temp_dir = Path(args.temp_dir).expanduser().resolve()
                if path_inside(temp_dir, repo_root()):
                    failures.append("temp_dir_inside_source_tree")
                else:
                    temp_dir.mkdir(parents=True, exist_ok=True)
            else:
                temp_dir = Path(tempfile.mkdtemp(prefix="archive-center-managed-probe-"))

            persist_path: Path | None = None
            if not failures:
                persist_path = temp_dir / f"{collection}.db"
                run_result = run_temp_milvus(
                    docs=docs,
                    search_plan=search_plan,
                    persist_path=persist_path,
                    collection=collection,
                    metric_type=metric_type,
                    # The probe uses a unique temp DB per run; dropping the
                    # collection can emit noisy manifest rename errors on
                    # Windows Milvus Lite even after successful searches.
                    drop_after=False,
                )
                run_result["requested"] = True
                if run_result.get("status") != "ok":
                    failures.append("milvus_run_error")

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

    report, exit_code = build_report(
        query_set_path=query_set_path,
        query_set=query_set,
        deps=deps,
        run_result=run_result,
        warnings=warnings,
        failures=failures,
    )

    output = json.dumps(report, indent=2, ensure_ascii=True)
    if args.out:
        out_path = Path(args.out).expanduser().resolve()
        json_dump(out_path, report)
    print(output)
    return exit_code


if __name__ == "__main__":
    raise SystemExit(main())
