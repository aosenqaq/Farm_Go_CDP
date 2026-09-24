# Account Identity Refresh Banner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the account identity dialog's status mark with a full-width, always-visible refresh banner using the approved Chinese copy and existing retry behavior.

**Architecture:** Keep retry orchestration in `AuthorizedApp` and make `AccountIdentityDialog` responsible only for presenting the retry control and its loading state. Reuse the existing `onRetry` callback, remove the duplicate error-only retry button, and constrain visual changes to the dialog styles.

**Tech Stack:** React 18, TypeScript, Lucide React, Vitest, Vite, CSS

---

### Task 1: Define the refresh banner behavior with component tests

**Files:**
- Modify: `frontend/src/components/AccountIdentityDialog.test.tsx`
- Test: `frontend/src/components/AccountIdentityDialog.test.tsx`

- [ ] **Step 1: Add failing assertions for the normal state**

Extend the existing identified-account test with assertions for the approved copy, the dedicated banner class, and the Lucide refresh icon:

```tsx
expect(html).toContain('如果识别错误/失败，可以点击这里重新识别');
expect(html).toContain('class="account-identity-refresh"');
expect(html).toContain('lucide-refresh-cw');
```

- [ ] **Step 2: Add failing assertions for error and loading states**

In the failure test, assert that the legacy error-only secondary action is gone:

```tsx
expect(html).toContain('如果识别错误/失败，可以点击这里重新识别');
expect(html).not.toContain('secondary-button');
```

Add a loading-state test:

```tsx
it('disables the refresh banner while identifying', () => {
  const html = renderToStaticMarkup(
    <AccountIdentityDialog open account={null} confirming={false} identifying onConfirm={() => undefined} onRetry={() => undefined} />,
  );

  expect(html).toMatch(/<button class="account-identity-refresh"[^>]*disabled=""/);
  expect(html).toContain('spin');
  expect(html).toContain('如果识别错误/失败，可以点击这里重新识别');
});
```

- [ ] **Step 3: Run the focused test and verify RED**

Run:

```powershell
npm test -- AccountIdentityDialog.test.tsx
```

Working directory: `frontend`

Expected: FAIL because `account-identity-refresh` and the approved copy are not rendered in the current normal state, and the legacy `secondary-button` still renders for errors.

### Task 2: Implement the banner structure and responsive styling

**Files:**
- Modify: `frontend/src/components/AccountIdentityDialog.tsx`
- Modify: `frontend/src/style.css`
- Test: `frontend/src/components/AccountIdentityDialog.test.tsx`

- [ ] **Step 1: Replace the status mark with the refresh banner**

Remove `AlertTriangle` from the Lucide import and replace `.account-identity-mark` markup with:

```tsx
<button className="account-identity-refresh" type="button" onClick={onRetry} disabled={identifying}>
  {identifying ? <Loader2 className="spin" size={18} /> : <RefreshCw size={18} />}
  <span>如果识别错误/失败，可以点击这里重新识别</span>
</button>
```

Keep `CheckCircle2`, `Loader2`, `RefreshCw`, and `UserRound` imports because they remain in use.

- [ ] **Step 2: Remove the duplicate error-only retry button**

Reduce the footer to the existing confirmation button only:

```tsx
<footer className="account-identity-actions">
  <button className="primary-button" type="button" onClick={onConfirm} disabled={!canConfirm || confirming || identifying}>
    {confirming ? <Loader2 className="spin" size={17} /> : <CheckCircle2 size={17} />}
    <span>{confirming ? '确认中' : '确认进入'}</span>
  </button>
</footer>
```

- [ ] **Step 3: Replace the status-mark CSS with full-width banner CSS**

Delete the `.account-identity-mark` rule and add:

```css
.account-identity-refresh {
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: center;
  width: calc(100% + 48px);
  min-height: 46px;
  margin: -24px -24px 22px;
  padding: 9px 16px;
  border: 0;
  border-bottom: 1px solid #b9d878;
  border-radius: 9px 9px 0 0;
  color: #273020;
  background: #e9ffb8;
  font: inherit;
  font-size: 12px;
  font-weight: 800;
  line-height: 1.5;
  text-align: left;
  cursor: pointer;
}

.account-identity-refresh:hover:not(:disabled) {
  background: #dfff91;
}

.account-identity-refresh:focus-visible {
  outline: 3px solid rgba(120, 165, 28, 0.32);
  outline-offset: -3px;
}

.account-identity-refresh:disabled {
  cursor: wait;
  opacity: 0.72;
}

.account-identity-refresh svg {
  flex: 0 0 auto;
}
```

The existing dialog width constraint provides enough room on desktop; Chinese text may wrap naturally on narrow viewports while the stable minimum height prevents layout jitter.

- [ ] **Step 4: Run the focused test and verify GREEN**

Run:

```powershell
npm test -- AccountIdentityDialog.test.tsx
```

Working directory: `frontend`

Expected: 3 tests pass.

- [ ] **Step 5: Commit the tested component change**

```powershell
git add -- frontend/src/components/AccountIdentityDialog.tsx frontend/src/components/AccountIdentityDialog.test.tsx frontend/src/style.css
git commit -m "feat: add account identity refresh banner"
```

### Task 3: Build and visually verify the dialog

**Files:**
- Verify: `frontend/src/components/AccountIdentityDialog.tsx`
- Verify: `frontend/src/style.css`
- Verify generated Vite output under `frontend/dist/`

- [ ] **Step 1: Run the complete frontend test suite**

Run:

```powershell
npm test
```

Working directory: `frontend`

Expected: all Vitest suites pass with zero failures.

- [ ] **Step 2: Build the frontend**

Run:

```powershell
npm run build
```

Working directory: `frontend`

Expected: TypeScript compilation and Vite production build exit successfully, and `frontend/dist/index.html` references the newly generated asset.

- [ ] **Step 3: Verify desktop and narrow layouts**

Open the running application and inspect the account identity dialog at approximately 1280x800 and 390x844. Confirm the banner stays flush with the top of the dialog, the text does not overflow or cover the avatar, the error message and confirmation button do not overlap, and the loading state does not resize the control.

- [ ] **Step 4: Check the final diff**

Run:

```powershell
git status --short
git diff --check HEAD~1..HEAD
```

Expected: no unintended tracked changes and no whitespace errors in the feature commit.
