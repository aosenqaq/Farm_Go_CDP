# Lan WebUI 运行模式移动端抽屉 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert the Lan WebUI run-mode selection and confirmation dialogs into compact, content-sized mobile bottom drawers without changing desktop behavior or mode-switching logic.

**Architecture:** Keep `AutomationView.tsx` and its two-step state machine unchanged. Add a stylesheet contract test, then update only the existing `.app-shell-remote` mobile overrides in `style.css`: remove the fixed `70dvh` height, cap the sheet to the visual viewport, add a decorative drawer handle, compact the option list, and make the footer an equal-width mobile action bar.

**Tech Stack:** React 18, TypeScript, Vitest, CSS media queries, Vite.

---

## File Structure

- Modify: `frontend/src/views/AutomationView.test.tsx:76-128`
  Add a stylesheet contract test near the existing run-mode interaction test. It reads the actual CSS through the existing `styleSource` fixture.
- Modify: `frontend/src/style.css:8271-8443`
  Change only the mobile `.app-shell-remote` rules for the run-mode dialog, its header, body, option list, and footer.

### Task 1: Add The Mobile Drawer Style Contract

**Files:**
- Modify: `frontend/src/views/AutomationView.test.tsx:76-128`
- Test: `frontend/src/views/AutomationView.test.tsx`

- [ ] **Step 1: Write the failing test**

Add this test immediately after `keeps async action content stable and disables its spinner for reduced motion`:

```tsx
  it('uses a content-sized bottom drawer for run modes in the remote mobile layout', () => {
    const mobileDrawer = styleSource.match(
      /\.app-shell-remote \.automation-run-mode-dialog\s*{([\s\S]*?)\n  }/,
    )?.[1] || '';

    expect(mobileDrawer).toContain('height: auto;');
    expect(mobileDrawer).toContain('max-height: calc(100dvh - 8px);');
    expect(mobileDrawer).not.toContain('height: min(70dvh, 560px);');
    expect(styleSource).toMatch(
      /\.app-shell-remote \.automation-run-mode-dialog::before\s*{[\s\S]*content: '';[\s\S]*display: block;/,
    );
    expect(styleSource).toMatch(
      /\.app-shell-remote \.automation-run-mode-dialog > footer\s*{[\s\S]*display: grid;[\s\S]*grid-template-columns: repeat\(2, minmax\(0, 1fr\)\);/,
    );
  });
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run from `frontend/`:

```powershell
npm test -- src/views/AutomationView.test.tsx
```

Expected: FAIL in `uses a content-sized bottom drawer for run modes in the remote mobile layout` because the dialog still declares `height: min(70dvh, 560px)` and has neither the drawer handle nor a grid footer.

- [ ] **Step 3: Keep the red test local**

Do not create a standalone red commit for this small CSS-only change. Continue to the implementation immediately, then commit the passing test and its CSS together.

### Task 2: Implement The Content-Sized Mobile Drawer

**Files:**
- Modify: `frontend/src/style.css:8297-8305`
- Modify: `frontend/src/style.css:8435-8443`
- Test: `frontend/src/views/AutomationView.test.tsx`

- [ ] **Step 1: Replace the fixed mobile height and add the drawer handle**

Replace the mobile run-mode dialog block with the following rules. They constrain tall content without forcing short content to fill 70% of the viewport:

```css
  .app-shell-remote .automation-run-mode-dialog {
    position: relative;
    height: auto;
    max-height: calc(100dvh - 8px);
  }

  .app-shell-remote .automation-run-mode-dialog::before {
    content: '';
    position: absolute;
    top: 8px;
    left: 50%;
    display: block;
    width: 36px;
    height: 4px;
    border-radius: 999px;
    background: #c7bea9;
    transform: translateX(-50%);
  }
```

- [ ] **Step 2: Give the header space for the handle and keep the body scrollable**

Split the existing grouped header selector so only the run-mode header receives top space, then retain the existing body scrolling rule:

```css
  .app-shell-remote .automation-scheduler-dialog > header,
  .app-shell-remote .automation-settings-dialog > header {
    padding: 11px 12px;
  }

  .app-shell-remote .automation-run-mode-dialog > header {
    padding: 22px 12px 11px;
  }

  .app-shell-remote .automation-run-mode-body {
    min-height: 0;
    overflow: auto;
    padding: 12px;
  }

  .app-shell-remote .automation-run-mode-options {
    gap: 8px;
  }
```

- [ ] **Step 3: Make the mobile action bar ergonomic and safe-area aware**

Replace the current mobile-only run-mode footer padding rule with this compact two-column action bar:

```css
  .app-shell-remote .automation-run-mode-dialog > footer {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    align-items: stretch;
    gap: 8px;
    padding: 10px 12px max(10px, env(safe-area-inset-bottom));
  }

  .app-shell-remote .automation-run-mode-dialog > footer > button {
    width: 100%;
    min-width: 0;
    min-height: 38px;
  }
```

Do not change `AutomationView.tsx`: `runModeSaving`, selection guards, labels, and confirmation actions are already correct and must retain their current behavior.

- [ ] **Step 4: Run the focused test to verify it passes**

Run from `frontend/`:

```powershell
npm test -- src/views/AutomationView.test.tsx
```

Expected: PASS with the new mobile-drawer style contract and the pre-existing two-step run-mode interaction test both passing.

- [ ] **Step 5: Commit the implementation**

```powershell
git add frontend/src/style.css frontend/src/views/AutomationView.test.tsx && git commit -m "fix: use mobile drawer for automation run modes"
```

### Task 3: Verify Behavior, Build, And Mobile Presentation

**Files:**
- Verify: `frontend/src/views/AutomationView.test.tsx`
- Verify: `frontend/src/style.css`

- [ ] **Step 1: Run the full frontend test suite**

Run from `frontend/`:

```powershell
npm test
```

Expected: exit code `0` with all Vitest suites passing.

- [ ] **Step 2: Run the production build**

Run from `frontend/`:

```powershell
npm run build
```

Expected: exit code `0`; TypeScript validation and the Vite production build both complete successfully.

- [ ] **Step 3: Perform a mobile visual check**

Run from `frontend/`:

```powershell
npm run dev -- --host 127.0.0.1 --port 5173
```

Open the Lan WebUI at a `375px`-wide viewport. For both the run-mode selection dialog and the confirmation dialog, verify all of the following:

```text
- The dialog attaches to the bottom of the viewport with rounded top corners and a centered handle.
- The sheet finishes immediately after its content instead of reserving a 70dvh empty area.
- The card list and guard-service note remain readable; when constrained by viewport height, only the body scrolls.
- Cancel/Back and Next/Confirm are equal-width, visible above the safe area, and do not overlap text.
- Desktop viewport styling remains the centered dialog.
```

- [ ] **Step 4: Inspect the final diff before handing off**

```powershell
git diff --check HEAD~1..HEAD && git status --short
```

Expected: no whitespace errors and no untracked or unstaged implementation files.
