# Automation Start Loading Feedback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the automation start action and injection launch/restart actions immediate, consistent spinner-plus-label feedback while their real asynchronous operations are pending.

**Architecture:** Keep each component's existing boolean state and request lifecycle. Render the existing Lucide `Loader2` plus the global `spin` class only for startup operations, add one shared `async-action-button` marker for stable layout and reduced-motion handling, and leave backend APIs and error flows untouched.

**Tech Stack:** React 18, TypeScript, Lucide React, Vitest, react-test-renderer, Vite CSS

---

## File Map

- Modify `frontend/src/views/AutomationView.tsx`: distinguish start from stop while the total automation toggle is pending.
- Modify `frontend/src/views/AutomationView.test.tsx`: cover pending start feedback, duplicate-click protection, and unchanged stop feedback.
- Modify `frontend/src/components/StartupInjectionDialog.tsx`: standardize busy markers and accessibility state for launch and restart actions.
- Modify `frontend/src/components/StartupInjectionDialog.test.tsx`: cover launch/restart busy rendering and idle rendering.
- Modify `frontend/src/style.css`: stabilize async button contents and honor reduced-motion preferences.
- Modify `frontend/src/views/AutomationView.test.tsx`: import the CSS as text and assert the reduced-motion rule exists.

### Task 1: Automation start feedback

**Files:**
- Modify: `frontend/src/views/AutomationView.test.tsx`
- Modify: `frontend/src/views/AutomationView.tsx:1,510-528,578-581`

- [ ] **Step 1: Write the failing pending-start test**

Add a test that holds the existing toggle promise open, inspects the rerendered button, calls the guarded handler a second time, and then settles the promise:

```tsx
it('shows explicit loading feedback while automation is starting and blocks duplicate starts', async () => {
  let finishToggle!: () => void;
  const pendingToggle = new Promise<void>((resolve) => { finishToggle = resolve; });
  const onToggleAutomation = vi.fn(() => pendingToggle);
  const renderer = TestRenderer.create(
    <AutomationView state={state} onRunTask={() => undefined} onToggleAutomation={onToggleAutomation} />,
  );
  let request!: Promise<void>;

  await act(async () => {
    request = renderer.root.findByProps({ 'aria-label': '启动自动化' }).props.onClick();
    await Promise.resolve();
  });

  const busyButton = renderer.root.findByProps({ 'aria-label': '正在启动自动化' });
  expect(busyButton.props.disabled).toBe(true);
  expect(busyButton.props['aria-busy']).toBe(true);
  expect(busyButton.findAllByProps({ className: 'spin' })).toHaveLength(1);
  expect(busyButton.findAllByType('span').map((node) => node.children.join(''))).toContain('启动中');

  await act(async () => {
    void busyButton.props.onClick();
    await Promise.resolve();
  });
  expect(onToggleAutomation).toHaveBeenCalledTimes(1);
  expect(onToggleAutomation).toHaveBeenCalledWith(true);

  await act(async () => {
    finishToggle();
    await request;
  });
});
```

- [ ] **Step 2: Write the failing stop-regression test**

```tsx
it('keeps the existing simple processing feedback while automation is stopping', async () => {
  let finishToggle!: () => void;
  const pendingToggle = new Promise<void>((resolve) => { finishToggle = resolve; });
  const renderer = TestRenderer.create(
    <AutomationView
      state={{ ...state, running: true }}
      onRunTask={() => undefined}
      onToggleAutomation={() => pendingToggle}
    />,
  );
  let request!: Promise<void>;

  await act(async () => {
    request = renderer.root.findByProps({ 'aria-label': '停止自动化' }).props.onClick();
    await Promise.resolve();
  });

  const busyButton = renderer.root.findByProps({ 'aria-label': '正在停止自动化' });
  expect(busyButton.findAllByProps({ className: 'spin' })).toHaveLength(0);
  expect(busyButton.findAllByType('span').map((node) => node.children.join(''))).toContain('处理中');

  await act(async () => {
    finishToggle();
    await request;
  });
});
```

- [ ] **Step 3: Run the focused test and verify RED**

Run: `npm test -- AutomationView.test.tsx` from `frontend`.

