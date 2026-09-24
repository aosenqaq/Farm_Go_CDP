# Reward Specified-Time Scheduler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make all eight reward tasks execute at their configured local daily time and expose that same time through `SchedulerTask.NextRunAt` for the scheduler center.

**Architecture:** Add a small Go scheduling module that owns reward-task config-key lookup and local daily-time calculations. The existing scheduler calls it when settings are configured and when tasks finish, while the existing daily-completion projection preserves the selected hour and minute. React remains a presentation layer and continues formatting the backend timestamp.

**Tech Stack:** Go 1.23, standard-library `time`, React 18, TypeScript, Vitest, Wails state bindings.

---

## File Structure

- Create `internal/farm/automation/reward_schedule.go`: reward task schedule metadata, parsing, and next-occurrence calculations.
- Create `internal/farm/automation/reward_schedule_test.go`: table-driven unit tests for all supported task IDs and time boundaries.
- Modify `internal/farm/automation/scheduler.go`: align runtime state during configuration and after task completion.
- Modify `internal/farm/automation/scheduler_test.go`: scheduler initialization, reconfiguration, mode switching, success, failure, and daily-completion tests.
- Modify `internal/farm/automation/daily_state.go`: preserve configured daily time when a completed task is projected into scheduler state.
- Modify `frontend/src/views/AutomationView.test.tsx`: prove that a specified-time reward timestamp renders in the scheduler-center row.

### Task 1: Reward Schedule Calculation

**Files:**
- Create: `internal/farm/automation/reward_schedule.go`
- Create: `internal/farm/automation/reward_schedule_test.go`

- [ ] **Step 1: Write failing metadata and time-boundary tests**

Create table-driven tests that define the complete supported surface and the agreed no-catch-up rule:

```go
package automation

import (
	"testing"
	"time"
)

func TestRewardScheduleConfigKeysCoversRewardTasks(t *testing.T) {
	want := map[string][2]string{
		"reward_claim":          {"autoRewardClaimScheduleMode", "autoRewardClaimScheduleTime"},
		"svip_daily_gift":       {"autoFarmSvipDailyGiftScheduleMode", "autoFarmSvipDailyGiftScheduleTime"},
		"monthly_card_reward":   {"autoFarmMonthlyCardRewardScheduleMode", "autoFarmMonthlyCardRewardScheduleTime"},
		"mall_daily_fertilizer": {"autoFarmMallDailyFertilizerScheduleMode", "autoFarmMallDailyFertilizerScheduleTime"},
		"share_reward":          {"autoFarmShareRewardScheduleMode", "autoFarmShareRewardScheduleTime"},
		"mail_reward":           {"autoFarmMailRewardScheduleMode", "autoFarmMailRewardScheduleTime"},
		"he_feng_travel_reward": {"autoFarmHeFengTravelRewardScheduleMode", "autoFarmHeFengTravelRewardScheduleTime"},
		"limited_seed_draw":     {"autoFarmLimitedSeedDrawScheduleMode", "autoFarmLimitedSeedDrawScheduleTime"},
	}
	for taskID, keys := range want {
		modeKey, timeKey, ok := rewardScheduleConfigKeys(taskID)
		if !ok || modeKey != keys[0] || timeKey != keys[1] {
			t.Fatalf("rewardScheduleConfigKeys(%q) = %q, %q, %v; want %q, %q, true", taskID, modeKey, timeKey, ok, keys[0], keys[1])
		}
	}
	if _, _, ok := rewardScheduleConfigKeys("own_base"); ok {
		t.Fatal("own_base must not be treated as a reward specified-time task")
	}
}

func TestNextRewardSpecifiedRunUsesNextLocalOccurrence(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	tests := []struct {
		name         string
		now          time.Time
		scheduleTime string
		want         time.Time
	}{
		{"upcoming today", time.Date(2026, 7, 10, 8, 15, 0, 0, location), "09:30", time.Date(2026, 7, 10, 9, 30, 0, 0, location)},
		{"elapsed tomorrow", time.Date(2026, 7, 10, 10, 0, 0, 0, location), "09:30", time.Date(2026, 7, 11, 9, 30, 0, 0, location)},
		{"equal minute tomorrow", time.Date(2026, 7, 10, 9, 30, 0, 0, location), "09:30", time.Date(2026, 7, 11, 9, 30, 0, 0, location)},
		{"invalid falls back", time.Date(2026, 7, 10, 7, 0, 0, 0, location), "invalid", time.Date(2026, 7, 10, 8, 0, 0, 0, location)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := map[string]any{
				"autoFarmMailRewardScheduleMode": "daily_time",
				"autoFarmMailRewardScheduleTime": tt.scheduleTime,
			}
			got, ok := nextRewardSpecifiedRun(config, "mail_reward", tt.now)
			if !ok || !got.Equal(tt.want) || got.Location() != location {
				t.Fatalf("nextRewardSpecifiedRun() = %v, %v; want %v, true", got, ok, tt.want)
			}
		})
	}
}

func TestNextRewardSpecifiedRunRejectsIntervalAndUnsupportedTasks(t *testing.T) {
	now := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	config := map[string]any{
		"autoFarmMailRewardScheduleMode": "interval",
		"autoFarmMailRewardScheduleTime": "09:30",
	}
	if _, ok := nextRewardSpecifiedRun(config, "mail_reward", now); ok {
		t.Fatal("interval mode must use existing scheduler behavior")
	}
	if _, ok := nextRewardSpecifiedRun(config, "own_base", now); ok {
		t.Fatal("unsupported task must use existing scheduler behavior")
	}
}
```

