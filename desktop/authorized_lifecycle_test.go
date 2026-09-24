package desktop

import (
	"context"
	"testing"

	farmruntime "Farm_Go/internal/runtime"
	"Farm_Go/internal/storage"
)

func TestStartupStartsLocalServices(t *testing.T) {
	app := NewApp()
	app.dataDir = t.TempDir()
	app.startup(context.Background())
	t.Cleanup(func() { app.shutdown(context.Background()) })
	if app.messagePushCancel == nil || app.messagePushWorkerCancel == nil {
		t.Fatal("push did not start")
	}
	if !app.guardian.Status().Running {
		t.Fatal("guardian did not start")
	}
}

func TestAuthorizedLifecycleCanActivateDeactivateAndReactivate(t *testing.T) {
	app := newAuthorizedTestApp(t)
	app.dataDir = t.TempDir()
	app.startup(context.Background())
	t.Cleanup(func() { app.shutdown(context.Background()) })
	if err := app.store.SaveRuntimeSettings(context.Background(), storage.RuntimeSettings{DefaultTarget: string(farmruntime.RuntimeTargetWeChatCDP), CurrentTarget: string(farmruntime.RuntimeTargetWeChatCDP), AutoStart: false}); err != nil {
		t.Fatalf("save runtime settings: %v", err)
	}

	if err := app.activateAuthorizedRuntime(context.Background(), 1); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if !app.guardian.Status().Running || app.messagePushCancel == nil || app.messagePushWorkerCancel == nil {
		t.Fatalf("authorized services did not start: guardian=%#v scheduler=%v worker=%v", app.guardian.Status(), app.messagePushCancel != nil, app.messagePushWorkerCancel != nil)
	}
	if err := app.activateAuthorizedRuntime(context.Background(), 1); err != nil {
		t.Fatalf("repeat activate: %v", err)
	}
	if err := app.deactivateAuthorizedRuntime(context.Background(), 1); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if app.guardian.Status().Running || app.messagePushCancel != nil || app.messagePushWorkerCancel != nil {
		t.Fatalf("authorized services remained active: guardian=%#v scheduler=%v worker=%v", app.guardian.Status(), app.messagePushCancel != nil, app.messagePushWorkerCancel != nil)
	}
	if err := app.deactivateAuthorizedRuntime(context.Background(), 1); err != nil {
		t.Fatalf("repeat deactivate: %v", err)
	}
	if err := app.activateAuthorizedRuntime(context.Background(), 2); err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	if !app.guardian.Status().Running || app.messagePushCancel == nil || app.messagePushWorkerCancel == nil {
		t.Fatal("authorized services did not restart")
	}
}
