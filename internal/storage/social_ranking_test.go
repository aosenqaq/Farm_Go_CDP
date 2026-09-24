package storage

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const (
	socialRankingStartMS = int64(1784016000000)
	socialRankingEndMS   = int64(1784620800000)
)

func TestQuerySocialRankingTimelinePages(t *testing.T) {
	ctx := context.Background()
	store := openSocialRankingTestStore(t, ctx)
	seedSocialRankingTimeline(t, ctx, store)

	tests := []struct {
		name       string
		tab        string
		wantFirst  []string
		wantSecond []string
	}{
		{name: "stolen by me", tab: "stolenByMe", wantFirst: []string{"s3", "s2"}, wantSecond: []string{"s1"}},
		{name: "stolen from me", tab: "stolenFromMe", wantFirst: []string{"v3", "v2"}, wantSecond: []string{"v1"}},
		{name: "visitors", tab: "visitors", wantFirst: []string{"v3", "v2"}, wantSecond: []string{"v4", "v1"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			startMS, endMS := socialRankingStartMS, socialRankingEndMS
			query := SocialRankingQuery{
				AccountKey: "  gid:10001  ", Tab: test.tab, ViewMode: "timeline",
				StartMS: &startMS, EndMS: &endMS, Limit: 2,
			}
			first, err := store.QuerySocialRankingPage(ctx, query)
			if err != nil {
				t.Fatal(err)
			}
			if got := socialRankingRowKeys(first.Rows); !reflect.DeepEqual(got, test.wantFirst) {
				t.Fatalf("first page = %v, want %v", got, test.wantFirst)
			}
			if !first.HasMore {
				t.Fatalf("first page hasMore = false: %#v", first)
			}
			if len(first.Rows) != 2 || first.Rows[0].PayloadJSON == "" || first.Rows[1].PayloadJSON == "" {
				t.Fatalf("first page payloads = %#v", first.Rows)
			}

			last := first.Rows[len(first.Rows)-1]
			query.Cursor = &SocialRankingCursor{TimeMS: last.TimeMS, Key: last.Key}
			second, err := store.QuerySocialRankingPage(ctx, query)
			if err != nil {
				t.Fatal(err)
			}
			if got := socialRankingRowKeys(second.Rows); !reflect.DeepEqual(got, test.wantSecond) {
				t.Fatalf("second page = %v, want %v", got, test.wantSecond)
			}
			if second.HasMore {
				t.Fatalf("second page hasMore = true: %#v", second)
			}

			seen := map[string]bool{}
			for _, key := range append(socialRankingRowKeys(first.Rows), socialRankingRowKeys(second.Rows)...) {
				if seen[key] {
					t.Fatalf("duplicate key across pages: %q", key)
				}
				seen[key] = true
			}
		})
	}
}

