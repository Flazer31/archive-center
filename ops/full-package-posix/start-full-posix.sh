#!/usr/bin/env sh
set -eu

# Archive Center 2.1 managed POSIX package launcher.
#
# This is intentionally installer-managed: normal users should not have to
# manually install MariaDB or ChromaDB. The script uses the platform package
# manager when bundled POSIX runtimes are not present.

PACKAGE_BUILD_VERSION="__ARCHIVE_CENTER_PACKAGE_VERSION__"

usage() {
	cat <<'EOF'
Usage:
  start-full-posix.sh --platform linux|macos|termux [options]

Options:
  --platform NAME   Target platform profile.
  --profile NAME    Runtime profile: client_only, core_lite, vector_external,
                    vector_local_native, or full_local.
  --vector-mode NAME
                    Vector mode: off, fallback, external, local_native,
                    local_proot, or bundled.
  --preflight       Print a JSON preflight report and exit.
  --install-only    Install/bootstrap dependencies, then exit.
  --no-install      Do not use package managers; only use existing/bundled tools.
  --readiness-timeout-seconds N
                    Overall readiness bound. Must be supplied with poll interval.
  --readiness-poll-interval-seconds N
                    Poll interval. Must be supplied with readiness timeout.
  --request-timeout-seconds N
                    Caller-selected bound for each local readiness HTTP request.
  --external-operation-timeout-seconds N
                    Caller-selected bound for updater/install subprocesses.
  --help            Show this help.

Environment:
  ARCHIVE_CENTER_DATA_DIR      Runtime data directory. Defaults inside package.
  AC_RUNTIME_PROFILE           Defaults to core_lite.
  AC_VECTOR_MODE               Defaults from AC_RUNTIME_PROFILE.
  AC_BIND_ADDR                 Defaults to 0.0.0.0:28080.
  AC_CHROMA_ENDPOINT           Required for vector_external; defaults to local
                               only for local vector profiles.
  AC_MARIADB_PORT              Defaults to 3307.
  AC_READINESS_TIMEOUT_SECONDS Optional caller-supplied readiness bound.
  AC_READINESS_POLL_INTERVAL_SECONDS
                               Optional caller-supplied readiness poll interval.
  AC_REQUEST_TIMEOUT_SECONDS   Optional caller-supplied local HTTP bound.
  AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS
                               Optional caller-supplied subprocess bound.
EOF
}

log() {
	printf '%s\n' "$*"
}

die() {
	printf 'ERROR: %s\n' "$*" >&2
	exit 1
}

has_cmd() {
	command -v "$1" >/dev/null 2>&1
}

json_escape() {
	printf '%s' "$1" | tr '\n' ' ' | sed 's/\\/\\\\/g; s/"/\\"/g; s/	/\\t/g'
}

json_bool() {
	if [ "$1" = "true" ]; then
		printf 'true'
	else
		printf 'false'
	fi
}

find_file() {
	for candidate in "$@"; do
		if [ -n "$candidate" ] && [ -f "$candidate" ]; then
			printf '%s' "$candidate"
			return 0
		fi
	done
	return 1
}

find_executable() {
	for candidate in "$@"; do
		if [ -n "$candidate" ] && [ -x "$candidate" ]; then
			printf '%s' "$candidate"
			return 0
		fi
	done
	return 1
}

command_path() {
	if has_cmd "$1"; then
		command -v "$1"
		return 0
	fi
	return 1
}

canonical_dir() {
	if [ -n "$1" ] && [ -d "$1" ]; then
		(CDPATH= cd -- "$1" && pwd -P)
		return 0
	fi
	return 1
}

is_package_root() {
	[ -n "$1" ] || return 1
	[ -f "$1/migrations/001_schema.sql" ] || return 1
	[ -f "$1/bin/archive-center-go" ] || return 1
	[ -f "$1/bin/mariadb-schema" ] || return 1
	return 0
}

resolve_package_root() {
	for candidate in "$@"; do
		candidate_dir=$(canonical_dir "$candidate" 2>/dev/null || true)
		if is_package_root "$candidate_dir"; then
			printf '%s' "$candidate_dir"
			return 0
		fi
		if [ -n "$candidate_dir" ]; then
			for child in "$candidate_dir"/Archive\ Center\ 2.1* "$candidate_dir"/*Archive*Center*2.1* "$candidate_dir"/archivecenter2.1* "$candidate_dir"/*archive*center*2.1* "$candidate_dir"/Archive\ Center\ 2.0* "$candidate_dir"/*Archive*Center*2.0* "$candidate_dir"/archivecenter2.0* "$candidate_dir"/*archive*center*2.0*; do
				child_dir=$(canonical_dir "$child" 2>/dev/null || true)
				if is_package_root "$child_dir"; then
					printf '%s' "$child_dir"
					return 0
				fi
			done
		fi
	done
	return 1
}

run_sudo() {
	if [ "$(id -u 2>/dev/null || printf 1)" = "0" ]; then
		run_external "$@"
	elif has_cmd sudo; then
		run_external sudo "$@"
	else
		die "sudo is required to install packages on this platform"
	fi
}

run_external() {
	[ -n "${EXTERNAL_OPERATION_TIMEOUT_SECONDS:-}" ] || die "external operation requires --external-operation-timeout-seconds or AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS"
	timeout_marker=$(mktemp "${TMPDIR:-/tmp}/archive-center-external-timeout.XXXXXX")
	rm -f -- "$timeout_marker"
	EXTERNAL_TIMEOUT_MARKER=$timeout_marker
	export EXTERNAL_TIMEOUT_MARKER
	"$@" &
	EXTERNAL_OPERATION_PID=$!
	export EXTERNAL_OPERATION_PID
	(
		sleep "$EXTERNAL_OPERATION_TIMEOUT_SECONDS"
		if kill -0 "$EXTERNAL_OPERATION_PID" >/dev/null 2>&1; then
			: >"$timeout_marker"
			kill "$EXTERNAL_OPERATION_PID" >/dev/null 2>&1 || true
		fi
	) &
	external_guard_pid=$!
	EXTERNAL_GUARD_PID=$external_guard_pid
	export EXTERNAL_GUARD_PID
	if wait "$EXTERNAL_OPERATION_PID"; then
		external_status=0
	else
		external_status=$?
	fi
	EXTERNAL_OPERATION_PID=
	EXTERNAL_GUARD_PID=
	export EXTERNAL_OPERATION_PID
	export EXTERNAL_GUARD_PID
	kill "$external_guard_pid" >/dev/null 2>&1 || true
	wait "$external_guard_pid" >/dev/null 2>&1 || true
	if [ -f "$timeout_marker" ]; then
		rm -f -- "$timeout_marker"
		EXTERNAL_TIMEOUT_MARKER=
		export EXTERNAL_TIMEOUT_MARKER
		return 124
	fi
	EXTERNAL_TIMEOUT_MARKER=
	export EXTERNAL_TIMEOUT_MARKER
	return "$external_status"
}

run_captured_external() {
	output_path=$1
	shift
	run_external "$@" >"$output_path" 2>&1
}

port_is_open() {
	port=$1
	[ -n "${REQUEST_TIMEOUT_SECONDS:-}" ] || die "local port probes require --request-timeout-seconds or AC_REQUEST_TIMEOUT_SECONDS"
	python_for_probe=${PYTHON_BIN:-python3}
	"$python_for_probe" - "$port" "$REQUEST_TIMEOUT_SECONDS" >/dev/null 2>&1 <<'PY'
import socket
import sys
port = int(sys.argv[1])
s = socket.socket()
s.settimeout(float(sys.argv[2]))
try:
    s.connect(("127.0.0.1", port))
except OSError:
    sys.exit(1)
finally:
    s.close()
PY
}

readiness_polling_enabled() {
	[ -n "${READINESS_TIMEOUT_SECONDS:-}" ] && [ -n "${READINESS_POLL_INTERVAL_SECONDS:-}" ]
}

wait_port() {
	port=$1
	label=$2
	process_id=${3:-}
	if readiness_polling_enabled; then
		readiness_deadline=$(( $(date +%s) + READINESS_TIMEOUT_SECONDS ))
	else
		readiness_deadline=
	fi
	while :; do
		if [ -n "$process_id" ] && ! kill -0 "$process_id" >/dev/null 2>&1; then
			break
		fi
		if port_is_open "$port"; then
			return 0
		fi
		if ! readiness_polling_enabled; then
			break
		fi
		if [ "$(date +%s)" -ge "$readiness_deadline" ]; then
			break
		fi
		sleep "$READINESS_POLL_INTERVAL_SECONDS"
	done
	if [ "$label" = "MariaDB" ] && [ -f "${LOG_DIR:-}/mariadb.log" ]; then
		log "MariaDB log tail:"
		tail -n 80 "$LOG_DIR/mariadb.log" >&2 || true
	fi
	if [ "$label" = "ChromaDB" ] && [ -f "${LOG_DIR:-}/chromadb.err.log" ]; then
		log "ChromaDB error log tail:"
		tail -n 80 "$LOG_DIR/chromadb.err.log" >&2 || true
	fi
	die "$label did not become ready on 127.0.0.1:$port"
}

json_string_field() {
	field=$1
	printf '%s' "$2" | sed -n "s/.*\"$field\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" | head -n 1
}

json_bool_field() {
	field=$1
	printf '%s' "$2" | sed -n "s/.*\"$field\"[[:space:]]*:[[:space:]]*\(true\|false\).*/\1/p" | head -n 1
}

