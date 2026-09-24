# Friend Social Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the approved `好友社交` workbench with real Go/Wails friend list, rule management, friend actions, rankings, visitor records, dog guard scanning, and import/export.

**Architecture:** Add `internal/farm/social` as the social domain layer, backed by SQLite methods in `internal/storage` and live runtime calls through the existing `RuntimeCaller` shape. `app.go` exposes Wails methods; React renders a primary friend table and second-level dialogs without inventing successful runtime data.

**Tech Stack:** Go, Wails bindings, SQLite via `modernc.org/sqlite`, React, Vitest, existing Farm_Go runtime supervisor.

---

## File Structure

- Create `internal/farm/social/types.go`: shared DTOs, status strings, rule types, friend rows, ranking rows, dog guard rows, import/export payloads.
- Create `internal/farm/social/normalize.go`: primitive conversion, list normalization, rule normalization, friend enrichment, protected gid helpers.
- Create `internal/farm/social/records.go`: steal record, visitor record, date range, and ranking summary logic.
- Create `internal/farm/social/service.go`: runtime-backed social state, friend actions, rankings, protocol block list, import/export orchestration.
- Create `internal/farm/social/dog_guard.go`: asynchronous dog guard scan runtime and cache normalization.
- Create tests beside each social file.
- Modify `internal/storage/migrations.go`: add social tables.
- Create `internal/storage/social.go` and `internal/storage/social_test.go`: account-scoped rules, record rows, visitor rows, dog guard cache, import/export persistence.
- Modify `app.go`, `app_test.go`: add Wails-facing `FarmSocial*` methods and App-level tests.
- Modify `frontend/src/App.tsx`: load social state and pass callbacks into workspace social area.
- Modify `frontend/src/views/FarmWorkspaceView.tsx`: route social area to `SocialView`.
- Create `frontend/src/views/SocialView.tsx` and `frontend/src/views/SocialView.test.tsx`.
- Modify `frontend/src/style.css`: add compact social workbench, dialogs, table, menu, and responsive rules.
- Regenerate `frontend/wailsjs/go/main/App.{js,d.ts}` and `frontend/wailsjs/go/models.ts` after Wails methods exist.

---

### Task 1: Social DTOs And Rule Normalization

**Files:**
- Create: `internal/farm/social/types.go`
- Create: `internal/farm/social/normalize.go`
- Test: `internal/farm/social/normalize_test.go`

- [ ] **Step 1: Write failing normalization tests**

```go
package social

import "testing"

func TestNormalizeFriendRulesDedupesListsAndRemovesScopeConflicts(t *testing.T) {
	rules := NormalizeFriendRules(FriendRules{
		WhitelistEnabled: true,
		WhitelistScopes:  []string{"steal", "help", "steal"},
		Whitelist:        []string{"10001", "10001", " Alice "},
		BlacklistEnabled: true,
		BlacklistScopes:  []string{"help", "mischief"},
		Blacklist:        []string{"10002", "10002"},
		MaskedBlacklist:  true,
		MaskedMaxLevel:   0,
	})

	if len(rules.WhitelistScopes) != 2 || rules.WhitelistScopes[0] != "steal" || rules.WhitelistScopes[1] != "help" {
		t.Fatalf("whitelist scopes = %#v", rules.WhitelistScopes)
	}
	if len(rules.BlacklistScopes) != 1 || rules.BlacklistScopes[0] != "mischief" {
		t.Fatalf("blacklist scopes = %#v", rules.BlacklistScopes)
	}
	if len(rules.Whitelist) != 2 || rules.Whitelist[0] != "10001" || rules.Whitelist[1] != "Alice" {
		t.Fatalf("whitelist = %#v", rules.Whitelist)
	}
	if rules.MaskedMaxLevel != 1 {
		t.Fatalf("masked max level = %d, want 1", rules.MaskedMaxLevel)
	}
}

func TestProtectedFriendGIDsAreHardSkipped(t *testing.T) {
	for _, gid := range []any{1184649322, "1142601927"} {
		if !IsProtectedFriendGID(gid) {
			t.Fatalf("gid %v should be protected", gid)
		}
	}
	if IsProtectedFriendGID("10001") {
		t.Fatal("normal gid should not be protected")
	}
}
```

- [ ] **Step 2: Run tests to verify RED**

