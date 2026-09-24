# Account Scoped Storage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add SQLite-backed account-scoped runtime data and a startup account confirmation gate.

**Architecture:** Keep launch-critical settings global, then switch runtime data to a confirmed `gid:<number>` scope after `gameCtl.getPlayerProfile` succeeds. The backend owns account scope, and the frontend blocks normal use until the account is confirmed.

**Tech Stack:** Go, modernc SQLite, Wails bindings, React, TypeScript, Vitest.

---

### Task 1: Storage Scope

**Files:**
- Modify: `internal/storage/migrations.go`
- Modify: `internal/storage/settings.go`
- Modify: `internal/storage/automation.go`
- Modify: `internal/storage/events.go`
- Test: `internal/storage/storage_test.go`

- [ ] Write failing tests for scoped settings and runtime events.
- [ ] Run `go test ./internal/storage` and verify the new tests fail because account scope is missing.
- [ ] Add `account_key` support, global/account-scoped settings helpers, runtime account metadata, and scoped event queries.
- [ ] Run `go test ./internal/storage` and verify the storage tests pass.

### Task 2: Backend Account APIs

**Files:**
- Modify: `app.go`
- Modify: `app_test.go`
- Modify: `frontend/wailsjs/go/main/App.js`
- Modify: `frontend/wailsjs/go/main/App.d.ts`

- [ ] Write failing tests for profile extraction and account confirmation.
- [ ] Run `go test .` and verify account API tests fail.
- [ ] Add `RuntimeAccount`, `IdentifyRuntimeAccount`, `ConfirmRuntimeAccount`, current account scope handling, and scoped service wiring.
- [ ] Update Wails frontend bindings for the new methods.
- [ ] Run `go test .` and verify backend tests pass.

### Task 3: Frontend Account Gate

**Files:**
- Create: `frontend/src/components/AccountIdentityDialog.tsx`
- Create: `frontend/src/components/AccountIdentityDialog.test.tsx`
- Modify: `frontend/src/components/AppShell.tsx`
- Modify: `frontend/src/components/AppShell.test.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/App.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] Write failing Vitest coverage for the account dialog and sidebar preview.
- [ ] Run `cd frontend && npm test -- AccountIdentityDialog AppShell` and verify the new tests fail.
- [ ] Implement account dialog, sidebar account/auth block, App ready-to-identify flow, and post-confirm refresh.
- [ ] Run `cd frontend && npm test -- AccountIdentityDialog AppShell App` and verify frontend tests pass.

### Task 4: Full Verification

**Files:**
- Verify all modified files.

- [ ] Run `go test ./...`.
- [ ] Run `cd frontend && npm test`.
- [ ] Run `cd frontend && npm run build`.
- [ ] Confirm `public/app/index.html` or the configured Wails build output is updated by the frontend build when applicable.
