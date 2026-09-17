# Archive Center 2.1 POSIX Lite - Read First

This package is the Lite/default package for Linux, macOS, and Android Termux.

The default profile is:

```text
AC_RUNTIME_PROFILE=core_lite
AC_VECTOR_MODE=fallback
```

That means the package should not install or start local ChromaDB on the normal
path. Basic save, chat log storage, and canonical memory persistence use the Go
backend plus MariaDB. Vector recall can be added later through external
ChromaDB or an explicit full/local profile.

## Start

Linux:

```sh
sh start-archive-center-linux.sh
```

macOS:

```sh
sh "Start Archive Center macOS.command"
```

Android / Termux:

```sh
sh install-and-start-termux.sh
```

## Termux

Termux Lite stores runtime data under:

```text
$HOME/.archive-center-2.0
```

Do not run the database directly from Android shared storage such as
`/storage/emulated/0/Download`. Shared storage can break file locks, sockets,
permissions, and direct binary execution.

If you need external vector recall, use a PC/NAS ChromaDB service:

```sh
AC_RUNTIME_PROFILE=vector_external \
AC_VECTOR_MODE=external \
AC_CHROMA_ENDPOINT=http://SERVER_IP_OR_DOMAIN:8000 \
sh install-and-start-termux.sh
```

Local/proot ChromaDB is heavy and opt-in only:

```sh
AC_RUNTIME_PROFILE=full_local \
AC_VECTOR_MODE=local_proot \
sh install-and-start-termux.sh
```

## Bridge URL

Same device:

```text
http://127.0.0.1:28080
```

Different device:

```text
http://SERVER_IP_OR_DOMAIN:28080
```

Do not use `localhost` from another PC or phone. It points to the browser's own
device, not the machine running Archive Center.
