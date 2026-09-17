#!/usr/bin/env sh
set -eu
REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
TEMP_ROOT=$(mktemp -d)
trap 'rm -rf -- "$TEMP_ROOT"' EXIT HUP INT TERM
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
package="$TEMP_ROOT/package"
mkdir -p "$package/bin" "$package/migrations"
touch "$package/migrations/001_schema.sql" "$package/bin/archive-center-go" "$package/bin/mariadb-schema"
chmod +x "$package/bin/"*
export ARCHIVE_CENTER_PACKAGE_ROOT="$package"
launcher="$REPO_ROOT/ops/full-package-posix/start-full-posix.sh"
for platform in linux macos termux; do
    export ARCHIVE_CENTER_DATA_DIR="$TEMP_ROOT/data-$platform"
    unset AC_CHROMA_ENDPOINT
    run() { sh "$launcher" --platform "$platform" --profile full_local "$@"; }
    run --preflight > "$TEMP_ROOT/default"
    grep -q '"chroma_endpoint": "http://127.0.0.1:8000"' "$TEMP_ROOT/default" || fail "$platform default"
    [ ! -e "$ARCHIVE_CENTER_DATA_DIR" ] || fail 'preflight changed settings'
    printf '8001\n' | run --configure-chroma-port > "$TEMP_ROOT/save"
    [ "$(cat "$ARCHIVE_CENTER_DATA_DIR/chroma-port.txt")" = 8001 ] || fail 'menu did not save'
    export AC_CHROMA_ENDPOINT=http://127.0.0.1:8000
    run --preflight > "$TEMP_ROOT/restart"
    grep -q '"chroma_endpoint": "http://127.0.0.1:8001"' "$TEMP_ROOT/restart" || fail 'restart lost saved setting'
    # The selection lives outside the release directory, including after replacement.
    mkdir -p "$TEMP_ROOT/replacement/bin" "$TEMP_ROOT/replacement/migrations"
    cp "$package/bin/"* "$TEMP_ROOT/replacement/bin/"
    cp "$package/migrations/001_schema.sql" "$TEMP_ROOT/replacement/migrations/"
    (export ARCHIVE_CENTER_PACKAGE_ROOT="$TEMP_ROOT/replacement"; run --preflight) > "$TEMP_ROOT/replaced"
    grep -q '"chroma_endpoint": "http://127.0.0.1:8001"' "$TEMP_ROOT/replaced" || fail 'package change lost saved setting'
    printf '\n' | run --configure-chroma-port > "$TEMP_ROOT/reset-empty"
    [ "$(cat "$ARCHIVE_CENTER_DATA_DIR/chroma-port.txt")" = 8000 ] || fail 'empty input did not restore default'
    for invalid in 0 65536 abc 1.5 -1 '8001;echo bad'; do
        if run --configure-chroma-port --chroma-port "$invalid" > "$TEMP_ROOT/invalid" 2>&1; then fail 'invalid port accepted'; fi
        [ "$(cat "$ARCHIVE_CENTER_DATA_DIR/chroma-port.txt")" = 8000 ] || fail 'invalid input overwrote setting'
    done
    run --chroma-port 08002 --preflight > "$TEMP_ROOT/explicit"
    grep -q '"chroma_endpoint": "http://127.0.0.1:8002"' "$TEMP_ROOT/explicit" || fail 'explicit selection failed'
    export AC_CHROMA_ENDPOINT=https://vector.example:9443
    run --profile vector_external --vector-mode external --preflight > "$TEMP_ROOT/external"
    grep -q '"chroma_endpoint": "https://vector.example:9443"' "$TEMP_ROOT/external" || fail 'external endpoint changed'
    run --profile core_lite --vector-mode off --preflight > "$TEMP_ROOT/off"
    grep -q '"chroma_endpoint": ""' "$TEMP_ROOT/off" || fail 'off profile activated ChromaDB'
    run --configure-chroma-port --chroma-port 8000 > "$TEMP_ROOT/reset"
    [ "$(cat "$ARCHIVE_CENTER_DATA_DIR/chroma-port.txt")" = 8000 ] || fail 'reset failed'
    for service in mariadb backend; do
        case "$service" in
            mariadb) selection=2; value=3311; default=3307 ;;
            backend) selection=3; value=28111; default=28080 ;;
        esac
        printf '%s\n%s\n' "$selection" "$value" | run --configure-ports > "$TEMP_ROOT/menu"
        [ "$(cat "$ARCHIVE_CENTER_DATA_DIR/$service-port.txt")" = "$value" ] || fail 'service menu did not save'
        for invalid in 0 65536 bad -1; do
            if run --configure-ports --port-service "$service" "--$service-port" "$invalid" > "$TEMP_ROOT/invalid" 2>&1; then fail 'invalid service port accepted'; fi
            [ "$(cat "$ARCHIVE_CENTER_DATA_DIR/$service-port.txt")" = "$value" ] || fail 'invalid service input changed settings'
        done
        # EOF at the port prompt also restores that service's default.
        run --configure-ports --port-service "$service" < /dev/null > "$TEMP_ROOT/eof"
        [ "$(cat "$ARCHIVE_CENTER_DATA_DIR/$service-port.txt")" = "$default" ] || fail 'EOF did not restore default'
        run "--$service-port" "$value" --preflight > "$TEMP_ROOT/port-preflight"
    done
    # Scope temporary fixture environment to this probe on every POSIX shell.
    (export AC_BIND_ADDR='[::]:28080'; run --preflight) > "$TEMP_ROOT/services"
    grep -Fq '"mariadb_port": "3311"' "$TEMP_ROOT/services" || fail 'MariaDB restart port'
    grep -Fq '"backend_bind": "[::]:28111"' "$TEMP_ROOT/services" || fail 'Backend port or IPv6 host'
    (export ARCHIVE_CENTER_PACKAGE_ROOT="$TEMP_ROOT/replacement"; run --preflight) > "$TEMP_ROOT/services-replaced"
    grep -Fq '"backend_bind": "0.0.0.0:28111"' "$TEMP_ROOT/services-replaced" || { cat "$TEMP_ROOT/services-replaced" >&2; fail 'Backend setting lost on update'; }
    [ ! -e "$ARCHIVE_CENTER_DATA_DIR/mariadb-data" ] || fail 'Configuration created DB'
