# Migrations

Status: active source-controlled MariaDB schema and compatibility tooling.

`001_schema.sql` is the canonical fresh-install schema used by the
`mariadb-schema` command. Existing installations receive additive compatibility
statements from the same command. Archive Center 3.0 reference-library tables
are additive and do not rewrite existing session rows.

`002_canon_pack_storage.sql` is the reviewed Archive Center 3.1 additive
Canon Pack storage migration. Its statements are also registered in the
canonical fresh-install schema and the production `mariadb-schema`
compatibility pass. The registration is for a full 3.1 package install or
upgrade only. Updater v1 continues to reject migration and `mariadb-schema`
changes and must not apply this migration.

The registration decision is backed by a local disposable MariaDB 11.4.12
run: `002` was applied twice through the production loader, all ten tables and
constraints were exercised, and the seeded pre-migration reference rows kept
the same fingerprint. The GitHub Actions equivalent remains as the repeatable
remote gate.

The tenth table, `source_discovery_jobs`, is an isolated pending-candidate ledger. It has no
foreign key to session, chat, memory, or admitted reference rows. Search-provider credentials,
LLM client metadata, fetched full text, cookies, and browser sessions must not be stored there.

No runtime database, backup, restore dump, vector persist directory, or generated migration artifact should be stored here unless it is a small source-controlled schema/tooling file.
