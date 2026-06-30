#!/usr/bin/env python3
"""Exercise the Go HTTP vector route against a disposable Milvus Lite endpoint.

This is bounded R1/R2-adjacent evidence only. It starts a managed Milvus Lite
server outside the source tree, prepares the Archive Center collection with the
Go SDK smoke tool, starts the Go HTTP server with AC_MILVUS_SDK_ENABLED=true,
and verifies that /milvus-shadow/bounded-live-read-drill returns Milvus-backed
hits for the real 0.8 memory embedding query-set.

It does not enable product live retrieval, does not retire Chroma, and does not
write to the 0.8 worktree.
"""

from __future__ import annotations

import argparse
import json
import os
import shutil
import socket
import subprocess
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any


SCHEMA_VERSION = "archive-center.milvus-http-live-read-probe.v1"
DEFAULT_RUNTIME_REL = Path(".runtime-cache") / "temp" / "archive-center-2.0" / "milvus-lite-runtime"
DEFAULT_DATA_REL = Path(".runtime-cache") / "temp" / "archive-center-2.0" / "http-live-read-probe"


def repo_root() -> Path:
    return Path(__file__).resolve().parents[1]


def workspace_root() -> Path:
    return repo_root().parent


def path_inside(path: Path, parent: Path) -> bool:
    try:
        path.resolve().relative_to(parent.resolve())
        return True
    except ValueError:
        return False


def default_runtime_python() -> Path:
    runtime_dir = workspace_root() / DEFAULT_RUNTIME_REL
    if os.name == "nt":
        return runtime_dir / "Scripts" / "python.exe"
    return runtime_dir / "bin" / "python"


def find_free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


def wait_for_tcp(host: str, port: int, timeout_s: float) -> bool:
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        try:
            with socket.create_connection((host, port), timeout=0.5):
                return True
        except OSError:
            time.sleep(0.2)
    return False


def http_json(method: str, url: str, payload: dict[str, Any] | None = None, timeout_s: float = 5.0) -> tuple[int, dict[str, Any]]:
    body = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        body = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout_s) as resp:
            data = resp.read().decode("utf-8")
            return resp.status, json.loads(data) if data else {}
    except urllib.error.HTTPError as exc:
        data = exc.read().decode("utf-8")
        try:
            payload = json.loads(data) if data else {}
        except json.JSONDecodeError:
            payload = {"raw": data}
        return exc.code, payload


def wait_for_http(url: str, timeout_s: float) -> bool:
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        try:
            status, _ = http_json("GET", url, timeout_s=1.0)
            if status == 200:
                return True
        except OSError:
            pass
        time.sleep(0.3)
    return False


def cleanup_dir(path: Path) -> str | None:
    for _ in range(8):
        try:
            shutil.rmtree(path)
            return None
        except FileNotFoundError:
            return None
        except OSError as exc:
            last = f"{type(exc).__name__}: {exc}"
            time.sleep(0.25)
    return last


def truncate(text: str, limit: int = 4000) -> str:
    if len(text) <= limit:
        return text
    return text[:limit] + "...<truncated>"


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


def query_docs(query_set: dict[str, Any], limit: int) -> list[dict[str, Any]]:
    docs: list[dict[str, Any]] = []
    seen: set[str] = set()
    for query in query_set.get("queries", []):
        if not isinstance(query, dict):
            continue
        source_id = str(query.get("source_id") or query.get("id") or "")
        if not source_id or source_id in seen:
            continue
        vector = normalize_vector(query.get("embedding"))
        if not vector:
            continue
        seen.add(source_id)
        docs.append({
            "id": source_id,
            "embedding": vector,
            "tier": str(query.get("tier") or "memory"),
            "source_table": str(query.get("source_table") or "memories"),
            "source_row_id": str(query.get("source_row_id") or source_id.split(":")[-1]),
            "schema_version": str(query.get("schema_version") or "q1a.v1"),
            "document_text": str(query.get("document_excerpt") or source_id),
        })
        if len(docs) >= limit:
            break
    return docs


