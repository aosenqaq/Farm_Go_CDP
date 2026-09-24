# Minimal Workbench Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the runtime overview card matrix with a single readiness hero, inline service signals, recent events, and compact route facts.

**Architecture:** Keep the existing `OverviewView` input contract and derive all display state from its runtime, guard, license, and event props. Render the service state as semantic inline text signals instead of individual cards. Reuse `LogViewer` event data if its markup can be styled without extra data plumbing; otherwise render only an overview-local event list from the existing `events` prop.

**Tech Stack:** React 18, TypeScript, Vitest, lucide-react, CSS.

---

### Task 1: Build The Minimal Runtime Overview

**Files:**
- Modify: `frontend/src/views/OverviewView.tsx`
- Modify: `frontend/src/views/OverviewView.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write failing overview behavior tests**

```tsx
it('renders one readiness hero with inline service signals', () => {
  const html = renderToStaticMarkup(<OverviewView status={readyStatus} licenseStatus={authorizedLicense} events={events} onRefresh={() => undefined} />)
  expect(html).toContain('QQ WS 已就绪')
  expect(html).toContain('可安全执行自动化')
  expect(html).toContain('网关 正常')
  expect(html).toContain('授权 已授权')
  expect(html).not.toContain('overview-status-card')
})

it('renders compact route facts and falls back for missing authorization expiry', () => {
  const html = renderToStaticMarkup(<OverviewView status={readyStatus} licenseStatus={{ authorized: true, heartbeatFailures: 0, expireTime: '' }} events={[]} onRefresh={() => undefined} />)
  expect(html).toContain('到期 —')
  expect(html).toContain('当前链路')
})
```

- [ ] **Step 2: Run focused tests to verify RED**

Run: `Set-Location frontend; npm test -- OverviewView`

Expected: FAIL because the existing view still renders `overview-status-card` matrix markup.

- [ ] **Step 3: Replace card matrix with semantic overview sections**

```tsx
<section className="workbench-hero">
  <p className="workbench-eyebrow">当前链路</p>
  <h2>{runtimeState} <strong>{status.ready ? '已就绪' : controlState}</strong></h2>
  <p>{status.ready ? '控制通道、进程守护与授权服务均处于可用状态。' : '正在等待运行时完成连接。'}</p>
</section>
<div className="workbench-signals" aria-label="服务状态">
  <span className="workbench-signal"><i className="signal-healthy" />网关 <strong>{gatewayState}</strong></span>
  <span className="workbench-signal"><i className={guardState === '监控中' ? 'signal-healthy' : 'signal-muted'} />守护 <strong>{guardState}</strong></span>
  <span className="workbench-signal"><i className={licenseStatus.authorized ? 'signal-healthy' : 'signal-muted'} />授权 <strong>{licenseStatus.authorized ? '已授权' : '未授权'}</strong></span>
  <span className="workbench-signal">到期 <strong>{formatAuthorizationExpiry(licenseStatus.expireTime)}</strong></span>
</div>
```

Render gateway, process guard, authorization, and expiry as inline signals. Preserve the existing safe `AuthorizationServiceStatus` prop boundary. Keep the refresh button and the existing event data. Use `—` for unavailable expiry.

- [ ] **Step 4: Add responsive minimal-layout CSS**

```css
.workbench-hero { border-bottom: 1px solid var(--line); }
.workbench-signals { display: flex; flex-wrap: wrap; }
.workbench-content { display: grid; grid-template-columns: minmax(0, 1fr) 250px; }
@media (max-width: 680px) { .workbench-content { grid-template-columns: 1fr; } }
```

Use unframed sections and divider lines rather than cards. Ensure text has `min-width: 0`, long event messages truncate safely, long expiry values wrap safely, and narrow screens stack without horizontal scrolling.

- [ ] **Step 5: Run focused tests to verify GREEN**

Run: `Set-Location frontend; npm test -- OverviewView`

Expected: PASS.

- [ ] **Step 6: Run full frontend verification**

Run:

```powershell
Set-Location frontend
npm test
npm run build
```

Expected: all Vitest tests and the TypeScript/Vite build PASS.

- [ ] **Step 7: Commit**

```powershell
git add frontend/src/views/OverviewView.tsx frontend/src/views/OverviewView.test.tsx frontend/src/style.css
git commit -m "feat: redesign minimal runtime workbench"
```
