# Dream Butterfly Mutation Support

## Scope

Support the newly added mutation type `梦蝶` and its `黄金·蝶梦星铃` output without changing unrelated mutation behavior.

## Evidence

- Mutation type IDs `1` through `10` are already defined; `11` is unused.
- `Plant.json` declares butterfly bell's mutation effect as `5_11:1128003:1`.
- Runtime plant ID `1128003` therefore represents the result of the `黄金` and `梦蝶` combination: `黄金·蝶梦星铃`.

## Design

1. Copy the supplied butterfly icon to the established mutation-icon path:
   `plant_images/stages/变异/变异宝典/梦蝶/梦蝶_00_变异图标.png`.
2. Register ID `11` and the display name `梦蝶` in the existing runtime mutation maps. The display order appends the new type after existing types to preserve their current order.
3. Register runtime plant ID `1128003` as `黄金·蝶梦星铃`.
4. Let mutation-image lookup use the existing activity-crop image resolver when the requested mutation output is an activity crop. This reuses the already imported `黄金·蝶梦星铃_00_主图.png` rather than duplicating it.
5. Add a land-details regression test for active mutation types `5` and `11`; it must expose the two labels, the dream-butterfly icon, and the local golden butterfly bell image.

## Verification

Run the focused farm regression test, then `go test ./...`. Run frontend tests and the production build to verify the existing land-details contract remains compatible.
