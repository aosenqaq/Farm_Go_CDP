# Land Rush Dialog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace redundant land detail actions with a full one-click ripening dialog copied from the reference project behavior.

**Architecture:** Keep the frontend dialog inside `AssetsLandView.tsx` to match the existing single-file assets page pattern. Add one Wails app method, `FarmLandRush`, that validates the dialog payload and bridges it to the existing injected `gameCtl.fertilizeLandsBatch` runtime API. Refresh land details after successful submission.

**Tech Stack:** Go/Wails backend, React/TypeScript frontend, Vitest render tests, Go unit tests.

---

### Task 1: Backend Action Surface

**Files:**
- Modify: `internal/farm/gameconfig.go`
- Test: `internal/farm/gameconfig_test.go`

- [ ] **Step 1: Write the failing test**

Add a test that expects land runtime actions to expose only `rush` labeled `一键催熟` and the migration gate to omit `refresh`, `upgrade`, and `shovel`.

- [ ] **Step 2: Implement the minimal action change**

Change `LandDetailsGate` and `BuildRuntimeLandDetailsForRoot` so both return:

```go
Actions: []RuntimeActionGate{
    {ID: "rush", Label: "一键催熟", Enabled: false, Reason: reason},
}
```

for the gate and:

```go
Actions: []RuntimeActionGate{
    {ID: "rush", Label: "一键催熟", Enabled: true},
}
```

for runtime payloads.

- [ ] **Step 3: Run backend test**

Run: `go test ./internal/farm`

Expected: the new action assertions pass.

### Task 2: Runtime Bridge

**Files:**
- Modify: `app.go`
- Modify: `app_test.go`
- Generated: `frontend/wailsjs/go/main/App.d.ts`
- Generated: `frontend/wailsjs/go/main/App.js`

- [ ] **Step 1: Write the failing Go test**

Add a test for a helper that builds the `gameCtl.fertilizeLandsBatch` argument from:

```go
map[string]any{
    "landIds": []any{1, 2},
    "fertilizerMode": "normal",
    "harvestLinkEnabled": true,
    "rushThresholdSec": 300,
}
```

Expected argument:

```go
map[string]any{
    "landIds": []int{1, 2},
    "type": "normal",
    "mode": "normal",
    "dryRun": false,
    "cleanupUi": true,
    "linkedHarvestAfterFertilize": true,
    "rushThresholdSec": 300,
}
```

- [ ] **Step 2: Implement validation helper**

Add a small helper in `app.go` that:

- accepts `map[string]any`
- normalizes positive unique land IDs from `landIds`, `landIdList`, or `landId`
- maps `fertilizerMode`, `mode`, or `type` to `normal` or `organic`
- rejects empty land IDs and unsupported fertilizer modes
- maps `harvestLinkEnabled` to `linkedHarvestAfterFertilize`

- [ ] **Step 3: Add Wails method**

Add:

```go
func (a *App) FarmLandRush(input map[string]any) map[string]any
```

It calls:

```go
a.supervisor.Call(a.contextOrBackground(), "gameCtl.fertilizeLandsBatch", []any{args}, 45*time.Second)
```

and returns the runtime result as a map. On error, return `{"ok": false, "error": err.Error()}`.

- [ ] **Step 4: Update Wails bindings**

Run: `wails generate module`

Expected: `frontend/wailsjs/go/main/App.d.ts` and `frontend/wailsjs/go/main/App.js` include `FarmLandRush`.

- [ ] **Step 5: Run backend tests**

Run: `go test ./...`

Expected: all Go tests pass.

### Task 3: Frontend Dialog

**Files:**
- Modify: `frontend/src/views/AssetsLandView.tsx`
- Modify: `frontend/src/views/AssetsLandView.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write failing render tests**

Add tests that assert:

- land gate renders `一键催熟`
- top-level `刷新土地`, `升级土地`, and top-level `铲除` do not render in the action gate
- opening the dialog renders `自动筛选`, `手动选择`, `成熟阈值(秒)`, `催熟肥料`, `无机`, `有机`, `催熟后自动收获`, and `催熟并收获`
- manual mode can render eligible rows such as `#1 白萝卜 · 5分钟后成熟`

- [ ] **Step 2: Add frontend state and submit bridge**

Import `FarmLandRush`. Track:

```ts
const [landRushOpen, setLandRushOpen] = useState(false);
const [landRushBusy, setLandRushBusy] = useState(false);
```

Implement `submitLandRush(payload)` to call `FarmLandRush(payload)`, close the dialog on success, and call `refreshLandDetails(false)`.

- [ ] **Step 3: Replace land action gate handling**

Pass `onAction` so `rush` opens the dialog. Do not wire `refresh`, `upgrade`, or `shovel` in the top-level action list.

- [ ] **Step 4: Add `LandRushDialog`**

Implement the reference behavior:

- default mode `auto`
- default threshold `300`
- default selected fertilizer `organic`
- default `harvestLinkEnabled` true
- automatic eligible land IDs from the current threshold
- manual mode with row checkboxes and all-select
- disabled submit when busy, disabled, or no selected lands

- [ ] **Step 5: Add focused CSS**

Add classes for the right drawer, segmented controls, threshold shortcuts, checkbox rows, and sticky footer while matching existing asset-page styling.

- [ ] **Step 6: Run frontend tests**

Run: `npm test -- AssetsLandView.test.tsx`

Expected: all `AssetsLandView` tests pass.

### Task 4: Final Verification

**Files:**
- Verify only

- [ ] **Step 1: Run full frontend build**

Run: `npm run build`

Expected: TypeScript and Vite build pass.

- [ ] **Step 2: Run focused Go tests**

Run: `go test ./internal/farm`

Expected: pass.

- [ ] **Step 3: Inspect diff**

Run: `git diff -- app.go internal/farm/gameconfig.go frontend/src/views/AssetsLandView.tsx frontend/src/views/AssetsLandView.test.tsx frontend/src/style.css frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/main/App.js`

Expected: diff only includes the land rush dialog, runtime bridge, action cleanup, and tests.

## Self-Review

- Spec coverage: all requested removals, dialog controls, payload fields, runtime bridge, and verification are covered.
- Placeholder scan: no placeholder implementation steps remain.
- Type consistency: the payload field names match the frontend, backend helper, and reference project names.
