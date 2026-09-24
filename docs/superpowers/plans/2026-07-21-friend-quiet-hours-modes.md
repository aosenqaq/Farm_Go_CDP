# Friend Quiet Hours Modes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Make friend quiet hours enforce automatic scheduler eligibility, add sleep/work modes and selectable steal/help/mischief scopes, and replace the current explanatory copy with the approved two-row controls.

**Architecture:** A pure Go quiet-hours decision helper owns config normalization, local-time window evaluation, task-to-scope mapping, and the next allowed boundary. The scheduler consumes that helper when selecting due tasks, calculating wake delay, and exposing effective NextRunAt; manual execution remains outside the scheduler path. React keeps the generic config payload and renders the approved compact controls without adding a new API or storage schema.

**Tech Stack:** Go, Go time package, Wails v2, React 18, TypeScript, Vitest, React Test Renderer, CSS, Vite

---

## File Map

- Create internal/farm/automation/quiet_hours.go for normalized time decisions.
- Create internal/farm/automation/quiet_hours_test.go for mode, window, scope, fallback, and boundary tests.
- Modify internal/farm/automation/config.go for mode and scope defaults.
- Modify internal/farm/automation/catalog_test.go to lock the default config contract.
- Modify internal/farm/automation/scheduler.go to filter due tasks and expose effective wake/state times.
- Modify internal/farm/automation/scheduler_test.go for scheduler integration.
- Modify frontend/src/views/AutomationView.tsx for the approved controls.
- Modify frontend/src/views/AutomationView.test.tsx for markup and save interactions.
- Modify frontend/src/style.css for desktop and mobile layout.

### Task 1: Add Normalized Quiet-Hours Decisions

**Files:**
- Create: internal/farm/automation/quiet_hours.go
- Create: internal/farm/automation/quiet_hours_test.go
- Modify: internal/farm/automation/config.go:115
- Modify: internal/farm/automation/catalog_test.go

- [ ] **Step 1: Write the failing default contract test**

Add reflect to catalog_test.go imports and add:

~~~go
func TestDefaultConfigIncludesFriendQuietHoursModeAndScopes(t *testing.T) {
	config := DefaultConfig()
	if config[friendQuietHoursModeConfigKey] != friendQuietHoursModeSleep {
		t.Fatalf("quiet-hours mode = %#v, want sleep", config[friendQuietHoursModeConfigKey])
	}
	if !reflect.DeepEqual(config[friendQuietHoursScopesConfigKey], []string{"steal", "help"}) {
		t.Fatalf("quiet-hours scopes = %#v, want steal/help", config[friendQuietHoursScopesConfigKey])
	}
	merged := MergeConfigWithDefaults(map[string]any{friendQuietHoursScopesConfigKey: []string{}})
	if scopes, ok := merged[friendQuietHoursScopesConfigKey].([]string); !ok || len(scopes) != 0 {
		t.Fatalf("explicit empty scopes were replaced: %#v", merged[friendQuietHoursScopesConfigKey])
	}
}
~~~

- [ ] **Step 2: Write failing pure-decision tests**

Create quiet_hours_test.go:

~~~go
package automation

import (
	"testing"
	"time"
)

