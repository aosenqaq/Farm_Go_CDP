package desktop

import (
	"context"
	"testing"

	"Farm_Go/internal/storage"
)

func TestUpdateCheckPreferencesAPILoadsDefaultsAndSaves(t *testing.T) {
	app := newUpdateCheckSettingsTestApp(t, true)

	got := app.UpdateCheckPreferences()
	wantDefault := storage.UpdateCheckPreferences{Enabled: true, IntervalMinutes: 120}
	if got != wantDefault {
		t.Fatalf("default preferences = %#v, want %#v", got, wantDefault)
	}

	wantSaved := storage.UpdateCheckPreferences{Enabled: true, IntervalMinutes: 30}
	saved, err := app.SaveUpdateCheckPreferences(wantSaved)
	if err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	if saved != wantSaved {
		t.Fatalf("saved preferences = %#v, want %#v", saved, wantSaved)
	}
	if loaded := app.UpdateCheckPreferences(); loaded != wantSaved {
		t.Fatalf("loaded preferences = %#v, want %#v", loaded, wantSaved)
	}
}

func newUpdateCheckSettingsTestApp(t *testing.T, authorized bool) *App {
	t.Helper()
	app := NewApp()
	app.authorizationForTests = authorized
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	app.store = store
	return app
}
