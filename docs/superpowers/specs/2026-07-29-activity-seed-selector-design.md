# Activity Seed Selector Design

## Goal

Ensure the automatic-planting backpack selector and backpack-first automation retain every real seed returned by the runtime, including all current and future activity seeds, without reintroducing dog food, gift packs, or other non-seed items.

## Root Cause

`gameCtl.getSeedList` already performs runtime seed validation and returns normalized entries with `type: 5` and `interactionType: "plant"`. `BuildBackpackSeedOptions` then applies a second, ID-only allowlist based on the bundled `Plant.json` and `ItemInfo.json`. A seed missing from an older embedded resource bundle is discarded despite the authoritative runtime classification.

The current source contains activity metadata for 发财红包, 帝王血, 铃兰, 月见草, 紫茉莉, 萱草, 月光花, 银星海棠, 紫薇, 梧桐, 勿忘我, 木槿, and 星语铃花. The design must not depend on that finite list.

## Design

For entries present in the runtime seed list, accept either of these proofs that the entry is plantable:

1. The bundled configuration recognizes the seed ID as a plant seed.
2. The runtime entry explicitly reports item type `5` and interaction type `plant`.

The second proof makes activity seed recognition forward-compatible while preserving the runtime script's seed gate as the authority for live backpack contents.

Force-priority retention applies only when an ID is absent from the live backpack response. It continues to require static plant metadata, so stale saved IDs cannot create dog-food, gift-pack, or placeholder rows.

The frontend remains unchanged: normalized runtime seed entries already carry the same explicit type and interaction fields, so its stale-config non-seed guard accepts them.

## Resource Bundle

Regenerate `resources/gameConfig.bundle.zip` from the updated `resources/gameConfig` directory. This synchronizes the embedded release resource package with current activity metadata for name, level, and plant-size enrichment.

## Tests

Add a backend regression test with a previously unknown seed ID explicitly identified by the runtime as `type: 5` and `interactionType: "plant"`; it must appear in the selector. Keep the existing non-seed regression test unchanged and passing. Run the focused farm package tests, the resource-bundle tests, and the full Go test suite after rebuilding the bundle.
