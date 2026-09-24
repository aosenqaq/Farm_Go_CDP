package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMain(m *testing.M) {
	root := repoRoot()
	qqHost, err := os.ReadFile(filepath.Join(root, "resources", "qq", "qq-host.js"))
	if err != nil {
		panic(err)
	}
	button, err := os.ReadFile(filepath.Join(root, "resources", "wmpf", "button.js"))
	if err != nil {
		panic(err)
	}
	fridaPython, err := os.ReadFile(filepath.Join(root, "resources", "frida-python.bundle.zip"))
	if err != nil {
		panic(err)
	}
	gameConfig, err := os.ReadFile(filepath.Join(root, "resources", "gameConfig.bundle.zip"))
	if err != nil {
		panic(err)
	}
	qqHostScript = string(qqHost)
	runtimeButtonScript = string(button)
	embeddedFridaResources = os.DirFS(root)
	icon, err := os.ReadFile(filepath.Join(root, "build", "windows", "icon.ico"))
	if err != nil {
		panic(err)
	}
	bundledFridaPythonArchive = fridaPython
	bundledGameConfigArchive = gameConfig
	appIcon = icon
	os.Exit(m.Run())
}

func repoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ".."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}
