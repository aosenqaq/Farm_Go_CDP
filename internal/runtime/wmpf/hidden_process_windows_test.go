//go:build windows

package wmpf

import (
	"os/exec"
	"testing"
)

func TestApplyHiddenWindowConfiguresWindowsProcessAttributes(t *testing.T) {
	cmd := exec.Command("python", "-c", "pass")

	applyHiddenWindow(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("expected process attributes")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("expected hidden window")
	}
	if cmd.SysProcAttr.CreationFlags&windowsCreateNoWindow == 0 {
		t.Fatalf("expected CREATE_NO_WINDOW flag, got %#x", cmd.SysProcAttr.CreationFlags)
	}
}