def first_query(query_set: dict[str, Any]) -> dict[str, Any]:
    query = query_set["queries"][0]
    if not isinstance(query, dict):
        raise ValueError("first query must be an object")
    vector = normalize_vector(query.get("embedding"))
    if not vector:
        raise ValueError("first query has no usable embedding")
    return query


def stop_process(proc: subprocess.Popen[str], report: dict[str, Any], key: str) -> None:
    if proc.poll() is None:
        proc.terminate()
    try:
        stdout, stderr = proc.communicate(timeout=5)
    except subprocess.TimeoutExpired:
        if os.name == "nt":
            subprocess.run(
                ["taskkill", "/PID", str(proc.pid), "/T", "/F"],
                text=True,
                capture_output=True,
                timeout=10,
            )
        else:
            proc.kill()
        try:
            stdout, stderr = proc.communicate(timeout=10)
        except subprocess.TimeoutExpired:
            stdout, stderr = "", "process tree did not exit after forced termination"
            report.setdefault("warnings", []).append(f"{key}_forced_termination_timeout")
    report[key]["returncode"] = proc.returncode
    report[key]["stdout"] = truncate(stdout or "")
    report[key]["stderr"] = truncate(stderr or "")
    report[key]["stopped_by_probe"] = True
    if proc.returncode not in (0, None):
        report[key]["returncode_note"] = "non-zero is acceptable after terminating a disposable local server"


