package fs

import (
	"context"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Lookup resolves a child name within this directory node
func (n *Node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	childPath := n.childPath(name)

	fi, err := n.fs.lstatOrStat(childPath)
	if err != nil {
		return nil, toErrno(err)
	}
	n.fs.fillAttr(fi, &out.Attr)

	child := n.NewInode(ctx, &Node{fs: n.fs}, fs.StableAttr{Mode: ifmt(fi)})
	return child, fs.OK
}

// Readdir lists the directory contents
func (n *Node) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	entries, err := n.fs.client.ReadDir(n.remotePath())
	if err != nil {
		return nil, toErrno(err)
	}

	dirEntries := make([]fuse.DirEntry, 0, len(entries))
	for _, fi := range entries {
		name := fi.Name()
		if name == "." || name == ".." {
			continue
		}
		dirEntries = append(dirEntries, fuse.DirEntry{
			Name: name,
			Mode: ifmt(fi),
		})
	}
	return fs.NewListDirStream(dirEntries), fs.OK
}

// Mkdir creates a directory
func (n *Node) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	childPath := n.childPath(name)
	if err := n.fs.client.Mkdir(childPath); err != nil {
		return nil, toErrno(err)
	}
	_ = n.fs.client.Chmod(childPath, fuseMode(mode))

	fi, err := n.fs.client.Stat(childPath)
	if err != nil {
		return nil, toErrno(err)
	}
	n.fs.fillAttr(fi, &out.Attr)

	child := n.NewInode(ctx, &Node{fs: n.fs}, fs.StableAttr{Mode: syscall.S_IFDIR})
	return child, fs.OK
}

// Rmdir removes an empty directory
func (n *Node) Rmdir(ctx context.Context, name string) syscall.Errno {
	childPath := n.childPath(name)
	if err := n.fs.client.RemoveDirectory(childPath); err != nil {
		// SFTP servers report a non-empty directory as a generic failure;
		// translate that to ENOTEMPTY for correct kernel behaviour.
		if e := toErrno(err); e == syscall.EPERM {
			return syscall.ENOTEMPTY
		} else {
			return e
		}
	}
	return fs.OK
}

// Unlink removes a file or symlink
func (n *Node) Unlink(ctx context.Context, name string) syscall.Errno {
	childPath := n.childPath(name)
	if err := n.fs.client.Remove(childPath); err != nil {
		return toErrno(err)
	}
	return fs.OK
}

// Rename moves a file or directory (prefers the POSIX atomic rename extension)
func (n *Node) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	oldPath := n.childPath(name)
	newNode, ok := newParent.(*Node)
	if !ok {
		return syscall.EIO
	}
	newPath := newNode.childPath(newName)

	var err error
	if _, hasExt := n.fs.client.HasExtension(extPosixRename); hasExt {
		err = n.fs.client.PosixRename(oldPath, newPath)
	} else {
		err = n.fs.client.Rename(oldPath, newPath)
	}
	if err != nil {
		return toErrno(err)
	}
	return fs.OK
}