func TestFriendQuietHoursDecisionModesWindowsAndScopes(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	tests := []struct {
		name    string
		config  map[string]any
		taskID  string
		now     time.Time
		allowed bool
		next    string
	}{
		{"sleep cross-midnight", quietHoursConfigForTest("sleep", "23:00", "07:00", []any{"steal", "help"}), "friend_steal", time.Date(2026, 7, 21, 23, 30, 0, 0, location), false, "2026-07-22T07:00:00+08:00"},
		{"sleep end boundary", quietHoursConfigForTest("sleep", "23:00", "07:00", []string{"steal"}), "friend_steal", time.Date(2026, 7, 22, 7, 0, 0, 0, location), true, ""},
		{"work before same-day window", quietHoursConfigForTest("work", "08:00", "18:00", []string{"help"}), "friend_help", time.Date(2026, 7, 21, 7, 30, 0, 0, location), false, "2026-07-21T08:00:00+08:00"},
		{"work start boundary", quietHoursConfigForTest("work", "08:00", "18:00", []string{"help"}), "friend_help", time.Date(2026, 7, 21, 8, 0, 0, 0, location), true, ""},
		{"unselected scope", quietHoursConfigForTest("sleep", "08:00", "18:00", []string{"steal"}), "friend_mischief", time.Date(2026, 7, 21, 12, 0, 0, 0, location), true, ""},
		{"non-friend task", quietHoursConfigForTest("work", "08:00", "18:00", []string{"steal", "help", "mischief"}), "own_base", time.Date(2026, 7, 21, 2, 0, 0, 0, location), true, ""},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			decision := friendQuietHoursDecision(testCase.config, testCase.taskID, testCase.now)
			if decision.Allowed != testCase.allowed {
				t.Fatalf("Allowed = %v, want %v", decision.Allowed, testCase.allowed)
			}
			gotNext := ""
			if !decision.NextAllowedAt.IsZero() {
				gotNext = decision.NextAllowedAt.Format(time.RFC3339)
			}
			if gotNext != testCase.next {
				t.Fatalf("NextAllowedAt = %q, want %q", gotNext, testCase.next)
			}
		})
	}
}

func TestFriendQuietHoursDecisionNormalizesFallbacksAndAllDay(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, location)

	invalid := quietHoursConfigForTest("unknown", "bad", "also-bad", []any{"STEAL", "steal", "unknown"})
	decision := friendQuietHoursDecision(invalid, "friend_steal", time.Date(2026, 7, 21, 23, 30, 0, 0, location))
	if decision.Allowed || decision.NextAllowedAt.Format("15:04") != "07:00" {
		t.Fatalf("invalid config fallback = %#v", decision)
	}

	allDaySleep := quietHoursConfigForTest("sleep", "08:00", "08:00", []string{"steal"})
	if decision = friendQuietHoursDecision(allDaySleep, "friend_steal", now); decision.Allowed || !decision.NextAllowedAt.IsZero() {
		t.Fatalf("all-day sleep = %#v", decision)
	}
	allDayWork := quietHoursConfigForTest("work", "08:00", "08:00", []string{"steal"})
	if decision = friendQuietHoursDecision(allDayWork, "friend_steal", now); !decision.Allowed {
		t.Fatalf("all-day work = %#v", decision)
	}
	empty := quietHoursConfigForTest("sleep", "08:00", "18:00", []string{})
	if decision = friendQuietHoursDecision(empty, "friend_steal", now); !decision.Allowed {
		t.Fatalf("empty scopes = %#v", decision)
	}
	disabled := quietHoursConfigForTest("sleep", "08:00", "18:00", []string{"steal"})
	disabled[friendQuietHoursEnabledConfigKey] = false
	if decision = friendQuietHoursDecision(disabled, "friend_steal", now); !decision.Allowed {
		t.Fatalf("disabled rule = %#v", decision)
	}
}

func quietHoursConfigForTest(mode, start, end string, scopes any) map[string]any {
	return map[string]any{
		friendQuietHoursEnabledConfigKey: true,
		friendQuietHoursModeConfigKey: mode,
		friendQuietHoursStartConfigKey: start,
		friendQuietHoursEndConfigKey: end,
		friendQuietHoursScopesConfigKey: scopes,
	}
}
~~~

- [ ] **Step 3: Run tests and verify RED**

~~~powershell
go test ./internal/farm/automation -run 'TestDefaultConfigIncludesFriendQuietHours|TestFriendQuietHoursDecision' -count=1
~~~

Expected: FAIL to compile because the new constants and decision helper do not exist.

- [ ] **Step 4: Add defaults and the pure helper**

In config.go, replace the three literal quiet-hours keys with constants and add:

~~~go
friendQuietHoursEnabledConfigKey: false,
friendQuietHoursStartConfigKey:   "23:00",
friendQuietHoursEndConfigKey:     "07:00",
friendQuietHoursModeConfigKey:    friendQuietHoursModeSleep,
friendQuietHoursScopesConfigKey:  []string{"steal", "help"},
~~~

Create quiet_hours.go:

~~~go
package automation

import (
	"fmt"
	"strings"
	"time"
)

const (
	friendQuietHoursEnabledConfigKey = "autoFarmFriendQuietHoursEnabled"
	friendQuietHoursStartConfigKey   = "autoFarmFriendQuietHoursStart"
	friendQuietHoursEndConfigKey     = "autoFarmFriendQuietHoursEnd"
	friendQuietHoursModeConfigKey    = "autoFarmFriendQuietHoursMode"
	friendQuietHoursScopesConfigKey  = "autoFarmFriendQuietHoursScopes"
	friendQuietHoursModeSleep        = "sleep"
	friendQuietHoursModeWork         = "work"
)

var friendQuietHoursScopeByTaskID = map[string]string{
	"friend_steal": "steal",
	"friend_help": "help",
	"friend_mischief": "mischief",
}

type quietHoursTaskDecision struct {
	Allowed       bool
	NextAllowedAt time.Time
}

func friendQuietHoursDecision(config map[string]any, taskID string, now time.Time) quietHoursTaskDecision {
	scope := friendQuietHoursScopeByTaskID[taskID]
	if scope == "" || !boolConfig(config[friendQuietHoursEnabledConfigKey], false) {
		return quietHoursTaskDecision{Allowed: true}
	}
	if !normalizedQuietHoursScopes(config)[scope] {
		return quietHoursTaskDecision{Allowed: true}
	}
	mode := strings.ToLower(strings.TrimSpace(fmt.Sprint(config[friendQuietHoursModeConfigKey])))
	if mode != friendQuietHoursModeWork {
		mode = friendQuietHoursModeSleep
	}
	startMinute := quietHoursMinute(config[friendQuietHoursStartConfigKey], 23*60)
	endMinute := quietHoursMinute(config[friendQuietHoursEndConfigKey], 7*60)
	if startMinute == endMinute {
		return quietHoursTaskDecision{Allowed: mode == friendQuietHoursModeWork}
	}
	active, nextStart, nextEnd := quietHoursWindow(now, startMinute, endMinute)
	if mode == friendQuietHoursModeSleep {
		if active {
			return quietHoursTaskDecision{Allowed: false, NextAllowedAt: nextEnd}
		}
		return quietHoursTaskDecision{Allowed: true}
	}
	if active {
		return quietHoursTaskDecision{Allowed: true}
	}
	return quietHoursTaskDecision{Allowed: false, NextAllowedAt: nextStart}
}

func normalizedQuietHoursScopes(config map[string]any) map[string]bool {
	value, exists := config[friendQuietHoursScopesConfigKey]
	if !exists {
		value = []string{"steal", "help"}
	}
	result := map[string]bool{}
	appendScope := func(item any) {
		scope := strings.ToLower(strings.TrimSpace(fmt.Sprint(item)))
		if scope == "steal" || scope == "help" || scope == "mischief" {
			result[scope] = true
		}
	}
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			appendScope(item)
		}
	case []any:
		for _, item := range typed {
			appendScope(item)
		}
	}
	return result
}

func quietHoursMinute(value any, fallback int) int {
	parsed, err := time.Parse("15:04", strings.TrimSpace(fmt.Sprint(value)))
	if err != nil {
		return fallback
	}
	return parsed.Hour()*60 + parsed.Minute()
}

