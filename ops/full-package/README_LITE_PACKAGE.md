# Deprecated Archive Center Windows Lite Note

Deprecated: separate Lite packages are retired for Archive Center 2.3. Use the
standard package and choose a lighter runtime profile if needed.

This is the historical Windows lightweight package note for Archive Center 2.1.

It includes:

- Go backend
- MariaDB runtime
- Archive Center.js
- migrations
- prompts

Lite defaults to `core_lite` / `fallback`. ChromaDB is not bundled or started by
default. Normal save, chat logs, and canonical memory persistence use MariaDB;
vector recall can be added later with an external ChromaDB service.

## Start

Double-click:

```text
01_start_archive_center_windows.bat
```

The launcher binds the backend to `0.0.0.0:28080`, so the same file works for
both same-PC and remote-browser use.

## RisuAI setup

1. Register `Archive Center.js` from this folder as the RisuAI plugin.
2. Use this backend URL on the same PC:

```text
http://127.0.0.1:28080
```

If RisuAI is opened from another PC or phone, use the server PC's reachable
LAN/VPN/Tailscale/proxy URL instead of `localhost`.

## Optional smoke test

After the server is running, double-click:

```text
02_smoke_test_windows.bat
```

## Defender or SmartScreen

Archive Center does not disable Microsoft Defender and does not add Defender
exclusions automatically. If a known-good build is blocked, run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\check-windows-trust.ps1
```

Submit the exact detected file, detection name, and SHA256 from the generated
trust report to Microsoft Security Intelligence.

## Optional external ChromaDB

To use an external vector service, edit `.env.full.local`:

```text
AC_RUNTIME_PROFILE=vector_external
AC_VECTOR_MODE=external
AC_CHROMA_ENDPOINT=http://SERVER_IP_OR_DOMAIN:8000
```

Keep Lite as `core_lite` / `fallback` when low memory use is the priority.
