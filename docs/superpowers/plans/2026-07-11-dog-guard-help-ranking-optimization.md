# Dog Guard, Friend Help, and Ranking Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Unify account-scoped dog guard data, add guard-dog-only concurrent friend help, simplify scanning, and render social records as consistent time/name/detail queues.

**Architecture:** The social service remains the owner of account-scoped dog guard cache interpretation and maps cached GIDs onto the current friend list. The automation runtime reads the same cache through a narrow reader interface, filters current help candidates, and runs a fixed three-worker batch. React receives only the effective dog guard count for enablement, while ranking preferences remain account-scoped and gain a second view-mode field.

**Tech Stack:** Go 1.25, SQLite storage, Wails v2, React 18, TypeScript, Vitest, CSS.

---

## File Map

- `internal/farm/social/types.go`: scan request and ranking preference contracts.
- `internal/farm/social/dog_guard.go`: scan interval normalization and between-friend delay.
- `internal/farm/social/dog_guard_test.go`: scan contract and delay tests.
- `internal/farm/social/service.go`: apply cached guard-dog GIDs to current friends and normalize both ranking modes.
- `internal/farm/social/service_test.go`: effective current-friend dog count and preference tests.
- `internal/storage/social.go`: persist both ranking view modes; existing dog cache table remains unchanged.
- `internal/storage/social_test.go`: account isolation for dog cache and ranking preferences.
- `internal/farm/automation/config.go`: default `autoFarmFriendHelpGuardDogOnly` value.
- `internal/farm/automation/catalog_test.go`: default configuration coverage.
- `internal/farm/automation/runtime.go`: narrow dog guard reader dependency and social-store constructor.
- `internal/farm/automation/runtime_friend.go`: guard-only filtering, candidate limit, three-worker help batch, and aggregate result.
- `internal/farm/automation/runtime_friend_test.go`: filtering, account key, concurrency cap, continuation, and stable summary tests.
- `app.go`: pass the account-scoped social adapter to automation and map both ranking preference fields.
- `app_test.go`: Wails-facing preference and dog guard contracts.
- `frontend/src/views/SocialView.tsx`: simplified scan controls, completion toast, dual ranking modes, and queue row model.
- `frontend/src/views/SocialView.test.tsx`: scan toast/control and ranking queue tests.
- `frontend/src/views/AutomationView.tsx`: disabled guard-only help option.
- `frontend/src/views/AutomationView.test.tsx`: option availability and auto-clear tests.
- `frontend/src/views/FarmWorkspaceView.tsx`: pass effective dog guard count into automation.
- `frontend/src/views/FarmWorkspaceView.test.tsx`: workspace prop wiring coverage.
- `frontend/src/App.tsx`: refresh social state for automation and update scan request typing.
- `frontend/src/App.test.tsx`: account-scoped refresh and preference merge source checks.
- `frontend/src/style.css`: compact scan options, disabled help hint, and responsive record queue.
- `frontend/wailsjs/go/models.ts`: regenerated model fields after Go contract changes.
- `frontend/dist/*`: regenerated production frontend assets.

## Task 1: Mark Current Friends From the Account Dog Guard Cache

**Files:**
- Modify: `internal/farm/social/service_test.go`
- Modify: `internal/farm/social/service.go`
- Modify: `internal/storage/social_test.go`

- [ ] **Step 1: Write failing social-service tests for current-friend intersection**

Add a test that seeds one current guard dog, one current non-guard friend, and one stale cached guard dog:

```go
func TestServiceStateMarksOnlyCurrentFriendsFromDogGuardCache(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{"list": []any{
			map[string]any{"gid": float64(10001), "name": "A"},
			map[string]any{"gid": float64(10002), "name": "B"},
		}},
	}}
	store := &memoryStore{dogGuard: DogGuardState{Results: []DogGuardRow{
		{GID: 10001, Scanned: true, HasGuardDog: true},
		{GID: 99999, Scanned: true, HasGuardDog: true},
	}}}
	service := NewService(store, caller, Options{AccountKey: "gid:10000"})

	state := service.State(context.Background(), StateRequest{})

	if !state.Friends[0].HasGuardDog || state.Friends[1].HasGuardDog {
		t.Fatalf("friend marks = %#v", state.Friends)
	}
	if state.Summary.DogGuardCount != 1 {
		t.Fatalf("dog guard count = %d, want 1", state.Summary.DogGuardCount)
	}
}
```