set_package_build_version() {
	case "$PACKAGE_BUILD_VERSION" in
		__ARCHIVE_CENTER_*) ;;
		*) AC_BUILD_VERSION=$PACKAGE_BUILD_VERSION ;;
	esac
	export AC_BUILD_VERSION
}

updater_safe_baseline_status() {
	case "$1" in
		no_pending|no_state|committed|rolled_back|nothing_to_rollback) return 0 ;;
		*) return 1 ;;
	esac
}

run_archive_updater() {
	action=$1
	UPDATER_OUTPUT=
	UPDATER_STATUS=
	UPDATER_CURRENT_VERSION=
	UPDATER_TARGET_VERSION=
	updater_capture="$EXEC_BIN_DIR/updater-$action-$$.log"
	if run_captured_external "$updater_capture" "$UPDATER_RUNNER" "$action" --root "$PACKAGE_ROOT"; then
		UPDATER_EXIT=0
	else
		UPDATER_EXIT=$?
	fi
	UPDATER_OUTPUT=$(cat "$updater_capture" 2>/dev/null || true)
	rm -f -- "$updater_capture"
	UPDATER_STATUS=$(json_string_field status "$UPDATER_OUTPUT")
	UPDATER_CURRENT_VERSION=$(json_string_field current_version "$UPDATER_OUTPUT")
	UPDATER_TARGET_VERSION=$(json_string_field target_version "$UPDATER_OUTPUT")
	return "$UPDATER_EXIT"
}

prepare_updater_runner() {
	UPDATER_RUNNER=
	updater_source="$PACKAGE_ROOT/bin/archive-center-updater"
	if [ ! -f "$PACKAGE_ROOT/.updates/pending-update.json" ] && [ ! -f "$PACKAGE_ROOT/.updates/update-state.json" ]; then
		return
	fi
	if [ ! -f "$updater_source" ]; then
		if [ -f "$PACKAGE_ROOT/.updates/pending-update.json" ] || [ -f "$PACKAGE_ROOT/.updates/update-state.json" ]; then
			die "pending update state exists but bin/archive-center-updater is missing"
		fi
		return
	fi
	mkdir -p "$EXEC_BIN_DIR"
	updater_runner_dir="$PACKAGE_ROOT/.updates/runner"
	mkdir -p "$updater_runner_dir"
	UPDATER_RUNNER="$updater_runner_dir/archive-center-updater-runner-$$"
	cp "$updater_source" "$UPDATER_RUNNER"
	chmod 700 "$UPDATER_RUNNER" 2>/dev/null || true
	[ -x "$UPDATER_RUNNER" ] || die "archive-center-updater runner is not executable: $UPDATER_RUNNER"
	export UPDATER_RUNNER
}

apply_pending_update() {
	PENDING_UPDATE_APPLIED=false
	PENDING_CURRENT_VERSION=
	PENDING_TARGET_VERSION=
	if [ -z "${UPDATER_RUNNER:-}" ]; then
		set_package_build_version
		return
	fi
	if ! run_archive_updater apply-pending; then
		apply_error=$UPDATER_OUTPUT
		if run_archive_updater status && updater_safe_baseline_status "$UPDATER_STATUS"; then
			log "Updater rejected the pending package before mutation; continuing the verified current package."
			set_package_build_version
			return
		fi
		die "updater apply-pending failed and a safe baseline could not be proven: $apply_error"
	fi
	case "$UPDATER_STATUS" in
		applied_pending_health)
			[ -n "$UPDATER_TARGET_VERSION" ] || die "updater applied a package without target_version"
			PENDING_UPDATE_APPLIED=true
			PENDING_CURRENT_VERSION=$UPDATER_CURRENT_VERSION
			PENDING_TARGET_VERSION=$UPDATER_TARGET_VERSION
			AC_BUILD_VERSION=$PENDING_TARGET_VERSION
			export PENDING_UPDATE_APPLIED PENDING_CURRENT_VERSION PENDING_TARGET_VERSION AC_BUILD_VERSION
			log "Applied a verified pending package. Main readiness will be checked before commit."
			;;
		no_pending|no_state|committed|rolled_back|nothing_to_rollback)
			set_package_build_version
			;;
		*)
			die "updater returned unsupported apply status: $UPDATER_STATUS"
			;;
	esac
}

stop_candidate_backend() {
	pid=${1:-}
	if [ -n "$pid" ] && kill -0 "$pid" >/dev/null 2>&1; then
		kill "$pid" >/dev/null 2>&1 || true
		wait "$pid" >/dev/null 2>&1 || true
	fi
}

