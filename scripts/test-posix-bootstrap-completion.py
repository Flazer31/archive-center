#!/usr/bin/env python3
"""Exercise the production timeout wrapper's output lifetime, without installing.

An inherited output pipe can keep diagnostics.sh's tee alive after --install-only
prints its completion message, delaying the caller's systemd registration.
This tests that boundary, not a real Oracle VM or systemd installation.
"""

import argparse
import json
import os
from pathlib import Path
import queue
import re
import shutil
import signal
import shlex
import subprocess
import tempfile
import threading
import time


ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "ops/full-package-posix/start-full-posix.sh"
GUARD_SECONDS = 4
OUTPUT_CLOSE_SECONDS = 1


def production_function(source, name):
    # Execute the current function verbatim; do not maintain a test-only copy.
    match = re.search(r"^" + re.escape(name) + r"\(\) \{\n.*?^\}", source, re.M | re.S)
    if match is None:
        raise RuntimeError(f"production function not found: {name}")
    return match.group(0)


def check_case(shell, functions, wrapped, expected_status):
    child = f"sleep 0.1; echo external-command-finished; exit {expected_status}"
    command = ("run_external " if wrapped else "") + "sh -c '" + child + "'"
    program = (
        "set -eu\nPATH=/usr/bin:/bin:$PATH\nexport PATH\n"
        + functions
        + f"\nEXTERNAL_OPERATION_TIMEOUT_SECONDS={GUARD_SECONDS}\n"
        + f"if {command}; then actual_status=0; else actual_status=$?; fi\n"
        + "printf 'bootstrap-return:%s\\n' \"$actual_status\"\n"
    )
    process = subprocess.Popen(
        [shell, "-c", program], stdin=subprocess.DEVNULL,
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
        start_new_session=(os.name == "posix"),
    )
    events = queue.Queue()
    output = []

    def read_output():
        for raw in iter(process.stdout.readline, b""):
            line = raw.decode("utf-8", errors="replace").rstrip()
            output.append(line)
            events.put((line, time.monotonic()))
        events.put((None, time.monotonic()))

    reader = threading.Thread(target=read_output, daemon=True)
    reader.start()
    returned_at = closed_at = None
    returned_status = None
    deadline = time.monotonic() + GUARD_SECONDS + 10
    try:
        while time.monotonic() < deadline:
            line, observed_at = events.get(timeout=max(0.01, deadline - time.monotonic()))
            if line is None:
                closed_at = observed_at
                break
            if line.startswith("bootstrap-return:"):
                returned_at = observed_at
                returned_status = int(line.split(":", 1)[1])
        process.wait(timeout=2)
    except (queue.Empty, subprocess.TimeoutExpired):
        pass
    finally:
        if process.poll() is None or reader.is_alive():
            if os.name == "posix":
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
            elif process.poll() is None:
                process.kill()
        process.wait(timeout=GUARD_SECONDS + 2)
        reader.join(timeout=GUARD_SECONDS + 2)
        process.stdout.close()

    close_delay = None if returned_at is None or closed_at is None else closed_at - returned_at
    passed = (
        "external-command-finished" in output
        and returned_status == expected_status and process.returncode == 0
        and close_delay is not None and close_delay < OUTPUT_CLOSE_SECONDS
    )
    result = {
        "case": ("production_run_external" if wrapped else "control") + f"_exit_{expected_status}",
        "passed": passed,
        "returned_status": returned_status,
        "output_close_delay_seconds": None if close_delay is None else round(close_delay, 3),
        "guard_seconds": GUARD_SECONDS,
    }
    print(json.dumps(result), flush=True)
    if not passed:
        print("POSIX_BOOTSTRAP_OUTPUT_NOT_RELEASED: output must close after the command returns, "
              "without waiting for its unused timeout. Review the 4.5 Linux install blocker.", flush=True)
        print("\n".join(output), flush=True)
    return passed


def shell_path(shell, path):
    if os.name != "nt":
        return str(path)
    return subprocess.check_output(
        [shell, "-c", 'PATH=/usr/bin:/bin:$PATH; cygpath -u "$1"', "path", str(path)],
        text=True,
    ).strip()


