//go:build !unix

package command

import "os/exec"

func configureCommandCleanup(cmd *exec.Cmd) {
	// Process-group cleanup is unsupported by this fallback for M3.
	// exec.CommandContext still terminates the direct child process.
}
