# Migrations

Status: active source-controlled MariaDB schema and compatibility tooling.

`mariadb-schema` treats this directory as one ordered migration inventory. When
it receives either the directory path or `001_schema.sql`, it loads every
sibling `.sql` file in sorted order. Fresh and existing installations therefore
use the same complete set; `001_schema.sql` does not have to repeat tables or
columns that are owned by later files.

`013_precise_memory_text_fields.sql` addresses GitHub #6 by widening the three
unindexed Critic-derived text fields in `precise_memory_units` to LONGTEXT.
Fresh schema and the Go compatibility pass use the same types. Existing text and
stored Critic results are retained; applying the migration again is safe. See
the [4.3 feedback verification](../docs/archive-center-4.3-feedback-work-log.md).

`002_canon_pack_storage.sql` is the reviewed Archive Center 3.1 additive
Canon Pack storage migration. Its statements are also registered in the
canonical fresh-install schema and the production `mariadb-schema`
compatibility pass. The registration is for a full 3.1 package install or
upgrade only.

Archive Center 3.6-E adds automatic update support for managed program files.
The updater does not reject a package because a migration file was added,
changed, or removed, and it does not impose filename, fingerprint, SQL-shape,
or migration-policy metadata gates. The launcher applies the complete installed
migration inventory before backend health commit. Managed program files are
recovered if schema execution or health verification fails; existing database
data, Chroma data, runtime directories, local environment files, and secrets
are not part of managed-file replacement. Database changes already applied by
the schema tool are not rolled back by the file rollback.

The registration decision is backed by a local disposable MariaDB 11.4.12
run: `002` was applied twice through the production loader, all ten tables and
constraints were exercised, and the seeded pre-migration reference rows kept
the same fingerprint. The GitHub Actions equivalent remains as the repeatable
remote gate.

The tenth table, `source_discovery_jobs`, is an isolated pending-candidate ledger. It has no
foreign key to session, chat, memory, or admitted reference rows. Search-provider credentials,
LLM client metadata, fetched full text, cookies, and browser sessions must not be stored there.

No runtime database, backup, restore dump, vector persist directory, or generated migration artifact should be stored here unless it is a small source-controlled schema/tooling file.
