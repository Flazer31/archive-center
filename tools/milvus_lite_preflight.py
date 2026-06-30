#!/usr/bin/env python3
"""Milvus Lite preflight and optional temp smoke evidence for Archive Center 2.0.

This tool is intentionally diagnostic-only by default. It does not enable the
Go vector store, does not touch the 0.8 runtime, and does not create vector
persist files in the source tree.
"""

from __future__ import annotations

import argparse
import importlib
import importlib.metadata
import importlib.util
import json
import platform
import shutil
import sys
import tempfile
import time
from pathlib import Path
from typing import Any


SCHEMA_VERSION = "archive-center.milvus-lite-preflight.v1"
DEFAULT_COLLECTION = "archive_center_lite_preflight"
DEFAULT_DIMENSION = 4


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
    except Exception as exc:  # pragma: no cover - depends on local package state.
        info["available"] = False
        info["import_error"] = f"{type(exc).__name__}: {exc}"
    return info


def milvus_lite_info() -> dict[str, Any]:
    return {
        "available": module_available("milvus_lite"),
        "version": package_version("milvus-lite"),
    }


def vector_rows(dimension: int) -> list[dict[str, Any]]:
    first = [0.1, 0.2, 0.3, 0.4][:dimension]
    second = [0.4, 0.3, 0.2, 0.1][:dimension]
    if dimension != DEFAULT_DIMENSION:
        first = [float(i + 1) / 10.0 for i in range(dimension)]
        second = list(reversed(first))
    return [
        {
            "id": 1,
            "vector": first,
            "text": "archive center milvus lite preflight alpha",
            "chat_session_id": "preflight-session",
            "source_table": "chat_logs",
            "source_row_id": "1",
        },
        {
            "id": 2,
            "vector": second,
            "text": "archive center milvus lite preflight beta",
            "chat_session_id": "preflight-session",
            "source_table": "memories",
            "source_row_id": "2",
        },
    ]


def close_client(client: Any) -> None:
    for method_name in ("close", "disconnect"):
        method = getattr(client, method_name, None)
        if callable(method):
            method()
            return


def cleanup_dir(path: Path) -> str | None:
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


