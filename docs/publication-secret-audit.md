# Publication Secret Audit

Audit date: 2026-07-17

This audit checks whether the Archive Center source boundary can proceed toward
public release. It does not authorize publication by itself.

## Scope and tools

- Gitleaks 8.30.1 official Windows x64 release
- Official release checksum verified before execution
- Gitleaks Git-history scan with full secret redaction
- Gitleaks directory/file scans over the 20 current public-scope source targets
- Independent filename-only high-signal pattern scan over all 99 reachable Git
  commits
- Historical filename review for environment files, private keys,
  certificates, credential stores, and databases
- Reviewed environment-example assignments without printing their values

Generated reports were written outside the source repository under the local
temporary audit directory. They are not publication artifacts and must not be
committed.

## Result

No real credential, private key, provider token, or credential-bearing remote
URL was found.

Gitleaks reported five historical findings across three commits and five file
paths. The current public-scope scan reported four findings across four files.
Every finding used the `generic-api-key` rule and was classified as a test
fixture rather than a live credential:

- Go HTTP API tests intentionally use fake `sk-`-shaped values to verify secret
  storage, routing, and masking behavior.
- `Risu Recomposer.js` and its local copy use a fake `sk-`-shaped value in a
  masking test. Recomposer is outside the Archive Center public scope.
- All inspected findings contained an explicit test marker such as `test`,
  `fake`, or `secret`; none was a live-length provider credential supplied by a
  user.

The independent scan found zero files containing high-signal private-key
headers or GitHub, AWS, Google, Slack, or similar provider-token patterns.

## Environment and repository metadata

- The only historically tracked environment files are reviewed example
  templates: `.env.example`, `ops/full-package/.env.full.example`, and
  `ops/live-test-pack/.env.live.example`.
- API-key, bearer-token, MariaDB DSN, and external-endpoint fields in those
  templates are blank.
- The Git remote uses HTTPS to GitHub and contains no embedded credentials.
- The repository has no Git submodules.
- No historically tracked private-key, certificate, credential-store, SQLite,
  MariaDB, or user DB filename was found.

## Important boundary

Ignored and generated local directories contain build caches, old packages,
runtime payloads, and user-controlled files. They are deliberately outside the
public source scan and are not cleared for publication. A public release must
be built from a clean checkout using the allowlist in
`docs/public-release-scope.md`; copying the current workspace directory is
prohibited.

The separate `Risu Recomposer.js`, local agent instructions, prompt copies,
release packages, runtime directories, recovery directories, security-audit
directories, `.env` files, DB files, and personal attachments remain excluded.

## Remaining actions

1. Remove excluded tracked files from the future public Git index without
   deleting their separate local copies.
2. Sanitize historical workstation paths listed in
   `docs/public-release-scope.md`.
3. Add a reviewed Gitleaks check to public CI so new commits cannot introduce
   credentials.
4. Repeat the redacted history scan after any history rewrite or repository
   split.

Secret-audit status: **pass with classified test fixtures**.
