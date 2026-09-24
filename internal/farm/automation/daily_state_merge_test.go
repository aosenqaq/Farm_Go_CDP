package automation

import (
	"testing"
	"time"
)

func TestFriendHelpDailyStateCountsCurrentDateOnly(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.Local)
	config := map[string]any{
		"autoFarmFriendHelpDailyLimit":    5,
		FriendHelpDailyCountDateConfigKey: "2026-07-17",
		FriendHelpDailyCountConfigKey:     3,
	}

	remaining, limited := FriendHelpDailyRemaining(config, now)

	if !limited || remaining != 2 {
		t.Fatalf("remaining = %d, limited = %v", remaining, limited)
	}
	config[FriendHelpDailyCountDateConfigKey] = "2026-07-16"
	remaining, limited = FriendHelpDailyRemaining(config, now)
	if !limited || remaining != 5 {
		t.Fatalf("new day remaining = %d, limited = %v", remaining, limited)
	}
}

func TestFriendHelpDailyStateTreatsZeroAsUnlimited(t *testing.T) {
	remaining, limited := FriendHelpDailyRemaining(map[string]any{"autoFarmFriendHelpDailyLimit": 0}, time.Now())

	if limited || remaining != 0 {
		t.Fatalf("remaining = %d, limited = %v", remaining, limited)
	}
}

func TestMarkFriendHelpDailySuccessesResetsAndIncrements(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.Local)
	config := MarkFriendHelpDailySuccesses(map[string]any{
		FriendHelpDailyCountDateConfigKey: "2026-07-16",
		FriendHelpDailyCountConfigKey:     20,
	}, now, 2)

	if config[FriendHelpDailyCountDateConfigKey] != "2026-07-17" || config[FriendHelpDailyCountConfigKey] != 2 {
		t.Fatalf("daily state = %#v", config)
	}
	config = MarkFriendHelpDailySuccesses(config, now, 3)
	if config[FriendHelpDailyCountConfigKey] != 5 {
		t.Fatalf("daily count = %#v", config[FriendHelpDailyCountConfigKey])
	}
}

func TestMergeRuntimeDailyStatePreservesOwnedMarkersOnly(t *testing.T) {
	current := map[string]any{
		FriendMischiefDailyDoneDateConfigKey:              "2026-07-10",
		DailyOnceTaskDoneDateConfigKey("svip_daily_gift"): "2026-07-10",
		FriendHelpDailyCountDateConfigKey:                 "2026-07-10",
		FriendHelpDailyCountConfigKey:                     7,
		"autoFarmPlantSeedId":                             20001,
	}
	incoming := map[string]any{
		"autoFarmPlantSeedId":             20002,
		FriendHelpDailyCountDateConfigKey: "2026-07-09",
		FriendHelpDailyCountConfigKey:     2,
	}

	merged := MergeRuntimeDailyState(incoming, current)

	if merged[FriendMischiefDailyDoneDateConfigKey] != "2026-07-10" || merged[DailyOnceTaskDoneDateConfigKey("svip_daily_gift")] != "2026-07-10" {
		t.Fatalf("runtime-owned markers were not preserved: %#v", merged)
	}
	if merged[FriendHelpDailyCountDateConfigKey] != "2026-07-10" || merged[FriendHelpDailyCountConfigKey] != 7 {
		t.Fatalf("friend help daily count was not preserved: %#v", merged)
	}
	if merged["autoFarmPlantSeedId"] != 20002 {
		t.Fatalf("user config was overwritten: %#v", merged)
	}
	merged["autoFarmPlantSeedId"] = 20003
	if incoming["autoFarmPlantSeedId"] != 20002 {
		t.Fatalf("incoming config was mutated: %#v", incoming)
	}
}
