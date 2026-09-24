# LAN Mobile Assets And Social Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver same-origin game images and a compact, fully operable phone layout for the LAN WebUI asset/land and social pages.

**Architecture:** The LAN server receives an explicit handler for only registered `/farm-assets/` images; the game-config resolver emits those URLs instead of Base64 or remote runtime sources. The React components retain their data flow and desktop markup while exposing small mobile-only row/sheet structures, with scoped remote CSS overriding the legacy full-screen-dialog rule.

**Tech Stack:** Go 1.22, `net/http`, React, TypeScript, Vite, Vitest, CSS media queries, Lucide React.

---

## File Structure

- `internal/farm/gameconfig.go`: normalize all emitted game-config image URLs to the registered local-image store.
- `internal/farm/gameconfig_land_assets_test.go`, `internal/farm/gameconfig_test.go`, `internal/farm/local_image_store_test.go`: cover land, atlas, warehouse, and registered-store URLs.
- `internal/lanaccess/server.go`, `internal/lanaccess/server_test.go`: accept a narrow game-image handler and retain LAN security at the new route.
- `lan_access.go`, `lan_access_test.go`, `app.go`: inject the game-image handler at the application boundary without coupling `internal/lanaccess` to `farm`.
- `frontend/src/views/AssetsLandView.tsx`, `frontend/src/views/AssetsLandView.test.tsx`: mobile warehouse rows and sheet landmarks while preserving land/atlas callbacks.
- `frontend/src/views/SocialView.tsx`, `frontend/src/views/SocialView.test.tsx`, `frontend/src/views/social/SocialRankingDialog.tsx`: explicit mobile action labels and fixed dialog regions for every social child flow.
- `frontend/src/style.css`: final scoped `.app-shell-remote` phone rules for asset/social layouts and bounded sheets.

### Task 1: Normalize Game-Config Image Payloads

**Files:**
- Modify: `internal/farm/gameconfig.go:917-920, 1366-1369, 1822-1834, 2715-2726, 2847-2926`
- Modify: `internal/farm/gameconfig_land_assets_test.go`
- Modify: `internal/farm/gameconfig_test.go`

- [ ] **Step 1: Write failing land/atlas/warehouse image tests**

Add focused tests that construct a temporary game-config root with a tiny PNG and assert every returned image URL begins with `/farm-assets/` and contains no `data:image/`:

```go
func TestBuildRuntimeLandDetailsRejectsRuntimeDataImageURL(t *testing.T) {
    root := writeLandAssetConfig(t)
    writeTinyPNG(t, filepath.Join(root, "plant_images", "stages", "作物", "白萝卜", "白萝卜_02_发芽.png"))

    payload := BuildRuntimeLandDetailsForRoot(map[string]any{
        "lands": []any{map[string]any{
            "landId": 1, "seedId": 20002, "plantName": "白萝卜",
            "currentStage": 2, "status": "growing",
            "imageUrl": "data:image/png;base64,blocked",
        }},
    }, root)

    got := payload.Lands[0].ImageURL
    if !strings.HasPrefix(got, "/farm-assets/") || strings.Contains(got, "data:image/") {
        t.Fatalf("ImageURL=%q", got)
    }
}
```

Add matching atlas crop, atlas mutation metadata, mapped warehouse, and default-warehouse assertions to existing fixture tests. Include a mutation-icon case with a runtime `data:` icon and assert the local mutation icon URL wins.

- [ ] **Step 2: Run the focused backend tests to verify failure**

Run:

```powershell
go test ./internal/farm -run 'Test(BuildRuntimeLandDetailsRejectsRuntimeDataImageURL|.*Atlas.*Image|.*Warehouse.*Image)' -count=1
```

Expected: FAIL because the current resolver emits Base64, or the runtime `data:` field wins.

- [ ] **Step 3: Replace Base64/unsafe runtime emissions with registered URLs**

In `gameconfig.go`, add a narrow predicate and use it only for already-issued local paths:

```go
func isLocalGameImageURL(value string) bool {
    return strings.HasPrefix(strings.TrimSpace(value), localImageAssetPrefix)
}
```

Set a land image using the runtime value only when that predicate passes:

```go
imageURL := resolver.resolvePlantStageImage(seedID, plantName, currentStage, phaseName)
if runtimeImageURL := stringFromMap(item, "imageUrl"); isLocalGameImageURL(runtimeImageURL) {
    imageURL = runtimeImageURL
}
```

