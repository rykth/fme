package mount

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/pkg/sftp"
	"github.com/rykth/fme/internal/conn"
	gfs "github.com/rykth/fme/internal/fs"
)

// Mount establishes the SSH/SFTP connection, mounts the FUSE filesystem,
// and blocks until the mount is unmounted or a signal is received
func Mount(ctx context.Context, opts Options) error {
	slog.Info("connecting", "host", opts.Conn.Host, "port", opts.Conn.Port, "user", opts.Conn.User)

	c, err := conn.Dial(ctx, opts.Conn)
	if err != nil {
		return fmt.Errorf("ssh connect: %w", err)
	}
	defer c.Close()

	r, w := c.ReadWriter()

	clientOpts := []sftp.ClientOption{
		sftp.UseConcurrentReads(true),
		sftp.UseConcurrentWrites(true),
		sftp.MaxConcurrentRequestsPerFile(64),
	}
	if opts.FS.MaxRead > 0 {
		clientOpts = append(clientOpts, sftp.MaxPacketUnchecked(opts.FS.MaxRead))
	}
	client, err := sftp.NewClientPipe(r, w, clientOpts...)
	if err != nil {
		return fmt.Errorf("sftp init: %w", err)
	}
	defer client.Close()

	// resolve the remote base path to its canonical form
	basePath, err := client.RealPath(opts.RemotePath)
	if err != nil {
		return fmt.Errorf("realpath %q: %w", opts.RemotePath, err)
	}
	slog.Info("remote base path", "path", basePath)

	if !opts.FS.NoCheckRoot {
		if _, err := client.Stat(basePath); err != nil {
			return fmt.Errorf("check root %q: %w", basePath, err)
		}
	}

	fsOpts := opts.FS
	fsOpts.BasePath = basePath

	root, err := gfs.NewFS(client, fsOpts)
	if err != nil {
		return fmt.Errorf("create filesystem: %w", err)
	}

	attrTimeout := opts.AttrTimeout
	entryTimeout := opts.EntryTimeout
	fuseOpts := &fs.Options{
		AttrTimeout:  &attrTimeout,
		EntryTimeout: &entryTimeout,
		MountOptions: fuse.MountOptions{
			AllowOther:   false,
			Debug:        opts.Debug,
			FsName:       opts.Conn.Host + ":" + opts.RemotePath,
			Name:         "fme",
			DirectMount:  false,
			MaxReadAhead: opts.FS.MaxRead,
		},
	}

	server, err := fs.Mount(opts.MountPoint, root, fuseOpts)
	if err != nil {
		return fmt.Errorf("fuse mount: %w", err)
	}

	slog.Info("mounted", "mountpoint", opts.MountPoint)

	// Handle signals for clean unmount.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	go func() {
		sig := <-sigCh
		slog.Info("signal received, unmounting", "signal", sig)
		if err := server.Unmount(); err != nil {
			slog.Error("unmount failed", "err", err)
			os.Exit(1)
		}
	}()

	server.Wait()
	slog.Info("unmounted")
	return nil
}
