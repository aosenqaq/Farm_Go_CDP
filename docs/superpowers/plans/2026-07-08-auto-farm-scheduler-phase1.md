# Auto Farm Scheduler Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first Go-native Farm_Go migration slice for `自动农场` and `调度中心`: feature groups, normalized defaults, scheduler snapshot, Wails methods, and the approved A-style React UI.

**Architecture:** Keep Farm_Go native. Go owns the automation catalog and scheduler/config model under `internal/farm/automation`; root `App` exposes Wails methods; React renders the A-style automation surface and scheduler dialog. Runtime execution buttons return structured `not_migrated` in this phase.

**Tech Stack:** Go 1.25, Wails v2, React, TypeScript, Vitest, lucide-react, existing CSS.

---

## Scope Check

This plan intentionally does not port live game execution from `auto-farm-executor.js`. It creates the stable data contract and UI shell for later script-by-script runtime migration.

## File Structure

- Create `internal/farm/automation/catalog.go`: automation feature groups, scheduler task definitions, default config, snapshot builder, and action result model.
- Create `internal/farm/automation/catalog_test.go`: tests for seven feature groups, 18 scheduler tasks, priority ordering, config defaults, and `not_migrated` manual run result.
- Modify `app.go`: add Wails methods `FarmAutomationState()` and `RunFarmAutomationTask(taskID string)`.
- Modify `app_test.go`: assert the root app exposes the automation state and returns `not_migrated` for manual task execution.
- Create `frontend/src/views/AutomationView.tsx`: approved A-style automation page, feature cards, scheduler dialog, settings dialog shell, and status summary.
- Create `frontend/src/views/AutomationView.test.tsx`: render tests for feature cards, scheduler button, task rows, and manual callback.
- Modify `frontend/src/views/FarmWorkspaceView.tsx`: render `AutomationView` for `area === 'automation'`.
- Modify `frontend/src/style.css`: add A-style automation page, cards, switches, scheduler modal, and compact settings styles.

### Task 1: Backend Automation Catalog

**Files:**
- Create: `internal/farm/automation/catalog_test.go`
- Create: `internal/farm/automation/catalog.go`

- [ ] **Step 1: Write the failing Go tests**

Create `internal/farm/automation/catalog_test.go`:

```go
package automation

import "testing"

func TestStateContainsApprovedFeatureGroups(t *testing.T) {
	state := DefaultState()
	want := []string{"own_base", "planting", "fertilizer", "friends", "rewards", "mystery_shop", "runtime"}
	if len(state.FeatureGroups) != len(want) {
		t.Fatalf("feature group count = %d, want %d", len(state.FeatureGroups), len(want))
	}
	for i, id := range want {
		if state.FeatureGroups[i].ID != id {
			t.Fatalf("feature group[%d] = %q, want %q", i, state.FeatureGroups[i].ID, id)
		}
	}
}

func TestStateContainsReferenceSchedulerTasksSortedByPriority(t *testing.T) {
	state := DefaultState()
	if len(state.Scheduler.Tasks) != 18 {
		t.Fatalf("scheduler task count = %d, want 18", len(state.Scheduler.Tasks))
	}
	if state.Scheduler.Tasks[0].ID != "own_base" || state.Scheduler.Tasks[0].Priority != 100 {
		t.Fatalf("first task = %#v, want own_base priority 100", state.Scheduler.Tasks[0])
	}
	if state.Scheduler.Tasks[len(state.Scheduler.Tasks)-1].ID != "friend_mischief" {
		t.Fatalf("last task = %#v, want friend_mischief", state.Scheduler.Tasks[len(state.Scheduler.Tasks)-1])
	}
}

func TestDefaultConfigMatchesReferenceIntervals(t *testing.T) {
	state := DefaultState()
	got := map[string]int{}
	for _, task := range state.Scheduler.Tasks {
		got[task.ID] = task.IntervalSec
	}
	checks := map[string]int{
		"own_base":              60,
		"own_collect":           30,
		"own_plant":             10,
		"own_fertilizer":        30,
		"friend_steal":          90,
		"reward_claim":          3600,
		"mystery_shop_auto_buy": 43200,
	}
	for id, want := range checks {
		if got[id] != want {
			t.Fatalf("%s interval = %d, want %d", id, got[id], want)
		}
	}
}

func TestRunTaskReturnsNotMigrated(t *testing.T) {
	result := RunTask("friend_steal")
	if result.OK {
		t.Fatalf("RunTask returned OK for phase 1: %#v", result)
	}
	if result.Status != StatusNotMigrated {
		t.Fatalf("status = %q, want %q", result.Status, StatusNotMigrated)
	}
	if result.TaskID != "friend_steal" {
		t.Fatalf("taskID = %q, want friend_steal", result.TaskID)
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run:

```powershell
go test ./internal/farm/automation
```

Expected: package or symbol failures because `internal/farm/automation` does not exist yet.

- [ ] **Step 3: Implement the catalog**

Create `internal/farm/automation/catalog.go` with:

```go
package automation