Apply `gameConfigLocalImageURL(r.root, path)` to `resolvePlantMainImage`,
atlas mutation `meta.ImagePath`, the warehouse default path, and every mapped
warehouse path. Change mutation-type construction to call
`r.resolveMutationIcon(name)` directly; delete `mutationExplicitIconURL`,
`imageFileDataURL`, and the now-unused `encoding/base64` import. Preserve
the existing empty-string fallback whenever no local file exists.

- [ ] **Step 4: Run and inspect focused game-config tests**

Run:

```powershell
go test ./internal/farm -run 'Test(BuildRuntimeLandDetails|.*Atlas.*|.*Warehouse.*|.*LocalImage)' -count=1
```

Expected: PASS. Verify the changed tests contain no `data:image/` expectation.

- [ ] **Step 5: Commit the resolver contract**

```powershell
git add internal/farm/gameconfig.go internal/farm/gameconfig_land_assets_test.go internal/farm/gameconfig_test.go
git commit -m "fix: emit local URLs for farm asset images"
```

### Task 2: Serve Registered Images From the LAN Server

**Files:**
- Modify: `internal/lanaccess/server.go:32-135`
- Modify: `internal/lanaccess/server_test.go`
- Modify: `internal/farm/local_image_store_test.go`
- Modify: `lan_access.go:43-76, 318-347`
- Modify: `lan_access_test.go`
- Modify: `app.go:342-350`

- [ ] **Step 1: Write failing LAN route tests**

In `internal/lanaccess/server_test.go`, create a recording `http.HandlerFunc`
and supply it as `GameAssets`. Assert a private-LAN `GET /farm-assets/id`
reaches it, keeps the CSP header, and permits the handler's immutable cache
header. Assert `POST /farm-assets/id`, `/farm-assets/id/extra`, and a
public remote address return `404` or `403` without invoking the handler.

In `internal/farm/local_image_store_test.go`, first prove that an unknown
registered URL remains `404` and a previously registered URL is GET-only with
the expected MIME/body. In `lan_access_test.go`, register a tiny config image through
`farm.NewLocalImageStore(root).URLFor(path)`, build the manager with a handler
delegating to `farm.ServeLocalGameConfigImage`, and assert a request to its
server handler returns the PNG bytes and `image/png`, while an unknown ID is
`404`.

- [ ] **Step 2: Run the route tests to verify failure**

Run:

```powershell
go test ./internal/lanaccess -run TestServerServesGameAssets -count=1
go test . -run TestLANAccessServesRegisteredGameConfigImage -count=1
```

Expected: FAIL because `lanaccess.Options` has no `GameAssets` field and
the LAN switch falls through to embedded static content.

- [ ] **Step 3: Add the explicit, GET-only handler boundary**

Extend the LAN options and server state:

```go
type Options struct {
    Config     Config
    Auth       *Authenticator
    Assets     fs.FS
    GameAssets http.Handler
    RPC        RPCDispatcher
    Poll       http.Handler
}

func (s *server) serveGameAsset(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet || s.gameAssets == nil ||
        strings.TrimPrefix(r.URL.Path, "/farm-assets/") == "" ||
        strings.Contains(strings.TrimPrefix(r.URL.Path, "/farm-assets/"), "/") {
        http.NotFound(w, r)
        return
    }
    s.gameAssets.ServeHTTP(w, r)
}
```

Route the prefix before the generic static fallback. Add
`GameAssets http.Handler` to the manager options/state and pass it to
`lanaccess.NewServer`. In `NewApp`, inject:

```go
GameAssets: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if !farm.ServeLocalGameConfigImage(w, r) {
        http.NotFound(w, r)
    }
}),
```

Do not import `farm` from `internal/lanaccess`; do not add filesystem-path
parameters, authentication exceptions, or a generic file-serving route.

- [ ] **Step 4: Run server and root route tests**

Run:

```powershell
go test ./internal/lanaccess -run 'TestServer.*GameAssets' -count=1
go test . -run TestLANAccessServesRegisteredGameConfigImage -count=1
go test ./internal/farm -run TestServeLocalGameConfigImage -count=1
```

Expected: PASS with CSP present, only registered IDs returning bytes, and
public remote addresses still rejected.

- [ ] **Step 5: Commit the LAN image route**

```powershell
git add internal/lanaccess/server.go internal/lanaccess/server_test.go lan_access.go lan_access_test.go app.go
git commit -m "fix: serve registered farm images over LAN"
```