wait_candidate_backend_ready() {
	pid=$1
	target=$2
	port=$(printf '%s' "$AC_BIND_ADDR" | sed -n 's/.*:\([0-9][0-9]*\)$/\1/p')
	[ -n "$port" ] || port=28080
	if readiness_polling_enabled; then
		readiness_deadline=$(( $(date +%s) + READINESS_TIMEOUT_SECONDS ))
	else
		readiness_deadline=
	fi
	while :; do
		if ! kill -0 "$pid" >/dev/null 2>&1; then
			return 1
		fi
		[ -n "${REQUEST_TIMEOUT_SECONDS:-}" ] || die "pending update health verification requires --request-timeout-seconds or AC_REQUEST_TIMEOUT_SECONDS"
		ready_body=$(curl --connect-timeout "$REQUEST_TIMEOUT_SECONDS" --max-time "$REQUEST_TIMEOUT_SECONDS" -fsS "http://127.0.0.1:$port/ready" 2>/dev/null || true)
		version_body=$(curl --connect-timeout "$REQUEST_TIMEOUT_SECONDS" --max-time "$REQUEST_TIMEOUT_SECONDS" -fsS "http://127.0.0.1:$port/version" 2>/dev/null || true)
		ready_status=$(json_bool_field ready "$ready_body")
		observed_version=$(json_string_field version "$version_body")
		if [ "$ready_status" = "true" ] && [ "$observed_version" = "$target" ]; then
			return 0
		fi
		if ! readiness_polling_enabled; then
			return 1
		fi
		if [ "$(date +%s)" -ge "$readiness_deadline" ]; then
			return 1
		fi
		sleep "$READINESS_POLL_INTERVAL_SECONDS"
	done
}

prepare_update_launcher_session() {
	[ -n "${PYTHON_BIN:-}" ] || die "managed update launcher session requires Python"
	launcher_token=$("$PYTHON_BIN" -c 'import secrets; print(secrets.token_hex(32))')
	[ "${#launcher_token}" -ge 32 ] || die "managed update launcher token generation failed"
	launcher_dir="$PACKAGE_ROOT/.updates"
	launcher_path="$launcher_dir/launcher-session.json"
	launcher_temp="$launcher_path.tmp.$$"
	mkdir -p "$launcher_dir"
	(umask 077 && printf '{"contract_version":"archive-center.update-launcher-session.v1","token":"%s"}\n' "$launcher_token" >"$launcher_temp")
	mv -f "$launcher_temp" "$launcher_path"
	AC_UPDATE_LAUNCHER_TOKEN=$launcher_token
	export AC_UPDATE_LAUNCHER_TOKEN
}

finalize_pending_update() {
	if [ "${PENDING_UPDATE_APPLIED:-false}" != "true" ]; then
		return
	fi
	mkdir -p "$LOG_DIR"
	prepare_update_launcher_session
	"$ARCHIVE_CENTER_GO_RUN" >"$LOG_DIR/update-candidate.out.log" 2>"$LOG_DIR/update-candidate.err.log" &
	candidate_pid=$!
	if wait_candidate_backend_ready "$candidate_pid" "$PENDING_TARGET_VERSION"; then
		if run_archive_updater commit && [ "$UPDATER_STATUS" = "committed" ]; then
			stop_candidate_backend "$candidate_pid"
			PACKAGE_BUILD_VERSION=$PENDING_TARGET_VERSION
			log "Pending Archive Center package committed after main readiness passed."
			return
		fi
		commit_error=$UPDATER_OUTPUT
		stop_candidate_backend "$candidate_pid"
		if run_archive_updater status && [ "$UPDATER_STATUS" = "committed" ]; then
			log "Update commit is durable; cleanup will be retried on the next start."
			return
		fi
		if ! run_archive_updater rollback || ! updater_safe_baseline_status "$UPDATER_STATUS"; then
			die "update commit failed ($commit_error) and rollback was not proven safe"
		fi
	else
		stop_candidate_backend "$candidate_pid"
		if ! run_archive_updater rollback || ! updater_safe_baseline_status "$UPDATER_STATUS"; then
			die "updated backend failed /ready or version verification and rollback was not proven safe"
		fi
	fi
	if [ -n "$PENDING_CURRENT_VERSION" ]; then
		PACKAGE_BUILD_VERSION=$PENDING_CURRENT_VERSION
		AC_BUILD_VERSION=$PENDING_CURRENT_VERSION
		export AC_BUILD_VERSION
		ROLLBACK_HEALTH_VERSION=$PENDING_CURRENT_VERSION
		export ROLLBACK_HEALTH_VERSION
	fi
	prepare_package_binaries
	log "Update was rolled back. Starting the verified previous backend."
}

rollback_pending_preparation_failure() {
	reason=$1
	if [ "${PENDING_UPDATE_APPLIED:-false}" != "true" ]; then
		return 1
	fi
	if ! run_archive_updater rollback || ! updater_safe_baseline_status "$UPDATER_STATUS"; then
		die "$reason; managed package rollback was not proven safe"
	fi
	if [ -n "$PENDING_CURRENT_VERSION" ]; then
		PACKAGE_BUILD_VERSION=$PENDING_CURRENT_VERSION
		AC_BUILD_VERSION=$PENDING_CURRENT_VERSION
		ROLLBACK_HEALTH_VERSION=$PENDING_CURRENT_VERSION
		export AC_BUILD_VERSION ROLLBACK_HEALTH_VERSION
	fi
	log "$reason Managed package files were rolled back; database files were preserved."
	return 0
}

cleanup_updater_runner() {
	if [ -n "${UPDATER_RUNNER:-}" ]; then
		rm -f -- "$UPDATER_RUNNER" >/dev/null 2>&1 || true
	fi
}

prepare_package_binaries() {
	MARIADB_SCHEMA_RUN="$PACKAGE_ROOT/bin/mariadb-schema"
	ARCHIVE_CENTER_GO_RUN="$PACKAGE_ROOT/bin/archive-center-go"

	[ -f "$MARIADB_SCHEMA_RUN" ] || die "missing packaged binary: $MARIADB_SCHEMA_RUN"
	[ -f "$ARCHIVE_CENTER_GO_RUN" ] || die "missing packaged binary: $ARCHIVE_CENTER_GO_RUN"

	if [ "$PLATFORM" = "termux" ]; then
		# Android shared storage such as /storage/emulated/0/Download often blocks
		# executing binaries even after chmod. Copy packaged Go tools into the
		# Termux-private runtime directory and execute them from there.
		mkdir -p "$EXEC_BIN_DIR"
		cp "$MARIADB_SCHEMA_RUN" "$EXEC_BIN_DIR/mariadb-schema"
		cp "$ARCHIVE_CENTER_GO_RUN" "$EXEC_BIN_DIR/archive-center-go"
		chmod 700 "$EXEC_BIN_DIR/mariadb-schema" "$EXEC_BIN_DIR/archive-center-go" 2>/dev/null || true
		MARIADB_SCHEMA_RUN="$EXEC_BIN_DIR/mariadb-schema"
		ARCHIVE_CENTER_GO_RUN="$EXEC_BIN_DIR/archive-center-go"
	else
		chmod +x "$MARIADB_SCHEMA_RUN" "$ARCHIVE_CENTER_GO_RUN" 2>/dev/null || true
	fi

	[ -x "$MARIADB_SCHEMA_RUN" ] || die "mariadb-schema is not executable: $MARIADB_SCHEMA_RUN"
	[ -x "$ARCHIVE_CENTER_GO_RUN" ] || die "archive-center-go is not executable: $ARCHIVE_CENTER_GO_RUN"
	export MARIADB_SCHEMA_RUN ARCHIVE_CENTER_GO_RUN
}

