# Automation Statistics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Count only confirmed work in session and history action statistics, using one count per successful own-farm round and one per successfully operated friend.

**Architecture:** Add an explicit `ActionCount` to each runtime action result, then preserve it on both manual and scheduler `task.done` events. The session and historical aggregators consume that field for action metrics while retaining their existing successful-task-round definition of `runs`.

**Tech Stack:** Go, Wails event payloads, Go standard-library tests.

---

## File Structure

- `internal/farm/automation/catalog.go`: public `ActionResult` event contract.
- `internal/farm/automation/runtime_own.go`: own-harvest and own-farm counts.
- `internal/farm/automation/runtime_friend.go`: per-successful-friend counts.
- `internal/farm/automation/scheduler.go`: scheduler-log propagation.
- `app.go`: manual and scheduler event payloads plus daily history aggregation.
- `internal/farm/run_statistics.go`: current-session aggregation.
- `internal/farm/automation/*_test.go`, `internal/farm/run_statistics_test.go`, `app_test.go`: behavior coverage.

### Task 1: Produce Explicit Runtime Action Counts

**Files:**
- Modify: `internal/farm/automation/catalog.go:18-25`
- Modify: `internal/farm/automation/runtime_own.go:18-126,130-228`
- Modify: `internal/farm/automation/runtime_friend.go:14-143,458-590,752-878`
- Test: `internal/farm/automation/runtime_own_test.go`
- Test: `internal/farm/automation/runtime_friend_test.go`

- [x] **Step 1: Write failing runtime assertions**

```go
if result.ActionCount != 1 {
    t.Fatalf("action count = %d, want 1", result.ActionCount)
}

// A harvest request with no successful result and a dead-land-only cleanup
// must retain ActionCount == 0.
```

Add friend-task cases asserting steal and help report the number of successful candidates, and mischief reports `1` after its successful friend's request.

- [x] **Step 2: Run the focused tests and verify failure**

Run: `go test ./internal/farm/automation -run 'TestRuntimeFacade(RunsOwnBaseThroughRuntimeActions|OwnCollectHarvestsCollectableOwnFarmLands|OwnCollectCleansDeadLandsWithoutHarvest|FriendSteal|FriendHelp|FriendMischief)' -count=1`

Expected: FAIL because `ActionResult` has no `ActionCount` field.

- [x] **Step 3: Add the result contract and set it only after confirmed work**

```go
type ActionResult struct {
    OK          bool         `json:"ok"`
    Status      ActionStatus `json:"status"`
    TaskID      string       `json:"taskId,omitempty"`
    Message     string       `json:"message"`
    ActionCount int          `json:"actionCount,omitempty"`
    SuccessfulFriends int    `json:"successfulFriends,omitempty"`
}
```

Set `ActionCount: 1` for successful own-base work and successful own-collect harvests (`harvestCount > 0` only). Set it to `successCount` in successful steal/help result branches and to `1` when the mischief request succeeds. Leave all skip, failed, and dead-land-cleanup-only results at zero. Preserve `SuccessfulFriends` for friend-help daily-limit persistence.

- [x] **Step 4: Run the focused tests and verify success**

Run: `go test ./internal/farm/automation -run 'TestRuntimeFacade(RunsOwnBaseThroughRuntimeActions|OwnCollectHarvestsCollectableOwnFarmLands|OwnCollectCleansDeadLandsWithoutHarvest|FriendSteal|FriendHelp|FriendMischief)' -count=1`

Expected: PASS.

### Task 2: Carry Action Counts Into Every Task-Done Event

**Files:**
- Modify: `internal/farm/automation/scheduler.go:35-44,301-350`
- Modify: `app.go:1380-1455`
- Test: `internal/farm/automation/scheduler_test.go`
- Test: `app_test.go`

- [x] **Step 1: Write failing event-propagation tests**

```go
if got := logs[len(logs)-1].ActionCount; got != 2 {
    t.Fatalf("scheduler action count = %d, want 2", got)
}

if got := event.Data["actionCount"]; got != 2 {
    t.Fatalf("event action count = %#v, want 2", got)
}
```

Use a scheduler runner returning `ActionResult{OK: true, Status: StatusOK, ActionCount: 2}` and validate both automatic and manual `task.done` event maps.

- [x] **Step 2: Run the focused tests and verify failure**

Run: `go test . ./internal/farm/automation -run 'Test(SchedulerDispatchesDueTaskAndRecordsLog|FarmWorkspaceRunStatisticsUsesCurrentSessionEventsAndTodaySellRecords)' -count=1`

