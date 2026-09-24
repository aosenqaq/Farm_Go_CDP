//go:build windows

package wmpf

import (
	"os/exec"
	"syscall"
)

const windowsCreateNoWindow uint32 = 0x08000000

func applyHiddenWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= windowsCreateNoWindow
}
