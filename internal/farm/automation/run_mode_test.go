package automation

import "testing"

func TestParseRunMode(t *testing.T) {
	if got, err := ParseRunMode(" safe "); err != nil || got != RunModeSafe {
		t.Fatalf("safe = %q, %v", got, err)
	}
	if got, err := ParseRunMode("GOD"); err != nil || got != RunModeGod {
		t.Fatalf("god = %q, %v", got, err)
	}
	if _, err := ParseRunMode("fast"); err == nil {
		t.Fatal("invalid mode should fail")
	}
	if got := NormalizeRunMode(RunMode("broken")); got != RunModeGod {
		t.Fatalf("normalized mode = %q, want %q", got, RunModeGod)
	}
}

func TestRunModeSurvivesSettingsStateConversion(t *testing.T) {
	state := StateFromSettings(Settings{
		RunMode: RunModeSafe,
		Config:  map[string]any{"autoFarmOneClickEnabled": false},
	})
	if state.RunMode != RunModeSafe {
		t.Fatalf("state mode = %q, want %q", state.RunMode, RunModeSafe)
	}
	if got := SettingsFromState(state).RunMode; got != RunModeSafe {
		t.Fatalf("round-trip mode = %q, want %q", got, RunModeSafe)
	}

	if got := StateFromSettings(Settings{RunMode: RunMode("broken")}).RunMode; got != RunModeGod {
		t.Fatalf("invalid mode = %q, want %q", got, RunModeGod)
	}
}
