package farm

import (
	"time"

	"Farm_Go/internal/eventbus"
)

// RunStatistics summarizes successful automation tasks since the app started.
type RunStatistics struct {
	StartedAt       string `json:"startedAt"`
	DurationSeconds int64  `json:"durationSeconds"`
	Collect         int    `json:"collect"`
	Farm            int    `json:"farm"`
	Steal           int    `json:"steal"`
	Help            int    `json:"help"`
	Mischief        int    `json:"mischief"`
	SaleEstimate    int64  `json:"saleEstimate"`
	EstimateReady   bool   `json:"estimateReady"`
}

func BuildRunStatistics(startedAt, now time.Time, events []eventbus.Event) RunStatistics {
	duration := int64(now.Sub(startedAt).Seconds())
	if duration < 0 {
		duration = 0
	}
	stats := RunStatistics{
		StartedAt:       startedAt.Format(time.RFC3339),
		DurationSeconds: duration,
	}
	for _, event := range events {
		if !isSuccessfulRunStatisticsTaskDone(event) {
			continue
		}
		actionCount := runStatisticsActionCount(event)
		if actionCount <= 0 {
			continue
		}
		switch runStatisticsTaskID(event) {
		case "own_collect", "own_fertilizer":
			stats.Collect += actionCount
		case "own_base":
			stats.Farm += actionCount
		case "friend_steal":
			stats.Steal += actionCount
		case "friend_help":
			stats.Help += actionCount
		case "friend_mischief":
			stats.Mischief += actionCount
		}
	}
	return stats
}

func isSuccessfulRunStatisticsTaskDone(event eventbus.Event) bool {
	return event.Source == "auto_farm" &&
		event.Type == "task.done" &&
		boolFromMap(event.Data, "ok") &&
		stringFromMap(event.Data, "status") == "ok"
}

func runStatisticsTaskID(event eventbus.Event) string {
	return stringFromMap(event.Data, "taskId")
}

func runStatisticsActionCount(event eventbus.Event) int {
	count := intFromMap(event.Data, "actionCount")
	if count < 0 {
		return 0
	}
	return count
}
