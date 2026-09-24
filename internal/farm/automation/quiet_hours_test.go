package automation

import (
	"testing"
	"time"
)

func TestFriendQuietHoursDecisionModesWindowsAndScopes(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	tests := []struct {
		name    string
		config  map[string]any
		taskID  string
		now     time.Time
		allowed bool
		next    string
	}{
		{"sleep cross-midnight", quietHoursConfigForTest("sleep", "23:00", "07:00", []any{"steal", "help"}), "friend_steal", time.Date(2026, 7, 21, 23, 30, 0, 0, location), false, "2026-07-22T07:00:00+08:00"},
		{"sleep end boundary", quietHoursConfigForTest("sleep", "23:00", "07:00", []string{"steal"}), "friend_steal", time.Date(2026, 7, 22, 7, 0, 0, 0, location), true, ""},
		{"work before same-day window", quietHoursConfigForTest("work", "08:00", "18:00", []string{"help"}), "friend_help", time.Date(2026, 7, 21, 7, 30, 0, 0, location), false, "2026-07-21T08:00:00+08:00"},
		{"work start boundary", quietHoursConfigForTest("work", "08:00", "18:00", []string{"help"}), "friend_help", time.Date(2026, 7, 21, 8, 0, 0, 0, location), true, ""},
		{"unselected scope", quietHoursConfigForTest("sleep", "08:00", "18:00", []string{"steal"}), "friend_mischief", time.Date(2026, 7, 21, 12, 0, 0, 0, location), true, ""},
		{"non-friend task", quietHoursConfigForTest("work", "08:00", "18:00", []string{"steal", "help", "mischief"}), "own_base", time.Date(2026, 7, 21, 2, 0, 0, 0, location), true, ""},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			decision := friendQuietHoursDecision(testCase.config, testCase.taskID, testCase.now)
			if decision.Allowed != testCase.allowed {
				t.Fatalf("Allowed = %v, want %v", decision.Allowed, testCase.allowed)
			}
			gotNext := ""
			if !decision.NextAllowedAt.IsZero() {
				gotNext = decision.NextAllowedAt.Format(time.RFC3339)
			}
			if gotNext != testCase.next {
				t.Fatalf("NextAllowedAt = %q, want %q", gotNext, testCase.next)
			}
		})
	}
}

func TestFriendQuietHoursDecisionNormalizesFallbacksAndAllDay(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, location)

	invalid := quietHoursConfigForTest("unknown", "bad", "also-bad", []any{"STEAL", "steal", "unknown"})
	decision := friendQuietHoursDecision(invalid, "friend_steal", time.Date(2026, 7, 21, 23, 30, 0, 0, location))
	if decision.Allowed || decision.NextAllowedAt.Format("15:04") != "07:00" {
		t.Fatalf("invalid config fallback = %#v", decision)
	}

	allDaySleep := quietHoursConfigForTest("sleep", "08:00", "08:00", []string{"steal"})
	if decision = friendQuietHoursDecision(allDaySleep, "friend_steal", now); decision.Allowed || !decision.NextAllowedAt.IsZero() {
		t.Fatalf("all-day sleep = %#v", decision)
	}
	allDayWork := quietHoursConfigForTest("work", "08:00", "08:00", []string{"steal"})
	if decision = friendQuietHoursDecision(allDayWork, "friend_steal", now); !decision.Allowed {
		t.Fatalf("all-day work = %#v", decision)
	}
	empty := quietHoursConfigForTest("sleep", "08:00", "18:00", []string{})
	if decision = friendQuietHoursDecision(empty, "friend_steal", now); !decision.Allowed {
		t.Fatalf("empty scopes = %#v", decision)
	}
	disabled := quietHoursConfigForTest("sleep", "08:00", "18:00", []string{"steal"})
	disabled[friendQuietHoursEnabledConfigKey] = false
	if decision = friendQuietHoursDecision(disabled, "friend_steal", now); !decision.Allowed {
		t.Fatalf("disabled rule = %#v", decision)
	}
}

func quietHoursConfigForTest(mode, start, end string, scopes any) map[string]any {
	return map[string]any{
		friendQuietHoursEnabledConfigKey: true,
		friendQuietHoursModeConfigKey:    mode,
		friendQuietHoursStartConfigKey:   start,
		friendQuietHoursEndConfigKey:     end,
		friendQuietHoursScopesConfigKey:  scopes,
	}
}
