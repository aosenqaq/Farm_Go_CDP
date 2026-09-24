# LAN Mobile Workbench Design

## Goal

Make the remote WebUI workbench practical on a phone without changing the
desktop workbench data model or the existing LAN APIs. The mobile page should
prioritize service health and recent task activity, avoid nested workbench
navigation, and keep the renewal and notice flows out of full-screen dialogs.

## Scope

The change applies to the mobile remote shell (`.app-shell-remote`) and the
runtime workbench. Desktop layout, runtime status derivation, authorization
state, event filtering, run statistics, renewal API, and notice API stay
unchanged.

## Mobile Workbench Layout

The mobile workbench remains one vertically scrolling page in this order:

1. A compact header has a one-line `运行时工作台` title, a compact target
   summary (`QQ WS · 已连接` when available), and direct actions.
2. The direct actions use `CreditCard + 续费`, `Megaphone + 公告`, and a
   standalone refresh icon. At the mobile breakpoint the announcement label is
   `公告`, not an icon-only control. Buttons retain accessible names and titles.
3. The readiness area summarizes the route as `QQ WS 已就绪` plus its existing
   ready/pending conclusion. It does not render the separate current-route
   facts list on mobile.
4. System health is the first major section. Its gateway, guard,
   authorization, and expiry values render as an even two-by-two grid.
5. Recent events follow system health. The event container has a stable mobile
   height using a viewport-bounded CSS value and `overflow-y: auto`; its title
   remains outside the scroller. Event entries change from a three-column
   desktop row to a two-column time and detail layout, so task source and
   result can wrap or truncate without horizontal overflow.
6. Run statistics remain last in normal document flow. The six existing metric
   cards render as a symmetrical two-column, three-row grid. They are not a
   tab, menu, drawer, or sticky viewport panel.

The current route facts aside remains available on desktop. The mobile CSS
only hides that duplicate detail surface and does not remove its data from the
component.

## Renewal And Notice Sheets

On remote mobile screens, the existing `LicenseRenewalDialog` and
`ProgramNoticeDialog` use bottom-aligned sheets rather than the generic rule
that forces every dialog to `100dvh`. The desktop dialogs retain their current
centered presentation.

The renewal sheet fits its short form to content and caps at about `56dvh`.
The input, result state, and actions remain visible or reachable without
making the dialog full-screen.

The program notice sheet caps at about `72dvh`. Its heading, close control,
and footer actions remain fixed while only the notice body scrolls. Existing
loading, empty, error, retry, success, close-button, and backdrop-close
behavior remain unchanged.

## Implementation Boundaries

- `frontend/src/views/OverviewView.tsx`: introduce responsive action label
  spans as needed; retain existing callbacks and all status/event/statistic
  calculations.
- `frontend/src/style.css`: add narrowly scoped remote mobile workbench and
  two-dialog sheet rules. Remove or override the generic remote full-screen
  rule only for renewal and notice dialogs.
- `frontend/src/views/OverviewView.test.tsx`: update static markup and CSS
  rule expectations for labelled actions, mobile status grid, event scroller,
  hidden route facts, and two-column statistic grid.
- Dialog tests remain responsible for renewal and notice data states; add CSS
  source assertions only where the project already uses them for responsive
  behavior.

## Error Handling And Accessibility

No new network state is introduced. Renewal continues to display backend
result messages, and notice continues to distinguish loading, unavailable,
and retryable failure. Sheet controls keep dialog semantics, `aria-modal`,
labelled headings, explicit close labels, keyboard-operable buttons, and the
existing click-outside close behavior. Long expiry values, messages, and event
content must not create horizontal page scrolling.

## Verification

1. Run the focused Overview and renewal/notice component tests.
2. Run the frontend test suite and production build.
3. Inspect the remote layout at a 360px-wide phone viewport and a desktop
   viewport: no header overlap, no horizontal overflow, events scroll within
   their own region, metrics stay two columns, and route facts are hidden only
   on mobile.
4. Open renewal and notice in the mobile remote shell and verify neither uses
   the full viewport; verify long notice content scrolls inside its body and
   retry/close actions remain usable.