detect_arch() {
	case "$(uname -m 2>/dev/null || printf unknown)" in
		x86_64|amd64) printf 'amd64' ;;
		aarch64|arm64) printf 'arm64' ;;
		*) printf 'unknown' ;;
	esac
}

runtime_dependencies_ready() {
	find_executable \
		"$PACKAGE_ROOT/runtime/MariaDB/bin/mariadbd" \
		"$PACKAGE_ROOT/runtime/mariadb/bin/mariadbd" \
		"$(command_path mariadbd 2>/dev/null || true)" \
		"$(command_path mysqld 2>/dev/null || true)" >/dev/null 2>&1 || return 1
	find_executable \
		"$PACKAGE_ROOT/runtime/MariaDB/bin/mariadb" \
		"$PACKAGE_ROOT/runtime/mariadb/bin/mariadb" \
		"$(command_path mariadb 2>/dev/null || true)" \
		"$(command_path mysql 2>/dev/null || true)" >/dev/null 2>&1 || return 1
	if ! has_cmd python3 && ! has_cmd python; then
		return 1
	fi
	if ! local_chromadb_requested; then
		return 0
	fi
	if [ "$PLATFORM" = "termux" ]; then
		has_cmd proot-distro
		return
	fi
	managed_chroma_python="$RUNTIME_DIR/chromadb-venv/bin/python"
	[ -x "$managed_chroma_python" ] || return 1
	"$managed_chroma_python" -c 'from importlib.metadata import version; import chromadb; assert version("chromadb") == "1.5.9"' >/dev/null 2>&1
}

install_linux_deps() {
	if runtime_dependencies_ready; then
		log "Required Linux runtime dependencies are already available; skipping package manager."
		return
	fi
	if [ "$NO_INSTALL" = "true" ]; then
		return
	fi
	if has_cmd apt-get; then
		run_sudo apt-get update
		run_sudo apt-get install -y mariadb-server mariadb-client python3 python3-venv python3-pip curl ca-certificates
	elif has_cmd dnf; then
		run_sudo dnf install -y mariadb-server mariadb python3 python3-pip curl ca-certificates
	elif has_cmd yum; then
		run_sudo yum install -y mariadb-server mariadb python3 python3-pip curl ca-certificates
	elif has_cmd pacman; then
		run_sudo pacman -Sy --needed --noconfirm mariadb python python-pip curl ca-certificates
	elif has_cmd zypper; then
		run_sudo zypper install -y mariadb mariadb-client python3 python3-pip curl ca-certificates
	elif has_cmd apk; then
		run_sudo apk add --no-cache mariadb mariadb-client python3 py3-pip curl ca-certificates
	else
		die "No supported Linux package manager found"
	fi
}

find_brew_command() {
	if has_cmd brew; then
		command -v brew
		return 0
	fi
	for candidate in /opt/homebrew/bin/brew /usr/local/bin/brew "$HOME/.linuxbrew/bin/brew"; do
		if [ -x "$candidate" ]; then
			printf '%s' "$candidate"
			return 0
		fi
	done
	return 1
}

load_homebrew_env() {
	brew_candidate=$(find_brew_command || true)
	if [ -z "$brew_candidate" ]; then
		return 1
	fi
	if "$brew_candidate" shellenv >/dev/null 2>&1; then
		eval "$("$brew_candidate" shellenv)"
	fi
	BREW_BIN=$(command -v brew 2>/dev/null || printf '%s' "$brew_candidate")
	export BREW_BIN
	return 0
}

ensure_homebrew() {
	if load_homebrew_env; then
		return
	fi
	if ! has_cmd curl; then
		die "curl is required to bootstrap Homebrew automatically"
	fi
	if [ ! -x /bin/bash ]; then
		die "/bin/bash is required to bootstrap Homebrew automatically"
	fi
	log "Homebrew was not found. Archive Center will bootstrap Homebrew automatically."
	log "macOS may ask for your password while installing Apple's command line tools or Homebrew."
	[ -n "${EXTERNAL_OPERATION_TIMEOUT_SECONDS:-}" ] || die "Homebrew bootstrap requires --external-operation-timeout-seconds or AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS"
	homebrew_installer=$(curl --connect-timeout "$EXTERNAL_OPERATION_TIMEOUT_SECONDS" --max-time "$EXTERNAL_OPERATION_TIMEOUT_SECONDS" -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)
	run_external env NONINTERACTIVE=1 /bin/bash -c "$homebrew_installer"
	if ! load_homebrew_env; then
		die "Homebrew bootstrap finished, but brew was still not found"
	fi
}

install_macos_deps() {
	load_homebrew_env || true
	if runtime_dependencies_ready; then
		log "Required macOS runtime dependencies are already available; skipping Homebrew."
		return
	fi
	if [ "$NO_INSTALL" = "true" ]; then
		return
	fi
	ensure_homebrew
	run_external "$BREW_BIN" update || true
	run_external "$BREW_BIN" install mariadb python || true
}

install_termux_deps() {
	if runtime_dependencies_ready; then
		log "Required Termux runtime dependencies are already available; skipping pkg."
		return
	fi
	if [ "$NO_INSTALL" = "true" ]; then
		return
	fi
	if ! has_cmd pkg; then
		die "Termux pkg command was not found"
	fi
	run_external pkg update -y
	run_external pkg install -y mariadb python curl
	if [ "$AC_VECTOR_MODE" = "local_proot" ]; then
		run_external pkg install -y proot-distro
	fi
}

ensure_python() {
	if has_cmd python3; then
		PYTHON_BIN=$(command -v python3)
	elif has_cmd python; then
		PYTHON_BIN=$(command -v python)
	else
		die "python3 was not found after dependency install"
	fi
	export PYTHON_BIN
}

is_local_chroma_endpoint() {
	case "$AC_CHROMA_ENDPOINT" in
		http://127.0.0.1:*|http://localhost:*|https://127.0.0.1:*|https://localhost:*)
			return 0
			;;
	esac
	return 1
}

use_external_chromadb() {
	if [ "$AC_VECTOR_MODE" = "external" ]; then
		return 0
	fi
	return 1
}

vector_requires_chromadb() {
	case "$AC_VECTOR_MODE" in
		external|local_native|local_proot|bundled)
			return 0
			;;
	esac
	return 1
}

local_chromadb_requested() {
	case "$AC_VECTOR_MODE" in
		local_native|local_proot|bundled)
			return 0
			;;
	esac
	return 1
}

