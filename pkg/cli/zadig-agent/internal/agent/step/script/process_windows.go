//go:build windows

package script

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

func configureCancellation(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		kill := filepath.Join(os.Getenv("SystemRoot"), "System32", "taskkill.exe")
		if err := exec.CommandContext(ctx, kill, "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").Run(); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}
