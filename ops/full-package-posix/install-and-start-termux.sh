#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
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
	done
	return 1
}

PWD_DIR=$(pwd -P 2>/dev/null || pwd)
ARCHIVE_CENTER_PACKAGE_ROOT=$(resolve_package_root \
	"${ARCHIVE_CENTER_PACKAGE_ROOT:-}" \
	"$SCRIPT_DIR" \
	"$SCRIPT_DIR/.." \
	"$PWD_DIR" \
	"$PWD_DIR/.." \
	2>/dev/null || true)
[ -n "$ARCHIVE_CENTER_PACKAGE_ROOT" ] || {
	printf 'ERROR: Archive Center package root was not found. Run this from the extracted package folder containing bin, migrations, prompts, and scripts.\n' >&2
	exit 1
}
export ARCHIVE_CENTER_PACKAGE_ROOT

if [ -f "$SCRIPT_DIR/start-full-posix.sh" ]; then
	START_SCRIPT="$SCRIPT_DIR/start-full-posix.sh"
elif [ -f "$SCRIPT_DIR/scripts/start-full-posix.sh" ]; then
	START_SCRIPT="$SCRIPT_DIR/scripts/start-full-posix.sh"
else
	printf 'ERROR: start-full-posix.sh was not found next to the launcher or under scripts/.\n' >&2
	exit 1
fi

exec sh "$START_SCRIPT" --platform termux "$@"
