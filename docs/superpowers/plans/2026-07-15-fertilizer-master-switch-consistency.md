# Fertilizer Master Switch Consistency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the fertilizer feature-group switch, `autoFarmFertilizerEnabled`, and `own_fertilizer.enabled` one consistent master switch that gates both planting fertilizer and rush fertilizer.

**Architecture:** Normalize the three persisted backend states at both load and save boundaries, using an explicit fertilizer feature-group value as the compatibility winner. Mirror the same rule in the React draft model so either visible switch updates the other immediately, while fertilizer modes remain independent child strategies. Gate both runtime fertilizer entry points with the canonical master key.

**Tech Stack:** Go 1.25, React 18, TypeScript 5.9, Vitest, react-test-renderer, pnpm/Vite

---

### Task 1: Canonicalize the backend fertilizer master switch

**Files:**
- Modify: `internal/farm/automation/catalog.go:208-373`
- Test: `internal/farm/automation/catalog_test.go:209-442`

- [ ] **Step 1: Write failing backend state tests**

Replace the fertilizer case that currently preserves a task preference independently from the group, and add conflict/fallback coverage:

```go
func TestStateFromSettingsUsesFertilizerGroupAsMasterSwitch(t *testing.T) {
	state := StateFromSettings(Settings{
		SchedulerEnabled: true,
		Tasks: []TaskSettings{{ID: "own_fertilizer", Enabled: false, Priority: 85, IntervalSec: 30}},
		Config: map[string]any{
			"autoFarmFeatureGroupEnabled.fertilizer": true,
			"autoFarmFertilizerEnabled":              false,
		},
	})

	if state.Config["autoFarmFertilizerEnabled"] != true {
		t.Fatalf("autoFarmFertilizerEnabled = %#v, want true", state.Config["autoFarmFertilizerEnabled"])
	}
	if task := findSchedulerTaskForTest(state.Scheduler.Tasks, "own_fertilizer"); task == nil || !task.Enabled {
		t.Fatalf("own_fertilizer should follow fertilizer group master switch: %#v", task)
	}
}

func TestStateFromSettingsFallsBackToLegacyFertilizerSwitch(t *testing.T) {
	state := StateFromSettings(Settings{
		SchedulerEnabled: true,
		Tasks: []TaskSettings{{ID: "own_fertilizer", Enabled: false, Priority: 85, IntervalSec: 30}},
		Config: map[string]any{"autoFarmFertilizerEnabled": true},
	})

	foundEnabledGroup := false
	for _, group := range state.FeatureGroups {
		if group.ID == "fertilizer" && group.Enabled {
			foundEnabledGroup = true
		}
	}
	if !foundEnabledGroup {
		t.Fatalf("fertilizer group should follow legacy switch: %#v", state.FeatureGroups)
	}
	if task := findSchedulerTaskForTest(state.Scheduler.Tasks, "own_fertilizer"); task == nil || !task.Enabled {
		t.Fatalf("own_fertilizer should follow legacy switch: %#v", task)
	}
}

func TestSettingsFromStateWritesConsistentFertilizerMasterSwitch(t *testing.T) {
	state := DefaultState()
	for index := range state.FeatureGroups {
		if state.FeatureGroups[index].ID == "fertilizer" {
			state.FeatureGroups[index].Enabled = true
		}
	}
	state.Config["autoFarmFertilizerEnabled"] = false

	settings := SettingsFromState(state)

	if settings.Config["autoFarmFeatureGroupEnabled.fertilizer"] != true || settings.Config["autoFarmFertilizerEnabled"] != true {
		t.Fatalf("fertilizer master config is inconsistent: %#v", settings.Config)
	}
	if task := findTaskSettingsForTest(settings.Tasks, "own_fertilizer"); task == nil || !task.Enabled {
		t.Fatalf("persisted own_fertilizer should be enabled: %#v", task)
	}
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `go test ./internal/farm/automation -run 'Test(StateFromSettingsUsesFertilizerGroupAsMasterSwitch|StateFromSettingsFallsBackToLegacyFertilizerSwitch|SettingsFromStateWritesConsistentFertilizerMasterSwitch)$' -count=1`

Expected: FAIL because the current group, config, and task values remain independent.

- [ ] **Step 3: Add minimal backend normalization**

Add focused helpers in `catalog.go` and call them before default merging in `StateFromSettings`, then after task conversion in `SettingsFromState`:

```go
const (
	fertilizerFeatureGroupConfigKey = "autoFarmFeatureGroupEnabled.fertilizer"
	fertilizerTaskConfigKey         = "autoFarmFertilizerEnabled"
	fertilizerTaskID                = "own_fertilizer"
)

