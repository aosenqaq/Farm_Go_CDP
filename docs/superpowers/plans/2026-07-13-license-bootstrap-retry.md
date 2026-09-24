# License Bootstrap Retry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Open the card gate without startup verification while reliably prefilling a locally remembered card in a valid release build.

**Architecture:** The React root starts in a quiet locked state and never reads authorization status during bootstrap. It retries only the local remembered-card Wails read until a saved card becomes available; online verification remains in the explicit login submission path.

**Tech Stack:** React 18, TypeScript, Vitest, react-test-renderer, Wails generated bindings.

---

### Task 1: Capture the Cold-Start Regression

**Files:**
- Modify: `frontend/src/AppBootstrap.test.tsx`
- Test: `frontend/src/AppBootstrap.test.tsx`

- [ ] **Step 1: Write the failing test**

```tsx
it('retries remembered-card reads without querying the license service', async () => {
  vi.useFakeTimers();
  vi.mocked(RememberedLicenseCard)
    .mockResolvedValueOnce({ card: '', remembered: false })
    .mockResolvedValueOnce({ card: 'remembered-card', remembered: true });

  let renderer!: ReturnType<typeof create>;
  await act(async () => { renderer = create(<App />); await Promise.resolve(); });
  await act(async () => { await vi.advanceTimersByTimeAsync(100); });

  const markup = JSON.stringify(renderer.toJSON());
  expect(LicenseStatus).not.toHaveBeenCalled();
  expect(markup).not.toContain('授权服务暂不可用，请稍后重试');
  expect(markup).toContain('remembered-card');
  renderer.unmount();
});
```

- [ ] **Step 2: Run the focused test and verify it fails**

Run: `npm test -- AppBootstrap.test.tsx`

Expected: FAIL because `App` reads the bootstrap bindings only once.

### Task 2: Retry Remembered-Card Reads

**Files:**
- Modify: `frontend/src/App.tsx`
- Test: `frontend/src/AppBootstrap.test.tsx`

- [ ] **Step 1: Add a quiet locked initial state and bounded retry helpers**

```ts
const locked: GateStatus = { phase: 'locked', authorized: false, generation: 0, heartbeatFailures: 0, expireTime: '', message: '', errorCode: '' };
const bootstrapRetryDelayMs = 100;
const bootstrapRetryLimit = 30;
```

- [ ] **Step 2: Replace startup status reads with remembered-card retries only**

```ts
void (async () => {
  for (let attempt = 0; active && attempt < bootstrapRetryLimit; attempt += 1) {
    try {
      const nextCard = normalizeRememberedCard(await RememberedLicenseCard());
      if (nextCard.remembered) {
        setCard(nextCard.card);
        setRemembered(true);
        return;
      }
    } catch {}
    await waitForBootstrapRetry();
  }
})();
```

- [ ] **Step 3: Run the focused test and verify it passes**

Run: `npm test -- AppBootstrap.test.tsx`

Expected: PASS with the remembered card rendered only after the ready response and no authorization status request.

### Task 3: Verify the Changed Surface

**Files:**
- Verify: `frontend/src/App.tsx`
- Verify: `frontend/src/AppBootstrap.test.tsx`

- [ ] **Step 1: Run all frontend tests**

Run: `npm test`

Expected: PASS with no failed Vitest suites.

- [ ] **Step 2: Build the production frontend**

Run: `npm run build`

Expected: TypeScript compilation and Vite build exit with status 0.
