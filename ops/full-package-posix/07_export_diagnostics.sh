#!/usr/bin/env sh
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
if [ -f "$SCRIPT_DIR/scripts/export-diagnostics.py" ]; then PACKAGE_ROOT=$SCRIPT_DIR; else PACKAGE_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd -P); fi
if [ -z "${AC_LOG_DIR:-}" ]; then
	if [ -n "${ARCHIVE_CENTER_DATA_DIR:-}" ]; then AC_LOG_DIR="$ARCHIVE_CENTER_DATA_DIR/logs"
	elif printf '%s' "${PREFIX:-}" | grep -qi com.termux; then AC_LOG_DIR="$HOME/.archive-center-2.0/logs"
	else AC_LOG_DIR="$PACKAGE_ROOT/.runtime/logs"; fi
fi
export AC_LOG_DIR
if [ -f "$PACKAGE_ROOT/.env.full.example" ]; then
	AC_BUILD_VERSION=$(sed -n 's/^AC_BUILD_VERSION=//p' "$PACKAGE_ROOT/.env.full.example" | head -n 1 | tr -d '\r')
	export AC_BUILD_VERSION
fi
report_path=${1:-"$HOME/Archive-Center-diagnostics-$(date +%Y%m%d-%H%M%S).json"}
umask 077
if command -v python3 >/dev/null 2>&1; then diagnostic_python=python3; else diagnostic_python=python; fi
"$diagnostic_python" "$PACKAGE_ROOT/scripts/export-diagnostics.py" >"$report_path"
printf 'Report saved: %s\n' "$report_path"
