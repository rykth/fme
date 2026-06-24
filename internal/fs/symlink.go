package fs

import (
	"context"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Symlink creates a symbolic link name -> target
func (n *Node) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	linkPath := n.childPath(name)
	if err := n.fs.client.Symlink(target, linkPath); err != nil {
		return nil, toErrno(err)
	}

	fi, err := n.fs.client.Lstat(linkPath)
	if err != nil {
		return nil, toErrno(err)
	}
	n.fs.fillAttr(fi, &out.Attr)

	child := n.NewInode(ctx, &Node{fs: n.fs}, fs.StableAttr{Mode: syscall.S_IFLNK})
	return child, fs.OK
}

// Readlink returns the target of a symbolic link
func (n *Node) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	target, err := n.fs.client.ReadLink(n.remotePath())
	if err != nil {
		return nil, toErrno(err)
	}

	if n.fs.opts.TransformSymlinks {
		target = n.transformSymlink(target)
	}
	return []byte(target), fs.OK
}

// transformSymlink rewrites absolute symlinks that point inside the remote
// base path into relative paths from the symlinks location. This prevents
// broken paths when the mount point differs from the remote base
func (n *Node) transformSymlink(target string) string {
	if !filepath.IsAbs(target) {
		return target
	}
	base := n.fs.opts.BasePath
	if !strings.HasPrefix(target, base) {
		return target
	}

	// Compute relative path from symlink's directory to target.
	rel, err := filepath.Rel(filepath.Dir(n.remotePath()), target)
	if err != nil {
		return target
	}
	return rel
}
