#!/usr/bin/env sh
set -eu

REPO="Flazer31/archive-center"
INSTALL_DIR="${ARCHIVE_CENTER_INSTALL_DIR:-$HOME/.archive-center}"
CHANNEL="latest"
START_AFTER=false
SYSTEMD=false
SERVICE_NAME="archive-center"
RUN_USER="${SUDO_USER:-$(id -un 2>/dev/null || printf archive-center)}"

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

sha256_file() {
	if has_cmd sha256sum; then
		sha256sum "$1" | awk '{print $1}'
	elif has_cmd shasum; then
		shasum -a 256 "$1" | awk '{print $1}'
	else
		die "sha256sum or shasum is required"
	fi
}

safe_link_current() {
	target=$1
	link=$2
	parent=$(dirname -- "$link")
	mkdir -p "$parent"
	ln -sfn "$target" "$link"
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

need_cmd curl
need_cmd python3
need_cmd unzip

PLATFORM=$(detect_platform)
FILTER=$(asset_filter_for_platform "$PLATFORM")
API_URL="https://api.github.com/repos/$REPO/releases/$CHANNEL"
WORK_DIR=$(mktemp -d "${TMPDIR:-/tmp}/archive-center-update.XXXXXX")
cleanup() {
	case "$WORK_DIR" in
		"${TMPDIR:-/tmp}"/archive-center-update.*)
			rm -rf -- "$WORK_DIR"
			;;
	esac
}
trap cleanup EXIT INT TERM

release_json="$WORK_DIR/release.json"
curl -fsSL -H "Accept: application/vnd.github+json" -H "User-Agent: Archive-Center-Installer" "$API_URL" -o "$release_json"

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

sums_name=$(python3 - "$release_json" <<'PY'
import json, sys
data=json.load(open(sys.argv[1], encoding="utf-8"))
for asset in data.get("assets", []):
    name=(asset.get("name") or "")
    low=name.lower()
    if low.startswith("sha256sums") and low.endswith(".txt"):
        print(name)
        break
PY
)
[ -n "$sums_name" ] || die "release has no SHA256SUMS asset"

sums_url=$(python3 - "$release_json" "$sums_name" <<'PY'
import json, sys
data=json.load(open(sys.argv[1], encoding="utf-8"))
want=sys.argv[2]
for asset in data.get("assets", []):
    if asset.get("name") == want:
        print(asset.get("browser_download_url") or "")
        break
PY
)
[ -n "$sums_url" ] || die "SHA256SUMS asset has no download URL"

zip_path="$WORK_DIR/$asset_name"
sums_path="$WORK_DIR/$sums_name"
curl -fsSL -H "User-Agent: Archive-Center-Installer" "$sums_url" -o "$sums_path"
curl -fL -H "User-Agent: Archive-Center-Installer" "$asset_url" -o "$zip_path"

expected=$(python3 - "$sums_path" "$asset_name" <<'PY'
import re
import sys
sums, want = sys.argv[1], sys.argv[2]
def comparable(value):
    return " ".join(re.sub(r"[^a-z0-9]+", " ", value.lower()).split())
want_cmp = comparable(want)
for line in open(sums, encoding="utf-8"):
    parts=line.strip().split()
    if len(parts) >= 2 and comparable(" ".join(parts[1:]).lstrip("*")) == want_cmp:
        print(parts[0].lower())
        break
PY
)
[ -n "$expected" ] || die "SHA256SUMS did not contain $asset_name"
actual=$(sha256_file "$zip_path")
[ "$actual" = "$expected" ] || die "sha256 mismatch for $asset_name"

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

safe_link_current "$package_root" "$INSTALL_DIR/current"
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
	sh "$package_root/scripts/start-full-linux.sh" --profile full_local --vector-mode local_native --install-only
	unit_path="/etc/systemd/system/$SERVICE_NAME.service"
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
ExecStart=/bin/sh $INSTALL_DIR/current/start-archive-center-linux.sh --no-install
Restart=on-failure
RestartSec=5
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
	case "$PLATFORM" in
		linux-*) exec sh "$INSTALL_DIR/current/start-archive-center-linux.sh" ;;
		macos-*) exec sh "$INSTALL_DIR/current/scripts/start-full-macos.sh" ;;
		termux-*) exec sh "$INSTALL_DIR/current/install-and-start-termux.sh" ;;
	esac
fi
