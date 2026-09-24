package desktop

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const bundledGameConfigManifestName = ".farm-go-resource-manifest.json"

var bundledGameConfigArchive []byte

type gameConfigManifest struct {
	Files map[string]string `json:"files"`
}

func installBundledGameConfig(dataDir string) (string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return "", fmt.Errorf("game config cache directory is empty")
	}
	bundle, manifest, err := openBundledGameConfig()
	if err != nil {
		return "", err
	}

	root := filepath.Join(dataDir, "cache", "gameConfig")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create game config cache: %w", err)
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
		entry := bundle[name]
		if entry == nil {
			return "", fmt.Errorf("bundled game config is missing %s", name)
		}
		if err := extractBundledGameConfigFile(entry, target); err != nil {
			return "", fmt.Errorf("extract %s: %w", name, err)
		}
	}
	for name := range installed.Files {
		if _, present := manifest.Files[name]; present {
			continue
		}
		target, err := gameConfigCachePath(root, name)
		if err != nil {
			return "", err
		}
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("remove retired game config %s: %w", name, err)
		}
	}
	if err := writeInstalledGameConfigManifest(root, manifest); err != nil {
		return "", err
	}
	return root, nil
}

func openBundledGameConfig() (map[string]*zip.File, gameConfigManifest, error) {
	archive, err := zip.NewReader(bytes.NewReader(bundledGameConfigArchive), int64(len(bundledGameConfigArchive)))
	if err != nil {
		return nil, gameConfigManifest{}, fmt.Errorf("open bundled game config: %w", err)
	}
	files := make(map[string]*zip.File, len(archive.File))
	var manifestRaw []byte
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		if entry.Name == bundledGameConfigManifestName {
			reader, err := entry.Open()
			if err != nil {
				return nil, gameConfigManifest{}, fmt.Errorf("open bundled manifest: %w", err)
			}
			manifestRaw, err = io.ReadAll(reader)
			closeErr := reader.Close()
			if err != nil {
				return nil, gameConfigManifest{}, fmt.Errorf("read bundled manifest: %w", err)
			}
			if closeErr != nil {
				return nil, gameConfigManifest{}, fmt.Errorf("close bundled manifest: %w", closeErr)
			}
			continue
		}
		files[entry.Name] = entry
	}
	manifest := gameConfigManifest{}
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil || len(manifest.Files) == 0 {
		if err == nil {
			err = fmt.Errorf("no files")
		}
		return nil, gameConfigManifest{}, fmt.Errorf("parse bundled game config manifest: %w", err)
	}
	return files, manifest, nil
}

func readInstalledGameConfigManifest(root string) gameConfigManifest {
	raw, err := os.ReadFile(filepath.Join(root, bundledGameConfigManifestName))
	if err != nil {
		return gameConfigManifest{Files: map[string]string{}}
	}
	manifest := gameConfigManifest{}
	if err := json.Unmarshal(raw, &manifest); err != nil || manifest.Files == nil {
		return gameConfigManifest{Files: map[string]string{}}
	}
	return manifest
}

func writeInstalledGameConfigManifest(root string, manifest gameConfigManifest) error {
	raw, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal game config manifest: %w", err)
	}
	return os.WriteFile(filepath.Join(root, bundledGameConfigManifestName), raw, 0o644)
}

func gameConfigCachePath(root string, name string) (string, error) {
	path := filepath.Clean(filepath.FromSlash(name))
	if path == "." || filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid bundled game config path %q", name)
	}
	return filepath.Join(root, path), nil
}

func extractBundledGameConfigFile(entry *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	reader, err := entry.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	temporary := target + ".tmp"
	output, err := os.Create(temporary)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, reader)
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(temporary)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(temporary)
		return closeErr
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, target)
}
