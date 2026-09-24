# Atlas Migration UI Design

## Goal

Migrate the reference project's atlas refresh and locked-crop seed purchase flow into this Wails app, then make the atlas page show separate crop and mutation atlas categories. The locked-seed purchase controls appear only on the crop atlas page.

## Backend Design

`App.FarmAtlasPreview()` remains the refresh entry point. It calls `gameCtl.requestAtlasUnlockRowsByProtocol`, then `farm.BuildRuntimeAtlasPreview` normalizes crop and mutation sections. If runtime is unavailable, it falls back to static game config.

`internal/farm/gameconfig.go` owns atlas data shaping and purchase planning. Runtime rows are normalized with crop/mutation metadata, image URLs, lock state, progress, and stable sort. A new locked-crop purchase plan filters to crop atlas rows only, skips already unlocked crops, missing seed IDs, level-locked crops, and seeds absent from the shop. `App.FarmAtlasBuyLockedPreview` returns the plan. `App.FarmAtlasBuyLockedCrops` builds the same plan and calls `gameCtl.buyShopSeedsBatch`.

## Frontend Design

`AssetsLandView` keeps the atlas page inside the existing asset tool surface. The atlas panel gains a compact segmented control for `作物图鉴` and `超变图鉴`, summary counters, image cards, and lock-state badges. The crop tab shows `解锁购买预览` and `确认购买`; the mutation tab hides purchase controls entirely.

The visual style follows the existing global UI: compact operational layout, 8px cards, `#fffdf7` and `#fffaf0` surfaces, deep green actions, restrained borders, and no landing-page or marketing treatment.

## Testing

Go tests cover runtime crop/mutation atlas shaping and purchase-plan filtering. React static-render tests cover category tabs and purchase controls only rendering on the crop tab. Verification runs Go tests, frontend tests, and frontend build.