done

# Load the production functions; only dependency/process/socket boundaries below are fakes.
sed '/^PLATFORM=${ARCHIVE_CENTER_PLATFORM:-}/,$d' "$launcher" > "$TEMP_ROOT/functions.sh"
. "$TEMP_ROOT/functions.sh"
DATA_ROOT="$TEMP_ROOT/launch-data"
RUNTIME_DIR="$DATA_ROOT"
CHROMA_DATA="$DATA_ROOT/chromadb-data"
LOG_DIR="$DATA_ROOT/logs"
AC_VECTOR_MODE=local_proot
CONFIGURE_PORTS=false
REQUESTED_CHROMA_PORT=8001
REQUESTED_MARIADB_PORT=3311
REQUESTED_BACKEND_PORT=28111
AC_BIND_ADDR=127.0.0.1:28080
MARIADB_PORT=3307
configure_service_ports
export AC_CHROMA_ENDPOINT
ensure_chromadb() {
    # External runtime provisioning is outside this test; paths are sentinels.
    PROOT_CHROMA_DISTRO=ubuntu
    PROOT_CHROMA_DATA='/root/existing memory/chromadb-data'
    PROOT_CHROMA_VENV=/root/archive-center/chromadb-venv
    CHROMA_PYTHON=fixture-python
}
port_is_open() { [ "$1" = 8001 ] || fail 'probe used wrong port'; return 1; }
start_managed_process() {
    printf '%s\n' "$@" > "$TEMP_ROOT/launched"
    printf '%s' "$AC_CHROMA_ENDPOINT" > "$TEMP_ROOT/backend-env"
    MANAGED_PROCESS_PID=fixture-process
}
wait_port() { [ "$1" = 8001 ] && [ "$3" = fixture-process ] || fail 'readiness used wrong port/process'; }
PLATFORM=termux
start_chromadb
grep -q -- "--port '8001' --path '/root/existing memory/chromadb-data'" "$TEMP_ROOT/launched" || fail 'Termux port or DB path'
[ "$(cat "$TEMP_ROOT/backend-env")" = http://127.0.0.1:8001 ] || fail 'backend endpoint differs'
for PLATFORM in linux macos; do
    AC_VECTOR_MODE=local_native
    start_chromadb
    grep -Fxq 8001 "$TEMP_ROOT/launched" || fail 'native port'
    grep -Fxq "$CHROMA_DATA" "$TEMP_ROOT/launched" || fail 'native DB path'
done
find_mariadb_tools() { MARIADBD=fixture-mariadb; }
port_is_open() { [ "$1" = 3311 ] || fail 'MariaDB probe port'; return 1; }
wait_port() { [ "$1" = 3311 ] && [ "$3" = fixture-process ] || fail 'MariaDB readiness port'; }
MARIADB_DATA="$DATA_ROOT/existing-mariadb"
mkdir -p "$MARIADB_DATA/mysql"
start_mariadb
grep -Fxq -- '--port=3311' "$TEMP_ROOT/launched" || fail 'MariaDB server port'
grep -Fxq -- "--datadir=$MARIADB_DATA" "$TEMP_ROOT/launched" || fail 'MariaDB data directory changed'
PACKAGE_ROOT="$package"
MARIADB_SCHEMA_RUN=fixture-schema
run_external() { printf '%s\n' "$@" > "$TEMP_ROOT/schema"; }
bootstrap_mariadb_schema
grep -Fxq '3311' "$TEMP_ROOT/schema" || fail 'Schema managed port'
grep -Fq '@tcp(127.0.0.1:3311)/' "$TEMP_ROOT/schema" || fail 'Schema DSN port'
case "$AC_MARIADB_DSN" in *'@tcp(127.0.0.1:3311)/'*) ;; *) fail 'Go DSN port' ;; esac
[ "$AC_BIND_ADDR" = 127.0.0.1:28111 ] || fail 'Go bind port or host'
printf 'Service ports: POSIX configuration and startup/schema boundaries passed.\n'