- [ ] **Step 2: Run the test and verify RED**

Run:

```powershell
go test ./internal/farm/social -run TestServiceStateMarksOnlyCurrentFriendsFromDogGuardCache -v
```

Expected: FAIL because `Service.State` does not load or apply the dog guard cache.

- [ ] **Step 3: Implement a focused cache-to-friend mapping helper**

In `service.go`, load `DogGuardState` only when the store implements `DogGuardStore`, then mark current rows before summarizing:

```go
func applyDogGuardState(friends []FriendRow, state DogGuardState) {
	guardGIDs := make(map[int]bool, len(state.Results))
	for _, row := range state.Results {
		if row.GID > 0 && row.HasGuardDog {
			guardGIDs[row.GID] = true
		}
	}
	for index := range friends {
		friends[index].HasGuardDog = guardGIDs[friends[index].GID]
	}
}
```

Call it after `enrichFriendRows` and before `summarizeFriends`. If cache loading fails, return a structured failed social state instead of silently showing an incorrect count.

- [ ] **Step 4: Add storage account-isolation coverage for dog cache**

Extend `TestSocialDogGuardCacheRoundTrip` to save distinct JSON for `gid:10001` and `gid:10002`, then assert each load returns only its own payload.

- [ ] **Step 5: Run package tests and verify GREEN**

```powershell
go test ./internal/farm/social ./internal/storage
```

Expected: PASS.

- [ ] **Step 6: Commit Task 1**

```powershell
git add internal/farm/social/service.go internal/farm/social/service_test.go internal/storage/social_test.go
git commit -m "feat: sync dog guard cache with current friends"
```

## Task 2: Replace Scan Limits and Waits With a 300ms Scan Interval

**Files:**
- Modify: `internal/farm/social/types.go`
- Modify: `internal/farm/social/dog_guard.go`
- Modify: `internal/farm/social/dog_guard_test.go`
- Modify: `app_test.go`

- [ ] **Step 1: Write failing tests for the new scan request contract**

Replace the old wait-focused test with tests that capture injected sleeps:

```go
func TestDogGuardScanUsesDefaultIntervalBetweenAllFriends(t *testing.T) {
	var sleeps []time.Duration
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{"list": []any{
			map[string]any{"gid": float64(10001)},
			map[string]any{"gid": float64(10002)},
			map[string]any{"gid": float64(10003)},
		}},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": true},
	}}
	scanner := NewDogGuardScanner(&memoryStore{}, caller, DogGuardOptions{
		AccountKey: "gid:1",
		Sleep: func(value time.Duration) { sleeps = append(sleeps, value) },
	})

	scanner.Start(context.Background(), DogGuardScanRequest{})
	state := scanner.Wait(context.Background())

	if state.Total != 3 || state.Scanned != 3 {
		t.Fatalf("state = %#v", state)
	}
	want := []time.Duration{300 * time.Millisecond, 300 * time.Millisecond}
	if !reflect.DeepEqual(sleeps, want) {
		t.Fatalf("sleeps = %#v, want %#v", sleeps, want)
	}
}
```

Add a second test for `ScanIntervalMS: 125` and a normalization test for non-positive and values above `60000`.

- [ ] **Step 2: Run the tests and verify RED**

```powershell
go test ./internal/farm/social -run DogGuardScan -v
```

Expected: FAIL because `ScanIntervalMS` does not exist and `Limit`, `EnterWaitMS`, and `AfterEnterWaitMS` still drive behavior.

- [ ] **Step 3: Change the Go request model**

Use this request shape:

```go
type DogGuardScanRequest struct {
	Refresh         bool `json:"refresh,omitempty"`
	SkipScanned     bool `json:"skipScanned,omitempty"`
	ExcludeGuardDog bool `json:"excludeGuardDog,omitempty"`
	ScanIntervalMS  int  `json:"scanIntervalMs,omitempty"`
}
```

Normalize it with explicit constants:

```go
const (
	defaultDogGuardScanIntervalMS = 300
	maxDogGuardScanIntervalMS     = 60000
)

func normalizeDogGuardScanRequest(req DogGuardScanRequest) DogGuardScanRequest {
	if req.ScanIntervalMS <= 0 {
		req.ScanIntervalMS = defaultDogGuardScanIntervalMS
	}
	if req.ScanIntervalMS > maxDogGuardScanIntervalMS {
		req.ScanIntervalMS = maxDogGuardScanIntervalMS
	}
	return req
}
```

Remove candidate truncation and use the runtime protocol's internal fixed reply wait. In the friend loop, sleep only when another candidate remains and stop has not been requested.

- [ ] **Step 4: Update Wails-facing contract tests**

In `app_test.go`, start or clear the scanner with the new request type and assert the returned state remains structured. Remove all construction of deleted fields.

- [ ] **Step 5: Run tests and verify GREEN**

```powershell
go test ./internal/farm/social ./...
```

Expected: PASS.

- [ ] **Step 6: Commit Task 2**

```powershell
git add internal/farm/social/types.go internal/farm/social/dog_guard.go internal/farm/social/dog_guard_test.go app_test.go
git commit -m "feat: simplify dog guard scan timing"
```

## Task 3: Persist Both Ranking View Modes Per Account

**Files:**
- Modify: `internal/farm/social/types.go`
- Modify: `internal/farm/social/service.go`
- Modify: `internal/farm/social/service_test.go`
- Modify: `internal/storage/social.go`
- Modify: `internal/storage/social_test.go`
- Modify: `app.go`
- Modify: `app_test.go`

- [ ] **Step 1: Write failing normalization and round-trip tests**

Update tests to use both fields:

```go
preferences := RankingPreferences{
	StolenByMeViewMode:   "ranking",
	StolenFromMeViewMode: "timeline",
}
saved, err := service.SaveRankingPreferences(context.Background(), preferences)
if err != nil || saved != preferences {
	t.Fatalf("saved = %#v, err = %v", saved, err)
}
```

In storage tests, save opposite combinations for two GID accounts and assert isolation. Also assert an invalid value normalizes to `timeline` independently for each field.

- [ ] **Step 2: Run tests and verify RED**

```powershell
go test ./internal/farm/social ./internal/storage ./ -run RankingPreferences -v
```

Expected: FAIL because `StolenByMeViewMode` is absent.

- [ ] **Step 3: Extend social and storage preference models**

```go
type RankingPreferences struct {
	StolenByMeViewMode   string `json:"stolenByMeViewMode"`
	StolenFromMeViewMode string `json:"stolenFromMeViewMode"`
}
```

Normalize each field with the same rule:

```go
func normalizeRankingViewMode(value string) string {
	if value == "ranking" {
		return "ranking"
	}
	return "timeline"
}
```

Apply this helper in both `social.NormalizeRankingPreferences` and `storage.normalizeSocialRankingPreferences`. Update `socialStorageAdapter` mappings in `app.go` to carry both fields.

- [ ] **Step 4: Update app exposure tests**

Assert `FarmSocialRankings`, `FarmSocialRankingPreferences`, stale-scope rejection, and save/load round trips preserve both fields.

- [ ] **Step 5: Run tests and verify GREEN**

```powershell
go test ./internal/farm/social ./internal/storage ./
```

Expected: PASS.

- [ ] **Step 6: Commit Task 3**

```powershell
git add internal/farm/social/types.go internal/farm/social/service.go internal/farm/social/service_test.go internal/storage/social.go internal/storage/social_test.go app.go app_test.go
git commit -m "feat: persist both social ranking modes"
```

