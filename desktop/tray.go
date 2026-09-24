package desktop

import (
	"runtime"

	"github.com/energye/systray"
)

type trayCallbacks struct {
	show func()
	quit func()
}

func (a *App) trayCallbacks() trayCallbacks {
	return trayCallbacks{
		show: a.ShowMainWindow,
		quit: a.ExitApplication,
	}
}

type trayMenuItem interface {
	Click(func())
}

type trayBackend interface {
	SetIcon([]byte)
	SetTooltip(string)
	AddMenuItem(string, string) trayMenuItem
	SetOnDoubleClick(func())
}

type energyeTrayBackend struct{}

func (energyeTrayBackend) SetIcon(icon []byte) {
	systray.SetIcon(icon)
}

func (energyeTrayBackend) SetTooltip(tooltip string) {
	systray.SetTooltip(tooltip)
}

func (energyeTrayBackend) AddMenuItem(title, tooltip string) trayMenuItem {
	return systray.AddMenuItem(title, tooltip)
}

func (energyeTrayBackend) SetOnDoubleClick(handler func()) {
	systray.SetOnDClick(func(systray.IMenu) {
		handler()
	})
}

func configureTray(tray trayBackend, icon []byte, callbacks trayCallbacks) {
	tray.SetIcon(icon)
	tray.SetTooltip("Farm_Go")
	tray.SetOnDoubleClick(callbacks.show)
	showItem := tray.AddMenuItem("显示主窗口", "恢复 Farm_Go")
	exitItem := tray.AddMenuItem("退出程序", "退出 Farm_Go")
	showItem.Click(callbacks.show)
	exitItem.Click(callbacks.quit)
}

func startTray(icon []byte, callbacks trayCallbacks) {
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		systray.Run(func() {
			configureTray(energyeTrayBackend{}, icon, callbacks)
		}, func() {})
	}()
}

func stopTray() {
	systray.Quit()
}
