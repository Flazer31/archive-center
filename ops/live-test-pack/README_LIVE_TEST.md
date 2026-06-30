# Archive Center 2.0 Live Test Pack

This folder is a lightweight release-candidate experiment pack. It is meant for
local testing before real distribution packaging.

It intentionally does not include:

- MariaDB runtime binaries
- ChromaDB runtime files
- database files
- vector data
- Go build cache
- generated `.exe` smoke binaries
- backup, release, or deploy output folders

## Required Services

You need these running locally or reachable from this machine:

1. MariaDB, with a database/user represented by `AC_MARIADB_DSN`.
2. ChromaDB HTTP server, represented by `AC_CHROMA_ENDPOINT`.
3. Optional but recommended: LLM critic endpoint and embedding endpoint.

Milvus Lite is not part of this live-test path.

## Quick Start

From this folder:

```powershell
Copy-Item .env.live.example .env.live.local
notepad .env.live.local
```

Fill at least:

- `AC_MARIADB_DSN`
- `AC_CHROMA_ENDPOINT`

If ChromaDB is not already running and Python has ChromaDB installed:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start-chromadb.ps1
```

If Python does not have ChromaDB yet and you accept installing it into the
current Python environment or venv:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start-chromadb.ps1 -InstallChroma
```

Start the Go backend:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start-backend.ps1 -RunSchema
```

In another terminal, run the live route smoke:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-live.ps1
```

Then load `Archive Center.js` from this folder in RisuAI and point the plugin
bridge URL to the backend bind address, for example:

```text
http://127.0.0.1:28080
```

## What The Smoke Proves

The smoke script exercises:

- `/health`
- `/ready`
- `/config/update`
- `/complete-turn`
- `/prepare-turn`
- `/search`
- `/rollback/{turn_index}`
- `/sessions/{chat_session_id}`

The most important result is not just HTTP 200. Check whether:

- `ready` reports MariaDB authority and ChromaDB vector readiness.
- `/complete-turn` writes to MariaDB.
- critic and embedding warnings are absent when providers are configured.
- `/prepare-turn` reports `engine=chromadb` and product-read source when
  vector query evidence is available.
- rollback deletes MariaDB rows and vector documents.
- session delete cleans up MariaDB and ChromaDB session vectors.

## Safety Notes

- Use a disposable database while testing.
- Do not point this at your only real story database first.
- Keep `.env.live.local` private.
- Delete `.runtime` inside this pack whenever you want a clean local ChromaDB
  data directory.
