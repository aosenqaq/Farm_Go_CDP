package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInstallBundledFridaPythonProvidesRuntimeExecutable(t *testing.T) {
	pythonPath, err := installBundledFridaPython(t.TempDir())
	if err != nil {
		t.Fatalf("install bundled Frida Python: %v", err)
	}
	if !filepath.IsAbs(pythonPath) {
		t.Fatalf("expected an absolute Python path, got %q", pythonPath)
	}
	info, err := os.Stat(pythonPath)
	if err != nil {
		t.Fatalf("stat bundled Python: %v", err)
	}
	if info.IsDir() || info.Size() == 0 {
		t.Fatalf("expected a non-empty Python executable, info=%v", info)
	}
	output, err := exec.Command(pythonPath, "-c", "import frida; print(frida.__version__)").CombinedOutput()
	if err != nil {
		t.Fatalf("load bundled Frida: %v\n%s", err, output)
	}
}