Expected: FAIL because `SchedulerLog` and event data omit `actionCount`.

- [x] **Step 3: Propagate the field without changing status semantics**

```go
type SchedulerLog struct {
    // Existing fields...
    ActionCount int
}

log.ActionCount = result.ActionCount
```

Add `"actionCount": log.ActionCount` to scheduler-recorded events and `"actionCount": result.ActionCount` to manually-recorded completion events. Keep `task.done`, `ok`, and `status` behavior unchanged.

- [x] **Step 4: Run the focused tests and verify success**

Run: `go test . ./internal/farm/automation -run 'Test(SchedulerDispatchesDueTaskAndRecordsLog|FarmWorkspaceRunStatisticsUsesCurrentSessionEventsAndTodaySellRecords)' -count=1`

Expected: PASS.

### Task 3: Aggregate Counts in Session and History Statistics

**Files:**
- Modify: `internal/farm/run_statistics.go:27-57`
- Modify: `internal/farm/run_statistics_test.go`
- Modify: `app.go:1743-1818`
- Test: `app_test.go`

- [x] **Step 1: Write failing aggregation tests**

```go
events := []eventbus.Event{
    runStatisticsTaskDone(startedAt, "own_collect", 1),
    runStatisticsTaskDone(startedAt, "friend_steal", 2),
    runStatisticsTaskDone(startedAt, "friend_help", 3),
    runStatisticsTaskDone(startedAt, "own_base", 0),
    runStatisticsTaskDone(startedAt, "friend_mischief", 1),
}
// Expect Collect=1, Farm=0, Steal=2, Help=3, Mischief=1.
```

Add an event without `actionCount` and assert it contributes zero to action metrics. Add the equivalent persisted-event scenario through `FarmAccountStatus`, while asserting `Runs` still includes every successful `task.done` round.

- [x] **Step 2: Run the focused tests and verify failure**

Run: `go test ./internal/farm . -run 'Test(BuildRunStatistics|FarmWorkspaceRunStatisticsUsesCurrentSessionEventsAndTodaySellRecords|FarmAccountStatus)' -count=1`

Expected: FAIL because aggregators currently increment each matching successful event by one.

- [x] **Step 3: Use positive event action counts for each action metric**

```go
count := actionCountFromEvent(event)
if count <= 0 {
    continue
}
switch runStatisticsTaskID(event) {
case "friend_help":
    stats.Help += count
}
```

Keep successful-event filtering for daily `Runs`; apply the positive-count condition only inside the action-metric switch. Use the same helper rule in both `BuildRunStatistics` and `automationStatsHistoryForAccount` so legacy events without the field contribute zero.

- [x] **Step 4: Run the focused tests and verify success**

Run: `go test ./internal/farm . -run 'Test(BuildRunStatistics|FarmWorkspaceRunStatisticsUsesCurrentSessionEventsAndTodaySellRecords|FarmAccountStatus)' -count=1`

Expected: PASS.

### Task 4: Run Regression Checks

**Files:**
- Modify: none

- [x] **Step 1: Format changed Go files**

Run: `gofmt -w app.go internal/farm/run_statistics.go internal/farm/run_statistics_test.go internal/farm/automation/catalog.go internal/farm/automation/runtime_own.go internal/farm/automation/runtime_friend.go internal/farm/automation/scheduler.go internal/farm/automation/scheduler_test.go app_test.go`

Expected: command exits with code 0.

- [x] **Step 2: Run package and frontend regression tests**

Run: `go test ./... -count=1`

Expected: PASS.

Run: `npm test -- --run src/views/FarmWorkspaceView.test.tsx`

Working directory: `frontend`

Expected: PASS; the frontend still renders the unchanged response fields.

- [x] **Step 3: Inspect the final diff**

Run: `git diff --check && git diff -- app.go internal/farm/run_statistics.go internal/farm/automation frontend/src`

Expected: no whitespace errors; only action-count contract, event propagation, aggregation, and focused tests change.

## Self-Review

- Spec coverage: Task 1 creates per-task action counts; Task 2 preserves the counts for automatic and manual events; Task 3 updates both statistics views and handles legacy events; Task 4 verifies regressions.
- Placeholder scan: no implementation placeholders remain.
- Type consistency: `ActionCount` is defined on `ActionResult`, copied to `SchedulerLog`, serialized as `actionCount`, and read by both aggregators.
