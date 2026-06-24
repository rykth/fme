package fs

import (
	"log/slog"
	"os"
	"os/user"
	"path"
	"strconv"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/pkg/sftp"
)

// OpenSSH SFTP extension names we probe via client.HasExtension
const (
	extPosixRename = "posix-rename@openssh.com"
	extStatvfs     = "statvfs@openssh.com"
	extHardlink    = "hardlink@openssh.com"
	extFsync       = "fsync@openssh.com"
)

// IDMapMode controls how remote UID/GID values are mapped to local ones
type IDMapMode int

const (
	IDMapNone IDMapMode = iota
	IDMapUser           // map remote owner UID/GID to local UID/GID
	IDMapFile           // explicit mapping from files
)

// Options configures the FUSE filesystem behaviour
type Options struct {
	BasePath          string
	MaxRead           int
	IDMap             IDMapMode
	UIDMap            map[uint32]uint32
	GIDMap            map[uint32]uint32
	ReverseUIDMap     map[uint32]uint32
	ReverseGIDMap     map[uint32]uint32
	TransformSymlinks bool
	FollowSymlinks    bool
	NoCheckRoot       bool
}

// FS holds shared state for the filesystem
type FS struct {
	client *sftp.Client
	opts   Options

	remoteUID uint32
	remoteGID uint32
	localUID  uint32
	localGID  uint32
}

// NewFS creates an FS and returns the root node ready for mounting
func NewFS(client *sftp.Client, opts Options) (*Node, error) {
	f := &FS{client: client, opts: opts}

	if opts.IDMap == IDMapUser {
		if err := f.detectRemoteUID(); err != nil {
			slog.Warn("could not detect remote uid", "err", err)
		}
	}

	root := &Node{fs: f}
	return root, nil
}

func (f *FS) detectRemoteUID() error {
	fi, err := f.client.Stat(f.opts.BasePath)
	if err != nil {
		return err
	}
	if st, ok := fi.Sys().(*sftp.FileStat); ok {
		f.remoteUID = st.UID
		f.remoteGID = st.GID
	}

	u, err := user.Current()
	if err != nil {
		return err
	}
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return err
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return err
	}
	f.localUID = uint32(uid)
	f.localGID = uint32(gid)
	slog.Debug("idmap user", "remote_uid", f.remoteUID, "local_uid", f.localUID)
	return nil
}

// mapRemoteToLocal translates UID/GID from remote to local (read path)
func (f *FS) mapRemoteToLocal(uid, gid *uint32) {
	switch f.opts.IDMap {
	case IDMapUser:
		if *uid == f.remoteUID {
			*uid = f.localUID
		}
		if *gid == f.remoteGID {
			*gid = f.localGID
		}
	case IDMapFile:
		if mapped, ok := f.opts.UIDMap[*uid]; ok {
			*uid = mapped
		}
		if mapped, ok := f.opts.GIDMap[*gid]; ok {
			*gid = mapped
		}
	}
}

// mapLocalToRemote translates local UID/GID to remote before setattr calls
func (f *FS) mapLocalToRemote(uid, gid *uint32) {
	switch f.opts.IDMap {
	case IDMapUser:
		if *uid == f.localUID {
			*uid = f.remoteUID
		}
		if *gid == f.localGID {
			*gid = f.remoteGID
		}
	case IDMapFile:
		if mapped, ok := f.opts.ReverseUIDMap[*uid]; ok {
			*uid = mapped
		}
		if mapped, ok := f.opts.ReverseGIDMap[*gid]; ok {
			*gid = mapped
		}
	}
}

// toStat converts a pkg/sftp os.FileInfo to a syscall.Stat_t
func toStat(fi os.FileInfo) syscall.Stat_t {
	var st syscall.Stat_t
	if s, ok := fi.Sys().(*sftp.FileStat); ok {
		st.Size = int64(s.Size)
		st.Mode = s.Mode
		st.Uid = s.UID
		st.Gid = s.GID
		st.Atim = syscall.Timespec{Sec: int64(s.Atime)}
		st.Mtim = syscall.Timespec{Sec: int64(s.Mtime)}
	} else {
		st.Size = fi.Size()
		st.Mtim = syscall.Timespec{Sec: fi.ModTime().Unix()}
		st.Mode = uint32(fi.Mode().Perm())
		if fi.IsDir() {
			st.Mode |= syscall.S_IFDIR
		} else {
			st.Mode |= syscall.S_IFREG
		}
	}
	st.Blksize = 512
	if st.Size > 0 {
		st.Blocks = (st.Size + 511) / 512
	}
	return st
}

