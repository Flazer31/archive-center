# Deprecated Archive Center POSIX Lite Note

Deprecated: separate Lite packages are retired for Archive Center 2.3. Use the
standard package and choose a lighter runtime profile if needed.

This file is retained as a historical Lite/default package note for Linux,
macOS, and Android Termux.

Default runtime:

```text
AC_RUNTIME_PROFILE=core_lite
AC_VECTOR_MODE=fallback
```

Lite starts the Go backend and MariaDB path, but does not install or start a
local ChromaDB service by default. ChromaDB can be added later as an external
service or as an explicit local/full profile, but it is not part of the normal
low-resource path.

## Linux

```sh
sh start-archive-center-linux.sh
```

## macOS

```sh
sh "Start Archive Center macOS.command"
```

## Android / Termux

```sh
sh install-and-start-termux.sh
```

Termux Lite uses Termux `pkg` packages and the app-private runtime directory.
It should not install Ubuntu/proot ChromaDB on the default path. Local/proot
ChromaDB is a heavy opt-in path for `full_local` / `local_proot` testing only.

Runtime data defaults to:

```text
$HOME/.archive-center-2.0
```

This avoids Android shared-storage limitations in paths such as
`/storage/emulated/0/Download`, where MariaDB file locks, sockets, permissions,
and direct binary execution can fail. The launcher copies packaged Go binaries
into the Termux-private runtime directory before running them.

## Optional External ChromaDB

To use ChromaDB from a PC/NAS instead of the phone:

```sh
AC_RUNTIME_PROFILE=vector_external \
AC_VECTOR_MODE=external \
AC_CHROMA_ENDPOINT=http://SERVER_IP_OR_DOMAIN:8000 \
sh install-and-start-termux.sh
```

## Optional Full Local Testing

Full local vector mode is intentionally not the normal Termux path:

```sh
AC_RUNTIME_PROFILE=full_local \
AC_VECTOR_MODE=local_proot \
sh install-and-start-termux.sh
```

Use this only for high-memory testing. For normal mobile use, keep Lite.

## Notes

- MariaDB remains the canonical store.
- ChromaDB is optional in Lite.
- Real target-device proof is still required before promotion to release.
