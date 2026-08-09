#!/usr/bin/env sh
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TEST_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/archive-center-simple-install-test.XXXXXX")

cleanup() {
	case "$TEST_ROOT" in
		"${TMPDIR:-/tmp}"/archive-center-simple-install-test.*)
			rm -rf -- "$TEST_ROOT"
			;;
	esac
}
trap cleanup EXIT INT TERM

fail() {
	printf 'simple fresh-install test failed: %s\n' "$*" >&2
	exit 1
}

snapshot_tree() {
	root=$1
	{
		find "$root" -mindepth 1 -type d -print | sort
		find "$root" -type f -exec ls -l {} \; | sort
		find "$root" -type f -exec cksum {} \; | sort
	}
}

FAKE_BIN="$TEST_ROOT/bin"
mkdir -p "$FAKE_BIN"

cat > "$FAKE_BIN/uname" <<'EOF'
#!/usr/bin/env sh
if [ "${1:-}" = "-s" ]; then
	printf '%s\n' "${AC_TEST_UNAME_S:-Darwin}"
else
	printf '%s\n' "${AC_TEST_UNAME_M:-x86_64}"
fi
EOF

for name in python3 unzip sha256sum; do
	cat > "$FAKE_BIN/$name" <<'EOF'
#!/usr/bin/env sh
exit 0
EOF
	chmod +x "$FAKE_BIN/$name"
done

cat > "$FAKE_BIN/curl" <<'EOF'
#!/usr/bin/env sh
set -eu
: "${AC_TEST_CURL_LOG:?}"
: "${AC_TEST_HELPER_LOG:?}"
out=""
url=""
while [ "$#" -gt 0 ]; do
	case "$1" in
		-o)
			shift
			out=$1
			;;
		http://*|https://*) url=$1 ;;
	esac
	shift
done
[ -n "$out" ] || exit 91
printf '%s\n' "$url" >> "$AC_TEST_CURL_LOG"
cat > "$out" <<'HELPER'
#!/usr/bin/env sh
set -eu
if [ "${AC_TEST_HELPER_FAIL:-0}" = "1" ]; then
	exit 42
fi
{
	printf 'timeout=%s\n' "${AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS:-}"
	for arg in "$@"; do
		printf 'arg=%s\n' "$arg"
	done
} > "$AC_TEST_HELPER_LOG"
install_dir=""
while [ "$#" -gt 0 ]; do
	if [ "$1" = "--install-dir" ]; then
		shift
		install_dir=$1
	fi
	shift
done
[ -n "$install_dir" ] || exit 43
mkdir -p "$install_dir/current/scripts"
for launcher in \
	"$install_dir/current/start-archive-center-linux.sh" \
	"$install_dir/current/scripts/start-full-macos.sh" \
	"$install_dir/current/install-and-start-termux.sh"; do
	cat > "$launcher" <<'STARTER'
#!/usr/bin/env sh
{
	printf 'started\n'
	printf 'data_root=%s\n' "${ARCHIVE_CENTER_DATA_DIR:-}"
	printf 'external_timeout=%s\n' "${AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS:-}"
	printf 'request_timeout=%s\n' "${AC_REQUEST_TIMEOUT_SECONDS:-}"
	printf 'readiness_timeout=%s\n' "${AC_READINESS_TIMEOUT_SECONDS:-}"
	printf 'readiness_poll_interval=%s\n' "${AC_READINESS_POLL_INTERVAL_SECONDS:-}"
} > "$AC_TEST_START_LOG"
STARTER
done
printf '%s\n' "$install_dir/data" > "$install_dir/data-root.txt"
cat > "$install_dir/start-archive-center.sh" <<'STABLE'
#!/usr/bin/env sh
set -eu
INSTALL_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
IFS= read -r ARCHIVE_CENTER_DATA_DIR < "$INSTALL_ROOT/data-root.txt"
export ARCHIVE_CENTER_DATA_DIR
exec sh "$INSTALL_ROOT/current/scripts/start-full-macos.sh" "$@"
STABLE
chmod +x "$install_dir/start-archive-center.sh"
HELPER
EOF
chmod +x "$FAKE_BIN/uname" "$FAKE_BIN/curl"

