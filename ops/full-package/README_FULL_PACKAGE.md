# Archive Center 2.1 Windows Package

This is the Windows package for Archive Center 2.1.

It includes:

- Go backend
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
- Milvus runtime/data
