# Friend Help Daily Limit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce `autoFarmFriendHelpDailyLimit` by counting friends successfully helped by automatic `friend_help` runs per account and local day.

**Architecture:** Keep the daily date and count as runtime-owned automation configuration, following the existing daily task markers. The runtime limits automatic candidates using the remaining allowance and returns a structured successful-friend count; the application serializes and persists that increment for the captured account.

**Tech Stack:** Go, existing automation runtime and account-scoped SQLite settings, Go `testing` package

---

## File Map

- Modify `internal/farm/automation/catalog.go`: expose the successful friend count on `ActionResult`.
- Modify `internal/farm/automation/daily_state.go`: own daily friend-help date/count keys and pure count helpers.
- Modify `internal/farm/automation/daily_state_merge_test.go`: verify reset, unlimited, increment, and merge behavior.
- Modify `internal/farm/automation/runtime_friend.go`: enforce the remaining automatic allowance and return the successful friend count.
- Modify `internal/farm/automation/runtime_friend_test.go`: cover automatic limits, partial success, reached limits, manual bypass, and zero-as-unlimited.
- Modify `app.go`: persist successful automatic friend counts under the captured account.
- Modify `app_test.go`: verify persistence, account isolation, manual exclusion, and settings-save preservation.

### Task 1: Daily Count State

**Files:**
- Modify: `internal/farm/automation/daily_state.go`
- Test: `internal/farm/automation/daily_state_merge_test.go`

- [ ] **Step 1: Write failing helper and merge tests**

Add tests that use a fixed local timestamp and assert:

```go
func TestFriendHelpDailyStateCountsCurrentDateOnly(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.Local)
	config := map[string]any{
		"autoFarmFriendHelpDailyLimit": 5,
		FriendHelpDailyCountDateConfigKey: "2026-07-17",
		FriendHelpDailyCountConfigKey: 3,
	}
	remaining, limited := FriendHelpDailyRemaining(config, now)
	if !limited || remaining != 2 {
		t.Fatalf("remaining = %d, limited = %v", remaining, limited)
	}
	config[FriendHelpDailyCountDateConfigKey] = "2026-07-16"
	remaining, limited = FriendHelpDailyRemaining(config, now)
	if !limited || remaining != 5 {
		t.Fatalf("new day remaining = %d, limited = %v", remaining, limited)
	}
}

func TestFriendHelpDailyStateTreatsZeroAsUnlimited(t *testing.T) {
	remaining, limited := FriendHelpDailyRemaining(map[string]any{"autoFarmFriendHelpDailyLimit": 0}, time.Now())
	if limited || remaining != 0 {
		t.Fatalf("remaining = %d, limited = %v", remaining, limited)
	}
}

func TestMarkFriendHelpDailySuccessesResetsAndIncrements(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.Local)
	config := MarkFriendHelpDailySuccesses(map[string]any{
		FriendHelpDailyCountDateConfigKey: "2026-07-16",
		FriendHelpDailyCountConfigKey: 20,
	}, now, 2)
	if config[FriendHelpDailyCountDateConfigKey] != "2026-07-17" || config[FriendHelpDailyCountConfigKey] != 2 {
		t.Fatalf("daily state = %#v", config)
	}
	config = MarkFriendHelpDailySuccesses(config, now, 3)
	if config[FriendHelpDailyCountConfigKey] != 5 {
		t.Fatalf("daily count = %#v", config[FriendHelpDailyCountConfigKey])
	}
}
```

Extend `TestMergeRuntimeDailyStatePreservesOwnedMarkersOnly` so stale incoming values cannot replace the current friend-help date/count.

- [ ] **Step 2: Run the tests and verify RED**

Run:

```powershell
go test ./internal/farm/automation -run 'TestFriendHelpDailyState|TestMarkFriendHelpDailySuccesses|TestMergeRuntimeDailyState' -count=1
```

Expected: compilation fails because the new keys and helpers do not exist.

- [ ] **Step 3: Implement minimal daily-state helpers**

In `daily_state.go`, add exported runtime-owned key constants and pure helpers:

