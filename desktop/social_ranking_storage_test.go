package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"Farm_Go/internal/farm/social"
	"Farm_Go/internal/storage"
)

func TestSocialStorageAdapterMergesReturnedStealPayloadsAndUsesStorageName(t *testing.T) {
	ctx := context.Background()
	store := openSocialRankingTestStore(t)
	rows := []storage.SocialRecordRow{
		socialRankingStealRow(t, "old", 1000, 7, "Old name", social.StealRecord{
			DisplayName: "Payload old", Items: []social.StealItem{{ItemID: 1, Name: "Carrot", Count: 2}},
		}),
		socialRankingStealRow(t, "new", 2000, 7, "Latest name", social.StealRecord{
			DisplayName: "Payload new", Items: []social.StealItem{
				{ItemID: 1, Name: "Carrot", Count: 3},
				{ItemID: 2, Name: "Corn", Count: 4},
			},
		}),
	}
	if err := store.AppendSocialStealRecords(ctx, "gid:1", rows); err != nil {
		t.Fatal(err)
	}
	endMS := int64(3000)

	page, err := (socialStorageAdapter{store: store}).QueryRankingPage(ctx, "gid:1", social.RankingQuery{
		Tab: "stolenByMe", ViewMode: "ranking", Window: social.DateRangeWindow{Range: "all", EndMS: &endMS}, Limit: 50,
	})

	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 1 || page.Rows[0].DisplayName != "Latest name" {
		t.Fatalf("rows = %#v", page.Rows)
	}
	items := page.Rows[0].Items
	if len(items) != 2 || items[0].ItemID != 1 || items[0].Count != 5 || items[1].ItemID != 2 || items[1].Count != 4 {
		t.Fatalf("merged items = %#v", items)
	}
}

func TestSocialStorageAdapterMapsTimelineStealCountFromReturnedPayload(t *testing.T) {
	ctx := context.Background()
	store := openSocialRankingTestStore(t)
	if err := store.AppendSocialStealRecords(ctx, "gid:1", []storage.SocialRecordRow{
		socialRankingStealRow(t, "timeline", 2000, 7, "Latest name", social.StealRecord{
			StealCount: 7,
			Items:      []social.StealItem{{ItemID: 1, Name: "Carrot", Count: 7}},
		}),
	}); err != nil {
		t.Fatal(err)
	}
	endMS := int64(3000)

	page, err := (socialStorageAdapter{store: store}).QueryRankingPage(ctx, "gid:1", social.RankingQuery{
		Tab: "stolenByMe", ViewMode: "timeline", Window: social.DateRangeWindow{Range: "all", EndMS: &endMS}, Limit: 50,
	})

	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 1 || page.Rows[0].StealCount != 7 {
		t.Fatalf("timeline rows = %#v", page.Rows)
	}
	if len(page.Rows[0].Items) != 1 || page.Rows[0].Items[0].Count != 7 {
		t.Fatalf("timeline items = %#v", page.Rows[0].Items)
	}
}