# Existing installs must stop before dependency preparation or network access.
EXISTING_HOME="$TEST_ROOT/existing-home"
mkdir -p "$EXISTING_HOME/.archive-center"
printf 'preserve-me\n' > "$EXISTING_HOME/.archive-center/sentinel.txt"
mkdir -p "$EXISTING_HOME/.archive-center/nested"
printf 'nested-preserve\n' > "$EXISTING_HOME/.archive-center/nested/value.txt"
EXISTING_SNAPSHOT_BEFORE=$(snapshot_tree "$EXISTING_HOME/.archive-center")
EXISTING_CURL_LOG="$TEST_ROOT/existing-curl.log"
if HOME="$EXISTING_HOME" \
	PATH="$FAKE_BIN:$PATH" \
	AC_TEST_CURL_LOG="$EXISTING_CURL_LOG" \
	AC_TEST_HELPER_LOG="$TEST_ROOT/existing-helper.log" \
	sh "$REPO_ROOT/install.sh" >"$TEST_ROOT/existing.out" 2>"$TEST_ROOT/existing.err"; then
	fail "existing install was accepted"
fi
[ ! -e "$EXISTING_CURL_LOG" ] || fail "existing install reached curl"
[ ! -e "$TEST_ROOT/existing-helper.log" ] || fail "existing install reached the release helper"
grep -q 'fresh install only' "$TEST_ROOT/existing.err" || fail "existing install error was not explicit"
EXISTING_SNAPSHOT_AFTER=$(snapshot_tree "$EXISTING_HOME/.archive-center")
[ "$EXISTING_SNAPSHOT_AFTER" = "$EXISTING_SNAPSHOT_BEFORE" ] || fail "existing install tree changed"

# A clean install must call the existing release helper with all normal defaults.
CLEAN_HOME="$TEST_ROOT/clean-home"
mkdir -p "$CLEAN_HOME"
CLEAN_CURL_LOG="$TEST_ROOT/clean-curl.log"
CLEAN_HELPER_LOG="$TEST_ROOT/clean-helper.log"
CLEAN_START_LOG="$TEST_ROOT/clean-start.log"
HOME="$CLEAN_HOME" \
	PATH="$FAKE_BIN:$PATH" \
	AC_TEST_CURL_LOG="$CLEAN_CURL_LOG" \
	AC_TEST_HELPER_LOG="$CLEAN_HELPER_LOG" \
	AC_TEST_START_LOG="$CLEAN_START_LOG" \
	sh "$REPO_ROOT/install.sh"

grep -Fxq 'https://raw.githubusercontent.com/Flazer31/archive-center/main/scripts/install-github-release.sh' "$CLEAN_CURL_LOG" || fail "wrong POSIX helper URL"
grep -Fxq 'timeout=1800' "$CLEAN_HELPER_LOG" || fail "default timeout was not passed"
grep -Fxq "arg=$CLEAN_HOME/.archive-center" "$CLEAN_HELPER_LOG" || fail "normal install directory was not passed"
if grep -Fxq 'arg=--start' "$CLEAN_HELPER_LOG"; then
	fail "foreground helper call unexpectedly owned package start"
fi
grep -Fxq 'started' "$CLEAN_START_LOG" || fail "fresh package was not started"
grep -Fxq "data_root=$CLEAN_HOME/.archive-center/data" "$CLEAN_START_LOG" || fail "stable data root was not passed to the package launcher"
grep -Fxq 'external_timeout=1800' "$CLEAN_START_LOG" || fail "external-operation timeout was not passed to the package launcher"
grep -Fxq 'request_timeout=30' "$CLEAN_START_LOG" || fail "request timeout was not passed to the package launcher"
grep -Fxq 'readiness_timeout=180' "$CLEAN_START_LOG" || fail "readiness timeout was not passed to the package launcher"
grep -Fxq 'readiness_poll_interval=1' "$CLEAN_START_LOG" || fail "readiness poll interval was not passed to the package launcher"

