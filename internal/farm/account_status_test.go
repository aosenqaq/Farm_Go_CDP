package farm

import "testing"

func TestBuildRuntimeAccountProfileConvertsCumulativeUnderscoreExpToLevelProgress(t *testing.T) {
	profile := BuildRuntimeAccountProfile(map[string]any{
		"level": 95,
		"_exp":  11341300,
	})

	if profile.Exp != 11341300 {
		t.Fatalf("expected _exp to populate account experience, got %d", profile.Exp)
	}
	if profile.LevelProgress.Current != 5000 {
		t.Fatalf("expected 5,000 experience within level 95, got %#v", profile.LevelProgress)
	}
	if profile.LevelProgress.Needed != 362000 || profile.LevelProgress.Remaining != 357000 {
		t.Fatalf("expected level 95 progress against level 96 threshold, got %#v", profile.LevelProgress)
	}
	if profile.LevelProgress.NextLevel != 96 {
		t.Fatalf("expected next level 96, got %#v", profile.LevelProgress)
	}
}

func TestBuildRuntimeAccountProfileBuildsProgressFromInLevelRuntimeExp(t *testing.T) {
	profile := BuildRuntimeAccountProfile(map[string]any{
		"level": 95,
		"exp":   331744,
	})

	if profile.LevelProgress.Current != 331744 {
		t.Fatalf("expected runtime experience within level 95, got %#v", profile.LevelProgress)
	}
	if profile.LevelProgress.Needed != 362000 || profile.LevelProgress.Remaining != 30256 {
		t.Fatalf("expected level 95 progress against level 96 threshold, got %#v", profile.LevelProgress)
	}
}
