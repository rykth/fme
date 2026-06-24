package fs

import (
	"context"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Statfs reports filesystem statistics via the statvfs@openssh.com extension
func (n *Node) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	if _, ok := n.fs.client.HasExtension(extStatvfs); !ok {
		// extension not available: return plausible fake values
		out.Blocks = 1 << 30 // 1 TB in 1K blocks
		out.Bfree = 1 << 29
		out.Bavail = 1 << 29
		out.Files = 1 << 20
		out.Ffree = 1 << 19
		out.Bsize = 4096
		out.NameLen = 255
		out.Frsize = 4096
		return fs.OK
	}

	r, err := n.fs.client.StatVFS(n.fs.opts.BasePath)
	if err != nil {
		return toErrno(err)
	}

	out.Blocks = r.Blocks
	out.Bfree = r.Bfree
	out.Bavail = r.Bavail
	out.Files = r.Files
	out.Ffree = r.Ffree
	out.Bsize = uint32(r.Bsize)
	out.NameLen = uint32(r.Namemax)
	out.Frsize = uint32(r.Frsize)
	return fs.OK
}

// Link creates a hard link via the hardlink@openssh.com extension
func (n *Node) Link(ctx context.Context, target fs.InodeEmbedder, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if _, ok := n.fs.client.HasExtension(extHardlink); !ok {
		return nil, syscall.ENOSYS
	}
	targetNode, ok := target.(*Node)
	if !ok {
		return nil, syscall.EIO
	}
	linkPath := n.childPath(name)
	if err := n.fs.client.Link(targetNode.remotePath(), linkPath); err != nil {
		return nil, toErrno(err)
	}

	fi, err := n.fs.client.Lstat(linkPath)
	if err != nil {
		return nil, toErrno(err)
	}
	n.fs.fillAttr(fi, &out.Attr)

	child := n.NewInode(ctx, &Node{fs: n.fs}, fs.StableAttr{Mode: ifmt(fi)})
	return child, fs.OK
}