Expected: FAIL because the master button has no automation-specific accessible label, no `aria-busy`, no spinner, and no text wrapper.

- [ ] **Step 4: Implement the minimal start-only rendering**

Add `Loader2` to the Lucide import, derive the pending operation, and update the button:

```tsx
const automationStarting = automationToggling && !state.running;

<button
  aria-busy={automationToggling || undefined}
  aria-label={automationToggling
    ? state.running ? '正在停止自动化' : '正在启动自动化'
    : state.running ? '停止自动化' : '启动自动化'}
  className="primary-button async-action-button"
  disabled={automationToggling || !onToggleAutomation}
  type="button"
  onClick={toggleAutomation}
>
  {automationStarting ? <Loader2 className="spin" size={16} /> : <Power size={16} />}
  <span>{automationStarting ? '启动中' : automationToggling ? '处理中' : state.running ? '停止' : '启动'}</span>
</button>
```

- [ ] **Step 5: Run the focused test and verify GREEN**

Run: `npm test -- AutomationView.test.tsx` from `frontend`.

Expected: PASS, including the new start and stop cases.

- [ ] **Step 6: Commit the automation button change**

```bash
git add frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx
git commit -m "feat: add automation start loading feedback"
```

### Task 2: Injection launch and restart feedback semantics

**Files:**
- Modify: `frontend/src/components/StartupInjectionDialog.test.tsx`
- Modify: `frontend/src/components/StartupInjectionDialog.tsx:106-131`

- [ ] **Step 1: Write failing launch and restart busy-state assertions**

Import `TestRenderer` and add one parameterized test. The existing idle launch and restart tests stay unchanged:

```tsx
import TestRenderer from 'react-test-renderer';

it('marks launch and restart actions as busy while their operations are pending', () => {
  const scenarios = [
    { launching: true, restarting: false, showLaunch: true, showRestart: false, label: '启动中' },
    { launching: false, restarting: true, showLaunch: false, showRestart: true, label: '重启中' },
  ];

  for (const scenario of scenarios) {
    const renderer = TestRenderer.create(
      <StartupInjectionDialog
        open
        state={{
          target: 'qq_ws',
          title: 'QQ 补丁注入',
          headline: 'QQ 小程序补丁已就绪',
          detail: '等待启动操作完成。',
          tone: 'success',
          primaryMetricLabel: '补丁状态',
          primaryMetricValue: '已就绪',
          secondaryMetricLabel: '目标',
          secondaryMetricValue: 'game.js',
          tertiaryMetricLabel: 'Host 版本',
          tertiaryMetricValue: '1.0.0',
          showRetry: false,
          showLaunch: scenario.showLaunch,
          showRestart: scenario.showRestart,
          inFlight: false,
          shouldShow: true,
        }}
        targetPath=""
        patching={false}
        launching={scenario.launching}
        restarting={scenario.restarting}
        onTargetPathChange={() => undefined}
        onRetry={() => undefined}
        onLaunch={() => undefined}
        onRestart={() => undefined}
        onHide={() => undefined}
      />,
    );

    const busyButton = renderer.root.findByProps({ 'aria-busy': true });
    expect(busyButton.props.className).toContain('async-action-button');
    expect(busyButton.props.disabled).toBe(true);
    expect(busyButton.findAllByProps({ className: 'spin' })).toHaveLength(1);
    expect(busyButton.findAllByType('span').map((node) => node.children.join(''))).toContain(scenario.label);
    renderer.unmount();
  }
});
```

- [ ] **Step 2: Run the focused dialog test and verify RED**

Run: `npm test -- StartupInjectionDialog.test.tsx` from `frontend`.

Expected: FAIL because the launch/restart buttons do not yet expose `aria-busy` or the shared `async-action-button` marker.

- [ ] **Step 3: Add the shared marker and busy semantics**

Update both action buttons without changing their existing loading icon, label, handlers, or disabled conditions:

```tsx
<button
  aria-busy={launching || undefined}
  className="primary-button async-action-button"
  type="button"
  onClick={onLaunch}
  disabled={!onLaunch || launching || patching || state.inFlight}
>
```