Run: `go test ./internal/farm/social -run 'TestNormalizeFriendRules|TestProtectedFriendGIDs'`

Expected: FAIL because package/files do not exist or symbols are undefined.

- [ ] **Step 3: Implement DTOs and normalization**

Add these core shapes:

```go
package social

type Status string

const (
	StatusOK              Status = "ok"
	StatusRuntimeNotReady Status = "runtime_not_ready"
	StatusUnsupported     Status = "unsupported_target"
	StatusBusy            Status = "busy"
	StatusSkipped         Status = "skipped"
	StatusFailed          Status = "failed"
)

type ActionResult struct {
	OK      bool   `json:"ok"`
	Status  Status `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type FriendRules struct {
	WhitelistEnabled bool     `json:"whitelistEnabled"`
	WhitelistScopes  []string `json:"whitelistScopes"`
	Whitelist        []string `json:"whitelist"`
	BlacklistEnabled bool     `json:"blacklistEnabled"`
	BlacklistScopes  []string `json:"blacklistScopes"`
	Blacklist        []string `json:"blacklist"`
	MaskedBlacklist  bool     `json:"maskedBlacklist"`
	MaskedMaxLevel   int      `json:"maskedMaxLevel"`
}

type FriendRow struct {
	GID              int            `json:"gid"`
	Name             string         `json:"name,omitempty"`
	DisplayName      string         `json:"displayName"`
	Remark           string         `json:"remark,omitempty"`
	AvatarURL        string         `json:"avatarUrl,omitempty"`
	Level            int            `json:"level,omitempty"`
	WorkCounts       map[string]int `json:"workCounts"`
	Stealable        bool           `json:"stealable"`
	Helpable         bool           `json:"helpable"`
	Mischiefable      bool           `json:"mischiefable"`
	Blacklisted      bool           `json:"blacklisted"`
	Whitelisted      bool           `json:"whitelisted"`
	MaskedBlocked    bool           `json:"maskedBlocked"`
	Protected        bool           `json:"protected"`
	ProtocolBlocked  bool           `json:"protocolBlocked"`
	HasGuardDog      bool           `json:"hasGuardDog"`
	Raw              map[string]any `json:"raw,omitempty"`
}
```

Implement `NormalizeFriendRules`, `NormalizeStringList`, `NormalizeScopeList`, `PositiveInt`, and `IsProtectedFriendGID`.

- [ ] **Step 4: Run tests to verify GREEN**

Run: `go test ./internal/farm/social -run 'TestNormalizeFriendRules|TestProtectedFriendGIDs'`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/farm/social/types.go internal/farm/social/normalize.go internal/farm/social/normalize_test.go
git commit -m "feat: add social rule normalization"
```

---

### Task 2: SQLite Social Persistence

**Files:**
- Modify: `internal/storage/migrations.go`
- Create: `internal/storage/social.go`
- Test: `internal/storage/social_test.go`

- [ ] **Step 1: Write failing persistence tests**

```go
package storage

import (
	"context"
	"testing"
)

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
		{ID: "r1", DateKey: "2026-07-08", OccurredAt: "2026-07-08T08:00:00Z", GID: 10001, DisplayName: "A", PayloadJSON: `{"gid":10001}`},
	})
	if err != nil {
		t.Fatalf("append records: %v", err)
	}
	rows, err := store.ListSocialStealRecords(ctx, "account-a", []string{"2026-07-08"})
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "r1" {
		t.Fatalf("rows = %#v", rows)
	}
}
```

- [ ] **Step 2: Run tests to verify RED**

Run: `go test ./internal/storage -run 'TestSocial'`

Expected: FAIL because `SocialFriendRules`, `SaveSocialFriendRules`, and record methods are undefined.

- [ ] **Step 3: Add migrations**

Add these tables to `Migrate`:

```sql
CREATE TABLE IF NOT EXISTS social_friend_rules (
	account_key TEXT PRIMARY KEY,
	rules_json TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS social_steal_records (
	id TEXT NOT NULL,
	account_key TEXT NOT NULL,
	date_key TEXT NOT NULL,
	occurred_at TEXT NOT NULL,
	gid INTEGER,
	display_name TEXT,
	payload_json TEXT NOT NULL,
	PRIMARY KEY (account_key, id)
);
CREATE INDEX IF NOT EXISTS idx_social_steal_records_account_date
	ON social_steal_records(account_key, date_key, occurred_at);
CREATE TABLE IF NOT EXISTS social_visitor_records (
	id TEXT NOT NULL,
	account_key TEXT NOT NULL,
	occurred_at_ms INTEGER NOT NULL,
	player_id INTEGER,
	display_name TEXT,
	action_type INTEGER,
	payload_json TEXT NOT NULL,
	PRIMARY KEY (account_key, id)
);
CREATE INDEX IF NOT EXISTS idx_social_visitor_records_account_time
	ON social_visitor_records(account_key, occurred_at_ms);
CREATE TABLE IF NOT EXISTS social_dog_guard_cache (
	account_key TEXT PRIMARY KEY,
	state_json TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
```

- [ ] **Step 4: Implement storage methods**

Create storage DTOs:

```go
type SocialFriendRules struct {
	WhitelistEnabled bool     `json:"whitelistEnabled"`
	WhitelistScopes  []string `json:"whitelistScopes"`
	Whitelist        []string `json:"whitelist"`
	BlacklistEnabled bool     `json:"blacklistEnabled"`
	BlacklistScopes  []string `json:"blacklistScopes"`
	Blacklist        []string `json:"blacklist"`
	MaskedBlacklist  bool     `json:"maskedBlacklist"`
	MaskedMaxLevel   int      `json:"maskedMaxLevel"`
}

type SocialRecordRow struct {
	ID          string
	DateKey     string
	OccurredAt  string
	GID         int
	DisplayName string
	PayloadJSON string
}
```

Implement load/save rules, append/list steal records, append/list visitor records, and load/save dog guard cache with `INSERT ... ON CONFLICT`.

- [ ] **Step 5: Run tests to verify GREEN**

Run: `go test ./internal/storage -run 'TestSocial'`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/storage/migrations.go internal/storage/social.go internal/storage/social_test.go
git commit -m "feat: persist social state"
```

---

### Task 3: Friend State Service

**Files:**
- Create: `internal/farm/social/service.go`
- Create: `internal/farm/social/service_test.go`
- Modify: `app.go`
- Modify: `app_test.go`

- [ ] **Step 1: Write failing service test**

```go
func TestServiceStateLoadsRuntimeFriendsAndAppliesRules(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{"list": []any{
			map[string]any{"gid": float64(10001), "name": "A", "level": float64(12), "workCounts": map[string]any{"collect": float64(2)}},
			map[string]any{"gid": float64(10002), "name": "B", "level": float64(1), "workCounts": map[string]any{"help": float64(1)}},
		}},
	}}
	store := newMemoryStore()
	store.rules = FriendRules{Blacklist: []string{"10002"}, Whitelist: []string{"10001"}, MaskedBlacklist: true, MaskedMaxLevel: 1}
	service := NewService(store, caller, Options{AccountKey: "account-a"})

	state := service.State(context.Background(), StateRequest{Refresh: true})

	if !state.OK || len(state.Friends) != 2 {
		t.Fatalf("state = %#v", state)
	}
	if !state.Friends[0].Whitelisted || !state.Friends[0].Stealable {
		t.Fatalf("first friend not enriched: %#v", state.Friends[0])
	}
	if !state.Friends[1].Blacklisted || !state.Friends[1].MaskedBlocked {
		t.Fatalf("second friend not marked: %#v", state.Friends[1])
	}
}
```

- [ ] **Step 2: Run test to verify RED**

Run: `go test ./internal/farm/social -run TestServiceStateLoadsRuntimeFriendsAndAppliesRules`

Expected: FAIL because `Service`, `State`, and `StateRequest` are undefined.

- [ ] **Step 3: Implement service state**

Add `RuntimeCaller`, `Store`, `Service`, `StateRequest`, `State`, and `Summary`. `State` calls:

```go
value, err := s.caller.Call(ctx, "gameCtl.getFriendList", []any{map[string]any{
	"refresh":     req.Refresh,
	"sort":        true,
	"includeSelf": false,
	"waitRefresh": req.Refresh,
	"silent":      true,
}}, 30*time.Second)
```

Normalize both `[]any` and `{list: []any}` responses, enrich friends with rules and cached dog guard rows, and return `runtime_not_ready` if caller is nil.

- [ ] **Step 4: Add App Wails state method**

Add:

```go
func (a *App) FarmSocialState(refresh bool) social.State {
	return a.socialService().State(a.contextOrBackground(), social.StateRequest{Refresh: refresh})
}
```

Add `socialService()` that passes `a.store`, `a.supervisor`, and a runtime account key. Use `"default"` until account identity extraction is available.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/farm/social ./... -run 'TestServiceState|TestFarmSocialState'`

