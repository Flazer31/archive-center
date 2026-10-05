# Archive Center __ARCHIVE_CENTER_PACKAGE_VERSION__ POSIX - Read First

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
path. This is the standard package line across desktop,
server, and Termux.
On Termux, local ChromaDB uses a managed proot runtime, so server-side or
external-vector deployment is still preferable when the phone feels slow.

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

There is no separate Lite ZIP. To choose the lighter profile, append
`--profile core_lite --vector-mode fallback` to the root launcher, for example:

```sh
sh start-archive-center-linux.sh --profile core_lite --vector-mode fallback
```

The root launchers explicitly select `full_local` and their platform vector mode;
setting only environment variables does not override those launcher arguments.

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
