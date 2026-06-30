#!/usr/bin/env python3
"""Measure Go-only low-resource readiness for Archive Center 2.0.

The probe builds the Go backend into a temporary directory, starts it in
shadow/noop mode, measures startup time, idle RSS, and read-only route latency,
then writes a JSON report. It does not start MariaDB or Milvus, does not mutate
runtime authority, and does not write into the 0.8 reference tree.
"""

from __future__ import annotations

import argparse
import json
import os
import platform
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any


SAFE_PATHS = ("/health", "/ready", "/version")
STARTUP_THRESHOLD_MS = 5_000
RSS_THRESHOLD_MB = 150.0
LATENCY_P95_THRESHOLD_MS = 100.0


def choose_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


def run_command(args: list[str], cwd: Path, env: dict[str, str], timeout: int = 120) -> dict[str, Any]:
    proc = subprocess.run(
        args,
        cwd=str(cwd),
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        timeout=timeout,
        check=False,
    )
    return {
        "args": args,
        "exit_code": proc.returncode,
        "stdout": proc.stdout.strip(),
        "stderr": proc.stderr.strip(),
    }


def request_once(url: str, timeout: float) -> dict[str, Any]:
    started = time.perf_counter()
    try:
        with urllib.request.urlopen(url, timeout=timeout) as response:  # noqa: S310 - local loopback only
            body = response.read(1 << 20)
            elapsed_ms = (time.perf_counter() - started) * 1000
            return {
                "ok": True,
                "status_code": int(response.status),
                "latency_ms": elapsed_ms,
                "bytes": len(body),
                "error": "",
            }
    except (urllib.error.URLError, TimeoutError, OSError) as exc:
        elapsed_ms = (time.perf_counter() - started) * 1000
        return {
            "ok": False,
            "status_code": 0,
            "latency_ms": elapsed_ms,
            "bytes": 0,
            "error": str(exc),
        }


def wait_health(base_url: str, timeout_sec: float) -> tuple[bool, int, float, str]:
    started = time.perf_counter()
    attempts = 0
    last_error = ""
    while (time.perf_counter() - started) < timeout_sec:
        attempts += 1
        result = request_once(f"{base_url}/health", timeout=1)
        if result["ok"] and result["status_code"] == 200:
            return True, attempts, (time.perf_counter() - started) * 1000, ""
        last_error = result["error"]
        time.sleep(0.1)
    return False, attempts, (time.perf_counter() - started) * 1000, last_error or "timeout"


def percentile(values: list[float], p: float) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    if len(ordered) == 1:
        return ordered[0]
    idx = (len(ordered) - 1) * p
    lower = int(idx)
    frac = idx - lower
    if lower >= len(ordered) - 1:
        return ordered[-1]
    return ordered[lower] + ((ordered[lower + 1] - ordered[lower]) * frac)


def measure_routes(base_url: str, count: int) -> list[dict[str, Any]]:
    reports: list[dict[str, Any]] = []
    for path in SAFE_PATHS:
        results = [request_once(f"{base_url}{path}", timeout=2) for _ in range(count)]
        latencies = [float(item["latency_ms"]) for item in results if item["ok"]]
        reports.append(
            {
                "path": path,
                "total": count,
                "success": sum(1 for item in results if item["ok"] and item["status_code"] == 200),
                "failure": sum(1 for item in results if not item["ok"] or item["status_code"] != 200),
                "p95_ms": percentile(latencies, 0.95),
                "max_ms": max(latencies) if latencies else 0.0,
            }
        )
    return reports


def read_rss_bytes(pid: int) -> tuple[int, str]:
    if platform.system().lower() == "windows":
        proc = subprocess.run(
            ["powershell", "-NoProfile", "-Command", f"(Get-Process -Id {pid}).WorkingSet64"],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            check=False,
        )
        if proc.returncode == 0:
            try:
                return int(proc.stdout.strip()), ""
            except ValueError:
                return 0, f"invalid_rss_output:{proc.stdout.strip()}"
        return 0, proc.stderr.strip() or "powershell_get_process_failed"

    status_path = Path(f"/proc/{pid}/status")
    if status_path.exists():
        for line in status_path.read_text(encoding="utf-8", errors="ignore").splitlines():
            if line.startswith("VmRSS:"):
                parts = line.split()
                if len(parts) >= 2:
                    return int(parts[1]) * 1024, ""
    return 0, "rss_not_available"