func TestQuerySocialRankingTimelineRows(t *testing.T) {
	ctx := context.Background()
	store := openSocialRankingTestStore(t, ctx)
	seedSocialRankingTimeline(t, ctx, store)
	startMS, endMS := socialRankingStartMS, socialRankingEndMS

	stolen, err := store.QuerySocialRankingPage(ctx, SocialRankingQuery{
		AccountKey: "gid:10001", Tab: "stolenByMe", ViewMode: "timeline",
		StartMS: &startMS, EndMS: &endMS, Limit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := stolen.Rows[0]; got.Kind != "stealRecord" || got.Key != "s3" || got.IdentityKey != "gid:11" || got.DisplayName != "Alpha" || got.TimeMS != endMS || got.PayloadJSON != `{"id":"s3"}` {
		t.Fatalf("steal row = %#v", got)
	}
	if got := stolen.Rows[1]; got.IdentityKey != "name:No ID" {
		t.Fatalf("fallback steal identity = %#v", got)
	}

	visitors, err := store.QuerySocialRankingPage(ctx, SocialRankingQuery{
		AccountKey: "gid:10001", Tab: "visitors", ViewMode: "timeline",
		StartMS: &startMS, EndMS: &endMS, Limit: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := visitors.Rows[2]; got.Kind != "visitorRecord" || got.Key != "v4" || got.IdentityKey != "name:Passerby" || got.ActionType != 2 || got.PayloadJSON != `{"id":"v4"}` {
		t.Fatalf("visitor row = %#v", got)
	}
	if got := visitors.Rows[0]; got.ActionTarget != "" {
		t.Fatalf("ordinary visitor row has protocol target: %#v", got)
	}

	stolenFromMe, err := store.QuerySocialRankingPage(ctx, SocialRankingQuery{
		AccountKey: "gid:10001", Tab: "stolenFromMe", ViewMode: "timeline",
		StartMS: &startMS, EndMS: &endMS, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := stolenFromMe.Rows[0]; got.ActionTarget != "303" {
		t.Fatalf("stolen-from-me row protocol target = %#v", got)
	}
}

func TestSocialRankingSummaryUsesRangeAndAccountIsolation(t *testing.T) {
	ctx := context.Background()
	store := openSocialRankingTestStore(t, ctx)
	seedSocialRankingTimeline(t, ctx, store)
	startMS, endMS := socialRankingStartMS, socialRankingEndMS

	page, err := store.QuerySocialRankingPage(ctx, SocialRankingQuery{
		AccountKey: " gid:10001 ", Tab: "stolenFromMe", ViewMode: "timeline",
		StartMS: &startMS, EndMS: &endMS, Limit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := SocialRankingSummary{
		VisitorCount: 4, StolenFromMeCount: 3,
		StolenByMeCount: 2, StolenByMeRecordCount: 3,
	}
	if page.Summary != want {
		t.Fatalf("summary = %#v, want %#v", page.Summary, want)
	}
}

func TestSocialRankingQueryPlanUsesCursorIndexes(t *testing.T) {
	ctx := context.Background()
	store := openSocialRankingTestStore(t, ctx)
	seedSocialRankingTimeline(t, ctx, store)
	if _, err := store.db.ExecContext(ctx, "ANALYZE"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		query     SocialRankingQuery
		wantIndex string
	}{
		{
			name: "all-time steal timeline",
			query: SocialRankingQuery{
				AccountKey: "gid:10001", Tab: "stolenByMe", ViewMode: "timeline", EndMS: int64Ptr(socialRankingEndMS), Limit: 50,
			},
			wantIndex: "idx_social_steal_records_page",
		},
		{
			name: "bounded stolen-from-me cursor timeline",
			query: SocialRankingQuery{
				AccountKey: "gid:10001", Tab: "stolenFromMe", ViewMode: "timeline",
				StartMS: int64Ptr(socialRankingStartMS), EndMS: int64Ptr(socialRankingEndMS),
				Cursor: &SocialRankingCursor{TimeMS: socialRankingEndMS, Key: "v2"}, Limit: 50,
			},
			wantIndex: "idx_social_visitor_records_action_page",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query, args, err := buildSocialTimelineQuery(test.query)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(strings.ToUpper(query), "IS NULL") {
				t.Fatalf("query contains nullable-bound predicate: %s", query)
			}
			if strings.Contains(strings.ToUpper(query), "INDEXED BY") {
				t.Fatalf("query hardcodes an index: %s", query)
			}
			if test.query.StartMS == nil && strings.Contains(query, "occurred_at_ms >=") {
				t.Fatalf("query contains omitted start bound: %s", query)
			}
			if test.query.Cursor == nil && strings.Contains(query, "id <") {
				t.Fatalf("query contains omitted cursor predicate: %s", query)
			}

			rows, err := store.db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+query, args...)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var plans []string
			for rows.Next() {
				var id, parent, unused int
				var detail string
				if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
					t.Fatal(err)
				}
				plans = append(plans, detail)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.Join(plans, "\n"), test.wantIndex) {
				t.Fatalf("query plan = %q, want index %q", plans, test.wantIndex)
			}
		})
	}
}

func TestSocialRankingGroupedQueryPlanUsesExpressionIndexes(t *testing.T) {
	ctx := context.Background()
	store := openSocialRankingTestStore(t, ctx)
	seedSocialRankingTimeline(t, ctx, store)
	if _, err := store.db.ExecContext(ctx, "ANALYZE"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		query     SocialRankingQuery
		wantIndex string
	}{
		{
			name: "steal ranking",
			query: SocialRankingQuery{
				AccountKey: "gid:10001", Tab: "stolenByMe", ViewMode: "ranking", Limit: 50,
			},
			wantIndex: "idx_social_steal_records_ranking",
		},
		{
			name: "stolen ranking",
			query: SocialRankingQuery{
				AccountKey: "gid:10001", Tab: "stolenFromMe", ViewMode: "ranking", Limit: 50,
			},
			wantIndex: "idx_social_visitor_records_ranking",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			statement, args, _, err := buildSocialGroupedRankingQuery(test.query)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(strings.ToUpper(statement), "INDEXED BY") {
				t.Fatalf("query hardcodes an index: %s", statement)
			}
			rows, err := store.db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+statement, args...)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var plans []string
			for rows.Next() {
				var id, parent, unused int
				var detail string
				if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
					t.Fatal(err)
				}
				plans = append(plans, detail)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.Join(plans, "\n"), test.wantIndex) {
				t.Fatalf("query plan = %q, want index %q", plans, test.wantIndex)
			}
		})
	}
}

func TestQuerySocialRankingGroupedStolenByMePages(t *testing.T) {
	ctx := context.Background()
	store := openSocialRankingTestStore(t, ctx)
	seedSocialRankingGroupedSteals(t, ctx, store)
	startMS, endMS := socialRankingStartMS, socialRankingEndMS
	query := SocialRankingQuery{
		AccountKey: "gid:10001", Tab: "stolenByMe", ViewMode: "ranking",
		StartMS: &startMS, EndMS: &endMS, Limit: 2,
	}

	first, err := store.QuerySocialRankingPage(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := socialRankingIdentityKeys(first.Rows), []string{"gid:11", "gid:22"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first page identities = %v, want %v", got, want)
	}
	if !first.HasMore {
		t.Fatalf("first page hasMore = false: %#v", first)
	}
	for index, row := range first.Rows {
		if row.Kind != "stealRanking" || row.Key != row.IdentityKey || row.Rank != index+1 || row.StealCount != 2 || row.EventCount != 2 {
			t.Fatalf("first page row %d = %#v", index, row)
		}
		if len(row.PayloadJSONs) != 2 || row.PayloadJSON != "" {
			t.Fatalf("first page detail payloads %d = %#v", index, row)
		}
	}

	last := first.Rows[len(first.Rows)-1]
	query.Cursor = &SocialRankingCursor{
		TimeMS: last.TimeMS, IdentityKey: last.IdentityKey,
		StealCount: last.StealCount, EventCount: last.EventCount, RankOffset: len(first.Rows),
	}
	second, err := store.QuerySocialRankingPage(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := socialRankingIdentityKeys(second.Rows), []string{"name:Cedar", "gid:44"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second page identities = %v, want %v", got, want)
	}
	if second.HasMore {
		t.Fatalf("second page hasMore = true: %#v", second)
	}
	for index, row := range second.Rows {
		if row.Rank != index+3 || row.StealCount != 1 || row.EventCount != 1 || len(row.PayloadJSONs) != 1 {
			t.Fatalf("second page row %d = %#v", index, row)
		}
	}
}

func TestQuerySocialRankingGroupedStolenFromMeUsesItemSumAndMixedCursor(t *testing.T) {
	ctx := context.Background()
	store := openSocialRankingTestStore(t, ctx)
	seedSocialRankingGroupedVisitors(t, ctx, store)
	startMS, endMS := socialRankingStartMS, socialRankingEndMS
	query := SocialRankingQuery{
		AccountKey: "gid:10001", Tab: "stolenFromMe", ViewMode: "ranking",
		StartMS: &startMS, EndMS: &endMS, Limit: 3,
	}

	first, err := store.QuerySocialRankingPage(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := socialRankingIdentityKeys(first.Rows), []string{"player:101", "player:202", "player:303"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first page identities = %v, want %v", got, want)
	}
	if got, want := [][2]int{{first.Rows[0].StealCount, first.Rows[0].EventCount}, {first.Rows[1].StealCount, first.Rows[1].EventCount}}, [][2]int{{5, 2}, {5, 1}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first page aggregates = %v, want %v", got, want)
	}
	if first.Rows[0].Rank != 1 || first.Rows[1].Rank != 2 || first.Rows[2].Rank != 3 || !first.HasMore {
		t.Fatalf("first page rank state = %#v", first)
	}

	last := first.Rows[len(first.Rows)-1]
	query.Cursor = &SocialRankingCursor{
		TimeMS: last.TimeMS, IdentityKey: last.IdentityKey,
		StealCount: last.StealCount, EventCount: last.EventCount, RankOffset: 3,
	}
	second, err := store.QuerySocialRankingPage(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := socialRankingIdentityKeys(second.Rows), []string{"player:404"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second page equal-sort identities = %v, want %v", got, want)
	}
	for index, row := range second.Rows {
		if row.Kind != "stolenRanking" || row.Key != row.IdentityKey || row.Rank != index+4 || row.StealCount != 4 || row.EventCount != 2 {
			t.Fatalf("second page row %d = %#v", index, row)
		}
		if row.PayloadJSON != "" || len(row.PayloadJSONs) != 0 {
			t.Fatalf("visitor ranking row contains detail payload: %#v", row)
		}
	}
}

func TestQuerySocialRankingGroupedStolenByMeUsesLatestDisplayName(t *testing.T) {
	ctx := context.Background()
	store := openSocialRankingTestStore(t, ctx)
	if err := store.AppendSocialStealRecords(ctx, "gid:10001", []SocialRecordRow{
		{ID: "new", OccurredAtMS: socialRankingEndMS, GID: 11, DisplayName: "Alpha New", PayloadJSON: `{}`},
		{ID: "old", OccurredAtMS: socialRankingEndMS - 1, GID: 11, DisplayName: "Zulu Old", PayloadJSON: `{}`},
	}); err != nil {
		t.Fatal(err)
	}

	page, err := store.QuerySocialRankingPage(ctx, SocialRankingQuery{
		AccountKey: "gid:10001", Tab: "stolenByMe", ViewMode: "ranking", Limit: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 1 || page.Rows[0].DisplayName != "Alpha New" {
		t.Fatalf("ranking rows = %#v, want latest display name Alpha New", page.Rows)
	}
}

func TestQuerySocialRankingGroupedStolenFromMeUsesLatestDisplayName(t *testing.T) {
	ctx := context.Background()
	store := openSocialRankingTestStore(t, ctx)
	if err := store.AppendSocialVisitorRecords(ctx, "gid:10001", []SocialVisitorRow{
		{ID: "new", OccurredAtMS: socialRankingEndMS, PlayerID: 101, DisplayName: "Alpha New", ActionType: 1, StealItemNum: 1, PayloadJSON: `{}`},
		{ID: "old", OccurredAtMS: socialRankingEndMS - 1, PlayerID: 101, DisplayName: "Zulu Old", ActionType: 1, StealItemNum: 1, PayloadJSON: `{}`},
	}); err != nil {
		t.Fatal(err)
	}

	page, err := store.QuerySocialRankingPage(ctx, SocialRankingQuery{
		AccountKey: "gid:10001", Tab: "stolenFromMe", ViewMode: "ranking", Limit: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 1 || page.Rows[0].DisplayName != "Alpha New" {
		t.Fatalf("ranking rows = %#v, want latest display name Alpha New", page.Rows)
	}
}

func TestQuerySocialRankingGroupedVisitorsIsUnsupported(t *testing.T) {
	ctx := context.Background()
	store := openSocialRankingTestStore(t, ctx)
	_, err := store.QuerySocialRankingPage(ctx, SocialRankingQuery{
		AccountKey: "gid:10001", Tab: "visitors", ViewMode: "ranking", Limit: 2,
	})
	if err == nil {
		t.Fatal("visitor ranking mode was accepted")
	}
}

func TestSocialRankingPagePayloadsContainOnlyReturnedIdentity(t *testing.T) {
	ctx := context.Background()
	store := openSocialRankingTestStore(t, ctx)
	seedSocialRankingGroupedSteals(t, ctx, store)
	page, err := store.QuerySocialRankingPage(ctx, SocialRankingQuery{
		AccountKey: "gid:10001", Tab: "stolenByMe", ViewMode: "ranking", Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 1 || page.Rows[0].IdentityKey != "gid:11" {
		t.Fatalf("page rows = %#v", page.Rows)
	}
	got := append([]string(nil), page.Rows[0].PayloadJSONs...)
	sort.Strings(got)
	want := []string{`{"items":[{"id":1,"num":1}]}`, `{"items":[{"id":2,"num":1}]}`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("returned detail payloads = %#v, want %#v", got, want)
	}
	for _, payload := range got {
		if strings.Contains(payload, "friend-not-on-page") {
			t.Fatalf("page loaded sentinel payload %q", payload)
		}
	}
}

func openSocialRankingTestStore(t *testing.T, ctx context.Context) *Store {
	t.Helper()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func seedSocialRankingTimeline(t *testing.T, ctx context.Context, store *Store) {
	t.Helper()
	stealRows := []SocialRecordRow{
		{ID: "s0", OccurredAtMS: socialRankingStartMS - 1, GID: 99, DisplayName: "Too old", PayloadJSON: `{"id":"s0"}`},
		{ID: "s1", OccurredAtMS: socialRankingStartMS, GID: 11, DisplayName: "Alpha", PayloadJSON: `{"id":"s1"}`},
		{ID: "s2", OccurredAtMS: socialRankingEndMS, DisplayName: "No ID", PayloadJSON: `{"id":"s2"}`},
		{ID: "s3", OccurredAtMS: socialRankingEndMS, GID: 11, DisplayName: "Alpha", PayloadJSON: `{"id":"s3"}`},
	}
	if err := store.AppendSocialStealRecords(ctx, "gid:10001", stealRows); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendSocialStealRecords(ctx, "gid:10002", []SocialRecordRow{{
		ID: "s9", OccurredAtMS: socialRankingEndMS, GID: 90, DisplayName: "Other account", PayloadJSON: `{"id":"s9"}`,
	}}); err != nil {
		t.Fatal(err)
	}

	visitorRows := []SocialVisitorRow{
		{ID: "v0", OccurredAtMS: socialRankingStartMS - 1, PlayerID: 9, DisplayName: "Too old", ActionType: 1, PayloadJSON: `{"id":"v0"}`},
		{ID: "v1", OccurredAtMS: socialRankingStartMS, PlayerID: 101, DisplayName: "One", ActionType: 1, StealItemNum: 1, PayloadJSON: `{"id":"v1"}`},
		{ID: "v4", OccurredAtMS: socialRankingStartMS + 1, DisplayName: "Passerby", ActionType: 2, PayloadJSON: `{"id":"v4"}`},
		{ID: "v2", OccurredAtMS: socialRankingEndMS, PlayerID: 202, DisplayName: "Two", ActionType: 1, StealItemNum: 2, PayloadJSON: `{"id":"v2"}`},
		{ID: "v3", OccurredAtMS: socialRankingEndMS, PlayerID: 303, DisplayName: "Three", ActionType: 1, StealItemNum: 3, PayloadJSON: `{"id":"v3"}`},
	}
	if err := store.AppendSocialVisitorRecords(ctx, "gid:10001", visitorRows); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendSocialVisitorRecords(ctx, "gid:10002", []SocialVisitorRow{{
		ID: "v9", OccurredAtMS: socialRankingEndMS, PlayerID: 909, DisplayName: "Other account", ActionType: 1, PayloadJSON: `{"id":"v9"}`,
	}}); err != nil {
		t.Fatal(err)
	}
}

func seedSocialRankingGroupedSteals(t *testing.T, ctx context.Context, store *Store) {
	t.Helper()
	rows := []SocialRecordRow{
		{ID: "a1", OccurredAtMS: socialRankingEndMS, GID: 11, DisplayName: "Alpha", PayloadJSON: `{"items":[{"id":1,"num":1}]}`},
		{ID: "a2", OccurredAtMS: socialRankingEndMS - 1, GID: 11, DisplayName: "Alpha", PayloadJSON: `{"items":[{"id":2,"num":1}]}`},
		{ID: "b1", OccurredAtMS: socialRankingEndMS - 10, GID: 22, DisplayName: "Bravo", PayloadJSON: `{"items":[{"id":3,"num":1}]}`},
		{ID: "b2", OccurredAtMS: socialRankingEndMS - 20, GID: 22, DisplayName: "Bravo", PayloadJSON: `{"items":[{"id":4,"num":1}]}`},
		{ID: "c1", OccurredAtMS: socialRankingEndMS - 30, DisplayName: " Cedar ", PayloadJSON: `{"items":[{"id":5,"num":1}]}`},
		{ID: "d1", OccurredAtMS: socialRankingEndMS - 40, GID: 44, DisplayName: "Delta", PayloadJSON: `{"marker":"friend-not-on-page"}`},
		{ID: "old", OccurredAtMS: socialRankingStartMS - 1, GID: 55, DisplayName: "Old", PayloadJSON: `{"marker":"out-of-window"}`},
	}
	if err := store.AppendSocialStealRecords(ctx, "gid:10001", rows); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendSocialStealRecords(ctx, "gid:other", []SocialRecordRow{{
		ID: "other", OccurredAtMS: socialRankingEndMS, GID: 66, DisplayName: "Other", PayloadJSON: `{"marker":"other-account"}`,
	}}); err != nil {
		t.Fatal(err)
	}
}

func seedSocialRankingGroupedVisitors(t *testing.T, ctx context.Context, store *Store) {
	t.Helper()
	rows := []SocialVisitorRow{
		{ID: "p101a", OccurredAtMS: socialRankingEndMS, PlayerID: 101, DisplayName: "One", ActionType: 1, StealItemNum: 3, PayloadJSON: `{}`},
		{ID: "p101b", OccurredAtMS: socialRankingEndMS - 1, PlayerID: 101, DisplayName: "One", ActionType: 1, StealItemNum: 2, PayloadJSON: `{}`},
		{ID: "p202", OccurredAtMS: socialRankingEndMS, PlayerID: 202, DisplayName: "Two", ActionType: 1, StealItemNum: 5, PayloadJSON: `{}`},
		{ID: "p303a", OccurredAtMS: socialRankingEndMS, PlayerID: 303, DisplayName: "Three", ActionType: 1, StealItemNum: 2, PayloadJSON: `{}`},
		{ID: "p303b", OccurredAtMS: socialRankingEndMS - 2, PlayerID: 303, DisplayName: "Three", ActionType: 1, StealItemNum: 2, PayloadJSON: `{}`},
		{ID: "p404a", OccurredAtMS: socialRankingEndMS, PlayerID: 404, DisplayName: "Four", ActionType: 1, StealItemNum: 1, PayloadJSON: `{}`},
		{ID: "p404b", OccurredAtMS: socialRankingEndMS - 3, PlayerID: 404, DisplayName: "Four", ActionType: 1, StealItemNum: 3, PayloadJSON: `{}`},
		{ID: "visit", OccurredAtMS: socialRankingEndMS, PlayerID: 505, DisplayName: "Visit", ActionType: 2, StealItemNum: 99, PayloadJSON: `{}`},
		{ID: "old", OccurredAtMS: socialRankingStartMS - 1, PlayerID: 606, DisplayName: "Old", ActionType: 1, StealItemNum: 99, PayloadJSON: `{}`},
	}
	if err := store.AppendSocialVisitorRecords(ctx, "gid:10001", rows); err != nil {
		t.Fatal(err)
	}
}

func socialRankingRowKeys(rows []SocialRankingPageRow) []string {
	keys := make([]string, len(rows))
	for i := range rows {
		keys[i] = rows[i].Key
	}
	return keys
}

func socialRankingIdentityKeys(rows []SocialRankingPageRow) []string {
	keys := make([]string, len(rows))
	for i := range rows {
		keys[i] = rows[i].IdentityKey
	}
	return keys
}

func int64Ptr(value int64) *int64 {
	return &value
}