## Task 4: Add Guard-Only Filtering and Three-Worker Friend Help

**Files:**
- Modify: `internal/farm/automation/config.go`
- Modify: `internal/farm/automation/catalog_test.go`
- Modify: `internal/farm/automation/runtime.go`
- Modify: `internal/farm/automation/runtime_friend.go`
- Modify: `internal/farm/automation/runtime_friend_test.go`
- Modify: `app.go`

- [ ] **Step 1: Write failing default-config and guard filtering tests**

Assert the new default is false:

```go
if got := DefaultConfig()["autoFarmFriendHelpGuardDogOnly"]; got != false {
	t.Fatalf("guard-only default = %#v", got)
}
```

Create a recording dog guard reader:

```go
type recordingDogGuardReader struct {
	accountKey string
	state      social.DogGuardState
	err        error
}

func (r *recordingDogGuardReader) LoadDogGuardState(_ context.Context, accountKey string) (social.DogGuardState, error) {
	r.accountKey = accountKey
	return r.state, r.err
}
```

Test that current helpable friends `10001` and `10002` are reduced to `10002` when only that GID has `HasGuardDog: true`, and assert the reader received the facade's account key.

- [ ] **Step 2: Write a failing concurrency and continuation test**

Use a custom runtime caller whose inspect calls block on a release channel and tracks the maximum active calls with a mutex. Supply at least five helpable friends, release the calls, and assert:

```go
if caller.maxActive > 3 {
	t.Fatalf("max active calls = %d, want <= 3", caller.maxActive)
}
if caller.maxActive != 3 {
	t.Fatalf("max active calls = %d, want exactly 3", caller.maxActive)
}
```

Make one candidate return a protocol error and the others succeed. Assert later candidates still receive help calls and the final message includes success and failure counts.

- [ ] **Step 3: Run tests and verify RED**

```powershell
go test ./internal/farm/automation -run 'FriendHelp|DefaultConfig' -v
```

Expected: FAIL because friend help is serial, stops after one success, and has no dog cache dependency.

- [ ] **Step 4: Add the account-scoped social reader dependency**

In `runtime.go`:

```go
type DogGuardStateReader interface {
	LoadDogGuardState(ctx context.Context, accountKey string) (social.DogGuardState, error)
}

type RuntimeFacade struct {
	caller              RuntimeCaller
	config              map[string]any
	stealRecordWriter   StealRecordWriter
	dogGuardStateReader DogGuardStateReader
	accountKey          string
}
```

Keep the existing steal-recorder constructor for current tests. Add a social-store constructor that accepts a value implementing both `StealRecordWriter` and `DogGuardStateReader`, then update `app.executeFarmAutomationTaskForAccount` to use it with `socialStorageAdapter` and the captured account key.

- [ ] **Step 5: Implement filtering, limiting, and worker results**

Add:

```go
const friendHelpConcurrency = 3

type friendHelpCandidateResult struct {
	Index        int
	GID          int
	LandCount    int
	Success      bool
	Skipped      bool
	ErrorMessage string
}
```

Implement these focused helpers:

```go
func limitFriendHelpCandidates(friends []map[string]any, config map[string]any) []map[string]any
func filterGuardDogHelpCandidates(friends []map[string]any, guardGIDs map[int]bool) []map[string]any
func (r RuntimeFacade) loadGuardDogGIDs(ctx context.Context) (map[int]bool, error)
func (r RuntimeFacade) runFriendHelpCandidates(ctx context.Context, friends []map[string]any) []friendHelpCandidateResult
func (r RuntimeFacade) runFriendHelpCandidate(ctx context.Context, index int, friend map[string]any) friendHelpCandidateResult
```

The worker-pool shape should match `runFriendStealCandidates`: buffered results, three workers or fewer, and `sort.SliceStable` by `Index` before aggregation.

- [ ] **Step 6: Aggregate the whole batch without early return**

`runFriendHelp` must:

1. Read the current friend list.
2. Select helpable candidates.
3. If guard-only is enabled, load the same-account cache and filter by GID.
4. Apply `autoFarmFriendHelpMaxFriends`.
5. Run all selected candidates through the three-worker pool.
6. Return OK when at least one succeeds, including success, item, skip, and failure counts.
7. Return failed only when no candidate succeeds and at least one candidate errors.
8. Return OK skipped when all candidates have no helpable lands or the guard-only set is empty.

- [ ] **Step 7: Run tests and verify GREEN**

```powershell
go test ./internal/farm/automation ./
```

Expected: PASS.

- [ ] **Step 8: Commit Task 4**

```powershell
git add internal/farm/automation/config.go internal/farm/automation/catalog_test.go internal/farm/automation/runtime.go internal/farm/automation/runtime_friend.go internal/farm/automation/runtime_friend_test.go app.go
git commit -m "feat: batch friend help for guard dog friends"
```

## Task 5: Simplify Scan UI and Add the Guard-Only Help Control

**Files:**
- Modify: `frontend/src/views/SocialView.test.tsx`
- Modify: `frontend/src/views/SocialView.tsx`
- Modify: `frontend/src/views/AutomationView.test.tsx`
- Modify: `frontend/src/views/AutomationView.tsx`
- Modify: `frontend/src/views/FarmWorkspaceView.test.tsx`
- Modify: `frontend/src/views/FarmWorkspaceView.tsx`
- Modify: `frontend/src/App.test.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write failing SocialView scan control and toast tests**

Update the scan-options test to assert:

```ts
expect(html).toContain('扫描间隔');
expect(html).toContain('value="300"');
expect(html).not.toContain('扫描数量');
expect(html).not.toContain('进入好友等待');
expect(html).not.toContain('读取前等待');
```

Add a React renderer test that updates dog state from running to naturally finished and expects exactly one toast containing:

```text
已扫描到 2 名护主犬好友，已自动标记
```

Update the same renderer with the identical `finishedAt` again and assert the toast is not recreated. Add stopped and error cases that do not show success copy.

- [ ] **Step 2: Write failing AutomationView availability tests**

Render friend settings with `dogGuardFriendCount={0}` and assert the named checkbox is disabled and the hint is visible. Render with count `2` and assert it is enabled.

Use a renderer test with a state whose config starts true, change the prop to count `0`, and assert the draft checkbox becomes unchecked.

- [ ] **Step 3: Run focused frontend tests and verify RED**

```powershell
npm test -- src/views/SocialView.test.tsx src/views/AutomationView.test.tsx src/views/FarmWorkspaceView.test.tsx src/App.test.tsx
```

Workdir: `frontend`

Expected: FAIL because the new props, request fields, and completion toast do not exist.

- [ ] **Step 4: Implement the simplified scan controls**

In `SocialView.tsx`:

```ts
const [dogScanIntervalMs, setDogScanIntervalMs] = useState(300);
```

Send only:

```ts
await onDogGuardAction?.({
  action: 'start',
  refresh: true,
  scanIntervalMs: dogScanIntervalMs,
  skipScanned: dogSkipScanned,
  excludeGuardDog: dogExcludeGuardDog,
});
```

Update the dog state type to include `startedAt`, `finishedAt`, and the request prop type to include `scanIntervalMs`. Replace `DogGuardOptionsPanel` with one numeric field and the two existing checkboxes.

- [ ] **Step 5: Add a deduplicated completion toast**

Track the previous running state and last handled `finishedAt` in refs. On a new natural completion (`!running`, `!stopRequested`, no error, non-empty new `finishedAt`), set the existing `visibleActionResult` to the approved success message. Reuse `SOCIAL_TOAST_AUTO_DISMISS_MS`; do not add a second toast component.

- [ ] **Step 6: Wire effective count into automation**

Add `dogGuardFriendCount?: number` to `AutomationViewProps`. Pass `socialState?.summary.dogGuardCount || 0` through `FarmWorkspaceView`.

Add this checkbox under the friend-help controls:

```tsx
<label className="automation-settings-check" title={dogGuardFriendCount > 0 ? undefined : '请先读取护主犬好友'}>
  <input
    checked={configBool('autoFarmFriendHelpGuardDogOnly', false)}
    disabled={dogGuardFriendCount <= 0}
    name="config-autoFarmFriendHelpGuardDogOnly"
    type="checkbox"
    onChange={(event) => updateConfig('autoFarmFriendHelpGuardDogOnly', event.currentTarget.checked)}
  />
  <span>仅帮助护主犬好友</span>
