package fs

import (
	"errors"
	"io"
	"os"
	"syscall"

	"github.com/pkg/sftp"
)

// SFTP status codes
const (
	sshFxEOF            = 1
	sshFxNoSuchFile     = 2
	sshFxPermDenied     = 3
	sshFxFailure        = 4
	sshFxBadMessage     = 5
	sshFxNoConnection   = 6
	sshFxConnectionLost = 7
	sshFxOpUnsupported  = 8
)

// toErrno converts an arbitrary error to a syscall.Errno suitable for FUSE
func toErrno(err error) syscall.Errno {
	if err == nil {
		return 0
	}
	if errors.Is(err, io.EOF) {
		return syscall.ENODATA
	}
	var se *sftp.StatusError
	if errors.As(err, &se) {
		return statusToErrno(se.Code)
	}
	if errors.Is(err, os.ErrNotExist) {
		return syscall.ENOENT
	}
	if errors.Is(err, os.ErrPermission) {
		return syscall.EACCES
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno
	}
	return syscall.EIO
}

// statusToErrno maps an SFTP status code to a syscall.Errno
func statusToErrno(code uint32) syscall.Errno {
	switch code {
	case 0:
		return 0
	case sshFxEOF:
		return syscall.ENODATA
	case sshFxNoSuchFile:
		return syscall.ENOENT
	case sshFxPermDenied:
		return syscall.EACCES
	case sshFxFailure:
		return syscall.EPERM
	case sshFxBadMessage:
		return syscall.EIO
	case sshFxNoConnection, sshFxConnectionLost:
		return syscall.ECONNRESET
	case sshFxOpUnsupported:
		return syscall.ENOSYS
	default:
		return syscall.EIO
	}
}
