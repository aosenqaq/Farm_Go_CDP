# Auto Farm Runtime Scheduler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a real Go scheduler for Farm_Go automation tasks that uses task priority, allows pure protocol reward tasks to run beside mainline farm tasks, and records task logs through the existing event system.

**Architecture:** Keep task execution behind the existing `RuntimeFacade`, and add a focused scheduler package surface inside `internal/farm/automation`. Tasks are selected by due time plus priority aging, dispatched into resource-aware lanes, and reported through an injected event callback so `App` can write the same runtime event log used by manual actions.

**Tech Stack:** Go 1.25, standard-library goroutines/channels/context/sync/heap, existing Wails `App`, existing `eventbus.Event`, existing storage-backed automation settings.

---

## File Structure

- Create `internal/farm/automation/scheduler_test.go`: TDD coverage for priority ordering, overdue aging, lane/resource concurrency, runtime state updates, and log callback emission.
- Create `internal/farm/automation/scheduler.go`: scheduler types, task specs, scoring, resource gate, dispatch loop, state snapshot, and lifecycle methods.
- Modify `internal/farm/automation/catalog.go`: expose task definitions as scheduler specs and enrich `SchedulerState` with runtime fields already present in `SchedulerTask`.
- Modify `app.go`: hold a scheduler instance, start/stop it with app lifecycle, expose start/stop/state methods, and route scheduler logs into `recordEvent`.
- Modify `app_test.go`: verify App-level scheduler methods return state and scheduler-triggered logs are event-shaped.

## Scope Check

This plan builds a backend scheduler only. It does not redesign the React automation page. Existing manual task buttons keep using `RunFarmAutomationTask`, and the scheduler uses the same `RuntimeFacade` execution path with trigger `"auto"`.

## Task 1: Scheduler Task Selection

**Files:**
- Create: `internal/farm/automation/scheduler_test.go`
- Create: `internal/farm/automation/scheduler.go`

- [ ] **Step 1: Write the failing test**

Add tests that describe the desired scheduler API before implementation:

```go
func TestSchedulerPicksHighestPriorityDueTask(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(Settings{
		SchedulerEnabled: true,
		SchedulerMinGapMs: 10,
		Tasks: []TaskSettings{
			{ID: "friend_steal", Enabled: true, Priority: 70, IntervalSec: 90},
			{ID: "own_base", Enabled: true, Priority: 100, IntervalSec: 60},
		},
		Config: DefaultConfig(),
	})

	task, ok := s.nextDueTask(now)

	if !ok || task.ID != "own_base" {
		t.Fatalf("next task = %#v, %v; want own_base", task, ok)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/farm/automation -run TestSchedulerPicksHighestPriorityDueTask`

Expected: fail because `NewScheduler`, `SchedulerOptions`, and `nextDueTask` do not exist.

- [ ] **Step 3: Write minimal implementation**

Create `scheduler.go` with `Scheduler`, `SchedulerOptions`, `TaskSpec`, `Lane`, `Resource`, `Configure`, and `nextDueTask`. Use existing settings and task definitions, sort due candidates by score.

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/farm/automation -run TestSchedulerPicksHighestPriorityDueTask`

Expected: pass.

## Task 2: Resource-Aware Lanes

**Files:**
- Modify: `internal/farm/automation/scheduler_test.go`
- Modify: `internal/farm/automation/scheduler.go`

- [ ] **Step 1: Write the failing tests**

Add tests proving protocol reward tasks can run while mainline tasks hold the scene resource, but two reward tasks sharing the reward resource cannot overlap:

```go
func TestSchedulerAllowsProtocolTaskBesideMainlineTask(t *testing.T) {
	s := NewScheduler(SchedulerOptions{})
	if !s.tryAcquire(taskSpecByID("own_base")) {
		t.Fatal("own_base should acquire resources")
	}
	if !s.tryAcquire(taskSpecByID("reward_claim")) {
		t.Fatal("reward_claim should run beside own_base")
	}
}

