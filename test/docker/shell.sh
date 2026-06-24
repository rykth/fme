#!/usr/bin/env bash
# Brings up the SSH server and drops you into an interactive shell on the client
# with the remote mounted at /mnt/remote via fme. Pass a command to run it
# instead of an interactive shell, e.g. `./shell.sh ls -l /mnt/remote`.
set -euo pipefail

cd "$(dirname "$0")"

if ! docker info >/dev/null 2>&1; then
  echo "error: docker is not available / not running" >&2
  exit 1
fi
if [ ! -e /dev/fuse ]; then
  echo "error: /dev/fuse not present on host (load the 'fuse' kernel module)" >&2
  exit 1
fi

mkdir -p .keys
if [ ! -f .keys/id_ed25519 ]; then
  echo "==> generating test SSH keypair"
  ssh-keygen -t ed25519 -N "" -f .keys/id_ed25519 -C fme-docker-test >/dev/null
fi

cleanup() { docker compose down -v --remove-orphans >/dev/null 2>&1 || true; }
trap cleanup EXIT

echo "==> building images"
docker compose build

echo "==> starting server"
docker compose up -d server

echo "==> entering client shell"
docker compose run --rm client "$@"
