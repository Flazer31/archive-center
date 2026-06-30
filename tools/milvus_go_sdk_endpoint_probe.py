#!/usr/bin/env python3
"""Start managed Milvus Lite server and run the Go SDK smoke against it.

This is R1 evidence only. It starts a disposable local Milvus Lite gRPC
endpoint outside the source tree, runs the guarded Go `milvus-sdk-smoke`
executor with `-execute -ensure-collection`, then shuts the endpoint down.

It does not enable live retrieval, does not retire Chroma, and does not touch
the 0.8 worktree.
"""

from __future__ import annotations

import argparse
import json
import os
import shutil
import socket
import subprocess
import sys
import time
from pathlib import Path
from typing import Any

SCHEMA_VERSION = "archive-center.milvus-go-sdk-endpoint-probe.v1"
DEFAULT_RUNTIME_REL = Path(".runtime-cache") / "temp" / "archive-center-2.0" / "milvus-lite-runtime"
DEFAULT_DATA_REL = Path(".runtime-cache") / "temp" / "archive-center-2.0" / "go-sdk-endpoint-probe"


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


def read_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def truncate(text: str, limit: int = 4000) -> str:
    if len(text) <= limit:
        return text
    return text[:limit] + "...<truncated>"


def run_probe(args: argparse.Namespace) -> tuple[dict[str, Any], int]:
    root = repo_root()
    runtime_python = Path(args.runtime_python).expanduser().resolve()
    data_root = Path(args.data_root).expanduser().resolve()
    go_cache = Path(args.go_cache).expanduser().resolve() if args.go_cache else workspace_root() / DEFAULT_DATA_REL / "go-cache"
    report: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "tool": "milvus_go_sdk_endpoint_probe",
        "status": "failed",
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "runtime_python": str(runtime_python),
        "data_root": str(data_root),
        "go_cache": str(go_cache),
        "query_set": str(Path(args.query_set).expanduser().resolve()) if args.query_set else None,
        "endpoint": None,
        "server": {},
        "go_smoke": {},
        "warnings": [],
        "failures": [],
        "safety": {
            "milvus_live_enabled": False,
            "live_retrieval_enabled": False,
            "does_not_touch_archive_center_0_8": True,
            "does_not_retire_chroma": True,
            "does_not_enable_go_default_runtime": True,
        },
    }

    if path_inside(data_root, root):
        report["failures"].append("data_root_inside_source_tree")
    if not runtime_python.exists():
        report["failures"].append("runtime_python_missing")

    if report["failures"]:
        return report, 1

    port = int(args.port) if args.port else find_free_port()
    endpoint = f"http://127.0.0.1:{port}"
    report["endpoint"] = endpoint
    server_data_dir = data_root / f"milvus-server-{port}.db"
    go_report_path = data_root / f"go-milvus-sdk-smoke-{port}.json"
    data_root.mkdir(parents=True, exist_ok=True)
    go_cache.mkdir(parents=True, exist_ok=True)

    server_cmd = [
        str(runtime_python),
        "-m",
        "milvus_lite",
        "server",
        "--data-dir",
        str(server_data_dir),
        "--host",
        "127.0.0.1",
        "--port",
        str(port),
    ]
    server = subprocess.Popen(server_cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    report["server"] = {
        "command": server_cmd,
        "data_dir": str(server_data_dir),
        "started": True,
        "port": port,
    }

    try:
        if not wait_for_tcp("127.0.0.1", port, float(args.wait_timeout_s)):
            report["failures"].append("milvus_endpoint_not_ready")
            return report, 1

        env = os.environ.copy()
        env["GOCACHE"] = str(go_cache)
        go_cmd = [
            "go",
            "run",
            "-buildvcs=false",
            "./cmd/milvus-sdk-smoke",
            "-execute",
            "-ensure-collection",
            "-endpoint",
            endpoint,
            "-dim",
            str(args.dimension),
            "-out",
            str(go_report_path),
        ]
        if args.query_set:
            query_set_path = Path(args.query_set).expanduser().resolve()
            if not query_set_path.exists():
                report["failures"].append("query_set_missing")
                return report, 1
            go_cmd.extend(["-query-set", str(query_set_path)])
        run = subprocess.run(
            go_cmd,
            cwd=root / "go-service",
            env=env,
            text=True,
            capture_output=True,
            timeout=float(args.go_timeout_s),
        )
        smoke = read_json(go_report_path) if go_report_path.exists() else {}
        report["go_smoke"] = {
            "command": go_cmd,
            "returncode": run.returncode,
            "stdout": truncate(run.stdout),
            "stderr": truncate(run.stderr),
            "report_path": str(go_report_path),
            "report": smoke,
        }
        if run.returncode != 0:
            report["failures"].append("go_milvus_sdk_smoke_failed")
        if smoke.get("status") != "ok":
            report["failures"].append("go_milvus_sdk_smoke_not_ok")
    except Exception as exc:
        report["failures"].append(f"{type(exc).__name__}: {exc}")
    finally:
        server.terminate()
        try:
            stdout, stderr = server.communicate(timeout=5)
        except subprocess.TimeoutExpired:
            server.kill()
            stdout, stderr = server.communicate(timeout=5)
        report["server"]["returncode"] = server.returncode
        report["server"]["stopped_by_probe"] = True
        if server.returncode not in (0, None):
            report["server"]["returncode_note"] = "non-zero is acceptable after terminating the disposable Milvus Lite server"
        report["server"]["stdout"] = truncate(stdout or "")
        report["server"]["stderr"] = truncate(stderr or "")
        if args.cleanup:
            cleanup_error = cleanup_dir(data_root)
            if cleanup_error:
                report["warnings"].append(f"cleanup_incomplete:{cleanup_error}")

    if report["failures"]:
        return report, 1
    report["status"] = "ok" if not report["warnings"] else "degraded"
    return report, 0


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run Go SDK smoke against managed Milvus Lite endpoint")
    parser.add_argument("--runtime-python", default=str(default_runtime_python()))
    parser.add_argument("--data-root", default=str(workspace_root() / DEFAULT_DATA_REL / "r1-94"))
    parser.add_argument("--go-cache", default=str(workspace_root() / DEFAULT_DATA_REL / "go-cache"))
    parser.add_argument("--out", help="write JSON report to this path")
    parser.add_argument(
        "--query-set",
        default=str(repo_root() / "benchmarks" / "chroma-milvus-query-set-2026-05-25-real.json"),
        help="real vector query-set JSON for Go SDK smoke",
    )
    parser.add_argument("--port", type=int, default=0)
    parser.add_argument("--dimension", type=int, default=4)
    parser.add_argument("--wait-timeout-s", type=float, default=20.0)
    parser.add_argument("--go-timeout-s", type=float, default=120.0)
    parser.add_argument("--cleanup", action="store_true", help="remove data-root after the run")
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
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
    raise SystemExit(main(sys.argv[1:]))
