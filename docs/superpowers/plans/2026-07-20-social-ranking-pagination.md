# Social Ranking Pagination Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the social ranking dialog open without waiting for a runtime visitor refresh and load only the selected view in stable 50-row SQLite cursor pages.

**Architecture:** Split the current all-in-one ranking request into a local `Rankings` page query and an independent `RefreshVisitors` runtime action. Normalize the fields required for cursor sorting and aggregation into SQLite columns, query summary and page rows in one read transaction, and let an always-mounted `SocialRankingDialog` own an account-remounted session cache keyed by date range, tab, and view mode.

**Tech Stack:** Go 1.25, `database/sql`, modernc SQLite, Wails v2, React 18, TypeScript, Vitest, react-test-renderer, Vite

---

## File Map

- Create `internal/farm/social/ranking_page.go`: ranking request validation, cursor encoding/decoding, page response normalization, and local page service orchestration.
- Create `internal/farm/social/ranking_page_test.go`: request, cursor, page, and refresh separation tests.
- Modify `internal/farm/social/types.go`: replace the full ranking response with the page contract.
- Modify `internal/farm/social/service.go`: narrow the ranking store interface and move visitor refresh out of `Rankings`.
- Modify `internal/farm/social/service_test.go`: remove old full-load expectations and test the independent refresh action.
- Modify `internal/storage/migrations.go`: add normalized numeric fields and cursor indexes through idempotent migrations.
- Modify `internal/storage/social.go`: populate normalized fields on writes while retaining full-list methods for import/export.
- Create `internal/storage/social_ranking.go`: read-transaction summary, timeline page, grouped ranking page, and current-page steal-item detail queries.
- Create `internal/storage/social_ranking_test.go`: range, cursor, grouping, query-plan, and payload-size tests.
- Create `internal/storage/social_ranking_benchmark_test.go`: 100k steal plus 100k visitor record benchmark.
- Create `social_ranking_storage.go`: map storage rows to social domain rows without expanding `app.go` further.
- Create `social_ranking_storage_test.go`: verify item aggregation, invalid payload handling, and Wails payload size.
- Modify `app.go`: expose the independent visitor refresh Wails method and use the page response.
- Modify `app_test.go`: verify authorization-safe page and refresh contracts.
- Create `frontend/src/lib/socialRankingPages.ts`: page types plus cache merge and stale-response helpers.
- Create `frontend/src/lib/socialRankingPages.test.ts`: cache-key, append, reset, and generation tests.
- Create `frontend/src/views/social/SocialRankingDialog.tsx`: isolated lazy-loading and paginated ranking dialog.
- Create `frontend/src/views/social/SocialRankingDialog.test.tsx`: dialog paging, cache, stale response, and refresh tests.
- Modify `frontend/src/views/SocialView.tsx`: lazy open, local page cache, scroll pagination, loading/error states, and explicit visitor refresh.
- Modify `frontend/src/views/SocialView.test.tsx`: ranking entry and dialog prop-wiring tests.
- Modify `frontend/src/views/FarmWorkspaceView.tsx`: pass page-query, refresh, preferences, and cache revision props.
- Modify `frontend/src/AuthorizedApp.tsx`: remove ranking prefetch/full ranking state, preserve preference state, and provide scoped async handlers.
- Modify `frontend/src/App.test.tsx`: assert social-tab policy no longer prefetches ranking rows.
- Modify `frontend/src/style.css`: stable first-page and bottom-loading list states.
- Regenerate `frontend/wailsjs/go/main/App.js`, `frontend/wailsjs/go/main/App.d.ts`, and `frontend/wailsjs/go/models.ts` from the exported Go contract.

### Task 1: Define The Page Contract And Stable Cursors

**Files:**
- Create: `internal/farm/social/ranking_page.go`
- Create: `internal/farm/social/ranking_page_test.go`
- Modify: `internal/farm/social/types.go`

- [ ] **Step 1: Write failing contract and cursor tests**

Add table tests that require default `limit=50`, maximum `limit=100`, `visitors` mode normalization, cursor scope validation, identical-sort-key round trips, and reuse of the first page's frozen window when a later page supplies a later `Now`:

```go
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
	if _, err := NormalizeRankingRequest("gid:10001", RankingRequest{Tab: "bad"}); err == nil {
		t.Fatal("invalid tab was accepted")
	}
}

func TestRankingCursorRejectsAnotherScope(t *testing.T) {
	query, err := NormalizeRankingRequest("gid:10001", RankingRequest{
		Tab: "stolenByMe", ViewMode: "timeline", DateRange: "all", Limit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := EncodeRankingCursor(query, RankingPageRow{TimeMS: 1234, Key: "row-2"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NormalizeRankingRequest("gid:10002", RankingRequest{
		Tab: "stolenByMe", ViewMode: "timeline", DateRange: "all", Cursor: cursor,
	}); err == nil {
		t.Fatal("cross-account cursor was accepted")
	}
}
```

- [ ] **Step 2: Run the tests and verify the contract does not exist**

Run: `go test ./internal/farm/social -run 'TestNormalizeRankingRequest|TestRankingCursor' -count=1`

