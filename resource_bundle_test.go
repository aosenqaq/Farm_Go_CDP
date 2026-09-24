package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"Farm_Go/internal/farm"
)

func TestRuntimeScriptRemovesGoldenBugDirectProtocols(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("resources", "wmpf", "button.js"))
	if err != nil {
		t.Fatalf("read runtime script: %v", err)
	}
	text := string(script)
	for _, symbol := range []string{
		"buildFriendGoldenBugRequestBytes",
		"buildGoldenBugCleanRequestBytes",
		"findGoldenBugLandIds",
		"cleanGoldenBugsByProtocol",
		"putFriendGoldenBugsByProtocol",
	} {
		if strings.Contains(text, symbol) {
			t.Errorf("retired golden-bug protocol symbol remains: %s", symbol)
		}
	}
	for _, stateField := range []string{
		"hasGoldenBug: hasGoldenBugRuntime",
		"needGoldenBug: hasGoldenBugRuntime",
		"needsGoldenBug: hasGoldenBugRuntime",
	} {
		if !strings.Contains(text, stateField) {
			t.Errorf("golden-bug state field is missing: %s", stateField)
		}
	}
}

func TestInstallBundledGameConfigCopiesConfigAndImages(t *testing.T) {
	root, err := installBundledGameConfig(t.TempDir())
	if err != nil {
		t.Fatalf("install bundled game config: %v", err)
	}

	for _, name := range []string{
		"Plant.json",
		filepath.Join("plant_images", "stages", "_mappings", "crop_level_mapping.json"),
		filepath.Join("plant_images", "stages", "作物", "白萝卜", "白萝卜_00_作物图.png"),
	} {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("expected extracted %s: %v", name, err)
		}
		if info.IsDir() || info.Size() == 0 {
			t.Fatalf("expected %s to be a non-empty file", name)
		}
	}
}

func TestInstallBundledGameConfigUpdatesOnlyChangedOrMissingFiles(t *testing.T) {
	cacheDir := t.TempDir()
	root, err := installBundledGameConfig(cacheDir)
	if err != nil {
		t.Fatalf("install bundled game config: %v", err)
	}

	plantPath := filepath.Join(root, "Plant.json")
	if err := os.WriteFile(plantPath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale plant config: %v", err)
	}
	imagePath := filepath.Join(root, "plant_images", "stages", "作物", "白萝卜", "白萝卜_00_作物图.png")
	if err := os.Remove(imagePath); err != nil {
		t.Fatalf("remove cached image: %v", err)
	}

	manifestPath := filepath.Join(root, bundledGameConfigManifestName)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read installed manifest: %v", err)
	}
	var manifest gameConfigManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("parse installed manifest: %v", err)
	}
	manifest.Files["Plant.json"] = "outdated"
	raw, err = json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal stale manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, raw, 0o644); err != nil {
		t.Fatalf("write stale manifest: %v", err)
	}

	if _, err := installBundledGameConfig(cacheDir); err != nil {
		t.Fatalf("incrementally install bundled game config: %v", err)
	}
	if raw, err := os.ReadFile(plantPath); err != nil || string(raw) == "stale" {
		t.Fatalf("expected changed Plant.json to be restored, raw=%q err=%v", raw, err)
	}
	if info, err := os.Stat(imagePath); err != nil || info.Size() == 0 {
		t.Fatalf("expected missing image to be restored, info=%v err=%v", info, err)
	}
}

func TestAppStartupUsesBundledGameConfig(t *testing.T) {
	farm.SetGameConfigRoot("")
	t.Cleanup(func() { farm.SetGameConfigRoot("") })

	app := NewApp()
	app.dataDir = t.TempDir()
	app.startup(context.Background())
	defer app.shutdown(context.Background())

	root := farm.DefaultGameConfigRoot()
	if !strings.HasPrefix(root, app.dataDir) {
		t.Fatalf("expected startup to use extracted resource root below %q, got %q", app.dataDir, root)
	}
	if _, err := os.Stat(filepath.Join(root, "Plant.json")); err != nil {
		t.Fatalf("expected Plant.json in startup resource root: %v", err)
	}
}

func TestAppShutdownClearsBundledGameConfigRoot(t *testing.T) {
	farm.SetGameConfigRoot("")
	t.Cleanup(func() { farm.SetGameConfigRoot("") })

	app := NewApp()
	app.dataDir = t.TempDir()
	app.startup(context.Background())
	app.shutdown(context.Background())

	if strings.HasPrefix(farm.DefaultGameConfigRoot(), app.dataDir) {
		t.Fatalf("expected shutdown to clear the bundled resource root, got %q", farm.DefaultGameConfigRoot())
	}
}
