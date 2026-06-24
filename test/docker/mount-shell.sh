#!/usr/bin/env bash
# Mounts the remote over fme, then drops into an interactive shell (or runs the
# command passed as arguments). fme runs in the background and the mount is torn
# down on exit.
set -uo pipefail

SERVER=${SERVER:-server}
RUSER=${REMOTE_USER:-testuser}
RPATH=${REMOTE_PATH:-/home/testuser/data}
KEY=${KEY:-/keys/id_ed25519}
MNT=${MNT:-/mnt/remote}
LOG=/tmp/fme.log

echo "==> waiting for sshd on ${SERVER}:22"
for _ in $(seq 1 30); do
  if (exec 3<>"/dev/tcp/${SERVER}/22") 2>/dev/null; then exec 3>&-; break; fi
  sleep 1
done

mkdir -p "$MNT"

echo "==> mounting ${RUSER}@${SERVER}:${RPATH} -> ${MNT}"
fme "${RUSER}@${SERVER}:${RPATH}" "$MNT" -i "$KEY" -f >"$LOG" 2>&1 &
FME_PID=$!

for _ in $(seq 1 30); do
  if mountpoint -q "$MNT"; then break; fi
  if ! kill -0 "$FME_PID" 2>/dev/null; then
    echo "!! fme exited before mounting"; cat "$LOG"; exit 1
  fi
  sleep 1
done
if ! mountpoint -q "$MNT"; then
  echo "!! mount timed out"; cat "$LOG"; exit 1
fi

cleanup() {
  echo
  echo "==> unmounting ${MNT}"
  fusermount3 -u "$MNT" 2>/dev/null || fusermount -u "$MNT" 2>/dev/null || kill "$FME_PID" 2>/dev/null
  wait "$FME_PID" 2>/dev/null || true
}
trap cleanup EXIT

echo "==> mounted at ${MNT} (fme log: ${LOG})"
echo "==> exit the shell to unmount and stop the container"
cd "$MNT"
exec "${@:-bash}"
