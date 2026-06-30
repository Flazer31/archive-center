#!/usr/bin/env sh
set -eu

# Archive Center 2.1 managed POSIX package launcher.
#
# This is intentionally installer-managed: normal users should not have to
# manually install MariaDB or ChromaDB. The script uses the platform package
# manager when bundled POSIX runtimes are not present.

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
  --keep-services   Do not stop MariaDB/ChromaDB when the Go backend exits.
  --help            Show this help.

Environment:
  ARCHIVE_CENTER_DATA_DIR      Runtime data directory. Defaults inside package.
  AC_RUNTIME_PROFILE           Defaults to core_lite.
  AC_VECTOR_MODE               Defaults from AC_RUNTIME_PROFILE.
  AC_BIND_ADDR                 Defaults to 0.0.0.0:28080.
  AC_CHROMA_ENDPOINT           Required for vector_external; defaults to local
                               only for local vector profiles.
  AC_MARIADB_PORT              Defaults to 3307.
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
		"$@"
	elif has_cmd sudo; then
		sudo "$@"
	else
		die "sudo is required to install packages on this platform"
	fi
}

wait_port() {
	port=$1
	label=$2
	python_for_probe=${PYTHON_BIN:-python3}
	i=0
	while [ "$i" -lt 90 ]; do
		if "$python_for_probe" - "$port" >/dev/null 2>&1 <<'PY'
import socket
import sys
port = int(sys.argv[1])
s = socket.socket()
s.settimeout(0.4)
try:
    s.connect(("127.0.0.1", port))
except OSError:
    sys.exit(1)
finally:
    s.close()
PY
		then
			return 0
		fi
		i=$((i + 1))
		sleep 1
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

install_linux_deps() {
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
	NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
	if ! load_homebrew_env; then
		die "Homebrew bootstrap finished, but brew was still not found"
	fi
}

install_macos_deps() {
	if [ "$NO_INSTALL" = "true" ]; then
		return
	fi
	ensure_homebrew
	"$BREW_BIN" update || true
	"$BREW_BIN" install mariadb python || true
}

install_termux_deps() {
	if [ "$NO_INSTALL" = "true" ]; then
		return
	fi
	if ! has_cmd pkg; then
		die "Termux pkg command was not found"
	fi
	pkg update -y
	pkg install -y mariadb python curl
	if [ "$AC_VECTOR_MODE" = "local_proot" ]; then
		pkg install -y proot-distro
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
			pkg install -y proot-distro
		fi
	fi
	has_cmd proot-distro || die "proot-distro was not found. Termux local ChromaDB requires a managed Ubuntu/proot runtime."

	PROOT_CHROMA_DISTRO=${AC_TERMUX_CHROMA_DISTRO:-ubuntu}
	PROOT_CHROMA_ROOT=${AC_TERMUX_CHROMA_ROOT:-/root/archive-center}
	PROOT_CHROMA_VENV="$PROOT_CHROMA_ROOT/chromadb-venv"
	PROOT_CHROMA_DATA="$PROOT_CHROMA_ROOT/chromadb-data"
	export PROOT_CHROMA_DISTRO PROOT_CHROMA_ROOT PROOT_CHROMA_VENV PROOT_CHROMA_DATA

	if ! proot-distro login "$PROOT_CHROMA_DISTRO" -- true >/dev/null 2>&1; then
		log "Installing Termux proot distro for ChromaDB: $PROOT_CHROMA_DISTRO"
		proot-distro install "$PROOT_CHROMA_DISTRO"
	fi

	if proot-distro login "$PROOT_CHROMA_DISTRO" -- bash -lc "test -x '$PROOT_CHROMA_VENV/bin/python' && '$PROOT_CHROMA_VENV/bin/python' -c 'import chromadb'" >/dev/null 2>&1; then
		return
	fi

	log "Preparing ChromaDB inside Termux proot distro: $PROOT_CHROMA_DISTRO"
	proot-distro login "$PROOT_CHROMA_DISTRO" -- bash -lc "apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y python3 python3-venv python3-pip curl ca-certificates"
	proot-distro login "$PROOT_CHROMA_DISTRO" -- bash -lc "mkdir -p '$PROOT_CHROMA_ROOT' '$PROOT_CHROMA_DATA' && python3 -m venv '$PROOT_CHROMA_VENV' && '$PROOT_CHROMA_VENV/bin/python' -m pip install --upgrade pip wheel setuptools && '$PROOT_CHROMA_VENV/bin/python' -m pip install 'chromadb>=0.5,<1.0'"
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
	if [ ! -x "$venv_dir/bin/python" ]; then
		log "Creating ChromaDB Python runtime"
		"$PYTHON_BIN" -m venv "$venv_dir" 2>/dev/null || "$PYTHON_BIN" -m virtualenv "$venv_dir"
	fi
	venv_python="$venv_dir/bin/python"
	"$venv_python" -m pip install --upgrade pip wheel setuptools
	if ! "$venv_python" -c 'import chromadb' >/dev/null 2>&1; then
		log "Installing ChromaDB into managed runtime"
		"$venv_python" -m pip install "chromadb>=0.5,<1.0"
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
		if ! "$MARIA_INSTALL_DB" --datadir="$MARIADB_DATA" --auth-root-authentication-method=normal >"$LOG_DIR/mariadb-init.log" 2>&1; then
			log "MariaDB init log tail:"
			tail -n 80 "$LOG_DIR/mariadb-init.log" >&2 || true
			return 1
		fi
	else
		if ! "$MARIADBD" --initialize-insecure --datadir="$MARIADB_DATA" >"$LOG_DIR/mariadb-init.log" 2>&1; then
			log "MariaDB init log tail:"
			tail -n 80 "$LOG_DIR/mariadb-init.log" >&2 || true
			return 1
		fi
	fi
}

start_mariadb() {
	init_mariadb_data
	if "$PYTHON_BIN" - "$MARIADB_PORT" >/dev/null 2>&1 <<'PY'
import socket
import sys
s=socket.socket()
s.settimeout(0.3)
try:
    s.connect(("127.0.0.1", int(sys.argv[1])))
except OSError:
    sys.exit(1)
finally:
    s.close()
PY
	then
		MARIADB_STARTED_BY_SCRIPT=false
		export MARIADB_STARTED_BY_SCRIPT
		return
	fi
	log "Starting MariaDB on 127.0.0.1:$MARIADB_PORT"
	mkdir -p "$LOG_DIR"
	"$MARIADBD" \
		--datadir="$MARIADB_DATA" \
		--port="$MARIADB_PORT" \
		--socket="$RUNTIME_DIR/mysql.sock" \
		--pid-file="$RUNTIME_DIR/mariadb.pid" \
		--skip-networking=0 \
		--bind-address=127.0.0.1 \
		--log-error="$LOG_DIR/mariadb.log" &
	MARIADB_PID=$!
	MARIADB_STARTED_BY_SCRIPT=true
	export MARIADB_PID MARIADB_STARTED_BY_SCRIPT
	wait_port "$MARIADB_PORT" "MariaDB"
}

bootstrap_mariadb_schema() {
	db_name=archive_center
	db_user=archive_center
	db_pass=archive-center-local-pass
	sql="CREATE DATABASE IF NOT EXISTS ${db_name} CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci; CREATE USER IF NOT EXISTS '${db_user}'@'127.0.0.1' IDENTIFIED BY '${db_pass}'; GRANT ALL PRIVILEGES ON ${db_name}.* TO '${db_user}'@'127.0.0.1'; CREATE USER IF NOT EXISTS '${db_user}'@'localhost' IDENTIFIED BY '${db_pass}'; GRANT ALL PRIVILEGES ON ${db_name}.* TO '${db_user}'@'localhost'; FLUSH PRIVILEGES;"
	"$MARIA_CLIENT" --protocol=tcp --ssl=0 -h 127.0.0.1 -P "$MARIADB_PORT" -u root -e "$sql"
	AC_MARIADB_DSN="${db_user}:${db_pass}@tcp(127.0.0.1:${MARIADB_PORT})/${db_name}?parseTime=true"
	export AC_MARIADB_DSN
	SCHEMA_FILE="$PACKAGE_ROOT/migrations/001_schema.sql"
	[ -f "$SCHEMA_FILE" ] || die "schema file was not found: $SCHEMA_FILE"
	"$MARIADB_SCHEMA_RUN" -dsn "$AC_MARIADB_DSN" -schema "$SCHEMA_FILE" -execute=true
}

start_chromadb() {
	if ! vector_requires_chromadb; then
		log "Skipping ChromaDB; runtime profile is $AC_RUNTIME_PROFILE with vector mode $AC_VECTOR_MODE"
		CHROMA_STARTED_BY_SCRIPT=false
		export CHROMA_STARTED_BY_SCRIPT
		return
	fi
	if use_external_chromadb; then
		log "Skipping local ChromaDB startup; using external endpoint: $AC_CHROMA_ENDPOINT"
		CHROMA_STARTED_BY_SCRIPT=false
		export CHROMA_STARTED_BY_SCRIPT
		return
	fi
	ensure_chromadb
	chroma_port=$(printf '%s' "$AC_CHROMA_ENDPOINT" | sed -n 's#.*:\([0-9][0-9]*\).*#\1#p')
	if [ -z "$chroma_port" ]; then
		chroma_port=8000
	fi
	if "$PYTHON_BIN" - "$chroma_port" >/dev/null 2>&1 <<'PY'
import socket
import sys
s=socket.socket()
s.settimeout(0.3)
try:
    s.connect(("127.0.0.1", int(sys.argv[1])))
except OSError:
    sys.exit(1)
finally:
    s.close()
PY
	then
		CHROMA_STARTED_BY_SCRIPT=false
		export CHROMA_STARTED_BY_SCRIPT
		return
	fi
	log "Starting ChromaDB on $AC_CHROMA_ENDPOINT"
	mkdir -p "$CHROMA_DATA" "$LOG_DIR"
	if [ "$PLATFORM" = "termux" ]; then
		proot-distro login "$PROOT_CHROMA_DISTRO" -- bash -lc "mkdir -p '$PROOT_CHROMA_DATA' && '$PROOT_CHROMA_VENV/bin/chroma' run --host 127.0.0.1 --port '$chroma_port' --path '$PROOT_CHROMA_DATA'" >"$LOG_DIR/chromadb.out.log" 2>"$LOG_DIR/chromadb.err.log" &
		CHROMA_PID=$!
		CHROMA_STARTED_BY_SCRIPT=true
		export CHROMA_PID CHROMA_STARTED_BY_SCRIPT
		wait_port "$chroma_port" "ChromaDB"
		return
	fi
	chroma_bin=$(find_executable "$RUNTIME_DIR/chromadb-venv/bin/chroma" || true)
	if [ -n "$chroma_bin" ]; then
		"$chroma_bin" run --host 127.0.0.1 --port "$chroma_port" --path "$CHROMA_DATA" >"$LOG_DIR/chromadb.out.log" 2>"$LOG_DIR/chromadb.err.log" &
	else
		"$CHROMA_PYTHON" -m chromadb.cli.cli run --host 127.0.0.1 --port "$chroma_port" --path "$CHROMA_DATA" >"$LOG_DIR/chromadb.out.log" 2>"$LOG_DIR/chromadb.err.log" &
	fi
	CHROMA_PID=$!
	CHROMA_STARTED_BY_SCRIPT=true
	export CHROMA_PID CHROMA_STARTED_BY_SCRIPT
	wait_port "$chroma_port" "ChromaDB"
}

cleanup() {
	if [ "$KEEP_SERVICES" = "true" ]; then
		return
	fi
	if [ "${CHROMA_STARTED_BY_SCRIPT:-false}" = "true" ] && [ -n "${CHROMA_PID:-}" ]; then
		kill "$CHROMA_PID" >/dev/null 2>&1 || true
	fi
	if [ "${MARIADB_STARTED_BY_SCRIPT:-false}" = "true" ] && [ -n "${MARIADB_PID:-}" ]; then
		kill "$MARIADB_PID" >/dev/null 2>&1 || true
	fi
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
  "normal_user_manual_chromadb_required": $(json_bool "$manual_chromadb_required"),
  "milvus_included": false
}
EOF
}

