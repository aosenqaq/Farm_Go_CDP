package storage

import (
	"context"
	"testing"
)

func TestWarehouseSellRecordsAreScopedByAccountAndDate(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	first := WarehouseSellRecord{
		ID:          "manual-1",
		DateKey:     "2026-07-09",
		OccurredAt:  "2026-07-09T10:00:00Z",
		Mode:        "manual",
		ItemKinds:   1,
		TotalCount:  3,
		TotalAmount: 720,
		Items:       []WarehouseSellRecordItem{{ItemID: 41221, Name: "青梅", Count: 3, Amount: 720}},
	}
	second := WarehouseSellRecord{
		ID:          "auto-1",
		DateKey:     "2026-07-08",
		OccurredAt:  "2026-07-08T10:00:00Z",
		Mode:        "auto",
		ItemKinds:   1,
		TotalCount:  1,
		TotalAmount: 2,
		Items:       []WarehouseSellRecordItem{{ItemID: 40002, Name: "白萝卜", Count: 1, Amount: 2}},
	}
	otherAccount := WarehouseSellRecord{
		ID:          "other-1",
		DateKey:     "2026-07-09",
		OccurredAt:  "2026-07-09T11:00:00Z",
		Mode:        "manual",
		ItemKinds:   1,
		TotalCount:  99,
		TotalAmount: 99,
	}

	if err := store.AppendWarehouseSellRecord(ctx, "gid:10001", first); err != nil {
		t.Fatalf("append first record: %v", err)
	}
	if err := store.AppendWarehouseSellRecord(ctx, "gid:10001", second); err != nil {
		t.Fatalf("append second record: %v", err)
	}
	if err := store.AppendWarehouseSellRecord(ctx, "gid:10002", otherAccount); err != nil {
		t.Fatalf("append other account record: %v", err)
	}

	records, err := store.ListWarehouseSellRecords(ctx, "gid:10001", "2026-07-09", 50)
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one filtered record, got %#v", records)
	}
	if records[0].ID != "manual-1" || records[0].Mode != "manual" || records[0].TotalAmount != 720 {
		t.Fatalf("unexpected filtered record: %#v", records[0])
	}
	if len(records[0].Items) != 1 || records[0].Items[0].Name != "青梅" {
		t.Fatalf("record items did not round trip: %#v", records[0].Items)
	}

	all, err := store.ListWarehouseSellRecords(ctx, "gid:10001", "", 50)
	if err != nil {
		t.Fatalf("list all records: %v", err)
	}
	if len(all) != 2 || all[0].ID != "manual-1" || all[1].ID != "auto-1" {
		t.Fatalf("expected account records newest first, got %#v", all)
	}
}

func TestWarehouseSellAmountsByDateAreScopedAndSummed(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	entries := []struct {
		account string
		record  WarehouseSellRecord
	}{
		{"gid:10001", WarehouseSellRecord{ID: "a-1", DateKey: "2026-07-21", OccurredAt: "2026-07-21T10:00:00Z", TotalAmount: 120}},
		{"gid:10001", WarehouseSellRecord{ID: "a-2", DateKey: "2026-07-21", OccurredAt: "2026-07-21T11:00:00Z", TotalAmount: 80}},
		{"gid:10001", WarehouseSellRecord{ID: "a-3", DateKey: "2026-07-22", OccurredAt: "2026-07-22T10:00:00Z", TotalAmount: 300}},
		{"gid:10001", WarehouseSellRecord{ID: "outside", DateKey: "2026-07-19", OccurredAt: "2026-07-19T10:00:00Z", TotalAmount: 999}},
		{"gid:10002", WarehouseSellRecord{ID: "other", DateKey: "2026-07-21", OccurredAt: "2026-07-21T12:00:00Z", TotalAmount: 777}},
	}
	for _, entry := range entries {
		if err := store.AppendWarehouseSellRecord(ctx, entry.account, entry.record); err != nil {
			t.Fatalf("append %s: %v", entry.record.ID, err)
		}
	}

	totals, err := store.SumWarehouseSellAmountsByDate(ctx, "gid:10001", "2026-07-20", "2026-07-22")
	if err != nil {
		t.Fatalf("sum sale amounts: %v", err)
	}
	if len(totals) != 2 || totals["2026-07-21"] != 200 || totals["2026-07-22"] != 300 {
		t.Fatalf("unexpected daily totals: %#v", totals)
	}
}
