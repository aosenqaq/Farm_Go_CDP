package social

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestNormalizeRankingRequestDefaultsAndValidates(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.Local)
	query, err := NormalizeRankingRequest("gid:10001", RankingRequest{
		Tab: "visitors", ViewMode: "ranking", DateRange: "7d", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if query.Limit != 50 || query.ViewMode != "timeline" || query.Window.Range != "7d" {
		t.Fatalf("query = %#v", query)
	}
	if query.Window.EndMS == nil || *query.Window.EndMS != now.UnixMilli() {
		t.Fatalf("window end = %#v, want %d", query.Window.EndMS, now.UnixMilli())
	}

	maximum, err := NormalizeRankingRequest("gid:10001", RankingRequest{
		Tab: "stolenByMe", ViewMode: "ranking", DateRange: "all", Limit: 100, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if maximum.Limit != 100 {
		t.Fatalf("limit = %d, want 100", maximum.Limit)
	}

	defaults, err := NormalizeRankingRequest("gid:10001", RankingRequest{Tab: "stolenByMe", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if defaults.ViewMode != "timeline" || defaults.Window.Range != "current" {
		t.Fatalf("defaults = %#v", defaults)
	}

	for _, test := range []struct {
		name    string
		account string
		request RankingRequest
	}{
		{name: "missing account", request: RankingRequest{Tab: "visitors"}},
		{name: "invalid tab", account: "gid:10001", request: RankingRequest{Tab: "bad"}},
		{name: "invalid view mode", account: "gid:10001", request: RankingRequest{Tab: "stolenByMe", ViewMode: "grid"}},
		{name: "invalid date range", account: "gid:10001", request: RankingRequest{Tab: "stolenByMe", DateRange: "forever"}},
		{name: "negative limit", account: "gid:10001", request: RankingRequest{Tab: "visitors", Limit: -1}},
		{name: "limit over maximum", account: "gid:10001", request: RankingRequest{Tab: "visitors", Limit: 101}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NormalizeRankingRequest(test.account, test.request); err == nil {
				t.Fatal("invalid request was accepted")
			}
		})
	}
}

func TestRankingCursorRejectsAnotherScope(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	query, err := NormalizeRankingRequest("gid:10001", RankingRequest{
		Tab: "stolenByMe", ViewMode: "ranking", DateRange: "all", Limit: 2, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := EncodeRankingCursor(query, RankingPageRow{
		Key: "gid:7", IdentityKey: "gid:7", TimeMS: 1234,
		Rank: 2, StealCount: 9, EventCount: 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name       string
		accountKey string
		request    RankingRequest
	}{
		{name: "account", accountKey: "gid:10002", request: RankingRequest{Tab: "stolenByMe", ViewMode: "ranking", DateRange: "all"}},
		{name: "tab", accountKey: "gid:10001", request: RankingRequest{Tab: "stolenFromMe", ViewMode: "ranking", DateRange: "all"}},
		{name: "mode", accountKey: "gid:10001", request: RankingRequest{Tab: "stolenByMe", ViewMode: "timeline", DateRange: "all"}},
		{name: "range", accountKey: "gid:10001", request: RankingRequest{Tab: "stolenByMe", ViewMode: "ranking", DateRange: "7d"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.request.Cursor = cursor
			test.request.Now = now.Add(24 * time.Hour)
			if _, err := NormalizeRankingRequest(test.accountKey, test.request); err == nil {
				t.Fatal("cursor from another scope was accepted")
			}
		})
	}
}

func TestRankingCursorRoundTripsIdenticalSortKeys(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	query, err := NormalizeRankingRequest("gid:10001", RankingRequest{
		Tab: "stolenByMe", ViewMode: "timeline", DateRange: "all", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"row-2", "row-1"} {
		encoded, err := EncodeRankingCursor(query, RankingPageRow{TimeMS: 1234, Key: key})
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := NormalizeRankingRequest("gid:10001", RankingRequest{
			Tab: "stolenByMe", ViewMode: "timeline", DateRange: "all", Cursor: encoded,
		})
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Cursor == nil || decoded.Cursor.TimeMS != 1234 || decoded.Cursor.Key != key {
			t.Fatalf("cursor = %#v, want time 1234 and key %q", decoded.Cursor, key)
		}
	}
}

func TestRankingCursorRoundTripsRankingSortFields(t *testing.T) {
	query, err := NormalizeRankingRequest("gid:10001", RankingRequest{
		Tab: "stolenFromMe", ViewMode: "ranking", DateRange: "30d",
		Now: time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	row := RankingPageRow{
		Key: "player:7", IdentityKey: "player:7", TimeMS: 1234,
		Rank: 17, StealCount: 8, EventCount: 5,
	}
	encoded, err := EncodeRankingCursor(query, row)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := NormalizeRankingRequest("gid:10001", RankingRequest{
		Tab: "stolenFromMe", ViewMode: "ranking", DateRange: "30d", Cursor: encoded,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := &RankingCursor{
		TimeMS: 1234, Key: "player:7", IdentityKey: "player:7",
		StealCount: 8, EventCount: 5, RankOffset: 17,
		StartMS: query.Window.StartMS, EndMS: *query.Window.EndMS,
	}
	if !reflect.DeepEqual(decoded.Cursor, want) {
		t.Fatalf("cursor = %#v, want %#v", decoded.Cursor, want)
	}
}

func TestRankingCursorReusesFrozenWindow(t *testing.T) {
	firstNow := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	first, err := NormalizeRankingRequest("gid:10001", RankingRequest{
		Tab: "visitors", ViewMode: "timeline", DateRange: "7d", Now: firstNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := EncodeRankingCursor(first, RankingPageRow{TimeMS: 1234, Key: "row-50"})
	if err != nil {
		t.Fatal(err)
	}

	later, err := NormalizeRankingRequest("gid:10001", RankingRequest{
		Tab: "visitors", ViewMode: "timeline", DateRange: "7d", Cursor: cursor,
		Now: firstNow.Add(48 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if later.Window.EndMS == nil || *later.Window.EndMS != *first.Window.EndMS {
		t.Fatalf("later end = %#v, want %#v", later.Window.EndMS, first.Window.EndMS)
	}
	if later.Window.StartMS == nil || first.Window.StartMS == nil || *later.Window.StartMS != *first.Window.StartMS {
		t.Fatalf("later start = %#v, want %#v", later.Window.StartMS, first.Window.StartMS)
	}
}

func TestRankingCursorRejectsMalformedPayloadAndMissingSortKeys(t *testing.T) {
	query, err := NormalizeRankingRequest("gid:10001", RankingRequest{
		Tab: "stolenByMe", ViewMode: "timeline", DateRange: "all",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NormalizeRankingRequest("gid:10001", RankingRequest{
		Tab: "stolenByMe", ViewMode: "timeline", DateRange: "all", Cursor: "not-base64!",
	}); err == nil {
		t.Fatal("malformed cursor was accepted")
	}
	if _, err := EncodeRankingCursor(query, RankingPageRow{}); err == nil {
		t.Fatal("cursor without timeline sort keys was encoded")
	}
	validTimeline, err := EncodeRankingCursor(query, RankingPageRow{Key: "row", TimeMS: 1})
	if err != nil {
		t.Fatal(err)
	}
	missingTime := rewriteCursorPayload(t, validTimeline, func(payload map[string]any) {
		delete(payload, "timeMS")
	})
	if _, err := NormalizeRankingRequest("gid:10001", RankingRequest{
		Tab: "stolenByMe", ViewMode: "timeline", DateRange: "all", Cursor: missingTime,
	}); err == nil {
		t.Fatal("cursor without a timeline timestamp was accepted")
	}

	rankingQuery, err := NormalizeRankingRequest("gid:10001", RankingRequest{
		Tab: "stolenByMe", ViewMode: "ranking", DateRange: "all",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EncodeRankingCursor(rankingQuery, RankingPageRow{Key: "row", TimeMS: 1}); err == nil {
		t.Fatal("cursor without ranking identity key was encoded")
	}
	validRanking, err := EncodeRankingCursor(rankingQuery, RankingPageRow{
		Key: "gid:7", IdentityKey: "gid:7", TimeMS: 1,
		Rank: 2, StealCount: 3, EventCount: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"timeMS", "identityKey", "stealCount", "eventCount", "rankOffset"} {
		t.Run("missing ranking "+field, func(t *testing.T) {
			missingField := rewriteCursorPayload(t, validRanking, func(payload map[string]any) {
				delete(payload, field)
			})
			if _, err := NormalizeRankingRequest("gid:10001", RankingRequest{
				Tab: "stolenByMe", ViewMode: "ranking", DateRange: "all", Cursor: missingField,
			}); err == nil {
				t.Fatalf("cursor without %s was accepted", field)
			}
		})
	}
}

func TestRankingCursorRejectsUnknownVersionAndInvalidWindow(t *testing.T) {
	query, err := NormalizeRankingRequest("gid:10001", RankingRequest{
		Tab: "stolenByMe", ViewMode: "timeline", DateRange: "7d",
		Now: time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeRankingCursor(query, RankingPageRow{Key: "row", TimeMS: 1})
	if err != nil {
		t.Fatal(err)
	}
	encoded = rewriteCursorPayload(t, encoded, func(payload map[string]any) {
		payload["version"] = float64(999)
	})
	if _, err := NormalizeRankingRequest("gid:10001", RankingRequest{
		Tab: "stolenByMe", ViewMode: "timeline", DateRange: "7d", Cursor: encoded,
	}); err == nil {
		t.Fatal("unknown cursor version was accepted")
	}

	startMS, endMS := int64(2), int64(1)
	invalidWindow := RankingQuery{
		Tab: "stolenByMe", ViewMode: "timeline", Limit: 50,
		Window:    DateRangeWindow{Range: "7d", StartMS: &startMS, EndMS: &endMS},
		ScopeHash: "scope",
	}
	if _, err := EncodeRankingCursor(invalidWindow, RankingPageRow{Key: "row", TimeMS: 1}); err == nil {
		t.Fatal("cursor with an end before its start was encoded")
	}
}

func rewriteCursorPayload(t *testing.T, encoded string, mutate func(map[string]any)) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	mutate(payload)
	raw, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}
