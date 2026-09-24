package automation

import "testing"

func TestSelectFriendsByWorkCountSkipsProtectedGIDs(t *testing.T) {
	friends := selectFriendsByWorkCount([]any{
		map[string]any{"gid": float64(10001), "workCounts": map[string]any{"collect": float64(2)}},
		map[string]any{"gid": float64(1184649322), "workCounts": map[string]any{"collect": float64(8)}},
		map[string]any{"gid": "1142601927", "workCounts": map[string]any{"collect": float64(5)}},
		map[string]any{"gid": float64(10002), "workCounts": map[string]any{"collect": float64(0)}},
	}, "collect")

	if len(friends) != 1 {
		t.Fatalf("friends = %#v", friends)
	}
	gid := intFromAny(friends[0]["gid"])
	if gid != 10001 {
		t.Fatalf("gid = %d, want 10001", gid)
	}
}

func TestSelectFriendsByWorkCountExceptSkipsProtectedEvenWithoutExclusionMap(t *testing.T) {
	friends := selectFriendsByWorkCountExcept(map[string]any{
		"list": []any{
			map[string]any{"gid": float64(1184649322), "workCounts": map[string]any{"help": float64(1)}},
			map[string]any{"gid": float64(10003), "workCounts": map[string]any{"help": float64(1)}},
		},
	}, "help", nil)
	if len(friends) != 1 || intFromAny(friends[0]["gid"]) != 10003 {
		t.Fatalf("friends = %#v", friends)
	}
}
