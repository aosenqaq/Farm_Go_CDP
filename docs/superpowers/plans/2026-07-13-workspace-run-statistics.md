# 工作台本次运行统计 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在工作台底部展示应用启动以来的自动化操作统计与当前仓库可售预估收益。

**Architecture:** `App` 在构造时记录会话起点，并从现有当前账户的成功自动化事件构建即时统计。统计接口借助现有仓库快照计算可售估值；前端把数据接入工作台的 A 方案单行卡片区域。

**Tech Stack:** Go、Wails、React 18、TypeScript、Vitest、SQLite 事件存储。

---

### Task 1: 运行统计数据模型

**Files:**
- Create: `internal/farm/run_statistics.go`
- Test: `internal/farm/run_statistics_test.go`

- [ ] **Step 1: 写入失败测试**

```go
func TestBuildRunStatisticsCountsSuccessfulAutomationTasks(t *testing.T) {
  started := time.Date(2026, 7, 13, 10, 0, 0, 0, time.Local)
  stats := BuildRunStatistics(started, started.Add(95*time.Second), []eventbus.Event{
    taskDone(started.Add(time.Second), "own_collect"), taskDone(started.Add(2*time.Second), "own_base"),
    taskDone(started.Add(3*time.Second), "friend_steal"), taskDone(started.Add(4*time.Second), "friend_help"),
    taskDone(started.Add(5*time.Second), "friend_mischief"), taskFailed(started.Add(6*time.Second), "own_collect"),
  })
  if stats.DurationSeconds != 95 || stats.Collect != 1 || stats.Farm != 1 || stats.Steal != 1 || stats.Help != 1 || stats.Mischief != 1 { t.Fatalf("stats=%#v", stats) }
}
```

- [ ] **Step 2: 验证测试失败**

Run: `go test ./internal/farm -run TestBuildRunStatisticsCountsSuccessfulAutomationTasks -count=1`

Expected: FAIL，因为 `BuildRunStatistics` 尚不存在。

- [ ] **Step 3: 实现最小模型和事件映射**

```go
type RunStatistics struct {
  StartedAt string `json:"startedAt"`; DurationSeconds int64 `json:"durationSeconds"`
  Collect int `json:"collect"`; Farm int `json:"farm"`; Steal int `json:"steal"`; Help int `json:"help"`; Mischief int `json:"mischief"`
  SaleEstimate int64 `json:"saleEstimate"`; EstimateReady bool `json:"estimateReady"`
}
```

`BuildRunStatistics` 仅接受 `source=auto_farm`、`type=task.done`、`data.ok=true` 且 `status=ok` 的事件，并将 `own_collect`、`own_base`、`friend_steal`、`friend_help`、`friend_mischief` 映射到五项计数。

- [ ] **Step 4: 验证测试通过**

Run: `go test ./internal/farm -run TestBuildRunStatisticsCountsSuccessfulAutomationTasks -count=1`

Expected: PASS。

### Task 2: App 会话接口和仓库估值

**Files:**
- Modify: `app.go:48-125`, `app.go:203-290`, `app.go:1566-1668`, `app.go:2372-2404`
- Test: `app_test.go`

- [ ] **Step 1: 写入失败测试**

```go
func TestFarmWorkspaceRunStatisticsUsesCurrentSessionEventsAndWarehouseEstimate(t *testing.T) {
  app, _ := newAppWithFakeRuntime(t, map[string]any{"gameCtl.refreshWarehouseSnapshot": warehouseFixture()})
  app.runStatisticsStartedAt = time.Now().Add(-2*time.Minute)
  app.recordEventForAccount(app.accountKey(), successfulAutomationEvent("own_collect"))
  app.recordEventForAccount(app.accountKey(), successfulAutomationEvent("friend_help"))
  stats := app.FarmWorkspaceRunStatistics()
  if stats.Collect != 1 || stats.Help != 1 || stats.DurationSeconds < 119 || !stats.EstimateReady || stats.SaleEstimate <= 0 { t.Fatalf("stats=%#v", stats) }
}
```

- [ ] **Step 2: 验证测试失败**

Run: `go test . -run TestFarmWorkspaceRunStatisticsUsesCurrentSessionEventsAndWarehouseEstimate -count=1`

Expected: FAIL，因为 App 方法不存在。

- [ ] **Step 3: 实现接口**

在 `App` 新增 `runStatisticsStartedAt time.Time` 并在 `NewApp` 赋 `time.Now()`。新增 `runStatisticsEvents(accountKey)`：优先调用 `ListRuntimeEventsForAccountSince(ctx, accountKey, runStatisticsStartedAt)`，存储不可用时拷贝同账户且不早于起点的内存事件。

