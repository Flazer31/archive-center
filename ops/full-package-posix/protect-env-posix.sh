#!/usr/bin/env sh
set -eu

ENV_FILE=${1:-.env.full.local}

if [ ! -f "$ENV_FILE" ]; then
	printf 'Env file not found: %s\n' "$ENV_FILE" >&2
	exit 1
fi

chmod 600 "$ENV_FILE"
printf 'Protected env permissions: %s\n' "$ENV_FILE"
printf 'Note: chmod 600 blocks other normal OS users, but not the same user or root/admin.\n'
