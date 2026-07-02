# Archive Center Oracle Cloud Systemd Update Guide

This guide is for Oracle Cloud Ubuntu servers where Archive Center and
PocketRisu run together. Archive Center must run as a background systemd
service, not as a foreground terminal process.

## Problem Symptom

If the terminal shows:

```text
Stop with Ctrl+C.
{"level":"INFO","msg":"loaded config",...}
{"level":"INFO","msg":"starting server","bind":"0.0.0.0:28080",...}
```

Archive Center is running in the foreground. This is not the desired server
mode because the terminal is attached to the Archive Center process.

On Oracle Cloud, use systemd so Archive Center stays alive in the background
while PocketRisu continues to run separately.

## Stop Foreground Run

If the current terminal is attached to Archive Center, stop it first:

```bash
Ctrl+C
```

## Install Or Update Through Systemd

Do not pass `--start` when the goal is a background service. Use the release
installer with `--systemd`, then control the service through `systemctl`.

```bash
cd /opt/archive-center-installer

sudo git fetch origin
sudo git reset --hard origin/main

sudo sh scripts/install-github-release.sh \
  --repo Flazer31/archive-center \
  --install-dir /opt/archive-center \
  --systemd \
  --user ubuntu
```

Then reload and restart the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable archive-center
sudo systemctl restart archive-center
```

## Verify Service State

```bash
systemctl status archive-center --no-pager -l
systemctl is-enabled archive-center
systemctl is-active archive-center
```

Check the listening ports:

```bash
ss -ltnp | grep -E ':(28080|3307|8000)'
```

Check readiness:

```bash
curl http://127.0.0.1:28080/ready
curl http://100.124.213.10:28080/ready
```

The service should report the full runtime profile:

```text
mode=live
store_mode=mariadb_authority
runtime_profile=full_local
vector_mode=local_native
degraded=false
```

## Check Logs

```bash
journalctl -u archive-center -n 80 --no-pager
```

Use this when `/ready` fails or the service restarts repeatedly.

## Normal Future Update Flow

For later release updates, repeat:

```bash
cd /opt/archive-center-installer
sudo git fetch origin
sudo git reset --hard origin/main

sudo sh scripts/install-github-release.sh \
  --repo Flazer31/archive-center \
  --install-dir /opt/archive-center \
  --systemd \
  --user ubuntu

sudo systemctl restart archive-center
```

Then verify:

```bash
cat /opt/archive-center/current-version.txt
curl http://127.0.0.1:28080/ready
journalctl -u archive-center -n 80 --no-pager
```

## PocketRisu Compatibility

PocketRisu can run on the same Oracle Cloud server as long as ports do not
conflict. Archive Center normally uses:

- `28080` for the Archive Center backend
- `3307` for the managed MariaDB runtime
- `8000` for the managed ChromaDB runtime

If PocketRisu uses other ports, both services can stay online at the same time.

## Important Rule

Do not run `scripts/start-full-posix.sh` directly for long-running Oracle Cloud
operation. Direct script execution is useful for local manual debugging, but
production-like server use should go through `systemctl`.
