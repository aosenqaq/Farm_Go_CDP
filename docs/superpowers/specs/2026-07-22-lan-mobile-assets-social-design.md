# LAN Mobile Assets And Social Design

## Goal

Make `资产与土地` and `好友社交` practical on phone-width LAN WebUI
screens while preserving the desktop experience and every existing action.
Fix mobile LAN crop, land, warehouse, and atlas images by serving registered
local game-config files from the LAN HTTP server as same-origin resources.

## Scope And Boundaries

The responsive work applies only under `.app-shell-remote` at
`max-width: 760px`. Desktop markup, desktop grids, action callbacks, polling,
storage, and protocol calls do not change. The four existing asset tabs stay
on one row. This work does not create secondary navigation or replace existing
business APIs.

The visual direction is the approved compact operational layout shown in the
visual companion at
`.superpowers/brainstorm/assets-social-mobile-20260722/content/assets-social-mobile-preview.html`.
It uses labelled commands, stable two-column visual grids where inspection
matters, and bounded bottom sheets rather than full-viewport dialogs.

## Same-Origin Game Images

### Cause

The LAN server correctly sends a strict CSP with `default-src 'self'`. Several
asset payload paths still emit `data:` image URLs, and land details can prefer
a raw runtime `imageUrl` over a locally registered stage path. The browser
therefore blocks those images, then `FallbackImage` changes the failed source
to `/logo.png`. The repeated Farm Go mark is a fallback, not the crop image.

The desktop Wails asset middleware already serves registered local images at
`/farm-assets/<sha256>`, but the independent LAN server does not currently
dispatch that route.

### Routing Design

`internal/lanaccess.Options` gains an optional narrowly scoped
`GameAssets http.Handler`; the server stores it separately from the embedded
frontend `Assets fs.FS`. `server.ServeHTTP` routes only the
`/farm-assets/` prefix to that handler before static-file fallback. When no
handler is present, this prefix returns `404`.

`lanAccessManagerOptions` and `lanAccessManager` carry the handler. `App`
passes a small handler that delegates to `farm.ServeLocalGameConfigImage`.
The latter can serve only IDs previously registered by
`gameConfigLocalImageURL`: the ID maps to a canonical file under the
game-config root and never accepts an arbitrary filesystem path. The LAN
route receives the existing security headers and remote-address filtering.
It has the same public static-resource semantics as the frontend bundle, so
images do not require an authenticated API session; unknown IDs, nested paths,
and methods other than `GET` return `404`. A successful local-image response
may retain its immutable cache header.

The CSP remains `default-src 'self'`; it is not widened to permit `data:`,
remote hosts, inline scripts, or external assets.

### Payload Normalization

All game-config image URLs emitted to LAN-capable views use
`gameConfigLocalImageURL(resolver.root, path)`:

- `resolvePlantMainImage`, atlas crop items, and atlas mutation
  `meta.ImagePath`.
- Warehouse default and mapped item image paths.
- Local mutation crop and mutation icon paths, which already use the same
  local store when a config path exists.

Land-detail runtime images are accepted only when they are already a valid
`/farm-assets/` path. Raw `data:`, `http:`, `https:`, and unrelated relative
URLs are ignored and the resolver chooses the matching local stage image.
The same policy applies to explicit mutation icon URLs: use the local config
icon resolver rather than emitting raw runtime Base64 or remote URLs. Empty
or unavailable configuration images remain empty and display the existing
textual placeholder. `FallbackImage` remains a last resort for an actual
same-origin image fetch failure; it is no longer the expected mobile path.

## Asset And Land Mobile Layout

### Shared Page Shell

The remote mobile asset view is one vertical scroll area. The four tab labels
stay readable in a compact equal-width row; icons remain supplemental. Header
actions wrap into touch-sized labelled controls without horizontal page
scrolling. Existing loading, error, disabled, and empty states remain in the
current panel rather than opening another view.

### Land Details

Land cards form a two-column grid. Every card has a fixed image area and
always shows land number, crop name, state, and maturity text. Seasonal,
occupancy, and work-state metadata wraps below without expanding neighboring
image areas. Per-land `无机`, `有机`, and `铲除` commands remain labelled and
touchable.

`一键催熟` and bulk fertilizer controls use the existing action flow, but the
land rush surface becomes a bottom sheet: bounded near `82dvh`, rounded at
the top, explicit close in its fixed header, scrollable body, and fixed
cancel/confirm footer. Background land polling and one-second maturity text
continue unchanged.

### Warehouse

The top action bar keeps labelled `刷新仓库`, `出售选中`, `自动出售设置`, and
`出售记录` commands. Category filters are one horizontally scrollable chip
row. The desktop table becomes concise list rows: selection control, name and
category, quantity, plus sale availability/value. The row image is optional;
no layout assumes it exists.

