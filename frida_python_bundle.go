package main

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const bundledFridaRuntimeManifestName = ".farm-go-frida-runtime-manifest.json"

//go:embed resources/frida-python.bundle.zip
var bundledFridaPythonArchive []byte

func bundledFridaPythonPath(dataDir string) string {
	return filepath.Join(dataDir, "runtime", "frida-python", "python.exe")
}

func installBundledFridaPython(dataDir string) (string, error) {
	archive, err := zip.NewReader(bytes.NewReader(bundledFridaPythonArchive), int64(len(bundledFridaPythonArchive)))
	if err != nil {
		return "", fmt.Errorf("open bundled Frida Python: %w", err)
	}
	entries := make(map[string]*zip.File, len(archive.File))
	var manifestRaw []byte
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		if entry.Name == bundledFridaRuntimeManifestName {
			reader, err := entry.Open()
			if err != nil {
				return "", err
			}
			manifestRaw, err = io.ReadAll(reader)
			_ = reader.Close()
			if err != nil {
				return "", err
			}
			continue
		}
		entries[entry.Name] = entry
	}
	manifest := gameConfigManifest{}
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil || len(manifest.Files) == 0 {
		return "", fmt.Errorf("parse bundled Frida Python manifest: %w", err)
	}
	root := filepath.Join(dataDir, "runtime", "frida-python")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	installed := readInstalledGameConfigManifest(root)
	for name, hash := range manifest.Files {
		target, err := gameConfigCachePath(root, name)
		if err != nil {
			return "", err
		}
		info, statErr := os.Stat(target)
		if installed.Files[name] == hash && statErr == nil && !info.IsDir() && info.Size() > 0 {
			continue
		}
		entry := entries[name]
		if entry == nil {
			return "", fmt.Errorf("bundled Frida Python is missing %s", name)
		}
		if err := extractBundledGameConfigFile(entry, target); err != nil {
			return "", err
		}
	}
	if err := writeInstalledGameConfigManifest(root, manifest); err != nil {
		return "", err
	}
	python := bundledFridaPythonPath(dataDir)
	if info, err := os.Stat(python); err != nil || info.IsDir() || info.Size() == 0 {
		return "", fmt.Errorf("bundled Frida Python executable is missing: %w", err)
	}
	return python, nil
}