- [ ] **Step 2: Run the new test and verify RED**

Run:

```powershell
go test ./internal/farm/automation -run 'TestRewardScheduleConfigKeys|TestNextRewardSpecifiedRun' -count=1
```

Expected: build failure because `rewardScheduleConfigKeys` and `nextRewardSpecifiedRun` do not exist.

- [ ] **Step 3: Implement the minimal reward scheduling module**

Create `reward_schedule.go` with explicit task metadata and two calculations: the next occurrence relative to now, and the configured time on the next calendar day for daily-completed projection.

```go
package automation

import (
	"fmt"
	"strings"
	"time"
)

type rewardScheduleConfig struct {
	modeKey string
	timeKey string
}

var rewardScheduleConfigs = map[string]rewardScheduleConfig{
	"reward_claim":          {modeKey: "autoRewardClaimScheduleMode", timeKey: "autoRewardClaimScheduleTime"},
	"svip_daily_gift":       {modeKey: "autoFarmSvipDailyGiftScheduleMode", timeKey: "autoFarmSvipDailyGiftScheduleTime"},
	"monthly_card_reward":   {modeKey: "autoFarmMonthlyCardRewardScheduleMode", timeKey: "autoFarmMonthlyCardRewardScheduleTime"},
	"mall_daily_fertilizer": {modeKey: "autoFarmMallDailyFertilizerScheduleMode", timeKey: "autoFarmMallDailyFertilizerScheduleTime"},
	"share_reward":          {modeKey: "autoFarmShareRewardScheduleMode", timeKey: "autoFarmShareRewardScheduleTime"},
	"mail_reward":           {modeKey: "autoFarmMailRewardScheduleMode", timeKey: "autoFarmMailRewardScheduleTime"},
	"he_feng_travel_reward": {modeKey: "autoFarmHeFengTravelRewardScheduleMode", timeKey: "autoFarmHeFengTravelRewardScheduleTime"},
	"limited_seed_draw":     {modeKey: "autoFarmLimitedSeedDrawScheduleMode", timeKey: "autoFarmLimitedSeedDrawScheduleTime"},
}

func rewardScheduleConfigKeys(taskID string) (string, string, bool) {
	schedule, ok := rewardScheduleConfigs[taskID]
	return schedule.modeKey, schedule.timeKey, ok
}

func rewardSpecifiedClock(config map[string]any, taskID string) (int, int, bool) {
	modeKey, timeKey, ok := rewardScheduleConfigKeys(taskID)
	if !ok || strings.TrimSpace(fmt.Sprint(config[modeKey])) != "daily_time" {
		return 0, 0, false
	}
	parsed, err := time.Parse("15:04", strings.TrimSpace(fmt.Sprint(config[timeKey])))
	if err != nil {
		parsed, _ = time.Parse("15:04", "08:00")
	}
	return parsed.Hour(), parsed.Minute(), true
}

func nextRewardSpecifiedRun(config map[string]any, taskID string, now time.Time) (time.Time, bool) {
	hour, minute, ok := rewardSpecifiedClock(config, taskID)
	if !ok {
		return time.Time{}, false
	}
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !target.After(now) {
		target = target.AddDate(0, 0, 1)
	}
	return target, true
}

func nextRewardSpecifiedRunAfterToday(config map[string]any, taskID string, now time.Time) (time.Time, bool) {
	hour, minute, ok := rewardSpecifiedClock(config, taskID)
	if !ok {
		return time.Time{}, false
	}
	nextDay := now.AddDate(0, 0, 1)
	return time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), hour, minute, 0, 0, now.Location()), true
}
```