import "sort"

type ActionStatus string

const (
	StatusOK           ActionStatus = "ok"
	StatusNotMigrated  ActionStatus = "not_migrated"
	StatusRuntimeReady ActionStatus = "runtime_not_ready"
	StatusFailed       ActionStatus = "failed"
)

type ActionResult struct {
	OK      bool         `json:"ok"`
	Status  ActionStatus `json:"status"`
	TaskID  string       `json:"taskId,omitempty"`
	Message string       `json:"message"`
}

type FeatureGroup struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Summary     string   `json:"summary"`
	Enabled     bool     `json:"enabled"`
	SettingKeys []string `json:"settingKeys"`
}

type SchedulerTask struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Priority     int    `json:"priority"`
	IntervalSec  int    `json:"intervalSec"`
	Enabled      bool   `json:"enabled"`
	LastStartedAt string `json:"lastStartedAt,omitempty"`
	LastFinishedAt string `json:"lastFinishedAt,omitempty"`
	LastSuccessAt  string `json:"lastSuccessAt,omitempty"`
	LastError      string `json:"lastError,omitempty"`
	LastResultSummary string `json:"lastResultSummary,omitempty"`
}

type SchedulerState struct {
	Enabled       bool            `json:"enabled"`
	MinGapMs      int             `json:"minGapMs"`
	RunningTaskID string          `json:"runningTaskId,omitempty"`
	Tasks         []SchedulerTask `json:"tasks"`
}

type State struct {
	Running       bool            `json:"running"`
	Summary       map[string]int  `json:"summary"`
	FeatureGroups []FeatureGroup  `json:"featureGroups"`
	Scheduler     SchedulerState  `json:"scheduler"`
}

type taskDef struct {
	id          string
	label       string
	priority    int
	intervalSec int
	enabled     bool
}

var featureGroups = []FeatureGroup{
	{ID: "own_base", Label: "自家基础任务", Summary: "一键务农、自动收获、除草、浇水、杀虫。", Enabled: true, SettingKeys: []string{"autoFarmOneClickEnabled", "autoFarmOwnCollectEnabled"}},
	{ID: "planting", Label: "自动种植策略", Summary: "主/副策略、指定种子、背包优先级、随机延迟。", Enabled: true, SettingKeys: []string{"autoFarmPlantPrimaryMode", "autoFarmPlantSecondaryMode"}},
	{ID: "fertilizer", Label: "肥料与催熟", Summary: "自动施肥、填充肥料、催熟联动、自动购买。", Enabled: false, SettingKeys: []string{"autoFarmFertilizerEnabled", "autoFarmFertilizerFillEnabled"}},
	{ID: "friends", Label: "好友自动化", Summary: "偷菜、帮忙、捣乱、冷却、黑白名单。", Enabled: false, SettingKeys: []string{"autoFarmFriendEnabled", "autoFarmFriendHelpEnabled"}},
	{ID: "rewards", Label: "奖励与活动", Summary: "任务奖励、礼包、月卡、邮件、抽奖。", Enabled: true, SettingKeys: []string{"autoRewardClaimEnabled", "autoFarmSvipDailyGiftEnabled"}},
	{ID: "mystery_shop", Label: "神秘商店自动购买", Summary: "目标种子、货币类型和折扣阈值。", Enabled: false, SettingKeys: []string{"autoFarmMysteryShopAutoBuyEnabled"}},
	{ID: "runtime", Label: "运行节奏与附属联动", Summary: "自动启动、等待时间、RPC 超时、仓库联动。", Enabled: true, SettingKeys: []string{"autoFarmAutoStartEnabled", "autoFarmRpcTimeoutMs"}},
}

