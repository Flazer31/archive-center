#!/usr/bin/env sh
set -eu

RELEASE_HELPER_URL="https://raw.githubusercontent.com/Flazer31/archive-center/main/scripts/install-github-release.sh"
EXTERNAL_OPERATION_TIMEOUT_SECONDS="${AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS:-1800}"
REQUEST_TIMEOUT_SECONDS="${AC_REQUEST_TIMEOUT_SECONDS:-30}"
READINESS_TIMEOUT_SECONDS="${AC_READINESS_TIMEOUT_SECONDS:-180}"
READINESS_POLL_INTERVAL_SECONDS="${AC_READINESS_POLL_INTERVAL_SECONDS:-1}"
SERVICE_RESTART_SECONDS="${AC_SERVICE_RESTART_SECONDS:-5}"
BOOTSTRAP_DIR=""
RESERVATION_ACTIVE=false
RESERVATION_MARKER=""

die() {
	printf 'ERROR: %s\n' "$*" >&2
	exit 1
}

has_cmd() {
	command -v "$1" >/dev/null 2>&1
}

cleanup() {
	if [ "$RESERVATION_ACTIVE" = "true" ] && [ -n "$RESERVATION_MARKER" ]; then
		case "$INSTALL_DIR" in
			/opt/archive-center)
				if run_as_root test -f "$RESERVATION_MARKER"; then
					run_as_root rm -rf -- "$INSTALL_DIR"
				fi
				;;
			"${HOME:-}"/.archive-center)
				if [ -f "$RESERVATION_MARKER" ]; then
					rm -rf -- "$INSTALL_DIR"
				fi
				;;
		esac
	fi
	if [ -n "$BOOTSTRAP_DIR" ]; then
		case "$BOOTSTRAP_DIR" in
			"${TMPDIR:-/tmp}"/archive-center-fresh-install.*)
				rm -rf -- "$BOOTSTRAP_DIR"
				;;
		esac
	fi
}

commit_reservation() {
	RESERVATION_ACTIVE=false
	if [ "$INSTALL_DIR" = "/opt/archive-center" ]; then
		run_as_root rm -f -- "$RESERVATION_MARKER" || true
	else
		rm -f -- "$RESERVATION_MARKER" || true
	fi
}

run_as_root() {
	if [ "$(id -u)" = "0" ]; then
		"$@"
	elif has_cmd sudo; then
		sudo "$@"
	else
		die "administrator access is required to prepare dependencies or install the systemd service"
	fi
}

dependencies_ready() {
	has_cmd curl && has_cmd python3 && has_cmd unzip && { has_cmd sha256sum || has_cmd shasum; }
}

prepare_linux_dependencies() {
	if has_cmd apt-get; then
		run_as_root apt-get update
		run_as_root apt-get install -y curl python3 unzip coreutils
	elif has_cmd dnf; then
		run_as_root dnf install -y curl python3 unzip coreutils
	elif has_cmd yum; then
		run_as_root yum install -y curl python3 unzip coreutils
	elif has_cmd zypper; then
		run_as_root zypper --non-interactive install curl python3 unzip coreutils
	elif has_cmd apk; then
		run_as_root apk add --no-cache curl python3 unzip coreutils
	elif has_cmd pacman; then
		run_as_root pacman -S --needed --noconfirm curl python unzip coreutils
	else
		die "cannot prepare curl, python3, unzip, and checksum tools: no supported Linux package manager was found"
	fi
}

ensure_homebrew() {
	if ! has_cmd brew; then
		brew_installer="$BOOTSTRAP_DIR/homebrew-install.sh"
		curl --connect-timeout "$EXTERNAL_OPERATION_TIMEOUT_SECONDS" \
			--max-time "$EXTERNAL_OPERATION_TIMEOUT_SECONDS" \
			-fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh \
			-o "$brew_installer"
		NONINTERACTIVE=1 /bin/bash "$brew_installer"
		PATH="/opt/homebrew/bin:/usr/local/bin:$PATH"
		export PATH
	fi
	has_cmd brew || die "Homebrew installation completed but brew was not found"
}

prepare_macos_dependencies() {
	if ! has_cmd python3 || ! has_cmd unzip || { ! has_cmd sha256sum && ! has_cmd shasum; }; then
		ensure_homebrew
	fi
	if ! has_cmd python3; then
		brew install python
	fi
	if ! has_cmd unzip; then
		brew install unzip
	fi
	if ! has_cmd sha256sum && ! has_cmd shasum; then
		brew install coreutils
		coreutils_prefix=$(brew --prefix coreutils)
		PATH="$coreutils_prefix/libexec/gnubin:$PATH"
		export PATH
	fi
}

case "$EXTERNAL_OPERATION_TIMEOUT_SECONDS" in
	''|*[!0-9]*|0) die "AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS must be a positive integer" ;;
esac
case "$SERVICE_RESTART_SECONDS" in
	''|*[!0-9]*|0) die "AC_SERVICE_RESTART_SECONDS must be a positive integer" ;;
esac

AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS=$EXTERNAL_OPERATION_TIMEOUT_SECONDS
AC_REQUEST_TIMEOUT_SECONDS=$REQUEST_TIMEOUT_SECONDS
AC_READINESS_TIMEOUT_SECONDS=$READINESS_TIMEOUT_SECONDS
AC_READINESS_POLL_INTERVAL_SECONDS=$READINESS_POLL_INTERVAL_SECONDS
export AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS AC_REQUEST_TIMEOUT_SECONDS
export AC_READINESS_TIMEOUT_SECONDS AC_READINESS_POLL_INTERVAL_SECONDS

os=$(uname -s 2>/dev/null || printf unknown)
machine=$(uname -m 2>/dev/null || printf unknown)
termux=false
if printf '%s' "${PREFIX:-}" | grep -qi 'com.termux'; then
	termux=true
fi

if [ "$termux" = "true" ]; then
	case "$machine" in
		aarch64|arm64) ;;
		*) die "Termux fresh install supports arm64 only (detected $machine)" ;;
	esac
elif [ "$os" = "Linux" ]; then
	case "$machine" in
		x86_64|amd64|aarch64|arm64) ;;
		*) die "unsupported Linux CPU: $machine" ;;
	esac
elif [ "$os" = "Darwin" ]; then
	case "$machine" in
		x86_64|amd64|aarch64|arm64) ;;
		*) die "unsupported macOS CPU: $machine" ;;
	esac
else
	die "unsupported operating system: $os"
fi

systemd=false
if [ "$termux" = "false" ] && [ "$os" = "Linux" ] && has_cmd systemctl && [ -d /run/systemd/system ]; then
	systemd=true
fi

if [ "$systemd" = "true" ]; then
	INSTALL_DIR="/opt/archive-center"
	RUN_USER="${SUDO_USER:-$(id -un)}"
	if [ -n "${HOME:-}" ]; then
		if [ -e "$HOME/.archive-center" ] || [ -L "$HOME/.archive-center" ]; then
			die "fresh install only: $HOME/.archive-center already exists. Updates use a separate path; nothing was changed."
		fi
	fi
else
	[ -n "${HOME:-}" ] || die "HOME is required for this fresh install"
	INSTALL_DIR="$HOME/.archive-center"
fi

if [ -e "$INSTALL_DIR" ] || [ -L "$INSTALL_DIR" ]; then
	die "fresh install only: $INSTALL_DIR already exists. Updates use a separate path; nothing was changed."
fi

BOOTSTRAP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/archive-center-fresh-install.XXXXXX")
trap cleanup EXIT INT TERM

if ! dependencies_ready; then
	if [ "$termux" = "true" ]; then
		has_cmd pkg || die "Termux pkg is required to prepare dependencies"
		pkg install -y curl python unzip coreutils
	elif [ "$os" = "Linux" ]; then
		prepare_linux_dependencies
	else
		prepare_macos_dependencies
	fi
fi
dependencies_ready || die "curl, python3, unzip, and sha256sum or shasum are required after dependency preparation"

helper_path="$BOOTSTRAP_DIR/install-github-release.sh"
curl --connect-timeout "$EXTERNAL_OPERATION_TIMEOUT_SECONDS" \
	--max-time "$EXTERNAL_OPERATION_TIMEOUT_SECONDS" \
	-fsSL "$RELEASE_HELPER_URL" -o "$helper_path"

# Reserve the previously absent root atomically so this fresh-only entrypoint
# cannot hand an install that appeared concurrently to the update-capable helper.
if [ "$systemd" = "true" ]; then
	if ! run_as_root mkdir "$INSTALL_DIR"; then
		die "fresh install only: $INSTALL_DIR appeared during setup. Updates use a separate path; nothing was overwritten."
	fi
elif ! mkdir "$INSTALL_DIR"; then
	die "fresh install only: $INSTALL_DIR appeared during setup. Updates use a separate path; nothing was overwritten."
fi
RESERVATION_MARKER="$INSTALL_DIR/.archive-center-fresh-install-reservation"
if [ "$systemd" = "true" ]; then
	run_as_root touch "$RESERVATION_MARKER"
else
	touch "$RESERVATION_MARKER"
fi
RESERVATION_ACTIVE=true

if [ "$systemd" = "true" ]; then
	run_as_root env \
		AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS="$EXTERNAL_OPERATION_TIMEOUT_SECONDS" \
		AC_REQUEST_TIMEOUT_SECONDS="$REQUEST_TIMEOUT_SECONDS" \
		AC_READINESS_TIMEOUT_SECONDS="$READINESS_TIMEOUT_SECONDS" \
		AC_READINESS_POLL_INTERVAL_SECONDS="$READINESS_POLL_INTERVAL_SECONDS" \
		AC_SERVICE_RESTART_SECONDS="$SERVICE_RESTART_SECONDS" \
		sh "$helper_path" \
		--install-dir "$INSTALL_DIR" \
		--systemd \
		--user "$RUN_USER"
	commit_reservation
else
	AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS="$EXTERNAL_OPERATION_TIMEOUT_SECONDS" \
		sh "$helper_path" --install-dir "$INSTALL_DIR"
	launcher="$INSTALL_DIR/start-archive-center.sh"
	[ -f "$launcher" ] || die "installed stable launcher was not found: $launcher"
	commit_reservation
	cleanup
	trap - EXIT INT TERM
	exec sh "$launcher"
fi
