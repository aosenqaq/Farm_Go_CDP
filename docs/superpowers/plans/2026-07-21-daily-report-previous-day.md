# Previous-Day Daily Report Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make manual and scheduled daily reports render and aggregate the complete previous Beijing calendar day.

**Architecture:** `internal/messagepush` owns the default report date and allows its live provider to return the exact aggregated date. `App.dailyReportForAccount` uses that date to select historical automation events and sales, while the scheduler continues to deduplicate by the current delivery date.

**Tech Stack:** Go, SQLite-backed storage, `testing`, `net/http/httptest`.

---

## File Structure

- `internal/messagepush/service.go`: Daily report DTO and rendered daily context.
- `internal/messagepush/messagepush_test.go`: Manual daily-send rendering regression.
- `app.go`: Previous-day activity/sale aggregation for the live provider.
- `app_test.go`: Storage-backed application regression for date isolation.
- `internal/messagepush/templates.go`: Built-in daily card wording.
- `internal/messagepush/template_test.go`: Default template wording regression.

### Task 1: Render The Report Date From The Live Daily Provider

**Files:**
- Modify: `internal/messagepush/service.go:20-30,157-170,275-307`
- Test: `internal/messagepush/messagepush_test.go:200-250`

- [x] **Step 1: Write the failing message-service regression test**

  Extend `TestSendDailyNowUsesLiveReportProviderValues` so its fixed `Now` remains
  `2026-07-12 09:00`, the webhook JSON template includes
  `"date":"{{daily.date}}"`, and the final assertion includes
  `body["date"] == "2026-07-11"`.

  ```go
  return DailyReport{
      Summary: "真实运行数据",
      Values: map[string]any{
          "name": "Dpo.L", "gold": 3891552777, "bean": 455521,
          "warehouseEstimate": 115998840, "sellAmount": 10336, "sellCount": 24,
      },
  }, nil

  // Full template content:
  `{"date":"{{daily.date}}","name":"{{daily.name}}","gold":{{daily.gold}},"bean":{{daily.bean}},"warehouseEstimate":{{daily.warehouseEstimate}},"sellAmount":{{daily.sellAmount}},"sellCount":{{daily.sellCount}},"summary":"{{daily.summary}}"}`

  if body["date"] != "2026-07-11" || body["name"] != "Dpo.L" || body["summary"] != "真实运行数据" {
      t.Fatalf("daily body=%#v", body)
  }
  ```

- [x] **Step 2: Run the message-service test and verify RED**

  Run: `go test ./internal/messagepush -run '^TestSendDailyNowUsesLiveReportProviderValues$' -count=1`

  Expected: FAIL because the current context renders `daily.date` as `2026-07-12`.

- [x] **Step 3: Implement the smallest date propagation change**

  Add `DateKey string` to `DailyReport`. Default `daily.date` and `SendDailyNow`'s
  `Payload.Meta["dateKey"]` to `ShiftDateKey(now, -1)`, then use a nonblank live
  provider date to replace the default.

  ```go
  type DailyReport struct {
      AccountGID string
      DateKey    string
      Summary    string
      Values     map[string]any
  }

  reportDateKey := ShiftDateKey(now, -1)
  dailyValues := map[string]any{
      "date": reportDateKey, "summary": summary, "name": "Farm_Go", "level": 0,
      "gold": 0, "bean": 0, "warehouseEstimate": 0, "warehouseSellableCount": 0,
      "sellCount": 0, "sellAmount": 0, "runs": 0, "collect": 0, "water": 0,
      "steal": 0, "help": 0, "mischiefGrass": 0, "mischiefBug": 0,
  }
  // After the provider succeeds:
  if strings.TrimSpace(report.DateKey) != "" {
      dailyValues["date"] = report.DateKey
  }
  ```

  Leave `RunDueDaily`, `NextDailyRunAt`, and `LastDailySummaryDateKey` unchanged so
  the scheduler remains idempotent by delivery day.

- [x] **Step 4: Run the message-service test and verify GREEN**

  Run: `go test ./internal/messagepush -run '^TestSendDailyNowUsesLiveReportProviderValues$' -count=1`

  Expected: PASS; the manual send emits `date: "2026-07-11"`.

- [x] **Step 5: Commit the message-service change**

  ```powershell
  git add internal/messagepush/service.go internal/messagepush/messagepush_test.go
  git commit -m "fix: render daily reports for the previous day"
  ```

### Task 2: Aggregate Only Previous-Day Account Activity And Sales

**Files:**
- Modify: `app.go:3738-3798`
- Test: `app_test.go` beside `TestAppDailyPushUsesLiveAccountWarehouseAndSaleData`

