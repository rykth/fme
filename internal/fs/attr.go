package fs

import (
	"context"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Getattr returns file attributes, preferring fstat on an open handle
func (n *Node) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	if fh != nil {
		if h, ok := fh.(*fileHandle); ok {
			fi, err := h.f.Stat()
			if err != nil {
				return toErrno(err)
			}
			n.fs.fillAttr(fi, &out.Attr)
			return fs.OK
		}
	}

	fi, err := n.fs.lstatOrStat(n.remotePath())
	if err != nil {
		return toErrno(err)
	}
	n.fs.fillAttr(fi, &out.Attr)
	return fs.OK
}

// Setattr handles chmod, chown, utimes, and truncate
func (n *Node) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	var h *fileHandle
	if fh != nil {
		if f, ok := fh.(*fileHandle); ok {
			h = f
		}
	}

	var err error

	if in.Valid&fuse.FATTR_MODE != 0 {
		mode := fuseMode(in.Mode)
		if h != nil {
			err = h.f.Chmod(mode)
		} else {
			err = n.fs.client.Chmod(n.remotePath(), mode)
		}
	}

	if err == nil && in.Valid&(fuse.FATTR_UID|fuse.FATTR_GID) != 0 {
		uid, gid := in.Uid, in.Gid
		n.fs.mapLocalToRemote(&uid, &gid)
		if h != nil {
			err = h.f.Chown(int(uid), int(gid))
		} else {
			err = n.fs.client.Chown(n.remotePath(), int(uid), int(gid))
		}
	}

	if err == nil && in.Valid&(fuse.FATTR_ATIME|fuse.FATTR_MTIME) != 0 {
		atime := time.Unix(int64(in.Atime), int64(in.Atimensec))
		mtime := time.Unix(int64(in.Mtime), int64(in.Mtimensec))
		// pkg/sftp's *File has no Chtimes so always go through the path
		err = n.fs.client.Chtimes(n.remotePath(), atime, mtime)
	}

	if err == nil && in.Valid&fuse.FATTR_SIZE != 0 {
		if h != nil {
			err = h.f.Truncate(int64(in.Size))
		} else {
			err = n.fs.client.Truncate(n.remotePath(), int64(in.Size))
		}
	}

	if err != nil {
		return toErrno(err)
	}
	return n.Getattr(ctx, fh, out)
}
