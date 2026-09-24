# Workbench Flat Run Statistics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the six oversized workbench statistic cards with a shared flat icon-and-value layout and prevent recent events from overlapping statistics on short LAN mobile screens.

**Architecture:** Keep the existing `WorkspaceRunStatistics` data contract and all formatters inside `OverviewView`. Give the overview its own four-row layout class, render each metric through a dedicated flat statistic component, and use the existing remote-shell breakpoint for the mobile two-column variant.

**Tech Stack:** React 18, TypeScript, Lucide React, CSS Grid, Vitest, Vite

---

## File Map

- Modify `frontend/src/views/OverviewView.tsx`: add the overview layout hook, six Lucide icon mappings, and flat statistic markup.
- Modify `frontend/src/views/OverviewView.test.tsx`: cover all six metrics, icons, flat markup, four-row flow, and responsive column counts.
- Modify `frontend/src/style.css`: define compact flat statistic styling and explicit desktop/mobile overview rows.
- Generate `frontend/dist/*`: rebuild the embedded desktop and LAN WebUI assets through Vite.

### Task 1: Flat Statistic Semantics

**Files:**
- Modify: `frontend/src/views/OverviewView.test.tsx`
- Modify: `frontend/src/views/OverviewView.tsx`

- [ ] **Step 1: Write the failing flat-statistic rendering test**

Add this test inside `describe('OverviewView', ...)`:

```tsx
it('renders the six run statistics as dedicated flat icon rows', () => {
  const html = renderToStaticMarkup(
    <OverviewView
      status={readyStatus}
      licenseStatus={authorizedLicense}
      events={[]}
      onRefresh={() => undefined}
      runStatistics={{
        durationSeconds: 61,
        collect: 47,
        farm: 93,
        steal: 5,
        help: 8,
        mischief: 2,
        saleEstimate: 78_255,
        estimateReady: true,
      }}
    />,
  );

  expect(html.match(/class="workbench-statistic"/g)).toHaveLength(6);
  expect(html).toContain('lucide-sprout');
  expect(html).toContain('lucide-tractor');
  expect(html).toContain('lucide-shopping-basket');
  expect(html).toContain('lucide-hand-heart');
  expect(html).toContain('lucide-bomb');
  expect(html).toContain('lucide-coins');
  expect(html).toContain('>47</strong>');
  expect(html).toContain('>78,255</strong>');
  expect(html).not.toContain('workbench-run-statistics-grid"><article class="metric-card"');
});
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
Set-Location frontend
npm test -- --run src/views/OverviewView.test.tsx
```

Expected: FAIL because `workbench-statistic` and the six statistic icons are not rendered yet.

- [ ] **Step 3: Add the icon imports and overview layout class**

Change the Lucide import to:

```tsx
import {
  Bomb,
  Coins,
  CreditCard,
  HandHeart,
  Megaphone,
  RefreshCw,
  ShoppingBasket,
  Sprout,
  Tractor,
  type LucideIcon,
} from 'lucide-react';
```

Change the root section to:

```tsx
<section className="view-stack overview-view">
```

- [ ] **Step 4: Replace the six metric calls with icon-aware flat items**

Replace the statistics grid contents with:

```tsx
<div className="workbench-run-statistics-grid">
  <WorkbenchMetric icon={Sprout} tone="collect" label="收获次数" value={formatMetricNumber(runStatistics?.collect)} />
  <WorkbenchMetric icon={Tractor} tone="farm" label="务农次数" value={formatMetricNumber(runStatistics?.farm)} />
  <WorkbenchMetric icon={ShoppingBasket} tone="steal" label="偷菜次数" value={formatMetricNumber(runStatistics?.steal)} />
  <WorkbenchMetric icon={HandHeart} tone="help" label="帮助次数" value={formatMetricNumber(runStatistics?.help)} />
  <WorkbenchMetric icon={Bomb} tone="mischief" label="捣乱次数" value={formatMetricNumber(runStatistics?.mischief)} />
  <WorkbenchMetric icon={Coins} tone="sale" label="出售预估收益" value={saleEstimate?.display || '-'} title={saleEstimate?.title} />
</div>
```

Replace `WorkbenchMetric` with:

```tsx
type WorkbenchMetricProps = {
  icon: LucideIcon;
  tone: 'collect' | 'farm' | 'steal' | 'help' | 'mischief' | 'sale';
  label: string;
  value: string;
  title?: string;
};

function WorkbenchMetric({ icon: Icon, tone, label, value, title }: WorkbenchMetricProps) {
  return (
    <article className="workbench-statistic">
      <Icon className={`workbench-statistic-icon workbench-statistic-icon-${tone}`} size={20} strokeWidth={1.8} aria-hidden="true" />
      <span className="workbench-statistic-label" title={label}>{label}</span>
      <strong className="workbench-statistic-value" title={title}>{value}</strong>
    </article>
  );
}
```

- [ ] **Step 5: Run the focused test and verify GREEN**

Run:

```powershell
Set-Location frontend
npm test -- --run src/views/OverviewView.test.tsx
```

Expected: PASS with all `OverviewView` tests green.

- [ ] **Step 6: Commit the semantic component change**

```powershell
git add -- frontend/src/views/OverviewView.tsx frontend/src/views/OverviewView.test.tsx
git commit -m "feat: render flat workbench statistics"
```

### Task 2: Four-Row Flow And Responsive Density

**Files:**
- Modify: `frontend/src/views/OverviewView.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Replace the obsolete statistics breakpoint test with the failing layout contract**

Replace `keeps workspace run statistics readable across breakpoints` with:

```tsx
it('keeps flat statistics in four-row flow across desktop and remote mobile layouts', () => {
  const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
  const narrowStyles = mediaDeclarations(css, 760);
  const overviewLayout = ruleDeclarations(css, '.overview-view');
  const statisticGrid = ruleDeclarations(css, '.workbench-run-statistics-grid');
  const statistic = ruleDeclarations(css, '.workbench-statistic');
  const statisticValue = ruleDeclarations(css, '.workbench-statistic-value');
  const narrowOverview = ruleDeclarations(narrowStyles, '.app-shell-remote .overview-view');
  const narrowGrid = ruleDeclarations(narrowStyles, '.app-shell-remote .workbench-run-statistics-grid');

  expect(overviewLayout).toMatch(/grid-template-rows:\s*auto\s+auto\s+minmax\(0,\s*1fr\)\s+auto;/);
  expect(statisticGrid).toMatch(/grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\);/);
  expect(statistic).toMatch(/grid-template-columns:\s*auto\s+minmax\(0,\s*1fr\)\s+auto;/);
  expect(statistic).toMatch(/box-shadow:\s*none;/);
  expect(statistic).toMatch(/border:\s*0;/);
  expect(statisticValue).toMatch(/text-align:\s*right;/);
  expect(narrowOverview).toMatch(/grid-template-rows:\s*repeat\(4,\s*auto\);/);
  expect(narrowOverview).toMatch(/height:\s*auto;/);
  expect(narrowGrid).toMatch(/grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/);
});
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
Set-Location frontend
npm test -- --run src/views/OverviewView.test.tsx
```

Expected: FAIL because the overview has no explicit four-row rule and the statistic grid still uses the old card layout.

- [ ] **Step 3: Define desktop four-row flow and flat statistic styles**

Add beside the existing `.view-stack` rules:

```css
.overview-view {
  grid-template-rows: auto auto minmax(0, 1fr) auto;
}
```

Replace the existing `.workbench-run-statistics-grid` and its `.metric-card` overrides with:

```css
.workbench-run-statistics-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin-top: 10px;
}

.workbench-statistic {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  min-width: 0;
  min-height: 48px;
  padding: 9px 11px;
  border: 0;
  border-radius: 6px;
  background: #f3f4f1;
  box-shadow: none;
}

.workbench-statistic-icon {
  flex: 0 0 auto;
}