def run_probe(args: argparse.Namespace) -> tuple[dict[str, Any], int]:
    root = repo_root()
    go_service = root / "go-service"
    runtime_python = Path(args.runtime_python).expanduser().resolve()
    data_root = Path(args.data_root).expanduser().resolve()
    go_cache = Path(args.go_cache).expanduser().resolve()
    query_set_path = Path(args.query_set).expanduser().resolve()
    product_read_proof = bool(getattr(args, "product_read_proof", False))
    report: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "tool": "milvus_http_live_read_probe",
        "status": "failed",
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "runtime_python": str(runtime_python),
        "data_root": str(data_root),
        "go_cache": str(go_cache),
        "query_set": str(query_set_path),
        "milvus_endpoint": None,
        "go_http_base_url": None,
        "milvus_server": {},
        "go_server_build": {},
        "go_server": {},
        "collection_prepare": {},
        "http_backfill_compare": {},
        "http_live_read_drill": {},
        "http_prepare_turn_recall": {},
        "rollback_proof": {},
        "chroma_decommission_proof": {},
        "summary": {},
        "warnings": [],
        "failures": [],
        "safety": {
            "product_read_proof_requested": product_read_proof,
            "persisted_milvus_live_enabled": False,
            "persisted_live_retrieval_enabled": False,
            "bounded_shadow_route_only": True,
            "does_not_touch_archive_center_0_8": True,
            "does_not_delete_chroma": True,
            "does_not_enable_go_default_runtime": True,
        },
    }

    if path_inside(data_root, root):
        report["failures"].append("data_root_inside_source_tree")
    if not runtime_python.exists():
        report["failures"].append("runtime_python_missing")
    if not query_set_path.exists():
        report["failures"].append("query_set_missing")
    if report["failures"]:
        return report, 1

    query_set = load_query_set(query_set_path)
    query = first_query(query_set)
    docs = query_docs(query_set, int(args.doc_limit))
    if not docs:
        report["failures"].append("query_set_has_no_docs")
        return report, 1

    data_root.mkdir(parents=True, exist_ok=True)
    go_cache.mkdir(parents=True, exist_ok=True)
    milvus_port = int(args.milvus_port) if args.milvus_port else find_free_port()
    go_port = int(args.go_port) if args.go_port else find_free_port()
    milvus_endpoint = f"http://127.0.0.1:{milvus_port}"
    go_base_url = f"http://127.0.0.1:{go_port}"
    report["milvus_endpoint"] = milvus_endpoint
    report["go_http_base_url"] = go_base_url
    milvus_data_dir = data_root / f"milvus-server-{milvus_port}.db"
    prepare_report_path = data_root / f"collection-prepare-{milvus_port}.json"

    milvus_cmd = [
        str(runtime_python),
        "-m",
        "milvus_lite",
        "server",
        "--data-dir",
        str(milvus_data_dir),
        "--host",
        "127.0.0.1",
        "--port",
        str(milvus_port),
    ]
    milvus_proc = subprocess.Popen(milvus_cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    report["milvus_server"] = {"command": milvus_cmd, "data_dir": str(milvus_data_dir), "port": milvus_port}
    go_proc: subprocess.Popen[str] | None = None

    try:
        if not wait_for_tcp("127.0.0.1", milvus_port, float(args.wait_timeout_s)):
            report["failures"].append("milvus_endpoint_not_ready")
            return report, 1

        env = os.environ.copy()
        env["GOCACHE"] = str(go_cache)
        prepare_cmd = [
            "go",
            "run",
            "-buildvcs=false",
            "./cmd/milvus-sdk-smoke",
            "-execute",
            "-ensure-collection",
            "-endpoint",
            milvus_endpoint,
            "-query-set",
            str(query_set_path),
            "-out",
            str(prepare_report_path),
        ]
        prepare = subprocess.run(
            prepare_cmd,
            cwd=go_service,
            env=env,
            text=True,
            capture_output=True,
            timeout=float(args.go_timeout_s),
        )
        prepare_payload = json.loads(prepare_report_path.read_text(encoding="utf-8")) if prepare_report_path.exists() else {}
        report["collection_prepare"] = {
            "command": prepare_cmd,
            "returncode": prepare.returncode,
            "stdout": truncate(prepare.stdout),
            "stderr": truncate(prepare.stderr),
            "report_path": str(prepare_report_path),
            "report": prepare_payload,
        }
        if prepare.returncode != 0 or prepare_payload.get("status") != "ok":
            report["failures"].append("collection_prepare_failed")
            return report, 1

        server_bin = data_root / ("archive-center-go-probe.exe" if os.name == "nt" else "archive-center-go-probe")
        build_cmd = [
            "go",
            "build",
            "-buildvcs=false",
            "-o",
            str(server_bin),
            "./cmd/archive-center-go",
        ]
        build = subprocess.run(
            build_cmd,
            cwd=go_service,
            env=env,
            text=True,
            capture_output=True,
            timeout=float(args.go_timeout_s),
        )
        report["go_server_build"] = {
            "command": build_cmd,
            "returncode": build.returncode,
            "stdout": truncate(build.stdout),
            "stderr": truncate(build.stderr),
            "binary": str(server_bin),
        }
        if build.returncode != 0:
            report["failures"].append("go_server_build_failed")
            return report, 1

        server_env = env.copy()
        server_env["AC_BIND_ADDR"] = f"127.0.0.1:{go_port}"
        server_env["AC_MODE"] = "shadow"
        server_env["AC_STORE_MODE"] = "noop"
        server_env["AC_MILVUS_SDK_ENABLED"] = "true"
        server_env["AC_MILVUS_ENDPOINT"] = milvus_endpoint
        server_env["AC_MILVUS_RECALL_READ_ENABLED"] = "true"
        if product_read_proof:
            server_env["AC_MILVUS_PRODUCT_READ_ENABLED"] = "true"
            server_env["AC_CHROMA_SHADOW_PERSIST_DIR"] = str(data_root / "not-used-chroma-shadow")
        go_cmd = [str(server_bin)]
        go_proc = subprocess.Popen(go_cmd, cwd=go_service, env=server_env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        report["go_server"] = {
            "command": go_cmd,
            "bind": server_env["AC_BIND_ADDR"],
            "milvus_sdk_enabled": True,
            "milvus_endpoint_configured": True,
            "milvus_recall_read_enabled": True,
            "milvus_product_read_enabled": product_read_proof,
            "chroma_shadow_persist_dir": server_env.get("AC_CHROMA_SHADOW_PERSIST_DIR"),
        }
        if not wait_for_http(f"{go_base_url}/health", float(args.wait_timeout_s)):
            report["failures"].append("go_http_server_not_ready")
            return report, 1

        sid = str(query.get("chat_session_id") or docs[0].get("chat_session_id") or "")
        if not sid:
            report["failures"].append("query_set_missing_chat_session_id")
            return report, 1
        source_id = str(query.get("source_id") or docs[0]["id"])
        filter_expr = f'chat_session_id == "{sid}"'
        req_limit = int(query_set.get("result_limit", args.result_limit) or args.result_limit)
        query_vector = normalize_vector(query.get("embedding"))
        backfill_request = {
            "chat_session_id": sid,
            "dry_run": True,
            "allow_shadow_boundary": True,
            "docs": docs,
            "query_vector": query_vector,
            "limit": req_limit,
            "filter": filter_expr,
        }
        backfill_status, backfill_payload = http_json(
            "POST",
            f"{go_base_url}/milvus-shadow/backfill-compare",
            backfill_request,
            timeout_s=float(args.http_timeout_s),
        )
        report["http_backfill_compare"] = {
            "status_code": backfill_status,
            "request_doc_count": len(docs),
            "response": backfill_payload,
        }

        drill_request = {
            "chat_session_id": sid,
            "query_vector": query_vector,
            "limit": req_limit,
            "filter": filter_expr,
        }
        drill_status, drill_payload = http_json(
            "POST",
            f"{go_base_url}/milvus-shadow/bounded-live-read-drill",
            drill_request,
            timeout_s=float(args.http_timeout_s),
        )
        report["http_live_read_drill"] = {
            "status_code": drill_status,
            "request": {
                "chat_session_id": sid,
                "query_id": str(query.get("query_id") or ""),
                "source_id": source_id,
                "query_vector_dim": len(query_vector),
                "limit": req_limit,
                "filter": filter_expr,
            },
            "response": drill_payload,
        }

        prepare_turn_request = {
            "chat_session_id": sid,
            "turn_index": 1,
            "raw_user_input": "Milvus recall read drill",
            "client_meta": {
                "milvus_query_vector": query_vector,
                "milvus_filter": filter_expr,
            },
            "settings": {
                "top_k": req_limit,
                "injection_enabled": False,
                "input_context_enabled": False,
            },
        }
        prepare_status, prepare_payload = http_json(
            "POST",
            f"{go_base_url}/prepare-turn",
            prepare_turn_request,
            timeout_s=float(args.http_timeout_s),
        )
        prepare_recall = prepare_payload.get("recall_result", {}) if isinstance(prepare_payload, dict) else {}
        prepare_vector = prepare_recall.get("vector_shadow", {}) if isinstance(prepare_recall, dict) else {}
        report["http_prepare_turn_recall"] = {
            "status_code": prepare_status,
            "request": {
                "chat_session_id": sid,
                "query_id": str(query.get("query_id") or ""),
                "source_id": source_id,
                "query_vector_dim": len(query_vector),
                "limit": req_limit,
                "filter": filter_expr,
            },
            "recall_result": prepare_recall,
        }
        if product_read_proof:
            if go_proc is not None:
                stop_process(go_proc, report, "go_server")
                go_proc = None

            rollback_port = find_free_port()
            rollback_env = env.copy()
            rollback_env["AC_BIND_ADDR"] = f"127.0.0.1:{rollback_port}"
            rollback_env["AC_MODE"] = "shadow"
            rollback_env["AC_STORE_MODE"] = "noop"
            rollback_env["AC_MILVUS_SDK_ENABLED"] = "true"
            rollback_env["AC_MILVUS_ENDPOINT"] = milvus_endpoint
            rollback_env["AC_CHROMA_SHADOW_PERSIST_DIR"] = str(data_root / "not-used-chroma-shadow")
            rollback_base_url = f"http://127.0.0.1:{rollback_port}"
            rollback_proc = subprocess.Popen([str(server_bin)], cwd=go_service, env=rollback_env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            report["rollback_proof"] = {
                "bind": rollback_env["AC_BIND_ADDR"],
                "milvus_sdk_enabled": True,
                "milvus_recall_read_enabled": False,
                "milvus_product_read_enabled": False,
                "started": True,
            }
            try:
                if not wait_for_http(f"{rollback_base_url}/health", float(args.wait_timeout_s)):
                    report["failures"].append("rollback_go_http_server_not_ready")
                else:
                    rollback_status, rollback_payload = http_json(
                        "POST",
                        f"{rollback_base_url}/prepare-turn",
                        prepare_turn_request,
                        timeout_s=float(args.http_timeout_s),
                    )
                    rollback_recall = rollback_payload.get("recall_result", {}) if isinstance(rollback_payload, dict) else {}
                    rollback_vector = rollback_recall.get("vector_shadow", {}) if isinstance(rollback_recall, dict) else {}
                    report["rollback_proof"].update({
                        "status_code": rollback_status,
                        "recall_result": rollback_recall,
                        "rolled_back": (
                            rollback_status == 200
                            and rollback_recall.get("would_call_vector") is False
                            and rollback_vector.get("product_read_enabled") is False
                            and rollback_vector.get("live_retrieval_enabled") is False
                            and rollback_vector.get("milvus_live_enabled") is False
                        ),
                    })
            finally:
                stop_process(rollback_proc, report, "rollback_proof")

        result_ids = []
        for item in drill_payload.get("search_results", []) if isinstance(drill_payload, dict) else []:
            if isinstance(item, dict) and item.get("id") is not None:
                result_ids.append(str(item["id"]))
        prepare_result_ids = []
        for item in prepare_vector.get("search_results", []) if isinstance(prepare_vector, dict) else []:
            if isinstance(item, dict) and item.get("id") is not None:
                prepare_result_ids.append(str(item["id"]))
        report["summary"] = {
            "status_code_ok": backfill_status == 200 and drill_status == 200,
            "http_route_used_milvus_sdk": drill_payload.get("vector_health_status") == "loaded",
            "search_result": drill_payload.get("search_result"),
            "search_result_count": drill_payload.get("search_result_count"),
            "query_source_id": source_id,
            "top_id": result_ids[0] if result_ids else None,
            "top1_match": bool(result_ids and result_ids[0] == source_id),
            "self_found": source_id in result_ids,
            "prepare_turn_status_code_ok": prepare_status == 200,
            "prepare_turn_used_milvus_sdk": prepare_vector.get("status") == "loaded",
            "prepare_turn_would_call_vector": prepare_recall.get("would_call_vector") is True if isinstance(prepare_recall, dict) else False,
            "prepare_turn_search_result": prepare_vector.get("search_result"),
            "prepare_turn_search_result_count": prepare_vector.get("search_result_count"),
            "prepare_turn_top_id": prepare_result_ids[0] if prepare_result_ids else None,
            "prepare_turn_top1_match": bool(prepare_result_ids and prepare_result_ids[0] == source_id),
            "prepare_turn_self_found": source_id in prepare_result_ids,
            "prepare_turn_product_read_enabled": prepare_vector.get("product_read_enabled"),
            "live_retrieval_enabled": drill_payload.get("live_retrieval_enabled"),
            "milvus_live_enabled": drill_payload.get("milvus_live_enabled"),
        }
        if product_read_proof:
            source_chroma = str(query_set.get("source_chroma_persist_dir") or "")
            configured_chroma = server_env.get("AC_CHROMA_SHADOW_PERSIST_DIR", "")
            report["chroma_decommission_proof"] = {
                "status": "ok",
                "source_chroma_persist_dir": source_chroma,
                "configured_chroma_shadow_persist_dir": configured_chroma,
                "configured_chroma_path_exists": Path(configured_chroma).exists() if configured_chroma else None,
                "old_chroma_path_deleted_by_probe": False,
                "prepare_turn_ok_without_chroma_shadow_runtime": prepare_status == 200 and report["summary"]["prepare_turn_search_result"] == "ok",
                "milvus_result_available_without_chroma_shadow_runtime": report["summary"]["prepare_turn_self_found"],
                "rollback_proof_available": bool(report.get("rollback_proof", {}).get("rolled_back")),
                "decommission_meaning": "selected recall read path no longer depends on Chroma at runtime; this probe does not delete old Chroma data",
            }
            report["summary"]["rollback_proof_ok"] = bool(report.get("rollback_proof", {}).get("rolled_back"))
            report["summary"]["chroma_decommission_proof_ok"] = (
                report["chroma_decommission_proof"]["prepare_turn_ok_without_chroma_shadow_runtime"]
                and report["chroma_decommission_proof"]["milvus_result_available_without_chroma_shadow_runtime"]
                and report["chroma_decommission_proof"]["rollback_proof_available"]
            )
        if not report["summary"]["status_code_ok"] or not report["summary"]["prepare_turn_status_code_ok"]:
            report["failures"].append("http_status_not_ok")
        if not report["summary"]["http_route_used_milvus_sdk"]:
            report["failures"].append("http_route_did_not_report_loaded_milvus")
        if not report["summary"]["prepare_turn_used_milvus_sdk"]:
            report["failures"].append("prepare_turn_did_not_report_loaded_milvus")
        if report["summary"]["search_result"] != "ok":
            report["failures"].append("http_live_read_search_not_ok")
        if report["summary"]["prepare_turn_search_result"] != "ok":
            report["failures"].append("prepare_turn_search_not_ok")
        if not report["summary"]["self_found"]:
            report["failures"].append("query_source_not_found")
        if not report["summary"]["prepare_turn_self_found"]:
            report["failures"].append("prepare_turn_query_source_not_found")
        if drill_payload.get("live_retrieval_enabled") is not False or drill_payload.get("milvus_live_enabled") is not False:
            report["failures"].append("unexpected_live_cutover_flag")
        if product_read_proof:
            if prepare_vector.get("live_retrieval_enabled") is not True or prepare_vector.get("milvus_live_enabled") is not True:
                report["failures"].append("prepare_turn_product_live_flags_not_true")
            if not report["summary"].get("rollback_proof_ok"):
                report["failures"].append("rollback_proof_failed")
            if not report["summary"].get("chroma_decommission_proof_ok"):
                report["failures"].append("chroma_decommission_proof_failed")
        elif prepare_vector.get("live_retrieval_enabled") is not False or prepare_vector.get("milvus_live_enabled") is not False:
            report["failures"].append("prepare_turn_unexpected_live_cutover_flag")
    except Exception as exc:
        report["failures"].append(f"{type(exc).__name__}: {exc}")
    finally:
        if go_proc is not None:
            stop_process(go_proc, report, "go_server")
        stop_process(milvus_proc, report, "milvus_server")
        if args.cleanup:
            cleanup_error = cleanup_dir(data_root)
            if cleanup_error:
                report["warnings"].append(f"cleanup_incomplete:{cleanup_error}")

    if report["failures"]:
        return report, 1
    report["status"] = "ok" if not report["warnings"] else "degraded"
    return report, 0


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run Go HTTP Milvus bounded live-read probe")
    parser.add_argument("--runtime-python", default=str(default_runtime_python()))
    parser.add_argument("--data-root", default=str(workspace_root() / DEFAULT_DATA_REL / "r1-95"))
    parser.add_argument("--go-cache", default=str(workspace_root() / DEFAULT_DATA_REL / "go-cache"))
    parser.add_argument("--query-set", default=str(repo_root() / "benchmarks" / "chroma-milvus-query-set-2026-05-25-real.json"))
    parser.add_argument("--out", help="write JSON report to this path")
    parser.add_argument("--milvus-port", type=int, default=0)
    parser.add_argument("--go-port", type=int, default=0)
    parser.add_argument("--doc-limit", type=int, default=5)
    parser.add_argument("--result-limit", type=int, default=5)
    parser.add_argument("--wait-timeout-s", type=float, default=45.0)
    parser.add_argument("--go-timeout-s", type=float, default=120.0)
    parser.add_argument("--http-timeout-s", type=float, default=20.0)
    parser.add_argument("--product-read-proof", action="store_true", help="enable R2 product-read flag, rollback proof, and Chroma decommission proof")
    parser.add_argument("--cleanup", action="store_true", help="remove data-root after the run")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    report, exit_code = run_probe(args)
    output = json.dumps(report, indent=2, ensure_ascii=True)
    if args.out:
        out = Path(args.out).expanduser().resolve()
        out.parent.mkdir(parents=True, exist_ok=True)
        out.write_text(output + "\n", encoding="utf-8")
    print(output)
    return exit_code


if __name__ == "__main__":
    raise SystemExit(main())