func resolveFertilizerMasterSwitch(settings Settings) (bool, bool) {
	if value, ok := settings.Config[fertilizerFeatureGroupConfigKey]; ok {
		return boolConfig(value, false), true
	}
	if value, ok := settings.Config[fertilizerTaskConfigKey]; ok {
		return boolConfig(value, false), true
	}
	for _, task := range settings.Tasks {
		if task.ID == fertilizerTaskID {
			return task.Enabled, true
		}
	}
	return false, false
}

func applyFertilizerMasterSwitch(settings Settings, enabled bool) Settings {
	settings.Config = cloneConfig(settings.Config)
	settings.Config[fertilizerFeatureGroupConfigKey] = enabled
	settings.Config[fertilizerTaskConfigKey] = enabled
	for index := range settings.Tasks {
		if settings.Tasks[index].ID == fertilizerTaskID {
			settings.Tasks[index].Enabled = enabled
		}
	}
	return settings
}
```

In `StateFromSettings`, resolve against migrated explicit configuration, merge defaults, then apply the resolved value before building `explicitConfig`:

```go
settings.Config = migrateLegacyFeatureGroupSwitches(settings.Config)
fertilizerEnabled, hasFertilizerEnabled := resolveFertilizerMasterSwitch(settings)
settings = mergeSettingsWithDefaults(settings)
if hasFertilizerEnabled {
	settings = applyFertilizerMasterSwitch(settings, fertilizerEnabled)
}
explicitConfig := cloneConfig(settings.Config)
```

In `SettingsFromState`, apply the explicit feature-group value after constructing `settings.Tasks` and before merging defaults:

```go
if fertilizerEnabled, ok := resolveFertilizerMasterSwitch(settings); ok {
	settings = applyFertilizerMasterSwitch(settings, fertilizerEnabled)
}
return mergeSettingsWithDefaults(settings)
```

- [ ] **Step 4: Run the focused and full catalog tests and verify GREEN**

Run: `go test ./internal/farm/automation -run 'Test(StateFromSettings|SettingsFromState|SettingsRoundTrip)' -count=1`

Expected: PASS, including existing non-fertilizer group-preference tests.

- [ ] **Step 5: Commit the backend canonicalization**

```bash
git add internal/farm/automation/catalog.go internal/farm/automation/catalog_test.go
git commit -m "fix: unify fertilizer master switch state"
```

### Task 2: Gate both fertilizer runtime paths

**Files:**
- Modify: `internal/farm/automation/runtime_own.go:276-408,870-880`
- Test: `internal/farm/automation/runtime_own_test.go:491-594,1099-1408`

- [ ] **Step 1: Write failing runtime gate tests**

Add tests proving both sub-strategies obey the master switch:

```go
func TestRuntimeFacadeOwnPlantSkipsPlantFertilizerWhenMasterDisabled(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFarmOwnership": map[string]any{"farmType": "own"},
		"gameCtl.getFarmStatus": map[string]any{
			"farmType": "own",
			"grids": []any{map[string]any{"landId": float64(1), "stageKind": "empty", "interactable": true}},
		},
		"gameCtl.autoPlant": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled": false,
		"autoFarmPlantPrimaryMode": "specified_seed",
		"autoFarmPlantSeedId": 20002,
		"autoFarmPlantFertilizerMode": "normal",
	})

	result := facade.RunTask(context.Background(), "own_plant")

	fertilizerCalled := false
	for _, call := range caller.calls {
		if call.method == "gameCtl.fertilizeLandsBatch" {
			fertilizerCalled = true
		}
	}
	if !result.OK || fertilizerCalled {
		t.Fatalf("disabled fertilizer master should skip plant fertilizer: result=%#v calls=%#v", result, caller.calls)
	}
}

func TestRuntimeFacadeOwnFertilizerSkipsWhenMasterDisabled(t *testing.T) {
	caller := &fakeRuntimeCaller{}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFertilizerEnabled": false,
		"autoFarmRushFertilizerMode": "organic",
	})

	result := facade.RunTask(context.Background(), "own_fertilizer")

	if !result.OK || len(caller.calls) != 0 {
		t.Fatalf("disabled fertilizer master should skip rush fertilizer: result=%#v calls=%#v", result, caller.calls)
	}
}
```

- [ ] **Step 2: Run the gate tests and verify RED**

Run: `go test ./internal/farm/automation -run 'TestRuntimeFacadeOwn(PlantSkipsPlantFertilizerWhenMasterDisabled|FertilizerSkipsWhenMasterDisabled)$' -count=1`

Expected: FAIL because both current runtime paths ignore `autoFarmFertilizerEnabled`.

- [ ] **Step 3: Implement the runtime master gate**

Add and reuse one helper:

```go
func fertilizerMasterEnabled(config map[string]any) bool {
	return boolConfigAny(config["autoFarmFertilizerEnabled"])
}