### Task 3: Add Asset-Page Mobile Structures Without Changing Actions

**Files:**
- Modify: `frontend/src/views/AssetsLandView.tsx:823-1930`
- Modify: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Write failing component render/source tests**

Add tests that render the land, warehouse, atlas, sale-record, and purchase
surfaces and assert these mobile-specific class contracts:

```ts
expect(html).toContain('warehouse-mobile-list');
expect(html).toContain('warehouse-mobile-row');
expect(html).toContain('warehouse-record-mobile-list');
expect(html).toContain('atlas-card-grid');
expect(source).toContain('aria-label="关闭出售记录"');
expect(source).toContain('aria-label="关闭确认购买"');
```

Keep the existing tests that prove selected-key selling, auto-sell persistence,
and atlas purchase callbacks. Add a render test with `/farm-assets/crop` and
`/farm-assets/mutation` sources that rejects `data:image/`.

- [ ] **Step 2: Run the focused frontend test to verify failure**

Run:

```powershell
Set-Location frontend
npm test -- src/views/AssetsLandView.test.tsx
```

Expected: FAIL because the mobile list classes and close labels do not yet
exist.

- [ ] **Step 3: Render concise mobile list rows and sheet landmarks**

Keep the current desktop `table.asset-table`, add
`warehouse-desktop-table`, and render a sibling mobile list from the same
`filteredItems`, `selectedSet`, `toggleItem`, and `submitSell` state:

```tsx
<div className="warehouse-mobile-list">
  {filteredItems.map((item) => (
    <label className="warehouse-mobile-row" key={item.id}>
      <input
        aria-label={`选择${item.name}`}
        checked={selectedSet.has(item.id)}
        disabled={!item.canSell || item.locked}
        type="checkbox"
        onChange={() => toggleItem(item)}
      />
      <span className="warehouse-mobile-copy">
        <strong>{item.name}</strong>
        <small>{item.categoryLabel} · {item.canSell ? '可出售' : item.locked ? '锁定' : '不可出售'}</small>
      </span>
      <span className="warehouse-mobile-values">x{item.count}<em>{item.estimatedSellPrice > 0 ? `预计 ${item.estimatedSellPrice}` : '-'}</em></span>
    </label>
  ))}
</div>
```

Give land rush, auto-sell, records, and atlas purchase surfaces explicit
header/body/footer classes and close buttons with Chinese `aria-label`
values. Render sale records as a mobile list from the existing `records`
array while retaining the desktop table for desktop widths. Do not duplicate
requests, alter selection IDs, or change save/sell/confirm handlers.

- [ ] **Step 4: Run focused asset tests**

Run:

```powershell
Set-Location frontend
npm test -- src/views/AssetsLandView.test.tsx
```

Expected: PASS. Confirm existing direct action, polling, sale, and purchase
tests remain green.

- [ ] **Step 5: Commit asset component structure**

```powershell
git add frontend/src/views/AssetsLandView.tsx frontend/src/views/AssetsLandView.test.tsx
git commit -m "feat: compact LAN mobile asset content"
```

### Task 4: Add Social Mobile Structures And Reachable Child-Flow Controls

**Files:**
- Modify: `frontend/src/views/SocialView.tsx:688-1297`
- Modify: `frontend/src/views/social/SocialRankingDialog.tsx:281-368`
- Modify: `frontend/src/views/SocialView.test.tsx`
- Modify: `frontend/src/views/social/SocialRankingDialog.test.tsx`

- [ ] **Step 1: Write failing social render tests**

Render a friend row and assert every operational action has a mobile-visible
label node, while the existing button `title` remains:

```ts
expect(html).toContain('social-icon-action-label');
expect(html).toContain('>进入<');
expect(html).toContain('>偷菜<');
expect(html).toContain('>帮忙<');
expect(html).toContain('>黑名单<');
expect(html).toContain('>白名单<');
```

Add source/render assertions for `social-dialog-footer`,
`aria-label="关闭好友功能"`, and a ranking sheet with
`social-ranking-dialog-body` plus `data-ranking-scroll`. Cover rules,
batch removal, dog-guard, protocol blacklist, God ranking, and import/export
so every child flow keeps its input/list/action content.

- [ ] **Step 2: Run focused social tests to verify failure**

Run:

```powershell
Set-Location frontend
npm test -- src/views/SocialView.test.tsx src/views/social/SocialRankingDialog.test.tsx
```

Expected: FAIL because the mobile action labels, dialog footer landmark, and
explicit close labels are missing from the generic social dialog.

