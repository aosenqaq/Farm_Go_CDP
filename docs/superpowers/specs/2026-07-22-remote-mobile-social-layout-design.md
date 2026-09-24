# Remote Mobile Social Layout Design

## Goal

Make the LAN WebUI usable on narrow phone screens without changing any existing Farm Go social or runtime behavior. The work covers the remote workspace overview, the friend feature dialog, and the social ranking dialog shown in the supplied captures.

## Approved Direction

Use a bottom Sheet for remote-phone social dialogs. The Sheet occupies at most `84dvh`, with a fixed header, optional fixed footer, and one scrollable body. This keeps the current social page visible beneath the dialog, provides a clear close action, and prevents desktop grid sizing from allowing record content to overlap.

## Scope

### Workspace Overview

- Reserve space above the remote bottom navigation so the last statistics cards stay reachable.
- Keep status tiles and run statistics in a two-column grid on phone widths.
- Change recent events to a stable layout: a fixed-width time column and a `minmax(0, 1fr)` text column. Titles and detail text must wrap or truncate inside that column instead of escaping into adjacent rows.
- Allow the view body, rather than the browser document, to scroll when its content exceeds the available height.

### Friend Social Page And Feature Dialogs

- Keep all existing feature-menu entries visible and actionable on a remote phone.
- Render the feature menu as a single column of full-width rows with a touch target of at least 46px.
- Use the shared Sheet height, header and body scrolling rules for the feature dialog, rule editors, import/export, protocol block list, dog-guard scan, batch removal, and God-rank dialogs.
- Keep dense friend rows usable by placing action buttons in a constrained right-side grid and allowing text cells to shrink and ellipsize.

### Ranking Dialog

- Keep the dialog header and close button fixed.
- Make primary tabs horizontally scrollable. Move the list/ranking mode selection to its own line at narrow widths.
- Render summary values in a two-column grid.
- Hide the desktop table heading on phones and render each record as a one-column mobile row: detail first, time and friend information as secondary lines, and its optional action below.
- Give the ranking list `min-height: 0`, a bounded parent grid, and the only vertical `overflow: auto` in the ranking content path. Loading, error, empty, and pagination rows use the same list flow.

## Non-Goals

- No changes to friend actions, rankings pagination, data fetches, protocol calls, or desktop layouts.
- No new navigation, data fields, or mobile-only feature modes.

## Architecture

The change stays in the React view markup only where a distinct mobile record structure is required, plus scoped `@media (max-width: 760px)` rules under `.app-shell-remote`. Desktop UI and Wails APIs remain untouched.

`SocialRankingDialog` owns the ranking list's structural grid. `style.css` owns breakpoints, sizing, truncation, and scroll boundaries. `OverviewView` retains its current data shape; only its semantic event markup may gain class names required for stable narrow-screen placement.

## Failure Handling

Existing loading, error, empty, disabled, and pagination behavior is preserved. CSS must not clip status text or prevent access to close/refresh controls. If a mobile list has no vertical room, the list itself scrolls; the dialog and browser viewport do not compete for scroll ownership.

## Testing And Verification

- Extend focused frontend tests for event markup and ranking-row mobile structure only if markup changes.
- Run the frontend test suite and production build.
- Start the LAN WebUI and inspect the overview, friend feature menu, and all ranking tabs at a phone viewport (360px and 412px wide). Confirm no horizontal overflow, overlap, inaccessible close button, or clipped final card/list rows.

## Self-Review

- Placeholder scan: no TODO, TBD, or deferred requirements.
- Consistency: all dialog types use the same remote Sheet direction; only the ranking list owns vertical scrolling within ranking content.
- Scope: this is one frontend-responsive-layout task and leaves domain behavior unchanged.
- Ambiguity: the breakpoint is explicitly the existing `760px` remote-phone breakpoint, and the recommended interaction is explicitly a bottom Sheet.
