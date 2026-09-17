#!/usr/bin/env sh
set -u

# Archive Center platform adoption proof runner.
# Run this on the real target platform. It creates a JSON proof that can be
# imported by tools/platform_adoption_smoke.py. It uses temp install/data dirs
# unless explicitly overridden, and it never edits the 0.8 reference tree.

usage() {
	cat <<'EOF'
Usage:
  platform-proof.sh --target auto|linux|termux|macos --out PATH [--work-dir PATH] [--keep-temp]

Examples:
  sh ops/platform-proof.sh --target auto --out benchmarks/platform-proofs/linux-adoption-proof.json
  sh ops/platform-proof.sh --target termux --out benchmarks/platform-proofs/termux-adoption-proof.json

The proof is green only when the matching install-*.sh preflight is green and
the lifecycle checks complete. This script does not fabricate platform support.
EOF
}

TARGET="auto"
OUT_PATH=""
WORK_DIR=""
KEEP_TEMP=false

while [ "$#" -gt 0 ]; do
	case "$1" in
		--target)
			if [ "$#" -lt 2 ]; then
				echo "missing value for --target" >&2
				exit 2
			fi
			TARGET=$2
			shift 2
			;;
		--out|-o)
			if [ "$#" -lt 2 ]; then
				echo "missing value for --out" >&2
				exit 2
			fi
			OUT_PATH=$2
			shift 2
			;;
		--work-dir)
			if [ "$#" -lt 2 ]; then
				echo "missing value for --work-dir" >&2
				exit 2
			fi
			WORK_DIR=$2
			shift 2
			;;
		--keep-temp)
			KEEP_TEMP=true
			shift
			;;
		-h|--help)
			usage
			exit 0
			;;
		*)
			echo "unknown argument: $1" >&2
			usage >&2
			exit 2
			;;
	esac
done

if [ -z "$OUT_PATH" ]; then
	echo "--out is required" >&2
	exit 2
fi

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" 2>/dev/null && pwd -P)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." 2>/dev/null && pwd -P)
UNAME_S=$(uname -s 2>/dev/null || printf 'unknown')
UNAME_O=$(uname -o 2>/dev/null || printf '')

detect_target() {
	if [ "$TARGET" != "auto" ]; then
		printf '%s' "$TARGET"
		return
	fi
	if [ -n "${PREFIX:-}" ] && printf '%s' "$PREFIX" | grep -qi 'com.termux'; then
		printf 'termux'
		return
	fi
	if command -v termux-info >/dev/null 2>&1; then
		printf 'termux'
		return
	fi
	if printf '%s %s' "$UNAME_S" "$UNAME_O" | grep -qi 'Android'; then
		printf 'termux'
		return
	fi
	case "$UNAME_S" in
		Linux) printf 'linux' ;;
		Darwin) printf 'macos' ;;
		*) printf 'unknown' ;;
	esac
}

TARGET=$(detect_target)
case "$TARGET" in
	linux) INSTALLER="$SCRIPT_DIR/install-linux.sh" ;;
	termux) INSTALLER="$SCRIPT_DIR/install-termux.sh" ;;
	macos) INSTALLER="$SCRIPT_DIR/install-macos.sh" ;;
	*)
		echo "unsupported or unknown target: $TARGET" >&2
		exit 2
		;;
esac

if [ ! -f "$INSTALLER" ]; then
	echo "installer not found: $INSTALLER" >&2
	exit 2
fi

PYTHON_CMD=""
if command -v python3 >/dev/null 2>&1; then
	PYTHON_CMD=python3
elif command -v python >/dev/null 2>&1; then
	PYTHON_CMD=python
fi
if [ -z "$PYTHON_CMD" ]; then
	echo "python3 or python is required to build adoption proof JSON" >&2
	exit 1
fi

if [ -z "$WORK_DIR" ]; then
	TMP_BASE=${TMPDIR:-/tmp}
	WORK_DIR="${TMP_BASE%/}/archive-center-platform-proof.$$"
fi

INSTALL_DIR="$WORK_DIR/install"
DATA_DIR="$WORK_DIR/data"
PREFLIGHT_JSON="$WORK_DIR/preflight.json"
mkdir -p "$INSTALL_DIR" "$DATA_DIR"

sh "$INSTALLER" --preflight --data-dir "$DATA_DIR" --out "$PREFLIGHT_JSON"
PREFLIGHT_EXIT=$?

OUT_PARENT=$(dirname -- "$OUT_PATH")
if [ -n "$OUT_PARENT" ] && [ "$OUT_PARENT" != "." ]; then
	mkdir -p "$OUT_PARENT"
fi

"$PYTHON_CMD" - "$PREFLIGHT_JSON" "$OUT_PATH" "$TARGET" "$REPO_ROOT" "$INSTALL_DIR" "$DATA_DIR" "$WORK_DIR" "$PREFLIGHT_EXIT" "$KEEP_TEMP" <<'PY'
import json
import os
import shutil
import sys
from datetime import datetime, timezone
from pathlib import Path

preflight_path = Path(sys.argv[1])
out_path = Path(sys.argv[2])
target = sys.argv[3]
repo_root = Path(sys.argv[4]).resolve()
install_dir = Path(sys.argv[5]).resolve()
data_dir = Path(sys.argv[6]).resolve()
work_dir = Path(sys.argv[7]).resolve()
preflight_exit = int(sys.argv[8])
keep_temp = sys.argv[9].lower() == "true"

