# Archive Center Public Release Scope

This document defines the source files that may be published as Archive Center
and the files that must stay outside the public repository. Publishing is
blocked until every item in the pre-publication gate is satisfied.

## Public project scope

The following paths are Archive Center source and may be published after the
secret, personal-path, provenance, and license audits pass:

- `Archive Center.js`
- `go-service/`
- `migrations/`
- canonical files under `prompts/`
- `ops/`, `scripts/`, and `tools/`
- `tests/` and `testdata/`
- reviewed `contracts/` and `docs/`
- `.github/workflows/`
- `.env.example` and versioned `*.env*.example` templates containing no real
  credentials or private endpoints
- `README.md`, `NOTICE`, `THIRD_PARTY_NOTICES.md`, and the project `LICENSE`

Archive Center release binaries must be reproducible from the public source
and versioned build scripts. Release ZIP files, runtime downloads, local DBs,
and generated test reports are not source files.

## Separate projects and repository-local instructions

The following tracked files are not part of Archive Center and must be removed
from the public Git index before publication:

- `Risu Recomposer.js`
- `AC Recomposer Agent.js`
- `Archive Center 3.4-C.js`
- `AGENTS.md`
- `.github/copilot-instructions.md`
- non-canonical prompt copies; only `prompts/critic_system.txt` and
  `prompts/supervisor_system.txt` belong in the public project

Removing these files from the Archive Center public scope must not delete or
rewrite their separate local source copies. `Risu Recomposer.js` remains a
separate plugin project.

## Never publish

The public repository, release assets, and build artifacts must never contain:

- `.env` or `.env.*` files other than reviewed example templates
- API keys, access tokens, passwords, DSNs with credentials, private endpoints,
  SSH keys, signing keys, certificates, or encrypted secret stores
- MariaDB, SQLite, ChromaDB, or other user database and vector-persist data
- `.runtime/`, `.runtime-cache/`, `.updates/`, caches, logs, crash dumps, or
  temporary files
- `_dist-*`, release, deploy, live-test, recovery, security-audit, or local
  VirusTotal test-output directories
- local backups, copied files, editor artifacts, personal attachments, or
  unrelated images
- MariaDB server binaries; the Windows launcher obtains the verified official
  runtime separately
- user `.env` and DB data in any installation or update ZIP

## Tracked content that requires sanitization

The following tracked source or documents contain historical absolute local
paths. They may remain public only after those paths are replaced with neutral
fixtures, arguments, or placeholders:

- `contracts/2.0-4-R1-11-resume-pack-extraction-contract.md`
- `contracts/openapi-contract-freeze.md`
- `contracts/openapi-contract-summary.json`
- `contracts/openapi-schema-freeze.json`
- `contracts/openapi-schema-inventory.md`
- `go-service/cmd/fixture-live-runner/main.go`
- `go-service/cmd/js-route-variant-smoke/main_test.go`
- `ops/packaging-hygiene.md`
- `tools/extract_openapi_contract.py`

Historical drive paths are not secrets by themselves, but publishing them is
unnecessary personal and workstation metadata. Sanitization must preserve test
meaning and must not replace a real production dependency with a fake success.

## Generated and third-party payloads

- ChromaDB/Python runtime files may be distributed only in release packages
  with their upstream license files retained; they are not committed as project
  source.
- MariaDB is installed as a separate official runtime and is not committed or
  bundled.
- Generated binaries, ZIP files, checksums, SBOMs, and attestations belong in
  release assets or CI artifacts, not in the source tree.
- `THIRD_PARTY_NOTICES.md` and exact dependency manifests must accompany
  releases.

## Pre-publication gate

Publication is allowed only after all of the following are complete:

1. The separate and repository-local files above are absent from the public
   Git index.
2. Current files and full Git history pass a secret scan.
3. Historical absolute local paths are sanitized without changing runtime
   behavior.
4. Third-party code provenance and license compatibility are reviewed.
5. An OSI-approved project `LICENSE` is committed.
6. A clean checkout can build and test the declared release packages without
   private files or untracked local inputs.
7. Release artifacts publish SHA-256 checksums and verifiable build provenance.

This scope document does not itself authorize publication or removal of local
files. It defines the boundary that the later cleanup and release steps must
enforce.
