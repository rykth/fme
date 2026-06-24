package fs

import (
	"syscall"
	"testing"
)

func TestStatusToErrno(t *testing.T) {
	cases := []struct {
		code uint32
		want syscall.Errno
	}{
		{0, 0},
		{sshFxNoSuchFile, syscall.ENOENT},
		{sshFxPermDenied, syscall.EACCES},
		{sshFxFailure, syscall.EPERM},
		{sshFxOpUnsupported, syscall.ENOSYS},
		{sshFxConnectionLost, syscall.ECONNRESET},
		{99, syscall.EIO},
	}
	for _, tc := range cases {
		if got := statusToErrno(tc.code); got != tc.want {
			t.Errorf("statusToErrno(%d) = %v, want %v", tc.code, got, tc.want)
		}
	}
}