- [ ] **Step 4: Run focused tests and verify GREEN**

Run:

```powershell
go test ./internal/farm/automation -run 'TestRewardScheduleConfigKeys|TestNextRewardSpecifiedRun' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit the pure scheduling helper**

```powershell
git add internal/farm/automation/reward_schedule.go internal/farm/automation/reward_schedule_test.go
git commit -m "feat: calculate reward specified run times"
```

### Task 2: Scheduler Configuration And Mode Changes

**Files:**
- Modify: `internal/farm/automation/scheduler.go:107-129`
- Modify: `internal/farm/automation/scheduler_test.go:184-227`

- [ ] **Step 1: Write failing scheduler configuration tests**

Add tests that observe public scheduler state after initial configuration, time edits, and switching back to interval mode:

```go
func TestSchedulerConfigureAlignsRewardSpecifiedTime(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 7, 10, 8, 15, 0, 0, location)
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	config := DefaultConfig()
	config["autoFarmMailRewardEnabled"] = true
	config["autoFarmMailRewardScheduleMode"] = "daily_time"
	config["autoFarmMailRewardScheduleTime"] = "09:30"
	s.Configure(Settings{
		SchedulerEnabled: true,
		Tasks: []TaskSettings{{ID: "mail_reward", Enabled: true, Priority: 93, IntervalSec: 43200}},
		Config: config,
	})

	task := findSchedulerTaskForTest(s.State().Scheduler.Tasks, "mail_reward")
	if task == nil || task.NextRunAt != "2026-07-10T09:30:00+08:00" {
		t.Fatalf("mail reward next run = %#v, want today's specified time", task)
	}
	if _, ok := s.nextDueTask(now); ok {
		t.Fatal("specified-time reward must not be due before 09:30")
	}
}

func TestSchedulerReconfigureUpdatesSpecifiedTimeAndClearsItForIntervalMode(t *testing.T) {
	now := time.Date(2026, 7, 10, 8, 15, 0, 0, time.UTC)
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	settings := DefaultSettings()
	settings.Config["autoFarmMailRewardEnabled"] = true
	settings.Config["autoFarmMailRewardScheduleMode"] = "daily_time"
	settings.Config["autoFarmMailRewardScheduleTime"] = "09:30"
	s.Configure(settings)

	settings.Config["autoFarmMailRewardScheduleTime"] = "10:45"
	s.Configure(settings)
	if got := findSchedulerTaskForTest(s.State().Scheduler.Tasks, "mail_reward").NextRunAt; got != "2026-07-10T10:45:00Z" {
		t.Fatalf("updated next run = %q, want 10:45", got)
	}

	settings.Config["autoFarmMailRewardScheduleMode"] = "interval"
	s.Configure(settings)
	if got := findSchedulerTaskForTest(s.State().Scheduler.Tasks, "mail_reward").NextRunAt; got != now.Format(time.RFC3339) {
		t.Fatalf("interval next run = %q, want immediate eligibility %q", got, now.Format(time.RFC3339))
	}
}
```

Use the existing `findSchedulerTaskForTest` helper from `catalog_test.go`, which is in the same Go package.

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```powershell
go test ./internal/farm/automation -run 'TestSchedulerConfigureAlignsRewardSpecifiedTime|TestSchedulerReconfigureUpdatesSpecifiedTime' -count=1
```

Expected: FAIL because `Configure` still sets new tasks to `now` and preserves the stale daily timestamp when mode changes.

- [ ] **Step 3: Align runtime timestamps in `Scheduler.Configure`**

Capture whether each task previously used specified-time mode before replacing `s.settings`, then assign the runtime timestamp using this precedence:

```go
previousSettings := s.settings
s.settings = merged
s.state = StateFromSettings(s.settings)
now := s.now()
for _, task := range s.state.Scheduler.Tasks {
	rt := s.runtime[task.ID]
	if nextRunAt, ok := nextRewardSpecifiedRun(s.settings.Config, task.ID, now); ok {
		rt.NextRunAt = nextRunAt
	} else {
		_, wasSpecified := nextRewardSpecifiedRun(previousSettings.Config, task.ID, now)
		if rt.NextRunAt.IsZero() || wasSpecified {
			rt.NextRunAt = now
		}
	}
	s.runtime[task.ID] = rt
}
```

Do not change the unchanged-settings early return. It prevents harmless refreshes from continually moving a daily timestamp to the following day when the clock reaches the selected minute.

- [ ] **Step 4: Run configuration and existing scheduler tests**

Run:

```powershell
go test ./internal/farm/automation -run 'TestSchedulerConfigure|TestSchedulerReconfigure|TestSchedulerPicksHighestPriorityDueTask|TestSchedulerUsesDetailedIntervalConfigForNextRun' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit scheduler configuration support**