```tsx
<button
  aria-busy={restarting || undefined}
  className="primary-button async-action-button"
  type="button"
  onClick={onRestart}
  disabled={!onRestart || restarting || patching || state.inFlight}
>
```

- [ ] **Step 4: Run the focused dialog test and verify GREEN**

Run: `npm test -- StartupInjectionDialog.test.tsx` from `frontend`.

Expected: PASS for both idle and pending launch/restart states.

- [ ] **Step 5: Commit the injection button change**

```bash
git add frontend/src/components/StartupInjectionDialog.tsx frontend/src/components/StartupInjectionDialog.test.tsx
git commit -m "feat: standardize injection action loading states"
```

### Task 3: Stable layout and reduced motion

**Files:**
- Modify: `frontend/src/views/AutomationView.test.tsx`
- Modify: `frontend/src/style.css:1042-1053,7743-7754`

- [ ] **Step 1: Write the failing CSS contract test**

Import the stylesheet as raw text in `AutomationView.test.tsx`:

```tsx
import styleSource from '../style.css?raw';
```

Add the contract test:

```tsx
it('keeps async action content stable and disables its spinner for reduced motion', () => {
  expect(styleSource).toContain('.async-action-button > svg');
  expect(styleSource).toMatch(
    /@media \(prefers-reduced-motion: reduce\)\s*{\s*\.async-action-button\[aria-busy="true"\] \.spin/,
  );
});
```

- [ ] **Step 2: Run the CSS contract test and verify RED**

Run: `npm test -- AutomationView.test.tsx` from `frontend`.

Expected: FAIL because `async-action-button` has no CSS contract yet.

- [ ] **Step 3: Add stable layout and reduced-motion styles**

Place the layout rules near `.primary-button`:

```css
.async-action-button {
  white-space: nowrap;
}

.async-action-button > svg {
  flex: 0 0 auto;
}
```

Add the spinner override as the first rule in the existing final reduced-motion block:

```css
.async-action-button[aria-busy="true"] .spin {
  animation: none;
}
```

- [ ] **Step 4: Run the CSS contract test and verify GREEN**

Run: `npm test -- AutomationView.test.tsx` from `frontend`.

Expected: PASS.

- [ ] **Step 5: Commit the motion styling**

```bash
git add frontend/src/style.css frontend/src/views/AutomationView.test.tsx
git commit -m "style: refine async action motion"
```

### Task 4: Full verification and embedded frontend build

**Files:**
- Verify: `frontend/src/views/AutomationView.test.tsx`
- Verify: `frontend/src/components/StartupInjectionDialog.test.tsx`
- Generated: `frontend/dist/index.html`
- Generated: `frontend/dist/assets/*`

- [ ] **Step 1: Run the complete frontend test suite**

Run: `npm test` from `frontend`.

Expected: all Vitest files pass with no unhandled errors.

- [ ] **Step 2: Run the production frontend build**

Run: `npm run build` from `frontend`.

Expected: TypeScript and Vite exit successfully and emit `frontend/dist/index.html` plus a newly hashed JavaScript/CSS asset set. This is the directory embedded by `main.go`.

- [ ] **Step 3: Verify the generated asset reference and timestamp**

Run from the repository root:

```powershell
Get-Content frontend\dist\index.html
Get-ChildItem frontend\dist\assets | Sort-Object LastWriteTime -Descending | Select-Object -First 5 Name,LastWriteTime,Length
```

Expected: `index.html` references the newly generated asset names and those files have the current build timestamp.

- [ ] **Step 4: Verify the source diff and whitespace**

Run:

```bash
git diff --check
git status --short
```

Expected: no whitespace errors; only the planned source, tests, style, plan, and normal tracked build outputs appear.

- [ ] **Step 5: Inspect the running frontend**

Start or reuse the Vite development server, open the automation page and the injection prompt, and verify that button dimensions do not shift while the spinner and label replace the idle content. Repeat with reduced-motion emulation and confirm the spinner is static while the busy label remains visible.

- [ ] **Step 6: Commit any tracked generated frontend output required by repository convention**

```bash
git add frontend/dist
git commit -m "build: refresh embedded frontend assets"
```

Skip this commit when `frontend/dist` is ignored or produces no tracked diff.
