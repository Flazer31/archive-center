#!/usr/bin/env sh
set -u

# Archive Center macOS installer preflight skeleton.
# This script does not install packages, initialize MariaDB, or create ChromaDB runtime state.

usage() {
	cat <<'EOF'
Usage:
  install-macos.sh --preflight [--out PATH] [--data-dir PATH]

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
if [ -n "${ARCHIVE_CENTER_DATA_DIR:-}" ]; then
	DATA_DIR=$ARCHIVE_CENTER_DATA_DIR
elif [ -n "${HOME:-}" ]; then
	DATA_DIR="${HOME}/Library/Application Support/ArchiveCenter"
else
	DATA_DIR="./ArchiveCenterData"
fi

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
ARCH=$(uname -m 2>/dev/null || printf 'unknown')

IS_MACOS=false
case "$UNAME_S" in
	Darwin) IS_MACOS=true ;;
esac
if [ "$IS_MACOS" != "true" ]; then
	append_failure "not_running_on_macos"
fi

ARCH_SUPPORTED=false
case "$ARCH" in
	x86_64|amd64|aarch64|arm64) ARCH_SUPPORTED=true ;;
esac
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

PKG_MANAGER="none"
if has_cmd brew; then
	PKG_MANAGER="brew"
fi

MARIADB_CLIENT_AVAILABLE=false
MARIADB_SERVER_AVAILABLE=false
MARIADB_BUNDLED_AVAILABLE=false
MARIADB_PROVIDER_MODE="missing"
MARIADB_PROVIDER_PATH=""
MARIADB_INSTALLER_MANAGED_REQUIRED=true
MARIADB_REQUIRED_ACTION="bundle runtime/MariaDB/bin/mariadbd or install/start MariaDB through the bootstrap; normal users must not do this manually"
if has_cmd mariadb || has_cmd mysql; then
	MARIADB_CLIENT_AVAILABLE=true
fi
for candidate in \
	"$REPO_ROOT/runtime/MariaDB/bin/mariadbd" \
	"$REPO_ROOT/runtime/mariadb/bin/mariadbd" \
	"$REPO_ROOT/vendor/MariaDB/bin/mariadbd" \
	"$REPO_ROOT/vendor/mariadb/bin/mariadbd"
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
elif [ "$PKG_MANAGER" = "brew" ]; then
	MARIADB_PROVIDER_MODE="homebrew_required"
	MARIADB_REQUIRED_ACTION="installer/bootstrap must install and initialize MariaDB through Homebrew"
	append_warning "mariadb_provider_homebrew_required"
else
	append_warning "mariadb_server_command_not_found"
fi

PYTHON_AVAILABLE=false
CHROMADB_AVAILABLE=false
CHROMA_PROVIDER_MODE="missing"
CHROMA_INSTALLER_MANAGED_REQUIRED=true
CHROMA_REQUIRED_ACTION="installer/bootstrap must provide chromadb or select an approved vector fallback; normal users must not do this manually"
if has_cmd python3; then
	PYTHON_AVAILABLE=true
	if python3 -c 'import chromadb' >/dev/null 2>&1; then
		CHROMADB_AVAILABLE=true
	fi
else
	append_warning "python3_not_found"
fi

if [ "$CHROMADB_AVAILABLE" != "true" ]; then
	append_warning "chromadb_not_available"
fi
if [ "$CHROMADB_AVAILABLE" = "true" ]; then
	CHROMA_PROVIDER_MODE="python_local_package"
	CHROMA_INSTALLER_MANAGED_REQUIRED=false
	CHROMA_REQUIRED_ACTION="use detected chromadb runtime"
elif [ "$PYTHON_AVAILABLE" = "true" ]; then
	CHROMA_PROVIDER_MODE="installer_python_package_required"
	CHROMA_REQUIRED_ACTION="installer/bootstrap must install chromadb into a managed runtime"
fi

SUPPORT_LEVEL="green"
PREFLIGHT_STATUS="ok"
FALLBACK_PROFILE="none"

if [ -n "$FAILURES" ]; then
	SUPPORT_LEVEL="red"
	PREFLIGHT_STATUS="unsupported"
	FALLBACK_PROFILE="none"
elif [ "$MARIADB_SERVER_AVAILABLE" != "true" ] || [ "$CHROMADB_AVAILABLE" != "true" ]; then
	SUPPORT_LEVEL="yellow"
	PREFLIGHT_STATUS="degraded"
	FALLBACK_PROFILE="macos_light_or_managed_dependency_profile_required"
else
	SUPPORT_LEVEL="green"
	PREFLIGHT_STATUS="ok"
	FALLBACK_PROFILE="none"
fi

REPORT=$(cat <<EOF
{
  "schema_version": "archive-center.preflight.v1",
  "target": "macos",
  "preflight_only": true,
  "platform": $(json_string "$UNAME_S"),
  "platform_detail": "",
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
  "macos": {
    "detected": $(json_bool "$IS_MACOS"),
    "package_manager": $(json_string "$PKG_MANAGER")
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
    "probe_available": false,
    "go_28080": "unknown",
    "mariadb_3307": "unknown"
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
