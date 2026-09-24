# Retire He Feng Reward Controls Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the ended He Feng Travel reward controls and ensure persisted or recommended settings cannot automatically run either retained task.

**Architecture:** Keep the existing task catalog and runtime protocol implementations intact. Enforce retirement at the backend configuration boundary by normalizing the three enable switches to `false`, align the recommended preset with that rule, and omit both rows from the frontend reward settings list.

**Tech Stack:** Go, React 18, TypeScript, Vitest, ReactDOM server rendering

---

### Task 1: Lock retired automation settings off in the backend

**Files:**
- Modify: `internal/farm/automation/catalog_test.go`
- Modify: `app_test.go`
- Modify: `internal/farm/automation/config.go`
- Modify: `recommended_automation_config.go`

- [ ] **Step 1: Write failing configuration-normalization tests**

Add this test near the other default-configuration tests in `internal/farm/automation/catalog_test.go`:

```go
func TestMergeConfigWithDefaultsDisablesRetiredHeFengTasks(t *testing.T) {
	config := MergeConfigWithDefaults(map[string]any{
		"autoFarmHeFengTravelRewardEnabled":  true,
		"autoFarmLimitedSeedDrawEnabled":     true,
		"autoFarmLimitedSeedDrawPaidEnabled": true,
	})

	for _, key := range []string{
		"autoFarmHeFengTravelRewardEnabled",
		"autoFarmLimitedSeedDrawEnabled",
		"autoFarmLimitedSeedDrawPaidEnabled",
	} {
		if got := config[key]; got != false {
			t.Fatalf("config[%s] = %#v, want false", key, got)
		}
	}
}
```

Add this test near `TestApplyRecommendedFarmAutomationConfigUpdatesOnlyRecommendedSettings` in `app_test.go` so it checks the raw preset before configuration normalization:

```go
func TestRecommendedFarmAutomationSettingsDisableRetiredHeFengTasks(t *testing.T) {
	recommended := recommendedFarmAutomationSettings()
	for _, taskID := range []string{"he_feng_travel_reward", "limited_seed_draw"} {
		if got := findTaskSettings(recommended.Tasks, taskID); got == nil || got.Enabled {
			t.Fatalf("retired task %s = %#v, want disabled", taskID, got)
		}
	}
	for _, key := range []string{
		"autoFarmHeFengTravelRewardEnabled",
		"autoFarmLimitedSeedDrawEnabled",
		"autoFarmLimitedSeedDrawPaidEnabled",
	} {
		if got := recommended.Config[key]; got != false {
			t.Fatalf("recommended config[%s] = %#v, want false", key, got)
		}
	}
}

func findTaskSettings(tasks []automation.TaskSettings, id string) *automation.TaskSettings {
	for index := range tasks {
		if tasks[index].ID == id {
			return &tasks[index]
		}
	}
	return nil
}
```

- [ ] **Step 2: Run the focused tests and verify they fail for the retired switches**

Run:

```powershell
go test ./internal/farm/automation -run TestMergeConfigWithDefaultsDisablesRetiredHeFengTasks -count=1
go test . -run TestRecommendedFarmAutomationSettingsDisableRetiredHeFengTasks -count=1
```

Expected: both commands fail because caller-provided and recommended values are currently `true`.

- [ ] **Step 3: Normalize the retired switches after every configuration merge**

Add a package-level list and the normalization loop to `internal/farm/automation/config.go`:

```go
var retiredAutomationSwitches = []string{
	"autoFarmHeFengTravelRewardEnabled",
	"autoFarmLimitedSeedDrawEnabled",
	"autoFarmLimitedSeedDrawPaidEnabled",
}

func MergeConfigWithDefaults(config map[string]any) map[string]any {
	merged := cloneConfig(DefaultConfig())
	for key, value := range config {
		if key == "" || value == nil {
			continue
		}
		merged[key] = value
	}
	for _, key := range retiredAutomationSwitches {
		merged[key] = false
	}
	return merged
}
```

This leaves the keys in `DefaultConfig` and preserves compatibility while overriding persisted `true` values.

- [ ] **Step 4: Disable the tasks and switches in the recommended preset**

In `recommended_automation_config.go`, change only the two task entries and three switch values:

```go
			{ID: "he_feng_travel_reward", Enabled: false, Priority: 92, IntervalSec: 43200},
			{ID: "limited_seed_draw", Enabled: false, Priority: 92, IntervalSec: 43200},
```

```go
			"autoFarmHeFengTravelRewardEnabled": false, "autoFarmHeFengTravelRewardIntervalMin": 720, "autoFarmHeFengTravelRewardIntervalSec": 43200,
```

