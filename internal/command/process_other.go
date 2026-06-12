//go:build !unix

package command

import "os/exec"

// OpenTunnel's command cancellation relies on Unix process groups. Release
// targets are linux and darwin; non-unix builds fail instead of silently
// degrading child-process cleanup.
var _ = unsupportedNonUnixOpenTunnelBuild

func configureCommandCleanup(cmd *exec.Cmd) {
	_ = cmd
}