Expected: PASS after adding an App test that `FarmSocialState(false)` returns a structured state instead of panicking.

- [ ] **Step 6: Commit**

```bash
git add internal/farm/social/service.go internal/farm/social/service_test.go app.go app_test.go
git commit -m "feat: expose live social state"
```

---

### Task 4: Friend Actions And Protocol Block List

**Files:**
- Modify: `internal/farm/social/service.go`
- Test: `internal/farm/social/actions_test.go`
- Modify: `app.go`
- Modify: `app_test.go`

- [ ] **Step 1: Write failing action tests**

```go
func TestFriendActionProtectedGIDSkipsWithoutRuntimeCall(t *testing.T) {
	caller := &fakeRuntimeCaller{}
	service := NewService(newMemoryStore(), caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "steal", Target: "1184649322"})

	if !result.OK || result.Status != StatusSkipped {
		t.Fatalf("result = %#v", result)
	}
	if len(caller.calls) != 0 {
		t.Fatalf("protected action should not call runtime: %#v", caller.calls)
	}
}

func TestFriendProtocolBlockListCallsRuntime(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendBlockListByProtocol": map[string]any{"list": []any{map[string]any{"gid": float64(10001)}}},
	}}
	service := NewService(newMemoryStore(), caller, Options{AccountKey: "account-a"})

	result := service.ProtocolBlockList(context.Background(), false)

	if !result.OK || len(result.Friends) != 1 || result.Friends[0].GID != 10001 {
		t.Fatalf("block list = %#v", result)
	}
}
```

- [ ] **Step 2: Run tests to verify RED**

Run: `go test ./internal/farm/social -run 'TestFriendAction|TestFriendProtocol'`

Expected: FAIL because action methods are undefined.

- [ ] **Step 3: Implement actions**

Support `enter`, `steal`, `help`, `mischief`, `block`, `unblock`, `unblock_batch`, `blacklist_toggle`, and `whitelist_toggle`.

Runtime calls:

- `enter`: `gameCtl.enterFriendFarm`
- `block`: `gameCtl.blockFriendByProtocol`
- `unblock`: `gameCtl.unblockFriendByProtocol`
- `steal`: enter friend, read `gameCtl.getFarmStatus`, call `gameCtl.harvestLandsBatchByProtocol`, then append a steal record if protocol replies or collect deltas indicate real steal
- `help`: enter friend, call `gameCtl.triggerOneClickOperation` with `FARM_WORK`
- `mischief`: enter friend, read status, call `gameCtl.friendMischiefLandsBatch`

Use the proven patterns from `internal/farm/automation/runtime_friend.go` and keep action-specific source strings prefixed with `farm_go_social_`.

- [ ] **Step 4: Add Wails action methods**

Add:

```go
func (a *App) FarmSocialAction(input social.FriendActionRequest) social.ActionResult
func (a *App) FarmSocialProtocolBlockList(refresh bool) social.ProtocolBlockList
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/farm/social ./... -run 'TestFriendAction|TestFriendProtocol|TestFarmSocialAction'`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/farm/social/service.go internal/farm/social/actions_test.go app.go app_test.go
git commit -m "feat: add social friend actions"
```

---

### Task 5: Rankings And Visitor Records

**Files:**
- Create: `internal/farm/social/records.go`
- Test: `internal/farm/social/records_test.go`
- Modify: `internal/farm/social/service.go`
- Modify: `app.go`

- [ ] **Step 1: Write failing record tests**

```go
func TestStealRankingSummarizesDateFilteredRecords(t *testing.T) {
	records := []StealRecord{
		{ID: "a", GID: 10001, DisplayName: "A", OccurredAt: "2026-07-08T08:00:00Z", StealCount: 1},
		{ID: "b", GID: 10001, DisplayName: "A", OccurredAt: "2026-07-07T08:00:00Z", StealCount: 1},
		{ID: "c", GID: 10002, DisplayName: "B", OccurredAt: "2026-06-01T08:00:00Z", StealCount: 1},
	}
	window := ResolveDateRangeWindow("3d", time.Date(2026, 7, 8, 12, 0, 0, 0, time.Local))

	ranking := SummarizeStealRanking(FilterStealRecords(records, window))

	if len(ranking) != 1 || ranking[0].GID != 10001 || ranking[0].EventCount != 2 {
		t.Fatalf("ranking = %#v", ranking)
	}
}

