#!/usr/bin/env sh
set -u

# Archive Center 2.1 Termux installer preflight skeleton.
# This script does not install, initialize, or mutate MariaDB/ChromaDB runtime state.

usage() {
	cat <<'EOF'
Usage:
  install-termux.sh --preflight [--out PATH] [--data-dir PATH]

Options:
  --preflight       Run JSON preflight only. Required.
  --out PATH        Write JSON report to PATH instead of stdout.
  --data-dir PATH   Override target data dir for write/path checks.
  -h, --help        Show this help.
EOF
}

json_escape() {
	printf '%s' "$1" | tr '\n' ' ' | sed 's/\\/\\\\/g; s/"/\\"/g; s/	/\\t/g'
}

json_string() {
	printf '"%s"' "$(json_escape "$1")"
}

json_bool() {
	if [ "$1" = "true" ]; then
		printf 'true'
	else
		printf 'false'
	fi
}

json_array_pipe() {
	value=$1
	if [ -z "$value" ]; then
		printf '[]'
		return
	fi

	old_ifs=$IFS
	IFS='|'
	first=true
	printf '['
	for item in $value; do
		if [ "$first" = "true" ]; then
			first=false
		else
			printf ','
		fi
		json_string "$item"
	done
	IFS=$old_ifs
	printf ']'
}

has_cmd() {
	command -v "$1" >/dev/null 2>&1
}

append_warning() {
	if [ -z "$WARNINGS" ]; then
		WARNINGS=$1
	else
		WARNINGS="${WARNINGS}|$1"
	fi
}

append_failure() {
	if [ -z "$FAILURES" ]; then
		FAILURES=$1
	else
		FAILURES="${FAILURES}|$1"
	fi
}

canonical_existing_dir() {
	if [ -d "$1" ]; then
		(cd "$1" 2>/dev/null && pwd -P) || printf '%s' "$1"
	else
		printf '%s' "$1"
	fi
}

PREFLIGHT=false
OUT_PATH=""
DATA_DIR="${ARCHIVE_CENTER_DATA_DIR:-${HOME:-.}/.archive-center}"

while [ "$#" -gt 0 ]; do
	case "$1" in
		--preflight)
			PREFLIGHT=true
			shift
			;;
		--out|-o)
			if [ "$#" -lt 2 ]; then
				echo "missing value for --out" >&2
				exit 2
			fi
			OUT_PATH=$2
			shift 2
			;;
		--data-dir)
			if [ "$#" -lt 2 ]; then
				echo "missing value for --data-dir" >&2
				exit 2
			fi
			DATA_DIR=$2
			shift 2
			;;
		-h|--help)
			usage
			exit 0
			;;
		*)
			echo "unknown argument: $1" >&2
			usage >&2
			exit 2
			;;
	esac
done

if [ "$PREFLIGHT" != "true" ]; then
	usage >&2
	exit 2
fi

WARNINGS=""
FAILURES=""

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" 2>/dev/null && pwd -P)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." 2>/dev/null && pwd -P)
UNAME_S=$(uname -s 2>/dev/null || printf 'unknown')
UNAME_O=$(uname -o 2>/dev/null || printf '')
ARCH=$(uname -m 2>/dev/null || printf 'unknown')
PREFIX_VALUE=${PREFIX:-}

IS_TERMUX=false
case "$PREFIX_VALUE" in
	*com.termux*) IS_TERMUX=true ;;
esac
if has_cmd termux-info; then
	IS_TERMUX=true
fi
if printf '%s' "$UNAME_O" | grep -qi 'Android'; then
	IS_TERMUX=true
fi

ARCH_SUPPORTED=false
case "$ARCH" in
	aarch64|arm64|x86_64|amd64) ARCH_SUPPORTED=true ;;
esac

if [ "$IS_TERMUX" != "true" ]; then
	append_failure "not_running_inside_termux"
fi
if [ "$ARCH_SUPPORTED" != "true" ]; then
	append_failure "unsupported_architecture"
fi

DATA_PARENT=$(dirname -- "$DATA_DIR")
if [ -d "$DATA_DIR" ]; then
	DATA_ABS=$(canonical_existing_dir "$DATA_DIR")
