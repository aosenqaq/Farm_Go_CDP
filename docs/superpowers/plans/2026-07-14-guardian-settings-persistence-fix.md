# Guardian Settings Persistence Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every numeric parameter in the three guardian cards retain and display its persisted value after status refreshes and application restarts.

**Architecture:** Add the existing normalized `guard.Settings` value to `GuardianStatusDTO`, sourced from persisted runtime settings. Keep the save and storage paths unchanged, and make `GuardView` render numeric inputs from this settings snapshot with compatibility fallbacks.

**Tech Stack:** Go, Wails, React 18, TypeScript, Vitest, react-test-renderer

---

### Task 1: Expose persisted guardian settings in status

**Files:**
- Modify: `app.go:154-162`
- Modify: `app.go:2626-2643`
- Test: `app_test.go:1808`

- [ ] **Step 1: Write the failing backend regression test**

Add a test that saves non-default values and requires `GuardianStatus` to expose them:

```go
func TestGuardianStatusReturnsPersistedSettings(t *testing.T) {
	app := newAppWithTempStore(t)
	_, err := app.SaveGuardianSettings(map[string]any{
		"timeoutThreshold":           7,
		"monitorIntervalMs":          4321,
		"networkReconnectIntervalMs": 1300,
		"networkRecoveryTimeoutMs":   18000,
		"otherPlaceLoginIntervalMs":  4500,
		"otherPlaceLoginDelayMin":    8,
	})
	if err != nil {
		t.Fatal(err)
	}

	settings := app.GuardianStatus().Settings
	if settings.TimeoutThreshold != 7 || settings.MonitorIntervalMS != 4321 ||
		settings.NetworkReconnectIntervalMS != 1300 || settings.NetworkRecoveryTimeoutMS != 18000 ||
		settings.OtherPlaceLoginIntervalMS != 4500 || settings.OtherPlaceLoginDelayMin != 8 {
		t.Fatalf("guardian status settings = %#v", settings)
	}
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test . -run '^TestGuardianStatusReturnsPersistedSettings$' -count=1`

Expected: compilation fails because `GuardianStatusDTO` has no `Settings` field.

- [ ] **Step 3: Add the minimal status contract**

Add the field:

```go
Settings guard.Settings `json:"settings"`
```

Build the snapshot once in `GuardianStatus`:

```go
settings := guardSettingsFromRuntimeSettings(a.currentRuntimeSettingsForChangeDetection())
```

Return it in both the nil-guardian and active-guardian response branches.

- [ ] **Step 4: Run the focused test and verify GREEN**

Run: `go test . -run '^TestGuardianStatusReturnsPersistedSettings$' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the backend contract**

```powershell
git add -- app.go app_test.go
git commit -m "fix: expose persisted guardian settings"
```

### Task 2: Render every guardian card from persisted settings

**Files:**
- Modify: `frontend/src/views/GuardView.tsx:4-31`
- Modify: `frontend/src/views/GuardView.tsx:169-207`
- Test: `frontend/src/views/GuardView.test.tsx`

- [ ] **Step 1: Write the failing frontend regression test**

Render a status with non-default settings, assert every input uses those values, then invoke each blur handler and assert the field-scoped payloads:

```tsx
it('renders and saves persisted numeric settings for all guardian cards', () => {
  const saves: Array<Record<string, unknown>> = [];
  let renderer: TestRenderer.ReactTestRenderer;
  act(() => {
    renderer = TestRenderer.create(
      <GuardView
        status={{
          enabled: true,
          phase: 'watching',
          restartCountInWindow: 0,
          maxRestartsPerWindow: 4,
          recentRestartEvents: [],
          settings: {
            timeoutThreshold: 7,
            monitorIntervalMs: 4321,
            networkReconnectIntervalMs: 1300,
            networkRecoveryTimeoutMs: 18000,
            otherPlaceLoginIntervalMs: 4500,
            otherPlaceLoginDelayMin: 8,
          },
        }}
        onRefresh={() => undefined}
        onLaunch={() => undefined}
        onRestart={() => undefined}
        onSaveSettings={(input) => saves.push(input)}
      />,
    );
  });

  const inputs = renderer!.root.findAllByType('input');
  expect(inputs.map((input) => input.props.defaultValue)).toEqual([7, 4321, 1300, 18000, 4500, 8]);

  const values = ['9', '5432', '1400', '19000', '4600', '10'];
  inputs.forEach((input, index) => input.props.onBlur({ currentTarget: { value: values[index] } }));
  expect(saves).toEqual([
    { timeoutThreshold: 9 },
    { monitorIntervalMs: 5432 },
    { networkReconnectIntervalMs: 1400 },
    { networkRecoveryTimeoutMs: 19000 },
    { otherPlaceLoginIntervalMs: 4600 },
    { otherPlaceLoginDelayMin: 10 },
  ]);
});
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `npm test -- --run src/views/GuardView.test.tsx`

Expected: FAIL because five fields still render hard-coded defaults and `GuardStatusDto` lacks `settings`.

- [ ] **Step 3: Add the settings DTO and replace hard-coded values**

Add the optional settings snapshot:

```ts
settings?: {
  timeoutThreshold?: number;
  monitorIntervalMs?: number;
  networkReconnectIntervalMs?: number;
  networkRecoveryTimeoutMs?: number;
  otherPlaceLoginIntervalMs?: number;
  otherPlaceLoginDelayMin?: number;
};
```

Use nullish fallbacks in each field definition, for example:

```ts
{ label: '检测间隔', key: 'monitorIntervalMs', value: status.settings?.monitorIntervalMs ?? 3000, unit: 'ms' }
```

Use `??`, not `||`, so the valid zero-minute other-place-login delay remains visible.

- [ ] **Step 4: Run the focused frontend test and verify GREEN**

Run: `npm test -- --run src/views/GuardView.test.tsx`

Expected: all `GuardView` tests PASS.

- [ ] **Step 5: Commit the frontend fix**

```powershell
git add -- frontend/src/views/GuardView.tsx frontend/src/views/GuardView.test.tsx
git commit -m "fix: restore guardian settings after refresh"
```

### Task 3: Verify integration and build

**Files:**
- Verify: `app.go`
- Verify: `app_test.go`
- Verify: `frontend/src/views/GuardView.tsx`
- Verify: `frontend/src/views/GuardView.test.tsx`

- [ ] **Step 1: Run backend affected tests**

Run: `go test . ./internal/storage ./internal/runtime/guard -count=1`

Expected: PASS with no failures.

- [ ] **Step 2: Run all frontend tests**

Run: `npm test`

Working directory: `frontend`

Expected: all test files PASS.

- [ ] **Step 3: Build the frontend**

Run: `npm run build`

Working directory: `frontend`

Expected: TypeScript compilation and Vite build exit successfully.

- [ ] **Step 4: Run all Go tests**

Run: `go test ./...`

Expected: all Go packages PASS.

- [ ] **Step 5: Inspect the final diff**

Run: `git diff HEAD~2 --check` and `git status --short`

Expected: no whitespace errors; only the planned implementation files and plan/design history are present.
