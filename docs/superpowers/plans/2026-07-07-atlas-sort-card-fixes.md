# Atlas Sort Card Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix atlas header compression, reference sorting, image/name mapping, and simplified reference-style cards.

**Architecture:** Keep the Wails/Go data boundary. Go performs atlas normalization from runtime rows plus local config mappings; React only renders normalized category/card data with compact controls.

**Tech Stack:** Go, React, TypeScript, Vitest, Wails generated bindings.

---

### Task 1: Backend Sorting And Mapping

- [ ] Add Go tests in `internal/farm/gameconfig_test.go` for mutation rows that lack names and rely on `mutation_mapping.json`.
- [ ] Run `go test ./internal/farm -run "Atlas"` and observe failure.
- [ ] Add mutation atlas metadata loading and reference sort keys in `internal/farm/gameconfig.go`.
- [ ] Run `go test ./internal/farm -run "Atlas"` and observe pass.

### Task 2: Frontend Compact Cards

- [ ] Add React tests in `frontend/src/views/AssetsLandView.test.tsx` for compact atlas toolbar and simplified card metadata.
- [ ] Run `npm test -- src/views/AssetsLandView.test.tsx` and observe failure.
- [ ] Update `frontend/src/views/AssetsLandView.tsx` and `frontend/src/style.css` to render the reference-style card grid and compact header.
- [ ] Run `npm test -- src/views/AssetsLandView.test.tsx` and observe pass.

### Task 3: Final Verification

- [ ] Run `go test -count=1 ./...`.
- [ ] Run `npm test` from `frontend`.
- [ ] Run `npm run build` from `frontend`.