var schedulerTaskDefs = []taskDef{
	{id: "own_base", label: "一键务农", priority: 100, intervalSec: 60, enabled: true},
	{id: "land_upgrade", label: "土地自动升级", priority: 99, intervalSec: 43200, enabled: true},
	{id: "reward_claim", label: "自动领取任务奖励", priority: 98, intervalSec: 3600, enabled: true},
	{id: "svip_daily_gift", label: "SVIP每日礼包", priority: 97, intervalSec: 43200, enabled: true},
	{id: "monthly_card_reward", label: "月卡奖励", priority: 96, intervalSec: 43200, enabled: true},
	{id: "mall_daily_fertilizer", label: "商城每日肥料", priority: 95, intervalSec: 43200, enabled: true},
	{id: "share_reward", label: "自动领取分享奖励", priority: 94, intervalSec: 43200, enabled: true},
	{id: "mail_reward", label: "自动领取邮件奖励", priority: 93, intervalSec: 43200, enabled: true},
	{id: "he_feng_travel_reward", label: "限时活动/荷风游记奖励领取", priority: 92, intervalSec: 43200, enabled: false},
	{id: "limited_seed_draw", label: "限时活动/荷风游记抽奖", priority: 92, intervalSec: 43200, enabled: false},
	{id: "mystery_shop_auto_buy", label: "神秘商店自动购买", priority: 91, intervalSec: 43200, enabled: false},
	{id: "own_collect", label: "自动收获", priority: 91, intervalSec: 30, enabled: true},
	{id: "own_plant", label: "自动种植", priority: 90, intervalSec: 10, enabled: true},
	{id: "fertilizer_fill", label: "自动填充化肥", priority: 86, intervalSec: 43200, enabled: false},
	{id: "own_fertilizer", label: "自动施肥", priority: 85, intervalSec: 30, enabled: false},
	{id: "friend_steal", label: "好友偷菜", priority: 70, intervalSec: 90, enabled: false},
	{id: "friend_help", label: "好友帮忙", priority: 65, intervalSec: 90, enabled: false},
	{id: "friend_mischief", label: "好友捣乱", priority: 60, intervalSec: 90, enabled: false},
}

func DefaultState() State {
	groups := make([]FeatureGroup, len(featureGroups))
	copy(groups, featureGroups)
	tasks := make([]SchedulerTask, 0, len(schedulerTaskDefs))
	for _, def := range schedulerTaskDefs {
		tasks = append(tasks, SchedulerTask{
			ID: def.id, Label: def.label, Priority: def.priority,
			IntervalSec: def.intervalSec, Enabled: def.enabled,
		})
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Priority != tasks[j].Priority {
			return tasks[i].Priority > tasks[j].Priority
		}
		return i < j
	})
	enabled := 0
	for _, task := range tasks {
		if task.Enabled {
			enabled++
		}
	}
	return State{
		Running: false,
		Summary: map[string]int{"enabledTasks": enabled, "totalTasks": len(tasks), "todayHarvest": 0},
		FeatureGroups: groups,
		Scheduler: SchedulerState{Enabled: true, MinGapMs: 350, Tasks: tasks},
	}
}

func RunTask(taskID string) ActionResult {
	if taskID == "" {
		return ActionResult{OK: false, Status: StatusFailed, Message: "任务 ID 不能为空"}
	}
	return ActionResult{
		OK: false,
		Status: StatusNotMigrated,
		TaskID: taskID,
		Message: "该自动化任务的真实运行时脚本尚未迁移，本阶段只提供配置和调度 UI。",
	}
}
```

- [ ] **Step 4: Run the test and verify it passes**

Run:

```powershell
go test ./internal/farm/automation
```

Expected: tests pass.

- [ ] **Step 5: Commit**

Run:

```powershell
git add internal/farm/automation/catalog.go internal/farm/automation/catalog_test.go
git commit -m "feat: add farm automation scheduler model"
```

### Task 2: Wails App Methods

**Files:**
- Modify: `app.go`
- Modify: `app_test.go`

- [ ] **Step 1: Write the failing root app test**

Append to `app_test.go`:

```go
func TestFarmAutomationStateIsExposed(t *testing.T) {
	app := NewApp()
	state := app.FarmAutomationState()
	if len(state.FeatureGroups) != 7 {
		t.Fatalf("feature group count = %d, want 7", len(state.FeatureGroups))
	}
	if len(state.Scheduler.Tasks) != 18 {
		t.Fatalf("scheduler task count = %d, want 18", len(state.Scheduler.Tasks))
	}
}

