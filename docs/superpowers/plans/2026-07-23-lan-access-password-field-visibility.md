# LAN Password Field Visibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the two LAN password controls visibly styled and replace the default save action text with “保存设置”.

**Architecture:** Keep the existing `LANAccessSettingsPanel` state and save flow intact. Add one component-specific class to each password input, style that class under the existing LAN form scope, and use the component test as a regression contract for the visual hook and label.

**Tech Stack:** React 18, TypeScript, Vitest, CSS, Vite.

---

## File Structure

- `frontend/src/components/LANAccessSettingsPanel.test.tsx`: Regression assertions for the password input styling hook and save label.
- `frontend/src/components/LANAccessSettingsPanel.tsx`: Adds the shared password-input class and changes the idle button label.
- `frontend/src/style.css`: Defines the scoped visible password-input presentation for `.lan-access-form`.

### Task 1: Define the Regression Contracts

**Files:**
- Modify: `frontend/src/components/LANAccessSettingsPanel.test.tsx`
- Test: `frontend/src/components/LANAccessSettingsPanel.test.tsx`

- [x] **Step 1: Write the failing password-field presentation test**

Add this test inside `describe('LANAccessSettingsPanel', ...)`:

```tsx
  it('uses the visible LAN password field style hook', async () => {
    let renderer!: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(<LANAccessSettingsPanel />);
      await flushEffects();
    });

    const password = renderer.root.findByProps({ name: 'lan-access-password' });
    const confirmation = renderer.root.findByProps({ name: 'lan-access-confirm-password' });
    expect(password.props.type).toBe('password');
    expect(confirmation.props.type).toBe('password');
    expect(password.props.className).toBe('lan-access-password-input');
    expect(confirmation.props.className).toBe('lan-access-password-input');
    renderer.unmount();
  });
```

- [x] **Step 2: Run the presentation test and confirm it fails**

Run from `frontend`:

```powershell
npx vitest run src/components/LANAccessSettingsPanel.test.tsx
```

Expected: FAIL because the two password inputs do not yet have `className="lan-access-password-input"`.

- [x] **Step 3: Write the failing save-label test**

Add this test inside `describe('LANAccessSettingsPanel', ...)`:

```tsx
  it('uses the concise LAN settings save label', async () => {
    let renderer!: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(<LANAccessSettingsPanel />);
      await flushEffects();
    });

    const markup = JSON.stringify(renderer.toJSON());
    expect(markup).toContain('保存设置');
    expect(markup).not.toContain('保存 LAN 设置');
    renderer.unmount();
  });
```

- [x] **Step 4: Run the save-label test and confirm it fails**

Run from `frontend`:

```powershell
npx vitest run src/components/LANAccessSettingsPanel.test.tsx
```

Expected: FAIL because the idle save button still renders “保存 LAN 设置”.

### Task 2: Implement the Scoped Visual Fix

**Files:**
- Modify: `frontend/src/components/LANAccessSettingsPanel.tsx`
- Modify: `frontend/src/style.css`
- Test: `frontend/src/components/LANAccessSettingsPanel.test.tsx`

- [x] **Step 1: Add the password field class and update the label**

On both inputs named `lan-access-password` and `lan-access-confirm-password`, add:

```tsx
className="lan-access-password-input"
```

Update the idle save text in the button to:

```tsx
<span>{saving ? '正在保存' : '保存设置'}</span>
```

- [x] **Step 2: Add the scoped password input style**

Immediately after the `.lan-access-panel` rule in `frontend/src/style.css`, add:

```css
.lan-access-form .lan-access-password-input {
  width: 100%;
  height: 38px;
  padding: 0 10px;
  border: 1px solid #e2d7bf;
  border-radius: 9px;
  color: #0c1110;
  background: #fffdf7;
  outline: none;
}
```

- [x] **Step 3: Run the component test and confirm it passes**

Run from `frontend`:

```powershell
npx vitest run src/components/LANAccessSettingsPanel.test.tsx
```

Expected: PASS with both new tests and all pre-existing LAN settings tests green.

- [x] **Step 4: Run the production frontend build**

Run from `frontend`:

```powershell
npm run build
```

Expected: `tsc && vite build` exits with code 0.

- [x] **Step 5: Inspect the change set before committing**

Run from the repository root:

```powershell
git diff --check
git diff -- frontend/src/components/LANAccessSettingsPanel.test.tsx frontend/src/components/LANAccessSettingsPanel.tsx frontend/src/style.css
```

Expected: no whitespace errors; changes are limited to the two tests, two input class hooks, save label, and scoped CSS rule.

- [x] **Step 6: Commit the implementation**

```powershell
git add -- frontend/src/components/LANAccessSettingsPanel.test.tsx frontend/src/components/LANAccessSettingsPanel.tsx frontend/src/style.css docs/superpowers/plans/2026-07-23-lan-access-password-field-visibility.md
git commit -m "fix: show LAN password inputs"
```