def run_smoke(persist_path: Path, collection: str, dimension: int, drop_after: bool) -> dict[str, Any]:
    started = time.time()
    result: dict[str, Any] = {
        "requested": True,
        "status": "error",
        "persist_path": str(persist_path),
        "collection": collection,
        "dimension": dimension,
        "insert_count": 0,
        "search_count": 0,
        "duration_ms": None,
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

        client.create_collection(collection_name=collection, dimension=dimension)
        rows = vector_rows(dimension)
        insert_result = client.insert(collection_name=collection, data=rows)
        result["insert_count"] = len(rows)
        result["insert_result"] = str(insert_result)

        search_result = client.search(
            collection_name=collection,
            data=[rows[0]["vector"]],
            limit=1,
            output_fields=["text", "chat_session_id", "source_table", "source_row_id"],
        )
        if search_result:
            result["search_count"] = len(search_result[0])
        result["status"] = "ok"
        return result
    finally:
        if drop_after:
            try:
                client.drop_collection(collection)
            except Exception as exc:
                result["cleanup_warnings"].append(f"drop_failed:{type(exc).__name__}: {exc}")
        try:
            close_client(client)
        except Exception as exc:
            result["cleanup_warnings"].append(f"close_failed:{type(exc).__name__}: {exc}")
        result["duration_ms"] = round((time.time() - started) * 1000.0, 3)


def build_report(args: argparse.Namespace) -> tuple[dict[str, Any], int]:
    warnings: list[str] = []
    failures: list[str] = []
    pymilvus = pymilvus_info()
    milvus_lite = milvus_lite_info()
    smoke: dict[str, Any] = {"requested": bool(args.smoke), "status": "not_run"}
    exit_code = 0

    if not pymilvus["available"]:
        warnings.append("pymilvus_not_available")
    elif not pymilvus["milvus_client_available"]:
        warnings.append("milvus_client_not_available")
    if not milvus_lite["available"]:
        warnings.append("milvus_lite_package_not_available")

    persist_path: Path | None = None
    temp_dir_path: Path | None = None
    if args.smoke:
        if (
            not pymilvus["available"]
            or not pymilvus["milvus_client_available"]
            or not milvus_lite["available"]
        ):
            smoke["status"] = "unavailable"
            failures.append("milvus_lite_smoke_unavailable")
            exit_code = 1
        else:
            if args.persist_path:
                persist_path = Path(args.persist_path).expanduser().resolve()
                if path_inside(persist_path, repo_root()):
                    failures.append("persist_path_inside_source_tree")
                    smoke["status"] = "blocked"
                    exit_code = 1
                else:
                    persist_path.parent.mkdir(parents=True, exist_ok=True)
            else:
                temp_dir_path = Path(tempfile.mkdtemp(prefix="archive-center-milvus-lite-"))
                persist_path = temp_dir_path / "milvus_lite_preflight.db"

            if persist_path is not None and not failures:
                try:
                    smoke = run_smoke(
                        persist_path,
                        args.collection,
                        args.dimension,
                        # This preflight writes to an isolated DB path. Avoid
                        # post-smoke drop_collection because Windows Milvus
                        # Lite can keep manifest files locked after a
                        # successful smoke, which creates misleading cleanup
                        # warnings while the insert/search itself is green.
                        drop_after=False,
                    )
                    if smoke.get("status") != "ok":
                        exit_code = 1
                except Exception as exc:  # pragma: no cover - depends on local package state.
                    smoke = {
                        "requested": True,
                        "status": "error",
                        "persist_path": str(persist_path),
                        "collection": args.collection,
                        "dimension": args.dimension,
                        "error": f"{type(exc).__name__}: {exc}",
                    }
                    failures.append("milvus_lite_smoke_error")
                    exit_code = 1
                finally:
                    if temp_dir_path is not None:
                        cleanup_error = cleanup_dir(temp_dir_path)
                        if cleanup_error:
                            smoke.setdefault("cleanup_warnings", []).append(
                                f"temp_cleanup_failed:{cleanup_error}"
                            )
                            warnings.append("milvus_lite_temp_cleanup_incomplete")

    if failures:
        support_level = "red"
        status = "failed"
    elif warnings:
        support_level = "yellow"
        status = "degraded"
    else:
        support_level = "green" if args.smoke else "yellow"
        status = "ok" if args.smoke else "ready_for_smoke"

    report = {
        "schema_version": SCHEMA_VERSION,
        "tool": "milvus_lite_preflight",
        "mode": "smoke" if args.smoke else "preflight",
        "status": status,
        "support_level": support_level,
        "python": {
            "version": platform.python_version(),
            "executable": sys.executable,
            "platform": platform.platform(),
        },
        "pymilvus": pymilvus,
        "milvus_lite": milvus_lite,
        "smoke": smoke,
        "warnings": warnings,
        "failures": failures,
        "non_goals": [
            "does_not_enable_go_vector_store",
            "does_not_touch_archive_center_0_8",
            "does_not_create_source_tree_persist_dir",
            "does_not_switch_live_retrieval",
        ],
    }
    return report, exit_code


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Milvus Lite preflight JSON reporter")
    parser.add_argument("--smoke", action="store_true", help="run temp insert/search/drop smoke")
    parser.add_argument("--out", help="write JSON report to this path")
    parser.add_argument("--persist-path", help="optional smoke DB path outside the source tree")
    parser.add_argument("--collection", default=DEFAULT_COLLECTION)
    parser.add_argument("--dimension", type=int, default=DEFAULT_DIMENSION)
    args = parser.parse_args(argv)
    if args.dimension <= 0:
        parser.error("--dimension must be positive")
    return args


def main(argv: list[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    report, exit_code = build_report(args)
    payload = json.dumps(report, indent=2, ensure_ascii=True)
    if args.out:
        out_path = Path(args.out).expanduser().resolve()
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(payload + "\n", encoding="utf-8")
    print(payload)
    return exit_code


if __name__ == "__main__":
    raise SystemExit(main())
