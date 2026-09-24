# 图鉴一键购买未解锁种子 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将图鉴购买入口收敛为一个扫描后确认的按钮，确保用户在提交前看到每种待购种子和数量。

**Architecture:** 保持 Go 侧 `FarmAtlasBuyLockedPreview` 与 `FarmAtlasBuyLockedCrops` 的运行时校验和批量购买协议不变。`AssetsLandView` 保存扫描得到的购买计划和弹窗开关，扫描完成后将 `plan.purchases` 传给受控确认弹窗；确认弹窗只负责展示、取消和提交。

**Tech Stack:** React 18、TypeScript、Lucide、Vitest、react-test-renderer、Vite。

---

## File Structure

- Modify: `frontend/src/views/AssetsLandView.tsx` - 保存确认弹窗状态，合并扫描/购买入口，渲染受控购买确认弹窗。
- Modify: `frontend/src/views/AssetsLandView.test.tsx` - 覆盖新的单一入口、计划展示、取消、确认及空计划反馈。
- Modify: `frontend/src/style.css` - 删除废弃页面内预览样式，为紧凑的购买确认弹窗添加响应式样式。

### Task 1: Lock The New UI Contract With Tests

**Files:**
- Modify: `frontend/src/views/AssetsLandView.test.tsx:1-225`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Add a failing static-render test for the unified action**

Add this assertion to the existing static-preview test:

```tsx
expect(html).toContain('一键购买未解锁种子');
expect(html).not.toContain('解锁购买预览');
expect(html).not.toContain('确认购买');
expect(html).not.toContain('购买预览');
```

- [ ] **Step 2: Run the focused test and verify the expected failure**

Run: `npm test -- src/views/AssetsLandView.test.tsx` from `frontend`

Expected: FAIL because the current toolbar still renders `解锁购买预览` and `确认购买`.

- [ ] **Step 3: Add failing controlled-dialog behavior tests**

Import `act` and `create` from `react-test-renderer` and `vi` from `vitest`. Before importing `AssetsLandView`, mock the generated Wails module with the same hoisted pattern used by `AppBootstrap.test.tsx`:

```tsx
const atlasActions = vi.hoisted(() => ({
  preview: vi.fn(),
  purchase: vi.fn(),
}));

vi.mock('../../wailsjs/go/main/App', () => ({
  FarmAtlasBuyLockedPreview: atlasActions.preview,
  FarmAtlasBuyLockedCrops: atlasActions.purchase,
  FarmAtlasPreview: vi.fn(),
  FarmCropAnalytics: vi.fn(),
  FarmFertilizeLand: vi.fn(),
  FarmLandDetails: vi.fn(),
  FarmLandDetailsSince: vi.fn(),
  FarmLandRush: vi.fn(),
  FarmShovelLands: vi.fn(),
  FarmWarehouseRefresh: vi.fn(),
  FarmWarehouseSell: vi.fn(),
  FarmWarehouseSellRecords: vi.fn(),
  SaveWarehouseAutoSellSettings: vi.fn(),
  WarehouseAutoSellSettings: vi.fn(),
}));
```

In `beforeEach`, reset both mocks. Mount `AssetsLandView` with a runtime crop atlas, click the new action, and resolve `atlasActions.preview` with:

```ts
{
  ok: true,
  plan: {
    purchases: [
      { seedId: 20060, seedName: '莲藕', goodsId: 1, price: 1, count: 1, requiredLevel: 1 },
      { seedId: 20061, seedName: '红玫瑰', goodsId: 2, price: 1, count: 2, requiredLevel: 1 },
    ],
    summary: { purchasable: 2, skipped: 0 },
  },
}
```

Verify the dialog has `aria-label="确认购买未解锁种子"`, shows `莲藕 x1` and `红玫瑰 x2`, cancellation does not invoke `atlasActions.purchase`, and confirmation invokes it once. Add a separate empty-plan case asserting no dialog and the text `没有可购买的未解锁作物种子`.

- [ ] **Step 4: Run the focused test and verify the expected failure**

Run: `npm test -- src/views/AssetsLandView.test.tsx` from `frontend`

Expected: FAIL because no dialog component, dialog labels, or empty-plan message exist.

### Task 2: Implement The Unified Scan And Confirmation Flow

**Files:**
- Modify: `frontend/src/views/AssetsLandView.tsx:299-315,486-529,1611-1696`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Add the minimal dialog state and scanning action**

Replace the dual action handlers with a scan handler that stores the returned preview, opens only when purchases exist, and exposes the empty-plan message:

```tsx
const [atlasPurchaseOpen, setAtlasPurchaseOpen] = useState(false);
const [atlasPurchaseNotice, setAtlasPurchaseNotice] = useState('');

function scanAtlasLockedCrops() {
  const cropItems = atlasPreview?.sections?.find((section) => section.id === 'crop')?.items || [];
  if (atlasPurchaseLoading || cropItems.length === 0) return;
  setAtlasPurchaseLoading(true);
  setAtlasPurchaseNotice('');
  setAtlasError('');
  FarmAtlasBuyLockedPreview({ items: cropItems, countPerSeed: 1 })
    .then((payload: AtlasPurchasePreviewPayloadLike) => {
      if (payload?.ok === false) throw new Error(payload.error || '图鉴购买扫描失败');
      setAtlasPurchasePreview(payload);
      if ((payload.plan?.purchases || []).length > 0) setAtlasPurchaseOpen(true);
      else setAtlasPurchaseNotice('没有可购买的未解锁作物种子');
    })
    .catch((error) => setAtlasError(error instanceof Error ? error.message : String(error)))
    .finally(() => setAtlasPurchaseLoading(false));
}
```

- [ ] **Step 2: Replace the toolbar actions and remove the page-inline preview**

Render only this crop-only button in `AtlasPanel`:

```tsx
<button className="secondary-action atlas-buy-action" type="button" disabled={purchaseLoading || !payload?.buyEnabled || items.length === 0} onClick={onScanPurchase}>
  {purchaseLoading ? '扫描中...' : '一键购买未解锁种子'}
</button>
```

Remove the `atlas-purchase-preview` render block. Pass `purchaseNotice` into `AtlasPanel` and render it after the summary strip as an `asset-empty` message only when non-empty.

- [ ] **Step 3: Add the controlled confirmation dialog**

Add and export `AtlasPurchaseConfirmDialog` in `AssetsLandView.tsx`. It accepts `open`, `purchases`, `busy`, `error`, `onCancel`, and `onConfirm`; returns `null` while closed; and renders:

```tsx
<div className="dialog-backdrop atlas-purchase-dialog-backdrop" role="presentation">
  <section className="atlas-purchase-dialog" role="dialog" aria-modal="true" aria-label="确认购买未解锁种子">
    <header><h2>确认购买未解锁种子</h2><p>以下种子将加入购买队列。</p></header>
    <ul>{purchases.map((item) => <li key={`${item.seedId}-${item.goodsId}`}><span>{item.seedName}</span><strong>x{item.count}</strong></li>)}</ul>
    {error && <p className="atlas-purchase-dialog-error" role="alert">{error}</p>}
    <footer>
      <button name="cancel-atlas-purchase" type="button" disabled={busy} onClick={onCancel}>取消</button>
      <button name="confirm-atlas-purchase" className="primary-button" type="button" disabled={busy} onClick={onConfirm}>{busy ? '购买中...' : '确认购买'}</button>
    </footer>
  </section>
</div>
```

Render it beside the existing view panels. Its confirm handler continues to call `FarmAtlasBuyLockedCrops`, closes only after an `ok !== false` payload, refreshes the atlas, and leaves the dialog open with `atlasError` on rejection.

- [ ] **Step 4: Run the focused test and verify it passes**

Run: `npm test -- src/views/AssetsLandView.test.tsx` from `frontend`

Expected: PASS, including the new toolbar, dialog, cancel, confirm, and empty-plan tests.

### Task 3: Style And Verify The Dialog

**Files:**
- Modify: `frontend/src/style.css:5348-5419`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Replace obsolete inline-preview CSS with dialog styling**

Remove `.atlas-purchase-preview` and `.atlas-purchase-list` rules. Add compact responsive rules for `.atlas-purchase-dialog`, `.atlas-purchase-dialog header`, `.atlas-purchase-dialog ul`, `.atlas-purchase-dialog li`, `.atlas-purchase-dialog li strong`, `.atlas-purchase-dialog footer`, and `.atlas-purchase-dialog-error`. Use the existing `#fffdf7` surface, `#e2d7bf` borders, `#4f7f3f` action green, 8px corners, and `width: min(460px, calc(100vw - 32px))`.

- [ ] **Step 2: Add a style-contract assertion**

Read `style.css` in the existing test file and assert it includes `.atlas-purchase-dialog` and `width: min(460px, calc(100vw - 32px))`, so the dialog remains constrained on mobile screens.

- [ ] **Step 3: Run the focused test and verify it passes**

Run: `npm test -- src/views/AssetsLandView.test.tsx` from `frontend`

Expected: PASS.

- [ ] **Step 4: Run the complete frontend verification**

Run: `npm test` from `frontend`

Expected: PASS with zero test failures.

- [ ] **Step 5: Build the Wails frontend artifact**

Run: `npm run build` from `frontend`

Expected: TypeScript checking and Vite build complete with exit code 0; `frontend/dist/index.html` and its referenced generated asset have fresh timestamps.

- [ ] **Step 6: Review the diff and commit the feature**

Run: `git diff --check; git status --short`

Expected: only `AssetsLandView.tsx`, `AssetsLandView.test.tsx`, `style.css`, and this plan are changed for this feature.

Commit:

```bash
git add frontend/src/views/AssetsLandView.tsx frontend/src/views/AssetsLandView.test.tsx frontend/src/style.css docs/superpowers/plans/2026-07-14-atlas-one-click-locked-seed-purchase.md
git commit -m "feat: add atlas one-click seed purchase"
```
