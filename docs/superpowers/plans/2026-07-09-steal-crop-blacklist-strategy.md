# Steal Crop Blacklist Strategy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply crop blacklist strategy consistently to automatic friend stealing, scheduler manual friend stealing, and social-list manual stealing.

**Architecture:** Introduce `internal/farm/stealrules` as a shared crop-rule decision package. Wire automation and social steal flows to call it after protocol inspection and before harvest submission.

**Tech Stack:** Go, existing `go test` test suites under `internal/farm/automation` and `internal/farm/social`.

---

### Task 1: Shared Crop Rule Decision

**Files:**
- Create: `internal/farm/stealrules/crop_rules.go`
- Test through automation/social package tests

- [ ] **Step 1: Add tests for strategy behavior**

Add tests that assert strategy `1` skips an entire farm when any collectable land is blacklisted and strategy `2` filters only matching lands.

- [ ] **Step 2: Verify tests fail**

Run: `go test ./internal/farm/automation ./internal/farm/social -run "FriendSteal|FriendActionSteal"`
Expected: failures for missing strategy behavior and missing social options support.

- [ ] **Step 3: Implement shared decision helper**

Create `stealrules.ApplyCropRules(config, inspect, landIDs)` returning filtered land IDs and whether to skip the whole farm.

- [ ] **Step 4: Verify helper through callers**

Run the same focused test command and fix compile/runtime issues.

### Task 2: Wire Automation and Social Entrypoints

**Files:**
- Modify: `internal/farm/automation/runtime_friend.go`
- Modify: `internal/farm/social/service.go`
- Modify: `internal/farm/social/actions.go`
- Modify: `app.go`

- [ ] **Step 1: Use shared helper in automation**

Replace the local filtering logic in `runFriendSteal` with the shared decision helper while preserving skip record behavior.

- [ ] **Step 2: Pass automation config into social service**

Extend `social.Options` with an automation config map, store it on `Service`, and populate it from `App.socialService()`.

- [ ] **Step 3: Apply rules in social manual steal**

After social protocol inspection, call the shared helper. On full skip, write a `steal_blacklist_skip` record and return a skip message. On partial filtering, submit only filtered land IDs.

- [ ] **Step 4: Run verification**

Run: `go test ./internal/farm/automation ./internal/farm/social`
Expected: both packages pass.
