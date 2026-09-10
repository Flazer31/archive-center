# Archive Center 2.1 POSIX Package Candidate

This package is a managed package candidate for Linux and macOS, and an
automatic install package for Termux.

Default runtime:

```text
AC_RUNTIME_PROFILE=full_local
Linux/macOS AC_VECTOR_MODE=local_native
Termux AC_VECTOR_MODE=local_proot
```

The package includes the Archive Center Go backend, schema tool, plugin JS,
migrations, prompts, and managed bootstrap scripts. The bootstrap script uses
the platform package manager when POSIX MariaDB or ChromaDB runtimes are not
already bundled.

## Linux

```sh
chmod +x scripts/start-full-linux.sh scripts/start-full-posix.sh
sh scripts/start-full-linux.sh
```

## macOS

```sh
chmod +x scripts/start-full-macos.sh scripts/start-full-posix.sh
sh scripts/start-full-macos.sh
```

If Homebrew is not present, the macOS launcher bootstraps Homebrew
automatically, then continues with MariaDB/Python/ChromaDB preparation. macOS
may ask for a password or command line tools during that bootstrap, but normal
users do not need to separately download MariaDB or ChromaDB installers.

## Change service ports

Append `--configure-ports` to the usual launcher and choose ChromaDB, MariaDB,
or Go backend. Enter a port, or leave the port input empty to restore that
service's default: **8000 / 3307 / 28080**, respectively. The menu saves and
exits; restart Archive Center normally to apply the setting.

For a standard Termux one-line installation:

```sh
sh ~/.archive-center/start.sh --configure-ports
```

For an extracted Termux package:
`sh install-and-start-termux.sh --configure-ports`.
Linux/macOS users append the same option to their existing start command.
`--configure-chroma-port` still opens the ChromaDB port prompt directly.
`--chroma-port 8001`, `--mariadb-port 3308`, and `--backend-port 28081` also
save the requested port and start normally.

Settings live in the existing data root as `chroma-port.txt`, `mariadb-port.txt`
and `backend-port.txt`. Database paths stay the same. The launcher applies DB
ports to both server startup and Go connection settings. External ChromaDB
keeps its existing endpoint. After changing the Go backend port, also update
the port in the backend URL saved in RisuAI.


## Termux

```sh
sh scripts/install-and-start-termux.sh
```

Termux installs base packages through `pkg`. ChromaDB is not installed into
native Termux Python because `onnxruntime`, `orjson`, and `tokenizers` can fail
on Android/Termux. The launcher instead prepares a managed Ubuntu runtime with
`proot-distro` and runs ChromaDB inside that runtime. This keeps the package
usable without asking normal users to manually compile Python native packages.

First startup can take a long time because it may install Termux packages,
download the Ubuntu proot image, create a Python venv, install ChromaDB, and
initialize MariaDB.

On Termux, runtime data defaults to:

```text
$HOME/.archive-center-2.0
```

This avoids Android shared-storage limitations in paths such as
`/storage/emulated/0/Download`, where MariaDB file locks, sockets, and
permissions may fail. Android shared storage can also reject direct execution of
packaged binaries, so the launcher copies `bin/archive-center-go` and
`bin/mariadb-schema` into `$HOME/.archive-center-2.0/bin` before running them.
Override with `ARCHIVE_CENTER_DATA_DIR` only if the target path is inside Termux
app-private storage.

If startup reports that `migrations/001_schema.sql` is missing, the package was
probably extracted partially or launched from the wrong folder. Re-extract the
full ZIP and make sure `bin`, `migrations`, `prompts`, and `scripts` are all in
the same package folder.

## Optional 1.0 DB migration

After the 2.1 server/MariaDB stack is running, migrate an old Archive Center 1.0
SQLite `memory.db` with:

```sh
chmod +x scripts/migrate-legacy-1.0.sh
sh scripts/migrate-legacy-1.0.sh /path/to/memory.db
```

That first command is dry-run only. Review the report path printed by the tool.
To import into MariaDB:

```sh
sh scripts/migrate-legacy-1.0.sh /path/to/memory.db --execute
```

After importing, confirm sessions/timeline in the UI and run vector reindex if
the imported memories should be searchable through ChromaDB.

## Runtime Notes

- MariaDB remains the canonical store.
- ChromaDB is the only vector engine.
- Normal users should not need to manually configure MariaDB or ChromaDB.
- Real device proof is still required before these packages are promoted to RC.