</label>
```

Pass the count into `renderSettingsGroup`. Add an effect that clears the draft config when the count becomes zero.

- [ ] **Step 7: Refresh social state while automation is active**

Split the current social-tab effect in `App.tsx`: refresh `FarmSocialState(false)` when either `social` or `automation` is active; keep rankings, protocol data, and dog scanner polling limited to the social page. Update `runSocialDogGuardAction` typing to the new request fields.

- [ ] **Step 8: Adjust compact styling**

Change `.social-dog-options` from five tracks to three stable tracks and add disabled/hint styling for the help option without introducing a new card.

- [ ] **Step 9: Run focused tests and verify GREEN**

```powershell
npm test -- src/views/SocialView.test.tsx src/views/AutomationView.test.tsx src/views/FarmWorkspaceView.test.tsx src/App.test.tsx
```

Workdir: `frontend`

Expected: PASS.

- [ ] **Step 10: Commit Task 5**

```powershell
git add frontend/src/views/SocialView.tsx frontend/src/views/SocialView.test.tsx frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx frontend/src/views/FarmWorkspaceView.tsx frontend/src/views/FarmWorkspaceView.test.tsx frontend/src/App.tsx frontend/src/App.test.tsx frontend/src/style.css
git commit -m "feat: expose guard dog scan and help controls"
```

## Task 6: Render All Social Records as Time, Friend, Detail Queues

**Files:**
- Modify: `frontend/src/views/SocialView.test.tsx`
- Modify: `frontend/src/views/SocialView.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write failing mode and column-order tests**

Extend fixture preferences:

```ts
preferences: {
  stolenByMeViewMode: 'timeline',
  stolenFromMeViewMode: 'timeline',
},
```

Add tests that:

- show mode tabs for `stolenByMe` and `stolenFromMe`;
- do not show mode tabs for `visitors`;
- save one field without overwriting the other;
- render `social-ranking-time`, then `social-ranking-name`, then `social-ranking-detail` in each row;
- render the latest event time in ranking mode;
- render individual steal records in steal timeline mode.

- [ ] **Step 2: Run SocialView tests and verify RED**

```powershell
npm test -- src/views/SocialView.test.tsx
```

Workdir: `frontend`

Expected: FAIL because stolen-by-me is always aggregated and timestamps are embedded in detail strings.

- [ ] **Step 3: Generalize ranking mode state and saving**

Use one shared type:

```ts
export type RankingViewMode = 'timeline' | 'ranking';

export type RankingPreferences = {
  stolenByMeViewMode: RankingViewMode;
  stolenFromMeViewMode: RankingViewMode;
};
```

Maintain committed refs for both fields. Replace the single-field saver with a function that merges the requested field into the complete current preference object before calling `onSaveRankingPreferences`, preserving stale-request rollback behavior.

- [ ] **Step 4: Replace display rows with explicit columns**

```ts
type RankingDisplayRow = {
  key: string | number;
  time: string;
  name: string;
  detail: string;
  rank?: number;
  actionTarget?: string;
};
```

Build rows as follows:

- stolen-by-me timeline: `stolenByMeRecords`, newest first;
- stolen-by-me ranking: `stolenByMe`, with `lastOccurredAt` and aggregated items;
- stolen-from-me timeline: visitor action type `1`;
- stolen-from-me ranking: `stolenFromMe`, with `lastTime`;
- visitors: all visitor records, timeline only.

Remove time concatenation from `detail`; `formatRankingTime` populates the dedicated time field.

- [ ] **Step 5: Render the queue and responsive CSS**