def build_report(repo_root: Path, count: int, startup_timeout: int) -> dict[str, Any]:
    root = repo_root.resolve()
    go_service = root / "go-service"
    with tempfile.TemporaryDirectory(prefix="archive-center-go-low-resource-") as tmp:
        temp_root = Path(tmp)
        binary = temp_root / ("archive-center-go.exe" if platform.system().lower() == "windows" else "archive-center-go")
        env = os.environ.copy()
        env["GOCACHE"] = str(temp_root / "gocache")
        env["AC_MODE"] = "shadow"
        env["AC_STORE_MODE"] = "noop"
        port = choose_port()
        env["AC_BIND_ADDR"] = f"127.0.0.1:{port}"

        build = run_command(
            ["go", "build", "-buildvcs=false", '-ldflags=-s -w', "-o", str(binary), "./cmd/archive-center-go"],
            cwd=go_service,
            env=env,
            timeout=180,
        )
        if build["exit_code"] != 0:
            return {
                "schema_version": "archive-center.low_resource_go_probe.v1",
                "status": "failed",
                "build": build,
            }

        create_flags = 0
        if platform.system().lower() == "windows":
            create_flags = getattr(subprocess, "CREATE_NO_WINDOW", 0)
        process = subprocess.Popen(  # noqa: S603 - local binary built from current workspace
            [str(binary)],
            cwd=str(go_service),
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            creationflags=create_flags,
        )
        try:
            base_url = f"http://127.0.0.1:{port}"
            ready, attempts, startup_ms, ready_error = wait_health(base_url, startup_timeout)
            rss_bytes, rss_error = read_rss_bytes(process.pid) if ready else (0, "not_ready")
            route_reports = measure_routes(base_url, count) if ready else []
        finally:
            process.terminate()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=5)

    max_p95 = max((item["p95_ms"] for item in route_reports), default=0.0)
    rss_mb = rss_bytes / (1024 * 1024) if rss_bytes else 0.0
    thresholds = {
        "startup_ms_max": STARTUP_THRESHOLD_MS,
        "idle_rss_mb_max": RSS_THRESHOLD_MB,
        "read_only_route_p95_ms_max": LATENCY_P95_THRESHOLD_MS,
    }
    checks = {
        "startup_ok": ready and startup_ms <= STARTUP_THRESHOLD_MS,
        "idle_rss_ok": rss_bytes > 0 and rss_mb <= RSS_THRESHOLD_MB,
        "read_only_latency_ok": bool(route_reports) and all(item["failure"] == 0 for item in route_reports) and max_p95 <= LATENCY_P95_THRESHOLD_MS,
        "combined_stack_measured": False,
    }
    status = "ok" if checks["startup_ok"] and checks["idle_rss_ok"] and checks["read_only_latency_ok"] else "degraded"
    return {
        "schema_version": "archive-center.low_resource_go_probe.v1",
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "status": status,
        "scope": {
            "repo_root": str(root),
            "go_only": True,
            "report_only": True,
            "mutates_source_tree": False,
            "mariadb_started": False,
            "milvus_started": False,
            "authority_switch": False,
            "go_default_switch": False,
        },
        "host": {
            "system": platform.system(),
            "release": platform.release(),
            "machine": platform.machine(),
            "python": sys.version.split()[0],
        },
        "build": {"exit_code": build["exit_code"], "stderr": build["stderr"]},
        "startup": {
            "ready": ready,
            "attempts": attempts,
            "elapsed_ms": startup_ms,
            "error": ready_error,
        },
        "process": {
            "rss_bytes": rss_bytes,
            "rss_mb": rss_mb,
            "rss_error": rss_error,
        },
        "routes": route_reports,
        "thresholds": thresholds,
        "checks": checks,
        "blockers": ["combined_stack_measurement_plan_only_until_approved"],
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--out", default="")
    parser.add_argument("--count", type=int, default=20)
    parser.add_argument("--startup-timeout", type=int, default=10)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    report = build_report(Path(args.repo_root), args.count, args.startup_timeout)
    payload = json.dumps(report, indent=2, ensure_ascii=False)
    if args.out:
        out_path = Path(args.out)
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(payload + "\n", encoding="utf-8")
    else:
        print(payload)
    return 0 if report.get("status") == "ok" else 2


if __name__ == "__main__":
    raise SystemExit(main())
