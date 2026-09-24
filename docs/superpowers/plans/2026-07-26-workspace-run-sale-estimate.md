# 工作台本次运行出售预估 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 工作台的出售预估收益只统计当前运行开始后发生的出售记录，账户状态页统计保持原样。

**Architecture:** `FarmWorkspaceRunStatistics` 继续是工作台唯一数据入口。为仓库出售记录增加一个按账号和精确发生时间汇总金额的存储查询，工作台用 `runStatisticsStartedAt` 调用它；账户状态页继续调用原有的按 `date_key` 聚合方法。

**Tech Stack:** Go、SQLite、Wails、Go testing、Vitest。

---

### Task 1: 按本次运行边界汇总工作台出售金额

**Files:**
- Modify: `app_test.go:4811`
- Modify: `internal/storage/warehouse.go:3-126`
- Modify: `app.go:1700-1720`
- Test: `app_test.go:4811`

- [ ] **Step 1: 写入失败的工作台回归测试**

将现有测试重命名为 `TestFarmWorkspaceRunStatisticsUsesCurrentSessionEventsAndCurrentRunSellRecords`，并在当前账号写入一条本次运行开始前、但仍属于当天的出售记录：

```go
startedAt := time.Now().Add(-2 * time.Minute)
app.runStatisticsStartedAt = startedAt
today := time.Now().Local().Format("2006-01-02")
if err := store.AppendWarehouseSellRecord(ctx, app.accountKey(), storage.WarehouseSellRecord{
    ID: "sale-before-run", DateKey: today,
    OccurredAt: startedAt.Add(-time.Second).Format(time.RFC3339Nano),
    TotalCount: 1, TotalAmount: 999,
}); err != nil {
    t.Fatalf("append pre-run sell record: %v", err)
}
```

保留两条运行开始后记录（`1200 + 80`）、昨天记录和其他账号记录，并断言 `stats.SaleEstimate == 1280`。

- [ ] **Step 2: 运行测试并确认它因现有当日汇总失败**

Run: `go test . -run '^TestFarmWorkspaceRunStatisticsUsesCurrentSessionEventsAndCurrentRunSellRecords$' -count=1`

Expected: FAIL，`SaleEstimate` 包含 `sale-before-run` 的 999，实际为 2279 而断言为 1280。

- [ ] **Step 3: 新增按开始时间汇总出售金额的存储查询**

在 `internal/storage/warehouse.go` 导入 `time`，新增：

```go
func (s *Store) SumWarehouseSellAmountSince(ctx context.Context, accountKey string, startedAt time.Time) (int64, error) {
    var total int64
    err := s.db.QueryRowContext(ctx, `
        SELECT COALESCE(SUM(total_amount), 0)
        FROM warehouse_sell_records
        WHERE account_key = ? AND occurred_at >= ?
    `, NormalizeAccountKey(accountKey), startedAt.Format(time.RFC3339Nano)).Scan(&total)
    return total, err
}
```

- [ ] **Step 4: 让工作台调用新查询，账户状态页保持不变**

将 `FarmWorkspaceRunStatistics` 的出售统计赋值替换为：

```go
stats.SaleEstimate, stats.EstimateReady = a.runWarehouseSellAmount(accountKey, a.runStatisticsStartedAt)
```

将旧的工作台私有帮助函数替换为：

```go
func (a *App) runWarehouseSellAmount(accountKey string, startedAt time.Time) (int64, bool) {
    if a.store == nil {
        return 0, true
    }
    total, err := a.store.SumWarehouseSellAmountSince(a.contextOrBackground(), accountKey, startedAt)
    if err != nil {
        a.lastErr = err
        return 0, false
    }
    return total, true
}
```

不改动 `automationStatsHistoryForAccount` 及其 `SumWarehouseSellAmountsByDate` 调用。

- [ ] **Step 5: 运行回归测试确认修复**

Run: `go test . -run '^TestFarmWorkspaceRunStatisticsUsesCurrentSessionEventsAndCurrentRunSellRecords$' -count=1`

Expected: PASS，工作台出售金额为 1280，运行前、昨天和其他账号记录均不计入。

- [ ] **Step 6: 运行相关后端和前端验证**

Run: `go test . -run '^(TestFarmWorkspaceRunStatisticsUsesCurrentSessionEventsAndCurrentRunSellRecords|TestAppAccountStatusAggregatesSuccessfulAutomationStats)$' -count=1`

Expected: PASS，工作台边界测试和账户状态相关测试均通过。

Run: `npm test -- --run src/views/OverviewView.test.tsx src/views/AccountStatusView.test.tsx`

Working directory: `frontend`

Expected: PASS，工作台展示契约与账户状态页统计展示均保持通过。

- [ ] **Step 7: 提交实现**

```bash
git add app.go app_test.go internal/storage/warehouse.go
git commit -m "fix: scope workspace sale estimate to current run"
```
