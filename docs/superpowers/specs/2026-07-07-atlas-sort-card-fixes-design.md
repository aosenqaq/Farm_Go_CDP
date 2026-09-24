# Atlas Sort Card Fixes Design

## Goal

Fix the atlas page after migration: compact the header, match reference project crop and mutation ordering, improve name/image mapping, and simplify cards to the reference visual shape.

## Backend

Runtime atlas rows are normalized with the same priorities as the reference project. Crop rows use runtime `sort` first, then crop level. Mutation rows use `mutation_mapping.json` metadata: group rank (`黄金果实`, `装扮果实`, `活动果实`), atlas point, atlas order, and input index. The mapping also fills missing mutation names, group labels, atlas points, and local main-image paths.

## Frontend

The atlas panel becomes a compact tool surface: header text and status/actions sit in one toolbar, category tabs sit below it, and summary cards are removed from the first viewport to stop top compression. Cards match the reference shape: large art area, bottom name, and compact pills. Crop cards show only lock state and `作物`; mutation cards show only lock state and the mutation group.

## Verification

Go tests cover mutation metadata-backed sorting and image/name mapping. React tests cover the compact header structure and simplified card content. Final verification runs Go tests, frontend tests, and frontend build.
