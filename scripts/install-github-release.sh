#!/usr/bin/env sh
set -eu

REPO="Flazer31/archive-center"
INSTALL_DIR="${ARCHIVE_CENTER_INSTALL_DIR:-$HOME/.archive-center}"
CHANNEL="latest"
START_AFTER=false
SYSTEMD=false
SERVICE_NAME="archive-center"
RUN_USER="${SUDO_USER:-$(id -un 2>/dev/null || printf archive-center)}"
EXTERNAL_OPERATION_TIMEOUT_SECONDS="${AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS:-}"
REQUEST_TIMEOUT_SECONDS="${AC_REQUEST_TIMEOUT_SECONDS:-30}"
READINESS_TIMEOUT_SECONDS="${AC_READINESS_TIMEOUT_SECONDS:-180}"
READINESS_POLL_INTERVAL_SECONDS="${AC_READINESS_POLL_INTERVAL_SECONDS:-1}"
SERVICE_RESTART_SECONDS="${AC_SERVICE_RESTART_SECONDS:-}"

usage() {
	cat <<'EOF'
Usage:
  install-github-release.sh [options]

Options:
  --repo OWNER/REPO       GitHub repository. Default: Flazer31/archive-center
  --install-dir PATH      Install root. Default: $HOME/.archive-center
  --start                 Start the selected package after install.
  --systemd               Linux only: install/update systemd service.
  --service-name NAME     systemd service name. Default: archive-center
  --user NAME             systemd service user. Default: $SUDO_USER/current user
  --external-operation-timeout-seconds N
                          Caller-selected bound for each GitHub HTTP operation.
                          Or set AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS.
  --service-restart-seconds N
                          systemd restart delay. Required with --systemd, or
                          set AC_SERVICE_RESTART_SECONDS.
  --help                  Show this help.

This installs from GitHub Release assets, not from raw git source.
EOF
}

die() {
	printf 'ERROR: %s\n' "$*" >&2
	exit 1
}

has_cmd() {
	command -v "$1" >/dev/null 2>&1
}

need_cmd() {
	has_cmd "$1" || die "$1 is required"
}

detect_platform() {
	os=$(uname -s 2>/dev/null || printf unknown)
	machine=$(uname -m 2>/dev/null || printf unknown)
	if printf '%s' "${PREFIX:-}" 2>/dev/null | grep -qi 'com.termux'; then
		case "$machine" in
			aarch64|arm64) printf 'termux-arm64' ; return 0 ;;
		esac
	fi
	case "$os/$machine" in
		Linux/x86_64|Linux/amd64) printf 'linux-x64' ;;
		Linux/aarch64|Linux/arm64) printf 'linux-arm64' ;;
		Darwin/x86_64|Darwin/amd64) printf 'macos-intel' ;;
		Darwin/arm64|Darwin/aarch64) printf 'macos-apple-silicon' ;;
		*) die "unsupported platform: $os/$machine" ;;
	esac
}

asset_filter_for_platform() {
	case "$1" in
		linux-x64) printf 'linux x64' ;;
		linux-arm64) printf 'linux arm64' ;;
		macos-intel) printf 'macos intel' ;;
		macos-apple-silicon) printf 'macos apple silicon' ;;
		termux-arm64) printf 'termux arm64' ;;
		*) die "unsupported update platform: $1" ;;
	esac
}

safe_link_current() {
	target=$1
	link=$2
	parent=$(dirname -- "$link")
	mkdir -p "$parent"
	ln -sfn "$target" "$link"
}

systemd_env_quote() {
	printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

while [ "$#" -gt 0 ]; do
	case "$1" in
		--repo)
			[ "$#" -ge 2 ] || die "missing value for --repo"
			REPO=$2
			shift 2
			;;
		--install-dir)
			[ "$#" -ge 2 ] || die "missing value for --install-dir"
			INSTALL_DIR=$2
			shift 2
			;;
		--start)
			START_AFTER=true
			shift
			;;
		--systemd)
			SYSTEMD=true
			shift
			;;
		--service-name)
			[ "$#" -ge 2 ] || die "missing value for --service-name"
			SERVICE_NAME=$2
			shift 2
			;;
		--user)
			[ "$#" -ge 2 ] || die "missing value for --user"
			RUN_USER=$2
			shift 2
			;;
		--external-operation-timeout-seconds)
			[ "$#" -ge 2 ] || die "missing value for --external-operation-timeout-seconds"
			EXTERNAL_OPERATION_TIMEOUT_SECONDS=$2
			shift 2
			;;
		--service-restart-seconds)
			[ "$#" -ge 2 ] || die "missing value for --service-restart-seconds"
			SERVICE_RESTART_SECONDS=$2
			shift 2
			;;
		--help|-h)
			usage
			exit 0
			;;
		*)
			die "unknown argument: $1"
			;;
	esac
