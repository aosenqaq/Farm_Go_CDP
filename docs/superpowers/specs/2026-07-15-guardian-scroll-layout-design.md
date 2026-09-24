# Guardian Service Scroll Layout Design

## Goal

Make the guardian service content scroll vertically within the application viewport and prevent expanded settings from overlapping the guardian status cards.

## Root Cause

The application shell fixes the main surface to the viewport and hides document overflow. `GuardView` uses `.guard-stack`, whose final grid row is constrained to the remaining height (`1fr`). The guardian capability grid therefore receives a capped row even when a worker card expands its advanced settings. Its content then paints beyond that row and covers the cards below.

## Design

Add a guardian-only scroll container beneath the page header. The scroll container owns the command panel and all capability cards, uses vertical auto overflow, and has a zero minimum height so it can shrink inside the fixed application shell. The page header remains outside this container.

The guardian stack becomes a two-row layout: the header uses its natural height and the new scroll container occupies remaining height. The capability grid remains a three-column responsive grid, but no longer participates as a fixed `1fr` page row. Expanded settings increase their card and grid height naturally within the scrollable content flow.

## Scope

- Change `GuardView` markup only to add the guardian scroll wrapper.
- Change guardian-specific CSS only to define the scroll container and remove the capped third-row layout.
- Add a frontend regression test that checks for the scroll wrapper class and CSS layout contract.
- Do not alter guardian status, setting persistence, button behavior, or global scrolling.

## Verification

Run the focused `GuardView` test, the frontend test suite, and the frontend build. Confirm the build updates the generated `public/app` asset.
