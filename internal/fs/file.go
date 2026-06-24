package fs

import (
	"context"
	"io"
	"os"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/pkg/sftp"
)

// fileHandle represents an open remote file
type fileHandle struct {
	n *Node
	f *sftp.File
}

var (
	_ fs.FileReader   = (*fileHandle)(nil)
	_ fs.FileWriter   = (*fileHandle)(nil)
	_ fs.FileReleaser = (*fileHandle)(nil)
	_ fs.FileFsyncer  = (*fileHandle)(nil)
)

// Open opens an existing file
func (n *Node) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	f, err := n.fs.client.OpenFile(n.remotePath(), openFlags(flags))
	if err != nil {
		return nil, 0, toErrno(err)
	}
	return &fileHandle{n: n, f: f}, fuse.FOPEN_KEEP_CACHE, fs.OK
}

// Create creates a new file and opens it
func (n *Node) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	childPath := n.childPath(name)

	f, err := n.fs.client.OpenFile(childPath, openFlags(flags)|os.O_CREATE)
	if err != nil {
		return nil, nil, 0, toErrno(err)
	}
	if mode != 0 {
		_ = f.Chmod(fuseMode(mode))
	}

	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, 0, toErrno(err)
	}
	n.fs.fillAttr(fi, &out.Attr)

	fh := &fileHandle{n: n, f: f}
	child := n.NewInode(ctx, &Node{fs: n.fs}, fs.StableAttr{Mode: syscall.S_IFREG})
	return child, fh, fuse.FOPEN_KEEP_CACHE, fs.OK
}

// Read reads data from the file at the given offset
func (fh *fileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	n, err := fh.f.ReadAt(dest, off)
	if err != nil && err != io.EOF {
		return nil, toErrno(err)
	}
	return fuse.ReadResultData(dest[:n]), fs.OK
}

// Write writes data to the file synchronously
func (fh *fileHandle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	n, err := fh.f.WriteAt(data, off)
	if err != nil {
		return uint32(n), toErrno(err)
	}
	return uint32(n), fs.OK
}

// Release is called when the last file descriptor is closed
func (fh *fileHandle) Release(ctx context.Context) syscall.Errno {
	if err := fh.f.Close(); err != nil {
		return toErrno(err)
	}
	return fs.OK
}

// Fsync flushes file data via the fsync@openssh.com extension when available
func (fh *fileHandle) Fsync(ctx context.Context, flags uint32) syscall.Errno {
	if _, ok := fh.n.fs.client.HasExtension(extFsync); !ok {
		return syscall.ENOSYS
	}
	if err := fh.f.Sync(); err != nil {
		return toErrno(err)
	}
	return fs.OK
}
