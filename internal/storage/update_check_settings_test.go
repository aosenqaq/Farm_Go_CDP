package storage

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestUpdateCheckPreferencesDefaultToEnabledEvery120Minutes(t *testing.T) {
	store := openUpdateCheckTestStore(t)

	got, err := store.LoadUpdateCheckPreferences(context.Background())
	if err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	want := UpdateCheckPreferences{Enabled: true, IntervalMinutes: 120}
	if got != want {
		t.Fatalf("preferences = %#v, want %#v", got, want)
	}
}

func TestUpdateCheckPreferencesRoundTrip(t *testing.T) {
	store := openUpdateCheckTestStore(t)
	ctx := context.Background()
	want := UpdateCheckPreferences{Enabled: false, IntervalMinutes: 45}

	if err := store.SaveUpdateCheckPreferences(ctx, want); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	got, err := store.LoadUpdateCheckPreferences(ctx)
	if err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	if got != want {
		t.Fatalf("preferences = %#v, want %#v", got, want)
	}
}

func TestUpdateCheckPreferencesFallsBackForInvalidStoredInterval(t *testing.T) {
	for _, value := range []string{"0", "10081", "not-a-number"} {
		t.Run(value, func(t *testing.T) {
			store := openUpdateCheckTestStore(t)
			ctx := context.Background()
			if _, err := store.db.ExecContext(
				ctx,
				settingUpsertSQL,
				GlobalSettingsAccountKey,
				"update.checkIntervalMinutes",
				value,
				time.Now().Format(time.RFC3339Nano),
			); err != nil {
				t.Fatalf("seed interval: %v", err)
			}

			got, err := store.LoadUpdateCheckPreferences(ctx)
			if err != nil {
				t.Fatalf("load preferences: %v", err)
			}
			if got.IntervalMinutes != 120 {
				t.Fatalf("interval = %d, want 120", got.IntervalMinutes)
			}
		})
	}
}

func TestSaveUpdateCheckPreferencesValidatesIntervalBounds(t *testing.T) {
	store := openUpdateCheckTestStore(t)
	ctx := context.Background()

	for _, interval := range []int{1, 10080} {
		if err := store.SaveUpdateCheckPreferences(ctx, UpdateCheckPreferences{Enabled: true, IntervalMinutes: interval}); err != nil {
			t.Fatalf("save valid interval %d: %v", interval, err)
		}
	}
	for _, interval := range []int{0, 10081} {
		err := store.SaveUpdateCheckPreferences(ctx, UpdateCheckPreferences{Enabled: true, IntervalMinutes: interval})
		if !errors.Is(err, ErrInvalidUpdateCheckInterval) {
			t.Fatalf("save invalid interval %d error = %v, want %v", interval, err, ErrInvalidUpdateCheckInterval)
		}
	}
}

func openUpdateCheckTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
