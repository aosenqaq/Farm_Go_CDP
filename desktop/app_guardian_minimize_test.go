package desktop

import (
	"errors"
	"testing"
	"time"

	farmruntime "Farm_Go/internal/runtime"
	"Farm_Go/internal/runtime/guard"
)

func TestSaveGuardianSettingsPersistsAutoMinimizeAfterRestart(t *testing.T) {
	app := newAuthorizedTestApp(t)
	saved, err := app.SaveGuardianSettings(map[string]any{"autoMinimizeAfterRestart": true})
	if err != nil || !saved.AutoMinimizeAfterRestart {
		t.Fatalf("saved = %#v, err = %v", saved, err)
	}
}

func TestGuardianQQRestartMinimizesReplacementWindowAfterReady(t *testing.T) {
	app := newAuthorizedTestApp(t)
	configureQQFinalRestartTest(t, app)
	if _, err := app.SaveGuardianSettings(map[string]any{"autoMinimizeAfterRestart": true}); err != nil {
		t.Fatalf("enable auto minimize: %v", err)
	}
	var minimized []guard.HostWindowSnapshot
	app.minimizeHostWindows = func(windows []guard.HostWindowSnapshot) error {
		minimized = append(minimized, windows...)
		return nil
	}
	result, err := app.restartHostForRuntimeTarget(string(farmruntime.RuntimeTargetQQWS))
	if err != nil || result.Status != guard.RestartStatusReconnected {
		t.Fatalf("restart = %#v, err = %v", result, err)
	}
	if len(minimized) != 1 || minimized[0].PID != 30 || minimized[0].HWND != 300 {
		t.Fatalf("minimized = %#v, want replacement PID 30 HWND 300", minimized)
	}
}

func TestGuardianQQRestartDoesNotMinimizeWhenDisabledOrRestartFails(t *testing.T) {
	for _, test := range []struct {
		name    string
		enabled bool
		fail    func(*App)
	}{
		{name: "disabled", enabled: false},
		{name: "launch fails", enabled: true, fail: func(app *App) {
			app.launchHost = func(guard.LaunchRequest) error { return errors.New("launch failed") }
		}},
		{name: "reconnect fails", enabled: true, fail: func(app *App) {
			app.waitForRuntimeStatus = func(time.Duration, func(farmruntime.Status) bool) bool { return false }
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := newAuthorizedTestApp(t)
			configureQQFinalRestartTest(t, app)
			if test.enabled {
				if _, err := app.SaveGuardianSettings(map[string]any{"autoMinimizeAfterRestart": true}); err != nil {
					t.Fatalf("enable auto minimize: %v", err)
				}
			}
			if test.fail != nil {
				test.fail(app)
			}
			calls := 0
			app.minimizeHostWindows = func([]guard.HostWindowSnapshot) error { calls++; return nil }
			_, _ = app.restartHostForRuntimeTarget(string(farmruntime.RuntimeTargetQQWS))
			app.recordProcessGuardRuntimeStatus(farmruntime.Status{Target: "qq_ws", Ready: true, Connected: true})
			if calls != 0 {
				t.Fatalf("minimize calls = %d, want 0", calls)
			}
		})
	}
}

func TestGuardianYYBRestartMinimizesRefreshedWindowAfterReady(t *testing.T) {
	app := newAuthorizedTestApp(t)
	configureYYBSoftRestartTest(t, app, nil)
	if _, err := app.SaveGuardianSettings(map[string]any{"autoMinimizeAfterRestart": true}); err != nil {
		t.Fatalf("enable auto minimize: %v", err)
	}
	var minimized []guard.HostWindowSnapshot
	app.minimizeHostWindows = func(windows []guard.HostWindowSnapshot) error {
		minimized = append(minimized, windows...)
		return nil
	}

	result := app.RestartHostProcess()
	if result.Status != guard.RestartStatusReconnected {
		t.Fatalf("restart = %#v", result)
	}
	if len(minimized) != 1 || minimized[0].PID != 42 || minimized[0].HWND != 421 {
		t.Fatalf("minimized = %#v, want refreshed HWND 421", minimized)
	}
}
