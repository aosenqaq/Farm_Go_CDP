# Land Details Memory Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove repeated local-image Base64 allocation and full land snapshots from five-second land-detail polling, without altering card visuals or land actions.

**Architecture:** A farm image registry maps canonical game-config image paths to opaque `/farm-assets/<id>` URLs and lazily caches bytes. Wails AssetServer middleware serves those URLs. A revisioned app snapshot creates deltas; React merges changed rows through a disposable polling controller.

**Tech Stack:** Go 1.25, Wails v2, React 18, TypeScript, Vitest, Go `httptest`, PowerShell.

---

### Task 1: Local Image Resource URLs

**Files:**
- Create: `internal/farm/local_image_store.go`
- Create: `internal/farm/local_image_store_test.go`
- Modify: `internal/farm/gameconfig.go`
- Modify: `internal/farm/gameconfig_land_assets_test.go`
- Modify: `main.go`

- [ ] Write failing tests for a registered local PNG: repeated `GET` calls to the same `store.URLFor(path)` must return `200`, identical bytes, `image/png`, and `Cache-Control: public, max-age=31536000, immutable`. Unknown IDs and `/farm-assets/../Plant.json` must return `404`.

- [ ] Run `go test ./internal/farm -run TestLocalImageStore -count=1` and confirm it fails because `NewLocalImageStore` is absent.

- [ ] Implement `LocalImageStore` with `sync.RWMutex`, canonical root, `byPath`, and `byID`. `URLFor` must reject a path outside the root, hash its canonical path with SHA-256, register the ID, and return `/farm-assets/<hex-id>`. `ServeHTTP` must accept only `GET`, serve only registered IDs, read each registered file once on first request, retain bytes/MIME by canonical path, and never derive a filesystem path from an HTTP request.

- [ ] Keep `imageFileDataURL` for non-land callers. Add a game-config-root keyed registry helper and change only `landImageResolver.resolvePlantStageImage`, `resolveMutationIcon`, and `resolveMutationPlantImage` to call `URLFor(resolvedPath)`. Preserve runtime-provided non-local image URLs unchanged.

- [ ] Add AssetServer middleware in `main.go` that calls `farm.ServeLocalGameConfigImage(w, r)` for `/farm-assets/` and delegates every other request to `next`, including the dev-server path.

- [ ] Update `gameconfig_land_assets_test.go` to expect `/farm-assets/` and reject `data:image/`; add served crop/mutation MIME and byte assertions.

- [ ] Run `go test ./internal/farm -run 'Test(LocalImageStore|BuildRuntimeLandDetails)' -count=1`; expect exit code `0`.

- [ ] Commit: `git add main.go internal/farm/local_image_store.go internal/farm/local_image_store_test.go internal/farm/gameconfig.go internal/farm/gameconfig_land_assets_test.go; git commit -m "feat: serve land images through cached local URLs"`.

### Task 2: Incremental Backend Poll API

**Files:**
- Modify: `internal/farm/gameconfig.go`
- Modify: `app.go`
- Modify: `app_test.go`
- Modify: `frontend/wailsjs/go/main/App.js`
- Modify: `frontend/wailsjs/go/main/App.d.ts`
- Modify: `frontend/wailsjs/go/models.ts`

- [ ] Write failing `app_test.go` cases with sequential fake `gameCtl.getFarmStatus` responses. `FarmLandDetails()` must return a revision. `FarmLandDetailsSince(revision)` must return `full: false` and `lands: []` when only `matureInSec` decreases; when `currentStage` changes from `2` to `3`, it must return only land `1` and a different `/farm-assets/` image URL.

- [ ] Run `go test . -run 'TestAppLandDetails(Delta|ReturnsChangedStage)' -count=1`; expect compile failure for the new method and revision field.

- [ ] Extend `LandDetailsPayload` with `Revision string`. Add `LandDetailsDeltaPayload` with `Full`, `Revision`, top-level status/message/farm type/actions/error, `Lands []LandDetailsItem`, and `RemovedLandIDs []int`; initialize empty slices as `[]`.

- [ ] Add a deterministic `LandStateSignature` over rendered non-countdown fields: land identity, crop/mutation identity, image URLs, stage/status, seasons, occupancy, work flags, and harvest status. Exclude `MatureInSec`, `MatureEtaText`, and generated `MatureAtMs`.

- [ ] Add `landDetailsMu`, `landDetailsRevision`, `landDetailsSnapshot`, and `landDetailsSigns` to `App`. Extract the runtime call into `readLandDetails()`. `FarmLandDetails` publishes a full snapshot; `FarmLandDetailsSince` returns a full snapshot for unknown revision and otherwise returns changed lands plus removed IDs. Retain a growing land's calculated `MatureAtMs` when its visual signature is unchanged.

