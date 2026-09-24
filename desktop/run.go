package desktop

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"Farm_Go/internal/farm"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

var appIcon []byte

type Resources struct {
	Frontend           fs.FS
	Icon               []byte
	QQHostScript       string
	ButtonScript       string
	FridaResources     fs.FS
	FridaPythonArchive []byte
	GameConfigArchive  []byte
}

func Run(resources Resources) {
	qqHostScript = resources.QQHostScript
	runtimeButtonScript = resources.ButtonScript
	embeddedFridaResources = resources.FridaResources
	bundledFridaPythonArchive = resources.FridaPythonArchive
	bundledGameConfigArchive = resources.GameConfigArchive

	app := NewApp(resources.Frontend)
	for _, cleanupErr := range cleanupLegacyWebviewDataDirs(app.dataDir, os.RemoveAll) {
		slog.Warn("clean legacy WebView2 data directory", "error", cleanupErr)
	}

	err := wails.Run(&options.App{
		Title:     "Farm_Go",
		Width:     1024,
		Height:    720,
		MinWidth:  960,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets:     resources.Frontend,
			Middleware: newAssetMiddleware(app),
		},
		BackgroundColour: &options.RGBA{R: 247, G: 246, B: 241, A: 1},
		Windows:          appWindowsOptions(app.dataDir),
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			startTray(resources.Icon, app.trayCallbacks())
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
