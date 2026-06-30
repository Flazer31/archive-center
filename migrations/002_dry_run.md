# Migration Dry-Run Guide ? Archive Center 2.0 R0

> Status: **DRAFT ? dry-run only**  
> Live migration application is **explicitly banned** in R0/R1.

---

## 1. Prerequisites

- MariaDB 10.6+ (JSON type support required).
- `utf8mb4` charset enabled.
- A dedicated database/user for Archive Center 2.0 (created manually by DBA).
- No credentials or DSN strings in this repository.

---

## 2. Files

| File | Purpose | Status |
|------|---------|--------|
| `migrations/001_schema.sql` | Canonical truth tables (8) | DRAFT |
| `migrations/README.md` | General migration policy | Existing |
| `internal/store/mariadb.go` | Go disabled skeleton | Implemented |
| `internal/store/noop.go` | Go no-op for R0/R1 | Implemented |

---

## 3. Dry-Run Checklist

Before any `mysql` client execution, verify each item:

1. [ ] Target MariaDB version is >= 10.6 (`SELECT VERSION();`).
2. [ ] `information_schema` confirms `utf8mb4` is available.
3. [ ] Dedicated database exists (e.g., `archive_center_2_0`).
4. [ ] Dedicated user exists with `CREATE`, `INSERT`, `SELECT`, `INDEX` grants.
5. [ ] `.env` or credential file is **not** in source tree (H-4e gate).
6. [ ] `001_schema.sql` syntax is valid (`mariadb --dry-run` or `mysqldump --no-data` comparison).
7. [ ] No `FOREIGN KEY` constraints are defined (soft references only).
8. [ ] All `JSON` columns use MariaDB native `JSON` type (not `TEXT`).
9. [ ] All tables use `InnoDB` engine.
10. [ ] All tables use `utf8mb4` charset.

---

## 4. Manual Dry-Run Commands

These commands are for DBA review only. Do not run in CI or automation.

### 4.1 Syntax validation (no connection)
```bash
# Visual inspection only. No runtime data dir created.
mariadb --dry-run < migrations/001_schema.sql
```

### 4.2 Apply to a throwaway test database
```bash
# Requires DBA-provided credentials. Never commit credentials.
export MARIADB_DSN="user:pass@tcp(localhost:3306)/archive_center_test"
mariadb --protocol=tcp -u user -p archive_center_test < migrations/001_schema.sql
```

### 4.3 Verify applied schema
```sql
USE archive_center_test;
SHOW TABLES;
DESCRIBE chat_logs;
DESCRIBE memories;
SHOW INDEX FROM chat_logs;
SHOW INDEX FROM memories;
```

### 4.4 Rollback (test database only)
```sql
DROP TABLE IF EXISTS chat_logs;
DROP TABLE IF EXISTS effective_input_logs;
DROP TABLE IF EXISTS memories;
DROP TABLE IF EXISTS direct_evidence_records;
DROP TABLE IF EXISTS kg_triples;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS critic_feedback;
DROP TABLE IF EXISTS character_events;
```

---

## 5. Go Store Connection (Disabled)

```go
// internal/store/mariadb.go
// OpenMariaDB is disabled in R0/R1.
store, err := store.OpenMariaDB("any-dsn")
// err == store.ErrNotEnabled
```

Live connection wiring is blocked until:
- `AC_MARIADB_DSN` env var is present.
- `config.Config.Readiness.MariaDBConfigured` is `true`.
- `config.Config.Validate()` still blocks `ModeLive` and `ModeCutover`.

---

## 6. R0 Constraints

- No automated migration runner exists yet.
- No schema versioning table exists yet (future: `schema_migrations`).
- No rollback script beyond manual `DROP TABLE`.
- No seed data scripts.
- No `.env` file or credential template in source tree.

---

## 7. Verification Checklist (Before R1)

- [ ] `001_schema.sql` reviewed by DBA.
- [ ] Dry-run applied to throwaway DB and verified.
- [ ] Index performance checked with `EXPLAIN` on projected queries.
- [ ] JSON column size limits tested (MariaDB max 4 GB per JSON value).
- [ ] `DATETIME(3)` precision verified.
- [ ] Go `internal/store` integration tested against real MariaDB (R1+).
- [ ] H-4e release hygiene scan passes.

---

*Guide version: R0-2026-05-21*
