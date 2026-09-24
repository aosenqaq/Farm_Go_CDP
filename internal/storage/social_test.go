package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestSocialRankingColumnsAreBackfilled(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "farm_go.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE social_steal_records (id TEXT NOT NULL, account_key TEXT NOT NULL, date_key TEXT NOT NULL, occurred_at TEXT NOT NULL, gid INTEGER, display_name TEXT, payload_json TEXT NOT NULL, PRIMARY KEY(account_key,id))`,
		`CREATE TABLE social_visitor_records (id TEXT NOT NULL, account_key TEXT NOT NULL, occurred_at_ms INTEGER NOT NULL, player_id INTEGER, display_name TEXT, action_type INTEGER, payload_json TEXT NOT NULL, PRIMARY KEY(account_key,id))`,
		`INSERT INTO social_steal_records VALUES ('s1','gid:1','2026-07-20','2026-07-20T08:00:00Z',7,'A','{"stealCount":1}')`,
		`INSERT INTO social_steal_records VALUES ('s2','gid:1','2026-07-20','invalid-old-value',9,'C','{"stealCount":2}')`,
		`INSERT INTO social_visitor_records VALUES ('v1','gid:1',1784534400000,8,'B',1,'{"stealItemNum":4}')`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var occurredAtMS int64
	if err := store.db.QueryRowContext(ctx, `SELECT occurred_at_ms FROM social_steal_records WHERE id='s1'`).Scan(&occurredAtMS); err != nil {
		t.Fatal(err)
	}
	var invalidOccurredAtMS int64
	if err := store.db.QueryRowContext(ctx, `SELECT occurred_at_ms FROM social_steal_records WHERE id='s2'`).Scan(&invalidOccurredAtMS); err != nil {
		t.Fatal(err)
	}
	var stealItemNum int
	if err := store.db.QueryRowContext(ctx, `SELECT steal_item_num FROM social_visitor_records WHERE id='v1'`).Scan(&stealItemNum); err != nil {
		t.Fatal(err)
	}
	if occurredAtMS != 1784534400000 || invalidOccurredAtMS != 0 || stealItemNum != 4 {
		t.Fatalf("backfill = %d, %d, %d", occurredAtMS, invalidOccurredAtMS, stealItemNum)
	}
}