```go
const (
	FriendHelpDailyCountDateConfigKey = "autoFarmFriendHelpDailyCountDate"
	FriendHelpDailyCountConfigKey     = "autoFarmFriendHelpDailyCount"
)

func FriendHelpDailyRemaining(config map[string]any, now time.Time) (int, bool) {
	limit := intFromAny(config["autoFarmFriendHelpDailyLimit"])
	if limit <= 0 {
		return 0, false
	}
	count := 0
	if strings.TrimSpace(fmt.Sprint(config[FriendHelpDailyCountDateConfigKey])) == automationDate(now) {
		count = intFromAny(config[FriendHelpDailyCountConfigKey])
	}
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	return remaining, true
}

func MarkFriendHelpDailySuccesses(config map[string]any, now time.Time, successCount int) map[string]any {
	next := cloneConfig(config)
	date := automationDate(now)
	count := 0
	if strings.TrimSpace(fmt.Sprint(next[FriendHelpDailyCountDateConfigKey])) == date {
		count = intFromAny(next[FriendHelpDailyCountConfigKey])
	}
	if successCount < 0 {
		successCount = 0
	}
	next[FriendHelpDailyCountDateConfigKey] = date
	next[FriendHelpDailyCountConfigKey] = count + successCount
	return next
}
```

Add both keys to `MergeRuntimeDailyState`'s preserved key list.

- [ ] **Step 4: Run the focused tests and verify GREEN**

