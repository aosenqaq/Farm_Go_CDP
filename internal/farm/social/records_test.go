package social

import (
	"testing"
	"time"
)

func TestStealRankingSummarizesDateFilteredRecords(t *testing.T) {
	records := []StealRecord{
		{ID: "a", GID: 10001, DisplayName: "A", OccurredAt: "2026-07-08T08:00:00Z", StealCount: 1},
		{ID: "b", GID: 10001, DisplayName: "A", OccurredAt: "2026-07-07T08:00:00Z", StealCount: 1},
		{ID: "c", GID: 10002, DisplayName: "B", OccurredAt: "2026-06-01T08:00:00Z", StealCount: 1},
	}
	window := ResolveDateRangeWindow("3d", time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC))

	ranking := SummarizeStealRanking(FilterStealRecords(records, window))

	if len(ranking) != 1 || ranking[0].GID != 10001 || ranking[0].EventCount != 2 {
		t.Fatalf("ranking = %#v", ranking)
	}
}

func TestVisitorSummaryBuildsStolenFromMeRanking(t *testing.T) {
	records := []VisitorRecord{
		{ID: "v1", PlayerID: 10001, DisplayName: "A", ActionType: 1, Time: 1783497600, StealItemNum: 3},
		{ID: "v2", PlayerID: 10001, DisplayName: "A", ActionType: 1, Time: 1783497700, StealItemNum: 2},
		{ID: "v3", PlayerID: 10002, DisplayName: "B", ActionType: 2, Time: 1783497800, Count: 1},
	}
	summary := SummarizeVisitorRankings(records)
	if len(summary.StolenFromMe) != 1 || summary.StolenFromMe[0].StealCount != 5 {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.VisitorRecords[0].ActionLabel == "" {
		t.Fatalf("visitor action label should be normalized: %#v", summary.VisitorRecords[0])
	}
}

func TestBuildStealRecordFallsBackToInspectLandItems(t *testing.T) {
	record := BuildStealRecord(10001, map[string]any{
		"friend": map[string]any{"displayName": "A"},
		"lands": []any{
			map[string]any{"landId": float64(3), "plantInfo": map[string]any{"id": float64(2001), "name": "百香果"}},
			map[string]any{"landId": float64(9), "plantInfo": map[string]any{"id": float64(2001), "name": "百香果"}},
			map[string]any{"landId": float64(10), "plantInfo": map[string]any{"id": float64(2002), "name": "葡萄"}},
		},
	}, []int{3, 9}, map[string]any{"ok": true}, StealRecordOptions{Action: "steal"})

	if record.DisplayName != "A" || record.Action != "steal" {
		t.Fatalf("record identity/action = %#v", record)
	}
	if len(record.Items) != 1 || record.Items[0].ItemID != 2001 || record.Items[0].Name != "百香果" || record.Items[0].Count != 2 {
		t.Fatalf("fallback items = %#v", record.Items)
	}
	if len(record.Items[0].LandIDs) != 2 || record.Items[0].LandIDs[0] != 3 || record.Items[0].LandIDs[1] != 9 {
		t.Fatalf("fallback item lands = %#v", record.Items[0].LandIDs)
	}
}

func TestNormalizeRankingDateRangeDefaultsToCurrent(t *testing.T) {
	if got := NormalizeRankingDateRange("weird"); got != "current" {
		t.Fatalf("range = %q, want current", got)
	}
	if got := NormalizeRankingDateRange("today"); got != "current" {
		t.Fatalf("today should normalize to current, got %q", got)
	}
}
