package wmpf

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	farmruntime "Farm_Go/internal/runtime"
)

type FridaHookOptions struct {
	Root              string
	Resources         fs.FS
	Target            farmruntime.RuntimeTarget
	Version           string
	DebugWebSocketURL string
}

type FridaAttachOptions struct {
	PID  int
	Hook FridaHookOptions
}

type FridaDevice interface {
	Attach(ctx context.Context, pid int) (FridaSession, error)
}

type FridaSession interface {
	CreateScript(ctx context.Context, source string) (FridaScript, error)
}

type FridaScript interface {
	Load(ctx context.Context) error
}

type fridaAddressConfig struct {
	Version                any    `json:"Version,omitempty"`
	LoadStartHookOffset    string `json:"LoadStartHookOffset"`
	CDPFilterHookOffset    string `json:"CDPFilterHookOffset"`
	SceneOffsets           []int  `json:"SceneOffsets"`
	ScenePathOffsets       []int  `json:"ScenePathOffsets,omitempty"`
	SceneWhitelist         []int  `json:"SceneWhitelist,omitempty"`
	ExtraPatchSceneNumbers []int  `json:"ExtraPatchSceneNumbers,omitempty"`
	DebugScenes            []int  `json:"DebugScenes,omitempty"`
	RuntimeTarget          string `json:"RuntimeTarget,omitempty"`
	DebugWebSocketURL      string `json:"DebugWebSocketURL,omitempty"`
}

func ValidateFridaResources(root string) error {
	if _, err := os.Stat(filepath.Join(root, "hook.js")); err != nil {
		return err
	}
	return filepath.WalkDir(filepath.Join(root, "config"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "addresses.") || !strings.HasSuffix(entry.Name(), ".json") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var cfg fridaAddressConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if cfg.LoadStartHookOffset == "" || cfg.CDPFilterHookOffset == "" || len(cfg.SceneOffsets) == 0 {
			return fmt.Errorf("%s: invalid WMPF CDP config schema", path)
		}
		return nil
	})
}

func BuildFridaHookScript(options FridaHookOptions) (string, error) {
	if options.Root == "" {
		return "", fmt.Errorf("frida resource root is required")
	}
	hook, err := readFridaResource(options, "hook.js")
	if err != nil {
		return "", err
	}
	config, err := loadFridaAddressConfig(options)
	if err != nil {
		return "", err
	}
	config.RuntimeTarget = hookRuntimeTarget(options.Target)
	config.DebugWebSocketURL = options.DebugWebSocketURL
	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(string(hook), "@@CONFIG@@", string(configJSON)), nil
}

func AttachAndLoadFridaHook(ctx context.Context, device FridaDevice, options FridaAttachOptions) error {
	if device == nil {
		return fmt.Errorf("frida device is required")
	}
	source, err := BuildFridaHookScript(options.Hook)
	if err != nil {
		return err
	}
	session, err := device.Attach(ctx, options.PID)
	if err != nil {
		return err
	}
	script, err := session.CreateScript(ctx, source)
	if err != nil {
		return err
	}
	return script.Load(ctx)
}

func loadFridaAddressConfig(options FridaHookOptions) (fridaAddressConfig, error) {
	if options.Version == "" {
		return fridaAddressConfig{}, fmt.Errorf("wmpf version is required")
	}
	name := path.Join("config", fmt.Sprintf("addresses.%s.json", options.Version))
	if options.Target == farmruntime.RuntimeTargetYYBCDP {
		name = path.Join("config", "yyb", fmt.Sprintf("addresses.%s.json", options.Version))
	}
	data, err := readFridaResource(options, name)
	if err != nil {
		return fridaAddressConfig{}, err
	}
	var cfg fridaAddressConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fridaAddressConfig{}, err
	}
	if cfg.LoadStartHookOffset == "" || cfg.CDPFilterHookOffset == "" || len(cfg.SceneOffsets) == 0 {
		return fridaAddressConfig{}, fmt.Errorf("%s: invalid WMPF CDP config schema", name)
	}
	return cfg, nil
}

func readFridaResource(options FridaHookOptions, name string) ([]byte, error) {
	if options.Resources != nil {
		return fs.ReadFile(options.Resources, path.Join(filepath.ToSlash(options.Root), name))
	}
	return os.ReadFile(filepath.Join(options.Root, filepath.FromSlash(name)))
}

func hookRuntimeTarget(target farmruntime.RuntimeTarget) string {
	if target == farmruntime.RuntimeTargetYYBCDP {
		return "yyb"
	}
	return "cdp"
}
