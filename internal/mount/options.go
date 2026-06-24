package mount

import (
	"fmt"
	"strings"
	"time"

	"github.com/rykth/fme/internal/conn"
	gfs "github.com/rykth/fme/internal/fs"
)

// Options holds all parsed parameters for mounting
type Options struct {
	// ssh
	Conn conn.Config

	// remote
	RemotePath string

	// local
	MountPoint string

	// behaviour
	FS    gfs.Options
	Debug bool

	// kernel-side metadata cache timeouts.
	AttrTimeout  time.Duration
	EntryTimeout time.Duration
}

// ParseRemote splits "[user@]host:[path]"
func ParseRemote(remote string, opts *Options) error {
	// split user@host from path
	hostPart, path, ok := strings.Cut(remote, ":")
	if !ok {
		return fmt.Errorf("remote must be in [user@]host:[path] form, got %q", remote)
	}
	if path == "" {
		path = "."
	}

	// extract optional user prefix
	if user, host, hasAt := strings.Cut(hostPart, "@"); hasAt {
		opts.Conn.User = user
		opts.Conn.Host = host
	} else {
		opts.Conn.Host = hostPart
	}
	opts.RemotePath = path
	return nil
}

func DefaultOptions() Options {
	return Options{
		Conn: conn.Config{
			Port:    22,
			Timeout: 30 * time.Second,
		},
		FS: gfs.Options{
			MaxRead: 1 << 16, // 64KB
		},
		AttrTimeout:  20 * time.Second,
		EntryTimeout: 20 * time.Second,
	}
}