Expected: FAIL because `NormalizeRankingRequest`, the new request fields, and page row types are undefined.

- [ ] **Step 3: Add the domain types and cursor implementation**

Extend `RankingRequest` and add the page contract in `types.go`. Keep the old `RefreshVisitors`, `IncludeNonFriends`, and `RankingState` fields temporarily so this first commit still compiles; Task 5 removes them atomically when the service switches to `RankingPage`:

```go
type RankingRequest struct {
	Tab               string    `json:"tab,omitempty"`
	ViewMode          string    `json:"viewMode,omitempty"`
	DateRange         string    `json:"dateRange,omitempty"`
	Cursor            string    `json:"cursor,omitempty"`
	Limit             int       `json:"limit,omitempty"`
	RefreshVisitors   bool      `json:"refreshVisitors,omitempty"`
	IncludeNonFriends bool      `json:"includeNonFriends,omitempty"`
	Now               time.Time `json:"-"`
}

type RankingPageRow struct {
	Kind         string      `json:"kind"`
	Key          string      `json:"key"`
	IdentityKey  string      `json:"-"`
	TimeMS       int64       `json:"timeMS"`
	DisplayName  string      `json:"displayName"`
	Rank         int         `json:"rank,omitempty"`
	EventCount   int         `json:"eventCount,omitempty"`
	StealCount   int         `json:"stealCount,omitempty"`
	Items        []StealItem `json:"items"`
	ActionType   int         `json:"actionType,omitempty"`
	ActionLabel  string      `json:"actionLabel,omitempty"`
	ActionTarget string      `json:"actionTarget,omitempty"`
}

type RankingPage struct {
	OK         bool             `json:"ok"`
	Status     Status           `json:"status"`
	Message    string           `json:"message"`
	Tab        string           `json:"tab"`
	ViewMode   string           `json:"viewMode"`
	DateRange  string           `json:"dateRange"`
	Summary    RankingSummary   `json:"summary"`
	Rows       []RankingPageRow `json:"rows"`
	NextCursor string           `json:"nextCursor,omitempty"`
	HasMore    bool             `json:"hasMore"`
}
```

In `ranking_page.go`, define `RankingQuery`, a versioned cursor payload, URL-safe base64 JSON encoding, and a SHA-256 scope fingerprint over account key, normalized tab, mode, range, start, and end. On the first page, freeze `EndMS` to the normalized request's `now` even for `all`; on a cursor page, decode first and reuse the cursor's frozen `StartMS`/`EndMS` instead of calculating a new window. Reject invalid base64, version, account/tab/mode/range fingerprint, missing sort keys, or an end before the start with a non-nil error. Clamp an omitted limit to 50 and reject values outside 1 to 100.

Use these internal types so later store and service tasks share one contract:

```go
type RankingCursor struct {
	TimeMS      int64
	Key         string
	IdentityKey string
	StealCount  int
	EventCount  int
	RankOffset  int
	StartMS     *int64
	EndMS       int64
}

type RankingQuery struct {
	Tab       string
	ViewMode  string
	Window    DateRangeWindow
	Cursor    *RankingCursor
	Limit     int
	ScopeHash string
}

type RankingPageData struct {
	Summary RankingSummary
	Rows    []RankingPageRow
	HasMore bool
}
```

- [ ] **Step 4: Run focused tests**

Run: `go test ./internal/farm/social -run 'TestNormalizeRankingRequest|TestRankingCursor' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the domain contract**

```powershell
git add internal/farm/social/types.go internal/farm/social/ranking_page.go internal/farm/social/ranking_page_test.go
git commit -m "feat: define social ranking page contract"
```

### Task 2: Add Normalized Cursor And Aggregate Columns

**Files:**
- Modify: `internal/storage/migrations.go`
- Modify: `internal/storage/social.go`
- Modify: `internal/storage/social_test.go`

- [ ] **Step 1: Write failing migration and write-path tests**

Add a migration test that creates the old table shape before `Open` runs migrations, then asserts the added and backfilled values:

```go
func TestSocialRankingColumnsAreBackfilled(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "farm_go.db"))
	if err != nil { t.Fatal(err) }
	for _, statement := range []string{
		`CREATE TABLE social_steal_records (id TEXT NOT NULL, account_key TEXT NOT NULL, date_key TEXT NOT NULL, occurred_at TEXT NOT NULL, gid INTEGER, display_name TEXT, payload_json TEXT NOT NULL, PRIMARY KEY(account_key,id))`,
		`CREATE TABLE social_visitor_records (id TEXT NOT NULL, account_key TEXT NOT NULL, occurred_at_ms INTEGER NOT NULL, player_id INTEGER, display_name TEXT, action_type INTEGER, payload_json TEXT NOT NULL, PRIMARY KEY(account_key,id))`,
		`INSERT INTO social_steal_records VALUES ('s1','gid:1','2026-07-20','2026-07-20T08:00:00Z',7,'A','{"stealCount":1}')`,
		`INSERT INTO social_visitor_records VALUES ('v1','gid:1',1784534400000,8,'B',1,'{"stealItemNum":4}')`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil { t.Fatal(err) }
	}
	if err := db.Close(); err != nil { t.Fatal(err) }

	store, err := Open(ctx, dir)
	if err != nil { t.Fatal(err) }
	defer store.Close()

	var occurredAtMS int64
	if err := store.db.QueryRowContext(ctx, `SELECT occurred_at_ms FROM social_steal_records WHERE id='s1'`).Scan(&occurredAtMS); err != nil { t.Fatal(err) }
	var stealItemNum int
	if err := store.db.QueryRowContext(ctx, `SELECT steal_item_num FROM social_visitor_records WHERE id='v1'`).Scan(&stealItemNum); err != nil { t.Fatal(err) }
	if occurredAtMS != 1784534400000 || stealItemNum != 4 {
		t.Fatalf("backfill = %d, %d", occurredAtMS, stealItemNum)
	}
}
```

Also extend round-trip tests to assert new writes preserve `OccurredAtMS` and `StealItemNum`.

- [ ] **Step 2: Run the migration tests and verify failure**

Run: `go test ./internal/storage -run 'TestSocialRankingColumns|TestSocialRecordsRoundTrip|TestSocialVisitorRowsRoundTrip' -count=1`

Expected: FAIL because the normalized columns and row fields do not exist.

- [ ] **Step 3: Implement idempotent migration and normalized writes**

Add these fields:

```go
type SocialRecordRow struct {
	ID string; DateKey string; OccurredAt string; OccurredAtMS int64
	GID int; DisplayName string; PayloadJSON string
}

