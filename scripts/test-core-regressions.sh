#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT/go-service"

if ! command -v node >/dev/null 2>&1; then
  printf '%s\n' '[core-regression] Node.js is required; refusing to skip JavaScript runtime fixtures' >&2
  exit 1
fi
ARCHIVE_CENTER_NODE_BINARY=$(command -v node)
export ARCHIVE_CENTER_NODE_BINARY
GOCACHE=${GOCACHE:-${TMPDIR:-/tmp}/archive-center-core-regression-go-build}
mkdir -p "$GOCACHE"
export GOCACHE

# Keep this list synchronized with testdata/core-regression-suite.json.
# Every selected test uses source-extracted JavaScript, httptest, fake Store,
# or fake Vector implementations. No live DB or user session is contacted.
run_suite() {
  package=$1
  pattern=$2
  expected=$3
  found=$(go test "$package" -list "$pattern" | grep -c '^Test' || true)
  if [ "$found" -ne "$expected" ]; then
    printf '%s\n' "[core-regression] discovery mismatch for $package: found $found, expected $expected" >&2
    exit 1
  fi
  go test "$package" -run "$pattern" -count=1
}

run_suite ./cmd/js-route-variant-smoke '^(TestCoreRegressionInputOwnershipAndCanonicalOutputRuntime|TestArchiveCenterJSOnlyModelTypeEntersPersistence|TestArchiveCenterJSModelPayloadIsNotBlockedByPromptMarkersOrTailMismatch|TestArchiveCenterJSRerollRollbackPath|TestArchiveCenterJSSeq13P181VX1bCopiedSessionDeletionReplayMarkers)$' 5
run_suite ./internal/httpapi '^(TestCompleteTurnIdempotentExecutionRunsWriterOnce|TestCompleteTurnIdempotentReplaySkipsDuplicateDerivedWrites|TestCompleteTurnPostprocessorPairAlreadyPersistedOnAnotherTurnSkipsDuplicate|TestCompleteTurnEmbeddingProviderFailureReportsWarning)$' 4
run_suite ./internal/httpapi '^(TestRollbackLiveWriteExecutesDeletions|TestRollbackMinFromTurnClampsAttachedSessionDelete|TestSessionMigratePreviewDryRunBlocksNonEmptyTarget|TestSessionMigratePreviewAllowsEmptyTargetDryRun|TestSessionMigrateCompleteCopiesOnlyAfterEmptyTargetPreview|TestSessionMigrationSourceLockExcludesPrepareSearchAndCompleteTurn)$' 6

printf '%s\n' '[core-regression] all isolated suites passed'
