# License UI Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Localize the license gate, present updates in a reusable modal, and show live authorization details in the overview.

**Architecture:** `App` continues to normalize and own the safe license status, but passes it to `AuthorizedApp` rather than only the heartbeat count. `AuthorizedApp` owns a single update-dialog state shared by the toast and Settings callback. `OverviewView` receives the same safe status through `FarmWorkspaceView` and renders the compact authorization-service card.

**Tech Stack:** React 18, TypeScript, Vitest, lucide-react, existing Wails bindings, CSS.

---

### Task 1: Localize License Gate And Propagate Safe Status

**Files:**
- Modify: `frontend/src/LicenseGate.tsx`
- Modify: `frontend/src/lib/license.ts`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/AuthorizedApp.tsx`
- Modify: `frontend/src/views/FarmWorkspaceView.tsx`
- Modify: `frontend/src/views/OverviewView.tsx`
- Modify: `frontend/src/LicenseGate.test.tsx`
- Modify: `frontend/src/views/OverviewView.test.tsx`

- [ ] **Step 1: Write failing Chinese fallback and authorization-card tests**

```tsx
it('uses a Chinese fallback for unavailable license status', () => {
  expect(normalizeLicenseStatus(null).message).toBe('授权服务暂不可用，请稍后重试')
})

it('renders authorization state and expiry placeholder', () => {
  const html = renderToStaticMarkup(<OverviewView license={{ authorized: true, heartbeatFailures: 0, expireTime: '' }} {...props} />)
  expect(html).toContain('授权服务')
  expect(html).toContain('已授权 · 心跳正常')
  expect(html).toContain('到期时间 —')
})
```

- [ ] **Step 2: Run focused tests and verify they fail**

Run: `Set-Location frontend; npm test -- LicenseGate OverviewView`

Expected: FAIL because the fallback and overview license prop do not exist.

- [ ] **Step 3: Implement minimal status propagation and localization**

```tsx
// App.tsx
if (status?.authorized) return <AuthorizedApp key={status.generation} licenseStatus={status} />

// AuthorizedApp.tsx
type AuthorizedAppProps = { licenseStatus: LicenseStatus }
<FarmWorkspaceView licenseStatus={licenseStatus} {...workspaceProps} />

// OverviewView.tsx
type OverviewLicense = Pick<LicenseStatus, 'authorized' | 'heartbeatFailures' | 'expireTime'>
```

Keep all user-visible gate labels, placeholders, busy text, checkbox labels, and fallback messages in Chinese. Add `expireTime` to the normalized safe license status with `''` as its unavailable value.

- [ ] **Step 4: Run focused tests and verify they pass**

Run: `Set-Location frontend; npm test -- LicenseGate OverviewView`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add frontend/src/LicenseGate.tsx frontend/src/lib/license.ts frontend/src/App.tsx frontend/src/AuthorizedApp.tsx frontend/src/views/FarmWorkspaceView.tsx frontend/src/views/OverviewView.tsx frontend/src/LicenseGate.test.tsx frontend/src/views/OverviewView.test.tsx
git commit -m "feat: localize license status experience"
```

### Task 2: Add Shared Update Details Dialog

**Files:**
- Create: `frontend/src/components/UpdateDetailsDialog.tsx`
- Create: `frontend/src/components/UpdateDetailsDialog.test.tsx`
- Modify: `frontend/src/components/UpdateAvailableToast.tsx`
- Modify: `frontend/src/AuthorizedApp.tsx`
- Modify: `frontend/src/views/SettingsView.tsx`
- Modify: `frontend/src/views/SettingsView.test.tsx`

- [ ] **Step 1: Write failing modal tests**

```tsx
it('shows release details without opening the browser', () => {
  const html = renderToStaticMarkup(<UpdateDetailsDialog open update={availableUpdate} onClose={() => undefined} onDownload={() => undefined} />)
  expect(html).toContain('发现新版本')
  expect(html).toContain(availableUpdate.latest.description)
  expect(html).not.toContain('BrowserOpenURL')
})
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `Set-Location frontend; npm test -- UpdateDetailsDialog SettingsView`

Expected: FAIL because `UpdateDetailsDialog` does not exist.

- [ ] **Step 3: Implement one root-owned dialog flow**

```tsx
const [dialogUpdate, setDialogUpdate] = useState<UpdateStateDto | null>(null)
const openUpdateDialog = (update: UpdateStateDto) => setDialogUpdate(update)

<UpdateAvailableToast update={updateState} onViewUpdates={() => updateState && openUpdateDialog(updateState)} />
<SettingsView onUpdateAvailable={openUpdateDialog} {...settingsProps} />
<UpdateDetailsDialog open={dialogUpdate !== null} update={dialogUpdate} onClose={() => setDialogUpdate(null)} onDownload={openValidatedDownload} />
```

The dialog opens after an available manual check and after clicking the toast action. It never opens the browser automatically. Its download command calls `BrowserOpenURL` only when the existing backend-safe URL is nonempty.

- [ ] **Step 4: Run focused tests and verify they pass**

Run: `Set-Location frontend; npm test -- UpdateDetailsDialog UpdateAvailableToast SettingsView`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add frontend/src/components/UpdateDetailsDialog.tsx frontend/src/components/UpdateDetailsDialog.test.tsx frontend/src/components/UpdateAvailableToast.tsx frontend/src/AuthorizedApp.tsx frontend/src/views/SettingsView.tsx frontend/src/views/SettingsView.test.tsx
git commit -m "feat: show update details in dialog"
```

### Task 3: Apply Approved Compact Styling And Verify Frontend

**Files:**
- Modify: `frontend/src/style.css`
- Modify: `frontend/src/components/UpdateDetailsDialog.tsx`
- Modify: `frontend/src/views/OverviewView.tsx`

- [ ] **Step 1: Add focused styling selectors and accessible dialog controls**

```css
.overview-license-card { border-color: #cbd8f2; background: #f8fbff; }
.overview-license-pill { background: #2f8d35; color: #fff; border-radius: 999px; }
.update-details-dialog { max-width: 520px; }
```

Use a compact white/blue bordered authorization card, black title, green rounded health band, and an expiry row. Use a modal backdrop and an explicit icon-only close button with an accessible label. Do not introduce storage for card values or raw license data.

- [ ] **Step 2: Run frontend tests**

Run: `Set-Location frontend; npm test`

Expected: PASS.

- [ ] **Step 3: Run production build**

Run: `Set-Location frontend; npm run build`

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
git add frontend/src/style.css frontend/src/components/UpdateDetailsDialog.tsx frontend/src/views/OverviewView.tsx
git commit -m "style: refine license and update surfaces"
```
