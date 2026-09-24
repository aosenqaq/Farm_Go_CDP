# Restore Qian Xing Travel Reward Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore the Qian Xing Travel reward configuration and scheduler task with a default-off 120-minute interval, without calling the retired He Feng protocol.

**Architecture:** Replace the retired He Feng task only at the public catalog and scheduler boundaries with `qian_xing_travel_reward`. The new task has its own configuration keys and uses the existing generic not-migrated fallback until a Qian Xing protocol is supplied. The React reward settings reuse the existing scheduling-card implementation.

**Tech Stack:** Go, React 18, TypeScript, Vitest, Go testing

---

### Task 1: Define the Qian Xing catalog and scheduler contract

**Files:**
- Modify: `internal/farm/automation/catalog_test.go:63-87, 191-234`
- Modify: `internal/farm/automation/reward_schedule_test.go:8-35`
- Modify: `internal/farm/automation/runtime_rewards_test.go:354-364`
- Modify: `internal/farm/automation/config.go:66-83`
- Modify: `internal/farm/automation/catalog.go:124-196`
- Modify: `internal/farm/automation/reward_schedule.go:13-23`
- Modify: `internal/farm/automation/scheduler.go:541-549`

- [ ] **Step 1: Write the failing catalog and configuration regression tests**

  In `internal/farm/automation/catalog_test.go`, update the public scheduler count from `17` to `18`, then add this test after `TestStateDoesNotExposeRetiredSchedulerTasks`:

  ```go
  func TestDefaultStateExposesQianXingTravelReward(t *testing.T) {
      state := DefaultState()
      task := findSchedulerTaskForTest(state.Scheduler.Tasks, "qian_xing_travel_reward")
      if task == nil {
          t.Fatal("qian xing travel reward task not found")
      }
      if task.Label != "千星游记奖励领取" || task.Enabled || task.IntervalSec != 7200 {
          t.Fatalf("unexpected qian xing task: %#v", task)
      }
      if task.EnabledConfigKey != "autoFarmQianXingTravelRewardEnabled" || task.IntervalConfigKey != "autoFarmQianXingTravelRewardIntervalSec" {
          t.Fatalf("unexpected qian xing task config keys: %#v", task)
      }
      for key, want := range map[string]any{
          "autoFarmQianXingTravelRewardEnabled":      false,
          "autoFarmQianXingTravelRewardIntervalSec":  7200,
          "autoFarmQianXingTravelRewardIntervalMin":  120,
          "autoFarmQianXingTravelRewardScheduleMode": "interval",
          "autoFarmQianXingTravelRewardScheduleTime": "08:00",
      } {
          if got := state.Config[key]; got != want {
              t.Fatalf("config[%s] = %#v, want %#v", key, got, want)
          }
      }
  }
  ```

  In `internal/farm/automation/reward_schedule_test.go`, replace the `he_feng_travel_reward` expectation with:

  ```go
  "qian_xing_travel_reward": {"autoFarmQianXingTravelRewardScheduleMode", "autoFarmQianXingTravelRewardScheduleTime"},
  ```

  Keep the retired He Feng normalization test unchanged so the old enable switch remains forced off.

- [ ] **Step 2: Run the focused backend tests and verify they fail**

  Run:

  ```powershell
  go test ./internal/farm/automation -run 'TestStateContainsReferenceSchedulerTasksSortedByPriority|TestDefaultStateExposesQianXingTravelReward|TestRewardScheduleConfigKeys' -count=1
  ```

  Expected: FAIL because the Qian Xing ID and configuration keys do not exist and the public scheduler has 17 tasks.