# A failed helper must remove only the root reserved by this fresh attempt so a retry is possible.
FAILED_HOME="$TEST_ROOT/failed-home"
mkdir -p "$FAILED_HOME"
if HOME="$FAILED_HOME" \
	PATH="$FAKE_BIN:$PATH" \
	AC_TEST_CURL_LOG="$TEST_ROOT/failed-curl.log" \
	AC_TEST_HELPER_LOG="$TEST_ROOT/failed-helper.log" \
	AC_TEST_START_LOG="$TEST_ROOT/failed-start.log" \
	AC_TEST_HELPER_FAIL=1 \
	sh "$REPO_ROOT/install.sh" >"$TEST_ROOT/failed.out" 2>"$TEST_ROOT/failed.err"; then
	fail "failed helper was reported as success"
fi
[ ! -e "$FAILED_HOME/.archive-center" ] || fail "failed helper left the reserved install root behind"

# The production POSIX release helper must persist one install-level data root
# across a later launcher execution after the managed package changes.
PRODUCTION_BIN="$TEST_ROOT/production-helper-bin"
PRODUCTION_INSTALL="$TEST_ROOT/production-install"
PRODUCTION_START_LOG="$TEST_ROOT/production-start.log"
mkdir -p "$PRODUCTION_BIN"
REAL_PYTHON3=${ARCHIVE_CENTER_TEST_PYTHON3:-$(command -v python3 2>/dev/null || true)}
[ -n "$REAL_PYTHON3" ] || fail "python3 is required for the production release-helper contract"
cat > "$PRODUCTION_BIN/python3" <<EOF
#!/usr/bin/env sh
exec "$REAL_PYTHON3" "\$@"
EOF
cat > "$PRODUCTION_BIN/uname" <<'EOF'
#!/usr/bin/env sh
case "${1:-}" in
	-s) printf 'Darwin\n' ;;
	-m) printf 'x86_64\n' ;;
	*) printf 'Darwin\n' ;;
esac
EOF
cat > "$PRODUCTION_BIN/curl" <<'EOF'
#!/usr/bin/env sh
set -eu
out=""
url=""
while [ "$#" -gt 0 ]; do
	case "$1" in
		-o) shift; out=$1 ;;
		http://*|https://*) url=$1 ;;
	esac
	shift
done
[ -n "$out" ] || exit 91
case "$url" in
	*/releases/latest)
		cat > "$out" <<'JSON'
{"tag_name":"v3.9.0-feedback","assets":[{"name":"Archive Center 3.9.0 macOS Intel Auto Install Package.zip","browser_download_url":"https://fixture.invalid/package.zip"},{"name":"SHA256SUMS.txt","browser_download_url":"https://fixture.invalid/SHA256SUMS.txt"}]}
JSON
		;;
	*/SHA256SUMS.txt)
		printf '%s  Archive Center 3.9.0 macOS Intel Auto Install Package.zip\n' 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' > "$out"
		;;
	*/package.zip)
		printf 'fixture-package\n' > "$out"
		;;
	*) exit 92 ;;
esac
EOF
cat > "$PRODUCTION_BIN/sha256sum" <<'EOF'
#!/usr/bin/env sh
printf '%s  %s\n' 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' "$1"
EOF
cat > "$PRODUCTION_BIN/unzip" <<'EOF'
#!/usr/bin/env sh
set -eu
destination=""
while [ "$#" -gt 0 ]; do
	case "$1" in
		-d) shift; destination=$1 ;;
	esac
	shift
done
[ -n "$destination" ] || exit 93
package_root="$destination/package"
mkdir -p "$package_root/scripts"
cat > "$package_root/Start Archive Center macOS.command" <<'LAUNCHER'
#!/usr/bin/env sh
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
exec sh "$SCRIPT_DIR/scripts/start-full-macos.sh" --profile "full_local" --vector-mode "local_native" "$@"
LAUNCHER
cat > "$package_root/scripts/start-full-macos.sh" <<'LAUNCHER'
#!/usr/bin/env sh
printf 'version=first\ndata_root=%s\nargs=%s\n' "${ARCHIVE_CENTER_DATA_DIR:-}" "$*" > "$AC_TEST_PRODUCTION_START_LOG"
LAUNCHER
chmod +x "$package_root/Start Archive Center macOS.command" "$package_root/scripts/start-full-macos.sh"
EOF
chmod +x "$PRODUCTION_BIN/python3" "$PRODUCTION_BIN/uname" "$PRODUCTION_BIN/curl" "$PRODUCTION_BIN/sha256sum" "$PRODUCTION_BIN/unzip"

