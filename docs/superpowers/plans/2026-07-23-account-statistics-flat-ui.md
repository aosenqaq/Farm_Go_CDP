# Account Statistics Flat UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add date-range sale estimate aggregation to account statistics and replace the eight dialog cards with compact colored icon rows.

**Architecture:** Aggregate warehouse sale amounts in one account/date-range SQLite query, merge totals into a complete daily history, and let the existing frontend date window sum only fully ready estimates. Render the dialog through an isolated `AccountStatistic` component so the rest of the account page keeps its existing cards.

**Tech Stack:** Go, SQLite, React 18, TypeScript, Lucide React, CSS Grid, Vitest, Vite

---

## File Map

- `internal/storage/warehouse.go` and `warehouse_test.go`: grouped daily sale totals.
- `internal/farm/account_status.go`, `app.go`, and `app_test.go`: daily estimate DTO and history merge.
- `frontend/src/views/AccountStatusView.tsx` and its test: range aggregation and flat statistic markup.
- `frontend/src/style.css`: four-column desktop and two-column mobile styles.
- `frontend/dist/*`: ignored generated WebUI embedded by `main.go`.

### Task 1: Group Sale Amounts By Date

**Files:**
- Modify: `internal/storage/warehouse_test.go`
- Modify: `internal/storage/warehouse.go`

- [ ] **Step 1: Write a failing storage test**

Add a test that appends two `gid:10001` records totaling `200` on `2026-07-21`, one totaling `300` on `2026-07-22`, one outside the requested range, and one for `gid:10002`. Assert:

```go
totals, err := store.SumWarehouseSellAmountsByDate(ctx, "gid:10001", "2026-07-20", "2026-07-22")
if err != nil {
	t.Fatalf("sum sale amounts: %v", err)
}
if len(totals) != 2 || totals["2026-07-21"] != 200 || totals["2026-07-22"] != 300 {
	t.Fatalf("unexpected daily totals: %#v", totals)
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/storage -run TestWarehouseSellAmountsByDateAreScopedAndSummed -count=1`

Expected: build FAIL because the method does not exist.

- [ ] **Step 3: Implement the structured grouped query**

Add:

