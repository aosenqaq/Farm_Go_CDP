# QQ Runtime Reconnect Dialog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop QQ WebSocket reconnects from repeatedly reinstalling an unchanged patch or reopening the same hash-mismatch dialog.

**Architecture:** Keep the automatic disk-patch check keyed by the Farm_Go patch hash, not the per-connection QQ WS `instanceId`. Run the lightweight runtime hash diagnostic for each connection so a genuine miniapp restart clears a stale hash promptly. Deduplicate dialog reveals by the `(runningHash, patchHash)` mismatch signature.

**Tech Stack:** React 18, TypeScript, Vitest.

---

### Task 1: Specify reconnect-safe automatic patch decisions

**Files:**

- Modify: `frontend/src/App.test.tsx:76-84`
- Modify: `frontend/src/AuthorizedApp.tsx:153-164`

- [x] **Step 1: Write the failing test**

Replace the instance-ID expectation with generation-aware cases:

```tsx
it('checks an automatic QQ patch once per patch generation, not per WS reconnect', () => {
  const firstConnection = { target: 'qq_ws', phase: 'ready', ready: true, connected: true, instanceId: 'one' } as const;
  const reconnected = { ...firstConnection, instanceId: 'two' } as const;

  expect(runtimePatchCheckDecision('', firstConnection, 'farm-hash')).toEqual({
    nextCheckedKey: 'qq_ws:farm-hash',
    shouldRun: true,
  });
  expect(runtimePatchCheckDecision('qq_ws:farm-hash', reconnected, 'farm-hash')).toEqual({
    nextCheckedKey: 'qq_ws:farm-hash',
    shouldRun: false,
  });
  expect(runtimePatchCheckDecision('qq_ws:farm-hash', reconnected, 'new-farm-hash').shouldRun).toBe(true);
});
```

- [x] **Step 2: Run test to verify it fails**

Run: `npm test -- --run frontend/src/App.test.tsx`

Expected: FAIL because `runtimePatchCheckDecision` does not accept or use the patch hash and still returns an `instanceId`-based key.

- [x] **Step 3: Write minimal implementation**

Change the exported helpers to accept an optional patch hash and construct the key from it:

```tsx
export function runtimePatchCheckKey(status: RuntimeStatusDto, patchHash = '') {
  if (status.target !== 'qq_ws' || status.ready !== true || status.connected !== true) return '';
  return `${status.target}:${patchHash || 'unverified'}`;
}

export function runtimePatchCheckDecision(checkedKey: string, status: RuntimeStatusDto, patchHash = '') {
  const nextCheckedKey = runtimePatchCheckKey(status, patchHash);
  return {
    nextCheckedKey,
    shouldRun: nextCheckedKey !== '' && nextCheckedKey !== checkedKey,
  };
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `npm test -- --run frontend/src/App.test.tsx`

Expected: PASS, including the generation-change assertion.

- [x] **Step 5: Commit**

```powershell
git add frontend/src/App.test.tsx frontend/src/AuthorizedApp.tsx
git commit -m "test: cover QQ reconnect patch generation"
```

### Task 2: Keep diagnostics fresh while deduplicating restart prompts

**Files:**

- Modify: `frontend/src/App.test.tsx:76-84`
- Modify: `frontend/src/AuthorizedApp.tsx:237-240,492-530`

- [x] **Step 1: Write the failing test**

Add focused helper tests that define the two independent identities:

```tsx
it('checks runtime hashes for each QQ connection but reveals a mismatch once per hash pair', () => {
  const first = { target: 'qq_ws', phase: 'ready', ready: true, connected: true, instanceId: 'one' } as const;
  const reconnected = { ...first, instanceId: 'two' } as const;

  expect(runtimeHashCheckKey(first, 'farm-hash')).toBe('qq_ws:one:farm-hash');
  expect(runtimeHashCheckKey(reconnected, 'farm-hash')).toBe('qq_ws:two:farm-hash');
  expect(runtimePatchMismatchDecision('', 'hq-hash', 'farm-hash').shouldReveal).toBe(true);
  expect(runtimePatchMismatchDecision('hq-hash:farm-hash', 'hq-hash', 'farm-hash').shouldReveal).toBe(false);
  expect(runtimePatchMismatchDecision('hq-hash:farm-hash', 'farm-hash', 'farm-hash').shouldClear).toBe(true);
});
```

- [x] **Step 2: Run test to verify it fails**

Run: `npm test -- --run frontend/src/App.test.tsx`

Expected: FAIL because `runtimeHashCheckKey` and `runtimePatchMismatchDecision` are not exported.

- [x] **Step 3: Write minimal implementation**

Add `runtimeHashCheckKey` and `runtimePatchMismatchDecision`. Split the current effect into a hash-diagnostic effect and a patch-install effect:

```tsx
const runtimeHashCheckedKey = useRef('');
const revealedPatchMismatchKey = useRef('');

const hashCheckKey = runtimeHashCheckKey(status, patchStatus?.scriptHash || '');
```

Run `host.describe` for every new hash-check key so a restarted miniapp can replace an old runtime hash, but reveal each mismatch signature only once. Keep `autoPatchCheckedKey` while QQ only disconnects; clear it only after the selected runtime target changes away from `qq_ws`. After `InstallQQDebugPatch`, store `runtimePatchCheckKey(status, patchResult.scriptHash)` so the result does not schedule a redundant second install. When a diagnostic reports matching hashes, clear `revealedPatchMismatchKey`; when it reports a new mismatch key, update the ref and call `setInjectionHidden(false)`.

- [x] **Step 4: Run tests to verify they pass**

Run: `npm test -- --run frontend/src/App.test.tsx frontend/src/lib/startupInjection.test.ts`

Expected: PASS. The existing test that requires a restart on a real hash mismatch remains green.

- [x] **Step 5: Commit**

```powershell
git add frontend/src/App.test.tsx frontend/src/AuthorizedApp.tsx
git commit -m "fix: suppress duplicate QQ restart prompts"
```

### Task 3: Verify the production frontend change

**Files:**

- Verify: `frontend/src/App.test.tsx`
- Verify: `frontend/src/lib/startupInjection.test.ts`
- Verify: `frontend/src/AuthorizedApp.tsx`

- [x] **Step 1: Run the complete frontend test suite**

Run: `npm test`

Expected: PASS with no failures.

- [x] **Step 2: Build the frontend**

Run: `npm run build`

Expected: PASS and produce the Vite distribution without TypeScript errors.

- [x] **Step 3: Inspect the final commit boundary**

Run: `git status --short; git log --oneline -3`

Expected: clean working tree and separate test/implementation commits for the QQ reconnect fix.