func quietHoursWindow(now time.Time, startMinute, endMinute int) (bool, time.Time, time.Time) {
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startToday := dayStart.Add(time.Duration(startMinute) * time.Minute)
	endToday := dayStart.Add(time.Duration(endMinute) * time.Minute)
	if startMinute < endMinute {
		if now.Before(startToday) {
			return false, startToday, endToday
		}
		if now.Before(endToday) {
			return true, startToday, endToday
		}
		return false, startToday.Add(24 * time.Hour), endToday.Add(24 * time.Hour)
	}
	if !now.Before(startToday) {
		return true, startToday, endToday.Add(24 * time.Hour)
	}
	if now.Before(endToday) {
		return true, startToday.Add(-24 * time.Hour), endToday
	}
	return false, startToday, endToday.Add(24 * time.Hour)
}
~~~

- [ ] **Step 5: Format and verify GREEN**

~~~powershell
gofmt -w internal/farm/automation/config.go internal/farm/automation/quiet_hours.go internal/farm/automation/quiet_hours_test.go internal/farm/automation/catalog_test.go
go test ./internal/farm/automation -run 'TestDefaultConfigIncludesFriendQuietHours|TestFriendQuietHoursDecision' -count=1
~~~

Expected: PASS.

- [ ] **Step 6: Commit the pure rule**

~~~powershell
git add internal/farm/automation/config.go internal/farm/automation/quiet_hours.go internal/farm/automation/quiet_hours_test.go internal/farm/automation/catalog_test.go
git commit -m "feat: add friend quiet hours rules"
~~~

### Task 2: Enforce the Rule in Scheduler Eligibility

**Files:**
- Modify: internal/farm/automation/scheduler.go:185
- Modify: internal/farm/automation/scheduler_test.go

- [ ] **Step 1: Write failing scheduler integration tests**

Append:

~~~go
func TestSchedulerQuietHoursSkipsBlockedFriendTaskButRunsOtherDueTask(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 7, 21, 23, 30, 0, 0, location)
	settings := schedulerSettingsForSingleTask("own_base", true, 60)
	for index := range settings.Tasks {
		if settings.Tasks[index].ID == "friend_steal" {
			settings.Tasks[index].Enabled = true
			settings.Tasks[index].Priority = 200
			settings.Tasks[index].IntervalSec = 90
		}
	}
	for key, value := range quietHoursConfigForTest("sleep", "23:00", "07:00", []string{"steal", "help"}) {
		settings.Config[key] = value
	}
	settings.Config["autoFarmFriendEnabled"] = true
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(settings)
	task, ok := s.nextDueTask(now)
	if !ok || task.ID != "own_base" {
		t.Fatalf("next task = %#v, %v; want own_base", task, ok)
	}
}

func TestSchedulerQuietHoursExposesEffectiveNextRunAndWakeDelay(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 7, 21, 22, 59, 50, 0, location)
	settings := schedulerSettingsForSingleTask("friend_help", true, 90)
	for key, value := range quietHoursConfigForTest("work", "23:00", "07:00", []string{"help"}) {
		settings.Config[key] = value
	}
	settings.Config["autoFarmFriendHelpEnabled"] = true
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(settings)

	if _, ok := s.nextDueTask(now); ok {
		t.Fatal("friend_help should be blocked before work window")
	}
	if delay := s.nextWakeDelay(now); delay != 10*time.Second {
		t.Fatalf("nextWakeDelay = %s, want 10s", delay)
	}
	task := findSchedulerTaskForTest(s.State().Scheduler.Tasks, "friend_help")
	if task == nil || task.NextRunAt != "2026-07-21T23:00:00+08:00" {
		t.Fatalf("friend_help state = %#v, want effective 23:00 next run", task)
	}
}

func TestSchedulerQuietHoursMakesOverdueTaskEligibleAtAllowedBoundary(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 7, 21, 22, 59, 50, 0, location)
	settings := schedulerSettingsForSingleTask("friend_steal", true, 90)
	for key, value := range quietHoursConfigForTest("work", "23:00", "07:00", []string{"steal"}) {
		settings.Config[key] = value
	}
	settings.Config["autoFarmFriendEnabled"] = true
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(settings)

	now = time.Date(2026, 7, 21, 23, 0, 0, 0, location)
	task, ok := s.nextDueTask(now)
	if !ok || task.ID != "friend_steal" {
		t.Fatalf("boundary task = %#v, %v; want friend_steal", task, ok)
	}
}
~~~