func TestVisitorSummaryBuildsStolenFromMeRanking(t *testing.T) {
	records := []VisitorRecord{
		{ID: "v1", PlayerID: 10001, DisplayName: "A", ActionType: 1, Time: 1783497600, StealItemNum: 3},
		{ID: "v2", PlayerID: 10001, DisplayName: "A", ActionType: 1, Time: 1783497700, StealItemNum: 2},
	}
	summary := SummarizeVisitorRankings(records)
	if len(summary.StolenFromMe) != 1 || summary.StolenFromMe[0].StealCount != 5 {
		t.Fatalf("summary = %#v", summary)
	}
}
```

- [ ] **Step 2: Run tests to verify RED**

Run: `go test ./internal/farm/social -run 'TestStealRanking|TestVisitorSummary'`

Expected: FAIL because record types/functions are undefined.

- [ ] **Step 3: Implement record normalization**

Port reference behavior:

- record limit `1000`;
- date ranges `current`, `today`, `3d`, `7d`, `30d`, `all`;
- visitor action labels for visit, help, and steal;
- stolen-by-me ranking grouped by gid/display name;
- stolen-from-me ranking grouped by player id/display name;
- raw record arrays preserved for UI raw view.

- [ ] **Step 4: Implement service ranking method**

Add:

```go
func (s *Service) Rankings(ctx context.Context, req RankingRequest) RankingState
```

When `req.RefreshVisitors` is true, call `gameCtl.getVisitorRecords` and merge visitor records before summarizing.

- [ ] **Step 5: Add Wails method**

Add:

```go
func (a *App) FarmSocialRankings(input social.RankingRequest) social.RankingState
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/farm/social ./internal/storage ./... -run 'TestStealRanking|TestVisitorSummary|TestFarmSocialRankings'`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/farm/social/records.go internal/farm/social/records_test.go internal/farm/social/service.go app.go
git commit -m "feat: add social rankings"
```

---

### Task 6: Dog Guard Scanner

**Files:**
- Create: `internal/farm/social/dog_guard.go`
- Test: `internal/farm/social/dog_guard_test.go`
- Modify: `internal/farm/social/service.go`
- Modify: `app.go`

- [ ] **Step 1: Write failing scanner tests**

```go
func TestDogGuardScanStartScansFriendsAndPersistsRows(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{"list": []any{
			map[string]any{"gid": float64(10001), "name": "A"},
		}},
		"gameCtl.startRuntimeSpies":       map[string]any{"ok": true},
		"gameCtl.resetRuntimeSpyEvents":   map[string]any{"ok": true},
		"gameCtl.enterFriendFarm":         map[string]any{"ok": true},
		"gameCtl.inspectDogGuardSignals": map[string]any{"sourceScans": []any{
			map[string]any{"matches": []any{
				map[string]any{"path": "root.pet_dog.id", "value": float64(90021)},
			}},
		}},
	}}
	store := newMemoryStore()
	scanner := NewDogGuardScanner(store, caller, DogGuardOptions{AccountKey: "account-a", Sleep: func(time.Duration) {}})

	state := scanner.Start(context.Background(), DogGuardScanRequest{Refresh: true, Limit: 1})
	state = scanner.Wait(context.Background())

	if state.Running || state.HasGuardDogCount != 1 || len(state.Results) != 1 || !state.Results[0].HasGuardDog {
		t.Fatalf("state = %#v", state)
	}
}
```

- [ ] **Step 2: Run test to verify RED**

Run: `go test ./internal/farm/social -run TestDogGuardScanStartScansFriendsAndPersistsRows`

Expected: FAIL because scanner does not exist.

- [ ] **Step 3: Implement asynchronous scanner**

