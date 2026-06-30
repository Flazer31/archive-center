#!/usr/bin/env python3
"""Measure disposable MariaDB + Milvus Lite + Go low-resource footprint.

This probe starts all three components in temporary locations outside the
source tree, measures startup/RSS/read-only latency, then tears everything
down. It does not import user data, enable persistent authority, switch the Go
default runtime, retire Python, or edit the 0.8 reference tree.
"""

from __future__ import annotations

import argparse
import json
import os
import platform
import shutil
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
COMBINED_RSS_THRESHOLD_MB = 1200.0
GO_RSS_THRESHOLD_MB = 150.0
STARTUP_THRESHOLD_MS = 30_000.0
LATENCY_P95_THRESHOLD_MS = 100.0


def repo_root() -> Path:
    return Path(__file__).resolve().parents[1]


def workspace_root() -> Path:
    return repo_root().parent


def default_runtime_cache() -> Path:
    return workspace_root() / ".runtime-cache" / "temp" / "archive-center-2.0" / "combined-low-resource"


def choose_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


def wait_tcp(host: str, port: int, timeout_s: float) -> tuple[bool, float]:
    started = time.perf_counter()
    while (time.perf_counter() - started) < timeout_s:
        try:
            with socket.create_connection((host, port), timeout=0.5):
                return True, (time.perf_counter() - started) * 1000
        except OSError:
            time.sleep(0.1)
    return False, (time.perf_counter() - started) * 1000


def request_once(url: str, timeout: float = 2.0) -> dict[str, Any]:
    started = time.perf_counter()
    try:
        with urllib.request.urlopen(url, timeout=timeout) as response:  # noqa: S310 - loopback only
            body = response.read(1 << 20)
            return {
                "ok": True,
                "status_code": int(response.status),
                "latency_ms": (time.perf_counter() - started) * 1000,
                "bytes": len(body),
                "error": "",
            }
    except (urllib.error.URLError, TimeoutError, OSError) as exc:
        return {
            "ok": False,
            "status_code": 0,
            "latency_ms": (time.perf_counter() - started) * 1000,
            "bytes": 0,
            "error": str(exc),
        }


def wait_http_ok(url: str, timeout_s: float) -> tuple[bool, int, float, str]:
    started = time.perf_counter()
    attempts = 0
    last_error = ""
    while (time.perf_counter() - started) < timeout_s:
        attempts += 1
        result = request_once(url, timeout=1)
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
        results = [request_once(f"{base_url}{path}") for _ in range(count)]
        latencies = [float(item["latency_ms"]) for item in results if item["ok"] and item["status_code"] == 200]
        reports.append(
            {
                "path": path,
                "total": count,
                "success": len(latencies),
                "failure": count - len(latencies),
                "p95_ms": percentile(latencies, 0.95),
                "max_ms": max(latencies) if latencies else 0.0,
            }
        )
    return reports


def run_command(args: list[str], cwd: Path | None = None, env: dict[str, str] | None = None, timeout: int = 120) -> dict[str, Any]:
    proc = subprocess.run(
        args,
        cwd=str(cwd) if cwd else None,
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
        "stdout_tail": proc.stdout[-2000:],
        "stderr_tail": proc.stderr[-4000:],
    }


def find_mariadb_provider(explicit: str = "") -> Path | None:
    names = ("mariadbd.exe", "mysqld.exe") if os.name == "nt" else ("mariadbd", "mysqld")
    candidates: list[Path] = []
    if explicit:
        candidates.append(Path(explicit))
    for name in names:
        found = shutil.which(name)
        if found:
            candidates.append(Path(found))
    roots = [
        workspace_root() / ".runtime-cache" / "archive-center-2.0-install" / "runtime" / "MariaDB",
        workspace_root() / ".runtime-cache" / "archive-center-2.0-install" / "runtime" / "mariadb",
    ]
    for root in roots:
        if root.exists():
            for name in names:
                candidates.extend(root.rglob(name))
    for candidate in candidates:
        if candidate.is_file():
            return candidate.resolve()
    return None


def find_milvus_python(explicit: str = "") -> Path | None:
    candidates: list[Path] = []
    if explicit:
        candidates.append(Path(explicit))
    candidates.append(workspace_root() / ".runtime-cache" / "temp" / "archive-center-2.0" / "milvus-lite-runtime" / "Scripts" / "python.exe")
    candidates.append(workspace_root() / ".runtime-cache" / "archive-center-2.0" / "milvus-lite-runtime" / "Scripts" / "python.exe")
    candidates.append(Path(sys.executable))
    for candidate in candidates:
        if candidate.is_file():
            probe = run_command([str(candidate), "-c", "import importlib.util, sys; sys.exit(0 if importlib.util.find_spec('milvus_lite') else 1)"], timeout=20)
            if probe["exit_code"] == 0:
                return candidate.resolve()
    return None


def find_mariadb_install_db(provider: Path) -> Path | None:
    names = ("mariadb-install-db.exe", "mysql_install_db.exe") if os.name == "nt" else ("mariadb-install-db", "mysql_install_db")
    for name in names:
        candidate = provider.parent / name
        if candidate.is_file():
            return candidate.resolve()
    return None


