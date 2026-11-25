package daemon

import (
	"golang.org/x/sys/unix"
	"net"
	"os"
	"syscall"
)

func peerUID(c *net.UnixConn) (int, error) {
	raw, err := c.SyscallConn()
	if err != nil {
		return -1, err
	}
	var uid int
	var credentialErr error
	err = raw.Control(func(fd uintptr) {
		cred, err := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		credentialErr = err
		if err == nil {
			uid = int(cred.Uid)
		}
	})
	if err != nil {
		return -1, err
	}
	return uid, credentialErr
}
func rootOwned(info os.FileInfo) bool { s, ok := info.Sys().(*syscall.Stat_t); return ok && s.Uid == 0 }
func lockBroker(f *os.File) error     { return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) }