ensure_termux_proot_chromadb() {
	if ! has_cmd proot-distro; then
		if has_cmd pkg && [ "$NO_INSTALL" != "true" ]; then
			run_external pkg install -y proot-distro
		fi
	fi
	has_cmd proot-distro || die "proot-distro was not found. Termux local ChromaDB requires a managed Ubuntu/proot runtime."

	PROOT_CHROMA_DISTRO=${AC_TERMUX_CHROMA_DISTRO:-ubuntu}
	PROOT_CHROMA_ROOT=${AC_TERMUX_CHROMA_ROOT:-/root/archive-center}
	PROOT_CHROMA_VENV="$PROOT_CHROMA_ROOT/chromadb-venv"
	PROOT_CHROMA_DATA="$PROOT_CHROMA_ROOT/chromadb-data"
	export PROOT_CHROMA_DISTRO PROOT_CHROMA_ROOT PROOT_CHROMA_VENV PROOT_CHROMA_DATA

	if ! run_external proot-distro login "$PROOT_CHROMA_DISTRO" -- true >/dev/null 2>&1; then
		log "Installing Termux proot distro for ChromaDB: $PROOT_CHROMA_DISTRO"
		run_external proot-distro install "$PROOT_CHROMA_DISTRO"
	fi

	if run_external proot-distro login "$PROOT_CHROMA_DISTRO" -- bash -lc "test -x '$PROOT_CHROMA_VENV/bin/python' && '$PROOT_CHROMA_VENV/bin/python' -c 'import chromadb'" >/dev/null 2>&1; then
		return
	fi

	log "Preparing ChromaDB inside Termux proot distro: $PROOT_CHROMA_DISTRO"
	run_external proot-distro login "$PROOT_CHROMA_DISTRO" -- bash -lc "apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y python3 python3-venv python3-pip curl ca-certificates"
	run_external proot-distro login "$PROOT_CHROMA_DISTRO" -- bash -lc "mkdir -p '$PROOT_CHROMA_ROOT' '$PROOT_CHROMA_DATA' && python3 -m venv '$PROOT_CHROMA_VENV' && '$PROOT_CHROMA_VENV/bin/python' -m pip install --upgrade pip wheel setuptools && '$PROOT_CHROMA_VENV/bin/python' -m pip install 'chromadb==1.5.9'"
}

ensure_chromadb() {
	if ! vector_requires_chromadb; then
		return
	fi
	ensure_python
	if use_external_chromadb; then
		log "Using external ChromaDB endpoint: $AC_CHROMA_ENDPOINT"
		return
	fi
	if [ "$PLATFORM" = "termux" ]; then
		ensure_termux_proot_chromadb
		return
	fi
	venv_dir="$RUNTIME_DIR/chromadb-venv"
	venv_python="$venv_dir/bin/python"
	if [ -x "$venv_python" ] && "$venv_python" -c 'from importlib.metadata import version; import chromadb; assert version("chromadb") == "1.5.9"' >/dev/null 2>&1; then
		CHROMA_PYTHON=$venv_python
		export CHROMA_PYTHON
		return
	fi
	if [ ! -x "$venv_dir/bin/python" ]; then
		log "Creating ChromaDB Python runtime"
		run_external "$PYTHON_BIN" -m venv "$venv_dir" 2>/dev/null || run_external "$PYTHON_BIN" -m virtualenv "$venv_dir"
	fi
	run_external "$venv_python" -m pip install --upgrade pip wheel setuptools
	if ! "$venv_python" -c 'from importlib.metadata import version; assert version("chromadb") == "1.5.9"' >/dev/null 2>&1; then
		log "Installing pinned ChromaDB 1.5.9 into managed runtime"
		run_external "$venv_python" -m pip install --upgrade "chromadb==1.5.9"
	fi
	CHROMA_PYTHON=$venv_python
	export CHROMA_PYTHON
}

find_mariadb_tools() {
	MARIADBD=$(find_executable \
		"$PACKAGE_ROOT/runtime/MariaDB/bin/mariadbd" \
		"$PACKAGE_ROOT/runtime/mariadb/bin/mariadbd" \
		"$(command_path mariadbd 2>/dev/null || true)" \
		"$(command_path mysqld 2>/dev/null || true)" || true)
	MARIA_INSTALL_DB=$(find_executable \
		"$PACKAGE_ROOT/runtime/MariaDB/bin/mariadb-install-db" \
		"$PACKAGE_ROOT/runtime/mariadb/bin/mariadb-install-db" \
		"$(command_path mariadb-install-db 2>/dev/null || true)" \
		"$(command_path mysql_install_db 2>/dev/null || true)" || true)
	MARIA_CLIENT=$(find_executable \
		"$PACKAGE_ROOT/runtime/MariaDB/bin/mariadb" \
		"$PACKAGE_ROOT/runtime/mariadb/bin/mariadb" \
		"$(command_path mariadb 2>/dev/null || true)" \
		"$(command_path mysql 2>/dev/null || true)" || true)
	[ -n "$MARIADBD" ] || die "mariadbd/mysqld was not found"
	[ -n "$MARIA_CLIENT" ] || die "mariadb/mysql client was not found"
	export MARIADBD MARIA_INSTALL_DB MARIA_CLIENT
}

init_mariadb_data() {
	find_mariadb_tools
	if [ -d "$MARIADB_DATA/mysql" ]; then
		return
	fi
	log "Initializing MariaDB data directory"
	mkdir -p "$MARIADB_DATA" "$LOG_DIR"
	if [ -n "$MARIA_INSTALL_DB" ]; then
		if ! run_external "$MARIA_INSTALL_DB" --datadir="$MARIADB_DATA" --auth-root-authentication-method=normal >"$LOG_DIR/mariadb-init.log" 2>&1; then
			log "MariaDB init log tail:"
			tail -n 80 "$LOG_DIR/mariadb-init.log" >&2 || true
			return 1
		fi
	else
		if ! run_external "$MARIADBD" --initialize-insecure --datadir="$MARIADB_DATA" >"$LOG_DIR/mariadb-init.log" 2>&1; then
			log "MariaDB init log tail:"
			tail -n 80 "$LOG_DIR/mariadb-init.log" >&2 || true
			return 1
		fi
	fi
}

start_mariadb() {
	init_mariadb_data
	if port_is_open "$MARIADB_PORT"; then
		return
	fi
	log "Starting MariaDB on 127.0.0.1:$MARIADB_PORT"
	mkdir -p "$LOG_DIR"
	start_managed_process "$MARIADBD" \
		--datadir="$MARIADB_DATA" \
		--port="$MARIADB_PORT" \
		--socket="$RUNTIME_DIR/mysql.sock" \
		--pid-file="$RUNTIME_DIR/mariadb.pid" \
		--skip-networking=0 \
		--bind-address=127.0.0.1 \
		--log-error="$LOG_DIR/mariadb.log"
	MARIADB_PID=$MANAGED_PROCESS_PID
	export MARIADB_PID
	wait_port "$MARIADB_PORT" "MariaDB" "$MARIADB_PID"
}

bootstrap_mariadb_schema() {
	db_name=archive_center
	db_user=archive_center
	db_pass=archive-center-local-pass
	AC_MARIADB_DSN="${db_user}:${db_pass}@tcp(127.0.0.1:${MARIADB_PORT})/${db_name}?parseTime=true"
	export AC_MARIADB_DSN
	SCHEMA_FILE="$PACKAGE_ROOT/migrations/001_schema.sql"
	[ -f "$SCHEMA_FILE" ] || die "schema file was not found: $SCHEMA_FILE"
	run_external "$MARIADB_SCHEMA_RUN" \
		-dsn "$AC_MARIADB_DSN" \
		-schema "$PACKAGE_ROOT/migrations" \
		-execute=true \
		-managed-bootstrap=true \
		-managed-host 127.0.0.1 \
		-managed-port "$MARIADB_PORT" \
		-expected-datadir "$MARIADB_DATA"
}