done

case "$REPO" in
	*/*) ;;
	*) die "repo must be OWNER/REPO" ;;
esac

case "$EXTERNAL_OPERATION_TIMEOUT_SECONDS" in
	''|*[!0-9]*|0)
		die "supply --external-operation-timeout-seconds or AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS as a positive integer; no hidden download deadline is used"
		;;
esac
case "$SERVICE_RESTART_SECONDS" in
	"") ;;
	*[!0-9]*|0) die "service restart seconds must be a positive integer" ;;
esac
if [ "$SYSTEMD" = "true" ] && [ -z "$SERVICE_RESTART_SECONDS" ]; then
	die "--systemd requires --service-restart-seconds or AC_SERVICE_RESTART_SECONDS; no hidden restart delay is used"
fi

AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS=$EXTERNAL_OPERATION_TIMEOUT_SECONDS
AC_REQUEST_TIMEOUT_SECONDS=$REQUEST_TIMEOUT_SECONDS
AC_READINESS_TIMEOUT_SECONDS=$READINESS_TIMEOUT_SECONDS
AC_READINESS_POLL_INTERVAL_SECONDS=$READINESS_POLL_INTERVAL_SECONDS
export AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS AC_REQUEST_TIMEOUT_SECONDS
export AC_READINESS_TIMEOUT_SECONDS AC_READINESS_POLL_INTERVAL_SECONDS

need_cmd curl
need_cmd python3
need_cmd unzip

PLATFORM=$(detect_platform)
FILTER=$(asset_filter_for_platform "$PLATFORM")
API_URL="https://api.github.com/repos/$REPO/releases/$CHANNEL"
WORK_DIR=$(mktemp -d "${TMPDIR:-/tmp}/archive-center-update.XXXXXX")
PERSISTENT_DATA_DIR="${ARCHIVE_CENTER_DATA_DIR:-$INSTALL_DIR/data}"
previous_current=""
if [ -e "$INSTALL_DIR/current" ]; then
	previous_current=$(python3 - "$INSTALL_DIR/current" <<'PY'
import os
import sys
print(os.path.realpath(sys.argv[1]))
PY
)
fi
cleanup() {
	case "$WORK_DIR" in
		"${TMPDIR:-/tmp}"/archive-center-update.*)
			rm -rf -- "$WORK_DIR"
			;;
	esac
}
trap cleanup EXIT INT TERM

release_json="$WORK_DIR/release.json"
curl --connect-timeout "$EXTERNAL_OPERATION_TIMEOUT_SECONDS" --max-time "$EXTERNAL_OPERATION_TIMEOUT_SECONDS" -fsSL -H "Accept: application/vnd.github+json" -H "User-Agent: Archive-Center-Installer" "$API_URL" -o "$release_json"

release_tag=$(python3 - "$release_json" <<'PY'
import json, sys
data=json.load(open(sys.argv[1], encoding="utf-8"))
print(data.get("tag_name") or "latest")
PY
)

asset_name=$(python3 - "$release_json" "$FILTER" <<'PY'
import json, sys
import re
data=json.load(open(sys.argv[1], encoding="utf-8"))
def comparable(value):
    return " ".join(re.sub(r"[^a-z0-9]+", " ", value.lower()).split())
needle=comparable(sys.argv[2])
for asset in data.get("assets", []):
    name=(asset.get("name") or "")
    low=comparable(name)
    if name.lower().endswith(".zip") and needle in low and "archive center" in low:
        print(name)
        break
PY
)
[ -n "$asset_name" ] || die "no release package asset matched platform $PLATFORM"

asset_url=$(python3 - "$release_json" "$asset_name" <<'PY'
import json, sys
data=json.load(open(sys.argv[1], encoding="utf-8"))
want=sys.argv[2]
for asset in data.get("assets", []):
    if asset.get("name") == want:
        print(asset.get("browser_download_url") or "")
        break
PY
)
[ -n "$asset_url" ] || die "selected asset has no download URL"

checksum_name=$(python3 - "$release_json" <<'PY'
import json, re, sys
data=json.load(open(sys.argv[1], encoding="utf-8"))
for asset in data.get("assets", []):
    name=(asset.get("name") or "").strip()
    if re.fullmatch(r"SHA256SUMS(?:-[A-Za-z0-9_.-]+)?\.txt", name):
        print(name)
        break
PY
)
[ -n "$checksum_name" ] || die "the latest release has no SHA256SUMS checksum asset"

checksum_url=$(python3 - "$release_json" "$checksum_name" <<'PY'
import json, sys
data=json.load(open(sys.argv[1], encoding="utf-8"))
want=sys.argv[2]
for asset in data.get("assets", []):
    if asset.get("name") == want:
        print(asset.get("browser_download_url") or "")
        break
PY
)
[ -n "$checksum_url" ] || die "selected checksum asset has no download URL"

zip_path="$WORK_DIR/$asset_name"
checksum_path="$WORK_DIR/$checksum_name"
curl --connect-timeout "$EXTERNAL_OPERATION_TIMEOUT_SECONDS" --max-time "$EXTERNAL_OPERATION_TIMEOUT_SECONDS" -fL -H "User-Agent: Archive-Center-Installer" "$checksum_url" -o "$checksum_path"
curl --connect-timeout "$EXTERNAL_OPERATION_TIMEOUT_SECONDS" --max-time "$EXTERNAL_OPERATION_TIMEOUT_SECONDS" -fL -H "User-Agent: Archive-Center-Installer" "$asset_url" -o "$zip_path"
python3 - "$checksum_path" "$asset_name" "$zip_path" <<'PY'
import hashlib, re, sys
sums_path, asset_name, zip_path = sys.argv[1:]
expected = ""
with open(sums_path, "r", encoding="ascii") as handle:
    for raw in handle:
        line = raw.rstrip("\r\n")
        parts = line.split("  ", 1)
        if len(parts) == 2 and parts[1] == asset_name and re.fullmatch(r"[0-9A-Fa-f]{64}", parts[0]):
            expected = parts[0].lower()
            break
if not expected:
    raise SystemExit("ERROR: release checksum list has no exact SHA-256 record for " + asset_name)
digest = hashlib.sha256()
with open(zip_path, "rb") as handle:
    for chunk in iter(lambda: handle.read(1024 * 1024), b""):
        digest.update(chunk)
if digest.hexdigest() != expected:
    raise SystemExit("ERROR: release package SHA-256 mismatch for " + asset_name)
PY

version_dir=$(printf '%s' "$release_tag" | tr -c 'A-Za-z0-9_.-' '_')
target_dir="$INSTALL_DIR/releases/$version_dir"
mkdir -p "$target_dir"
unzip -q -o "$zip_path" -d "$target_dir"

package_root=$(python3 - "$target_dir" <<'PY'
import os
import sys
root = sys.argv[1]
needles = {
    "start-archive-center-linux.sh",
    "Start Archive Center macOS.command",
    "install-and-start-termux.sh",
}
for current, dirs, files in os.walk(root):
    rel_depth = os.path.relpath(current, root).count(os.sep)
    if rel_depth > 2:
        dirs[:] = []
        continue
    for name in files:
        if name in needles:
            print(current)
            raise SystemExit(0)
raise SystemExit(0)
PY
)
[ -n "$package_root" ] || die "extracted package launcher was not found"

mkdir -p "$PERSISTENT_DATA_DIR"
if [ ! -d "$PERSISTENT_DATA_DIR/mariadb-data" ] && [ -n "$previous_current" ] && [ -d "$previous_current/.runtime/mariadb-data" ]; then
	printf 'Migrating package-local runtime data to persistent data directory:\n'
	printf '  From: %s/.runtime\n' "$previous_current"
	printf '  To:   %s\n' "$PERSISTENT_DATA_DIR"
	cp -a "$previous_current/.runtime/." "$PERSISTENT_DATA_DIR/"
	rm -f "$PERSISTENT_DATA_DIR/mysql.sock" "$PERSISTENT_DATA_DIR/mariadb.pid"
fi

safe_link_current "$package_root" "$INSTALL_DIR/current"
data_root_pointer="$INSTALL_DIR/data-root.txt"
stable_launcher="$INSTALL_DIR/start-archive-center.sh"
case "$PLATFORM" in
	linux-*) current_launcher="start-archive-center-linux.sh" ;;
	macos-*) current_launcher="Start Archive Center macOS.command" ;;
	termux-*) current_launcher="install-and-start-termux.sh" ;;
	*) die "installed package launcher could not be selected for $PLATFORM" ;;
esac
printf '%s\n' "$PERSISTENT_DATA_DIR" > "$data_root_pointer"
{
	printf '%s\n' '#!/usr/bin/env sh'
	printf '%s\n' 'set -eu'
	printf '%s\n' 'INSTALL_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)'
	printf '%s\n' 'DATA_ROOT_POINTER="$INSTALL_ROOT/data-root.txt"'
	printf '%s\n' '[ -f "$DATA_ROOT_POINTER" ] || { printf '\''ERROR: installed data-root pointer is missing: %s\n'\'' "$DATA_ROOT_POINTER" >&2; exit 1; }'
	printf '%s\n' 'IFS= read -r ARCHIVE_CENTER_DATA_DIR < "$DATA_ROOT_POINTER"'
	printf '%s\n' '[ -n "$ARCHIVE_CENTER_DATA_DIR" ] || { printf '\''ERROR: installed data-root pointer is empty: %s\n'\'' "$DATA_ROOT_POINTER" >&2; exit 1; }'
	printf '%s\n' 'export ARCHIVE_CENTER_DATA_DIR'
	printf 'exec sh "$INSTALL_ROOT/current/%s" "$@"\n' "$current_launcher"
} > "$stable_launcher"
chmod 755 "$stable_launcher"
printf '%s\n' "$release_tag" > "$INSTALL_DIR/current-version.txt"

printf 'Installed Archive Center %s\n' "$release_tag"
printf '  Platform: %s\n' "$PLATFORM"
printf '  Package:  %s\n' "$package_root"
printf '  Current:  %s/current\n' "$INSTALL_DIR"

if [ "$SYSTEMD" = "true" ]; then
	case "$PLATFORM" in
		linux-*) ;;
		*) die "--systemd is supported only on Linux" ;;
	esac
	[ "$(id -u)" = "0" ] || die "--systemd requires root"
	ARCHIVE_CENTER_DATA_DIR="$PERSISTENT_DATA_DIR" sh "$package_root/scripts/start-full-linux.sh" --profile full_local --vector-mode local_native --install-only
	unit_path="/etc/systemd/system/$SERVICE_NAME.service"
	data_dir_escaped=$(systemd_env_quote "$PERSISTENT_DATA_DIR")
	cat > "$unit_path" <<EOF
[Unit]
Description=Archive Center
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$RUN_USER
Group=$RUN_USER
WorkingDirectory=$INSTALL_DIR/current
Environment="ARCHIVE_CENTER_DATA_DIR=$data_dir_escaped"
Environment="AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS=$EXTERNAL_OPERATION_TIMEOUT_SECONDS"
Environment="AC_REQUEST_TIMEOUT_SECONDS=$REQUEST_TIMEOUT_SECONDS"
Environment="AC_READINESS_TIMEOUT_SECONDS=$READINESS_TIMEOUT_SECONDS"
Environment="AC_READINESS_POLL_INTERVAL_SECONDS=$READINESS_POLL_INTERVAL_SECONDS"
ExecStart=/bin/sh $INSTALL_DIR/start-archive-center.sh --no-install
Restart=on-failure
RestartSec=$SERVICE_RESTART_SECONDS
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOF
	chown -R "$RUN_USER:$RUN_USER" "$INSTALL_DIR" 2>/dev/null || true
	systemctl daemon-reload
	systemctl enable "$SERVICE_NAME"
	systemctl restart "$SERVICE_NAME"
	printf 'systemd service restarted: %s\n' "$SERVICE_NAME"
fi

if [ "$START_AFTER" = "true" ]; then
	exec sh "$stable_launcher"
fi