// fillAttr converts a remote FileInfo into a fuse.Attr, applying idmap
func (f *FS) fillAttr(fi os.FileInfo, out *fuse.Attr) {
	st := toStat(fi)
	f.mapRemoteToLocal(&st.Uid, &st.Gid)
	out.FromStat(&st)
}

// ifmt returns the file-type bits (S_IFMT) for a remote FileInfo, defaulting
// to a regular file when the server omits the type
func ifmt(fi os.FileInfo) uint32 {
	m := toStat(fi).Mode & syscall.S_IFMT
	if m == 0 {
		m = syscall.S_IFREG
	}
	return m
}

// fuseMode converts a raw unix permission word (including setuid/setgid/sticky)
// into an os.FileMode for chmod
func fuseMode(mode uint32) os.FileMode {
	m := os.FileMode(mode & 0o777)
	if mode&syscall.S_ISUID != 0 {
		m |= os.ModeSetuid
	}
	if mode&syscall.S_ISGID != 0 {
		m |= os.ModeSetgid
	}
	if mode&syscall.S_ISVTX != 0 {
		m |= os.ModeSticky
	}
	return m
}

// openFlags maps FUSE/POSIX open flags to os.O_* flags for client.OpenFile
func openFlags(flags uint32) int {
	var o int
	switch flags & syscall.O_ACCMODE {
	case syscall.O_WRONLY:
		o = os.O_WRONLY
	case syscall.O_RDWR:
		o = os.O_RDWR
	default:
		o = os.O_RDONLY
	}
	if flags&syscall.O_APPEND != 0 {
		o |= os.O_APPEND
	}
	if flags&syscall.O_CREAT != 0 {
		o |= os.O_CREATE
	}
	if flags&syscall.O_TRUNC != 0 {
		o |= os.O_TRUNC
	}
	if flags&syscall.O_EXCL != 0 {
		o |= os.O_EXCL
	}
	return o
}

// lstatOrStat calls Lstat or Stat depending on the follow_symlinks option
func (f *FS) lstatOrStat(path string) (os.FileInfo, error) {
	if f.opts.FollowSymlinks {
		return f.client.Stat(path)
	}
	return f.client.Lstat(path)
}

// Node is a single inode in the remote filesystem tree
type Node struct {
	fs.Inode
	fs *FS
}

// remotePath returns the nodes current remote path, derived from its live
// position in the inode tree. Deriving it (rather than storing it) keeps the
// path correct across renames, where the kernel moves a node to a new name
func (n *Node) remotePath() string {
	return path.Join(n.fs.opts.BasePath, n.Path(nil))
}

func (n *Node) childPath(name string) string {
	return path.Join(n.remotePath(), name)
}

var (
	_ fs.NodeGetattrer  = (*Node)(nil)
	_ fs.NodeSetattrer  = (*Node)(nil)
	_ fs.NodeLookuper   = (*Node)(nil)
	_ fs.NodeReaddirer  = (*Node)(nil)
	_ fs.NodeMkdirer    = (*Node)(nil)
	_ fs.NodeRmdirer    = (*Node)(nil)
	_ fs.NodeUnlinker   = (*Node)(nil)
	_ fs.NodeRenamer    = (*Node)(nil)
	_ fs.NodeCreater    = (*Node)(nil)
	_ fs.NodeOpener     = (*Node)(nil)
	_ fs.NodeSymlinker  = (*Node)(nil)
	_ fs.NodeReadlinker = (*Node)(nil)
	_ fs.NodeLinker     = (*Node)(nil)
	_ fs.NodeStatfser   = (*Node)(nil)
)