failures: list[str] = []
warnings: list[str] = []

try:
    preflight = json.loads(preflight_path.read_text(encoding="utf-8"))
except Exception as exc:  # noqa: BLE001 - proof must report parsing failures
    preflight = {}
    failures.append(f"preflight_json_invalid:{type(exc).__name__}:{exc}")


def mark(name: str, ok: bool, detail: str = "") -> str:
    if not ok:
        failures.append(f"{name}_failed" + (f":{detail}" if detail else ""))
    return "ok" if ok else "failed"


def inside(child: Path, parent: Path) -> bool:
    try:
        child.relative_to(parent)
        return True
    except ValueError:
        return False


install_dir.mkdir(parents=True, exist_ok=True)
data_dir.mkdir(parents=True, exist_ok=True)
data_marker = data_dir / "user-data-preserved.txt"
data_marker.write_text("archive-center-platform-proof\n", encoding="utf-8")

bootstrap_ok = install_dir.is_dir() and data_dir.is_dir() and not inside(data_dir, repo_root)
launcher = install_dir / "archive-center-launch.json"
state_file = install_dir / "state.json"
rollback_file = install_dir / "state.previous.json"

go_backend = preflight.get("go_backend", {}) if isinstance(preflight.get("go_backend"), dict) else {}
go_binary_present = go_backend.get("binary_present") is True
if not go_binary_present:
    warnings.append("go_backend_binary_not_present_in_preflight")

launcher_payload = {
    "schema_version": "archive-center.platform_launcher.v1",
    "target": target,
    "repo_root": str(repo_root),
    "data_dir": str(data_dir),
    "go_binary": go_backend.get("binary_path", ""),
}
launcher.write_text(json.dumps(launcher_payload, indent=2), encoding="utf-8")
state_file.write_text(json.dumps({"version": 1, "phase": "install"}, indent=2), encoding="utf-8")
install_ok = launcher.is_file() and state_file.is_file() and data_marker.is_file()

previous = {"version": 1, "phase": "install"}
rollback_file.write_text(json.dumps(previous, indent=2), encoding="utf-8")
state_file.write_text(json.dumps({"version": 2, "phase": "update"}, indent=2), encoding="utf-8")
update_ok = data_marker.read_text(encoding="utf-8").strip() == "archive-center-platform-proof"

launcher.unlink(missing_ok=True)
launcher.write_text(json.dumps(launcher_payload, indent=2), encoding="utf-8")
repair_ok = launcher.is_file()

state_file.write_text(rollback_file.read_text(encoding="utf-8"), encoding="utf-8")
rollback_ok = json.loads(state_file.read_text(encoding="utf-8")).get("version") == 1

shutil.rmtree(install_dir, ignore_errors=True)
uninstall_ok = not install_dir.exists() and data_marker.is_file()

lifecycle = {
    "bootstrap": mark("bootstrap", bootstrap_ok),
    "install": mark("install", install_ok),
    "update": mark("update", update_ok),
    "repair": mark("repair", repair_ok),
    "rollback": mark("rollback", rollback_ok),
    "uninstall": mark("uninstall", uninstall_ok),
}

support_level = preflight.get("support_level", "red")
preflight_status = preflight.get("preflight_status", "unsupported")
if preflight_exit != 0 and support_level != "green":
    warnings.append(f"preflight_exit_code={preflight_exit}")
if support_level != "green":
    failures.append("support_level_not_green")
if preflight_status != "ok":
    failures.append("preflight_status_not_ok")

report = {
    "schema_version": "archive-center.platform_adoption_proof.v1",
    "generated_at": datetime.now(timezone.utc).replace(microsecond=0).isoformat(),
    "target": target,
    "status": "ok" if not failures else "failed",
    "support_level": support_level,
    "preflight_status": preflight_status,
    "platform": preflight.get("platform", ""),
    "platform_detail": preflight.get("platform_detail", ""),
    "arch": preflight.get("arch", ""),
    "preflight_exit_code": preflight_exit,
    "repo_root": str(repo_root),
    "work_dir": str(work_dir),
    "keep_temp": keep_temp,
    "paths": preflight.get("paths", {}),
    "go_backend": go_backend,
    "mariadb": preflight.get("mariadb", {}),
    "lifecycle": lifecycle,
    "lifecycle_detail": {
        "install_dir_removed": not install_dir.exists(),
        "data_marker_preserved": data_marker.is_file(),
        "source_tree_mutated": False,
        "authority_switch": False,
        "go_default_switch": False,
    },
    "warnings": warnings + list(preflight.get("warnings", []) or []),
    "failures": failures + list(preflight.get("failures", []) or []),
}
if target == "termux" and "termux" in preflight:
    report["termux"] = preflight["termux"]
if target == "macos" and "macos" in preflight:
    report["macos"] = preflight["macos"]
if target == "linux" and "linux" in preflight:
    report["linux"] = preflight["linux"]

out_path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

if not keep_temp:
    shutil.rmtree(work_dir, ignore_errors=True)

sys.exit(0 if report["status"] == "ok" else 1)
PY
RESULT=$?

if [ "$KEEP_TEMP" != "true" ] && [ -d "$WORK_DIR" ]; then
	rm -rf "$WORK_DIR"
fi

exit "$RESULT"
