package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/rykth/fme/internal/mount"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "fme:", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	opts := mount.DefaultOptions()

	var (
		port           int
		identityFiles  []string
		password       string
		knownHostsFile string
		sftpServer     string
		debug          bool
		foreground     bool
		transformLinks bool
		followLinks    bool
		noCheckRoot    bool
		maxRead        int
		idmap          string
		cacheStatTTL   time.Duration
		cacheDirTTL    time.Duration
	)

	cmd := &cobra.Command{
		Use:   "fme [user@]host:[path] mountpoint",
		Short: "Mount a remote directory over SSH",
		Long: `fme mounts a remote directory accessible via SFTP (SSH File Transfer Protocol).

Examples:
  fme user@myserver:/data /mnt/remote
  fme myserver: /mnt/home -p 2222
  fme myserver:/var/log /mnt/logs --follow-symlinks`,
		Version:       "0.1.0",
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := mount.ParseRemote(args[0], &opts); err != nil {
				return err
			}
			opts.MountPoint = args[1]

			// SSH options
			opts.Conn.Port = port
			opts.Conn.IdentityFiles = identityFiles
			opts.Conn.Password = password
			opts.Conn.KnownHostsFile = knownHostsFile
			opts.Conn.SFTPServerPath = sftpServer

			// use default user to current OS user if not specified
			if opts.Conn.User == "" {
				if u, err := user.Current(); err == nil {
					opts.Conn.User = u.Username
				}
			}

			// filesystem options
			opts.FS.TransformSymlinks = transformLinks
			opts.FS.FollowSymlinks = followLinks
			opts.FS.NoCheckRoot = noCheckRoot
			if maxRead > 0 {
				opts.FS.MaxRead = maxRead
			}
			// kernel-side metadata cache timeouts
			if cacheStatTTL > 0 {
				opts.AttrTimeout = cacheStatTTL
			}
			if cacheDirTTL > 0 {
				opts.EntryTimeout = cacheDirTTL
			}

			switch strings.ToLower(idmap) {
			case "none", "":
				// default
			case "user":
				opts.FS.IDMap = 1 // IDMapUser
			case "file":
				opts.FS.IDMap = 2 // IDMapFile
			default:
				return fmt.Errorf("unknown idmap mode %q (use none, user, or file)", idmap)
			}

			opts.Debug = debug

			// set up structured logging
			level := slog.LevelInfo
			if debug {
				level = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

			return mount.Mount(context.Background(), opts)
		},
	}

	f := cmd.Flags()
	f.IntVarP(&port, "port", "p", 22, "SSH port")
	f.StringArrayVarP(&identityFiles, "identity", "i", nil, "Identity file(s)")
	f.StringVar(&password, "password", "", "SSH password (prefer key auth)")
	f.StringVar(&knownHostsFile, "known-hosts", "", "Known hosts file (default ~/.ssh/known_hosts)")
	f.StringVar(&sftpServer, "sftp-server", "", "Path to sftp-server on remote (default: use sftp subsystem)")
	f.BoolVarP(&debug, "debug", "d", false, "Enable debug output and run in foreground")
	f.BoolVarP(&foreground, "foreground", "f", false, "Run in foreground")
	f.BoolVar(&transformLinks, "transform-symlinks", false, "Rewrite absolute symlinks inside the mount")
	f.BoolVar(&followLinks, "follow-symlinks", false, "Follow symlinks on stat")
	f.BoolVar(&noCheckRoot, "no-check-root", false, "Skip root directory check on mount")
	f.IntVar(&maxRead, "max-read", 0, "Maximum SFTP packet size (bytes, default 65536)")
	f.StringVar(&idmap, "idmap", "none", "UID/GID mapping mode: none, user, or file")
	f.DurationVar(&cacheStatTTL, "cache-stat-timeout", 0, "Stat cache TTL (default 20s)")
	f.DurationVar(&cacheDirTTL, "cache-dir-timeout", 0, "Directory cache TTL (default 20s)")

	return cmd
}
