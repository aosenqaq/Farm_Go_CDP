# CDP Handshake Status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Present the CDP `setupContext` fallback as handshake progress instead of an injection error for WeChat and YingYongBao.

**Architecture:** Add a neutral progress string to the shared runtime status DTO. The CDP link publishes it for the direct `Runtime.enable` fallback while leaving `LastError` empty; frontend normalization and the startup dialog render the progress with the selected in-progress treatment.

**Tech Stack:** Go, React, TypeScript, Vitest, Vite, Wails bindings.

---

### Task 1: Publish fallback as progress

**Files:**
- Modify: `internal/runtime/status.go`
- Modify: `internal/runtime/wmpf/link.go`
- Test: `internal/runtime/wmpf/debug_server_test.go`

- [ ] **Step 1: Write the failing Go test**

Add an assertion to the existing direct fallback test that the handshaking status has no `LastError` and has `ProgressDetail == "miniapp setupContext received; using direct CDP Runtime.enable fallback"`.

- [ ] **Step 2: Run the focused Go test to verify it fails**

Run: `go test ./internal/runtime/wmpf -run TestCDPLinkUsesDirectRuntimeEnableAfterSetupContext -count=1`

Expected: FAIL because `ProgressDetail` does not yet exist or the fallback still sets `LastError`.

- [ ] **Step 3: Implement the minimal status change**

Add `ProgressDetail string` to the shared status type and write the fallback message into that field in `connectRuntimeOnce`. Keep `LastError` empty for this path.

- [ ] **Step 4: Run the focused Go test to verify it passes**

Run: `go test ./internal/runtime/wmpf -run TestCDPLinkUsesDirectRuntimeEnableAfterSetupContext -count=1`

Expected: PASS.

### Task 2: Normalize and render handshake progress

**Files:**
- Modify: `frontend/src/lib/startupInjection.ts`
- Modify: `frontend/src/lib/startupInjection.test.ts`
- Modify: `frontend/src/components/StartupInjectionDialog.tsx`
- Modify: `frontend/src/components/StartupInjectionDialog.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write failing frontend tests**

Add one WeChat and one YingYongBao state test where `phase` is `handshaking`, `connected` is true, and `progressDetail` is the fallback message. Assert the headline is `正在建立可执行上下文`, tone is `info`, no error field is exposed, and the detail retains the compatibility explanation. Add a dialog render assertion for the progress class and `连接方式` label.

- [ ] **Step 2: Run the focused frontend tests to verify they fail**

Run: `npm test -- startupInjection.test.ts StartupInjectionDialog.test.tsx`

Expected: FAIL because the status type and dialog have no progress representation.

- [ ] **Step 3: Implement the selected dialog treatment**

Map `progressDetail` to the in-progress WMPF state, add a compact connection-method row, then add a restrained progress bar and info styling that reuse existing dialog dimensions.

- [ ] **Step 4: Run the focused frontend tests to verify they pass**

Run: `npm test -- startupInjection.test.ts StartupInjectionDialog.test.tsx`

Expected: PASS.

### Task 3: Verify delivery artifact

**Files:**
- Generated: `public/app/*`

- [ ] **Step 1: Run focused regression checks**

Run: `go test ./internal/runtime/wmpf -run TestCDPLinkUsesDirectRuntimeEnableAfterSetupContext -count=1` and `npm test -- startupInjection.test.ts StartupInjectionDialog.test.tsx`

Expected: both commands PASS.

- [ ] **Step 2: Build the frontend bundle**

Run: `pnpm run frontend:build`

Expected: TypeScript and Vite exit with code 0 and update `public/app/index.html` asset references.
