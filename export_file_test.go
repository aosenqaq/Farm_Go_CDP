package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeDefaultFileNameReplacesInvalidCharacters(t *testing.T) {
	name := safeDefaultFileName(`bad<name>:farm?.txt`)

	if name != "bad_name__farm_.txt" {
		t.Fatalf("safe name = %q", name)
	}
}

func TestSaveTextFileToPathAppendsDefaultExtension(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "farm-export")

	result, err := saveTextFileToPath(path, "10001\n10002", "txt")

	if err != nil {
		t.Fatalf("save text file: %v", err)
	}
	if filepath.Base(result.Path) != "farm-export.txt" {
		t.Fatalf("path = %q", result.Path)
	}
	raw, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if string(raw) != "10001\n10002" {
		t.Fatalf("content = %q", string(raw))
	}
}

func TestSaveTextFileToPathKeepsExistingExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "farm-export.log")

	result, err := saveTextFileToPath(path, "hello", "txt")

	if err != nil {
		t.Fatalf("save text file: %v", err)
	}
	if !strings.HasSuffix(result.Path, "farm-export.log") {
		t.Fatalf("path = %q", result.Path)
	}
}
