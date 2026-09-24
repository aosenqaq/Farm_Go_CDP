package main

import (
	"embed"

	"Farm_Go/desktop"
)

//go:embed all:frontend/dist
var frontendAssets embed.FS

//go:embed build/windows/icon.ico
var appIcon []byte

//go:embed resources/qq/qq-host.js
var qqHostScript string

//go:embed resources/wmpf/button.js
var runtimeButtonScript string

//go:embed all:resources/wmpf/frida
var fridaResources embed.FS

//go:embed resources/frida-python.bundle.zip
var fridaPythonArchive []byte

//go:embed resources/gameConfig.bundle.zip
var gameConfigArchive []byte

func main() {
	desktop.Run(desktop.Resources{
		Frontend:           frontendAssets,
		Icon:               appIcon,
		QQHostScript:       qqHostScript,
		ButtonScript:       runtimeButtonScript,
		FridaResources:     fridaResources,
		FridaPythonArchive: fridaPythonArchive,
		GameConfigArchive:  gameConfigArchive,
	})
}