```powershell
git add internal/farm/automation/scheduler.go internal/farm/automation/scheduler_test.go
git commit -m "feat: align reward scheduler configuration"
```

### Task 3: Completion Rescheduling And Daily-Done Projection

**Files:**
- Modify: `internal/farm/automation/scheduler.go:301-354`
- Modify: `internal/farm/automation/daily_state.go:90-96`
- Modify: `internal/farm/automation/scheduler_test.go:51-77,123-182`

- [ ] **Step 1: Write failing success and failure rescheduling tests**

Add a table-driven test. Set the runtime timestamp to `now` under the lock so `RunDue` executes the daily task without weakening the initial no-catch-up rule:

```go
func TestSchedulerSpecifiedRewardUsesNextDailyTimeAfterResult(t *testing.T) {
	for _, resultOK := range []bool{true, false} {
		t.Run(map[bool]string{true: "success", false: "failure"}[resultOK], func(t *testing.T) {
			now := time.Date(2026, 7, 10, 9, 30, 0, 0, time.UTC)
			s := NewScheduler(SchedulerOptions{
				Now: func() time.Time { return now },
				Runner: func(ctx context.Context, taskID string) ActionResult {
					return ActionResult{OK: resultOK, Status: map[bool]ActionStatus{true: StatusOK, false: StatusFailed}[resultOK], TaskID: taskID, Message: "result"}
				},
			})
			config := DefaultConfig()
			config["autoFarmMailRewardEnabled"] = true
			config["autoFarmMailRewardScheduleMode"] = "daily_time"
			config["autoFarmMailRewardScheduleTime"] = "09:30"
			s.Configure(Settings{
				SchedulerEnabled: true,
				Tasks: []TaskSettings{{ID: "mail_reward", Enabled: true, Priority: 93, IntervalSec: 60}},
				Config: config,
			})
			s.mu.Lock()
			rt := s.runtime["mail_reward"]
			rt.NextRunAt = now
			s.runtime["mail_reward"] = rt
			s.mu.Unlock()

			s.RunDue(context.Background())

			task := findSchedulerTaskForTest(s.State().Scheduler.Tasks, "mail_reward")
			if task == nil || task.NextRunAt != "2026-07-11T09:30:00Z" {
				t.Fatalf("mail reward next run = %#v, want next specified occurrence after result", task)
			}
		})
	}
}
```

- [ ] **Step 2: Update the existing daily-completion test expectation and verify RED**

Configure the existing `TestSchedulerStateMarksDailyOnceTaskDoneTodayAndNextRun` with:

```go
config["autoFarmSvipDailyGiftScheduleMode"] = "daily_time"
config["autoFarmSvipDailyGiftScheduleTime"] = "09:30"
```

Change its expected `NextRunAt` from midnight to:

```go
if task.NextRunAt != "2026-07-09T09:30:00Z" {
	t.Fatalf("NextRunAt = %q, want next day at configured time", task.NextRunAt)
}
```

Run:

```powershell
go test ./internal/farm/automation -run 'TestSchedulerSpecifiedRewardUsesNextDailyTimeAfterResult|TestSchedulerStateMarksDailyOnceTaskDoneTodayAndNextRun' -count=1
```

Expected: FAIL because success still uses `IntervalSec`, failure still uses backoff, and daily completion still forces midnight.

- [ ] **Step 3: Override result scheduling for specified-time rewards**

