# New Crop Resource Support Design

## Goal

Add the newly collected crop resources to Farm Go so the warehouse resolves their seed, fruit, and golden-fruit names; land details render their crop state; and the steal-crop blacklist/whitelist displays their thumbnails with activity crops last.

## Scope

The source of truth for this change is:

`C:\Users\奥森\AppData\Roaming\QQEX\miniapp\temps\miniapp_src\1112386029_3_299cdabac04ffa90c15a88aa6676ed53\fetched-resources\新增资源整理`

The update includes these crop families: 发财红包, 帝王血, 铃兰, 月见草, 紫茉莉, 萱草, 月光花, 银星海棠, 紫薇, 梧桐, 勿忘我, 木槿, 星语铃花, and 蝶梦星铃.

It does not import unrelated outfits, currency, gift-pack, or scene-skin resources from the source bundle.

## Resource Data

Add only the relevant source `Plant` records and their matching `ItemInfo` records to the project game configuration. This covers ordinary seeds and fruits, golden fruits, and the special activity crop item used for 蝶梦星铃. Keep all existing configuration entries unchanged.

Copy the supplied crop and seed artwork into the existing game-config image structure and extend the generated mapping data used by the local image resolver. Activity crops belong in the `活动果实` image category. The category is explicit resource metadata and must not be inferred from names or numeric ranges.

## Warehouse Names

The warehouse uses `ItemInfo` for the display name and type. The imported records must ensure that an incoming item ID resolves to the correct name and category:

- Type `5` is a seed.
- Type `6` is a fruit.
- Type `17` is a golden/mutation fruit.
- The activity crop item for 蝶梦星铃 remains a tool-category item, preserving the upstream item type.

No fallback name should be added for unrelated IDs.

## Land Detail Images

The existing land image resolver remains the single image URL provider. It will resolve the imported crops by plant ID, seed ID, fruit ID, or name, then return the local asset URL for the active phase and golden state.

The source bundle does not contain the generic first-stage seed sprite. For every imported crop, phase one falls back to one existing crop's phase-one seed image. The fallback is deterministic and local.

星语铃花 and 蝶梦星铃 have no supplied phase-two-through-phase-six artwork. For those crops, any unavailable requested phase falls back to the crop's own supplied seed/main image. This keeps land cards informative and nonblank without inventing a crop-specific growth image.

## Steal Crop Selection

`BuildStealCropOptions` continues to generate option data in Go and the automation UI continues to render the provided `imageUrl` with `FallbackImage`.

The backend assigns activity crops a final sort group after shop-eligible and ordinary crops. It determines this from the activity image category metadata, not from crop name, ID range, or frontend state. The frontend preserves `sortGroup`, then level, configured order, and plant ID ordering. Both blacklist and whitelist consequently show the same stable order and thumbnails.

## Error Handling

- Missing individual image files follow the existing image fallback behavior; the option/card still renders its label.
- A missing mapped phase falls back only as described above and never exposes a filesystem path.
- A duplicate imported ID is ignored during data generation rather than replacing an existing, unrelated record.
- Existing crop, mutation, warehouse, and non-activity sorting behavior remains unchanged.

## Tests And Verification

Add Go tests for the imported item IDs and display names, activity sort grouping, normal/golden stage paths, phase-one fallback, and seed-main fallback for the two crops without full stage assets.

Add a frontend test proving that the blacklist/whitelist crop grid receives and renders image URLs while preserving backend activity-last ordering.

Run focused Go and frontend tests, then `go test ./...`, the frontend test suite, and the frontend production build.

## Acceptance Criteria

- Newly received seed, fruit, and golden-fruit IDs show their upstream names in the warehouse.
- A land card for every imported crop has an image URL for each observed phase; phase one uses the local seed-stage fallback where needed.
- 星语铃花 and 蝶梦星铃 never render a blank crop image because their unavailable phases use their supplied seed/main image.
- Both steal-crop list modes show imported crop thumbnails.
- Any crop classified in `活动果实` appears after non-activity crops in both list modes.
- No unrelated source-bundle resource is imported.
