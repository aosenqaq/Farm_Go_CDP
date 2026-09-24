# Land Details Memory Design

## Goal

Eliminate repeated Base64 image allocation and Go/Wails/WebView2 transfer during land-detail polling while preserving the current crop and mutation images, land actions, and countdown display.

## Scope

- Serve game-config images through stable, local Wails asset URLs.
- Cache image bytes by canonical game-config path in the backend.
- Make land polling incremental after an initial full snapshot.
- Stop polling and discard late poll results when the land-detail view is no longer active.
- Add regression coverage for cache reuse, incremental responses, image refresh on visual changes, and view cleanup.

Warehouse, crop-analysis, atlas, and non-land runtime APIs remain unchanged.

## Architecture

### Local image resources

The Wails AssetServer middleware owns the `/farm-assets/` path. It resolves only opaque image identifiers previously registered by the land-image resolver. An identifier maps to a canonical path below the installed game-config root; arbitrary filesystem paths are never accepted from a URL.

The resource registry holds entries keyed by canonical path. Each entry contains the stable URL, MIME type, file modification stamp, and image bytes. The first request reads the file; later requests reuse the cached bytes until the file stamp changes. Responses set a cacheable `Cache-Control` header because the opaque identifier is derived from the canonical path and modification stamp. Missing, malformed, and non-image IDs return `404`.

`BuildRuntimeLandDetailsForRoot` retains its existing image-resolution rules, but emits resource URLs for crop stages, mutation stages, and mutation icons. It does not read image bytes or encode Base64. Runtime-provided non-local URLs remain unchanged.

### Polling protocol

The first `FarmLandDetails` response is a full `LandDetailsPayload`. The frontend stores its revision token. Later calls pass that token to `FarmLandDetailsSince` and receive a `LandDetailsDeltaPayload` containing the current revision, top-level mutable state, and only land entries whose state signature changed.

A land state signature includes every rendered non-countdown field, including crop/mutation identity, current stage, image URLs, status, action flags, season, and maturity target. `matureAtMs` is stable after the initial snapshot, so the frontend's one-second local clock continues to render the countdown without polling full records. A changed crop or stage therefore includes the updated image URL; an unchanged land sends no image field or full record.

The backend stores the last normalized snapshot per app instance. A delta request with an unknown or stale revision receives a full snapshot, preserving recovery after frontend reloads or runtime reconnects.

### Frontend lifecycle

`AssetsLandView` performs the initial full read only while the land tab is active. Its five-second timer requests a delta and merges changed records by `landId`, retaining existing objects and image URLs for unchanged lands. Manual refresh and successful land actions request a fresh full snapshot so that user-triggered changes are immediately reflected.

Each land request is guarded by an active-view generation. The timer is cleared on tab change and component unmount; cleanup invalidates the generation, so late Wails promises cannot update React state after the view has left the page. The existing local one-second countdown timer follows the same lifecycle.

## Error Handling

- Asset failures leave the existing card fallback visual in place and do not affect land-state polling.
- A runtime poll error preserves the last full snapshot and reports an error only for a foreground refresh.
- A delta that cannot be applied triggers one full refresh rather than clearing visible land cards.
- Runtime-status cache invalidation clears the land snapshot/revision cache as well as the existing runtime call cache.

## Tests And Verification

- Go tests prove that a local stage/mutation path produces a stable non-Base64 URL, that repeated resource requests reuse cached bytes, and that modified files refresh the cached entry.
- Go tests prove full snapshot creation, empty delta for unchanged data, and a changed land entry with a new image URL when crop or stage changes.
- Frontend fake-timer tests prove that only the land tab polls, a delta updates only the changed card, and switching away or unmounting clears both timers and suppresses late results.
- Run `go test ./...`, `npm test`, and `npm run build`.
- Run the packaged or development Wails app against a stable farm status, capture WebView2 renderer private working set before and after at least twelve five-second polls, and verify it does not climb continuously while resource request counts stay bounded by unique images.

## Acceptance Criteria

- No locally resolved land image is serialized as a `data:image/...;base64` URL.
- Repeated land polls do not reread or retransmit unchanged images.
- A no-change poll contains no changed land records.
- Crop, mutation, or stage changes update the matching card image.
- Leaving land details stops its timers and late poll responses do not mutate its state.
- Existing crop and mutation image appearance remains unchanged.