PATH="$PRODUCTION_BIN:$PATH" \
	AC_TEST_PRODUCTION_START_LOG="$PRODUCTION_START_LOG" \
	AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS=1800 \
	sh "$REPO_ROOT/scripts/install-github-release.sh" --install-dir "$PRODUCTION_INSTALL" --start
grep -Fxq "data_root=$PRODUCTION_INSTALL/data" "$PRODUCTION_START_LOG" || fail "production helper first start used a drifting data root"
grep -Fxq 'args=--profile full_local --vector-mode local_native' "$PRODUCTION_START_LOG" || fail "production helper bypassed the public macOS full_local launcher"
[ -x "$PRODUCTION_INSTALL/start-archive-center.sh" ] || fail "production helper did not create the stable installed launcher"
grep -Fxq "$PRODUCTION_INSTALL/data" "$PRODUCTION_INSTALL/data-root.txt" || fail "production helper did not persist the install-level data root"

cat > "$PRODUCTION_INSTALL/current/scripts/start-full-macos.sh" <<'EOF'
#!/usr/bin/env sh
printf 'version=updated\ndata_root=%s\nargs=%s\n' "${ARCHIVE_CENTER_DATA_DIR:-}" "$*" > "$AC_TEST_PRODUCTION_START_LOG"
EOF
chmod +x "$PRODUCTION_INSTALL/current/scripts/start-full-macos.sh"
unset ARCHIVE_CENTER_DATA_DIR
AC_TEST_PRODUCTION_START_LOG="$PRODUCTION_START_LOG" sh "$PRODUCTION_INSTALL/start-archive-center.sh"
grep -Fxq 'version=updated' "$PRODUCTION_START_LOG" || fail "stable installed launcher did not enter the updated package"
grep -Fxq "data_root=$PRODUCTION_INSTALL/data" "$PRODUCTION_START_LOG" || fail "updated package restart changed the database root"
grep -Fxq 'args=--profile full_local --vector-mode local_native' "$PRODUCTION_START_LOG" || fail "updated stable launcher bypassed the public macOS full_local launcher"

# Once MariaDB, Python, and the managed ChromaDB runtime are ready, every
# supported POSIX launcher must skip its OS package manager and pip bootstrap.
LAUNCHER_FIXTURE="$TEST_ROOT/launcher-fixture"
LAUNCHER_PACKAGE="$LAUNCHER_FIXTURE/package"
LAUNCHER_BIN="$LAUNCHER_FIXTURE/bin"
LAUNCHER_CALLS="$LAUNCHER_FIXTURE/package-manager-calls.log"
mkdir -p "$LAUNCHER_PACKAGE/bin" "$LAUNCHER_PACKAGE/migrations" "$LAUNCHER_BIN"
: > "$LAUNCHER_PACKAGE/bin/archive-center-go"
: > "$LAUNCHER_PACKAGE/bin/mariadb-schema"
: > "$LAUNCHER_PACKAGE/migrations/001_schema.sql"
chmod +x "$LAUNCHER_PACKAGE/bin/archive-center-go" "$LAUNCHER_PACKAGE/bin/mariadb-schema"

for name in mariadbd mariadb python3; do
	cat > "$LAUNCHER_BIN/$name" <<'EOF'
#!/usr/bin/env sh
exit 0
EOF
	chmod +x "$LAUNCHER_BIN/$name"
done
cat > "$LAUNCHER_BIN/id" <<'EOF'
#!/usr/bin/env sh
if [ "${1:-}" = "-u" ]; then
	printf '0\n'
	exit 0