```go
func (a *App) FarmWorkspaceRunStatistics() farm.RunStatistics {
  if a.requireAuthorized("FarmWorkspaceRunStatistics") != nil { return farm.RunStatistics{} }
  stats := farm.BuildRunStatistics(a.runStatisticsStartedAt, time.Now(), a.runStatisticsEvents(a.accountKey()))
  warehouse := a.FarmWarehouseRefresh()
  if warehouse.RuntimeError == "" { stats.SaleEstimate, stats.EstimateReady = warehouse.Summary.EstimatedAllSellPrice, true }
  return stats
}
```

保留 `automationStatsHistory` 与账户状态页逻辑不变。

- [ ] **Step 4: 验证接口测试**

Run: `go test . -run TestFarmWorkspaceRunStatisticsUsesCurrentSessionEventsAndWarehouseEstimate -count=1`

Expected: PASS。

### Task 3: Wails 接口和工作台渲染

**Files:**
- Modify: `frontend/wailsjs/go/main/App.js`
- Modify: `frontend/wailsjs/go/main/App.d.ts`
- Modify: `frontend/src/AuthorizedApp.tsx`
- Modify: `frontend/src/views/FarmWorkspaceView.tsx`
- Test: `frontend/src/views/FarmWorkspaceView.test.tsx`

- [ ] **Step 1: 写入失败渲染测试**

```tsx
expect(html).toContain('本次运行统计')
expect(html).toContain('00:18:42')
expect(html).toContain('收获次数')
expect(html).toContain('出售预估收益')
expect(html).toContain('1,280')
```

其中 `FarmWorkspaceView` 传入 `{ durationSeconds: 1122, collect: 8, farm: 5, steal: 1, help: 2, mischief: 3, saleEstimate: 1280, estimateReady: true }`。

- [ ] **Step 2: 验证测试失败**

Run: `npm test -- FarmWorkspaceView.test.tsx`

Expected: FAIL，因为 `runStatistics` prop 和统计区尚不存在。

- [ ] **Step 3: 接入绑定与数据状态**

添加 `FarmWorkspaceRunStatistics()` 的 JS 包装和 `Promise<farm.RunStatistics>` 类型。`AuthorizedApp` 仅在 `workspace` tab 激活时请求统计；计数沿用 2.5 秒状态刷新，仓库估值最多每 60 秒请求一次。使用账户 scope token 丢弃过期响应，并将状态作为 `runStatistics` prop 传给 `FarmWorkspaceView`。

- [ ] **Step 4: 按 A 方案渲染**

在最近事件和当前链路两栏之后添加：标题“本次运行统计”、标题右侧 `HH:MM:SS` 运行时间、六张 `metric-card`（收获次数、务农次数、偷菜次数、帮助次数、捣乱次数、出售预估收益）。估值未就绪显示 `-`。

- [ ] **Step 5: 验证前端测试**

Run: `npm test -- FarmWorkspaceView.test.tsx`

Expected: PASS。

### Task 4: 响应式样式与完整验证

**Files:**
- Modify: `frontend/src/style.css`
- Test: `frontend/src/views/OverviewView.test.tsx`

- [ ] **Step 1: 写入失败样式测试**

```tsx
expect(ruleDeclarations(css, '.workbench-run-statistics-grid')).toMatch(/repeat\(6,\s*minmax\(0,\s*1fr\)/)
expect(mediaDeclarations(css, 980)).toMatch(/workbench-run-statistics-grid[\s\S]*repeat\(3/)
```

- [ ] **Step 2: 验证测试失败**

Run: `npm test -- OverviewView.test.tsx`

Expected: FAIL，因为统计区样式不存在。

- [ ] **Step 3: 添加局部响应式规则**

`.workbench-run-statistics-grid` 使用六列 `minmax(0, 1fr)`；980px 以下变三列，680px 以下变两列。区域以现有 `#e2d7bf` 分隔线、`metric-card` 和 8px 以内圆角保持同一视觉系统。

- [ ] **Step 4: 运行完整验证**

Run: `go test ./...`

Expected: PASS。

Run: `npm test`

Expected: PASS。

Run: `npm run build`

Expected: exit 0。

- [ ] **Step 5: 提交实现**

```bash
git add app.go app_test.go internal/farm/run_statistics.go internal/farm/run_statistics_test.go frontend/src/AuthorizedApp.tsx frontend/src/views/FarmWorkspaceView.tsx frontend/src/views/FarmWorkspaceView.test.tsx frontend/src/views/OverviewView.test.tsx frontend/src/style.css frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts
git commit -m "feat: add workspace run statistics"
```