start_chromadb() {
	if ! vector_requires_chromadb; then
		log "Skipping ChromaDB; runtime profile is $AC_RUNTIME_PROFILE with vector mode $AC_VECTOR_MODE"
		return
	fi
	if use_external_chromadb; then
		log "Skipping local ChromaDB startup; using external endpoint: $AC_CHROMA_ENDPOINT"
		return
	fi
	ensure_chromadb
	chroma_port=$(printf '%s' "$AC_CHROMA_ENDPOINT" | sed -n 's#.*:\([0-9][0-9]*\).*#\1#p')
	if [ -z "$chroma_port" ]; then
		chroma_port=8000
	fi
	if port_is_open "$chroma_port"; then
		return
	fi
	log "Starting ChromaDB on $AC_CHROMA_ENDPOINT"
	mkdir -p "$CHROMA_DATA" "$LOG_DIR"
	if [ "$PLATFORM" = "termux" ]; then
		start_managed_process proot-distro login "$PROOT_CHROMA_DISTRO" -- bash -lc "mkdir -p '$PROOT_CHROMA_DATA' && '$PROOT_CHROMA_VENV/bin/chroma' run --host 127.0.0.1 --port '$chroma_port' --path '$PROOT_CHROMA_DATA'" >"$LOG_DIR/chromadb.out.log" 2>"$LOG_DIR/chromadb.err.log"
		CHROMA_PID=$MANAGED_PROCESS_PID
		export CHROMA_PID
		wait_port "$chroma_port" "ChromaDB" "$CHROMA_PID"
		return
	fi
	chroma_bin=$(find_executable "$RUNTIME_DIR/chromadb-venv/bin/chroma" || true)
	if [ -n "$chroma_bin" ]; then
		start_managed_process "$chroma_bin" run --host 127.0.0.1 --port "$chroma_port" --path "$CHROMA_DATA" >"$LOG_DIR/chromadb.out.log" 2>"$LOG_DIR/chromadb.err.log"
	else
		start_managed_process "$CHROMA_PYTHON" -m chromadb.cli.cli run --host 127.0.0.1 --port "$chroma_port" --path "$CHROMA_DATA" >"$LOG_DIR/chromadb.out.log" 2>"$LOG_DIR/chromadb.err.log"
	fi
	CHROMA_PID=$MANAGED_PROCESS_PID
	export CHROMA_PID
	wait_port "$chroma_port" "ChromaDB" "$CHROMA_PID"
}

start_lifetime_watchdog() {
	mkdir -p "$RUNTIME_DIR"
	LIFETIME_PID_FILE="$RUNTIME_DIR/launcher-managed-process-groups"
	LIFETIME_STOP_FILE="$RUNTIME_DIR/launcher-stop-request"
	LIFETIME_DONE_FILE="$RUNTIME_DIR/launcher-stop-complete"
	rm -f -- "$LIFETIME_PID_FILE" "$LIFETIME_STOP_FILE" "$LIFETIME_DONE_FILE"
	: >"$LIFETIME_PID_FILE"
	"$PYTHON_BIN" "$LIFETIME_HELPER" watch "$$" "$LIFETIME_PID_FILE" "$LIFETIME_STOP_FILE" "$LIFETIME_DONE_FILE" &
	LIFETIME_WATCHDOG_PID=$!
	export LIFETIME_PID_FILE LIFETIME_STOP_FILE LIFETIME_DONE_FILE LIFETIME_WATCHDOG_PID
}

start_managed_process() {
	"$PYTHON_BIN" "$LIFETIME_HELPER" run "$LIFETIME_PID_FILE" "$$" "$@" &
	MANAGED_PROCESS_PID=$!
}

stop_lifetime_watchdog() {
	if [ -z "${LIFETIME_PID_FILE:-}" ]; then
		return
	fi
	if [ -n "${LIFETIME_WATCHDOG_PID:-}" ] && kill -0 "$LIFETIME_WATCHDOG_PID" >/dev/null 2>&1; then
		: >"$LIFETIME_STOP_FILE"
		wait_count=0
		while kill -0 "$LIFETIME_WATCHDOG_PID" >/dev/null 2>&1 && [ "$wait_count" -lt 12 ]; do
			sleep 1
			wait_count=$((wait_count + 1))
		done
		wait "$LIFETIME_WATCHDOG_PID" >/dev/null 2>&1 || true
	else
		"$PYTHON_BIN" "$LIFETIME_HELPER" stop "$LIFETIME_PID_FILE" >/dev/null 2>&1 || true
	fi
	rm -f -- "$LIFETIME_PID_FILE" "$LIFETIME_STOP_FILE" "$LIFETIME_DONE_FILE"
	LIFETIME_PID_FILE=
	LIFETIME_STOP_FILE=
	LIFETIME_DONE_FILE=
	LIFETIME_WATCHDOG_PID=
}

cleanup() {
	if [ "${LIFETIME_CLEANUP_ACTIVE:-false}" = "true" ]; then
		return
	fi
	LIFETIME_CLEANUP_ACTIVE=true
	cleanup_updater_runner
	if [ -n "${EXTERNAL_OPERATION_PID:-}" ]; then
		kill "$EXTERNAL_OPERATION_PID" >/dev/null 2>&1 || true
	fi
	if [ -n "${EXTERNAL_GUARD_PID:-}" ]; then
		kill "$EXTERNAL_GUARD_PID" >/dev/null 2>&1 || true
	fi
	if [ -n "${EXTERNAL_TIMEOUT_MARKER:-}" ]; then
		rm -f -- "$EXTERNAL_TIMEOUT_MARKER" >/dev/null 2>&1 || true
	fi
	stop_lifetime_watchdog
	BACKEND_PID=
	CHROMA_PID=
	MARIADB_PID=
	LIFETIME_CLEANUP_ACTIVE=false
}

shutdown_from_signal() {
	exit_code=$1
	trap - EXIT HUP INT TERM
	cleanup
	exit "$exit_code"
}

print_preflight() {
	arch=$(detect_arch)
	mariadb_present=false
	chromadb_present=false
	manual_chromadb_required=false
	if find_executable "$PACKAGE_ROOT/runtime/MariaDB/bin/mariadbd" "$PACKAGE_ROOT/runtime/mariadb/bin/mariadbd" "$(command_path mariadbd 2>/dev/null || true)" "$(command_path mysqld 2>/dev/null || true)" >/dev/null 2>&1; then
		mariadb_present=true
	fi
	if has_cmd python3 && python3 -c 'import chromadb' >/dev/null 2>&1; then
		chromadb_present=true
	fi
	if [ "$AC_VECTOR_MODE" = "external" ]; then
		manual_chromadb_required=true
	fi
	cat <<EOF
{
  "status": "ok",
  "package_profile": "$PACKAGE_PROFILE",
  "runtime_profile": "$AC_RUNTIME_PROFILE",
  "vector_mode": "$AC_VECTOR_MODE",
  "platform": "$PLATFORM",
  "arch": "$arch",
  "package_root": "$(json_escape "$PACKAGE_ROOT")",
  "runtime_dir": "$(json_escape "$RUNTIME_DIR")",
  "mariadb_present": $(json_bool "$mariadb_present"),
  "chromadb_present": $(json_bool "$chromadb_present"),
  "normal_user_manual_mariadb_required": false,
  "normal_user_manual_chromadb_required": $(json_bool "$manual_chromadb_required")
}
EOF
}

