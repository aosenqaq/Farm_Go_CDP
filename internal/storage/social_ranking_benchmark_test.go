package storage

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkSocialRankingPage100K(b *testing.B) {
	ctx := context.Background()
	dir := b.TempDir()
	store, err := Open(ctx, dir)
	if err != nil {
		b.Fatal(err)
	}
	seedSocialRankingBenchmark(b, ctx, store, 100_000)
	if err := store.Close(); err != nil {
		b.Fatal(err)
	}
	store, err = Open(ctx, dir)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = store.Close() })

	const dayMS = int64(24 * 60 * 60 * 1000)
	endMS := int64(1784620800000)
	cases := []struct {
		name    string
		startMS *int64
	}{
		{name: "current", startMS: socialRankingBenchmarkInt64Ptr(endMS - dayMS)},
		{name: "30d", startMS: socialRankingBenchmarkInt64Ptr(endMS - 30*dayMS)},
		{name: "all"},
	}
	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			query := SocialRankingQuery{
				AccountKey: "gid:benchmark", Tab: "stolenByMe", ViewMode: "ranking",
				StartMS: test.startMS, EndMS: &endMS, Limit: 50,
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				page, err := store.QuerySocialRankingPage(ctx, query)
				if err != nil {
					b.Fatal(err)
				}
				if len(page.Rows) != 50 {
					b.Fatalf("page rows = %d, want 50", len(page.Rows))
				}
			}
		})
	}
}

func seedSocialRankingBenchmark(b *testing.B, ctx context.Context, store *Store, count int) {
	b.Helper()
	const (
		batchSize = 5_000
		dayMS     = int64(24 * 60 * 60 * 1000)
		endMS     = int64(1784620800000)
	)
	for batchStart := 0; batchStart < count; batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > count {
			batchEnd = count
		}
		tx, err := store.db.BeginTx(ctx, nil)
		if err != nil {
			b.Fatal(err)
		}
		stealStatement, err := tx.PrepareContext(ctx, `
			INSERT INTO social_steal_records
				(id, account_key, date_key, occurred_at, occurred_at_ms, gid, display_name, payload_json)
			VALUES (?, 'gid:benchmark', '', '', ?, ?, ?, ?)`)
		if err != nil {
			_ = tx.Rollback()
			b.Fatal(err)
		}
		visitorStatement, err := tx.PrepareContext(ctx, `
			INSERT INTO social_visitor_records
				(id, account_key, occurred_at_ms, player_id, display_name, action_type, steal_item_num, payload_json)
			VALUES (?, 'gid:benchmark', ?, ?, ?, 1, ?, '{}')`)
		if err != nil {
			_ = stealStatement.Close()
			_ = tx.Rollback()
			b.Fatal(err)
		}
		for i := batchStart; i < batchEnd; i++ {
			stealIdentity := i%2_000 + 1
			visitorIdentity := i%2_500 + 1
			occurredAtMS := endMS - int64((i/2_000)%60)*dayMS - int64(i%1_000)
			if _, err := stealStatement.ExecContext(
				ctx, fmt.Sprintf("s-%06d", i), occurredAtMS, stealIdentity,
				fmt.Sprintf("Friend %d", stealIdentity), fmt.Sprintf(`{"items":[{"id":%d,"num":1}]}`, i%100),
			); err != nil {
				_ = visitorStatement.Close()
				_ = stealStatement.Close()
				_ = tx.Rollback()
				b.Fatal(err)
			}
			if _, err := visitorStatement.ExecContext(
				ctx, fmt.Sprintf("v-%06d", i), occurredAtMS, visitorIdentity,
				fmt.Sprintf("Visitor %d", visitorIdentity), i%5+1,
			); err != nil {
				_ = visitorStatement.Close()
				_ = stealStatement.Close()
				_ = tx.Rollback()
				b.Fatal(err)
			}
		}
		if err := visitorStatement.Close(); err != nil {
			_ = stealStatement.Close()
			_ = tx.Rollback()
			b.Fatal(err)
		}
		if err := stealStatement.Close(); err != nil {
			_ = tx.Rollback()
			b.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			b.Fatal(err)
		}
	}
}

func socialRankingBenchmarkInt64Ptr(value int64) *int64 {
	return &value
}
