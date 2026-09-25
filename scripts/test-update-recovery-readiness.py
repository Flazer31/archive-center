"""Exercise the production launcher readiness functions against local HTTP.

Only sleeping is accelerated. The launcher still makes real HTTP requests and
parses real 503 bodies. No user installation, DB, updater or provider is used.
"""
import argparse
import http.server
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import threading

ROOT = Path(__file__).resolve().parents[1]
state = {"case": "healthy", "polls": 0}


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_GET(self):
        if self.path == "/version":
            payload = {"version": "wrong" if state["case"] == "wrong_version" else "4.7.0"}
            code = 200
        elif self.path == "/ready":
            state["polls"] += 1
            case = state["case"]
            recovery = None
            if case in ("running", "waiting") and state["polls"] <= 65:
                recovery = case
            elif case == "failed":
                recovery = "failed"
            ready = recovery is None and case != "unrelated_error"
            payload = {"ready": ready, "checks": {"chromadb_recovery": recovery} if recovery else {}}
            code = 200 if ready else 503
        else:
            self.send_error(404)
            return
        data = json.dumps(payload).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


def ps_quote(text):
    return "'" + str(text).replace("'", "''") + "'"


def shell_function(text, name):
    match = re.search(r"^" + re.escape(name) + r"\(\) \{\n.*?^\}", text, re.M | re.S)
    if not match:
        raise RuntimeError("Missing production function: " + name)
    return match.group(0)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--launcher-root", type=Path, default=ROOT)
    parser.add_argument("--expect-broken", action="store_true", help="negative control against pre-fix launchers")
    args = parser.parse_args()
    ps = shutil.which("powershell.exe") or shutil.which("pwsh")
    bash = shutil.which("bash")
    if os.name == "nt" and Path(r"C:\Program Files\Git\bin\bash.exe").exists():
        bash = r"C:\Program Files\Git\bin\bash.exe"
    if not ps or not bash:
        raise RuntimeError("Both PowerShell and bash are required for this cross-launcher contract test")
    windows = args.launcher_root / "ops/full-package/scripts/start-full-windows.ps1"
    posix = args.launcher_root / "ops/full-package-posix/start-full-posix.sh"
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    results = []
    try:
        with tempfile.TemporaryDirectory(prefix="ac-recovery-readiness-") as tmp:
            for shell in ("windows", "posix"):
                for case in ("healthy", "running", "waiting", "failed", "unrelated_error", "wrong_version", "exited"):
                    state.update(case=case, polls=0)
                    expected = case in ("healthy", "running", "waiting")
                    if args.expect_broken and case in ("running", "waiting"):
                        expected = False
                    if shell == "windows":
                        script = Path(tmp) / "probe.ps1"
                        # Parse the production file without executing installation code.
                        script.write_text("\n".join([
                            "$ErrorActionPreference = 'Stop'",
                            "$tokens=$null; $errors=$null",
                            "$ast=[System.Management.Automation.Language.Parser]::ParseFile(" + ps_quote(windows) + ",[ref]$tokens,[ref]$errors)",
                            "if ($errors.Count) { throw 'parse failure' }",
                            "$owner=$ast.Find({param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $n.Name -eq 'Wait-BackendMainReady'},$true)",
                            "Invoke-Expression $owner.Extent.Text",
                            "function Start-Sleep { param($Seconds) }",
                            "$process=Get-Process -Id $PID" if case != "exited" else "$process=Start-Process -WindowStyle Hidden -FilePath cmd.exe -ArgumentList '/c exit 1' -PassThru; $process.WaitForExit()",
                            "$result=Wait-BackendMainReady -Process $process -Port " + str(server.server_port) + " -ExpectedVersion '4.7.0' -TimeoutSeconds 60",
                            "$result | ConvertTo-Json -Compress",
                            "if ($result.Ready -ne $" + str(expected).lower() + ") { exit 2 }",
                        ]), encoding="utf-8-sig")
                        command = [ps, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", str(script)]
                    else:
                        source = posix.read_text(encoding="utf-8-sig")
                        library = "\n".join(shell_function(source, name) for name in (
                            "json_string_field", "json_bool_field", "readiness_polling_enabled", "wait_candidate_backend_ready"))
                        script = Path(tmp) / "probe.sh"
                        # Counter-based date advances only on failed polling, avoiding
                        # a minute-long test while retaining the production deadline.
                        counter = Path(tmp) / "clock"
                        counter.write_text("0")
                        path = counter.as_posix()
                        if os.name == "nt":
                            path = "/" + path[0].lower() + path[2:]
                        script.write_text("\n".join([
                            "#!/usr/bin/env sh", "set -eu", library,
                            "log() { printf '%s\\n' \"$*\"; }", "die() { printf '%s\\n' \"$*\"; exit 3; }",
                            "sleep() { :; }",
                            "date() { n=$(cat '" + path + "'); n=$((n+1)); printf '%s' \"$n\" >'" + path + "'; printf '%s\\n' \"$n\"; }",
                            "READINESS_TIMEOUT_SECONDS=60", "READINESS_POLL_INTERVAL_SECONDS=1", "REQUEST_TIMEOUT_SECONDS=2",
                            "AC_BIND_ADDR=127.0.0.1:" + str(server.server_port),
                            "probe_pid=$$" if case != "exited" else "probe_pid=99999999",
                            "if wait_candidate_backend_ready \"$probe_pid\" 4.7.0; then result=true; else result=false; fi",
                            "printf 'ready=%s\\n' \"$result\"",
                            "[ \"$result\" = " + str(expected).lower() + " ]",
                        ]), encoding="utf-8", newline="\n")
                        script_path = script.as_posix()
                        if os.name == "nt":
                            script_path = "/" + script_path[0].lower() + script_path[2:]
                        command = [bash, "-lc", "sh '" + script_path + "'"]
                    completed = subprocess.run(command, capture_output=True, text=True, timeout=55)
                    if completed.returncode:
                        raise AssertionError(f"{shell}/{case}: {completed.stdout}\n{completed.stderr}")
                    if case in ("running", "waiting") and not args.expect_broken and state["polls"] <= 65:
                        raise AssertionError("recovery was accepted before readiness became true")
                    results.append({"launcher": shell, "case": case, "ready": expected, "http_ready_polls": state["polls"]})
    finally:
        server.shutdown()
        server.server_close()
    print(json.dumps({"negative_control": args.expect_broken, "cases": results}, indent=2))


if __name__ == "__main__":
    main()