PLATFORM=${ARCHIVE_CENTER_PLATFORM:-}
REQUESTED_RUNTIME_PROFILE=${AC_RUNTIME_PROFILE:-}
REQUESTED_VECTOR_MODE=${AC_VECTOR_MODE:-}
PREFLIGHT=false
INSTALL_ONLY=false
NO_INSTALL=false
KEEP_SERVICES=false

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
		--keep-services)
			KEEP_SERVICES=true
			shift
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
MARIADB_DATA="$RUNTIME_DIR/mariadb-data"
CHROMA_DATA="$RUNTIME_DIR/chromadb-data"
LOG_DIR="$RUNTIME_DIR/logs"
EXEC_BIN_DIR="$RUNTIME_DIR/bin"
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
prepare_package_binaries
if local_chromadb_requested; then
	ensure_chromadb
fi
find_mariadb_tools

if [ "$INSTALL_ONLY" = "true" ]; then
	log "Install/bootstrap completed. Run this script again without --install-only to start Archive Center."
	exit 0
fi

trap cleanup EXIT INT TERM

start_mariadb
bootstrap_mariadb_schema
start_chromadb

export AC_MODE=live
export AC_STORE_MODE=mariadb_authority
export AC_PROMPT_DIR="$PACKAGE_ROOT/prompts"
export AC_PRUNE_POLICY=${AC_PRUNE_POLICY:-soft}

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

exec "$ARCHIVE_CENTER_GO_RUN"
