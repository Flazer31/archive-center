# Archive Center 2.1 POSIX - Read First

This package is the standard local package for Linux, macOS, and Android Termux.

Default runtime:

```text
AC_RUNTIME_PROFILE=full_local
```

Default vector mode:

```text
Linux/macOS: AC_VECTOR_MODE=local_native
Termux:      AC_VECTOR_MODE=local_proot
```

The standard package starts the Go backend, MariaDB path, and a local ChromaDB
path. For 2.3, this is the single supported package line across desktop,
server, and Termux.
On Termux, local ChromaDB uses a managed proot runtime, so server-side or
external-vector deployment is still preferable when the phone feels slow.

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

## Low-Memory Runtime Options

Separate Lite ZIPs are retired for 2.3. Low-memory deployments should keep the
standard package but choose a lighter runtime profile when needed:

```text
AC_RUNTIME_PROFILE=core_lite
AC_VECTOR_MODE=fallback
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
