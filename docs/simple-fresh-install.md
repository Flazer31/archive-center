# Archive Center Simple Fresh Install Contract

This is the permanent normal-user fresh-install interface. These entry points
install the latest Archive Center GitHub Release and start it immediately. They
are for a clean machine only and do not own any update behavior.

## Linux, macOS, and Termux

```sh
curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | sh
```

The installer selects the supported package for the current OS and CPU and
prepares the release helper's required command-line tools. Linux hosts running
systemd install to `/opt/archive-center`, register the service for the current
login user, and start it. Other Linux hosts, macOS, and Termux install to
`$HOME/.archive-center` and start the packaged launcher. Termux supports arm64
only.

The POSIX install root owns one stable `data` directory and one stable launcher.
Linux systemd, macOS, Termux, and later manual restarts all enter the current
package through that launcher, so later package updates keep using the same
MariaDB and ChromaDB data.

## Windows

Run this in PowerShell:

```powershell
irm https://raw.githubusercontent.com/Flazer31/archive-center/main/install-windows.ps1 | iex
```

The Windows installer installs to `%LOCALAPPDATA%\ArchiveCenter` and starts the
packaged launcher.

If the selected install directory already exists, the fresh installer stops
before downloading a Release helper or changing anything. Updates use a
separate path.

Normal users are never asked to choose a timeout, install directory, CPU asset,
service mode, restart delay, or start flag. The two commands above are the only
normal-user fresh-install commands. Internal Release helpers are not additional
public install methods.

The repository CI executes the production entrypoints with controlled helper
boundaries. It verifies both the clean-install invocation and the
existing-install no-network/no-mutation result so future documentation or
installer changes cannot silently restore the old multi-step interface.