func plantFertilizerMode(config map[string]any) string {
	if !fertilizerMasterEnabled(config) {
		return ""
	}
	mode, _ := config["autoFarmPlantFertilizerMode"].(string)
	if mode == "normal" || mode == "organic" {
		return mode
	}
	return ""
}
```

At the beginning of `runOwnFertilizer`, return an OK skip before checking the rush mode when the master is disabled. Keep the existing `none` behavior unchanged when the master is enabled.

Update pre-existing success-path runtime test configurations to include `"autoFarmFertilizerEnabled": true`; this makes each test's intended precondition explicit.

- [ ] **Step 4: Run all own-farm runtime tests and verify GREEN**

Run: `go test ./internal/farm/automation -run 'TestRuntimeFacadeOwn' -count=1`

Expected: PASS with planting fertilizer, rush fertilizer, disabled-master, and `none`-mode behavior all covered.

- [ ] **Step 5: Commit the runtime gate**

```bash
git add internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go
git commit -m "fix: gate fertilizer operations with master switch"
```

### Task 3: Synchronize the visible frontend switches

**Files:**
- Modify: `frontend/src/views/AutomationView.tsx:370-400,493-508,829-947`
- Test: `frontend/src/views/AutomationView.test.tsx:102-146,409-484,724-742`

- [ ] **Step 1: Write failing frontend state and interaction tests**

Export a wished-for `syncFertilizerMasterEnabledState` API and add tests for compatibility normalization and both visible entry points:

```tsx
it('uses the fertilizer feature group as the visible master switch', () => {
  const synced = syncFertilizerMasterEnabledState({
    ...state,
    featureGroups: state.featureGroups.map((group) => (group.id === 'fertilizer' ? { ...group, enabled: true } : group)),
    config: { ...state.config, autoFarmFertilizerEnabled: false },
    scheduler: {
      ...state.scheduler,
      tasks: [...state.scheduler.tasks, { id: 'own_fertilizer', label: '自动施肥', priority: 85, intervalSec: 30, enabled: false }],
    },
  });

  expect(synced.config['autoFarmFeatureGroupEnabled.fertilizer']).toBe(true);
  expect(synced.config.autoFarmFertilizerEnabled).toBe(true);
  expect(synced.scheduler.tasks.find((task) => task.id === 'own_fertilizer')?.enabled).toBe(true);
});

it('syncs the fertilizer feature card switch into scheduler state', async () => {
  const savedStates: FarmAutomationState[] = [];
  const fertilizerState = {
    ...state,
    scheduler: {
      ...state.scheduler,
      tasks: [...state.scheduler.tasks, { id: 'own_fertilizer', label: '自动施肥', priority: 85, intervalSec: 30, enabled: false }],
    },
  };
  const renderer = TestRenderer.create(
    <AutomationView state={fertilizerState} onRunTask={() => undefined} onSaveState={(next) => { savedStates.push(next); throw new Error('capture'); }} />,
  );

  await act(async () => {
    await renderer.root.findByProps({ 'aria-label': '开启自动施肥' }).props.onClick();
  });

  expect(savedStates[0].config.autoFarmFertilizerEnabled).toBe(true);
  expect(savedStates[0].scheduler.tasks.find((task) => task.id === 'own_fertilizer')?.enabled).toBe(true);
});

