//go:build !windows

package wmpf

import "os/exec"

func applyHiddenWindow(cmd *exec.Cmd) {}
