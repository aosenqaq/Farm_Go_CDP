package main

import (
	"context"
	"reflect"
	"testing"
)

func TestTrayIconUsesWindowsICOFormat(t *testing.T) {
	if len(appIcon) < 4 || !reflect.DeepEqual(appIcon[:4], []byte{0, 0, 1, 0}) {
		t.Fatal("tray icon must use the Windows ICO file format")
	}
}

type fakeDesktopRuntime struct {
	events    []string
	quitCalls int
	hideCalls int
	showCalls int
}

func (r *fakeDesktopRuntime) Emit(_ context.Context, event string) {
	r.events = append(r.events, event)
}
func (r *fakeDesktopRuntime) Quit(context.Context) { r.quitCalls++ }
func (r *fakeDesktopRuntime) Hide(context.Context) { r.hideCalls++ }
func (r *fakeDesktopRuntime) Show(context.Context) { r.showCalls++ }

func TestBeforeClosePreventsNormalCloseAndEmitsRequest(t *testing.T) {
	runtime := &fakeDesktopRuntime{}
	app := NewApp()
	app.desktopRuntime = runtime
	ctx := context.Background()

	if prevented := app.beforeClose(ctx); !prevented {
		t.Fatal("normal close should be prevented")
	}
	if got := runtime.events; !reflect.DeepEqual(got, []string{"app:close-requested"}) {
		t.Fatalf("events = %#v", got)
	}
}

func TestExitApplicationAllowsExactlyOneClose(t *testing.T) {
	runtime := &fakeDesktopRuntime{}
	app := NewApp()
	app.ctx = context.Background()
	app.desktopRuntime = runtime

	app.ExitApplication()
	if runtime.quitCalls != 1 {
		t.Fatalf("quit calls = %d, want 1", runtime.quitCalls)
	}
	if prevented := app.beforeClose(app.ctx); prevented {
		t.Fatal("approved close should proceed")
	}
	if prevented := app.beforeClose(app.ctx); !prevented {
		t.Fatal("later close should be prevented")
	}
}

func TestMinimizeAndShowUseDesktopRuntime(t *testing.T) {
	runtime := &fakeDesktopRuntime{}
	app := NewApp()
	app.ctx = context.Background()
	app.desktopRuntime = runtime

	app.MinimizeToTray()
	app.ShowMainWindow()

	if runtime.hideCalls != 1 || runtime.showCalls != 1 {
		t.Fatalf("hide/show calls = %d/%d, want 1/1", runtime.hideCalls, runtime.showCalls)
	}
}

func TestTrayCallbacksRouteToWindowActions(t *testing.T) {
	runtime := &fakeDesktopRuntime{}
	app := NewApp()
	app.ctx = context.Background()
	app.desktopRuntime = runtime

	callbacks := app.trayCallbacks()
	callbacks.show()
	callbacks.quit()

	if runtime.showCalls != 1 || runtime.quitCalls != 1 {
		t.Fatalf("show/quit calls = %d/%d, want 1/1", runtime.showCalls, runtime.quitCalls)
	}
	if prevented := app.beforeClose(app.ctx); prevented {
		t.Fatal("tray exit should approve one close")
	}
}

type fakeTrayMenuItem struct {
	onClick func()
}

func (item *fakeTrayMenuItem) Click(handler func()) {
	item.onClick = handler
}

type fakeTrayBackend struct {
	icon        []byte
	tooltip     string
	doubleClick func()
	menuItems   map[string]*fakeTrayMenuItem
}

func (tray *fakeTrayBackend) SetIcon(icon []byte) {
	tray.icon = append([]byte(nil), icon...)
}

func (tray *fakeTrayBackend) SetTooltip(tooltip string) {
	tray.tooltip = tooltip
}

func (tray *fakeTrayBackend) AddMenuItem(title, _ string) trayMenuItem {
	item := &fakeTrayMenuItem{}
	if tray.menuItems == nil {
		tray.menuItems = make(map[string]*fakeTrayMenuItem)
	}
	tray.menuItems[title] = item
	return item
}

func (tray *fakeTrayBackend) SetOnDoubleClick(handler func()) {
	tray.doubleClick = handler
}

func TestConfigureTrayRestoresOnDoubleClickAndRoutesMenuActions(t *testing.T) {
	var showCalls, quitCalls int
	tray := &fakeTrayBackend{}
	icon := []byte{0, 0, 1, 0}

	configureTray(tray, icon, trayCallbacks{
		show: func() { showCalls++ },
		quit: func() { quitCalls++ },
	})

	if got := tray.tooltip; got != "Farm_Go" {
		t.Fatalf("tooltip = %q, want Farm_Go", got)
	}
	if !reflect.DeepEqual(tray.icon, icon) {
		t.Fatalf("icon = %v, want %v", tray.icon, icon)
	}
	if tray.doubleClick == nil {
		t.Fatal("tray double-click handler was not configured")
	}

	tray.doubleClick()
	tray.menuItems["显示主窗口"].onClick()
	tray.menuItems["退出程序"].onClick()

	if showCalls != 2 {
		t.Fatalf("show calls = %d, want 2", showCalls)
	}
	if quitCalls != 1 {
		t.Fatalf("quit calls = %d, want 1", quitCalls)
	}
}