it('syncs the scheduler fertilizer switch back into the feature group', async () => {
  const savedStates: FarmAutomationState[] = [];
  const enabledState = syncFertilizerMasterEnabledState({
    ...state,
    featureGroups: state.featureGroups.map((group) => (group.id === 'fertilizer' ? { ...group, enabled: true } : group)),
    scheduler: {
      ...state.scheduler,
      tasks: [...state.scheduler.tasks, { id: 'own_fertilizer', label: '自动施肥', priority: 85, intervalSec: 30, enabled: true }],
    },
  });
  const renderer = TestRenderer.create(
    <AutomationView state={enabledState} onRunTask={() => undefined} onSaveState={(next) => { savedStates.push(next); throw new Error('capture'); }} initialSchedulerOpen />,
  );

  await act(async () => {
    renderer.root.findByProps({ name: 'enabled-own_fertilizer' }).props.onChange({ target: { checked: false } });
    await renderer.root.findByProps({ className: 'automation-save-button' }).props.onClick();
  });

  expect(savedStates[0].featureGroups.find((group) => group.id === 'fertilizer')?.enabled).toBe(false);
  expect(savedStates[0].config.autoFarmFertilizerEnabled).toBe(false);
});
```

- [ ] **Step 2: Run the frontend tests and verify RED**

Run: `pnpm --dir frontend test -- AutomationView.test.tsx`

Expected: FAIL because the synchronization helper and fertilizer-specific bidirectional behavior do not exist.

- [ ] **Step 3: Implement minimal frontend synchronization**

Add an exported helper and call it from `normalizeAutomationState`; allow an explicit value for scheduler-originated changes:

```tsx
export function syncFertilizerMasterEnabledState(state: FarmAutomationState, forcedEnabled?: boolean): FarmAutomationState {
  const fertilizerGroup = state.featureGroups.find((group) => group.id === 'fertilizer');
  const fertilizerTask = state.scheduler.tasks.find((task) => task.id === 'own_fertilizer');
  const enabled = typeof forcedEnabled === 'boolean'
    ? forcedEnabled
    : fertilizerGroup?.enabled ?? boolFromConfig(state.config.autoFarmFertilizerEnabled, fertilizerTask?.enabled ?? false);

  return {
    ...state,
    featureGroups: state.featureGroups.map((group) => (group.id === 'fertilizer' ? { ...group, enabled } : group)),
    config: {
      ...state.config,
      'autoFarmFeatureGroupEnabled.fertilizer': enabled,
      autoFarmFertilizerEnabled: enabled,
    },
    scheduler: {
      ...state.scheduler,
      tasks: state.scheduler.tasks.map((task) => (task.id === 'own_fertilizer' ? { ...task, enabled } : task)),
    },
  };
}
```

Call the helper at the start of `normalizeAutomationState`:

```diff
 function normalizeAutomationState(state: FarmAutomationState, options: { intervalSource?: 'config' | 'scheduler' } = {}): FarmAutomationState {
+  state = syncFertilizerMasterEnabledState(state);
   const tasks = state.scheduler.tasks.map((task) => ({
```

In `updateTask`, build the existing next state, then force fertilizer synchronization for a scheduler-originated toggle before normalizing:

```tsx
let nextState: FarmAutomationState = {
  ...current,
  config: currentTask ? schedulerConfigForTaskUpdate(current.config, currentTask, next) : current.config,
  scheduler: {
    ...current.scheduler,
    tasks: current.scheduler.tasks.map((task) => (task.id === taskId ? { ...task, ...next } : task)),
  },
};
if (taskId === 'own_fertilizer' && typeof next.enabled === 'boolean') {
  nextState = syncFertilizerMasterEnabledState(nextState, next.enabled);
}
return normalizeAutomationState(nextState, { intervalSource: 'scheduler' });
```

Add ``name={`enabled-${task.id}`}`` to the existing scheduler checkbox so the control has a stable form/accessibility identifier used by the interaction test. Add `syncFertilizerMasterEnabledState` to the named imports in `AutomationView.test.tsx`. Do not connect either fertilizer mode selector to the master switch.

- [ ] **Step 4: Run focused and full frontend tests and verify GREEN**

Run: `pnpm --dir frontend test -- AutomationView.test.tsx`

Then run: `pnpm --dir frontend test`

Expected: all Vitest tests PASS.

- [ ] **Step 5: Commit the frontend synchronization**

```bash
git add frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx
git commit -m "fix: synchronize fertilizer master switches"
```

### Task 4: Full verification and production frontend build

**Files:**
- Generated: `frontend/dist/index.html`
- Generated: `frontend/dist/assets/*`
- Verify: `public/app/index.html` if the repository build syncs that deployment surface

- [ ] **Step 1: Run formatting and complete Go tests**

Run: `gofmt -w internal/farm/automation/catalog.go internal/farm/automation/catalog_test.go internal/farm/automation/runtime_own.go internal/farm/automation/runtime_own_test.go`

Run: `go test ./... -count=1`

Expected: all Go packages PASS.

- [ ] **Step 2: Run complete frontend tests**

Run: `pnpm --dir frontend test`

Expected: all Vitest files PASS.

- [ ] **Step 3: Run the required production frontend build**

Run the repository-equivalent build command: `pnpm --dir frontend run build`

Expected: TypeScript checking and Vite production build exit with code 0, `frontend/dist/index.html` references a newly generated hashed asset, and that asset has a fresh timestamp. If `pnpm run frontend:build` is available in the active shell environment, run it as the deployment sync step and verify `public/app/index.html` references the new asset; otherwise report that the repository has no root `package.json` script and verify the actual embedded `frontend/dist` surface used by `main.go`.

- [ ] **Step 4: Inspect the final diff and working tree**

Run: `git diff --check`

Run: `git status --short`

Expected: only the intended source/tests and generated frontend assets differ, plus the user's pre-existing unrelated changes; no whitespace errors.