def check_lifecycle_case(shell, source, case, capture=False):
    """Run production cleanup/traps and real sleep; observe only our children.

    The sleep wrapper records its PID then execs the real program. It does not
    fake cancellation or timeout. Diagnostics use the actual FIFO/tee functions.
    """
    names = ("die", "run_external", "cleanup_updater_runner",
             "stop_lifetime_watchdog", "cleanup", "shutdown_from_signal")
    functions = "\n".join(production_function(source, name) for name in names)
    bootstrap = source[:source.index("REQUESTED_RUNTIME_PROFILE=")]
    traps = "\n".join(re.findall(r"^trap .*", bootstrap, re.M))
    diagnostics = (ROOT / "ops/full-package-posix/diagnostics.sh").read_text(encoding="utf-8")
    signal_number = {"hup": 1, "interrupt": 2, "terminate": 15}.get(case)
    expected = 128 + signal_number if signal_number else {"error": 23, "timeout": 124}.get(case, 0)
    with tempfile.TemporaryDirectory(prefix="archive-center-bootstrap-") as tmp:
        root = Path(tmp)
        bin_dir = root / "bin"
        bin_dir.mkdir()
        posix_root = shell_path(shell, root)
        real_sleep = subprocess.check_output(
            [shell, "-c", "PATH=/usr/bin:/bin:$PATH; command -v sleep"], text=True,
        ).strip()
        sleep_path = bin_dir / "sleep"
        sleep_path.write_text(
            '#!/bin/sh\nprintf "%s\\n" "$$" >> "$AC_TEST_CHILD_PIDS"\n'
            'printf "sleep-started:%s\\n" "$1"\nexec ' + shlex.quote(real_sleep) + ' "$@"\n',
            encoding="utf-8", newline="\n",
        )
        sleep_path.chmod(0o755)
        if signal_number or case == "timeout":
            child = 'printf "command-stdout\\n"; printf "command-stderr\\n" >&2; exec sleep 8'
        else:
            child = ('printf "command-stdout\\n"; printf "command-stderr\\n" >&2; '
                     'sleep 0.1; exit ' + str(expected))
        command = "run_external " + shlex.quote(shell_path(shell, Path(shell))) + " -c " + shlex.quote(child)
        if case == "repeat":
            # Production callers are external executables, not shell builtins.
            command = ('i=0; while [ "$i" -lt 10 ]; do run_external '
                       + shlex.quote(shell_path(shell, Path(shell)))
                       + ' -c "exit 0" || exit $?; i=$((i + 1)); done')
        if case == "immediate":
            command = 'i=0; while [ "$i" -lt 10 ]; do run_external true || exit $?; i=$((i + 1)); done'
        program = (
            "set -eu\nPATH=" + shlex.quote(posix_root + "/bin") + ":/usr/bin:/bin:$PATH\nexport PATH\n"
            + "AC_TEST_CHILD_PIDS=" + shlex.quote(posix_root + "/children") + "\nexport AC_TEST_CHILD_PIDS\n"
            + "TMPDIR=" + shlex.quote(posix_root) + "\nexport TMPDIR\n"
            + "AC_LOG_DIR=" + shlex.quote(posix_root + "/logs") + "\n"
            + functions + "\n" + diagnostics + "\n" + traps + "\n"
            + ("start_diagnostic_capture\n" if capture else "")
            + f"EXTERNAL_OPERATION_TIMEOUT_SECONDS={GUARD_SECONDS}\n"
            + 'printf "launcher-pid:%s\\n" "$$"\n'
            + f"if {command}; then status=0; else status=$?; fi\n"
            + 'printf "bootstrap-return:%s\\n" "$status"\nexit "$status"\n'
        )
        process = subprocess.Popen([shell, "-c", program], stdin=subprocess.DEVNULL,
                                   stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                   start_new_session=(os.name == "posix"))
        events = queue.Queue()
        output = []

        def reader():
            for raw in iter(process.stdout.readline, b""):
                line = raw.decode("utf-8", errors="replace").rstrip()
                output.append(line)
                events.put(line)
            events.put(None)

        thread = threading.Thread(target=reader, daemon=True)
        thread.start()
        start = time.monotonic()
        launcher_pid = None
        signal_sent = False
        closed = False
        deadline = start + GUARD_SECONDS + 8
        try:
            while time.monotonic() < deadline:
                line = events.get(timeout=max(0.01, deadline - time.monotonic()))
                if line is None:
                    closed = True
                    break
                if line.startswith("launcher-pid:"):
                    launcher_pid = int(line.split(":", 1)[1])
                if signal_number and not signal_sent and line == f"sleep-started:{GUARD_SECONDS}":
                    subprocess.run([shell, "-c", f'kill -{signal_number} "$1"',
                                    "cancel", str(launcher_pid)], check=True)
                    signal_sent = True
            process.wait(timeout=2)
        except (queue.Empty, subprocess.TimeoutExpired):
            pass
        elapsed = time.monotonic() - start
        child_file = root / "children"
        child_pids = child_file.read_text().split() if child_file.exists() else []
        live = subprocess.check_output(
            [shell, "-c", 'for pid do if kill -0 "$pid" 2>/dev/null; then printf "%s\\n" "$pid"; fi; done',
             "inspect", *child_pids], text=True,
        ).split()
        # Failure cleanup targets only PIDs recorded by this isolated test.
        if live:
            subprocess.run([shell, "-c", 'for pid do kill -KILL "$pid" 2>/dev/null || true; done',
                            "cleanup", *live], check=True)
        if process.poll() is None:
            if os.name == "posix":
                os.killpg(process.pid, signal.SIGKILL)
            else:
                process.kill()
        process.wait(timeout=3)
        thread.join(timeout=3)
        # Do not deadlock closing a buffered pipe while a failed-case reader
        # still holds its lock. The daemon reader is diagnostic, not a service.
        if not thread.is_alive():
            process.stdout.close()
        stdout_ok = case in ("repeat", "immediate") or "command-stdout" in output
        stderr_ok = case in ("repeat", "immediate") or "command-stderr" in output
        log_ok = True
        if capture:
            log = root / "logs/launcher.log"
            log_text = log.read_text(encoding="utf-8") if log.exists() else ""
            log_ok = ("command-stdout" in log_text and "command-stderr" in log_text
                      and (expected == 0 or f"exit code {expected}" in log_text))
        expected_duration = GUARD_SECONDS + 2 if case in ("timeout", "repeat", "immediate") else GUARD_SECONDS
        passed = (closed and process.returncode == expected and not live
                  and elapsed < expected_duration and stdout_ok and stderr_ok and log_ok
                  and (not signal_number or signal_sent)
                  and not list(root.glob("archive-center-external-timeout.*")))
        print(json.dumps({"case": case, "capture": capture, "passed": passed,
                          "exit_code": process.returncode, "elapsed_seconds": round(elapsed, 3),
                          "remaining_children": live, "log_preserved": log_ok if capture else None}), flush=True)
        if not passed:
            print("\n".join(output), flush=True)
            if capture:
                print("captured-log:" + repr(log_text), flush=True)
        return passed


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--shell", help="POSIX shell to test (e.g. dash or bash)")
    parser.add_argument("--source", type=Path, default=SOURCE, help="launcher source, also used for broken-code controls")
    parser.add_argument("--skip-fifo", action="store_true", help="explicitly omit native FIFO checks on unsupported hosts")
    parser.add_argument("--case", help="run one lifecycle case while investigating a failure")
    args = parser.parse_args()
    shell = args.shell or shutil.which("sh")
    if shell is None and os.name == "nt":
        candidate = Path(os.environ.get("ProgramFiles", "C:/Program Files")) / "Git/usr/bin/sh.exe"
        if candidate.is_file():
            shell = str(candidate)
    if shell is None:
        raise RuntimeError("a POSIX shell is required; this check must not silently skip")
    source = args.source.read_text(encoding="utf-8")
    functions = "\n".join(production_function(source, name) for name in ("die", "run_external"))
    results = [] if args.case else [check_case(shell, functions, wrapped, status)
               for wrapped, status in ((False, 0), (True, 0), (True, 23))]
    cases = (args.case,) if args.case else ("normal", "error", "timeout", "hup", "interrupt", "terminate", "repeat", "immediate")
    results.extend(check_lifecycle_case(shell, source, case) for case in cases)
    if args.skip_fifo:
        print(json.dumps({"case": "diagnostic_fifo", "status": "explicitly_unverified"}), flush=True)
    else:
        capture_cases = (args.case,) if args.case else ("normal", "error", "timeout", "terminate")
        results.extend(check_lifecycle_case(shell, source, case, capture=True) for case in capture_cases)
    return 0 if all(results) else 1


if __name__ == "__main__":
    raise SystemExit(main())
