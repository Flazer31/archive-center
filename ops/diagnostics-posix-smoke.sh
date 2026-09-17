#!/usr/bin/env sh
set -eu
repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
test_root=$(mktemp -d)
trap 'rm -rf -- "$test_root"' EXIT
export AC_LOG_DIR="$test_root/logs"
export AC_DIAGNOSTIC_HELPER="$repo/ops/full-package-posix/diagnostics.sh"
cat >"$test_root/failure.sh" <<'EOF'
#!/usr/bin/env sh
set -eu
. "$AC_DIAGNOSTIC_HELPER"
start_diagnostic_capture
trap 'code=$?; finish_diagnostic_capture "$code"' EXIT
printf '{"status":"fixture"}\n'
printf 'fixture POSIX startup failure: password=private-value\n' >&2
exit 7
EOF
code=0
sh "$test_root/failure.sh" >"$test_root/out" 2>"$test_root/err" || code=$?
[ "$code" = 7 ] || { echo 'exit code changed'; exit 1; }
[ "$(cat "$test_root/out")" = '{"status":"fixture"}' ] || { echo 'stdout protocol changed'; exit 1; }
grep -q 'fixture POSIX startup failure' "$test_root/err"
grep -q 'fixture POSIX startup failure' "$AC_LOG_DIR/launcher.log"
mkdir -p "$test_root/package/scripts"
cp "$repo/ops/full-package-posix/export-diagnostics.py" "$test_root/package/scripts/"
cp "$repo/ops/full-package-posix/07_export_diagnostics.sh" "$test_root/package/"
sh "$test_root/package/07_export_diagnostics.sh" "$test_root/report.json"
python3 - "$test_root/report.json" <<'PY'
import json, sys
from pathlib import Path
data = Path(sys.argv[1]).read_text(encoding='utf-8')
report = json.loads(data)
assert report['collector'] == 'native_offline'
assert 'fixture POSIX startup failure' in data
assert 'private-value' not in data
assert '[REDACTED]' in data
PY
printf 'PASS: POSIX stderr/stdout preservation, exit code, durable log, native offline export without Go, redaction.\n'
