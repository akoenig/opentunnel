//go:build !unix

package command

import "os/exec"

func configureCommandCleanup(cmd *exec.Cmd) {
}