PLATFORM=${ARCHIVE_CENTER_PLATFORM:-}
REQUESTED_RUNTIME_PROFILE=${AC_RUNTIME_PROFILE:-}
REQUESTED_VECTOR_MODE=${AC_VECTOR_MODE:-}
PREFLIGHT=false
INSTALL_ONLY=false
NO_INSTALL=false
READINESS_TIMEOUT_SECONDS=${AC_READINESS_TIMEOUT_SECONDS:-}
READINESS_POLL_INTERVAL_SECONDS=${AC_READINESS_POLL_INTERVAL_SECONDS:-}
REQUEST_TIMEOUT_SECONDS=${AC_REQUEST_TIMEOUT_SECONDS:-}
EXTERNAL_OPERATION_TIMEOUT_SECONDS=${AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS:-}

while [ "$#" -gt 0 ]; do
	case "$1" in
		--platform)
			[ "$#" -ge 2 ] || die "missing value for --platform"
			PLATFORM=$2
			shift 2
			;;
		--profile|--runtime-profile)
			[ "$#" -ge 2 ] || die "missing value for --profile"
			REQUESTED_RUNTIME_PROFILE=$2
			shift 2
			;;
		--vector-mode)
			[ "$#" -ge 2 ] || die "missing value for --vector-mode"
			REQUESTED_VECTOR_MODE=$2
			shift 2
			;;
		--preflight)
			PREFLIGHT=true
			shift
			;;
		--install-only)
			INSTALL_ONLY=true
			shift
			;;
		--no-install)
			NO_INSTALL=true
			shift
			;;
		--readiness-timeout-seconds)
			[ "$#" -ge 2 ] || die "missing value for --readiness-timeout-seconds"
			READINESS_TIMEOUT_SECONDS=$2
			shift 2
			;;
		--readiness-poll-interval-seconds)
			[ "$#" -ge 2 ] || die "missing value for --readiness-poll-interval-seconds"
			READINESS_POLL_INTERVAL_SECONDS=$2
			shift 2
			;;
		--request-timeout-seconds)
			[ "$#" -ge 2 ] || die "missing value for --request-timeout-seconds"
			REQUEST_TIMEOUT_SECONDS=$2
			shift 2
			;;
		--external-operation-timeout-seconds)
			[ "$#" -ge 2 ] || die "missing value for --external-operation-timeout-seconds"
			EXTERNAL_OPERATION_TIMEOUT_SECONDS=$2
			shift 2
			;;
		--help|-h)
			usage
			exit 0
			;;
		*)
			die "unknown argument: $1"
			;;
	esac
done

[ -n "$PLATFORM" ] || die "missing --platform"
case "$READINESS_TIMEOUT_SECONDS" in
	"") ;;
	*[!0-9]*|0) die "readiness timeout must be a positive integer" ;;
esac
case "$READINESS_POLL_INTERVAL_SECONDS" in
	"") ;;
	*[!0-9]*|0) die "readiness poll interval must be a positive integer" ;;
esac
case "$REQUEST_TIMEOUT_SECONDS" in
	"") ;;
	*[!0-9]*|0) die "request timeout must be a positive integer" ;;
esac
case "$EXTERNAL_OPERATION_TIMEOUT_SECONDS" in
	"") ;;
	*[!0-9]*|0) die "external operation timeout must be a positive integer" ;;
esac
if { [ -n "$READINESS_TIMEOUT_SECONDS" ] && [ -z "$READINESS_POLL_INTERVAL_SECONDS" ]; } || { [ -z "$READINESS_TIMEOUT_SECONDS" ] && [ -n "$READINESS_POLL_INTERVAL_SECONDS" ]; }; then
	die "readiness timeout and poll interval must be supplied together"
fi

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
PWD_DIR=$(pwd -P 2>/dev/null || pwd)
if [ -n "${ARCHIVE_CENTER_PACKAGE_ROOT:-}" ]; then
	PACKAGE_ROOT=$(canonical_dir "$ARCHIVE_CENTER_PACKAGE_ROOT" 2>/dev/null || true)
	if ! is_package_root "$PACKAGE_ROOT"; then
		PACKAGE_ROOT=$(resolve_package_root "$SCRIPT_DIR/.." "$SCRIPT_DIR" "$PWD_DIR" "$PWD_DIR/.." "$SCRIPT_DIR/../.." "$ARCHIVE_CENTER_PACKAGE_ROOT" || true)
	fi
	if ! is_package_root "$PACKAGE_ROOT"; then
		die "ARCHIVE_CENTER_PACKAGE_ROOT is not a complete package root and no fallback package root was found: ${ARCHIVE_CENTER_PACKAGE_ROOT}. Required files: migrations/001_schema.sql, bin/archive-center-go, bin/mariadb-schema."
	fi
else
	PACKAGE_ROOT=$(resolve_package_root "$SCRIPT_DIR/.." "$SCRIPT_DIR" "$PWD_DIR" "$PWD_DIR/.." "$SCRIPT_DIR/../.." || true)
fi
[ -n "$PACKAGE_ROOT" ] || die "Archive Center package root was not found. Extract the full package ZIP into one folder, then run the launcher from inside that extracted folder. Required files: migrations/001_schema.sql, bin/archive-center-go, bin/mariadb-schema."
if [ -z "$REQUESTED_RUNTIME_PROFILE" ]; then
	REQUESTED_RUNTIME_PROFILE=core_lite
fi
case "$REQUESTED_RUNTIME_PROFILE" in
	client_only|core_lite|vector_external|vector_local_native|full_local)
		AC_RUNTIME_PROFILE=$REQUESTED_RUNTIME_PROFILE
		;;
	*)
		die "unsupported runtime profile: $REQUESTED_RUNTIME_PROFILE"
		;;
esac
if [ -z "$REQUESTED_VECTOR_MODE" ]; then
	case "$AC_RUNTIME_PROFILE" in
		client_only)
			REQUESTED_VECTOR_MODE=off
			;;
		vector_external)
			REQUESTED_VECTOR_MODE=external
			;;
		vector_local_native)
			REQUESTED_VECTOR_MODE=local_native
			;;
		full_local)
			if [ "$PLATFORM" = "termux" ]; then
				REQUESTED_VECTOR_MODE=local_proot
			else
				REQUESTED_VECTOR_MODE=local_native
			fi
			;;
		*)
			REQUESTED_VECTOR_MODE=fallback
			;;
	esac
fi
case "$REQUESTED_VECTOR_MODE" in
	off|fallback|external|local_native|local_proot|bundled)
		AC_VECTOR_MODE=$REQUESTED_VECTOR_MODE
		;;
	*)
		die "unsupported vector mode: $REQUESTED_VECTOR_MODE"
		;;