- [x] **Step 1: Write the failing application-level regression test**

  Add `TestAppDailyReportUsesPreviousDayActivityAndSales`. Create the app with fake
  `gameCtl.getPlayerProfile` and `gameCtl.refreshWarehouseSnapshot` responses, then
  add one previous-day and one delivery-day sale record plus distinct successful
  `auto_farm/task.done` events. Call `dailyReportForAccount` with a fixed
  `2026-07-21 09:00` value and assert only the July 20 values are present.

  ```go
  reportAt := time.Date(2026, 7, 21, 9, 0, 0, 0, time.Local)
  previousDay := "2026-07-20"
  currentDay := "2026-07-21"

  previousEvent := eventbus.Event{
      Timestamp: time.Date(2026, 7, 20, 12, 0, 0, 0, time.Local),
      Source: "auto_farm", Type: "task.done",
      Data: map[string]any{"taskId": "own_collect", "ok": true, "status": "ok"},
  }
  currentEvent := eventbus.Event{
      Timestamp: time.Date(2026, 7, 21, 8, 0, 0, 0, time.Local),
      Source: "auto_farm", Type: "task.done",
      Data: map[string]any{"taskId": "friend_help", "ok": true, "status": "ok"},
  }
  if err := store.AppendRuntimeEventForAccount(ctx, "gid:10001", previousEvent); err != nil { t.Fatal(err) }
  if err := store.AppendRuntimeEventForAccount(ctx, "gid:10001", currentEvent); err != nil { t.Fatal(err) }
  if err := store.AppendWarehouseSellRecord(ctx, "gid:10001", storage.WarehouseSellRecord{
      ID: "previous-sale", DateKey: previousDay, OccurredAt: previousEvent.Timestamp.Format(time.RFC3339Nano), TotalCount: 3, TotalAmount: 120,
  }); err != nil { t.Fatal(err) }
  if err := store.AppendWarehouseSellRecord(ctx, "gid:10001", storage.WarehouseSellRecord{
      ID: "current-sale", DateKey: currentDay, OccurredAt: currentEvent.Timestamp.Format(time.RFC3339Nano), TotalCount: 9, TotalAmount: 999,
  }); err != nil { t.Fatal(err) }

  report, err := app.dailyReportForAccount(ctx, "gid:10001", "10001", reportAt)
  if err != nil { t.Fatal(err) }
  if report.DateKey != previousDay || report.Values["collect"] != 1 || report.Values["help"] != 0 || report.Values["sellCount"] != 3 || report.Values["sellAmount"] != 120 {
      t.Fatalf("report=%#v", report)
  }
  ```

- [x] **Step 2: Run the application test and verify RED**

  Run: `go test . -run '^TestAppDailyReportUsesPreviousDayActivityAndSales$' -count=1`

  Expected: FAIL because the current report selects July 21 activity/sales and has no
  returned report date.

- [x] **Step 3: Implement date-isolated aggregation**

  Derive the report key once, fetch a two-day history window, select only the matching
  day, and query warehouse sales using that same key. Start activity with an empty
  `farm.DayStats` for the report date; do not use `profile.TodayStats` as fallback.

  ```go
  reportDateKey := messagepush.ShiftDateKey(now, -1)
  stats := farm.DayStats{DateKey: reportDateKey}
  if history := a.automationStatsHistoryForAccount(ctx, accountKey, now, 2); len(history.Days) > 0 {
      for _, day := range history.Days {
          if day.DateKey == reportDateKey {
              stats = day
              break
          }
      }
  }

  records, err := a.store.ListWarehouseSellRecords(ctx, accountKey, reportDateKey, 500)
  if err != nil {
      return messagepush.DailyReport{}, fmt.Errorf("读取日报出售记录失败: %w", err)
  }
  for _, record := range records {
      sellCount += record.TotalCount
      sellAmount += record.TotalAmount
  }

  summary := fmt.Sprintf("金币 %d，金豆 %d；%s 出售 %d 件，获得 %d 金币；仓库可售估值 %d 金币。", profile.Gold, profile.Bean, reportDateKey, sellCount, sellAmount, warehouse.Summary.EstimatedAllSellPrice)
  return messagepush.DailyReport{AccountGID: actualGID, DateKey: reportDateKey, Summary: summary, Values: values}, nil
  ```

- [x] **Step 4: Run the application test and verify GREEN**

  Run: `go test . -run '^TestAppDailyReportUsesPreviousDayActivityAndSales$' -count=1`

  Expected: PASS; the report uses July 20's one collection and sale total of 3/120,
  excluding July 21's help event and sale total of 9/999.

- [x] **Step 5: Commit the aggregation change**

  ```powershell
  git add app.go app_test.go
  git commit -m "fix: aggregate daily reports from the previous day"
  ```

### Task 3: Correct Built-In Historical Report Wording And Verify The Suite

**Files:**
- Modify: `internal/messagepush/templates.go:150-180`
- Modify: `internal/messagepush/template_test.go:225-250`

- [x] **Step 1: Write the failing default-template wording test**

  Add this package-local test to `template_test.go`.

  ```go
  func TestDefaultDailyTemplateUsesReportPeriodLabels(t *testing.T) {
      template := defaultDailyTemplate("feishu", TemplateModeCard)
      if !strings.Contains(template.Content, "**当日运行** {{daily.runs}}") {
          t.Fatalf("daily card should label historical activity by report period: %s", template.Content)
      }
      if strings.Contains(template.Content, "**今日运行**") {
          t.Fatalf("daily card still refers to historical activity as today: %s", template.Content)
      }
  }
  ```

- [x] **Step 2: Run the template test and verify RED**

  Run: `go test ./internal/messagepush -run '^TestDefaultDailyTemplateUsesReportPeriodLabels$' -count=1`

  Expected: FAIL because the Feishu card contains `**今日运行**`.

- [x] **Step 3: Replace the incorrect built-in label**

  In the Feishu card returned by `defaultDailyTemplate`, replace only
  `**今日运行** {{daily.runs}}` with `**当日运行** {{daily.runs}}`. Do not alter user
  templates or unrelated channels.

- [x] **Step 4: Run focused and complete verification**

  Run:

  ```powershell
  gofmt -w app.go app_test.go internal/messagepush/service.go internal/messagepush/messagepush_test.go internal/messagepush/templates.go internal/messagepush/template_test.go
  go test . -run '^TestAppDailyReportUsesPreviousDayActivityAndSales$' -count=1
  go test ./internal/messagepush -count=1
  go test ./...
  ```

  Expected: all commands exit 0. The existing daily scheduler idempotency test remains
  green, proving it still sends only once per delivery day.

- [x] **Step 5: Commit the wording and verification changes**

  ```powershell
  git add internal/messagepush/templates.go internal/messagepush/template_test.go
  git commit -m "fix: label daily report activity by report date"
  ```