fi
exit 1
EOF
cat > "$LAUNCHER_BIN/apt-get" <<'EOF'
#!/usr/bin/env sh
printf 'apt-get %s\n' "$*" >> "$AC_TEST_PACKAGE_MANAGER_CALLS"
exit 91
EOF
cat > "$LAUNCHER_BIN/brew" <<'EOF'
#!/usr/bin/env sh
if [ "${1:-}" = "shellenv" ]; then
	exit 0
fi
printf 'brew %s\n' "$*" >> "$AC_TEST_PACKAGE_MANAGER_CALLS"
exit 92
EOF
cat > "$LAUNCHER_BIN/pkg" <<'EOF'
#!/usr/bin/env sh
printf 'pkg %s\n' "$*" >> "$AC_TEST_PACKAGE_MANAGER_CALLS"
exit 93
EOF
cat > "$LAUNCHER_BIN/proot-distro" <<'EOF'
#!/usr/bin/env sh
exit 0
EOF
chmod +x "$LAUNCHER_BIN/id" "$LAUNCHER_BIN/apt-get" "$LAUNCHER_BIN/brew" "$LAUNCHER_BIN/pkg" "$LAUNCHER_BIN/proot-distro"

for platform in linux macos termux; do
	data_root="$LAUNCHER_FIXTURE/data-$platform"
	vector_mode=local_native
	if [ "$platform" = "termux" ]; then
		vector_mode=local_proot
	else
		mkdir -p "$data_root/chromadb-venv/bin"
		cat > "$data_root/chromadb-venv/bin/python" <<'EOF'
#!/usr/bin/env sh
if [ "${1:-}" = "-c" ]; then
	exit 0
fi
exit 94
EOF
		chmod +x "$data_root/chromadb-venv/bin/python"
	fi
	ARCHIVE_CENTER_PACKAGE_ROOT="$LAUNCHER_PACKAGE" \
		ARCHIVE_CENTER_DATA_DIR="$data_root" \
		AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS=5 \
		AC_TEST_PACKAGE_MANAGER_CALLS="$LAUNCHER_CALLS" \
		PATH="$LAUNCHER_BIN:$PATH" \
		sh "$REPO_ROOT/ops/full-package-posix/start-full-posix.sh" \
			--platform "$platform" --profile full_local --vector-mode "$vector_mode" --install-only \
			> "$LAUNCHER_FIXTURE/$platform.out"
	grep -q 'runtime dependencies are already available' "$LAUNCHER_FIXTURE/$platform.out" || fail "$platform ready-runtime fast path was not reported"
done
[ ! -e "$LAUNCHER_CALLS" ] || fail "ready POSIX runtime called an OS package manager"

# The same production branch must still enter installation when a required
# managed ChromaDB runtime is absent.
missing_data="$LAUNCHER_FIXTURE/data-linux-missing"
if ARCHIVE_CENTER_PACKAGE_ROOT="$LAUNCHER_PACKAGE" \
	ARCHIVE_CENTER_DATA_DIR="$missing_data" \
	AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS=5 \
	AC_TEST_PACKAGE_MANAGER_CALLS="$LAUNCHER_CALLS" \
	PATH="$LAUNCHER_BIN:$PATH" \
	sh "$REPO_ROOT/ops/full-package-posix/start-full-posix.sh" \
		--platform linux --profile full_local --vector-mode local_native --install-only \
		> "$LAUNCHER_FIXTURE/linux-missing.out" 2> "$LAUNCHER_FIXTURE/linux-missing.err"; then
	fail "missing Linux runtime skipped dependency installation"
fi
grep -q '^apt-get update$' "$LAUNCHER_CALLS" || fail "missing Linux runtime did not enter the existing package-manager install path"

grep -Fxq 'curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | sh' "$REPO_ROOT/README.md" || fail "README POSIX command drifted"
grep -Fxq 'irm https://raw.githubusercontent.com/Flazer31/archive-center/main/install-windows.ps1 | iex' "$REPO_ROOT/README.md" || fail "README Windows command drifted"

printf 'simple POSIX fresh-install contract: ok\n'
