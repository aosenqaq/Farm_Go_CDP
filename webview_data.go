package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

func webviewDataDir(dataDir string) string {
	return filepath.Join(dataDir, "webview2")
}

func appWindowsOptions(dataDir string) *windows.Options {
	return &windows.Options{WebviewUserDataPath: webviewDataDir(dataDir)}
}

func cleanupLegacyWebviewDataDirs(dataDir string, removeAll func(string) error) []error {
	parent := filepath.Dir(filepath.Clean(dataDir))
	entries, err := os.ReadDir(parent)
	if err != nil {
		return []error{fmt.Errorf("read legacy WebView2 data parent %q: %w", parent, err)}
	}

	var cleanupErrors []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasPrefix(name, "farm_go") || !strings.HasSuffix(name, ".exe") {
			continue
		}

		candidate := filepath.Join(parent, entry.Name())
		webviewInfo, err := os.Stat(filepath.Join(candidate, "EBWebView"))
		if err != nil || !webviewInfo.IsDir() {
			continue
		}
		if err := removeAll(candidate); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove legacy WebView2 data %q: %w", candidate, err))
		}
	}
	return cleanupErrors
}
