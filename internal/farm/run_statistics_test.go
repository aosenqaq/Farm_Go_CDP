package farm

import (
	"testing"
	"time"

	"Farm_Go/internal/eventbus"
)

func TestBuildRunStatisticsCountsOnlySuccessfulAutomationTasks(t *testing.T) {
	startedAt := time.Date(2026, 7, 13, 10, 0, 0, 0, time.Local)
	now := startedAt.Add(95 * time.Second)
	events := []eventbus.Event{
		runStatisticsTaskDone(startedAt.Add(time.Second), "own_collect", 1),
		runStatisticsTaskDone(startedAt.Add(1500*time.Millisecond), "own_fertilizer", 2),
		runStatisticsTaskDone(startedAt.Add(2*time.Second), "own_base", 1),
		runStatisticsTaskDone(startedAt.Add(3*time.Second), "friend_steal", 2),
		runStatisticsTaskDone(startedAt.Add(4*time.Second), "friend_help", 3),
		runStatisticsTaskDone(startedAt.Add(5*time.Second), "friend_mischief", 1),
		runStatisticsTaskDone(startedAt.Add(6*time.Second), "own_collect", 0),
		{Timestamp: startedAt.Add(7 * time.Second), Source: "auto_farm", Type: "task.done", Data: map[string]any{"taskId": "friend_steal", "ok": true, "status": "ok"}},
		{Timestamp: startedAt.Add(8 * time.Second), Source: "auto_farm", Type: "task.done", Data: map[string]any{"taskId": "own_collect", "ok": false, "status": "failed"}},
		{Timestamp: startedAt.Add(9 * time.Second), Source: "manual", Type: "task.done", Data: map[string]any{"taskId": "own_collect", "ok": true, "status": "ok", "actionCount": 1}},
	}

	stats := BuildRunStatistics(startedAt, now, events)

	if stats.DurationSeconds != 95 {
		t.Fatalf("duration = %d, want 95", stats.DurationSeconds)
	}
	if stats.Collect != 3 || stats.Farm != 1 || stats.Steal != 2 || stats.Help != 3 || stats.Mischief != 1 {
		t.Fatalf("unexpected task counts: %#v", stats)
	}
}

func TestBuildRunStatisticsNeverReturnsNegativeDuration(t *testing.T) {
	startedAt := time.Date(2026, 7, 13, 10, 0, 0, 0, time.Local)

	stats := BuildRunStatistics(startedAt, startedAt.Add(-time.Second), nil)

	if stats.DurationSeconds != 0 {
		t.Fatalf("duration = %d, want 0", stats.DurationSeconds)
	}
}

func runStatisticsTaskDone(timestamp time.Time, taskID string, actionCount int) eventbus.Event {
	return eventbus.Event{
		Timestamp: timestamp,
		Source:    "auto_farm",
		Type:      "task.done",
		Data:      map[string]any{"taskId": taskID, "ok": true, "status": "ok", "actionCount": actionCount},
	}
}