- [ ] **Step 3: Refactor the social dialog shell around fixed regions**

Extend `Dialog` with optional `footer?: ReactNode` and an ID derived from
the title. It must render a named close control and a footer only when supplied:

```tsx
<section className={wide ? 'social-dialog wide' : 'social-dialog'} role="dialog" aria-modal="true" aria-labelledby={titleID}>
  <header>
    <h2 id={titleID}>{title}</h2>
    <button className="icon-button light" type="button" onClick={onClose} aria-label={`关闭${title}`} title="关闭"><X size={16} /></button>
  </header>
  <div className="social-dialog-body">{children}</div>
  {footer && <footer className="social-dialog-footer">{footer}</footer>}
</section>
```

Move save, batch removal, and protocol-unblock commands into that footer where
they commit data; leave informational dialog bodies and the dog scan command
in their body toolbar. Retain disabled and loading states. Update `IconAction`
to render `<span className="social-icon-action-label">{title}</span>` after
its icon. Keep desktop CSS hiding that label.

Give the ranking dialog its existing explicit close label, fixed controls area,
and list-only scroll region; do not change cache keys, pagination, view-mode
persistence, or protocol-block callback behavior.

- [ ] **Step 4: Run focused social tests**

Run:

```powershell
Set-Location frontend
npm test -- src/views/SocialView.test.tsx src/views/social/SocialRankingDialog.test.tsx
```

Expected: PASS. Verify batch, ranking pagination, import/export, and existing
friend action tests still pass.

- [ ] **Step 5: Commit social component structure**

```powershell
git add frontend/src/views/SocialView.tsx frontend/src/views/SocialView.test.tsx frontend/src/views/social/SocialRankingDialog.tsx frontend/src/views/social/SocialRankingDialog.test.tsx
git commit -m "feat: make LAN social actions mobile-readable"
```

### Task 5: Add Scoped Remote-Mobile Asset CSS

**Files:**
- Modify: `frontend/src/style.css: after the existing .app-shell-remote max-width 760px rules`
- Modify: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Write failing CSS contract tests**

Add source assertions for a dedicated final mobile block that includes all of:

```ts
expect(css).toContain('.app-shell-remote .warehouse-mobile-list');
expect(css).toContain('.app-shell-remote .land-grid');
expect(css).toContain('grid-template-columns: repeat(2, minmax(0, 1fr))');
expect(css).toContain('.app-shell-remote .land-rush-drawer');
expect(css).toContain('.app-shell-remote .atlas-purchase-dialog');
expect(css).toContain('max-height: 82dvh');
```

- [ ] **Step 2: Run the asset test to verify failure**

Run:

```powershell
Set-Location frontend
npm test -- src/views/AssetsLandView.test.tsx
```

Expected: FAIL because the final scoped remote-mobile asset block is absent.

- [ ] **Step 3: Add the responsive layout and sheet rules**

Append a later `@media (max-width: 760px)` block that is scoped to
`.app-shell-remote`. Use this shape so it overrides the older generic
`100dvh` rule without changing desktop styles:

```css
.app-shell-remote .warehouse-desktop-table { display: none; }
.app-shell-remote .warehouse-mobile-list { display: grid; gap: 8px; }
.app-shell-remote .land-grid,
.app-shell-remote .atlas-card-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.app-shell-remote .land-card-art,
.app-shell-remote .atlas-card-art { aspect-ratio: 1 / 1; min-height: 0; }
.app-shell-remote .land-rush-drawer,
.app-shell-remote .warehouse-settings-dialog,
.app-shell-remote .warehouse-records-dialog,
.app-shell-remote .atlas-purchase-dialog {
  align-self: end;
  width: 100%;
  min-height: 0;
  height: min(82dvh, 720px);
  max-height: 82dvh;
  border-radius: 14px 14px 0 0;
}
```

Give each sheet a three-row `header / minmax(0, 1fr) / footer` grid and
`overflow: auto` only on its named body/list. Keep warehouse category chips
horizontally scrollable. Use text wrapping/ellipsis inside cards and never
introduce a page-level horizontal overflow.

- [ ] **Step 4: Run the asset test and production build**

Run:

```powershell
Set-Location frontend
npm test -- src/views/AssetsLandView.test.tsx
npm run build
```

Expected: PASS and a successful Vite build.

- [ ] **Step 5: Commit remote asset CSS**

