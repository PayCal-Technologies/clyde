//go:build windows

package clyde

import "os/exec"

func configureMCPCommand(cmd *exec.Cmd) {}

func killMCPProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
