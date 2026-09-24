package storage

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestLANAccessSettingsRoundTripKeepsHashOutOfPublicDTO(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	defaults, err := store.LoadLANAccessSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantDefaults := LANAccessSettings{Mode: LANAccessModeLAN, Port: 8788}
	if !reflect.DeepEqual(defaults, wantDefaults) {
		t.Fatalf("defaults = %#v want %#v", defaults, wantDefaults)
	}

	want := LANAccessSettings{
		Enabled:      true,
		Mode:         LANAccessModeTunnel,
		Port:         8788,
		PasswordHash: "argon2id$secret",
	}
	if err := store.SaveLANAccessSettings(ctx, want); err != nil {
		t.Fatal(err)
	}

	got, err := store.LoadLANAccessSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}

	public := got.Public()
	wantPublic := LANAccessPublicSettings{
		Enabled:            true,
		Mode:               LANAccessModeTunnel,
		Port:               8788,
		PasswordConfigured: true,
	}
	if !reflect.DeepEqual(public, wantPublic) {
		t.Fatalf("public = %#v want %#v", public, wantPublic)
	}
	if _, ok := reflect.TypeOf(public).FieldByName("PasswordHash"); ok {
		t.Fatal("public DTO exposes PasswordHash")
	}
	if strings.Contains(fmt.Sprintf("%#v", public), "secret") {
		t.Fatal("public DTO leaked hash")
	}
}
