package storage

import (
	"context"
	"testing"
)

func TestRuntimeSettingsPersistAutoMinimizeAfterRestart(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	input := RuntimeSettings{
		ProcessGuardAutoMinimizeAfterRestart: true,
	}
	if err := store.SaveRuntimeSettings(ctx, input); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	got, err := store.LoadRuntimeSettings(ctx)
	if err != nil || !got.ProcessGuardAutoMinimizeAfterRestart {
		t.Fatalf("got = %#v, err = %v", got, err)
	}
}

func TestRuntimeSettingsAutoMinimizeAfterRestartDefaultsFalse(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	got, err := store.LoadRuntimeSettings(ctx)
	if err != nil || got.ProcessGuardAutoMinimizeAfterRestart {
		t.Fatalf("got = %#v, err = %v", got, err)
	}
}
