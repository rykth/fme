#!/bin/sh
set -e

USER=testuser
HOME_DIR=/home/$USER

# Create the user (idempotent) with a password so PAM account checks pass even
# though we authenticate with a key.
useradd -m -u 1000 -s /bin/bash "$USER" 2>/dev/null || true
echo "$USER:testpass" | chpasswd

# Install the authorized key shared via the read-only /keys volume.
mkdir -p "$HOME_DIR/.ssh"
cp /keys/id_ed25519.pub "$HOME_DIR/.ssh/authorized_keys"
chmod 700 "$HOME_DIR/.ssh"
chmod 600 "$HOME_DIR/.ssh/authorized_keys"
chown -R "$USER:$USER" "$HOME_DIR/.ssh"

# Seed a small tree to mount.
mkdir -p "$HOME_DIR/data/subdir"
printf 'hello world' > "$HOME_DIR/data/hello.txt"
printf 'in subdir\n' > "$HOME_DIR/data/subdir/note.txt"
chown -R "$USER:$USER" "$HOME_DIR/data"

# Generate host keys and start sshd in the foreground with logging to stderr.
ssh-keygen -A
echo "sshd starting; sftp subsystem ready" >&2
exec /usr/sbin/sshd -D -e