Keep all existing success/failure bookkeeping, then replace its timestamp when the task has a specified schedule:

```go
if nextRunAt, ok := nextRewardSpecifiedRun(s.settings.Config, task.ID, finished); ok {
	rt.NextRunAt = nextRunAt
}
```

Place this after the existing `if result.OK { ... } else { ... }` block so both outcomes use the daily time without changing their error fields or failure counts.

- [ ] **Step 4: Preserve the configured clock for completed daily tasks**

Change `applyDailyOnceTaskState` to prefer the explicit next-day daily timestamp:

```go
task.DailyDoneToday = true
if nextRunAt, ok := nextRewardSpecifiedRunAfterToday(config, task.ID, now); ok {
	task.NextRunAt = nextRunAt.Format(time.RFC3339)
	return
}
task.NextRunAt = nextAutomationDayStart(now).Format(time.RFC3339)
```

This keeps midnight behavior for friend mischief and interval-mode daily-once tasks.

- [ ] **Step 5: Run all automation package tests**

Run:

```powershell
go test ./internal/farm/automation -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit completion and projection behavior**

```powershell
git add internal/farm/automation/scheduler.go internal/farm/automation/daily_state.go internal/farm/automation/scheduler_test.go
git commit -m "feat: reschedule daily reward task results"
```

### Task 4: Scheduler Center Compatibility Test

**Files:**
- Modify: `frontend/src/views/AutomationView.test.tsx:128-161`

- [ ] **Step 1: Add a specified-time reward rendering test**

Add a distinct test using the backend contract rather than recomputing from config:

```tsx
it('renders a reward specified-time next run from scheduler state', () => {
  const html = renderToStaticMarkup(
    <AutomationView
      state={
        {
          ...state,
          scheduler: {
            ...state.scheduler,
            tasks: [
              {
                id: 'mail_reward',
                label: '自动领取邮件奖励',
                priority: 93,
                intervalSec: 43200,
                enabled: true,
                nextRunAt: '2026-07-11T00:04:00',
              },
            ],
          },
        } as FarmAutomationState
      }
      onRunTask={() => undefined}
      initialSchedulerOpen
    />,
  );

  expect(html).toContain('自动领取邮件奖励');
  expect(html).toContain('预计下次');
  expect(html).toContain('07-11 00:04');
});
```

The offset-free timestamp is intentional in this rendering-only test so the assertion is independent of the machine timezone. Go tests verify that production values are RFC3339 timestamps with the scheduler clock's location.

- [ ] **Step 2: Run the focused frontend test**

Run:

```powershell
Set-Location frontend
npm test -- --run src/views/AutomationView.test.tsx
```

Expected: PASS with the existing `formatSchedulerTime` implementation. No production frontend edit is necessary when this passes.

- [ ] **Step 3: Commit the compatibility regression test**

```powershell
git add frontend/src/views/AutomationView.test.tsx
git commit -m "test: cover reward time in scheduler center"
```

### Task 5: Full Verification

**Files:**
- Verify only; no planned production edits.

- [ ] **Step 1: Format changed Go files**

Run:

```powershell
gofmt -w internal/farm/automation/reward_schedule.go internal/farm/automation/reward_schedule_test.go internal/farm/automation/scheduler.go internal/farm/automation/scheduler_test.go internal/farm/automation/daily_state.go
```

Expected: command exits successfully.

- [ ] **Step 2: Run all Go tests**

Run:

```powershell
go test ./...
```

Expected: PASS for every package.

- [ ] **Step 3: Run all frontend tests**

Run:

```powershell
Set-Location frontend
npm test
```

Expected: PASS for every Vitest suite.

- [ ] **Step 4: Build the frontend**

Run:

```powershell
Set-Location frontend
npm run build
```

Expected: TypeScript and Vite build successfully without errors.

- [ ] **Step 5: Inspect the final diff boundary**

Run:

```powershell
Set-Location ..
git diff --check
git status --short
```

Expected: no whitespace errors; only the planned scheduling files and any pre-existing untracked `graphify-out` analysis directories are present.

- [ ] **Step 6: Commit any formatting-only changes if needed**

If `gofmt` changed files after their task commits:

```powershell
git add internal/farm/automation
git commit -m "style: format reward scheduler changes"
```

Expected: either a formatting commit is created or Git reports there is nothing to commit.
