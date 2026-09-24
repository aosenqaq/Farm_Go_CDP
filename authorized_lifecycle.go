package main

import (
	"context"
	"fmt"
	"log/slog"

	"Farm_Go/internal/eventbus"
	farmruntime "Farm_Go/internal/runtime"
)

// activateAuthorizedRuntime starts services that are permitted only for one authorized session.
func (a *App) activateAuthorizedRuntime(ctx context.Context, generation uint64) error {
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()

	if a.authorized && a.authorizedGeneration == generation {
		return nil
	}
	if a.authorized {
		return fmt.Errorf("an authorized runtime session is already active")
	}
	if a.store == nil {
		return fmt.Errorf("application storage is not initialized")
	}
	if ctx == nil {
		ctx = a.processCtx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	a.authorized = true
	a.authorizedGeneration = generation
	a.authorizedSessionCancel = cancel

	if err := a.startAuthorizedRuntime(sessionCtx); err != nil {
		a.stopAuthorizedRuntimeLocked()
		return err
	}
	return nil
}

// deactivateAuthorizedRuntime stops an authorized session. A stale generation cannot stop a newer one.
func (a *App) deactivateAuthorizedRuntime(_ context.Context, generation uint64) error {
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()
	if !a.authorized || (generation != 0 && generation != a.authorizedGeneration) {
		return nil
	}
	a.stopAuthorizedRuntimeLocked()
	return nil
}

func (a *App) startAuthorizedRuntime(ctx context.Context) error {
	a.ensureMessagePushService()
	a.startMessagePushDeliveryWorker(ctx)
	a.startMessagePushDailyScheduler(ctx)

	settings, err := a.store.LoadRuntimeSettings(ctx)
	if err != nil {
		a.lastErr = err
		a.recordEvent(eventbus.Event{Level: eventbus.LevelError, Source: "settings", Type: "settings.load", Message: "load runtime settings failed", Data: map[string]any{"error": err.Error()}})
		return err
	}
	settings.CurrentTarget = settings.DefaultTarget
	a.applyRuntimeSettings(settings)
	a.rebuildCDPLinks()
	a.setIdleRuntimeTarget(settings.DefaultTarget)
	if err := a.store.SaveRuntimeSettings(ctx, settings); err != nil {
		a.lastErr = err
		slog.Error("reset current runtime target", "error", err)
		a.recordEvent(eventbus.Event{Level: eventbus.LevelError, Source: "settings", Type: "settings.save", Message: "reset current runtime target failed", Data: map[string]any{"error": err.Error()}})
		return err
	}
	a.recordEvent(eventbus.Event{Level: eventbus.LevelInfo, Source: "settings", Type: "settings.load", Message: "runtime settings loaded", Data: map[string]any{
		"defaultTarget": settings.DefaultTarget, "currentTarget": settings.CurrentTarget, "autoStart": settings.AutoStart, "cdpPort": settings.CDPPort, "wmpfDebugPort": settings.WMPFDebugPort,
	}})

	if a.guardian != nil {
		a.guardian.Start(ctx)
	}
	if a.cfg.Runtime.AutoStart {
		go a.autostartAuthorizedRuntime(ctx)
	}
	if a.cfg.Runtime.DefaultTarget == string(farmruntime.RuntimeTargetQQWS) {
		go a.runQQDebugPatch("startup", "")
	}
	return nil
}

func (a *App) autostartAuthorizedRuntime(ctx context.Context) {
	a.recordEvent(eventbus.Event{Level: eventbus.LevelInfo, Source: a.cfg.Runtime.DefaultTarget, Type: "runtime.autostart", Message: "auto starting runtime link"})
	if err := a.supervisor.Switch(ctx, farmruntime.RuntimeTarget(a.cfg.Runtime.DefaultTarget)); err != nil {
		a.lastErr = err
		slog.Error("runtime link stopped", "error", err)
		a.recordEvent(eventbus.Event{Level: eventbus.LevelError, Source: a.cfg.Runtime.DefaultTarget, Type: "runtime.autostart", Message: "auto start runtime link failed", Data: map[string]any{"error": err.Error()}})
	}
}

func (a *App) stopAuthorizedRuntimeLocked() {
	// Mark unavailable and cancel first so no new session work is dispatched while stopping.
	a.authorized = false
	a.authorizedGeneration = 0
	cancel := a.authorizedSessionCancel
	a.authorizedSessionCancel = nil
	if cancel != nil {
		cancel()
	}
	a.StopFarmAutomationScheduler()
	a.stopMessagePushDailyScheduler()
	a.stopMessagePushDeliveryWorker()
	if a.guardian != nil {
		a.guardian.Close()
	}
	if a.supervisor != nil {
		if err := a.supervisor.Stop(context.Background()); err != nil {
			slog.Error("stop runtime supervisor", "error", err)
		}
	}
	a.resetAccountScopedServices()
}