- [ ] **Step 2: Run tests and verify RED**

~~~powershell
go test ./internal/farm/automation -run 'TestSchedulerQuietHours' -count=1
~~~

Expected: FAIL because blocked tasks remain eligible and state/wake calculations use the raw due time.

- [ ] **Step 3: Filter candidates**

In nextDueTaskLocked, after enabled/running checks and before daily completion checks:

~~~go
if decision := friendQuietHoursDecision(s.settings.Config, task.ID, now); !decision.Allowed {
	continue
}
~~~

- [ ] **Step 4: Calculate effective wake time**

In nextWakeDelay, before evaluating next:

~~~go
decision := friendQuietHoursDecision(s.settings.Config, task.ID, now)
if !decision.Allowed && decision.NextAllowedAt.IsZero() {
	continue
}
next := rt.NextRunAt
if !decision.Allowed && (next.IsZero() || decision.NextAllowedAt.After(next)) {
	next = decision.NextAllowedAt
}
~~~

Keep the existing earliest-task, minimum tick, and 30-second idle cap behavior.

- [ ] **Step 5: Publish display-only effective NextRunAt**

In applyRuntimeToStateLocked, keep the existing DailyDoneToday reset, apply the existing daily-once state first, then merge quiet-hours eligibility into that display value:

~~~go
task.NextRunAt = ""
if !rt.NextRunAt.IsZero() {
	task.NextRunAt = rt.NextRunAt.Format(time.RFC3339)
}
applyDailyOnceTaskState(task, s.settings.Config, now)

displayNextRunAt := time.Time{}
if task.NextRunAt != "" {
	displayNextRunAt, _ = time.Parse(time.RFC3339, task.NextRunAt)
}
decision := friendQuietHoursDecision(s.settings.Config, task.ID, now)
if !decision.Allowed {
	if decision.NextAllowedAt.IsZero() {
		displayNextRunAt = time.Time{}
	} else if displayNextRunAt.IsZero() || decision.NextAllowedAt.After(displayNextRunAt) {
		displayNextRunAt = decision.NextAllowedAt
	}
}
task.NextRunAt = ""
if !displayNextRunAt.IsZero() {
	task.NextRunAt = displayNextRunAt.Format(time.RFC3339)
}
~~~

Remove the old later applyDailyOnceTaskState call so it runs exactly once in the order above. Do not write displayNextRunAt back to s.runtime; overdue tasks must become immediately eligible at the allowed boundary.

- [ ] **Step 6: Format and verify GREEN plus the manual-path regression**

~~~powershell
gofmt -w internal/farm/automation/scheduler.go internal/farm/automation/scheduler_test.go
go test ./internal/farm/automation -run 'TestSchedulerQuietHours|TestSchedulerNextWakeDelay|TestSchedulerPicksHighestPriorityDueTask' -count=1
go test . -run 'TestRunFarmAutomationTaskRecordsManualTaskEvents|TestRunFarmAutomationFriendHelpManualRunDoesNotCount' -count=1
~~~

Expected: PASS. The existing app tests verify RunFarmAutomationTask stays on the manual path and bypasses scheduler eligibility.

- [ ] **Step 7: Commit scheduler enforcement**

~~~powershell
git add internal/farm/automation/scheduler.go internal/farm/automation/scheduler_test.go
git commit -m "feat: enforce friend quiet hours in scheduler"
~~~

### Task 3: Replace the Quiet-Hours UI

**Files:**
- Modify: frontend/src/views/AutomationView.tsx:2144
- Modify: frontend/src/views/AutomationView.test.tsx:1010
- Modify: frontend/src/style.css:3991

- [ ] **Step 1: Write failing render assertions**

Replace the old explanatory-copy assertions with:

~~~ts
expect(html).toContain('启用静默时间');
expect(html).toContain('运行模式');
expect(html).toContain('指定时间休眠');
expect(html).toContain('指定时间工作');
expect(html).toContain('作用范围');
expect(html).toContain('name="config-autoFarmFriendQuietHoursScopes-steal"');
expect(html).toContain('name="config-autoFarmFriendQuietHoursScopes-help"');
expect(html).toContain('name="config-autoFarmFriendQuietHoursScopes-mischief"');
expect(html).not.toContain('指定时间内不进行偷菜/捣乱/帮助');
~~~

- [ ] **Step 2: Write the failing save interaction**

Add:

~~~tsx
it('edits and saves friend quiet-hours mode, scopes, and times', async () => {
  const savedStates: FarmAutomationState[] = [];
  const renderer = TestRenderer.create(
    <AutomationView
      state={state}
      onRunTask={() => undefined}
      onSaveState={(next) => {
        savedStates.push(next);
        throw new Error('stop after capturing save payload');
      }}
      initialSettingsGroupId="friends"
    />,
  );

  expect(renderer.root.findByProps({ name: 'config-autoFarmFriendQuietHoursScopes-steal' }).props.checked).toBe(true);
  expect(renderer.root.findByProps({ name: 'config-autoFarmFriendQuietHoursScopes-help' }).props.checked).toBe(true);
  expect(renderer.root.findByProps({ name: 'config-autoFarmFriendQuietHoursScopes-mischief' }).props.checked).toBe(false);

  await act(async () => {
    renderer.root.findByProps({ 'aria-label': '指定时间工作' }).props.onClick();
  });
  await act(async () => {
    renderer.root.findByProps({ name: 'config-autoFarmFriendQuietHoursScopes-help' }).props.onChange({ currentTarget: { checked: false } });
  });
  await act(async () => {
    renderer.root.findByProps({ name: 'config-autoFarmFriendQuietHoursScopes-mischief' }).props.onChange({ currentTarget: { checked: true } });
  });
  await act(async () => {
	    renderer.root.findByProps({ name: 'config-autoFarmFriendQuietHoursStart' }).props.onChange({ currentTarget: { value: '09:00' } });
	    renderer.root.findByProps({ name: 'config-autoFarmFriendQuietHoursEnd' }).props.onChange({ currentTarget: { value: '17:30' } });
  });
  await act(async () => {
    await renderer.root.findByProps({ className: 'automation-save-button' }).props.onClick();
  });

  expect(savedStates[0].config.autoFarmFriendQuietHoursMode).toBe('work');
  expect(savedStates[0].config.autoFarmFriendQuietHoursScopes).toEqual(['steal', 'mischief']);
  expect(savedStates[0].config.autoFarmFriendQuietHoursStart).toBe('09:00');
  expect(savedStates[0].config.autoFarmFriendQuietHoursEnd).toBe('17:30');
});
~~~

Add exact CSS contracts:

~~~ts
expect(styleSource).toMatch(
  /\.automation-friend-quiet-primary\s*\{[^}]*grid-template-columns:\s*minmax\([^;]+repeat\(2,/s,
);
expect(styleSource).toMatch(
  /\.automation-friend-quiet-times\s*\{[^}]*grid-template-columns:\s*repeat\(2,/s,
);
expect(styleSource).toMatch(
  /@media \(max-width: 760px\)[\s\S]*\.automation-friend-quiet-primary,[\s\S]*\.automation-friend-quiet-times\s*\{[^}]*grid-template-columns:\s*1fr/s,
);
~~~

- [ ] **Step 3: Run the focused frontend test and verify RED**

~~~powershell
Set-Location frontend
npm test -- AutomationView.test.tsx
~~~

Expected: FAIL because mode/scope controls do not exist and old copy still renders.

- [ ] **Step 4: Derive stable UI defaults**

At the start of the friends branch:

~~~tsx
const quietHoursMode = String(configValue('autoFarmFriendQuietHoursMode', 'sleep')) === 'work' ? 'work' : 'sleep';
const quietHoursScopesValue = configValue('autoFarmFriendQuietHoursScopes', ['steal', 'help']);
const quietHoursScopes = Array.isArray(quietHoursScopesValue)
  ? stringListFromConfig(quietHoursScopesValue).filter((scope) => ['steal', 'help', 'mischief'].includes(scope))
  : ['steal', 'help'];
const quietHoursScopeOptions = [
  ['steal', '偷菜'],
  ['help', '帮助'],
  ['mischief', '捣乱'],
] as const;
~~~

- [ ] **Step 5: Replace the current quiet-card markup**

Use the confirmed two-row structure:

~~~tsx
<section className="automation-friend-quiet-card">
  <div className="automation-friend-quiet-primary">
    <label className="automation-settings-check">
      <input
        checked={configBool('autoFarmFriendQuietHoursEnabled', false)}
        name="config-autoFarmFriendQuietHoursEnabled"
        type="checkbox"
        onChange={(event) => updateConfig('autoFarmFriendQuietHoursEnabled', event.currentTarget.checked)}
      />
      <span>启用静默时间</span>
    </label>
    <div className="automation-quiet-choice-field">
      <strong>运行模式</strong>
      <div aria-label="运行模式" className="automation-quiet-segmented" role="radiogroup">
        {([['sleep', '指定时间休眠'], ['work', '指定时间工作']] as const).map(([value, label]) => (
          <button
            aria-checked={quietHoursMode === value}
            aria-label={label}
            className={quietHoursMode === value ? 'active' : ''}
            key={value}
            role="radio"
            type="button"
            onClick={() => updateConfig('autoFarmFriendQuietHoursMode', value)}
          >
            {label}
          </button>
        ))}
      </div>
    </div>
    <div className="automation-quiet-choice-field">
      <strong>作用范围</strong>
      <div className="automation-quiet-scopes">
        {quietHoursScopeOptions.map(([value, label]) => (
          <label key={value}>
            <input
              checked={quietHoursScopes.includes(value)}
              name={'config-autoFarmFriendQuietHoursScopes-' + value}
              type="checkbox"
              onChange={(event) =>
                updateConfig('autoFarmFriendQuietHoursScopes', toggleStringList(quietHoursScopes, value, event.currentTarget.checked))
              }
            />
            <span>{label}</span>
          </label>
        ))}
      </div>
    </div>
  </div>
  <div className="automation-friend-quiet-times">
    <label className="automation-settings-field">
      开始时间
      <input
        name="config-autoFarmFriendQuietHoursStart"
        type="time"
        value={String(configValue('autoFarmFriendQuietHoursStart', '23:00'))}
        onChange={(event) => updateConfig('autoFarmFriendQuietHoursStart', event.currentTarget.value)}
      />
    </label>
    <label className="automation-settings-field">
      结束时间
      <input
        name="config-autoFarmFriendQuietHoursEnd"
        type="time"
        value={String(configValue('autoFarmFriendQuietHoursEnd', '07:00'))}
        onChange={(event) => updateConfig('autoFarmFriendQuietHoursEnd', event.currentTarget.value)}
      />
    </label>
  </div>
</section>
~~~

- [ ] **Step 6: Replace the old card grid and paragraph CSS**

~~~css
.automation-friend-quiet-card {
  grid-column: 1 / -1;
  display: grid;
  gap: 10px;
  min-width: 0;
  padding: 10px;
  border: 1px solid #eadfc9;
  border-radius: 8px;
  background: #fff8ea;
}

.automation-friend-quiet-primary {
  display: grid;
  grid-template-columns: minmax(150px, 0.8fr) repeat(2, minmax(220px, 1.25fr));
  gap: 10px;
  min-width: 0;
}

.automation-friend-quiet-times {
  display: grid;
  grid-template-columns: repeat(2, minmax(150px, 1fr));
  gap: 10px;
  min-width: 0;
}

.automation-quiet-choice-field {
  display: grid;
  gap: 7px;
  min-width: 0;
  padding: 9px 11px;
  border: 1px solid #e0d0b3;
  border-radius: 8px;
  background: #fffdf7;
}

.automation-quiet-choice-field > strong {
  color: #776e5b;
  font-size: 11px;
  font-weight: 950;
}

.automation-quiet-segmented {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px;
  padding: 3px;
  border: 1px solid #d6c9ae;
  border-radius: 8px;
  background: #f6efdf;
}

.automation-quiet-segmented button {
  min-width: 0;
  height: 30px;
  border: 1px solid transparent;
  border-radius: 7px;
  color: #665f50;
  background: transparent;
  font-size: 11px;
  font-weight: 900;
  cursor: pointer;
}

.automation-quiet-segmented button.active {
  border-color: #b99254;
  color: #20271f;
  background: #fffaf0;
  box-shadow: 0 2px 8px rgba(75, 59, 32, 0.12);
}

.automation-quiet-scopes {
  display: flex;
  align-items: center;
  gap: 14px;
  min-height: 32px;
  color: #2a3129;
  font-size: 12px;
  font-weight: 900;
}

.automation-quiet-scopes label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.automation-quiet-scopes input {
  width: 15px;
  height: 15px;
  accent-color: #338a35;
}
~~~

Inside the existing max-width 760px block:

~~~css
.automation-friend-quiet-primary,
.automation-friend-quiet-times {
  grid-template-columns: 1fr;
}

.automation-quiet-scopes {
  flex-wrap: wrap;
}
~~~

- [ ] **Step 7: Verify frontend GREEN and production build**

~~~powershell
Set-Location frontend
npm test -- AutomationView.test.tsx
npm run build
~~~

Expected: focused Vitest passes, TypeScript and Vite succeed, and frontend/dist/index.html references a new assets/index-*.js file.

- [ ] **Step 8: Commit the UI**

~~~powershell
git add frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx frontend/src/style.css
git commit -m "feat: upgrade friend quiet hours controls"
~~~

### Task 4: Full Regression and Visual Verification

**Files:**
- Verify: internal/farm/automation/quiet_hours.go
- Verify: internal/farm/automation/scheduler.go
- Verify: frontend/src/views/AutomationView.tsx
- Verify generated ignored files: frontend/dist/index.html and frontend/dist/assets/*

- [ ] **Step 1: Run all Go tests**

~~~powershell
Set-Location E:\desktop\Farm_Go
go test ./...
~~~

Expected: PASS across the root app and internal packages.

- [ ] **Step 2: Run all frontend tests**

~~~powershell
Set-Location E:\desktop\Farm_Go\frontend
npm test
~~~

Expected: all Vitest suites PASS without warnings or unhandled errors.

- [ ] **Step 3: Rebuild and verify the generated asset**

~~~powershell
Set-Location E:\desktop\Farm_Go\frontend
$before = (Get-Item -LiteralPath 'dist\index.html' -ErrorAction SilentlyContinue).LastWriteTimeUtc
npm run build
$index = Get-Item -LiteralPath 'dist\index.html'
$asset = Select-String -Path 'dist\index.html' -Pattern '/assets/index-[^"]+\.js' | Select-Object -First 1
$index.LastWriteTimeUtc
$asset.Matches.Value
~~~

Expected: build succeeds, index.html is newer than $before, and a current hashed JavaScript asset is printed.

- [ ] **Step 4: Start the development server and inspect desktop/mobile**

~~~powershell
Set-Location E:\desktop\Farm_Go\frontend
npm run dev -- --host 127.0.0.1 --port 5180
~~~

Inspect the friend settings above and below 760px. Confirm the first row is switch/mode/scopes, the second is start/end, old copy is absent, interactions do not shift or overflow, and mobile stacks to one column.

- [ ] **Step 5: Check the final diff and working tree**

~~~powershell
Set-Location E:\desktop\Farm_Go
git diff --check HEAD~3..HEAD
git status --short
~~~

Expected: no whitespace errors and no unexpected tracked changes.
