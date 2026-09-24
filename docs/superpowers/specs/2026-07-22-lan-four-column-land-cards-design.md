# LAN Four-Column Land Cards Design

## Goal

Replace the mobile LAN land-details card presentation with a compact four-column operational grid. Every remote mobile viewport shows four land cards in each row without horizontal page scrolling.

## Layout

The remote land grid uses four equal `minmax(0, 1fr)` columns with a small fixed gap. Each card is a fixed, compact vertical stack:

1. Land number and short runtime status.
2. A fixed square crop-art area.
3. Truncated crop name and compact maturity state.
4. Three equal icon-only controls for inorganic fertilizer, organic fertilizer, and shovel.

The crop-art area and action controls have fixed dimensions so changing crop names, statuses, or loading icons cannot resize cards. Existing crop image trimming stays in place; this layout does not attempt further crop-art positioning changes.

## Behavior

The existing land polling, incremental updates, status badges, maturity calculation, disabled/busy rules, per-land commands, and top-level rush/bulk fertilizer actions remain unchanged. The per-land commands retain their current handlers and receive `aria-label` and hover titles when their visible text is hidden on mobile.

Desktop continues to use the existing detailed four-column cards. The compact rules are scoped under `.app-shell-remote` and the mobile breakpoint only.

## Verification

- Add a render test for the three labelled icon controls and the existing action handlers.
- Add a CSS regression test that requires the remote land grid to use four columns and compact fixed card/image dimensions.
- Run the focused land view test suite, the complete frontend test suite, and the Vite production build.
- Verify the running LAN source serves the four-column mobile rules, then hard-refresh an already-open mobile browser page.