func TestSocialRankingMigrationOptimizesPopulatedLegacyDatabase(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "farm_go.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE social_steal_records (id TEXT NOT NULL, account_key TEXT NOT NULL, date_key TEXT NOT NULL, occurred_at TEXT NOT NULL, gid INTEGER, display_name TEXT, payload_json TEXT NOT NULL, PRIMARY KEY(account_key,id))`,
		`CREATE TABLE social_visitor_records (id TEXT NOT NULL, account_key TEXT NOT NULL, occurred_at_ms INTEGER NOT NULL, player_id INTEGER, display_name TEXT, action_type INTEGER, payload_json TEXT NOT NULL, PRIMARY KEY(account_key,id))`,
		`WITH RECURSIVE sequence(value) AS (VALUES(1) UNION ALL SELECT value + 1 FROM sequence WHERE value < 2000)
			INSERT INTO social_steal_records
			SELECT printf('s-%04d', value), 'gid:1', '2026-07-20', '2026-07-20T08:00:00Z', value % 100 + 1,
				printf('Friend %d', value % 100 + 1), '{}'
			FROM sequence`,
		`WITH RECURSIVE sequence(value) AS (VALUES(1) UNION ALL SELECT value + 1 FROM sequence WHERE value < 2000)
			INSERT INTO social_visitor_records
			SELECT printf('v-%04d', value), 'gid:1', 1784534400000, value % 100 + 1,
				printf('Visitor %d', value % 100 + 1), 1, '{"stealItemNum":1}'
			FROM sequence`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			_ = db.Close()
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var statisticCount int
	if err := store.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_stat1
		WHERE idx IN ('idx_social_steal_records_ranking', 'idx_social_visitor_records_ranking')
	`).Scan(&statisticCount); err != nil {
		t.Fatal(err)
	}
	if statisticCount != 2 {
		t.Fatalf("ranking index statistics = %d, want 2", statisticCount)
	}

	for _, query := range []SocialRankingQuery{
		{AccountKey: "gid:1", Tab: "stolenByMe", ViewMode: "ranking", Limit: 50},
		{AccountKey: "gid:1", Tab: "stolenFromMe", ViewMode: "ranking", Limit: 50},
	} {
		statement, args, _, err := buildSocialGroupedRankingQuery(query)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := store.db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+statement, args...)
		if err != nil {
			t.Fatal(err)
		}
		var plans []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				_ = rows.Close()
				t.Fatal(err)
			}
			plans = append(plans, detail)
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
		wantIndex := "idx_social_steal_records_ranking"
		if query.Tab == "stolenFromMe" {
			wantIndex = "idx_social_visitor_records_ranking"
		}
		if !strings.Contains(strings.Join(plans, "\n"), wantIndex) {
			t.Fatalf("%s query plan = %q, want index %q", query.Tab, plans, wantIndex)
		}
	}
}

func TestSocialFriendRulesRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	want := SocialFriendRules{
		WhitelistEnabled: true,
		WhitelistScopes:  []string{"steal"},
		Whitelist:        []string{"10001"},
		BlacklistEnabled: true,
		BlacklistScopes:  []string{"mischief"},
		Blacklist:        []string{"10002"},
		MaskedBlacklist:  true,
		MaskedMaxLevel:   2,
	}
	if err := store.SaveSocialFriendRules(ctx, "10000", want); err != nil {
		t.Fatalf("save rules: %v", err)
	}
	got, err := store.LoadSocialFriendRules(ctx, "10000")
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}
	if got.Whitelist[0] != "10001" || got.Blacklist[0] != "10002" || got.MaskedMaxLevel != 2 {
		t.Fatalf("rules round trip = %#v", got)
	}
}

func TestSocialRecordsRoundTripByAccountAndRange(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	err = store.AppendSocialStealRecords(ctx, "account-a", []SocialRecordRow{
		{ID: "r1", DateKey: "2026-07-08", OccurredAt: "2026-07-08T08:00:00Z", OccurredAtMS: 1783497600123, GID: 10001, DisplayName: "A", PayloadJSON: `{"gid":10001}`},
		{ID: "r2", DateKey: "2026-07-07", OccurredAt: "2026-07-07T08:00:00Z", GID: 10002, DisplayName: "B", PayloadJSON: `{"gid":10002}`},
	})
	if err != nil {
		t.Fatalf("append records: %v", err)
	}
	rows, err := store.ListSocialStealRecords(ctx, "account-a", []string{"2026-07-08"})
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "r1" || rows[0].OccurredAtMS != 1783497600123 {
		t.Fatalf("rows = %#v", rows)
	}
	derivedRows, err := store.ListSocialStealRecords(ctx, "account-a", []string{"2026-07-07"})
	if err != nil {
		t.Fatalf("list derived records: %v", err)
	}
	if len(derivedRows) != 1 || derivedRows[0].OccurredAtMS != 1783411200000 {
		t.Fatalf("derived rows = %#v", derivedRows)
	}
}

func TestAppendSocialStealRecordsRejectsInvalidTimestamp(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	err = store.AppendSocialStealRecords(ctx, "account-a", []SocialRecordRow{{
		ID: "invalid", DateKey: "2026-07-08", OccurredAt: "not-a-timestamp", OccurredAtMS: 1, PayloadJSON: `{}`,
	}})
	if err == nil {
		t.Fatal("invalid nonempty timestamp was accepted")
	}
}

func TestSocialVisitorRowsRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	err = store.AppendSocialVisitorRecords(ctx, "account-a", []SocialVisitorRow{
		{ID: "v1", OccurredAtMS: 1783497600000, PlayerID: 10001, DisplayName: "A", ActionType: 1, StealItemNum: 3, PayloadJSON: `{"playerId":10001}`},
		{ID: "v2", OccurredAtMS: 1783497600001, PlayerID: 10002, DisplayName: "B", ActionType: 1, PayloadJSON: `{"playerId":10002,"stealItemNum":4}`},
	})
	if err != nil {
		t.Fatalf("append visitors: %v", err)
	}
	rows, err := store.ListSocialVisitorRecords(ctx, "account-a")
	if err != nil {
		t.Fatalf("list visitors: %v", err)
	}
	if len(rows) != 2 || rows[0].ID != "v2" || rows[0].StealItemNum != 4 || rows[1].ID != "v1" || rows[1].StealItemNum != 3 {
		t.Fatalf("visitor rows = %#v", rows)
	}
}

func TestSocialDogGuardCacheRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	wantA := `{"results":[{"gid":10001}]}`
	wantB := `{"results":[{"gid":10002}]}`
	if err := store.SaveSocialDogGuardCache(ctx, "gid:10001", wantA); err != nil {
		t.Fatalf("save account a dog cache: %v", err)
	}
	if err := store.SaveSocialDogGuardCache(ctx, "gid:10002", wantB); err != nil {
		t.Fatalf("save account b dog cache: %v", err)
	}
	gotA, err := store.LoadSocialDogGuardCache(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load account a dog cache: %v", err)
	}
	gotB, err := store.LoadSocialDogGuardCache(ctx, "gid:10002")
	if err != nil {
		t.Fatalf("load account b dog cache: %v", err)
	}
	if gotA != wantA {
		t.Fatalf("account a cache = %q", gotA)
	}
	if gotB != wantB {
		t.Fatalf("account b cache = %q", gotB)
	}
}

func TestSocialRankingPreferencesRoundTripByAccount(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	if err := store.SaveSocialRankingPreferences(ctx, "gid:10001", SocialRankingPreferences{StolenByMeViewMode: "ranking", StolenFromMeViewMode: "timeline"}); err != nil {
		t.Fatalf("save account a preferences: %v", err)
	}
	if err := store.SaveSocialRankingPreferences(ctx, "gid:10002", SocialRankingPreferences{StolenByMeViewMode: "timeline", StolenFromMeViewMode: "ranking"}); err != nil {
		t.Fatalf("save account b preferences: %v", err)
	}
	if err := store.saveSettingForAccount(ctx, "gid:legacy", "social.rankingPreferences", `{"stolenFromMeViewMode":"ranking"}`); err != nil {
		t.Fatalf("seed legacy preferences: %v", err)
	}

	gotA, err := store.LoadSocialRankingPreferences(ctx, "gid:10001")
	if err != nil {
		t.Fatalf("load account a preferences: %v", err)
	}
	gotB, err := store.LoadSocialRankingPreferences(ctx, "gid:10002")
	if err != nil {
		t.Fatalf("load account b preferences: %v", err)
	}
	gotDefault, err := store.LoadSocialRankingPreferences(ctx, "gid:missing")
	if err != nil {
		t.Fatalf("load default preferences: %v", err)
	}
	gotLegacy, err := store.LoadSocialRankingPreferences(ctx, "gid:legacy")
	if err != nil {
		t.Fatalf("load legacy preferences: %v", err)
	}

	if gotA.StolenByMeViewMode != "ranking" || gotA.StolenFromMeViewMode != "timeline" {
		t.Fatalf("account a preferences = %#v", gotA)
	}
	if gotB.StolenByMeViewMode != "timeline" || gotB.StolenFromMeViewMode != "ranking" {
		t.Fatalf("account b preferences = %#v", gotB)
	}
	if gotDefault.StolenByMeViewMode != "timeline" || gotDefault.StolenFromMeViewMode != "timeline" {
		t.Fatalf("default preferences = %#v", gotDefault)
	}
	if gotLegacy.StolenByMeViewMode != "timeline" || gotLegacy.StolenFromMeViewMode != "ranking" {
		t.Fatalf("legacy preferences = %#v", gotLegacy)
	}
}

func TestSocialRankingPreferencesNormalizeEachModeIndependently(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	tests := []struct {
		accountKey string
		input      SocialRankingPreferences
		want       SocialRankingPreferences
	}{
		{
			accountKey: "gid:invalid-stolen-by-me",
			input:      SocialRankingPreferences{StolenByMeViewMode: "grid", StolenFromMeViewMode: "ranking"},
			want:       SocialRankingPreferences{StolenByMeViewMode: "timeline", StolenFromMeViewMode: "ranking"},
		},
		{
			accountKey: "gid:invalid-stolen-from-me",
			input:      SocialRankingPreferences{StolenByMeViewMode: "ranking", StolenFromMeViewMode: "list"},
			want:       SocialRankingPreferences{StolenByMeViewMode: "ranking", StolenFromMeViewMode: "timeline"},
		},
	}

	for _, test := range tests {
		if err := store.SaveSocialRankingPreferences(ctx, test.accountKey, test.input); err != nil {
			t.Fatalf("save %s preferences: %v", test.accountKey, err)
		}
		got, err := store.LoadSocialRankingPreferences(ctx, test.accountKey)
		if err != nil {
			t.Fatalf("load %s preferences: %v", test.accountKey, err)
		}
		if got != test.want {
			t.Fatalf("%s preferences = %#v, want %#v", test.accountKey, got, test.want)
		}
	}
}