Implement `Start`, `Stop`, `Clear`, `State`, and `Wait`. The scan sequence per friend is:

1. `gameCtl.startRuntimeSpies`
2. `gameCtl.resetRuntimeSpyEvents`
3. `gameCtl.enterFriendFarm`
4. wait `AfterEnterWaitMS`
5. `gameCtl.inspectDogGuardSignals`
6. detect dog id `90021`
7. persist cache after every row

- [ ] **Step 4: Add service and Wails methods**

Add:

```go
func (a *App) FarmSocialDogGuardState() social.DogGuardState
func (a *App) FarmSocialDogGuardAction(input social.DogGuardActionRequest) social.DogGuardState
```

The `start` action must refuse to start a second scan and return the current state with `running: true`.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/farm/social ./... -run 'TestDogGuard|TestFarmSocialDogGuard'`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/farm/social/dog_guard.go internal/farm/social/dog_guard_test.go internal/farm/social/service.go app.go app_test.go
git commit -m "feat: add dog guard scan"
```

---

### Task 7: Import And Export

**Files:**
- Modify: `internal/farm/social/service.go`
- Test: `internal/farm/social/import_export_test.go`
- Modify: `app.go`

- [ ] **Step 1: Write failing import/export tests**

```go
func TestExportSelectedSocialGroups(t *testing.T) {
	store := newMemoryStore()
	store.rules = FriendRules{Blacklist: []string{"10002"}, Whitelist: []string{"10001"}}
	service := NewService(store, nil, Options{AccountKey: "account-a", Now: func() time.Time {
		return time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	}})

	payload := service.Export(context.Background(), ExportRequest{Groups: []string{"rules"}})

	if payload.Version != 1 || payload.AccountKey != "account-a" || payload.Rules == nil || payload.Rules.Blacklist[0] != "10002" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestImportRejectsUnknownVersion(t *testing.T) {
	service := NewService(newMemoryStore(), nil, Options{AccountKey: "account-a"})
	result := service.Import(context.Background(), ImportPayload{Version: 99, Groups: []string{"rules"}})
	if result.OK || result.Status != StatusFailed {
		t.Fatalf("result = %#v", result)
	}
}
```

- [ ] **Step 2: Run tests to verify RED**

Run: `go test ./internal/farm/social -run 'TestExportSelected|TestImportRejects'`

Expected: FAIL because export/import methods are undefined.

- [ ] **Step 3: Implement selected groups**

Support groups:

- `rules`;
- `friend_snapshot`;
- `steal_records`;
- `visitor_records`;
- `dog_guard`.

Reject unknown versions and unknown group names. Import only selected groups and write local persisted state; never import protocol block list into runtime.

- [ ] **Step 4: Add Wails methods**

Add:

```go
func (a *App) FarmSocialExport(input social.ExportRequest) social.ImportExportPayload
func (a *App) FarmSocialImport(input social.ImportExportPayload) social.ActionResult
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/farm/social ./... -run 'TestExport|TestImport|TestFarmSocialExport'`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/farm/social/service.go internal/farm/social/import_export_test.go app.go app_test.go
git commit -m "feat: add social import export"
```

---

### Task 8: Frontend Social Workbench

**Files:**
- Create: `frontend/src/views/SocialView.tsx`
- Test: `frontend/src/views/SocialView.test.tsx`
- Modify: `frontend/src/views/FarmWorkspaceView.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write failing render tests**