```go
			"autoFarmLimitedSeedDrawEnabled": false, "autoFarmLimitedSeedDrawIntervalMin": 720, "autoFarmLimitedSeedDrawIntervalSec": 43200,
			"autoFarmLimitedSeedDrawPaidEnabled": false, "autoFarmLimitedSeedDrawScheduleMode": "daily_time", "autoFarmLimitedSeedDrawScheduleTime": "00:07",
```

- [ ] **Step 5: Format and run the focused tests until they pass**

Run:

```powershell
gofmt -w internal/farm/automation/config.go internal/farm/automation/catalog_test.go recommended_automation_config.go app_test.go
go test ./internal/farm/automation -run TestMergeConfigWithDefaultsDisablesRetiredHeFengTasks -count=1
go test . -run TestRecommendedFarmAutomationSettingsDisableRetiredHeFengTasks -count=1
```

Expected: both test commands report `ok`.

- [ ] **Step 6: Commit the backend behavior**

```powershell
git add -- internal/farm/automation/config.go internal/farm/automation/catalog_test.go recommended_automation_config.go app_test.go
git commit -m "fix: disable retired He Feng automation"
```

### Task 2: Remove the frontend configuration entries

**Files:**
- Modify: `frontend/src/views/AutomationView.test.tsx`
- Modify: `frontend/src/views/AutomationView.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Change the reward-settings render test to require absent controls**

Replace the He Feng assertions in `renders editable rewards and events detailed settings` with:

```tsx
    expect(html).not.toContain('荷风游记奖励');
    expect(html).not.toContain('荷风游记抽奖');
    expect(html).not.toContain('允许付费抽奖');
    expect(html).not.toContain('automation-he-feng-draw-card');
    expect(html).not.toContain('name="config-autoFarmHeFengTravelRewardEnabled"');
    expect(html).not.toContain('name="config-autoFarmLimitedSeedDrawEnabled"');
    expect(html).not.toContain('name="config-autoFarmLimitedSeedDrawPaidEnabled"');
    expect(html).not.toContain('name="config-autoFarmLimitedSeedDrawScheduleMode"');
    expect(html).not.toContain('name="config-autoFarmLimitedSeedDrawIntervalMin"');
    expect(html).not.toContain('name="config-autoFarmLimitedSeedDrawIntervalSec"');
```

Keep the positive assertions for the other six reward controls and their shared schedule controls.

- [ ] **Step 2: Run the focused frontend test and verify it fails**

Run:

```powershell
npm test -- src/views/AutomationView.test.tsx
```

Working directory: `frontend`

Expected: the render test fails because both He Feng rows and the paid-draw checkbox are still present.

- [ ] **Step 3: Remove only the He Feng rows and their conditional markup**

Delete these two entries from `rewardItems` in `frontend/src/views/AutomationView.tsx`:

```tsx
    ['荷风游记奖励', 'he_feng_travel_reward', 43200],
    ['荷风游记抽奖', 'limited_seed_draw', 43200],
```

Remove `isHeFengDraw`, use the common card class directly, and delete the paid-draw conditional:

```tsx
            <section className="automation-reward-schedule-card" key={taskId}>
```

Do not remove scheduler key maps, labels, runtime handlers, or protocol code.

Delete the now-unreferenced `.automation-he-feng-draw-card` and `.automation-paid-draw-check` rules from `frontend/src/style.css`.

- [ ] **Step 4: Run the focused frontend test until it passes**

Run:

```powershell
npm test -- src/views/AutomationView.test.tsx
```

Working directory: `frontend`

Expected: all tests in `AutomationView.test.tsx` pass.

- [ ] **Step 5: Commit the frontend change**

```powershell
git add -- frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx frontend/src/style.css
git commit -m "fix: hide retired He Feng reward controls"
```

### Task 3: Run regression verification

**Files:**
- Verify only; no planned modifications

- [ ] **Step 1: Run backend package tests**

```powershell
go test ./internal/farm/automation -count=1
go test . -count=1
```

Expected: both commands report `ok`.

- [ ] **Step 2: Run frontend tests and production build**

Run from `frontend`:

```powershell
npm test
npm run build
```

Expected: Vitest reports all tests passing and Vite completes a production build without TypeScript errors.

- [ ] **Step 3: Confirm retained implementation and scoped diff**

```powershell
rg -n "he_feng_travel_reward|limited_seed_draw" internal/farm/automation/runtime_rewards.go internal/farm/automation/scheduler.go
git diff --check HEAD~2..HEAD
git status --short
```

Expected: runtime and scheduler matches remain, `git diff --check` is silent, and the working tree is clean.
