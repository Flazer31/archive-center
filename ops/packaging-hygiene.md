# Packaging Hygiene for Archive Center 2.0

> Status: **R0/R1 design contract**  
> Live packaging / release pipeline is banned until explicit approval.

---

## 1. Purpose

This document defines the artifact exclusion rules that prevent runtime, private, generated, and transient files from entering a release candidate manifest.

It is the Archive Center 2.0 counterpart to the 0.8(fix) H-4e release-hygiene gate.

---

## 2. Exclusion Rules

### 2.1 Secrets & Environment
| Pattern | Kind | Rationale |
|---------|------|-----------|
| `.env` | file | Live secrets/config. |
| `.env.*` | file | Environment overrides (`.env.local`, `.env.production`). |
| `*.env` | file | Generic env file pattern. |

### 2.2 Databases & SQLite
| Pattern | Kind | Rationale |
|---------|------|-----------|
| `*.db` | file | SQLite runtime database. |
| `*.sqlite` | file | SQLite database. |
| `*.sqlite3` | file | SQLite database (Chroma internal). |
| `*.db-wal` | file | SQLite WAL journal. |
| `*.db-shm` | file | SQLite WAL shared-memory file. |

### 2.3 Vector Store Persist Dirs
| Pattern | Kind | Rationale |
|---------|------|-----------|
| `.chroma_shadow/**` | directory | Chroma vector store runtime data. |
| `chroma_data/**` | directory | Chroma persist directory. |
| `milvus_data/**` | directory | Historical/future-only Milvus runtime data; must not ship unless a later explicit decision reopens this lane. |
| `milvus.db` | file | Historical/future-only Milvus Lite local database; must not ship. |

### 2.4 Generated Caches
| Pattern | Kind | Rationale |
|---------|------|-----------|
| `__pycache__/**` | directory | Python bytecode cache. |
| `.pytest_cache/**` | directory | Pytest cache. |
| `*.pyc` | file | Compiled Python bytecode. |
| `.gocache/**` | directory | Go build cache. |
| `.runtime/**` | directory | Local managed runtime/cache artifacts. |
| `.runtime-cache/**` | directory | Local managed runtime/cache artifacts. |
| `cache/**` | directory | Generic cache directory. |
| `.cache/**` | directory | Generic hidden cache directory. |
| `caches/**` | directory | Generic caches directory. |
| `*.cache` | file | Generic cache file. |

### 2.5 Local Virtual Environments
| Pattern | Kind | Rationale |
|---------|------|-----------|
| `.venv/**` | directory | Python virtual environment. |
| `backend/.venv/**` | directory | Backend-specific virtual environment. |

### 2.6 Logs
| Pattern | Kind | Rationale |
|---------|------|-----------|
| `*.log` | file | Log files. |
| `logs/**` | directory | Log directory. |
| `log/**` | directory | Log directory. |

### 2.7 Backup & Temp Artifacts
| Pattern | Kind | Rationale |
|---------|------|-----------|
| `*.bak` | file | Backup file. |
| `*.backup*` | file | Backup file. |
| `*.backup_*` | file | Backup file. |
| `_tmp_*.py`, `tmp_*.py` | file | Temporary Python file. |
| `repair*.py`, `repair*.txt` | file | Local repair scratch file. |
| `patch_*.py` | file | Local patch helper file. |
| `debug*.py`, `debug*.txt` | file | Local debug scratch file. |
| `check_*.py`, `find_*.py` | file | Local inspection scratch file. |
| `tmp/**` | directory | Temporary directory. |
| `temp/**` | directory | Temporary directory. |

### 2.8 SCM & OS Artifacts
| Pattern | Kind | Rationale |
|---------|------|-----------|
| `.git/**` | directory | Git repository metadata. |
| `.DS_Store` | file | macOS metadata. |
| `Thumbs.db` | file | Windows thumbnail cache. |

---

## 3. Scan Tool

A minimal Go scan tool lives at:

```
go-service/cmd/artifact-scan/main.go
```

Usage:
```bash
cd go-service
go run ./cmd/artifact-scan/main.go -root ".."
```

For release gates, prefer an absolute root path to avoid shell/cwd ambiguity:

```powershell
cd "Archive Center 2.0\go-service"
go run -buildvcs=false ./cmd/artifact-scan/main.go -root "M:\risulongmemory\Archive Center 2.0"
```

The tool:
- Walks the workspace root recursively.
- Skips matched directories immediately (no heavy recursion into `.gocache`, `.git`, etc.).
- Prints a `FAIL` line and exits non-zero if any risky artifact is found.
- Prints `PASS` and exits zero if the workspace is clean.

Current status against Archive Center 2.0:
```
$ go run ./cmd/artifact-scan/main.go -root ".."
[runtime_dir] .runtime
[windows_binary] go-service\js-route-variant-smoke.exe
[scratch_patch_file] go-service\patch_all.py
FAIL: 43 risky artifact(s) found
```

> Runtime caches, local test binaries, and scratch helper files are expected during development, but they are release blockers until excluded from the package manifest.

---

## 4. .gitignore Parity

The workspace `.gitignore` (`Archive Center 2.0\.gitignore`) already covers every rule above. The scan tool and `.gitignore` are kept in sync intentionally.

| Rule | .gitignore | Scan Tool |
|------|------------|-----------|
| `.env` | ✅ | ✅ |
| `*.db` / `*.sqlite` / `*.sqlite3` | ✅ | ✅ |
| `.chroma_shadow/` | ✅ | ✅ |
| `__pycache__/` / `*.pyc` | ✅ | ✅ |
| `.gocache/` | ✅ | ✅ |
| `.runtime/` / `.runtime-cache/` | ✅ | ✅ |
| `*.log` / `tmp/` / `temp/` | ✅ | ✅ |
| scratch helpers (`repair*.py`, `patch_*.py`, `debug*.py`, `tmp_*.py`) | ✅ | ✅ |
| `*.bak` / `*.backup` | ✅ | ✅ |
| `.DS_Store` / `Thumbs.db` | ✅ | ✅ |

---

## 5. Evidence Checklist

- [x] All artifact classes from the H-4e gate are covered.
- [x] ChromaDB persist directories are explicitly listed.
- [x] Historical/future-only Milvus persist artifacts are excluded from release packages.
- [x] Cache directories (`cache/`, `.cache/`, `caches/`, `*.cache`) are included.
- [x] Go build cache (`.gocache`) is tracked.
- [x] Scan tool compiles and runs against the workspace.
- [ ] Release pipeline itself (explicitly banned in R0/R1).

---

*Contract version: R0-2026-06-11*  
*Reference: `Archive Center Beta 0.8(fix)/backend/services/release_hygiene.py`, `backend/test_h4e_release_hygiene_gate.py`, `.gitignore`*