Render each row in this DOM order:

```tsx
<time className="social-ranking-time">{row.time || '-'}</time>
<strong className="social-ranking-name">{row.rank ? `第 ${row.rank} · ` : ''}{row.name}</strong>
<span className="social-ranking-detail">{row.detail}</span>
```

Keep the protocol-block icon button as a fourth auto-width action track only for relevant rows. Use stable grid columns around `172px 190px minmax(0, 1fr)` and stack them in the same order at narrow widths.

- [ ] **Step 6: Run tests and verify GREEN**

```powershell
npm test -- src/views/SocialView.test.tsx
```

Workdir: `frontend`

Expected: PASS.

- [ ] **Step 7: Commit Task 6**

```powershell
git add frontend/src/views/SocialView.tsx frontend/src/views/SocialView.test.tsx frontend/src/style.css
git commit -m "feat: align social ranking record queues"
```

## Task 7: Regenerate Bindings and Verify the Complete Change

**Files:**
- Regenerate: `frontend/wailsjs/go/models.ts`
- Regenerate if changed: `frontend/wailsjs/go/main/App.d.ts`
- Regenerate if changed: `frontend/wailsjs/go/main/App.js`
- Build output, not committed: `frontend/dist/index.html`
- Build output, not committed: `frontend/dist/assets/*`

- [ ] **Step 1: Run Go formatting**

```powershell
gofmt -w app.go app_test.go internal/farm/social/types.go internal/farm/social/service.go internal/farm/social/service_test.go internal/farm/social/dog_guard.go internal/farm/social/dog_guard_test.go internal/storage/social.go internal/storage/social_test.go internal/farm/automation/config.go internal/farm/automation/catalog_test.go internal/farm/automation/runtime.go internal/farm/automation/runtime_friend.go internal/farm/automation/runtime_friend_test.go
```

- [ ] **Step 2: Regenerate Wails bindings**

```powershell
wails generate module
```

Expected: generated TypeScript models contain `scanIntervalMs`, omit the deleted scan fields, and contain both ranking preference fields. Preserve unrelated pre-existing changes in `frontend/wailsjs/go/models.ts`; inspect the diff before staging.

- [ ] **Step 3: Run all Go tests**

```powershell
go test ./...
```

Expected: PASS with no package failures.

- [ ] **Step 4: Run all frontend tests**

```powershell
npm test
```

Workdir: `frontend`

Expected: PASS.

- [ ] **Step 5: Build the production frontend**

```powershell
npm run build
```

Workdir: `frontend`

Expected: TypeScript and Vite build succeed. This repository embeds `frontend/dist` from `main.go`, so verify `frontend/dist/index.html` references the newly generated hashed asset and its timestamp is newer than the pre-build file.

- [ ] **Step 6: Build the Wails application**

```powershell
wails build
```

Expected: PASS and produce the configured desktop binary.

- [ ] **Step 7: Inspect final diffs and generated assets**

```powershell
git status --short
git diff --check
git diff --stat
Get-Content -Raw frontend/dist/index.html
```

Confirm no unrelated files are staged, deleted scan fields are absent from source and generated models, and the new hashed asset exists under `frontend/dist/assets`.

- [ ] **Step 8: Commit generated bindings and build output**

Stage only files produced by this feature, leaving `frontend-vite.out.log` and unrelated user changes untouched:

```powershell
git add frontend/wailsjs/go/models.ts frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/main/App.js
git commit -m "build: regenerate dog guard social bindings"
```

- [ ] **Step 9: Final acceptance check**

Verify these observable outcomes:

1. Scan settings show interval `300ms` plus the two existing filters.
2. A natural complete scan shows one success toast and updates friend pills and the top count.
3. Clearing the account cache removes current marks and disables the guard-only help option.
4. Guard-only help never targets unmarked current friends and handles up to three friends concurrently.
5. Steal and stolen-from-me tabs have both modes; visitors remain timeline-only.
6. Every visible record row orders time, friend, and detail without overlap at desktop and narrow widths.