def read_rss_bytes(pid: int) -> tuple[int, str]:
    if pid <= 0:
        return 0, "invalid_pid"
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


def stop_process(proc: subprocess.Popen[str] | None) -> dict[str, Any]:
    if proc is None:
        return {"status": "not_started"}
    proc.terminate()
    try:
        stdout, stderr = proc.communicate(timeout=8)
    except subprocess.TimeoutExpired:
        proc.kill()
        stdout, stderr = proc.communicate(timeout=8)
    return {
        "status": "stopped",
        "returncode": proc.returncode,
        "stdout_tail": (stdout or "")[-2000:],
        "stderr_tail": (stderr or "")[-4000:],
    }


def build_go_binary(root: Path, out_dir: Path, env: dict[str, str]) -> tuple[Path, dict[str, Any]]:
    binary = out_dir / ("archive-center-go.exe" if os.name == "nt" else "archive-center-go")
    result = run_command(
        ["go", "build", "-buildvcs=false", "-ldflags=-s -w", "-o", str(binary), "./cmd/archive-center-go"],
        cwd=root / "go-service",
        env=env,
        timeout=180,
    )
    return binary, result


def start_mariadb(provider: Path, data_dir: Path, port: int) -> tuple[subprocess.Popen[str] | None, dict[str, Any]]:
    data_dir.mkdir(parents=True, exist_ok=True)
    install_db = find_mariadb_install_db(provider)
    if install_db:
        init = run_command([str(install_db), f"--datadir={data_dir}", "--password="], timeout=120)
    else:
        init = run_command([str(provider), "--no-defaults", "--initialize-insecure", "--datadir", str(data_dir)], timeout=120)
    if init["exit_code"] != 0:
        return None, {"status": "failed", "init": init}

    args = [
        str(provider),
        "--no-defaults",
        "--datadir",
        str(data_dir),
        "--port",
        str(port),
        "--socket",
        str(data_dir / "mysql.sock"),
        "--skip-networking=0",
        "--bind-address=127.0.0.1",
        "--pid-file",
        str(data_dir / "mysqld.pid"),
    ]
    proc = subprocess.Popen(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    ready, startup_ms = wait_tcp("127.0.0.1", port, 30)
    return proc, {"status": "ok" if ready else "failed", "pid": proc.pid, "port": port, "startup_ms": startup_ms, "provider": str(provider)}


def start_milvus(python_exe: Path, data_dir: Path, port: int) -> tuple[subprocess.Popen[str] | None, dict[str, Any]]:
    data_dir.parent.mkdir(parents=True, exist_ok=True)
    args = [
        str(python_exe),
        "-m",
        "milvus_lite",
        "server",
        "--data-dir",
        str(data_dir),
        "--host",
        "127.0.0.1",
        "--port",
        str(port),
    ]
    proc = subprocess.Popen(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    ready, startup_ms = wait_tcp("127.0.0.1", port, 30)
    return proc, {"status": "ok" if ready else "failed", "pid": proc.pid, "port": port, "startup_ms": startup_ms, "runtime_python": str(python_exe)}


def start_go(root: Path, binary: Path, env: dict[str, str], port: int, mariadb_port: int, milvus_port: int) -> tuple[subprocess.Popen[str] | None, dict[str, Any]]:
    go_env = env.copy()
    go_env.update(
        {
            "AC_BIND_ADDR": f"127.0.0.1:{port}",
            "AC_MODE": "shadow",
            "AC_STORE_MODE": "mariadb_read_shadow",
            "AC_MARIADB_DSN": f"root@tcp(127.0.0.1:{mariadb_port})/?timeout=5s&parseTime=true",
            "AC_MILVUS_SDK_ENABLED": "true",
            "AC_MILVUS_ENDPOINT": f"http://127.0.0.1:{milvus_port}",
            "AC_MILVUS_RECALL_READ_ENABLED": "false",
            "AC_MILVUS_PRODUCT_READ_ENABLED": "false",
        }
    )
    create_flags = getattr(subprocess, "CREATE_NO_WINDOW", 0) if os.name == "nt" else 0
    proc = subprocess.Popen([str(binary)], cwd=str(root / "go-service"), env=go_env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, creationflags=create_flags)
    ready, attempts, startup_ms, error = wait_http_ok(f"http://127.0.0.1:{port}/health", 30)
    return proc, {"status": "ok" if ready else "failed", "pid": proc.pid, "port": port, "startup_ms": startup_ms, "attempts": attempts, "error": error}


def build_report(args: argparse.Namespace) -> dict[str, Any]:
    root = Path(args.repo_root).resolve()
    runtime_root = Path(args.runtime_root).resolve()
    runtime_root.mkdir(parents=True, exist_ok=True)
    provider = find_mariadb_provider(args.mariadb_provider)
    milvus_python = find_milvus_python(args.milvus_python)
    report: dict[str, Any] = {
        "schema_version": "archive-center.combined_stack_low_resource_probe.v1",
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "status": "failed",
        "scope": {
            "repo_root": str(root),
            "runtime_root": str(runtime_root),
            "combined_stack": True,
            "disposable": True,
            "mutates_source_tree": False,
            "authority_switch": False,
            "go_default_switch": False,
            "milvus_product_read": False,
            "python_retirement": False,
        },
        "host": {
            "system": platform.system(),
            "release": platform.release(),
            "machine": platform.machine(),
            "python": sys.version.split()[0],
        },
        "components": {},
        "rss": {},
        "routes": [],
        "warnings": [],
        "failures": [],
    }
    if provider is None:
        report["failures"].append("mariadb_provider_not_found")
        return report
    if milvus_python is None:
        report["failures"].append("milvus_lite_runtime_not_found")
        return report

    env = os.environ.copy()
    env["GOCACHE"] = str(runtime_root / "go-cache")
    temp_dir = Path(tempfile.mkdtemp(prefix="run-", dir=str(runtime_root)))
    mariadb_proc: subprocess.Popen[str] | None = None
    milvus_proc: subprocess.Popen[str] | None = None
    go_proc: subprocess.Popen[str] | None = None
    try:
        binary, build = build_go_binary(root, temp_dir, env)
        report["components"]["go_build"] = build
        if build["exit_code"] != 0:
            report["failures"].append("go_build_failed")
            return report

        mariadb_port = choose_port()
        milvus_port = choose_port()
        go_port = choose_port()

        mariadb_proc, mariadb_info = start_mariadb(provider, temp_dir / "mariadb-data", mariadb_port)
        report["components"]["mariadb"] = mariadb_info
        if mariadb_info.get("status") != "ok":
            report["failures"].append("mariadb_not_ready")
            return report

        milvus_proc, milvus_info = start_milvus(milvus_python, temp_dir / "milvus.db", milvus_port)
        report["components"]["milvus_lite"] = milvus_info
        if milvus_info.get("status") != "ok":
            report["failures"].append("milvus_lite_not_ready")
            return report

        go_proc, go_info = start_go(root, binary, env, go_port, mariadb_port, milvus_port)
        report["components"]["go"] = go_info
        if go_info.get("status") != "ok":
            report["failures"].append("go_not_ready")
            return report

        time.sleep(1.0)
        rss_items: dict[str, Any] = {}
        total_rss = 0
        for name, proc in (("mariadb", mariadb_proc), ("milvus_lite", milvus_proc), ("go", go_proc)):
            rss_bytes, rss_error = read_rss_bytes(proc.pid if proc else 0)
            total_rss += rss_bytes
            rss_items[name] = {
                "pid": proc.pid if proc else 0,
                "rss_bytes": rss_bytes,
                "rss_mb": rss_bytes / (1024 * 1024) if rss_bytes else 0.0,
                "rss_error": rss_error,
            }
        report["rss"] = {
            "components": rss_items,
            "total_bytes": total_rss,
            "total_mb": total_rss / (1024 * 1024) if total_rss else 0.0,
        }
        report["routes"] = measure_routes(f"http://127.0.0.1:{go_port}", args.count)
        max_p95 = max((item["p95_ms"] for item in report["routes"]), default=0.0)
        checks = {
            "mariadb_ready": mariadb_info.get("status") == "ok",
            "milvus_ready": milvus_info.get("status") == "ok",
            "go_ready": go_info.get("status") == "ok",
            "go_rss_ok": rss_items["go"]["rss_mb"] <= GO_RSS_THRESHOLD_MB,
            "combined_rss_ok": report["rss"]["total_mb"] <= COMBINED_RSS_THRESHOLD_MB,
            "startup_ok": go_info.get("startup_ms", 0) <= STARTUP_THRESHOLD_MS,
            "read_only_latency_ok": bool(report["routes"]) and all(item["failure"] == 0 for item in report["routes"]) and max_p95 <= LATENCY_P95_THRESHOLD_MS,
        }
        report["thresholds"] = {
            "combined_rss_mb_max": COMBINED_RSS_THRESHOLD_MB,
            "go_rss_mb_max": GO_RSS_THRESHOLD_MB,
            "go_startup_ms_max": STARTUP_THRESHOLD_MS,
            "read_only_route_p95_ms_max": LATENCY_P95_THRESHOLD_MS,
        }
        report["checks"] = checks
        report["status"] = "ok" if all(checks.values()) else "degraded"
        return report
    finally:
        report["stop"] = {
            "go": stop_process(go_proc),
            "milvus_lite": stop_process(milvus_proc),
            "mariadb": stop_process(mariadb_proc),
        }
        if not args.keep_temp:
            shutil.rmtree(temp_dir, ignore_errors=True)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", default=str(repo_root()))
    parser.add_argument("--runtime-root", default=str(default_runtime_cache()))
    parser.add_argument("--out", default="")
    parser.add_argument("--count", type=int, default=20)
    parser.add_argument("--mariadb-provider", default="")
    parser.add_argument("--milvus-python", default="")
    parser.add_argument("--keep-temp", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    report = build_report(args)
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
