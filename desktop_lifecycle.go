package main

import (
	"context"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const closeRequestedEvent = "app:close-requested"

type desktopRuntime interface {
	Emit(context.Context, string)
	Quit(context.Context)
	Hide(context.Context)
	Show(context.Context)
}

type wailsDesktopRuntime struct{}

func (wailsDesktopRuntime) Emit(ctx context.Context, event string) {
	wailsruntime.EventsEmit(ctx, event)
}
func (wailsDesktopRuntime) Quit(ctx context.Context) { wailsruntime.Quit(ctx) }
func (wailsDesktopRuntime) Hide(ctx context.Context) { wailsruntime.WindowHide(ctx) }
func (wailsDesktopRuntime) Show(ctx context.Context) { wailsruntime.WindowShow(ctx) }

func (a *App) beforeClose(ctx context.Context) bool {
	a.closeMu.Lock()
	exitApproved := a.exitApproved
	a.exitApproved = false
	a.closeMu.Unlock()
	if exitApproved {
		return false
	}
	a.desktopRuntime.Emit(ctx, closeRequestedEvent)
	return true
}

func (a *App) ExitApplication() {
	a.closeMu.Lock()
	a.exitApproved = true
	a.closeMu.Unlock()
	a.desktopRuntime.Quit(a.contextOrBackground())
}

func (a *App) MinimizeToTray() {
	a.desktopRuntime.Hide(a.contextOrBackground())
}

func (a *App) ShowMainWindow() {
	a.desktopRuntime.Show(a.contextOrBackground())
}
