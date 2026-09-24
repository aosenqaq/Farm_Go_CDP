package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWebviewDataDirUsesStableApplicationRoot(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "Farm_Go")
	want := filepath.Join(dataDir, "webview2")
	if got := webviewDataDir(dataDir); got != want {
		t.Fatalf("webviewDataDir() = %q, want %q", got, want)
	}
}

func TestAppWindowsOptionsUsesStableWebviewDataDir(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "Farm_Go")
	got := appWindowsOptions(dataDir)
	want := filepath.Join(dataDir, "webview2")
	if got.WebviewUserDataPath != want {
		t.Fatalf("WebviewUserDataPath = %q, want %q", got.WebviewUserDataPath, want)
	}
}

func TestCleanupLegacyWebviewDataDirsRemovesOnlyRecognizedProfiles(t *testing.T) {
	appDataRoot := t.TempDir()
	dataDir := filepath.Join(appDataRoot, "Farm_Go")
	mustMkdirAll(t, filepath.Join(dataDir, "data"))
	mustWriteFile(t, filepath.Join(dataDir, "data", "farm_go.db"))

	removeNames := []string{
		"Farm_Go.exe",
		"Farm_Go_V1.0.4.exe",
		"farm_go-dev.EXE",
		"Farm_Go内测版V1.0.0.exe",
	}
	for _, name := range removeNames {
		mustMkdirAll(t, filepath.Join(appDataRoot, name, "EBWebView", "Default"))
	}

	preserveNames := []string{
		"Farm_Go_V1.0.5.exe",
		"AnotherApp.exe",
	}
	mustMkdirAll(t, filepath.Join(appDataRoot, preserveNames[0], "not-webview-data"))
	mustMkdirAll(t, filepath.Join(appDataRoot, preserveNames[1], "EBWebView"))
	mustWriteFile(t, filepath.Join(appDataRoot, "Farm_Go_file.exe"))

	if cleanupErrors := cleanupLegacyWebviewDataDirs(dataDir, os.RemoveAll); len(cleanupErrors) != 0 {
		t.Fatalf("cleanup errors = %v", cleanupErrors)
	}

	for _, name := range removeNames {
		if _, err := os.Stat(filepath.Join(appDataRoot, name)); !os.IsNotExist(err) {
			t.Errorf("legacy profile %q still exists; stat error = %v", name, err)
		}
	}
	for _, name := range preserveNames {
		if _, err := os.Stat(filepath.Join(appDataRoot, name)); err != nil {
			t.Errorf("protected directory %q was changed: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dataDir, "data", "farm_go.db")); err != nil {
		t.Fatalf("Farm_Go business data was changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDataRoot, "Farm_Go_file.exe")); err != nil {
		t.Fatalf("matching regular file was changed: %v", err)
	}
}

func TestCleanupLegacyWebviewDataDirsContinuesAfterRemovalFailure(t *testing.T) {
	appDataRoot := t.TempDir()
	dataDir := filepath.Join(appDataRoot, "Farm_Go")
	failedName := "Farm_Go_A.exe"
	removedName := "Farm_Go_B.exe"
	for _, name := range []string{failedName, removedName} {
		mustMkdirAll(t, filepath.Join(appDataRoot, name, "EBWebView"))
	}

	removeAll := func(path string) error {
		if filepath.Base(path) == failedName {
			return errors.New("profile is locked")
		}
		return os.RemoveAll(path)
	}
	cleanupErrors := cleanupLegacyWebviewDataDirs(dataDir, removeAll)

	if len(cleanupErrors) != 1 {
		t.Fatalf("cleanup errors = %v, want one error", cleanupErrors)
	}
	if _, err := os.Stat(filepath.Join(appDataRoot, failedName)); err != nil {
		t.Fatalf("failed profile should remain for a later launch: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDataRoot, removedName)); !os.IsNotExist(err) {
		t.Fatalf("later removable profile still exists; stat error = %v", err)
	}
}

func TestCleanupLegacyWebviewDataDirsReportsUnreadableParent(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "missing", "Farm_Go")
	if cleanupErrors := cleanupLegacyWebviewDataDirs(dataDir, os.RemoveAll); len(cleanupErrors) != 1 {
		t.Fatalf("cleanup errors = %v, want one parent read error", cleanupErrors)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