type SocialVisitorRow struct {
	ID string; OccurredAtMS int64; PlayerID int; DisplayName string
	ActionType int; StealItemNum int; PayloadJSON string
}
```

After base table creation, call `ensureColumn` for `social_steal_records.occurred_at_ms INTEGER NOT NULL DEFAULT 0` and `social_visitor_records.steal_item_num INTEGER NOT NULL DEFAULT 0`. Backfill with guarded SQL:

```sql
UPDATE social_steal_records
SET occurred_at_ms = CAST(strftime('%s', occurred_at) AS INTEGER) * 1000
WHERE occurred_at_ms = 0;

UPDATE social_visitor_records
SET steal_item_num = CASE
  WHEN json_valid(payload_json) THEN COALESCE(CAST(json_extract(payload_json, '$.stealItemNum') AS INTEGER), 0)
  ELSE 0
END
WHERE steal_item_num = 0;
```

Create the four cursor indexes only after the columns exist:

```sql
CREATE INDEX IF NOT EXISTS idx_social_steal_records_page
ON social_steal_records(account_key, occurred_at_ms DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_social_steal_records_date_page
ON social_steal_records(account_key, date_key, occurred_at_ms DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_social_visitor_records_page
ON social_visitor_records(account_key, occurred_at_ms DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_social_visitor_records_action_page
ON social_visitor_records(account_key, action_type, occurred_at_ms DESC, id DESC);
```

Update insert/upsert statements so both normalized columns are written and updated. When `OccurredAtMS` is zero, parse `OccurredAt` before inserting and return an error for a non-empty invalid timestamp; when `StealItemNum` is zero, read it from valid `PayloadJSON`. This keeps existing callers correct before the App adapter is updated. Preserve the existing full-list methods because import/export still uses them.

- [ ] **Step 4: Run storage tests**

Run: `go test ./internal/storage -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the migration**

```powershell
git add internal/storage/migrations.go internal/storage/social.go internal/storage/social_test.go
git commit -m "feat: normalize social ranking storage fields"
```

### Task 3: Implement Range-Filtered Timeline Pages And Summary

**Files:**
- Create: `internal/storage/social_ranking.go`
- Create: `internal/storage/social_ranking_test.go`

- [ ] **Step 1: Write failing timeline, summary, and index-plan tests**

Seed two accounts, records inside and outside a 7-day window, equal timestamps with different IDs, and both visitor action types. Assert:

```go
startMS, endMS := int64(1784016000000), int64(1784620800000)
query := SocialRankingQuery{
	AccountKey: "gid:10001", Tab: "stolenFromMe", ViewMode: "timeline",
	StartMS: &startMS, EndMS: &endMS, Limit: 2,
}
page, err := store.QuerySocialRankingPage(ctx, query)
if err != nil { t.Fatal(err) }
if got := rowKeys(page.Rows); !reflect.DeepEqual(got, []string{"v3", "v2"}) {
	t.Fatalf("first page = %v", got)
}
if !page.HasMore || page.Summary.VisitorCount != 4 || page.Summary.StolenFromMeCount != 3 {
	t.Fatalf("page = %#v", page)
}
last := page.Rows[len(page.Rows)-1]
query.Cursor = &SocialRankingCursor{TimeMS: last.TimeMS, Key: last.Key}
next, err := store.QuerySocialRankingPage(ctx, query)
if err != nil { t.Fatal(err) }
if got := rowKeys(next.Rows); !reflect.DeepEqual(got, []string{"v1"}) {
	t.Fatalf("second page = %v", got)
}
```

Add `EXPLAIN QUERY PLAN` assertions that the all-time steal timeline uses `idx_social_steal_records_page` and the stolen-from-me timeline uses `idx_social_visitor_records_action_page`.

- [ ] **Step 2: Run focused storage tests and verify failure**

Run: `go test ./internal/storage -run 'TestQuerySocialRankingTimeline|TestSocialRankingSummary|TestSocialRankingQueryPlan' -count=1`

Expected: FAIL because `QuerySocialRankingPage` and its query types do not exist.

- [ ] **Step 3: Implement the read transaction, summary, and timeline dispatch**

Define storage-only query and result types mirroring the normalized domain values. `QuerySocialRankingPage` must:

```go
type SocialRankingCursor struct {
	TimeMS int64; Key string; IdentityKey string
	StealCount int; EventCount int; RankOffset int
}

type SocialRankingQuery struct {
	AccountKey string; Tab string; ViewMode string
	StartMS *int64; EndMS *int64
	Cursor *SocialRankingCursor; Limit int
}

type SocialRankingPageRow struct {
	Kind string; Key string; IdentityKey string; TimeMS int64
	DisplayName string; Rank int; EventCount int; StealCount int
	ActionType int; ActionLabel string; ActionTarget string
	PayloadJSON string; PayloadJSONs []string
}

type SocialRankingPage struct {
	Summary SocialRankingSummary
	Rows []SocialRankingPageRow
	HasMore bool
}

type SocialRankingSummary struct {
	VisitorCount int
	StolenFromMeCount int
	StolenByMeCount int
	StolenByMeRecordCount int
}
```

1. begin a read-only transaction with `sql.TxOptions{ReadOnly: true}`;
2. calculate all four summary counts with the same account/time predicates;
3. dispatch to steal, stolen, or visitor timeline query;
4. fetch `limit+1`, trim the extra row, and set `HasMore`;
5. commit only after both summary and rows succeed.

Build the `WHERE` clause from a fixed set of parameterized fragments so absent bounds do not introduce `OR ? IS NULL` expressions that prevent range-index use. The resulting steal query for a bounded cursor page must be equivalent to:

```sql
SELECT id, occurred_at_ms, gid, display_name, payload_json
FROM social_steal_records
WHERE account_key = ?
  AND occurred_at_ms >= ?
  AND occurred_at_ms <= ?
  AND (occurred_at_ms < ? OR (occurred_at_ms = ? AND id < ?))
ORDER BY occurred_at_ms DESC, id DESC
LIMIT ?;
```

For an omitted start or first-page cursor, omit that entire fixed fragment and its arguments. Never interpolate user input; only append these predefined SQL fragments.

Visitor timeline uses the same cursor shape. Add `AND action_type = 1` only for `stolenFromMe`. Build summary identities as `gid`/`player_id` when positive and `display_name` otherwise; count visitor records without the action filter and stolen-from-me identities with `action_type=1`.

- [ ] **Step 4: Run focused and complete storage tests**

Run: `go test ./internal/storage -run 'TestQuerySocialRankingTimeline|TestSocialRankingSummary|TestSocialRankingQueryPlan' -count=1`

Expected: PASS.

Run: `go test ./internal/storage -count=1`

Expected: PASS.

- [ ] **Step 5: Commit timeline pagination**

```powershell
git add internal/storage/social_ranking.go internal/storage/social_ranking_test.go
git commit -m "feat: query social ranking timeline pages"
```

### Task 4: Implement Grouped Ranking Pages And Current-Page Details

**Files:**
- Modify: `internal/storage/social_ranking.go`
- Modify: `internal/storage/social_ranking_test.go`
- Create: `internal/storage/social_ranking_benchmark_test.go`

- [ ] **Step 1: Write failing grouped-page and item-detail tests**

Seed more identities than one page, including two records for one friend with different item payloads. Assert stable rank numbers, aggregate counts, current-page-only payloads, and second-page identity keys:

```go
page, err := store.QuerySocialRankingPage(ctx, SocialRankingQuery{
	AccountKey: "gid:10001", Tab: "stolenByMe", ViewMode: "ranking", Limit: 2,
})
if err != nil { t.Fatal(err) }
if len(page.Rows) != 2 || page.Rows[0].Rank != 1 || page.Rows[0].EventCount != 2 {
	t.Fatalf("page = %#v", page)
}
if len(page.Rows[0].PayloadJSONs) != 2 {
	t.Fatalf("detail payloads = %#v", page.Rows[0].PayloadJSONs)
}
for _, row := range page.Rows {
	if row.IdentityKey == "friend-not-on-page" && len(row.PayloadJSONs) > 0 {
		t.Fatal("ranking page loaded detail payloads for an off-page identity")
	}
}
```

Add a visitor ranking case that checks `steal_count = SUM(steal_item_num)` and an equal aggregate case that falls back to last time and identity key.

- [ ] **Step 2: Run grouped ranking tests and verify failure**

Run: `go test ./internal/storage -run 'TestQuerySocialRankingGrouped|TestSocialRankingPagePayload' -count=1`

Expected: FAIL because ranking mode is not implemented.

- [ ] **Step 3: Implement SQL grouping, mixed-direction cursor predicates, and detail lookup**

Use these identity expressions consistently in `SELECT`, `GROUP BY`, detail lookup, and cursor encoding:

```sql
CASE WHEN gid > 0 THEN 'gid:' || CAST(gid AS TEXT) ELSE 'name:' || trim(display_name) END
CASE WHEN player_id > 0 THEN 'player:' || CAST(player_id AS TEXT) ELSE 'name:' || trim(display_name) END
```

The steal ranking query groups with `COUNT(*)` for both current `steal_count` and `event_count`, uses `MAX(occurred_at_ms)`, applies the full aggregate cursor in `HAVING`, and orders by counts descending, last time descending, identity ascending. The visitor ranking query filters `action_type=1`, uses `SUM(steal_item_num)`, `COUNT(*)`, and the same deterministic cursor strategy.

Fetch `limit+1` aggregate rows. Assign the first row rank from cursor offset plus one; include `RankOffset` in the ranking cursor so later pages continue numbering. After trimming, fetch steal `payload_json` only for the returned identity keys and same date window, attach payloads by identity key, and never load payloads for the extra sentinel row.

- [ ] **Step 4: Add and run the 100k benchmark**

Create `BenchmarkSocialRankingPage100K` that inserts 100,000 steal and 100,000 visitor rows in batched transactions before `b.ResetTimer()`, then queries the first 50 rows for `current`, `30d`, and `all` sub-benchmarks.

Run: `go test ./internal/storage -run '^$' -bench BenchmarkSocialRankingPage100K -benchtime=3x -benchmem`

Expected: each first-page sub-benchmark reports less than `300 ms/op` on the implementation machine and completes without loading an unbounded result slice.

- [ ] **Step 5: Run all storage tests and commit**

Run: `go test ./internal/storage -count=1`

Expected: PASS.

```powershell
git add internal/storage/social_ranking.go internal/storage/social_ranking_test.go internal/storage/social_ranking_benchmark_test.go
git commit -m "feat: aggregate social ranking pages in sqlite"
```

### Task 5: Split Local Page Queries From Runtime Visitor Refresh

**Files:**
- Modify: `internal/farm/social/service.go`
- Modify: `internal/farm/social/service_test.go`
- Modify: `internal/farm/social/ranking_page.go`
- Modify: `internal/farm/social/ranking_page_test.go`
- Create: `social_ranking_storage.go`
- Create: `social_ranking_storage_test.go`
- Modify: `app.go`
- Modify: `app_test.go`

- [ ] **Step 1: Write failing service and App tests**

Replace the old `RefreshVisitors`-inside-`Rankings` test with focused tests. The first asserts that `Rankings` invokes only the page store. The second blocks the fake runtime refresh and proves a local page query still finishes. The remaining tests assert invalid page requests return `StatusFailed` with empty rows, runtime success plus `SaveVisitorRecords` failure returns a failed action, and empty results serialize `rows`/`items` as arrays:

```go
func TestServiceRankingQueryDoesNotCallRuntime(t *testing.T) {
	store := &memoryStore{rankingPage: RankingPageData{Rows: []RankingPageRow{}, HasMore: false}}
	caller := &fakeRuntimeCaller{responses: map[string]any{"gameCtl.getVisitorRecords": errors.New("must not run")}}
	page := NewService(store, caller, Options{AccountKey: "gid:1"}).Rankings(
		context.Background(), RankingRequest{Tab: "visitors", ViewMode: "timeline", DateRange: "all"},
	)
	if !page.OK || len(caller.calls) != 0 { t.Fatalf("page=%#v calls=%#v", page, caller.calls) }
}

func TestServiceRefreshVisitorsPersistsWithoutReturningRankingRows(t *testing.T) {
	store := &memoryStore{}
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getVisitorRecords": map[string]any{"records": []any{map[string]any{
			"playerId": float64(7), "displayName": "A", "actionType": float64(1), "time": float64(1784534400),
		}}},
	}}
	result := NewService(store, caller, Options{AccountKey: "gid:1"}).RefreshVisitors(context.Background())
	if !result.OK || len(store.visitors) != 1 { t.Fatalf("result=%#v visitors=%#v", result, store.visitors) }
}
```

Add App tests asserting unauthorized page responses contain `rows: []`, and `FarmSocialRefreshVisitors` returns the authorization error without calling runtime.

Add adapter tests that supply two current-page steal payloads, assert their items merge once, assert an invalid payload fails the page instead of silently dropping detail, and marshal a 50-row `RankingPage` to verify the actual Wails response stays below `100*1024` bytes.

- [ ] **Step 2: Run focused tests and verify failure**

Run: `go test ./internal/farm/social . -run 'TestServiceRankingQuery|TestServiceRefreshVisitors|TestFarmSocialRanking' -count=1`

Expected: FAIL because the new page store and refresh method are missing.

- [ ] **Step 3: Implement service orchestration and storage mapping**

Keep the existing full-list `RankingStore` for import/export and add a page-specific service interface:

```go
type RankingPageStore interface {
	QueryRankingPage(context.Context, string, RankingQuery) (RankingPageData, error)
}
```

`Rankings` normalizes the request, calls the page store, guarantees `Rows` and every row's `Items` are non-nil, and encodes `NextCursor` from the last returned row only when `HasMore` is true. `RefreshVisitors` performs the existing runtime call and `SaveVisitorRecords` behavior, but returns only `ActionResult`.

At this point remove deprecated `RefreshVisitors` and `IncludeNonFriends` from `RankingRequest`, delete `RankingState`, and remove the old full-list/filter/aggregate branch from `Service.Rankings`. Do not delete full-list storage methods or the original `RankingStore`, because import/export still depends on them.

Implement `socialStorageAdapter.QueryRankingPage` in `social_ranking_storage.go`. Map storage summary and rows into domain values; unmarshal only `PayloadJSON`/`PayloadJSONs` supplied for returned rows, merge steal items in one pass by item ID plus name, and preserve the storage-computed rank and cursor fields.

Expose:

```go
func (a *App) FarmSocialRankings(input social.RankingRequest) social.RankingPage
func (a *App) FarmSocialRefreshVisitors() social.ActionResult
```

Both methods use the existing authorization gate. The unauthorized ranking result must initialize `Rows: []social.RankingPageRow{}`.

- [ ] **Step 4: Run backend tests**

Run: `go test ./internal/farm/social . -count=1`

Expected: PASS.

Run: `go test ./... -count=1`

Expected: PASS.

- [ ] **Step 5: Commit service and Wails API changes**

```powershell
git add internal/farm/social app.go app_test.go social_ranking_storage.go social_ranking_storage_test.go
git commit -m "feat: split ranking pages from visitor refresh"
```

### Task 6: Add Frontend Page Cache Primitives

**Files:**
- Create: `frontend/src/lib/socialRankingPages.ts`
- Create: `frontend/src/lib/socialRankingPages.test.ts`

- [ ] **Step 1: Write failing cache and stale-generation tests**

```ts
const request = (tab: RankingTab, viewMode: RankingViewMode): RankingPageRequest => ({
  tab, viewMode, dateRange: 'current', cursor: '', limit: 50,
});
const page = (keys: string[], nextCursor: string): RankingPage => ({
  ok: true, status: 'ok', message: '', tab: 'stolenByMe', viewMode: 'timeline', dateRange: 'current',
  summary: { visitorCount: 0, stolenFromMeCount: 0, stolenByMeCount: 0, stolenByMeRecordCount: keys.length },
  rows: keys.map((key) => ({ kind: 'stealRecord', key, timeMS: 1, displayName: key, items: [] })),
  nextCursor, hasMore: nextCursor !== '',
});

it('keeps each ranking view in an independent cache entry', () => {
  const current = emptyRankingCache();
  const first = mergeRankingPage(current, request('stolenByMe', 'timeline'), page(['a'], 'next'));
  const second = mergeRankingPage(first, request('visitors', 'timeline'), page(['v'], ''));
  expect(selectRankingPage(second, request('stolenByMe', 'timeline')).rows.map((row) => row.key)).toEqual(['a']);
  expect(selectRankingPage(second, request('visitors', 'timeline')).rows.map((row) => row.key)).toEqual(['v']);
});

it('appends one cursor page without duplicating an existing key', () => {
  const first = mergeRankingPage(emptyRankingCache(), request('stolenByMe', 'timeline'), page(['a', 'b'], 'c2'));
  const next = mergeRankingPage(first, { ...request('stolenByMe', 'timeline'), cursor: 'c2' }, page(['b', 'c'], ''));
  expect(selectRankingPage(next, request('stolenByMe', 'timeline')).rows.map((row) => row.key)).toEqual(['a', 'b', 'c']);
});

it('rejects a response from an older request generation', () => {
  expect(shouldApplyRankingResponse({ requested: 3, current: 4 })).toBe(false);
  expect(shouldApplyRankingResponse({ requested: 4, current: 4 })).toBe(true);
});
```

- [ ] **Step 2: Run the tests and verify failure**

Run: `Set-Location frontend; npm test -- src/lib/socialRankingPages.test.ts`

Expected: FAIL because the cache module does not exist.

- [ ] **Step 3: Implement page types and pure cache helpers**

Define `RankingTab`, `RankingViewMode`, `RankingPageRequest`, `RankingPageRow`, `RankingPage`, `RankingCacheEntry`, and `RankingCache = Record<string, RankingCacheEntry>`. Use a cache key of `dateRange|tab|viewMode`; a cursor request appends and deduplicates by row key, while an empty cursor replaces that entry. Export helpers for empty cache, selection, merge, affected-tab invalidation, and generation equality.

Keep this module independent of React. The old `RankingState` types remain in `SocialView.tsx` until Task 8 switches all call sites atomically; this task must not leave the frontend in a type-broken intermediate state.

- [ ] **Step 4: Run cache tests and typecheck**

Run: `Set-Location frontend; npm test -- src/lib/socialRankingPages.test.ts`

Expected: PASS.

Run: `Set-Location frontend; npm run build`

Expected: PASS.

- [ ] **Step 5: Commit cache primitives**

```powershell
git add frontend/src/lib/socialRankingPages.ts frontend/src/lib/socialRankingPages.test.ts
git commit -m "feat: add social ranking page cache"
```

### Task 7: Build An Isolated Lazy And Paginated Ranking Dialog

**Files:**
- Create: `frontend/src/views/social/SocialRankingDialog.tsx`
- Create: `frontend/src/views/social/SocialRankingDialog.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write failing dialog behavior tests**

Add async renderer tests with deferred page promises that assert:

- changing `open` from false to true calls `onRankings` with `{tab:'stolenByMe', viewMode:'timeline', dateRange:'current', limit:50}` and no `refreshVisitors`;
- toggling `open` false and true renders the cached page immediately without a duplicate first-page request;
- a scroll event within 96px of the bottom requests exactly one `nextCursor` page;
- `hasMore=false` prevents another request;
- changing date/tab/mode uses a new cache entry and a fresh empty cursor;
- an older deferred response cannot replace a newer view;
- a first-page failure shows a retry action whose click repeats the same empty-cursor request;
- a next-page failure keeps existing rows and permits another scroll retry;
- refresh failure keeps existing rows and renders a non-blocking status.

Use a callback contract that returns the page instead of mutating parent state:

```tsx
<SocialRankingDialog
  open
  dataVersions={{ stolenByMe: 0, visitors: 0 }}
  rankingPreferences={{ stolenByMeViewMode: 'timeline', stolenFromMeViewMode: 'timeline' }}
  onRankings={async (request) => pageFor(request)}
  onRefreshVisitors={async () => ({ ok: true, status: 'ok', message: '访客记录已刷新。' })}
  onClose={() => undefined}
  onProtocolBlock={() => undefined}
/>
```

- [ ] **Step 2: Run focused SocialView tests and verify failure**

Run: `Set-Location frontend; npm test -- src/views/social/SocialRankingDialog.test.tsx`

Expected: FAIL because the isolated dialog component does not exist.

- [ ] **Step 3: Implement lazy first page, scroll pagination, and explicit refresh**

Inside `SocialRankingDialog` maintain:

```ts
const [rankingCache, setRankingCache] = useState<RankingCache>(() => emptyRankingCache());
const [rankingLoading, setRankingLoading] = useState<'idle' | 'first' | 'more'>('idle');
const [rankingError, setRankingError] = useState('');
const rankingGeneration = useRef(0);
const rankingInFlight = useRef(new Set<string>());
```

The parent always mounts `SocialRankingDialog`. When `open=false`, the component returns `null` but retains its React state; changing to `open=true` requests an empty-cursor page only when the selected cache entry is absent. The component renders the existing `social-dialog-backdrop` and `social-dialog wide` structure directly when open. `loadRankingPage` captures the generation, deduplicates `cacheKey|cursor`, merges only a current response, and clears in-flight state in `finally`. The list `onScroll` calls `loadMore` only when the remaining distance is at most 96px. A `dataVersions.stolenByMe` change invalidates only steal pages; a `dataVersions.visitors` change invalidates visitor and stolen-from-me pages before reloading an affected open view.

The explicit refresh handler awaits `onRefreshVisitors`, invalidates `visitors` and `stolenFromMe` cache entries after success, then loads the current affected view from an empty cursor. It never clears rows before success. A first-page failure renders a retry command in the list area; a next-page failure keeps rows and clears the in-flight key so scrolling can retry. Replace `buildRankingRows` with an O(page-size) projection over `RankingPageRow`; render rank from `row.rank`, time from `row.timeMS`, and details from row counts/items/action fields.

Add fixed-height first-load and bottom-load styles without changing the toolbar geometry. Preserve existing list overflow behavior and 8px-or-smaller component radii.

- [ ] **Step 4: Run ranking dialog tests**

Run: `Set-Location frontend; npm test -- src/views/social/SocialRankingDialog.test.tsx`

Expected: PASS.

Run: `Set-Location frontend; npm run build`

Expected: PASS because the new component is isolated and existing callers are unchanged.

- [ ] **Step 5: Commit dialog pagination**

```powershell
git add frontend/src/views/social/SocialRankingDialog.tsx frontend/src/views/social/SocialRankingDialog.test.tsx frontend/src/style.css
git commit -m "feat: build paginated social ranking dialog"
```

### Task 8: Integrate The Dialog, Remove Prefetch, And Regenerate Bindings

**Files:**
- Modify: `frontend/src/views/SocialView.tsx`
- Modify: `frontend/src/views/SocialView.test.tsx`
- Modify: `frontend/src/AuthorizedApp.tsx`
- Modify: `frontend/src/App.test.tsx`
- Modify: `frontend/src/views/FarmWorkspaceView.tsx`
- Modify: `frontend/src/views/FarmWorkspaceView.test.tsx`
- Regenerate: `frontend/wailsjs/go/main/App.js`
- Regenerate: `frontend/wailsjs/go/main/App.d.ts`
- Regenerate: `frontend/wailsjs/go/models.ts`

- [ ] **Step 1: Write failing lifecycle and prop-wiring tests**

Change the social refresh policy expectation to:

```ts
expect(socialRefreshPolicyForTab('social')).toEqual({
  refreshState: true,
  refreshRankingPreferences: true,
  refreshDogGuard: true,
  pollDogGuard: true,
});
```

Add source/renderer assertions that `AuthorizedApp` no longer owns `socialRankings`, entering the social tab does not call `FarmSocialRankings`, the page loader returns a scoped promise, and `FarmWorkspaceView` passes `onRefreshSocialVisitors` to `SocialView`.

Add a `SocialView` test that clicking “排行榜” mounts `SocialRankingDialog` and passes the preference, page loader, refresh, close, and protocol-block callbacks without issuing a second legacy request.

- [ ] **Step 2: Run lifecycle tests and verify failure**

Run: `Set-Location frontend; npm test -- src/App.test.tsx src/views/FarmWorkspaceView.test.tsx src/views/SocialView.test.tsx`

Expected: FAIL because the current policy prefetches rankings and the old props remain.

- [ ] **Step 3: Implement scoped handlers and preference-only state**

In `AuthorizedApp`:

- replace `socialRankings` state with `socialRankingPreferences` and `socialRankingDataVersions: {stolenByMe:number; visitors:number}`;
- remove `refreshRankings` from `SocialRefreshPolicy` and the social-tab effect;
- implement `loadSocialRankingPage(request)` by capturing the account scope, awaiting `FarmSocialRankings`, and rejecting a stale-scope result;
- implement `refreshSocialVisitors()` with the same scope check, incrementing only `visitors` after an `ok` result;
- increment only `stolenByMe` after a successful `action === 'steal'`; after import, increment `stolenByMe` when `payload.groups` contains `steal_records` and increment `visitors` when it contains `visitor_records`;
- keep the existing latest-save queue, but commit into preference state rather than a full ranking result.

Update `FarmWorkspaceView` and `SocialView` props to pass preferences, data versions, page loader, and visitor refresh independently.

Delete the old `RankingPanel`, `buildRankingRows`, full-array ranking types, and dialog-local ranking controls from `SocialView`; always mount the isolated `SocialRankingDialog` and pass `open={rankingOpen}`. Account changes already remount `FarmWorkspaceView` through `accountScopeRenderKey`, while the two data versions invalidate only affected same-account views after record mutations.

- [ ] **Step 4: Regenerate Wails bindings and run focused frontend tests**

Run: `wails generate module`

Expected: generated files expose `FarmSocialRefreshVisitors(): Promise<social.ActionResult>`, `FarmSocialRankings(...): Promise<social.RankingPage>`, and the new social page model classes. Inspect generated diffs; do not hand-edit unrelated generated models.

Run: `Set-Location frontend; npm test -- src/App.test.tsx src/views/FarmWorkspaceView.test.tsx src/views/SocialView.test.tsx src/views/social/SocialRankingDialog.test.tsx src/lib/socialRankingPages.test.ts`

Expected: PASS.

Run: `Set-Location frontend; npm run build`

Expected: PASS.

- [ ] **Step 5: Commit lifecycle and generated binding changes**

```powershell
git add frontend/src/AuthorizedApp.tsx frontend/src/App.test.tsx frontend/src/views/FarmWorkspaceView.tsx frontend/src/views/FarmWorkspaceView.test.tsx frontend/src/views/SocialView.tsx frontend/src/views/SocialView.test.tsx frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/models.ts
git commit -m "feat: load social rankings on demand"
```

### Task 9: Verify Behavior, Query Plans, And Performance

**Files:**
- Modify only if a verification failure identifies a defect in files already listed above.

- [ ] **Step 1: Run formatting and diff checks**

Run: `gofmt -w internal/farm/social/ranking_page.go internal/farm/social/ranking_page_test.go internal/farm/social/types.go internal/farm/social/service.go internal/farm/social/service_test.go internal/storage/migrations.go internal/storage/social.go internal/storage/social_ranking.go internal/storage/social_ranking_test.go internal/storage/social_ranking_benchmark_test.go social_ranking_storage.go social_ranking_storage_test.go app.go app_test.go`

Run: `git diff --check`

Expected: no formatting or whitespace errors.

- [ ] **Step 2: Run complete backend verification**

Run: `go test ./... -count=1`

Expected: PASS.

Run: `go test -race ./internal/farm/social ./internal/storage -count=1`

Expected: PASS with no race reports.

- [ ] **Step 3: Run complete frontend verification**

Run: `Set-Location frontend; npm test`

Expected: all Vitest suites pass.

Run: `Set-Location frontend; npm run build`

Expected: TypeScript compilation and Vite production build pass.

- [ ] **Step 4: Run the performance acceptance benchmark**

Run: `go test ./internal/storage -run '^$' -bench BenchmarkSocialRankingPage100K -benchtime=5x -benchmem`

Expected: `current`, `30d`, and `all` first-page cases each report below `300 ms/op`; benchmark allocation does not scale by returning 100k-element slices.

- [ ] **Step 5: Inspect the final behavior and commit verification fixes**

Run: `git status --short`

Expected: only intended source, test, generated binding, and documentation changes are present; `graphify-out/memory/` remains outside implementation commits unless separately requested.

If verification required code fixes, stage only those exact files and commit:

```powershell
git add app.go app_test.go social_ranking_storage.go internal/farm/social internal/storage frontend/src frontend/wailsjs
git commit -m "fix: complete social ranking pagination verification"
```

If no verification fix was required, do not create an empty commit.