- [ ] **Step 3: Replace the public task metadata with Qian Xing values**

  In `internal/farm/automation/config.go`, add these defaults after the mail-reward keys; remove the old He Feng default entries but leave its retired enable key in `retiredAutomationSwitches`:

  ```go
  "autoFarmQianXingTravelRewardEnabled":      false,
  "autoFarmQianXingTravelRewardIntervalSec":  7200,
  "autoFarmQianXingTravelRewardIntervalMin":  120,
  "autoFarmQianXingTravelRewardScheduleMode": "interval",
  "autoFarmQianXingTravelRewardScheduleTime": "08:00",
  ```

  In `internal/farm/automation/catalog.go`, replace every public `he_feng_travel_reward` mapping with `qian_xing_travel_reward` and its `autoFarmQianXingTravelReward...` keys. Replace the reward feature-group entry and task definition with:

  ```go
  {id: "qian_xing_travel_reward", label: "千星游记奖励领取", priority: 92, intervalSec: 7200, enabled: false},
  ```

  In `internal/farm/automation/reward_schedule.go`, replace the He Feng entry with:

  ```go
  "qian_xing_travel_reward": {modeKey: "autoFarmQianXingTravelRewardScheduleMode", timeKey: "autoFarmQianXingTravelRewardScheduleTime"},
  ```

  In `internal/farm/automation/scheduler.go`, replace the retired scheduler specification with:

  ```go
  {ID: "qian_xing_travel_reward", Label: "千星游记奖励领取", Lane: LaneProtocol, Resources: []Resource{ResourceProtocol, ResourceReward}},
  ```

  Do not add `qian_xing_travel_reward` to `rewardRuntimeTaskSpecs`: unknown task IDs already return `StatusNotMigrated` without calling the game runtime.

- [ ] **Step 4: Add and verify the no-protocol runtime safeguard**

  In `internal/farm/automation/runtime_rewards_test.go`, add:

  ```go
  func TestRuntimeFacadeQianXingTravelRewardDoesNotCallRetiredProtocol(t *testing.T) {
      caller := &fakeRuntimeCaller{}
      result := NewRuntimeFacade(caller).RunTask(context.Background(), "qian_xing_travel_reward")

      if result.OK || result.Status != StatusNotMigrated {
          t.Fatalf("qian deng result = %#v, want not migrated", result)
      }
      if len(caller.calls) != 0 {
          t.Fatalf("runtime calls = %#v, want none", caller.calls)
      }
  }
  ```

  Run:

  ```powershell
  gofmt -w internal/farm/automation/config.go internal/farm/automation/catalog.go internal/farm/automation/reward_schedule.go internal/farm/automation/scheduler.go internal/farm/automation/catalog_test.go internal/farm/automation/reward_schedule_test.go internal/farm/automation/runtime_rewards_test.go
  go test ./internal/farm/automation -run 'TestStateContainsReferenceSchedulerTasksSortedByPriority|TestDefaultStateExposesQianXingTravelReward|TestStateDoesNotExposeRetiredSchedulerTasks|TestRewardScheduleConfigKeys|TestRuntimeFacadeQianXingTravelRewardDoesNotCallRetiredProtocol' -count=1
  ```

  Expected: PASS. The new task is public and configured, old He Feng remains hidden, and the new ID cannot invoke the retired protocol.

- [ ] **Step 5: Commit the backend catalog change**

  ```powershell
  git add -- internal/farm/automation/config.go internal/farm/automation/catalog.go internal/farm/automation/reward_schedule.go internal/farm/automation/scheduler.go internal/farm/automation/catalog_test.go internal/farm/automation/reward_schedule_test.go internal/farm/automation/runtime_rewards_test.go
  git commit -m "feat: restore qian deng reward scheduler"
  ```

### Task 2: Restore the reward settings card and task labels

**Files:**
- Modify: `frontend/src/views/AutomationView.test.tsx:1135-1163`
- Modify: `frontend/src/views/AutomationView.tsx:140-188, 1575-1584`
- Modify: `frontend/src/views/FarmWorkspaceView.tsx:91-99`
- Modify: `frontend/src/lib/events.test.ts:29-36`
- Modify: `frontend/src/lib/events.ts:45-58`
- Modify: `export_logs_test.go`
- Modify: `export_logs.go:166-190`