```powershell
git add frontend/src/style.css frontend/src/views/AssetsLandView.test.tsx
git commit -m "style: bound LAN mobile asset sheets"
```

### Task 6: Add Scoped Remote-Mobile Social CSS

**Files:**
- Modify: `frontend/src/style.css: final remote-mobile section`
- Modify: `frontend/src/views/SocialView.test.tsx`
- Modify: `frontend/src/views/social/SocialRankingDialog.test.tsx`

- [ ] **Step 1: Write failing social CSS contract tests**

Assert the stylesheet contains a social-specific remote mobile override that
does not use `100dvh` for social surfaces:

```ts
expect(css).toContain('.app-shell-remote .social-header .header-actions');
expect(css).toContain('.app-shell-remote .social-summary-strip');
expect(css).toContain('.app-shell-remote .social-friend-row');
expect(css).toContain('.app-shell-remote .social-dialog');
expect(css).toContain('.app-shell-remote .social-dialog-footer');
expect(css).toContain('height: min(84dvh, 720px)');
```

- [ ] **Step 2: Run focused social tests to verify failure**

Run:

```powershell
Set-Location frontend
npm test -- src/views/SocialView.test.tsx src/views/social/SocialRankingDialog.test.tsx
```

Expected: FAIL because the scoped grid, row, and sheet rules do not exist.

- [ ] **Step 3: Implement the remote social layout and sheet overrides**

In the same final remote breakpoint, add rules equivalent to:

```css
.app-shell-remote .social-header .header-actions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); width: 100%; }
.app-shell-remote .social-summary-strip { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.app-shell-remote .social-friend-head { display: none; }
.app-shell-remote .social-friend-row { grid-template-columns: minmax(0, 1fr) auto; gap: 6px; min-height: 0; padding: 10px 0; }
.app-shell-remote .social-icon-action-label { display: inline; }
.app-shell-remote .social-dialog-backdrop { align-items: end; padding: 0 8px; }
.app-shell-remote .social-dialog { width: 100%; min-height: 0; height: min(84dvh, 720px); max-height: 84dvh; border-radius: 14px 14px 0 0; }
.app-shell-remote .social-dialog-body { min-height: 0; overflow: auto; }
.app-shell-remote .social-dialog-footer { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); }
```

Stack import/export panels and text areas, make social feature buttons and
protocol controls fit their sheets, and keep only the ranking list scrollable
inside the ranking shell. Give controls a stable minimum height and preserve
the desktop icon-only row actions by hiding the label outside the remote media
query.

- [ ] **Step 4: Run focused social tests and Vite build**

Run:

```powershell
Set-Location frontend
npm test -- src/views/SocialView.test.tsx src/views/social/SocialRankingDialog.test.tsx
npm run build
```

Expected: PASS and no TypeScript or CSS build errors.

- [ ] **Step 5: Commit remote social CSS**

```powershell
git add frontend/src/style.css frontend/src/views/SocialView.test.tsx frontend/src/views/social/SocialRankingDialog.test.tsx
git commit -m "style: bound LAN mobile social sheets"
```

### Task 7: Full Regression And Visual Verification

**Files:**
- Verify only.

- [ ] **Step 1: Run all automated checks**

Run:

```powershell
go test ./...
Set-Location frontend
npm test
npm run build
```

Expected: every command exits `0`.

- [ ] **Step 2: Start the WebUI and verify the mobile contract**

Run:

```powershell
Set-Location frontend
npm run dev -- --host 0.0.0.0
```

At a 360px-wide remote session, inspect all four asset tabs and the social
page. Confirm crop/land/warehouse/atlas `img` sources use `/farm-assets/`,
land and atlas images do not show the Farm Go fallback, and the following
sheets retain visible close/cancel/save/confirm controls while their body
scrolls: land rush, auto-sell, sale records, atlas purchase, friend features,
rule settings, batch rules, dog guard, protocol blacklist, God ranking,
import/export, and ranking.

- [ ] **Step 3: Verify desktop non-regression and repository hygiene**

At a desktop viewport, confirm the existing asset tables/grids and social
table remain desktop layouts. Then run:

```powershell
git diff --check
git status --short
```

Expected: no whitespace errors; only intentional task changes before the
final commit.

- [ ] **Step 4: Commit final verification note only if a documentation update is needed**

Do not create a documentation-only commit when no verification document
changed. Otherwise, add the exact verification note file and commit it with:

```powershell
git add docs/superpowers
git commit -m "docs: verify LAN mobile assets and social UI"
```
