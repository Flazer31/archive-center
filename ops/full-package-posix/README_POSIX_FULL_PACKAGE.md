# Archive Center __ARCHIVE_CENTER_PACKAGE_VERSION__ POSIX Auto Install Package

## 오류 보고

플러그인 **설정 → 오류·진단 보고서 → 보고서 저장**으로 최근 오류를 저장합니다.
서버 연결이 안 되면 패키지 폴더에서 **`sh 07_export_diagnostics.sh`**를 실행하세요.
기존 Python으로 로그를 읽어 홈 폴더에 JSON 보고서를 저장하며 백엔드와 DB는 실행하지 않습니다.
로그 경로는 시작 화면에 표시됩니다. API 키 등은 보고서에서 마스킹하며 자동 전송하지 않습니다.
한 줄 설치 후 새 터미널에서 수집할 때는 설치된 데이터 경로를 함께 전달하세요:

```sh
ARCHIVE_CENTER_DATA_DIR="$(cat "$HOME/.archive-center/data-root.txt")" \
  sh "$HOME/.archive-center/current/07_export_diagnostics.sh"
```

설치 위치를 바꿨다면 두 경로의 `$HOME/.archive-center`를 해당 설치 폴더로 바꾸세요.
`AC_LOG_DIR`를 별도로 설정했다면 시작 화면의 실제 로그 경로를 사용합니다.


This is the managed automatic install package for Linux, macOS, and Termux.

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

Run the following commands from the extracted package root. For an existing
one-line installation, use its stable launcher described under Data location.

## Linux

```sh
sh start-archive-center-linux.sh
```

## macOS

```sh
sh "Start Archive Center macOS.command"
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
sh "$HOME/.archive-center/start-archive-center.sh" --configure-ports
```

For an extracted Termux package:
`sh install-and-start-termux.sh --configure-ports`.
One-line installations on Linux/macOS also use `start-archive-center.sh`
in the installation directory. On Linux with an active systemd environment,
the one-line installer selects `/opt/archive-center`; otherwise its default
is `$HOME/.archive-center`. Replace the directory in the command accordingly.
For extracted Linux/macOS packages, append the option to the root launcher
shown below.
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
sh install-and-start-termux.sh
```

Termux installs base packages through `pkg`. ChromaDB is not installed into
native Termux Python because `onnxruntime`, `orjson`, and `tokenizers` can fail
on Android/Termux. The launcher instead prepares a managed Ubuntu runtime with
`proot-distro` and runs ChromaDB inside that runtime. This keeps the package
usable without asking normal users to manually compile Python native packages.

First startup can take a long time because it may install Termux packages,
download the Ubuntu proot image, create a Python venv, install ChromaDB, and
initialize MariaDB.

## Data location and subsequent starts

The one-line installer and a directly extracted ZIP choose different defaults:

| Installation method | Default data root |
| --- | --- |
| One-line installation, all POSIX platforms | `<install directory>/data`, normally `$HOME/.archive-center/data` |
| Linux one-line installation with systemd detected | `/opt/archive-center/data`, unless the install/data directory is overridden |
| Directly extracted Termux ZIP | `$HOME/.archive-center-2.0` |
| Directly extracted Linux/macOS ZIP | `<package directory>/.runtime` |

For a per-user one-line installation, start and configure ports through
`sh "$HOME/.archive-center/start-archive-center.sh"`. Use `/opt/archive-center`
for the Linux systemd installation, or the custom installation directory. That stable launcher reads the
installation's `data-root.txt` and exports `ARCHIVE_CENTER_DATA_DIR` before
calling the current package launcher. At installation time, an existing
`ARCHIVE_CENTER_DATA_DIR` overrides the default saved in that pointer. Running
a package launcher directly in a new shell does not read the installation's
pointer; use the stable launcher to continue with the installed data root.

For direct ZIP execution, `ARCHIVE_CENTER_DATA_DIR` overrides the defaults in
the table. On Termux, keep the data root inside app-private storage. Android
shared storage such as `/storage/emulated/0/Download` can reject MariaDB file
locks, sockets, permissions, and binary execution. The Termux launcher copies
`bin/archive-center-go` and `bin/mariadb-schema` into `<data root>/bin` before
running them, using the selected data root rather than a fixed directory.

If startup reports that `migrations/001_schema.sql` is missing, the package was
probably extracted partially or launched from the wrong folder. Re-extract the
full ZIP and make sure `bin`, `migrations`, `prompts`, and `scripts` are all in
the same package folder.

## Optional 1.0 DB migration

This release's normal packages do not include `bin/legacy10-migrate`. The
retained `scripts/migrate-legacy-1.0.sh` wrapper cannot migrate an old SQLite
`memory.db` without that separate executable. No Legacy Migration Tools asset
is included in this release. Preserve the old database and arrange a separate
migration procedure before attempting to import it.

## Runtime Notes

- MariaDB remains the canonical store.
- ChromaDB is the only vector engine.
- Normal users should not need to manually configure MariaDB or ChromaDB.
- Linux, macOS, and Termux packages are cross-built; this does not establish
  native-device installation, update, or recovery verification.
