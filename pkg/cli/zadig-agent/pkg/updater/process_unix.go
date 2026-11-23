//go:build linux || darwin

package updater

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

func tryLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, errLocked
		}
		return nil, err
	}
	return f, nil
}

func syncDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	err = f.Sync()
	if errors.Is(err, unix.EINVAL) {
		return nil
	}
	return err
}

func replaceFile(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return err
	}
	if err := syncDirectory(filepath.Dir(to)); err != nil {
		return err
	}
	if filepath.Dir(from) != filepath.Dir(to) {
		return syncDirectory(filepath.Dir(from))
	}
	return nil
}

func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

func diskFree(path string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}

func managedService() bool {
	return os.Getenv("INVOCATION_ID") != "" || os.Getenv("NOTIFY_SOCKET") != "" || os.Getenv("LAUNCH_JOBKEY_LABEL") != ""
}
