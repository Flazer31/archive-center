# Archive Center 2.1 Windows Package

This is the Windows package for Archive Center 2.1.

It includes:

- Go backend
- next-start package updater
- MariaDB runtime
- ChromaDB/Python runtime
- Archive Center.js
- migrations
- prompts

Normal users do not need to install MariaDB or ChromaDB manually.

## Start

Double-click:

```text
01_start_archive_center_windows.bat
```

The launcher binds the backend to `0.0.0.0:28080`, so the same file works for both same-PC and remote-browser use.
It creates `.env.full.local` if it does not exist, starts bundled MariaDB, starts bundled ChromaDB, applies schema migrations, and starts the Go backend.

Leave the console window open while using Archive Center.

## Updates are applied on the next start

The Archive Center settings UI can check for an update and download a verified
package into `.updates/`. It does not apply packages in the background and does
not interrupt a running story session.

On the next launcher start, before the env file, MariaDB, ChromaDB, or backend
is opened, the launcher runs a temporary copy of `archive-center-updater.exe`.
If no verified pending package exists, startup follows the normal path without
changing package files. If a package is pending, the updater verifies the
managed payload manifest, stages an atomic replacement, and keeps the old
managed files available for rollback.

After the existing local runtime services and additive schema migrations are
ready, the launcher starts the updated backend and checks `/ready`. Only the
main `ready` result is the update gate; an optional original-work reference
vector degradation does not reject an otherwise healthy backend. A successful
check commits the package. A failed check stops the candidate backend, restores
the previous managed files, and starts the previous backend. If the updater
cannot prove either `no_mutation` or a safe rollback, startup stops with a
recovery error instead of running a mixed package.

Updates do not move, replace, or copy `.runtime/`, `.updates/`,
`.env.full.local`, or `.env.full.local.protected`. MariaDB and ChromaDB keep
using their existing package-local data directories. The v1 automatic updater
rejects a package that adds or changes managed migration SQL or
`mariadb-schema.exe`; database-changing releases require a separately reviewed
manual migration path.

## RisuAI setup

1. Register `Archive Center.js` from this folder as the RisuAI plugin.
2. Use this backend URL:

```text
http://127.0.0.1:28080
```

If RisuAI is opened from another PC or phone, do not use `localhost`.
Remote-browser `localhost` means the user's device, not the server PC.
Use the server PC's reachable backend URL instead:

```text
http://SERVER_IP_OR_DOMAIN:28080
```

`SERVER_IP_OR_DOMAIN` can be a LAN IP, direct connection IP, VPN/Tailscale IP, forwarded public IP, or domain name. If the RisuAI page is HTTPS and the browser blocks HTTP backend calls, expose port 28080 through Tailscale Serve or another HTTPS proxy and use that HTTPS URL.

## Optional smoke test

After the server is running, double-click:

```text
02_smoke_test_windows.bat
```

## Defender or SmartScreen

Archive Center does not disable Microsoft Defender and does not add Defender
exclusions automatically.

If Defender or SmartScreen blocks a file, run the read-only trust report:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\check-windows-trust.ps1
```

Then open `.runtime\reports\windows-trust-report.json` and submit the exact
detected file, detection name, and SHA256 to Microsoft Security Intelligence.
See `WINDOWS_TRUST_AND_DEFENDER.md` for the submission checklist.

## Optional 1.0 DB migration

If you have an old Archive Center 1.0 `memory.db`, start Archive Center 2.1 first,
then double-click:

```text
06_migrate_1_0_to_2_0_windows.bat
```

The first pass is a dry-run only. It reads the old SQLite DB in read-only mode,
exports recognized tables, validates the export, and writes a report under
`.runtime/legacy-migration/`. It imports into MariaDB only after you type `YES`.

After importing, open the Archive Center UI and confirm sessions/timeline data.
Run vector reindex if imported memories should become searchable through
ChromaDB.

## Protect local env secrets

If you put API keys or private endpoints into `.env.full.local`, double-click:

```text
04_protect_env_windows.bat
```

This writes `.env.full.local.protected` with Windows DPAPI encryption tied to the current Windows user account, then removes the plaintext `.env.full.local`.

To edit settings later, double-click:

```text
05_unprotect_env_windows.bat
```

Edit `.env.full.local`, then run `04_protect_env_windows.bat` again.

## Do not ship local runtime data

Do not put these into a release zip:

- `.runtime/`
- database files
- ChromaDB persist data
- API keys
- `.git`
- `.runtime-cache`