Run the Step 2 command. Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/farm/automation/daily_state.go internal/farm/automation/daily_state_merge_test.go
git commit -m "feat: track friend help daily count"
```

### Task 2: Runtime Enforcement

**Files:**
- Modify: `internal/farm/automation/catalog.go`
- Modify: `internal/farm/automation/runtime_friend.go`
- Test: `internal/farm/automation/runtime_friend_test.go`

- [ ] **Step 1: Write failing runtime behavior tests**

Add focused cases that configure `automationTrigger: "auto"` and today's persisted count:

```go
func TestRuntimeFacadeFriendHelpLimitsAutomaticBatchToDailyRemaining(t *testing.T) {
	now := time.Now()
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": friendHelpTestFriends(10001, 10002, 10003),
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": true, "workLandIds": map[string]any{"water": []any{float64(1)}}},
		"gameCtl.friendFarmingByProtocol": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"automationTrigger": "auto",
		"autoFarmFriendHelpMaxFriends": 5,
		"autoFarmFriendHelpDailyLimit": 3,
		FriendHelpDailyCountDateConfigKey: now.Format("2006-01-02"),
		FriendHelpDailyCountConfigKey: 2,
	})
	result := facade.RunTask(context.Background(), "friend_help")
	if !result.OK || result.SuccessfulFriends != 1 {
		t.Fatalf("result = %#v", result)
	}
	helpCalls := 0
	for _, method := range calledRuntimeMethods(caller.calls) {
		if method == "gameCtl.friendFarmingByProtocol" {
			helpCalls++
		}
	}
	if helpCalls != 1 {
		t.Fatalf("help calls = %d", helpCalls)
	}
}
```

Add separate tests proving:

- reached automatic limit returns an OK skip and performs zero runtime calls;
- per-batch max wins when it is smaller than remaining daily allowance;
- only successful friends populate `SuccessfulFriends` in a partial-success batch;
- `automationTrigger: "manual"` ignores an exhausted limit;
- automatic daily limit `0` remains unlimited.

- [ ] **Step 2: Run runtime tests and verify RED**

Run:

```powershell
go test ./internal/farm/automation -run 'TestRuntimeFacadeFriendHelp.*(Daily|Automatic|Manual|Zero|Success)' -count=1
```

Expected: compilation fails because `SuccessfulFriends` is missing, then behavior assertions fail until enforcement is implemented.

- [ ] **Step 3: Add structured success count and enforce the allowance**

Extend `ActionResult` in `catalog.go`:

```go
SuccessfulFriends int `json:"successfulFriends,omitempty"`
```

At the beginning of `runFriendHelp`, when `automationTrigger` is `auto`, call `FriendHelpDailyRemaining`. Return an OK skip before checking `r.caller` when limited and remaining is zero.

After friend rules are applied, cap candidates using a helper that applies both positive limits:

```go
func limitFriendHelpCandidates(friends []map[string]any, config map[string]any, dailyRemaining int, dailyLimited bool) []map[string]any {
	limit := len(friends)
	if maxFriends := intFromAny(config["autoFarmFriendHelpMaxFriends"]); maxFriends > 0 && maxFriends < limit {
		limit = maxFriends
	}
	if dailyLimited && dailyRemaining < limit {
		limit = dailyRemaining
	}
	return friends[:limit]
}
```

Set `SuccessfulFriends: successCount` on the successful batch result. Failed and skipped candidates therefore consume no allowance.

- [ ] **Step 4: Run all friend-help tests and verify GREEN**

Run:

```powershell
go test ./internal/farm/automation -run FriendHelp -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/farm/automation/catalog.go internal/farm/automation/runtime_friend.go internal/farm/automation/runtime_friend_test.go
git commit -m "fix: enforce automatic friend help daily limit"
```

### Task 3: Account-Scoped Persistence

**Files:**
- Modify: `app.go`
- Test: `app_test.go`

- [ ] **Step 1: Write failing application tests**

Add an integration test with a temporary store that runs scheduled `friend_help` twice with limit `2` and verifies:

```go
state := app.FarmAutomationState()
if state.Config[automation.FriendHelpDailyCountDateConfigKey] != time.Now().Format("2006-01-02") {
	t.Fatalf("daily date = %#v", state.Config)
}
if intFromAny(state.Config[automation.FriendHelpDailyCountConfigKey], 0) != 2 {
	t.Fatalf("daily count = %#v", state.Config)
}
```

The second scheduled run must skip without runtime calls. Add companion tests that a manual run leaves both keys absent, account A's scheduled run does not modify account B, and `SaveFarmAutomationState` preserves the runtime-owned values from current state over stale UI values.

- [ ] **Step 2: Run application tests and verify RED**

Run:

```powershell
go test . -run 'TestRunScheduledFarmAutomationFriendHelp|TestRunFarmAutomationFriendHelpManual|TestFriendHelpDailyCountAccount|TestSaveFarmAutomationStatePreservesRuntimeOwnedDailyMarkers' -count=1
```

Expected: count persistence assertions fail because automatic results are not yet stored.

- [ ] **Step 3: Persist automatic successful friend counts**

After `facade.RunTask` in `executeFarmAutomationTaskForAccount`, add:

```go
if trigger == "auto" && result.TaskID == "friend_help" && result.SuccessfulFriends > 0 {
	a.markFriendHelpDailySuccessesForAccount(accountKey, now, result.SuccessfulFriends)
}
```

Add the account-scoped writer following the existing daily-state lock and save pattern:

```go
func (a *App) markFriendHelpDailySuccessesForAccount(accountKey string, now time.Time, successCount int) {
	if a.store == nil || successCount <= 0 {
		return
	}
	a.automationSettingsWriteMu.Lock()
	defer a.automationSettingsWriteMu.Unlock()
	settings := automation.SettingsFromState(a.farmAutomationStateForAccount(accountKey))
	settings.Config = automation.MarkFriendHelpDailySuccesses(settings.Config, now, successCount)
	a.saveAutomationDailyStateLocked(accountKey, settings, "auto_farm.friend_help_daily_count.save", "保存好友帮助每日计数失败：")
}
```

Use the captured `accountKey`; do not re-read the active account after the runtime call.

- [ ] **Step 4: Run focused application tests and verify GREEN**

Run the Step 2 command. Expected: PASS.

- [ ] **Step 5: Run full verification**

```powershell
go test ./... -count=1
```

Expected: all Go packages PASS.

Then run:

```powershell
git diff --check
git status --short
```

Expected: no whitespace errors; only intended implementation and test files are modified.

- [ ] **Step 6: Commit**

```powershell
git add app.go app_test.go
git commit -m "fix: persist automatic friend help daily count"
```