elif [ -d "$DATA_PARENT" ]; then
	PARENT_ABS=$(canonical_existing_dir "$DATA_PARENT")
	DATA_ABS="${PARENT_ABS}/$(basename -- "$DATA_DIR")"
else
	DATA_ABS=$DATA_DIR
fi

DATA_OUTSIDE_SOURCE=true
case "$DATA_ABS" in
	"$REPO_ROOT"|"$REPO_ROOT"/*)
		DATA_OUTSIDE_SOURCE=false
		append_failure "data_dir_inside_source_tree"
		;;
esac

WRITE_PROBE_OK=false
WRITE_PROBE_TARGET=""
if [ "$DATA_OUTSIDE_SOURCE" = "true" ]; then
	if [ -d "$DATA_DIR" ]; then
		WRITE_PROBE_TARGET=$DATA_DIR
	elif [ -d "$DATA_PARENT" ]; then
		WRITE_PROBE_TARGET=$DATA_PARENT
	fi
	if [ -n "$WRITE_PROBE_TARGET" ]; then
		PROBE_FILE="${WRITE_PROBE_TARGET}/.archive-center-preflight.$$"
		if (umask 077 && : > "$PROBE_FILE") 2>/dev/null; then
			rm -f "$PROBE_FILE"
			WRITE_PROBE_OK=true
		else
			append_failure "install_target_not_writable"
		fi
	else
		append_failure "install_target_parent_missing"
	fi
fi

GO_BINARY="${ARCHIVE_CENTER_GO_BINARY:-$REPO_ROOT/go-service/archive-center-go}"
GO_BINARY_PRESENT=false
if [ -x "$GO_BINARY" ]; then
	GO_BINARY_PRESENT=true
else
	append_warning "go_backend_binary_not_found"
fi

GO_TOOL_AVAILABLE=false
if has_cmd go; then
	GO_TOOL_AVAILABLE=true
fi

MARIADB_CLIENT_AVAILABLE=false
MARIADB_SERVER_AVAILABLE=false
MARIADB_BUNDLED_AVAILABLE=false
MARIADB_PROVIDER_MODE="missing"
MARIADB_PROVIDER_PATH=""
MARIADB_INSTALLER_MANAGED_REQUIRED=true
MARIADB_REQUIRED_ACTION="bundle a Termux-compatible MariaDB provider or install/start MariaDB through the Termux bootstrap; normal users must not do this manually"
if has_cmd mariadb || has_cmd mysql; then
	MARIADB_CLIENT_AVAILABLE=true
fi
for candidate in \
	"$REPO_ROOT/runtime/MariaDB/bin/mariadbd" \
	"$REPO_ROOT/runtime/mariadb/bin/mariadbd" \
	"$REPO_ROOT/vendor/MariaDB/bin/mariadbd" \
	"$REPO_ROOT/vendor/mariadb/bin/mariadbd" \
	"$REPO_ROOT/resources/MariaDB/bin/mariadbd" \
	"$REPO_ROOT/resources/mariadb/bin/mariadbd"
do
	if [ -x "$candidate" ]; then
		MARIADB_BUNDLED_AVAILABLE=true
		MARIADB_PROVIDER_PATH=$candidate
		break
	fi
done
if [ "$MARIADB_BUNDLED_AVAILABLE" = "true" ]; then
	MARIADB_SERVER_AVAILABLE=true
	MARIADB_PROVIDER_MODE="bundled_runtime"
	MARIADB_INSTALLER_MANAGED_REQUIRED=false
	MARIADB_REQUIRED_ACTION="use bundled MariaDB provider"
elif has_cmd mariadbd; then
	MARIADB_SERVER_AVAILABLE=true
	MARIADB_PROVIDER_MODE="system_command"
	MARIADB_PROVIDER_PATH=$(command -v mariadbd 2>/dev/null || printf '')
	MARIADB_INSTALLER_MANAGED_REQUIRED=false
	MARIADB_REQUIRED_ACTION="use detected mariadbd command"
elif has_cmd mysqld; then
	MARIADB_SERVER_AVAILABLE=true
	MARIADB_PROVIDER_MODE="system_command"
	MARIADB_PROVIDER_PATH=$(command -v mysqld 2>/dev/null || printf '')
	MARIADB_INSTALLER_MANAGED_REQUIRED=false
	MARIADB_REQUIRED_ACTION="use detected mysqld command"
elif has_cmd pkg || has_cmd apt; then
	MARIADB_PROVIDER_MODE="termux_pkg_required"
	MARIADB_REQUIRED_ACTION="Termux bootstrap must install and initialize MariaDB through pkg/apt"
	append_warning "mariadb_provider_termux_pkg_required"
else
	append_warning "mariadb_server_command_not_found"
fi

TERMUX_PKG_AVAILABLE=false
if has_cmd pkg || has_cmd apt; then
	TERMUX_PKG_AVAILABLE=true
else
	append_warning "termux_package_manager_not_found"
fi

PYTHON_AVAILABLE=false
CHROMADB_AVAILABLE=false
CHROMA_PROVIDER_MODE="missing"
CHROMA_INSTALLER_MANAGED_REQUIRED=true
CHROMA_REQUIRED_ACTION="Termux bootstrap will prepare ChromaDB inside a managed proot-distro Ubuntu runtime; normal users must not compile native Python packages manually"
if has_cmd python3; then
	PYTHON_AVAILABLE=true
	if python3 -c 'import chromadb' >/dev/null 2>&1; then
		CHROMADB_AVAILABLE=true
	fi
else
	append_warning "python3_not_found"
fi

if [ "$CHROMADB_AVAILABLE" != "true" ]; then
	append_warning "termux_native_chromadb_not_available_using_proot_distro"
fi
if [ "$CHROMADB_AVAILABLE" = "true" ]; then
	CHROMA_PROVIDER_MODE="python_local_package"
	CHROMA_INSTALLER_MANAGED_REQUIRED=false
	CHROMA_REQUIRED_ACTION="use detected chromadb runtime"
elif has_cmd proot-distro || has_cmd pkg || has_cmd apt; then
	CHROMA_PROVIDER_MODE="termux_proot_distro_managed_required"
	CHROMA_REQUIRED_ACTION="Termux bootstrap must install proot-distro and prepare ChromaDB inside Ubuntu/proot"
elif [ "$PYTHON_AVAILABLE" = "true" ]; then
	CHROMA_PROVIDER_MODE="termux_proot_distro_missing"
	CHROMA_REQUIRED_ACTION="Termux bootstrap requires proot-distro for local ChromaDB"
fi

PORT_PROBE_AVAILABLE=false
GO_PORT_STATUS="unknown"
MARIADB_PORT_STATUS="unknown"
if [ "$PYTHON_AVAILABLE" = "true" ]; then
	PORT_PROBE_AVAILABLE=true
	GO_PORT_STATUS=$(python3 -c 'import socket; s=socket.socket(); ok=True
try:
    s.bind(("127.0.0.1", 28080))
except OSError:
    ok=False
finally:
    s.close()
print("free" if ok else "busy")' 2>/dev/null || printf 'unknown')
	MARIADB_PORT_STATUS=$(python3 -c 'import socket; s=socket.socket(); ok=True
try:
    s.bind(("127.0.0.1", 3307))
except OSError:
    ok=False
finally:
    s.close()
print("free" if ok else "busy")' 2>/dev/null || printf 'unknown')
fi

SUPPORT_LEVEL="green"
PREFLIGHT_STATUS="ok"
FALLBACK_PROFILE="none"

if [ -n "$FAILURES" ]; then
	SUPPORT_LEVEL="red"
	PREFLIGHT_STATUS="unsupported"
	FALLBACK_PROFILE="none"
elif [ "$MARIADB_SERVER_AVAILABLE" != "true" ]; then
	SUPPORT_LEVEL="yellow"
	PREFLIGHT_STATUS="degraded"
	FALLBACK_PROFILE="termux_mariadb_pkg_bootstrap_required"
else
	SUPPORT_LEVEL="green"
	PREFLIGHT_STATUS="ok"
	FALLBACK_PROFILE="none"
fi

REPORT=$(cat <<EOF
{
  "schema_version": "archive-center.preflight.v1",
  "target": "termux",
  "preflight_only": true,
  "platform": $(json_string "$UNAME_S"),
  "platform_detail": $(json_string "$UNAME_O"),
  "arch": $(json_string "$ARCH"),
  "support_level": $(json_string "$SUPPORT_LEVEL"),
  "preflight_status": $(json_string "$PREFLIGHT_STATUS"),
  "install_status": "not_run_preflight_only",
  "fallback_profile": $(json_string "$FALLBACK_PROFILE"),
  "paths": {
    "repo_root": $(json_string "$REPO_ROOT"),
    "data_dir": $(json_string "$DATA_ABS"),
    "data_path_outside_source": $(json_bool "$DATA_OUTSIDE_SOURCE"),
    "write_probe_target": $(json_string "$WRITE_PROBE_TARGET"),
    "write_probe_ok": $(json_bool "$WRITE_PROBE_OK")
  },
  "termux": {
    "detected": $(json_bool "$IS_TERMUX"),
    "prefix": $(json_string "$PREFIX_VALUE"),
    "package_manager_available": $(json_bool "$TERMUX_PKG_AVAILABLE")
  },
  "go_backend": {
    "binary_path": $(json_string "$GO_BINARY"),
    "binary_present": $(json_bool "$GO_BINARY_PRESENT"),
    "go_tool_available": $(json_bool "$GO_TOOL_AVAILABLE"),
    "health_status": "not_run_preflight_only",
    "ready_status": "not_run_preflight_only",
    "version_status": "not_run_preflight_only"
  },
  "mariadb": {
    "client_available": $(json_bool "$MARIADB_CLIENT_AVAILABLE"),
    "server_command_available": $(json_bool "$MARIADB_SERVER_AVAILABLE"),
    "bundled_provider_available": $(json_bool "$MARIADB_BUNDLED_AVAILABLE"),
    "provider_mode": $(json_string "$MARIADB_PROVIDER_MODE"),
    "provider_path": $(json_string "$MARIADB_PROVIDER_PATH"),
    "installer_managed_required": $(json_bool "$MARIADB_INSTALLER_MANAGED_REQUIRED"),
    "normal_user_manual_mariadb_required": false,
    "required_action": $(json_string "$MARIADB_REQUIRED_ACTION"),
    "schema_status": "not_run_preflight_only",
    "smoke_status": "not_run_preflight_only"
  },
  "chromadb": {
    "python3_available": $(json_bool "$PYTHON_AVAILABLE"),
    "chromadb_available": $(json_bool "$CHROMADB_AVAILABLE"),
    "provider_mode": $(json_string "$CHROMA_PROVIDER_MODE"),
    "installer_managed_required": $(json_bool "$CHROMA_INSTALLER_MANAGED_REQUIRED"),
    "normal_user_manual_chromadb_required": false,
    "required_action": $(json_string "$CHROMA_REQUIRED_ACTION"),
    "smoke_status": "not_run_preflight_only"
  },
  "ports": {
    "probe_available": $(json_bool "$PORT_PROBE_AVAILABLE"),
    "go_28080": $(json_string "$GO_PORT_STATUS"),
    "mariadb_3307": $(json_string "$MARIADB_PORT_STATUS")
  },
  "warnings": $(json_array_pipe "$WARNINGS"),
  "failures": $(json_array_pipe "$FAILURES")
}
EOF
)

if [ -n "$OUT_PATH" ]; then
	OUT_PARENT=$(dirname -- "$OUT_PATH")
	if [ -n "$OUT_PARENT" ] && [ "$OUT_PARENT" != "." ]; then
		mkdir -p "$OUT_PARENT"
	fi
	printf '%s\n' "$REPORT" > "$OUT_PATH"
else
	printf '%s\n' "$REPORT"
fi

case "$SUPPORT_LEVEL" in
	green) exit 0 ;;
	yellow) exit 0 ;;
	*) exit 1 ;;
esac