esac
if [ "$AC_RUNTIME_PROFILE" = "client_only" ] && [ "$AC_VECTOR_MODE" != "off" ]; then
	die "client_only requires AC_VECTOR_MODE=off"
fi
if [ "$AC_RUNTIME_PROFILE" = "core_lite" ] && [ "$AC_VECTOR_MODE" != "fallback" ] && [ "$AC_VECTOR_MODE" != "off" ]; then
	die "core_lite supports only fallback or off vector modes"
fi
if [ "$AC_RUNTIME_PROFILE" = "vector_external" ] && [ "$AC_VECTOR_MODE" != "external" ]; then
	die "vector_external requires AC_VECTOR_MODE=external"
fi
PACKAGE_PROFILE="managed_${AC_RUNTIME_PROFILE}_candidate"
if [ -n "${ARCHIVE_CENTER_DATA_DIR:-}" ]; then
	DATA_ROOT=$ARCHIVE_CENTER_DATA_DIR
elif [ "$PLATFORM" = "termux" ]; then
	DATA_ROOT="${HOME:-$PACKAGE_ROOT}/.archive-center-2.0"
else
	DATA_ROOT="$PACKAGE_ROOT/.runtime"
fi
RUNTIME_DIR="$DATA_ROOT"
export ARCHIVE_CENTER_DATA_DIR="$DATA_ROOT"
MARIADB_DATA="$RUNTIME_DIR/mariadb-data"
CHROMA_DATA="$RUNTIME_DIR/chromadb-data"
LOG_DIR="$RUNTIME_DIR/logs"
EXEC_BIN_DIR="$RUNTIME_DIR/bin"
LIFETIME_HELPER="$SCRIPT_DIR/process-lifetime.py"
[ -f "$LIFETIME_HELPER" ] || die "process lifetime helper was not found: $LIFETIME_HELPER"
MARIADB_PORT=${AC_MARIADB_PORT:-3307}
AC_BIND_ADDR=${AC_BIND_ADDR:-0.0.0.0:28080}
if [ "$AC_VECTOR_MODE" = "external" ]; then
	[ -n "${AC_CHROMA_ENDPOINT:-}" ] || die "AC_CHROMA_ENDPOINT is required for vector_external"
elif local_chromadb_requested; then
	AC_CHROMA_ENDPOINT=${AC_CHROMA_ENDPOINT:-http://127.0.0.1:8000}
else
	AC_CHROMA_ENDPOINT=
fi
AC_CHROMA_COLLECTION=${AC_CHROMA_COLLECTION:-archive_center_vectors}
AC_CHROMA_API_PATH=${AC_CHROMA_API_PATH:-/api/v2}
if [ "$PLATFORM" = "termux" ]; then
	AC_DNS_SERVERS=${AC_DNS_SERVERS:-1.1.1.1:53,8.8.8.8:53}
	export AC_DNS_SERVERS
fi
export PACKAGE_ROOT RUNTIME_DIR MARIADB_DATA CHROMA_DATA LOG_DIR EXEC_BIN_DIR MARIADB_PORT
export AC_RUNTIME_PROFILE AC_VECTOR_MODE AC_BIND_ADDR AC_CHROMA_ENDPOINT AC_CHROMA_COLLECTION AC_CHROMA_API_PATH
AC_UPDATE_STAGING_DIR="$PACKAGE_ROOT/.updates"
AC_UPDATE_APPLY_MODE=managed_launcher_exit_75
export AC_UPDATE_STAGING_DIR AC_UPDATE_APPLY_MODE

if [ "$PREFLIGHT" = "true" ]; then
	print_preflight
	exit 0
fi

if [ "$AC_RUNTIME_PROFILE" = "client_only" ]; then
	log "Archive Center client_only profile selected."
	log "No local backend, MariaDB, or ChromaDB service will be started on this device."
	log "Configure the RisuAI plugin Bridge URL to the PC/NAS Archive Center backend."
	exit 0
fi

case "$PLATFORM" in
	linux)
		install_linux_deps
		;;
	macos)
		install_macos_deps
		;;
	termux)
		install_termux_deps
		;;
	*)
		die "unsupported platform: $PLATFORM"
		;;
esac

ensure_python
if local_chromadb_requested; then
	ensure_chromadb
fi
find_mariadb_tools

if [ "$INSTALL_ONLY" = "true" ]; then
	log "Install/bootstrap completed. Run this script again without --install-only to start Archive Center."
	exit 0
fi

trap cleanup EXIT
trap 'shutdown_from_signal 129' HUP
trap 'shutdown_from_signal 130' INT
trap 'shutdown_from_signal 143' TERM

ROLLBACK_HEALTH_VERSION=
while :; do
	start_lifetime_watchdog
	prepare_updater_runner
	apply_pending_update
	prepare_package_binaries

	start_mariadb
	if ! bootstrap_mariadb_schema; then
		if rollback_pending_preparation_failure "Updated package schema/bootstrap failed."; then
			cleanup
			MARIADB_PID=
			CHROMA_PID=
			UPDATER_RUNNER=
			continue
		fi
		die "MariaDB schema/bootstrap failed for the verified current package"
	fi
	start_chromadb

	export AC_MODE=live
	export AC_STORE_MODE=mariadb_authority
	export AC_PROMPT_DIR="$PACKAGE_ROOT/prompts"
	export AC_PRUNE_POLICY=${AC_PRUNE_POLICY:-soft}

	finalize_pending_update

	log "Starting Archive Center 2.1"
	log "  Backend:  http://$AC_BIND_ADDR"
	log "  MariaDB:  127.0.0.1:$MARIADB_PORT"
	if vector_requires_chromadb; then
		log "  ChromaDB: $AC_CHROMA_ENDPOINT"
	else
		log "  ChromaDB: disabled ($AC_VECTOR_MODE)"
	fi
	log "  Profile:  $AC_RUNTIME_PROFILE"
	log "  Vector:   $AC_VECTOR_MODE"
	log "  Package:  $PACKAGE_PROFILE"
	log "Stop with Ctrl+C."

	cleanup_updater_runner
	UPDATER_RUNNER=
	prepare_update_launcher_session
	start_managed_process "$ARCHIVE_CENTER_GO_RUN"
	BACKEND_PID=$MANAGED_PROCESS_PID
	export BACKEND_PID
	if [ -n "$ROLLBACK_HEALTH_VERSION" ]; then
		if ! wait_candidate_backend_ready "$BACKEND_PID" "$ROLLBACK_HEALTH_VERSION"; then
			die "rolled-back backend failed /ready or exact /version verification"
		fi
		log "Rolled-back backend passed /ready and exact /version verification."
		ROLLBACK_HEALTH_VERSION=
	fi
	if wait "$BACKEND_PID"; then
		backend_exit=0
	else
		backend_exit=$?
	fi
	BACKEND_PID=
	export BACKEND_PID
	if [ "$backend_exit" -ne 75 ]; then
		exit "$backend_exit"
	fi
	log "Backend requested immediate pending-update apply (exit 75)."
	cleanup
	MARIADB_PID=
	CHROMA_PID=
	UPDATER_RUNNER=
done