Auto-sell settings and sale records are bottom sheets with a fixed header and
footer. Settings keep enabled state, interval, category choices, save and
cancel. Records keep their date/mode filtering and item detail but use stacked
record rows instead of a wide table. Only the interior list/body scrolls.

### Atlas

The crop/mutation selector remains a two-part segmented control. Atlas items
are a two-column image grid with a stable art aspect ratio and a compact
bottom region for name and unlocked/locked state. Long names truncate or wrap
inside their card. Summary and refresh/purchase controls remain visible
without pushing the grid off screen.

The locked-seed purchase confirmation uses the same bounded-sheet contract:
fixed header with close, independently scrollable purchase list, and fixed
cancel/confirm footer. Existing preview, disabled, error, and request
semantics are unchanged.

## Social Mobile Layout

The top actions become a two-column labelled grid for `刷新好友`, `好友功能`,
`排行榜`, and `导入导出`. The six summary metrics form a compact three-by-two
grid. Search, sorting, and filters remain directly on the page; filter chips
scroll horizontally as one row rather than forcing a dense multi-row toolbar.

The desktop friend table becomes a vertical set of summary rows. Each row
shows name, GID and level, current rule/protection state, available actions,
and the existing `进入`, `偷菜`, `帮忙`, `黑名单`, and `白名单` operations. The
actions retain visible names on mobile so users do not have to infer an icon.

All social child functions receive the same remote mobile sheet contract:
`好友功能`, rule settings, batch rule removal, dog-guard scan, protocol
blacklist, God ranking, import/export, and `SocialRankingDialog`. A sheet is
bottom aligned, capped between roughly `72dvh` and `86dvh` according to its
content, with a fixed header/explicit close and a fixed action footer whenever
it has commit or batch actions. Its main list or form is the only scrollable
area. Ranking keeps tab/date/view controls above its independently scrolling
rows; protocol blacklist keeps selection and unblocking actions reachable;
import/export stacks its action grid and text areas vertically. No child
function loses existing fields, batch selection, import/export behavior, or
backend error state.

## Implementation Boundaries

- `internal/lanaccess/server.go` and tests: add the optional local-image
  handler route with route/method/security-header coverage.
- `lan_access.go`, `app.go`, and LAN manager tests: inject the handler without
  importing or exposing filesystem paths from the LAN package.
- `internal/farm/gameconfig.go` and focused game-config tests: replace the
  remaining Base64 image emitters and normalize raw runtime land/mutation image
  URLs to registered local paths.
- `frontend/src/views/AssetsLandView.tsx`, `SocialView.tsx`,
  `social/SocialRankingDialog.tsx`, and their tests: preserve behavior while
  adding semantic mobile-local markup/classes where needed for compact rows
  and fixed sheet regions.
- `frontend/src/style.css`: add a final, dedicated
  `.app-shell-remote` mobile section for the asset and social page rules.
  It overrides the older generic remote rule that makes most dialogs
  `100dvh`; asset/social sheets must never inherit that full-screen behavior.

## Testing And Acceptance

Backend tests first prove the image contract:

1. A raw land `data:` URL resolves to a `/farm-assets/` local stage URL.
2. Crop atlas, mutation atlas, and warehouse image fields emit local asset
   URLs, never `data:image/`.
3. An ID registered by the local image store is served through the LAN handler
   with correct bytes/MIME and security headers; unknown IDs, nested paths,
   POST, and disallowed remote addresses fail safely.

Frontend tests cover the responsive contract in the existing source/render
style: two-column land/atlas grids, compact warehouse/friend rows, visible
text labels for mobile actions, mobile sheet bounds and header/footer regions,
and each social child dialog. Image render cases use `/farm-assets/` URLs and
assert that no `data:image/` appears in LAN-capable asset payload markup.

Verification runs focused Go and Vitest files, then `go test ./...`,
`npm test`, and `npm run build` in `frontend`. Manual LAN verification at a
360px-wide phone viewport must load land and atlas images without logo
fallbacks; inspect every sheet with enough data to scroll and confirm close,
cancel, save, batch, and confirm controls remain reachable. A desktop pass
confirms original desktop layouts are unchanged.

## Non-Goals

- Do not loosen CSP or proxy arbitrary image URLs.
- Do not expose game-config filesystem paths, add a generic LAN file server,
  or require an API session just to load the already registered static image.
- Do not change farm, warehouse, social, ranking, or rule business behavior.
- Do not redesign the desktop asset/social pages or add a second navigation
  level for mobile.
