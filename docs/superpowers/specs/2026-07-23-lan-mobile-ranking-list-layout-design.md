# LAN Mobile Ranking List Layout Design

## Goal

Fix overlapping rows in the LAN WebUI mobile ranking sheet when any ranking
tab contains enough data to exceed the available list height.

## Scope

The change applies only below the existing `.app-shell-remote` mobile breakpoint
(`max-width: 760px`) and covers all `SocialRankingDialog` views:

- `stolenByMe` in timeline and ranking modes;
- `stolenFromMe` in timeline and ranking modes;
- `visitors`.

Desktop layout, ranking queries, cursors, summaries, refresh behavior,
preference persistence, and row data formats remain unchanged.

## Cause

`social-ranking-list` is a vertical Flex container. Ranking rows retain the
default `flex-shrink: 1`, so a long page of rows can shrink every row below its
readable content height instead of overflowing the dedicated list viewport.
The resulting text from multiple rows occupies the same pixels.

## Design

The ranking sheet keeps its existing fixed header, tabs, date selector,
visitor-refresh action, view-mode tabs, summary metrics, and inline status
above a bounded ranking panel. The panel reserves its remaining height for
`social-ranking-list` only.

Every ranking row becomes an inflexible list item. When the current page or
loaded cursor pages exceed the panel height, `social-ranking-list` owns the
vertical scroll. Its existing `onScroll` pagination trigger continues to read
that same element. First-page loading, empty data, retry, next-page loading,
and next-page errors remain children of the scroll owner so they cannot expand
or collapse the sheet.

The CSS declaration is shared by all tab values. No tab-specific condition or
new component state is needed.

## Verification

1. Add a focused source-style regression assertion proving the mobile ranking
   list owns vertical overflow and ranking rows cannot flex-shrink.
2. Run the focused `SocialRankingDialog` Vitest suite before and after the CSS
   change.
3. Build the frontend and verify the generated LAN bundle updates in
   `public/app`.
4. Inspect the LAN page at a phone-width viewport with enough rows in each of
   the three tabs to confirm that rows remain separate and only the list
   scrolls.
