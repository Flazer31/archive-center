#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
ARCHIVE_CENTER_PACKAGE_ROOT=${ARCHIVE_CENTER_PACKAGE_ROOT:-$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd -P)}
export ARCHIVE_CENTER_PACKAGE_ROOT
exec sh "$SCRIPT_DIR/start-full-posix.sh" --platform linux "$@"
