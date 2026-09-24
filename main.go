package main

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"Farm_Go/internal/farm"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var appIcon []byte

func newAssetMiddleware(app *App) func(http.Handler) http.Handler {
	pollHandler := newRuntimePollHandler(newAppRuntimePollService(app))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, runtimePollPrefix) {
				pollHandler.ServeHTTP(w, r)
				return
			}
			if strings.HasPrefix(r.URL.Path, "/farm-assets/") && farm.ServeLocalGameConfigImage(w, r) {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func main() {
	// Create an instance of the app structure
	app := NewApp(assets)
	for _, cleanupErr := range cleanupLegacyWebviewDataDirs(app.dataDir, os.RemoveAll) {
		slog.Warn("clean legacy WebView2 data directory", "error", cleanupErr)
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "Farm_Go",
		Width:     1024,
		Height:    720,
		MinWidth:  960,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: newAssetMiddleware(app),
		},
		BackgroundColour: &options.RGBA{R: 247, G: 246, B: 241, A: 1},
		Windows:          appWindowsOptions(app.dataDir),
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			startTray(appIcon, app.trayCallbacks())
		},
		OnBeforeClose: app.beforeClose,
		OnShutdown: func(ctx context.Context) {
			stopTray()
			app.shutdown(ctx)
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
