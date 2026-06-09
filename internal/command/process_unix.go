//go:build unix

package command

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

const processCleanupGracePeriod = 100 * time.Millisecond

func configureCommandCleanup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		processGroupID := -cmd.Process.Pid
		if err := syscall.Kill(processGroupID, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		time.Sleep(processCleanupGracePeriod)
		if err := syscall.Kill(processGroupID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		return nil
	}
}
