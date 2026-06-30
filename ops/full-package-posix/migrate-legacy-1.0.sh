#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
PACKAGE_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd -P)

SOURCE_DB=${1:-}
if [ -z "$SOURCE_DB" ]; then
	printf '%s\n' "Archive Center 1.0 -> 2.0 DB migration"
	printf '%s\n' "Usage:"
	printf '  %s /path/to/old/memory.db [--execute]\n' "$0"
	printf '%s\n' ""
	printf '%s\n' "Default is dry-run only. Pass --execute after reviewing the dry-run."
	exit 2
fi

EXECUTE=${2:-}
TOOL="$PACKAGE_ROOT/bin/legacy10-migrate"
if [ ! -x "$TOOL" ]; then
	printf 'ERROR: missing migration tool: %s\n' "$TOOL" >&2
	exit 1
fi
if [ ! -f "$SOURCE_DB" ]; then
	printf 'ERROR: source DB not found: %s\n' "$SOURCE_DB" >&2
	exit 1
fi

WORK_ROOT="${ARCHIVE_CENTER_DATA_DIR:-$PACKAGE_ROOT/.runtime}/legacy-migration"
STAMP=$(date -u +"%Y%m%d-%H%M%S")
WORK_DIR="$WORK_ROOT/$STAMP"
mkdir -p "$WORK_DIR"

REPORT="$WORK_DIR/legacy10-migrate-report.json"
"$TOOL" -sqlite-db "$SOURCE_DB" -work-dir "$WORK_DIR" -out "$REPORT"
printf 'Dry-run report: %s\n' "$REPORT"

if [ "$EXECUTE" != "--execute" ]; then
	printf '%s\n' "Stopped after dry-run. Re-run with --execute to import into MariaDB."
	exit 0
fi

: "${AC_MARIADB_DSN:=archive_center:archive-center-local-pass@tcp(127.0.0.1:3307)/archive_center?parseTime=true}"
EXECUTE_REPORT="$WORK_DIR/legacy10-migrate-execute-report.json"
"$TOOL" -sqlite-db "$SOURCE_DB" -work-dir "$WORK_DIR" -dsn "$AC_MARIADB_DSN" -execute -out "$EXECUTE_REPORT"
printf 'Import report: %s\n' "$EXECUTE_REPORT"
printf '%s\n' "Next: confirm sessions/timeline in Archive Center 2.1, then reindex vectors if ChromaDB search should use imported memories."
