package storage

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestMysteryShopPurchaseRecordsAreScopedSortedAndLimited(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	for index := 0; index < 51; index++ {
		record := MysteryShopPurchaseRecord{
			ID:           fmt.Sprintf("account-a-%02d", index),
			OccurredAt:   time.Date(2026, 7, 26, 10, index, 0, 0, time.UTC).Format(time.RFC3339Nano),
			GoodsID:      8800 + index,
			ItemID:       31001,
			ItemName:     "高级化肥",
			Count:        2,
			UnitPrice:    80,
			CurrencyID:   1005,
			CurrencyName: "金豆豆",
			Discount:     50,
			Payload:      map[string]any{"cardId": index},
		}
		if err := store.AppendMysteryShopPurchaseRecord(ctx, "gid:10001", record); err != nil {
			t.Fatalf("append account record %d: %v", index, err)
		}
	}
	if err := store.AppendMysteryShopPurchaseRecord(ctx, "gid:10002", MysteryShopPurchaseRecord{
		ID: "other-account", OccurredAt: "2026-07-27T00:00:00Z", ItemName: "其他商品",
	}); err != nil {
		t.Fatalf("append other account record: %v", err)
	}

	records, err := store.ListMysteryShopPurchaseRecords(ctx, "gid:10001", 100)
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(records) != 50 {
		t.Fatalf("record count = %d, want 50", len(records))
	}
	if records[0].ID != "account-a-50" || records[len(records)-1].ID != "account-a-01" {
		t.Fatalf("records not newest-first or scoped: first=%q last=%q", records[0].ID, records[len(records)-1].ID)
	}
	if records[0].CurrencyName != "金豆豆" || records[0].Payload["cardId"] != float64(50) {
		t.Fatalf("record did not round trip: %#v", records[0])
	}
}
