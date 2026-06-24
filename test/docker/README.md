# Docker interactive mount

Spins up two containers and drops you into a shell on a live `fme` mount:

- **server** - Debian + OpenSSH with the `sftp` subsystem, a `testuser` (key auth),
  and a seeded `/home/testuser/data` tree.
- **client** - builds `fme` from source, mounts `testuser@server:/home/testuser/data`
  over SSH/FUSE at `/mnt/remote`, and gives you an interactive shell there.

## Run

```sh
./test/docker/shell.sh        # or: make docker-shell
```

This generates an ephemeral keypair (`test/docker/.keys/`, gitignored), builds both
images, starts the server, and drops you into a bash shell on the client with the
remote mounted at `/mnt/remote` (your working directory):

```
testuser@client:/mnt/remote$ ls -la
testuser@client:/mnt/remote$ cat hello.txt
testuser@client:/mnt/remote$ echo hi > newfile && cat newfile
testuser@client:/mnt/remote$ tail -f /tmp/fme.log    # watch SFTP traffic
```

Exit the shell (`exit` / Ctrl-D) to unmount and tear everything down. Pass a command
to run it non-interactively instead:

```sh
./test/docker/shell.sh ls -l /mnt/remote
./test/docker/shell.sh df -h /mnt/remote
```

## Requirements

- Docker with `docker compose` (v2).
- The host `fuse` kernel module loaded (`/dev/fuse` present).
- The client runs `privileged` so FUSE works regardless of host AppArmor/seccomp.
  See the comment in `docker-compose.yml` to tighten this to `/dev/fuse` + `SYS_ADMIN`.

## Layout

| File | Purpose |
|---|---|
| `shell.sh` | Orchestrates keygen → build → start server → client shell → teardown. |
| `mount-shell.sh` | Client entrypoint: mounts the remote, then execs your shell/command. |
| `docker-compose.yml` | Wires the two services and the shared key volume. |
| `Dockerfile.server` / `entrypoint-server.sh` | SSH server + seeded data. |
| `Dockerfile.client` | Builds `fme`. |