.workbench-statistic-icon-collect { color: #59b91f; }
.workbench-statistic-icon-farm { color: #00a986; }
.workbench-statistic-icon-steal { color: #f07822; }
.workbench-statistic-icon-help { color: #2689d9; }
.workbench-statistic-icon-mischief { color: #dd4b62; }
.workbench-statistic-icon-sale { color: #d99c00; }

.workbench-statistic-label,
.workbench-statistic-value {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workbench-statistic-label {
  color: #6d746e;
  font-size: 13px;
  font-weight: 700;
}

.workbench-statistic-value {
  color: #202a35;
  font-size: 17px;
  font-weight: 900;
  text-align: right;
}
```

Remove the obsolete `@media (max-width: 980px)` rule that changes `.workbench-run-statistics-grid` to three columns, because three columns are now the desktop default.

- [ ] **Step 4: Make remote mobile overview rows natural and keep two statistic columns**

Inside `@media (max-width: 760px)`, add:

```css
.app-shell-remote .overview-view {
  grid-template-rows: repeat(4, auto);
  height: auto;
  min-height: 100%;
}

.app-shell-remote .workbench-run-statistics {
  margin-top: 0;
  padding-top: 12px;
}

.app-shell-remote .workbench-run-statistics-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 7px;
}

.app-shell-remote .workbench-statistic {
  min-height: 44px;
  padding: 8px;
  gap: 6px;
}

.app-shell-remote .workbench-statistic-label {
  font-size: 12px;
}

.app-shell-remote .workbench-statistic-value {
  font-size: 16px;
}
```

Keep the existing remote event list height and scrolling rules. The natural overview rows make the entire recent-events section consume its actual height before statistics begin.

- [ ] **Step 5: Run the focused test and verify GREEN**

Run:

```powershell
Set-Location frontend
npm test -- --run src/views/OverviewView.test.tsx
```

Expected: PASS with all `OverviewView` tests green.

- [ ] **Step 6: Commit the responsive layout change**

```powershell
git add -- frontend/src/style.css frontend/src/views/OverviewView.test.tsx
git commit -m "fix: prevent mobile workbench section overlap"
```

### Task 3: Full Verification, Build, And Visual QA

**Files:**
- Verify: `frontend/src/views/OverviewView.tsx`
- Verify: `frontend/src/views/OverviewView.test.tsx`
- Verify: `frontend/src/style.css`
- Generate: `frontend/dist/*`

- [ ] **Step 1: Run the complete frontend test suite**

Run:

```powershell
Set-Location frontend
npm test
```

Expected: all Vitest files and tests PASS with exit code `0`.

- [ ] **Step 2: Build the embedded WebUI**

Run:

```powershell
Set-Location frontend
npm run build
```

Expected: TypeScript and Vite exit `0`; `frontend/dist/index.html` references newly emitted hashed assets. This repository embeds `frontend/dist` from `main.go` and has no root `pnpm run frontend:build` or active `public/app` surface.

- [ ] **Step 3: Verify the generated asset reference and timestamp**

Run:

```powershell
Get-Content frontend/dist/index.html
Get-ChildItem frontend/dist/assets | Sort-Object LastWriteTime -Descending | Select-Object -First 5 Name, LastWriteTime, Length
```

Expected: the JavaScript and CSS names referenced by `index.html` exist under `frontend/dist/assets` and have the current build timestamp.

- [ ] **Step 4: Start a local preview and inspect desktop and mobile layouts**

Run:

```powershell
Set-Location frontend
npm run dev -- --host 127.0.0.1
```

Use the browser against the printed local URL. Check at least `1280×720`, `390×844`, `360×740`, and `320×568`. For the mobile checks, load the LAN/remote shell route or state used by `RemoteApp` so `.app-shell-remote` is active.

Expected: desktop shows `3×2`, remote mobile shows `2×3`, recent events end before the statistics heading, all six values stay inside their rows, and the fixed bottom navigation does not cover the final statistics row.

- [ ] **Step 5: Check the final diff without touching unrelated changes**

Run:

```powershell
git diff --check
git status --short
git diff -- frontend/src/views/OverviewView.tsx frontend/src/views/OverviewView.test.tsx frontend/src/style.css frontend/dist/index.html
```

Expected: no whitespace errors; only the intended source, tests, generated WebUI assets, and the pre-existing unrelated `resources/gameConfig.bundle.zip` change appear.

- [ ] **Step 6: Commit generated WebUI assets if tracked**

Run:

```powershell
git add -- frontend/dist
git diff --cached --quiet
if ($LASTEXITCODE -ne 0) { git commit -m "build: refresh embedded workbench webui" }
```

Expected: tracked generated assets are committed when changed; otherwise no empty commit is created.