func TestSocialStorageAdapterVisitorRowsUseActionLabelWithoutItemsOrProtocolTarget(t *testing.T) {
	ctx := context.Background()
	store := openSocialRankingTestStore(t)
	record := social.VisitorRecord{
		ID: "visitor", PlayerID: 17, DisplayName: "Visitor", ActionType: 1,
		StealItemID: 3, StealItemName: "Corn", StealItemNum: 4,
	}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendSocialVisitorRecords(ctx, "gid:1", []storage.SocialVisitorRow{{
		ID: "visitor", OccurredAtMS: 2000, PlayerID: 17, DisplayName: "Visitor",
		ActionType: 1, StealItemNum: 4, PayloadJSON: string(raw),
	}}); err != nil {
		t.Fatal(err)
	}
	endMS := int64(3000)

	page, err := (socialStorageAdapter{store: store}).QueryRankingPage(ctx, "gid:1", social.RankingQuery{
		Tab: "visitors", ViewMode: "timeline", Window: social.DateRangeWindow{Range: "all", EndMS: &endMS}, Limit: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 1 {
		t.Fatalf("rows = %#v", page.Rows)
	}
	got := page.Rows[0]
	if got.ActionLabel != "摘取Corn4个" || len(got.Items) != 0 || got.ActionTarget != "" {
		t.Fatalf("visitor row = %#v", got)
	}
}

func TestSocialStorageAdapterParsesOnlyReturnedPayloadsAndMapsCursor(t *testing.T) {
	ctx := context.Background()
	store := openSocialRankingTestStore(t)
	if err := store.AppendSocialStealRecords(ctx, "gid:1", []storage.SocialRecordRow{
		socialRankingStealRow(t, "new", 3000, 7, "Newest", social.StealRecord{Items: []social.StealItem{{ItemID: 1, Name: "Carrot", Count: 1}}}),
		{ID: "bad", OccurredAtMS: 2000, GID: 8, DisplayName: "Bad", PayloadJSON: "{"},
	}); err != nil {
		t.Fatal(err)
	}
	endMS := int64(4000)
	query := social.RankingQuery{
		Tab: "stolenByMe", ViewMode: "timeline", Window: social.DateRangeWindow{Range: "all", EndMS: &endMS}, Limit: 1,
	}
	adapter := socialStorageAdapter{store: store}

	first, err := adapter.QueryRankingPage(ctx, "gid:1", query)
	if err != nil {
		t.Fatalf("unreturned invalid payload was parsed: %v", err)
	}
	if len(first.Rows) != 1 || !first.HasMore || first.Rows[0].Key != "new" {
		t.Fatalf("first page = %#v", first)
	}
	query.Cursor = &social.RankingCursor{TimeMS: first.Rows[0].TimeMS, Key: first.Rows[0].Key}
	if _, err := adapter.QueryRankingPage(ctx, "gid:1", query); err == nil {
		t.Fatal("returned invalid payload was accepted")
	}
}

func TestSocialRankingPageFiftyRowsMarshalsUnder100KB(t *testing.T) {
	ctx := context.Background()
	store := openSocialRankingTestStore(t)
	rows := make([]storage.SocialRecordRow, 0, 50)
	for index := 0; index < 50; index++ {
		id := fmt.Sprintf("row-%02d", index)
		rows = append(rows, socialRankingStealRow(t, id, int64(1000+index), 1000+index, "Friend "+id, social.StealRecord{
			Items: []social.StealItem{{ItemID: 1, Name: "Carrot", Count: 2}, {ItemID: 2, Name: "Corn", Count: 3}},
		}))
	}
	if err := store.AppendSocialStealRecords(ctx, "gid:1", rows); err != nil {
		t.Fatal(err)
	}
	service := social.NewService(socialStorageAdapter{store: store}, nil, social.Options{AccountKey: "gid:1"})

	page := service.Rankings(ctx, social.RankingRequest{
		Tab: "stolenByMe", ViewMode: "timeline", DateRange: "all", Limit: 50,
		Now: time.UnixMilli(5000),
	})

	if !page.OK || len(page.Rows) != 50 {
		t.Fatalf("page = %#v", page)
	}
	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) >= 100*1024 {
		t.Fatalf("50-row ranking page = %d bytes, want under 102400", len(raw))
	}
}

func openSocialRankingTestStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func socialRankingStealRow(t *testing.T, id string, occurredAtMS int64, gid int, displayName string, record social.StealRecord) storage.SocialRecordRow {
	t.Helper()
	record.ID = id
	record.GID = gid
	record.DisplayName = displayName
	record.OccurredAt = time.UnixMilli(occurredAtMS).UTC().Format(time.RFC3339Nano)
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return storage.SocialRecordRow{
		ID: id, OccurredAt: record.OccurredAt, OccurredAtMS: occurredAtMS,
		GID: gid, DisplayName: displayName, PayloadJSON: string(raw),
	}
}
