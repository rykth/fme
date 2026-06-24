# fme

Mount a remote directory over SSH and use it like a local filesystem.

`fme` is a FUSE filesystem that talks to a remote host over SFTP (the SSH File
Transfer Protocol). Point it at any server you can `ssh` into and its files show
up under a local mountpoint (read, write, list, rename, symlink, and so on), all
tunnelled over your existing SSH connection. No agent or special software is
required on the remote, the standard OpenSSH `sftp` subsystem is enough.

It is inspired by [sshfs](https://github.com/libfuse/sshfs) and implemented in 
Go on top of [`go-fuse`](https://github.com/hanwen/go-fuse) and
[`pkg/sftp`](https://github.com/pkg/sftp).

## Features

- **Plain SSH transport** - uses the remote's `sftp` subsystem (or an explicit
  `sftp-server` binary). Nothing to install server-side.
- **Flexible auth** - SSH agent, identity files, or password, with `known_hosts`
  host-key verification.
- **Full read/write** - open, read, write, create, truncate, `mkdir`, `rmdir`,
  `unlink`, `rename`, `chmod`, `chown`, and timestamps.
- **Symlinks & hard links** - create and read symlinks, optionally rewriting
  absolute symlink targets to stay inside the mount, plus hard links where the
  server supports them.
- **UID/GID mapping** - remap remote ownership to your local user so files don't
  all appear owned by someone else.
- **Kernel-side metadata caching** - attribute and directory-entry lookups are
  cached by the kernel for a configurable TTL to cut down on round-trips.
- **OpenSSH extensions** - uses `posix-rename`, `statvfs`, `hardlink`, and
  `fsync` when the server advertises them.

## Requirements

- A working FUSE setup:
  - **Linux** - `fuse3` (`fusermount3`) and the `fuse` kernel module.
  - **macOS** - [macFUSE](https://osxfuse.github.io/).
- SSH access to the remote host with the `sftp` subsystem enabled (the default
  for OpenSSH).

## Install

```sh
go install github.com/rykth/fme/cmd/fme@latest
```

Or build from source:

```sh
git clone https://github.com/rykth/fme
cd fme
make build        # produces ./fme
```

## Usage

```
fme [user@]host:[path] mountpoint [flags]
```

```sh
# Mount a remote data directory
fme user@myserver:/data /mnt/remote

# Mount your remote home (empty path defaults to the login directory)
fme myserver: /mnt/home -p 2222

# Follow symlinks when stat-ing, with a specific key
fme myserver:/var/log /mnt/logs --follow-symlinks -i ~/.ssh/id_ed25519
```

Unmount when you're done:

```sh
fusermount3 -u /mnt/remote     # Linux
umount /mnt/remote             # macOS
```

### Flags

| Flag | Description |
|---|---|
| `-p, --port` | SSH port (default `22`). |
| `-i, --identity` | Identity (private key) file; may be repeated. |
| `--password` | SSH password (prefer key auth). |
| `--known-hosts` | Known-hosts file (default `~/.ssh/known_hosts`). |
| `--sftp-server` | Path to `sftp-server` on the remote (default: use the `sftp` subsystem). |
| `--transform-symlinks` | Rewrite absolute symlinks so they resolve inside the mount. |
| `--follow-symlinks` | Follow symlinks when stat-ing. |
| `--no-check-root` | Skip the root-directory check at mount time. |
| `--idmap` | UID/GID mapping mode: `none`, `user`, or `file`. |
| `--max-read` | Maximum SFTP packet size in bytes (default `65536`). |
| `--cache-stat-timeout` | Attribute cache TTL (default `20s`). |
| `--cache-dir-timeout` | Directory-entry cache TTL (default `20s`). |
| `-d, --debug` | Enable debug logging. |
| `-f, --foreground` | Run in the foreground. |

## Try it with Docker

If you don't have a remote handy, the `test/docker` harness spins up a
self-contained SSH server and a client that mounts it - no host SSH setup
required. It drops you into a shell sitting on a live `fme` mount.

```sh
./test/docker/shell.sh      # or: make docker-shell
```

This generates an ephemeral SSH keypair, builds both container images, starts the
server, and mounts `testuser@server:/home/testuser/data` at `/mnt/remote` on the
client - your working directory inside the shell:

```
testuser@client:/mnt/remote$ ls -la
testuser@client:/mnt/remote$ cat hello.txt
testuser@client:/mnt/remote$ echo hi > newfile && cat newfile
testuser@client:/mnt/remote$ mkdir d && mv newfile d/ && ls d
testuser@client:/mnt/remote$ tail -f /tmp/fme.log    # watch SFTP traffic
```

Exit the shell (`exit` / Ctrl-D) to unmount and tear everything down. You can also
run a one-off command instead of an interactive shell:

```sh
./test/docker/shell.sh ls -l /mnt/remote
./test/docker/shell.sh df -h /mnt/remote
```

Requirements: Docker with `docker compose` (v2), and the host `fuse` module
loaded (`/dev/fuse` present - the client runs `privileged` so FUSE works inside
the container). See [`test/docker/README.md`](test/docker/README.md) for details.

## Development

```sh
make build          # build ./fme
make test           # run unit tests
make vet            # go vet
make lint           # golangci-lint (if installed)
make bench          # benchmarks
make docker-shell   # spin up an SSH server + client and drop into a live mount
```

`make docker-shell` is the quickest way to exercise the filesystem end to end 
(see [Try it with Docker](#try-it-with-docker) above).
