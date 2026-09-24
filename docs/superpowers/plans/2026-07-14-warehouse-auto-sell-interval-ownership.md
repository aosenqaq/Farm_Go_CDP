# Warehouse Auto-Sell Interval Ownership Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent generic automation saves from resetting a warehouse-selected auto-sell interval to 60 minutes.

**Architecture:** `WarehouseAutoSellSettings` remains the sole persisted source for warehouse auto-sell configuration. Generic automation saves load and overlay that configuration into the scheduler state, rather than deriving warehouse values from a potentially stale scheduler payload.

**Tech Stack:** Go, SQLite-backed `internal/storage.Store`, Go standard-library tests.

---

### Task 1: Preserve Warehouse Settings During Automation Saves

**Files:**
- Modify: `app_test.go:674-713`
- Modify: `app.go:1059-1139`

- [ ] **Step 1: Write the failing regression test**

Replace the existing test that expects `SaveFarmAutomationState` to persist an edited warehouse task with a test that saves warehouse settings at 120 minutes, submits an automation state with a stale 3600-second warehouse task, and expects the stored settings and returned scheduler state to remain at 7200 seconds.

```go
if _, err := app.SaveWarehouseAutoSellSettings(storage.WarehouseAutoSellSettings{
    Enabled: true, IntervalMinute: 120, Categories: []string{"seed"}, RefreshOnlyOnAutoSell: true,
}); err != nil {
    t.Fatalf("seed warehouse settings: %v", err)
}
state := app.FarmAutomationState()
state.Config["autoWarehouseSellIntervalSec"] = 60 * 60
for index := range state.Scheduler.Tasks {
    if state.Scheduler.Tasks[index].ID == "auto_warehouse_sell" {
        state.Scheduler.Tasks[index].IntervalSec = 60 * 60
    }
}
saved, err := app.SaveFarmAutomationState(state)
if err != nil {
    t.Fatalf("save automation state: %v", err)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test . -run TestSaveFarmAutomationStateDoesNotOverwriteWarehouseAutoSellSettings -count=1`

Expected: FAIL because the stored warehouse setting is overwritten to 60 minutes.

- [ ] **Step 3: Write the minimal implementation**

In `SaveFarmAutomationState`, remove the conversion from submitted automation settings to `WarehouseAutoSellSettings` and its warehouse save. Load the existing account-scoped warehouse settings, then overlay them with `applyWarehouseAutoSellToAutomationSettings` before saving the generic automation settings and configuring the scheduler.

```go
warehouse = loaded
settings = applyWarehouseAutoSellToAutomationSettings(settings, warehouse)
if err := a.store.SaveAutoFarmSettingsForAccount(a.contextOrBackground(), accountKey, storageAutoFarmSettings(settings)); err != nil {
    // existing error handling
}
```

- [ ] **Step 4: Run the regression test to verify it passes**

Run: `go test . -run TestSaveFarmAutomationStateDoesNotOverwriteWarehouseAutoSellSettings -count=1`

Expected: PASS.

- [ ] **Step 5: Run affected package tests**

Run: `go test . -count=1`

Expected: PASS with no test failures.

- [ ] **Step 6: Commit the implementation**

```bash
git add app.go app_test.go
git commit -m "fix: preserve warehouse auto-sell interval"
```