```tsx
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { SocialView, type SocialState } from './SocialView';

const state: SocialState = {
  ok: true,
  status: 'ok',
  message: '',
  summary: { totalFriends: 2, stealableFriends: 1, helpableFriends: 1, blacklisted: 1, whitelisted: 1, dogGuardCount: 1 },
  rules: { whitelistEnabled: true, whitelistScopes: ['steal'], whitelist: ['10001'], blacklistEnabled: true, blacklistScopes: ['mischief'], blacklist: ['10002'], maskedBlacklist: true, maskedMaxLevel: 1 },
  friends: [
    { gid: 10001, displayName: 'A', level: 12, workCounts: { collect: 2 }, stealable: true, helpable: false, mischiefable: false, whitelisted: true, blacklisted: false, maskedBlocked: false, protected: false, protocolBlocked: false, hasGuardDog: false },
    { gid: 10002, displayName: 'B', level: 1, workCounts: { help: 1 }, stealable: false, helpable: true, mischiefable: false, whitelisted: false, blacklisted: true, maskedBlocked: true, protected: false, protocolBlocked: false, hasGuardDog: true },
  ],
};

describe('SocialView', () => {
  it('renders friend list as the primary content', () => {
    const html = renderToStaticMarkup(<SocialView state={state} onRefresh={() => undefined} onAction={() => undefined} />);
    expect(html).toContain('好友社交');
    expect(html).toContain('好友列表');
    expect(html).toContain('A');
    expect(html).toContain('B');
    expect(html).toContain('好友功能');
    expect(html).toContain('排行榜');
    expect(html).toContain('导入导出');
  });

  it('renders ranking dialog tabs when opened', () => {
    const html = renderToStaticMarkup(<SocialView state={state} onRefresh={() => undefined} onAction={() => undefined} initialRankingOpen />);
    expect(html).toContain('偷取记录');
    expect(html).toContain('被偷记录');
    expect(html).toContain('访客记录');
    expect(html).toContain('原始数据');
  });
});
```

- [ ] **Step 2: Run frontend tests to verify RED**

Run: `cd frontend && npm test -- SocialView.test.tsx`

Expected: FAIL because `SocialView` does not exist.

- [ ] **Step 3: Implement `SocialView`**

Implement:

- header actions;
- summary metrics;
- friend table;
- search input;
- filter chips;
- row action buttons;
- friend feature menu;
- rule dialog;
- ranking dialog;
- protocol block dialog;
- dog guard dialog;
- import/export dialog.

The component receives data and callbacks as props; it does not import Wails directly.

- [ ] **Step 4: Wire App and workspace**

`App.tsx` owns state:

```tsx
const [socialState, setSocialState] = useState<SocialState | null>(null);
const refreshSocial = useCallback((refresh = false) => {
  FarmSocialState(refresh).then((value) => setSocialState(value as SocialState)).catch(console.error);
}, []);
```

Pass social props through `FarmWorkspaceView` only when `area === 'social'`.

- [ ] **Step 5: Run frontend tests**

Run: `cd frontend && npm test -- SocialView.test.tsx FarmWorkspaceView.test.tsx`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/views/SocialView.tsx frontend/src/views/SocialView.test.tsx frontend/src/views/FarmWorkspaceView.tsx frontend/src/App.tsx frontend/src/style.css
git commit -m "feat: add social workbench UI"
```

---

### Task 9: Bindings And Full Verification

**Files:**
- Modify generated Wails files under `frontend/wailsjs/go/main` and `frontend/wailsjs/go/models.ts`

- [ ] **Step 1: Regenerate Wails bindings**

Run the project's Wails generate command used in this repo. If the local Wails binary is unavailable, run the existing project build path that regenerates bindings.

Expected generated methods include:

- `FarmSocialState`
- `FarmSocialAction`
- `FarmSocialRankings`
- `FarmSocialProtocolBlockList`
- `FarmSocialDogGuardState`
- `FarmSocialDogGuardAction`
- `FarmSocialExport`
- `FarmSocialImport`

- [ ] **Step 2: Run backend tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 3: Run frontend tests**

Run: `cd frontend && npm test`

Expected: PASS.

- [ ] **Step 4: Build frontend**

Run: `cd frontend && npm run build`

Expected: PASS and `frontend/dist` updates.

- [ ] **Step 5: Verify no placeholder social page remains**

Run: `rg -n "待迁移好友|待迁移偷取排行|排行榜会复刻旧项目" frontend/src`

Expected: no matches for the old social placeholder copy.

- [ ] **Step 6: Commit verification/bindings**

```bash
git add frontend/wailsjs frontend/dist frontend/src
git commit -m "chore: verify social migration"
```

---

## Self-Review

- Spec coverage: friend list, rules, friend actions, protocol block list, invalid rule cleanup, batch removals, rankings, visitor records, dog guard, import/export, UI dialogs, and verification are covered by Tasks 1-9.
- Placeholder scan: the plan contains no unfinished placeholders. The word "placeholder" appears only in the verification command that removes old placeholder copy.
- Type consistency: public Go method names use the `FarmSocial*` prefix throughout; frontend callback names are prop-driven and do not call Wails directly inside `SocialView`.