func TestRunFarmAutomationTaskReturnsNotMigrated(t *testing.T) {
	app := NewApp()
	result := app.RunFarmAutomationTask("friend_steal")
	if result.OK {
		t.Fatalf("expected not migrated result, got %#v", result)
	}
	if result.Status != "not_migrated" {
		t.Fatalf("status = %q, want not_migrated", result.Status)
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run:

```powershell
go test ./...
```

Expected: `App` has no `FarmAutomationState` or `RunFarmAutomationTask`.

- [ ] **Step 3: Implement the Wails methods**

Add this import to `app.go`:

```go
	"Farm_Go/internal/farm/automation"
```

Add these methods near other Wails-facing app methods:

```go
func (a *App) FarmAutomationState() automation.State {
	return automation.DefaultState()
}

func (a *App) RunFarmAutomationTask(taskID string) automation.ActionResult {
	return automation.RunTask(taskID)
}
```

- [ ] **Step 4: Run Go tests**

Run:

```powershell
go test ./...
```

Expected: all Go tests pass.

- [ ] **Step 5: Commit**

Run:

```powershell
git add app.go app_test.go
git commit -m "feat: expose farm automation state"
```

### Task 3: Automation React View

**Files:**
- Create: `frontend/src/views/AutomationView.test.tsx`
- Create: `frontend/src/views/AutomationView.tsx`
- Modify: `frontend/src/views/FarmWorkspaceView.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write the failing frontend tests**

Create `frontend/src/views/AutomationView.test.tsx`:

```tsx
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { AutomationView, type FarmAutomationState } from './AutomationView';

const state: FarmAutomationState = {
  running: false,
  summary: { enabledTasks: 8, totalTasks: 18, todayHarvest: 0 },
  featureGroups: [
    { id: 'own_base', label: '自家基础任务', summary: '一键务农、自动收获、除草、浇水、杀虫。', enabled: true, settingKeys: [] },
    { id: 'planting', label: '自动种植策略', summary: '主/副策略、指定种子、背包优先级、随机延迟。', enabled: true, settingKeys: [] },
    { id: 'fertilizer', label: '肥料与催熟', summary: '自动施肥、填充肥料、催熟联动、自动购买。', enabled: false, settingKeys: [] },
    { id: 'friends', label: '好友自动化', summary: '偷菜、帮忙、捣乱、冷却、黑白名单。', enabled: false, settingKeys: [] },
    { id: 'rewards', label: '奖励与活动', summary: '任务奖励、礼包、月卡、邮件、抽奖。', enabled: true, settingKeys: [] },
    { id: 'mystery_shop', label: '神秘商店自动购买', summary: '目标种子、货币类型和折扣阈值。', enabled: false, settingKeys: [] },
    { id: 'runtime', label: '运行节奏与附属联动', summary: '自动启动、等待时间、RPC 超时、仓库联动。', enabled: true, settingKeys: [] },
  ],
  scheduler: {
    enabled: true,
    minGapMs: 350,
    tasks: [
      { id: 'own_base', label: '一键务农', priority: 100, intervalSec: 60, enabled: true },
      { id: 'friend_steal', label: '好友偷菜', priority: 70, intervalSec: 90, enabled: false },
    ],
  },
};

describe('AutomationView', () => {
  it('renders approved feature groups and scheduler entry', () => {
    const html = renderToStaticMarkup(<AutomationView state={state} onRunTask={() => undefined} />);
    expect(html).toContain('农场自动化');
    expect(html).toContain('自家基础任务');
    expect(html).toContain('自动种植策略');
    expect(html).toContain('肥料与催熟');
    expect(html).toContain('好友自动化');
    expect(html).toContain('奖励与活动');
    expect(html).toContain('神秘商店自动购买');
    expect(html).toContain('运行节奏与附属联动');
    expect(html).toContain('调度中心');
  });

  it('renders scheduler task rows in the dialog surface', () => {
    const html = renderToStaticMarkup(<AutomationView state={state} onRunTask={() => undefined} initialSchedulerOpen />);
    expect(html).toContain('一键务农');
    expect(html).toContain('好友偷菜');
    expect(html).toContain('100');
    expect(html).toContain('90s');
  });
});
```

- [ ] **Step 2: Run the test and verify it fails**

Run:

```powershell
cd frontend
npm test -- AutomationView.test.tsx
```

Expected: missing `AutomationView` module.

- [ ] **Step 3: Implement `AutomationView.tsx`**

Create `frontend/src/views/AutomationView.tsx`:

```tsx
import { Bot, CalendarClock, Play, Power, Settings } from 'lucide-react';
import { useState } from 'react';

export type FarmAutomationFeatureGroup = {
  id: string;
  label: string;
  summary: string;
  enabled: boolean;
  settingKeys: string[];
};

export type FarmAutomationSchedulerTask = {
  id: string;
  label: string;
  priority: number;
  intervalSec: number;
  enabled: boolean;
  lastStartedAt?: string;
  lastFinishedAt?: string;
  lastSuccessAt?: string;
  lastError?: string;
  lastResultSummary?: string;
};

export type FarmAutomationState = {
  running: boolean;
  summary: Record<string, number>;
  featureGroups: FarmAutomationFeatureGroup[];
  scheduler: {
    enabled: boolean;
    minGapMs: number;
    runningTaskId?: string;
    tasks: FarmAutomationSchedulerTask[];
  };
};

type AutomationViewProps = {
  state: FarmAutomationState;
  onRunTask: (taskId: string) => void;
  initialSchedulerOpen?: boolean;
};

function formatInterval(sec: number) {
  if (sec >= 3600 && sec % 3600 === 0) return `${sec / 3600}h`;
  if (sec >= 60 && sec % 60 === 0) return `${sec / 60}m`;
  return `${sec}s`;
}

export function AutomationView({ state, onRunTask, initialSchedulerOpen = false }: AutomationViewProps) {
  const [schedulerOpen, setSchedulerOpen] = useState(initialSchedulerOpen);
  const [settingsGroup, setSettingsGroup] = useState<FarmAutomationFeatureGroup | null>(null);
  const enabledTasks = state.summary.enabledTasks ?? state.scheduler.tasks.filter((task) => task.enabled).length;
  const totalTasks = state.summary.totalTasks ?? state.scheduler.tasks.length;

  return (
    <section className="view-stack fill automation-view">
      <header className="page-header automation-header">
        <div>
          <h1>农场自动化</h1>
          <p>总开关控制大功能，齿轮进入设置；调度中心管理优先级和手动执行。</p>
        </div>
        <div className="header-actions">
          <button className="secondary-button automation-wide-action" type="button">应用推荐配置</button>
          <button className="secondary-button automation-wide-action dark" type="button" onClick={() => setSchedulerOpen(true)}>
            <CalendarClock size={16} />
            调度中心
          </button>
          <button className="primary-button" type="button">
            <Power size={16} />
            {state.running ? '停止' : '启动'}
          </button>
        </div>
      </header>

      <div className="automation-summary-strip">
        <div><span>状态</span><strong>{state.running ? '运行中' : '已停止'}</strong></div>
        <div><span>启用任务</span><strong>{enabledTasks} / {totalTasks}</strong></div>
        <div><span>下次执行</span><strong>{state.scheduler.tasks.find((task) => task.enabled)?.label || '-'}</strong></div>
        <div><span>本轮</span><strong>{state.scheduler.runningTaskId || '等待中'}</strong></div>
        <div><span>今日统计</span><strong>收获 {state.summary.todayHarvest ?? 0}</strong></div>
      </div>

      <div className="automation-feature-grid">
        {state.featureGroups.map((group) => (
          <article className="automation-feature-card" key={group.id}>
            <div className="automation-feature-icon"><Bot size={18} /></div>
            <div className="automation-feature-copy">
              <strong>{group.label}</strong>
              <p>{group.summary}</p>
            </div>
            <button className={group.enabled ? 'automation-switch on' : 'automation-switch'} type="button">
              {group.enabled ? 'ON' : 'OFF'}
            </button>
            <button className="icon-button light" type="button" aria-label={`${group.label}设置`} onClick={() => setSettingsGroup(group)}>
              <Settings size={16} />
            </button>
          </article>
        ))}
      </div>

      {schedulerOpen ? (
        <div className="dialog-backdrop automation-dialog-backdrop" role="presentation">
          <section className="automation-scheduler-dialog" role="dialog" aria-label="调度中心">
            <header>
              <div>
                <h2>调度中心</h2>
                <p>任务按优先级执行；本阶段手动执行会返回未迁移状态。</p>
              </div>
              <button className="icon-button light" type="button" onClick={() => setSchedulerOpen(false)}>×</button>
            </header>
            <div className="automation-scheduler-controls">
              <label>
                最小间隔(ms)
                <input value={state.scheduler.minGapMs} readOnly />
              </label>
              <span className={state.scheduler.enabled ? 'migration-state migration-state-done' : 'migration-state'}>
                {state.scheduler.enabled ? '已启用' : '已关闭'}
              </span>
            </div>
            <div className="automation-task-table">
              <div className="automation-task-row head">
                <span>任务</span><span>优先级</span><span>间隔</span><span>状态</span><span>执行</span>
              </div>
              {state.scheduler.tasks.map((task) => (
                <div className="automation-task-row" key={task.id}>
                  <strong>{task.label}</strong>
                  <span>{task.priority}</span>
                  <span>{formatInterval(task.intervalSec)}</span>
                  <span>{task.enabled ? '启用' : '关闭'}</span>
                  <button type="button" onClick={() => onRunTask(task.id)}>
                    <Play size={13} />
                    跑
                  </button>
                </div>
              ))}
            </div>
          </section>
        </div>
      ) : null}

      {settingsGroup ? (
        <div className="dialog-backdrop automation-dialog-backdrop" role="presentation">
          <section className="automation-settings-dialog" role="dialog" aria-label={`${settingsGroup.label}设置`}>
            <header>
              <div>
                <h2>{settingsGroup.label}</h2>
                <p>{settingsGroup.summary}</p>
              </div>
              <button className="icon-button light" type="button" onClick={() => setSettingsGroup(null)}>×</button>
            </header>
            <p className="automation-settings-note">详细脚本设置会按功能切片迁移；当前先固定入口和配置边界。</p>
          </section>
        </div>
      ) : null}
    </section>
  );
}
```

- [ ] **Step 4: Wire `FarmWorkspaceView`**

Modify `frontend/src/views/FarmWorkspaceView.tsx` to import and render:

```tsx
import { AutomationView, type FarmAutomationState } from './AutomationView';
```

Add a local fallback state for phase 1 if Wails data has not been loaded:

```tsx
const automationState: FarmAutomationState = {
  running: false,
  summary: { enabledTasks: 8, totalTasks: 18, todayHarvest: 0 },
  featureGroups: [...],
  scheduler: { enabled: true, minGapMs: 350, tasks: [...] },
};
```

Then replace the automation placeholder branch with:

```tsx
if (area === 'automation') {
  return <AutomationView state={automationState} onRunTask={() => undefined} />;
}
```

- [ ] **Step 5: Add CSS**

Append focused classes to `frontend/src/style.css`: `.automation-view`, `.automation-summary-strip`, `.automation-feature-grid`, `.automation-feature-card`, `.automation-switch`, `.automation-scheduler-dialog`, `.automation-task-table`, `.automation-task-row`, `.automation-settings-dialog`.

- [ ] **Step 6: Run frontend tests**

Run:

```powershell
cd frontend
npm test -- AutomationView.test.tsx FarmWorkspaceView.test.tsx
```

Expected: tests pass.

- [ ] **Step 7: Commit**

Run:

```powershell
git add frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx frontend/src/views/FarmWorkspaceView.tsx frontend/src/style.css
git commit -m "feat: add farm automation control surface"
```

### Task 4: Full Verification

**Files:**
- No source edits expected unless tests reveal issues.

- [ ] **Step 1: Run Go tests**

Run:

```powershell
go test ./...
```

Expected: all Go tests pass.

- [ ] **Step 2: Run frontend tests**

Run:

```powershell
cd frontend
npm test
```

Expected: all frontend tests pass.

- [ ] **Step 3: Run frontend build**

Run:

```powershell
cd frontend
npm run build
```

Expected: TypeScript and Vite build complete successfully.

## Self-Review

- Spec coverage: Phase 1 covers the approved A-style UI, seven feature groups, 18 scheduler tasks, top-right scheduler dialog, and `not_migrated` runtime boundary.
- Placeholder scan: The plan uses explicit file paths, test code, implementation code, and commands. Later runtime execution is explicitly out of scope for this plan rather than a placeholder.
- Type consistency: Go JSON names match frontend TypeScript names: `featureGroups`, `scheduler.tasks`, `taskId`, `intervalSec`, `minGapMs`, and `not_migrated`.