- [ ] **Step 1: Write the failing frontend and log-label tests**

  In the `renders editable rewards and events detailed settings` test, replace the negative He Feng assertions with:

  ```tsx
  expect(html).toContain('千星游记奖励领取');
  expect(html).toContain('name="config-autoFarmQianXingTravelRewardEnabled"');
  expect(html).toContain('name="config-autoFarmQianXingTravelRewardIntervalMin"');
  expect(html).toContain('name="config-autoFarmQianXingTravelRewardIntervalSec"');
  expect(html.indexOf('邮件奖励')).toBeLessThan(html.indexOf('千星游记奖励领取'));
  expect(html).not.toContain('荷风游记奖励');
  ```

  In the existing `maps task ids to Chinese labels` test in `frontend/src/lib/events.test.ts`, add:

  ```tsx
  expect(automationTaskLabel('qian_xing_travel_reward')).toBe('千星游记奖励领取');
  ```

  Add this Go test to `export_logs_test.go`:

  ```go
  func TestExportAutomationTaskLabelUsesQianXingTravelRewardName(t *testing.T) {
      if got := exportAutomationTaskLabel("qian_xing_travel_reward"); got != "千星游记奖励领取" {
          t.Fatalf("task label = %q, want qian xing label", got)
      }
  }
  ```

- [ ] **Step 2: Run the focused tests and verify they fail**

  Run:

  ```powershell
  npm test -- src/views/AutomationView.test.tsx src/lib/events.test.ts
  go test . -run TestExportAutomationTaskLabelUsesQianXingTravelRewardName -count=1
  ```

  Working directory for the first command: `frontend`.

  Expected: both commands fail because the reward card and labels have not yet been registered for the new ID.

- [ ] **Step 3: Add the Qian Xing settings card and labels**

  In `frontend/src/views/AutomationView.tsx`, replace the three He Feng entries in `schedulerTaskConfigKeyById`, `schedulerTaskIntervalConfigKeyById`, and `schedulerTaskScheduleModeConfigKeyById` with `qian_xing_travel_reward` and the corresponding `autoFarmQianXingTravelReward...` keys. Add this entry immediately after the mail reward in `rewardItems`:

  ```tsx
  ['千星游记奖励领取', 'qian_xing_travel_reward', 7200],
  ```

  In `frontend/src/views/FarmWorkspaceView.tsx`, replace the mocked He Feng task with:

  ```tsx
  { id: 'qian_xing_travel_reward', label: '千星游记奖励领取', priority: 92, intervalSec: 7200, enabled: false },
  ```

  Add the Qian Xing label while retaining historical He Feng labels in both task-label maps:

  ```ts
  qian_xing_travel_reward: '千星游记奖励领取',
  ```

  ```go
  "qian_xing_travel_reward": "千星游记奖励领取",
  ```

- [ ] **Step 4: Run the focused tests until they pass**

  Run:

  ```powershell
  npm test -- src/views/AutomationView.test.tsx src/lib/events.test.ts
  gofmt -w export_logs.go export_logs_test.go
  go test . -run TestExportAutomationTaskLabelUsesQianXingTravelRewardName -count=1
  ```

  Expected: PASS. The setting appears after mail reward and all surfaces use the intended label.

- [ ] **Step 5: Commit the frontend and label changes**

  ```powershell
  git add -- frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx frontend/src/views/FarmWorkspaceView.tsx frontend/src/lib/events.ts frontend/src/lib/events.test.ts export_logs.go export_logs_test.go
  git commit -m "feat: expose qian deng reward settings"
  ```

### Task 3: Verify the restored task end to end

**Files:**
- Verify: `internal/farm/automation/catalog.go`
- Verify: `frontend/src/views/AutomationView.tsx`

- [ ] **Step 1: Run Go formatting and the full Go test suite**

  Run:

  ```powershell
  gofmt -w internal/farm/automation/config.go internal/farm/automation/catalog.go internal/farm/automation/reward_schedule.go internal/farm/automation/scheduler.go internal/farm/automation/catalog_test.go internal/farm/automation/reward_schedule_test.go internal/farm/automation/runtime_rewards_test.go export_logs.go export_logs_test.go
  go test ./...
  ```

  Expected: exit code 0 with no failing Go package.

- [ ] **Step 2: Run frontend regression and production build**

  Run from `frontend`:

  ```powershell
  npm test
  npm run build
  ```

  Expected: Vitest exits 0 and TypeScript/Vite production build completes without errors.

- [ ] **Step 3: Inspect the final change set**

  Run:

  ```powershell
  git diff --check
  git status --short
  git log -2 --oneline
  ```

  Expected: no whitespace errors; only the expected feature commits are present; working tree is clean.
