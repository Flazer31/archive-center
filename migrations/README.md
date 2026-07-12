# Migrations

Status: active source-controlled MariaDB schema and compatibility tooling.

`001_schema.sql` is the canonical fresh-install schema used by the
`mariadb-schema` command. Existing installations receive additive compatibility
statements from the same command. Archive Center 3.0 reference-library tables
are additive and do not rewrite existing session rows.

No runtime database, backup, restore dump, vector persist directory, or generated migration artifact should be stored here unless it is a small source-controlled schema/tooling file.