- [ ] Add `clearLandDetailsSnapshot()` next to existing runtime-cache invalidation so reconnects force the next poll full.

- [ ] Add `FarmLandDetailsSince(revision)` to the generated-equivalent Wails JS/declaration files and add the matching `LandDetailsDeltaPayload` constructor to `models.ts` using the current payload conversion pattern.

- [ ] Run `go test . -run TestAppLandDetails -count=1`; expect exit code `0`. Commit: `git add app.go app_test.go internal/farm/gameconfig.go frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/models.ts; git commit -m "feat: return incremental land detail polls"`.

### Task 3: Frontend Delta Merge And Cleanup

**Files:**
- Create: `frontend/src/views/lib/landDetailsPolling.ts`
- Create: `frontend/src/views/lib/landDetailsPolling.test.ts`
- Modify: `frontend/src/views/AssetsLandView.tsx`
- Modify: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] Write a failing merge test: with lands `1` and `2`, a delta for only `2` must retain `lands[0]` object identity, give land `2` its new `/farm-assets/new` URL, and remove `removedLandIds`. With fake timers, start then stop a controller before a deferred request resolves; after ten seconds, assert no subsequent request or result callback occurs.

- [ ] Run `npm test -- src/views/lib/landDetailsPolling.test.ts`; expect module-not-found failure.

- [ ] Implement `mergeLandDetailsDelta(current, delta)` as a pure function: replace only changed rows, remove IDs, retain unchanged references, and replace the full snapshot when `delta.full`. Implement `createLandDetailsPolling(readDelta, apply, reportError)` with one interval, one in-flight flag, and an increasing generation. `stop()` clears interval and increments generation; async callbacks apply only when their captured generation remains current.

- [ ] Update `AssetsLandView`: first load, manual refresh, and post-action refresh call `FarmLandDetails()` and store its revision. The active land tab starts `FarmLandDetailsSince(revision)` through the controller, merges through functional `setLandDetails`, updates revision, and disposes on tab change/unmount. Guard initial and foreground reads by the same active generation. Keep the one-second countdown timer inside this land-only cleanup boundary.

- [ ] Replace the source-text polling assertion with behavioral tests. Add a land card render test with crop and mutation `/farm-assets/` sources that explicitly rejects `data:image/`.

- [ ] Run `npm test -- src/views/lib/landDetailsPolling.test.ts src/views/AssetsLandView.test.tsx`; expect exit code `0`. Commit: `git add frontend/src/views/lib/landDetailsPolling.ts frontend/src/views/lib/landDetailsPolling.test.ts frontend/src/views/AssetsLandView.tsx frontend/src/views/AssetsLandView.test.tsx; git commit -m "feat: stop land polling and merge incremental updates"`.

### Task 4: Repeatable WebView2 Memory Evidence

**Files:**
- Create: `scripts/measure-land-webview2-memory.ps1`
- Modify: `README.md`

- [ ] Implement a non-mutating sampler with `FarmGoProcessId`, `Samples`, `IntervalSeconds`, and `OutputPath`. Use `Get-CimInstance Win32_Process` to find descendants of Farm_Go, sum `PrivateMemorySize64` for `msedgewebview2` renderers, and output `sample,timestamp,rendererCount,privateBytes` CSV rows. Exit nonzero when no renderer is found; never manipulate processes.

- [ ] Document: `.\scripts\measure-land-webview2-memory.ps1 -FarmGoProcessId <pid> -Samples 12 -IntervalSeconds 5 -OutputPath .\land-memory.csv`. Acceptance is that, after initial image load, samples may fluctuate but must not rise every interval, and image requests are bounded by distinct visible URLs.

- [ ] Run `powershell -NoProfile -Command "[scriptblock]::Create((Get-Content -Raw scripts/measure-land-webview2-memory.ps1)) | Out-Null"`; expect exit code `0`. Commit: `git add scripts/measure-land-webview2-memory.ps1 README.md; git commit -m "test: document WebView2 land memory observation"`.

### Task 5: Full Verification

**Files:**
- Verify only.

- [ ] Run `go test ./...`; expect exit code `0`.

- [ ] Run `npm test` and `npm run build` from `frontend`; expect both exit code `0`.

- [ ] Run `wails dev`, open `资产与土地` then `土地详情`, wait for first image load, run the Task 4 sampler for twelve polls, and inspect that CSV does not rise continuously. Confirm no local land `img` source is Base64.

- [ ] Run `git diff HEAD~4..HEAD --check` and `git status --short`; expect no whitespace errors or unintentional files.