func TestSchedulerBlocksTasksSharingRewardResource(t *testing.T) {
	s := NewScheduler(SchedulerOptions{})
	if !s.tryAcquire(taskSpecByID("reward_claim")) {
		t.Fatal("reward_claim should acquire resources")
	}
	if s.tryAcquire(taskSpecByID("mail_reward")) {
		t.Fatal("mail_reward should wait for reward resource")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/farm/automation -run "TestSchedulerAllowsProtocolTaskBesideMainlineTask|TestSchedulerBlocksTasksSharingRewardResource"`

Expected: fail because resource acquisition is not implemented.

- [ ] **Step 3: Write minimal implementation**

Add task spec metadata:

```go
const (
	LaneMainline Lane = "mainline"
	LaneProtocol Lane = "protocol"
)

const (
	ResourceScene    Resource = "scene"
	ResourceProtocol Resource = "protocol"
	ResourceReward   Resource = "reward"
	ResourceFriend   Resource = "friend"
	ResourceShop     Resource = "shop"
)
```

Map own/friend/plant/fertilizer tasks to `LaneMainline + ResourceScene`; map reward tasks to `LaneProtocol + ResourceProtocol + ResourceReward`; map mystery shop auto-buy to `LaneProtocol + ResourceProtocol + ResourceShop`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/farm/automation -run "TestSchedulerAllowsProtocolTaskBesideMainlineTask|TestSchedulerBlocksTasksSharingRewardResource"`

Expected: pass.

## Task 3: Dispatch, State, and Logs

**Files:**
- Modify: `internal/farm/automation/scheduler_test.go`
- Modify: `internal/farm/automation/scheduler.go`

- [ ] **Step 1: Write the failing tests**

Add tests for dispatching a due task, updating last run fields, and emitting task lifecycle logs:

```go
func TestSchedulerDispatchesDueTaskAndRecordsLog(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	var logs []SchedulerLog
	runner := func(ctx context.Context, taskID string) ActionResult {
		return ActionResult{OK: true, Status: StatusOK, TaskID: taskID, Message: "done"}
	}
	s := NewScheduler(SchedulerOptions{
		Now:    func() time.Time { return now },
		Runner: runner,
		OnLog:  func(log SchedulerLog) { logs = append(logs, log) },
	})
	s.Configure(Settings{
		SchedulerEnabled: true,
		SchedulerMinGapMs: 1,
		Tasks: []TaskSettings{{ID: "own_base", Enabled: true, Priority: 100, IntervalSec: 60}},
		Config: DefaultConfig(),
	})

	s.RunDue(context.Background())

	state := s.State()
	if state.Scheduler.Tasks[0].LastSuccessAt == "" {
		t.Fatalf("LastSuccessAt not recorded: %#v", state.Scheduler.Tasks[0])
	}
	if len(logs) < 2 || logs[0].Type != "task.start" || logs[len(logs)-1].Type != "task.done" {
		t.Fatalf("logs = %#v, want start and done", logs)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/farm/automation -run TestSchedulerDispatchesDueTaskAndRecordsLog`

Expected: fail because dispatch/state/log support is missing.

- [ ] **Step 3: Write minimal implementation**

Add `SchedulerLog`, `RunDue`, `State`, task runtime fields, resource release on completion, and next-run calculation. Keep `RunDue` synchronous for deterministic tests; the background loop can call it repeatedly.

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/farm/automation -run TestSchedulerDispatchesDueTaskAndRecordsLog`

Expected: pass.

## Task 4: App Integration and Task Logs

**Files:**
- Modify: `app.go`
- Modify: `app_test.go`

- [ ] **Step 1: Write failing App tests**

Add tests that call the new App methods and check scheduler state shape:

```go
func TestFarmAutomationSchedulerStateIsExposed(t *testing.T) {
	app := NewApp()
	state := app.FarmAutomationSchedulerState()
	if len(state.Tasks) == 0 {
		t.Fatalf("scheduler state has no tasks: %#v", state)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./... -run TestFarmAutomationSchedulerStateIsExposed`

Expected: fail because App methods are not implemented.

- [ ] **Step 3: Write minimal implementation**

Add an `automationScheduler *automation.Scheduler` field to `App`, lazily initialize it with:

```go
Runner: func(ctx context.Context, taskID string) automation.ActionResult {
	return a.runFarmAutomationTask(taskID, "auto")
},
OnLog: func(log automation.SchedulerLog) {
	a.recordEvent(eventbus.Event{...})
},
```

Expose `FarmAutomationSchedulerState`, `StartFarmAutomationScheduler`, and `StopFarmAutomationScheduler`.

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./... -run TestFarmAutomationSchedulerStateIsExposed`

Expected: pass.

## Task 5: Full Verification

**Files:**
- No source edits expected unless tests reveal issues.

- [ ] **Step 1: Run automation package tests**

Run: `go test ./internal/farm/automation`

Expected: pass.

- [ ] **Step 2: Run all Go tests**

Run: `go test ./...`

Expected: pass.

- [ ] **Step 3: Check git diff**

Run: `git diff --stat`

Expected: only plan, scheduler, app integration, and tests changed; existing runtime log files may remain dirty but unrelated.

## Self-Review

- Spec coverage: Tasks cover priority scheduling, protocol sidecar concurrency, resource exclusion, scheduler state, App entry points, and task logging.
- Placeholder scan: No TBD/TODO placeholders are used as implementation instructions.
- Type consistency: Plan uses `Settings`, `TaskSettings`, `State`, `SchedulerState`, and `ActionResult` already present in `internal/farm/automation`, plus new `Scheduler`, `SchedulerOptions`, `SchedulerLog`, `Lane`, and `Resource` in `scheduler.go`.