```go
func (s *Store) SumWarehouseSellAmountsByDate(ctx context.Context, accountKey string, startDateKey string, endDateKey string) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT date_key, COALESCE(SUM(total_amount), 0)
		FROM warehouse_sell_records
		WHERE account_key = ? AND date_key >= ? AND date_key <= ?
		GROUP BY date_key
		ORDER BY date_key ASC
	`, NormalizeAccountKey(accountKey), strings.TrimSpace(startDateKey), strings.TrimSpace(endDateKey))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	totals := map[string]int64{}
	for rows.Next() {
		var dateKey string
		var amount int64
		if err := rows.Scan(&dateKey, &amount); err != nil {
			return nil, err
		}
		totals[dateKey] = amount
	}
	return totals, rows.Err()
}
```

- [ ] **Step 4: Verify GREEN and commit**

Run: `go test ./internal/storage -count=1`

Commit:

```powershell
git add -- internal/storage/warehouse.go internal/storage/warehouse_test.go
git commit -m "feat: aggregate warehouse sales by date"
```

### Task 2: Add Estimates To Daily Account History

**Files:**
- Modify: `internal/farm/account_status.go`
- Modify: `app_test.go`
- Modify: `app.go`

- [ ] **Step 1: Write a failing history test**

Create a store, append `500` for `2026-07-23` and `300` for `2026-07-22`, then call:

```go
now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.Local)
history := app.automationStatsHistoryForAccount(ctx, app.accountKey(), now, 3)
if len(history.Days) != 3 {
	t.Fatalf("expected one entry per calendar day, got %#v", history.Days)
}
want := map[string]int64{"2026-07-21": 0, "2026-07-22": 300, "2026-07-23": 500}
for _, day := range history.Days {
	if !day.EstimateReady || day.SaleEstimate != want[day.DateKey] {
		t.Fatalf("unexpected estimate for %s: %#v", day.DateKey, day)
	}
}
```

- [ ] **Step 2: Verify RED**

Run: `go test . -run TestAutomationStatsHistoryIncludesDailySaleEstimates -count=1`

Expected: build FAIL because `DayStats` lacks estimate fields.

- [ ] **Step 3: Extend the DTO and runtime parser**

Add to `DayStats`:

```go
SaleEstimate int64 `json:"saleEstimate"`
EstimateReady bool  `json:"estimateReady"`
```

Add matching `int64FromMap` and `boolFromMap` assignments in `buildDayStats`.

- [ ] **Step 4: Merge totals while filling every calendar date**

In `automationStatsHistoryForAccount`, query once from `start` through `todayKey`. Record query errors in `a.lastErr`. Replace the sparse day loop with:

```go
days := make([]farm.DayStats, 0, windowDays)
for day := start; !day.After(localDateStart(now)); day = day.AddDate(0, 0, 1) {
	dateKey := day.Format("2006-01-02")
	stats := byDate[dateKey]
	stats.DateKey = dateKey
	stats.SaleEstimate = saleAmounts[dateKey]
	stats.EstimateReady = saleErr == nil
	days = append(days, stats)
}
```

Remove the today-only insertion. Update `todayWarehouseSellAmount` to use the same grouped query with identical start/end keys.

- [ ] **Step 5: Verify GREEN and commit**

Run:

```powershell
go test . -run 'TestAutomationStatsHistoryIncludesDailySaleEstimates|TestAppAccountStatusAggregatesSuccessfulAutomationStats' -count=1
go test ./internal/farm ./internal/storage -count=1
```

Commit:

```powershell
git add -- internal/farm/account_status.go app.go app_test.go
git commit -m "feat: include sale estimates in account history"
```

### Task 3: Aggregate The Selected Frontend Range

**Files:**
- Modify: `frontend/src/views/AccountStatusView.test.tsx`
- Modify: `frontend/src/views/AccountStatusView.tsx`

- [ ] **Step 1: Write a failing range test**

Export and import `buildStatsWindow`. Use three complete dates with estimates `100`, `300`, and `500`, then assert:

```tsx
expect(buildStatsWindow(profile, 'today').stats.saleEstimate).toBe(500);
const threeDays = buildStatsWindow(profile, '3d');
expect(threeDays.stats.saleEstimate).toBe(900);
expect(threeDays.stats.estimateReady).toBe(true);
profile.statsHistory.days[1].estimateReady = false;
expect(buildStatsWindow(profile, '3d').stats.estimateReady).toBe(false);
```

- [ ] **Step 2: Verify RED**

Run: `npm test -- --run src/views/AccountStatusView.test.tsx` from `frontend`.

Expected: build FAIL because the function is private and estimate fields do not exist.

- [ ] **Step 3: Extend frontend daily statistics**

Add optional `saleEstimate` and `estimateReady` fields. Add `saleEstimate` to `statKeys`; normalize it with `positiveNumber` and normalize readiness with `source.estimateReady === true`.

Export `buildStatsWindow`. In the multi-day branch:

```tsx
const stats = sumDays(selected);
stats.estimateReady = selected.length === windowDays && selected.every((day) => day.estimateReady);
return { stats, coveredDays: selected.length, windowDays, rangeLabel: `${startKey} ~ ${todayKey}` };
```

The today branch retains its normalized daily readiness.

- [ ] **Step 4: Verify GREEN and commit**

Run: `npm test -- --run src/views/AccountStatusView.test.tsx` from `frontend`.

Commit:

```powershell
git add -- frontend/src/views/AccountStatusView.tsx frontend/src/views/AccountStatusView.test.tsx
git commit -m "feat: aggregate account sale estimates by range"
```

### Task 4: Render Eight Flat Dialog Statistics

**Files:**
- Modify: `frontend/src/views/AccountStatusView.test.tsx`
- Modify: `frontend/src/views/AccountStatusView.tsx`

- [ ] **Step 1: Write a failing component test**

Open the dialog using the existing renderer pattern. Assert eight exact `account-statistic` nodes, the icon classes `activity`, `sprout`, `tractor`, `shopping-basket`, `hand-heart`, `bomb`, `coins`, and `chart-no-axes-combined`, the text `预估收益`, absence of `覆盖天数`, and exactly six underlying page `metric-card` nodes.

- [ ] **Step 2: Verify RED**

Run: `npm test -- --run src/views/AccountStatusView.test.tsx` from `frontend`.

Expected: FAIL because the dialog still adds eight generic cards.

- [ ] **Step 3: Add the dedicated component**

Import the required Lucide icons and `LucideIcon`, then add:

```tsx
function AccountStatistic({ icon: Icon, tone, label, value, title }: {
  icon: LucideIcon;
  tone: 'runs' | 'collect' | 'farm' | 'steal' | 'help' | 'mischief' | 'sale' | 'estimate';
  label: string;
  value: string;
  title?: string;
}) {
  return (
    <article className="account-statistic">
      <Icon className={`account-statistic-icon account-statistic-icon-${tone}`} size={20} strokeWidth={1.8} aria-hidden="true" />
      <span className="account-statistic-label" title={label}>{label}</span>
      <strong className="account-statistic-value" title={title}>{value}</strong>
    </article>
  );
}
```

- [ ] **Step 4: Replace only the eight dialog metrics**

Map existing values to `Activity`, `Sprout`, `Tractor`, `ShoppingBasket`, `HandHeart`, `Bomb`, `Coins`, and `ChartNoAxesCombined`. The eighth item is:

```tsx
<AccountStatistic
  icon={ChartNoAxesCombined}
  tone="estimate"
  label="预估收益"
  value={statsWindow.stats.estimateReady ? formatNumber(statsWindow.stats.saleEstimate) : '-'}
  title={statsWindow.stats.estimateReady ? formatNumber(statsWindow.stats.saleEstimate) : undefined}
