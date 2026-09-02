# Archive Center Windows Auto Install Package

This is the lightweight Windows auto-install package for the version recorded in
`PACKAGE_FILE_MANIFEST.json`.

It includes:

- Go backend
- next-start package updater
- per-user MariaDB and ChromaDB runtime installers
- Archive Center.js
- migrations
- prompts

MariaDB, Python, and ChromaDB are not contained in the Archive Center ZIP. On
first start, the launcher downloads the pinned official MariaDB ZIP and the
official CPython installer. It verifies both SHA-256 values, also requires a
valid Python Software Foundation Authenticode signature, and installs the
runtimes under `%LOCALAPPDATA%\ArchiveCenter\runtime`. ChromaDB 1.5.9 is then
installed into a dedicated managed Python environment.

The standard runtime profile is `full_local` with vector mode `bundled`, so the
normal first start prepares and starts both MariaDB and local ChromaDB. The
first start requires an internet connection and can take several minutes.
No timeout value must be added to `.env.full.local`; the Windows launcher uses
the validated blocking install and readiness flow recorded by this package.
The local full package requires ChromaDB. It installs and starts the pinned
managed ChromaDB runtime by default, or it can verify a configured external
ChromaDB endpoint. Startup stops if the endpoint and its upsert/readback/delete
round trip cannot be verified.

## Start

Double-click:

```text
01_start_archive_center_windows.bat
```

The launcher binds the backend to `0.0.0.0:28080`, so the same file works for both same-PC and remote-browser use.
It creates `.env.full.local` if it does not exist, prepares or starts the
separate per-user MariaDB runtime, applies schema migrations, and starts the Go
backend. It does not install a Windows service or require administrator rights.

Leave the console window open while using Archive Center.

## Stop or cancel shutdown

While the backend is running, press `Ctrl+C`. The PowerShell launcher asks
whether to stop Archive Center and every managed service.

- Enter `N` to cancel. The same backend, MariaDB, and ChromaDB processes keep
  running.
- Enter `Y` to enter the existing bounded cleanup path and stop the managed
  process group.

The launcher prevents the managed child processes from receiving the console
signal before this choice. The generic `cmd.exe` batch prompt is not the owner
of service shutdown and `N` is not implemented by restarting an already stopped
backend.

After a confirmed `Y` shutdown, Windows can additionally display `Terminate
batch job (Y/N)?`. Enter `Y` there to close the BAT window. That second generic
prompt appears only after service shutdown was confirmed; entering `N` there
does not restart the stopped services.

After a successful shutdown the BAT exits without an additional `pause`. It
pauses only after a nonzero launcher exit so the error code remains visible.

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
`.env.full.local`, or `.env.full.local.protected`. MariaDB and any separately
configured ChromaDB keep using their existing data directories. The MariaDB
executable runtime remains outside the versioned package under
`%LOCALAPPDATA%\ArchiveCenter`. The separately installed Python and ChromaDB
runtime also remain outside the package. The v1 automatic updater
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

Legacy 1.0 migration executables are not part of the normal Windows package.
They are offline migration utilities, not normal startup dependencies. Users
who still need to migrate an old `memory.db` must use the separately published
Legacy Migration Tools package that matches this Archive Center release.

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
- MariaDB, Python, or ChromaDB runtime binaries
- database files
- ChromaDB persist data
- API keys
- `.git`
- `.runtime-cache`