/>
```

- [ ] **Step 5: Verify GREEN and commit**

Run: `npm test -- --run src/views/AccountStatusView.test.tsx` from `frontend`.

Commit:

```powershell
git add -- frontend/src/views/AccountStatusView.tsx frontend/src/views/AccountStatusView.test.tsx
git commit -m "feat: render flat account statistics"
```

### Task 5: Apply Responsive Flat Styling

**Files:**
- Modify: `frontend/src/views/AccountStatusView.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write a failing CSS contract test**

Reuse the CSS parser helpers from `OverviewView.test.tsx`. Assert desktop four columns, mobile two columns, a three-column item layout, zero border, no shadow, and right-aligned values.

- [ ] **Step 2: Verify RED**

Run: `npm test -- --run src/views/AccountStatusView.test.tsx` from `frontend`.

- [ ] **Step 3: Add isolated flat styles**

Set `.account-stats-grid` to four equal columns with an 8px gap. Define `.account-statistic` with `auto minmax(0, 1fr) auto`, 48px minimum height, `9px 11px` padding, zero border, 6px radius, `#f3f4f1` background, and no shadow. Add truncating 13px labels, truncating right-aligned 17px bold values, and eight color tone classes matching the workbench palette.

Remove old dialog `.metric-card` overrides and the redundant 980px grid rule.

- [ ] **Step 4: Add compact mobile dimensions**

Keep the existing 640px two-column grid and add 44px item minimum height, 8px padding, 6px gap, 12px labels, and 16px values.

- [ ] **Step 5: Verify GREEN and commit**

Run: `npm test -- --run src/views/AccountStatusView.test.tsx` from `frontend`.

Commit:

```powershell
git add -- frontend/src/style.css frontend/src/views/AccountStatusView.test.tsx
git commit -m "style: flatten account statistics dialog"
```

### Task 6: Full Verification And Build

- [ ] **Step 1: Run all Go tests**

Run: `go test ./...`

- [ ] **Step 2: Run all frontend tests**

Run: `npm test` from `frontend`.

- [ ] **Step 3: Build the embedded WebUI**

Run: `npm run build` from `frontend`.

Expected: TypeScript and Vite exit `0`; `frontend/dist/index.html` references new hashed assets.

- [ ] **Step 4: Verify served source and repository state**

```powershell
$css = (Invoke-WebRequest -UseBasicParsing 'http://127.0.0.1:5174/src/style.css' -TimeoutSec 5).Content
$view = (Invoke-WebRequest -UseBasicParsing 'http://127.0.0.1:5174/src/views/AccountStatusView.tsx' -TimeoutSec 5).Content
"CSS_ACCOUNT_STATISTIC=$($css -match '\.account-statistic\s*\{')"
"VIEW_ESTIMATE=$($view -match '预估收益')"
git diff --check
git status --short
```

Expected: served checks are true, no whitespace errors exist, and only the pre-existing `resources/gameConfig.bundle.zip` remains modified.
