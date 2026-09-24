# Remote Mobile Event Time Design

## Goal

Keep the complete `HH:MM:SS` value visible in each recent-event row of the
LAN WebUI on phone-width remote layouts.

## Root Cause

`formatEventTime` consistently emits an eight-character `HH:MM:SS` value.
The remote-phone event grid assigns its time cell a fixed width of `44px` and
the shared event-time style clips overflowing text. At the existing `10px`
monospace font size, the available width cannot contain the formatted time.

## Design

At the existing `max-width: 760px` remote-only breakpoint, increase the first
column of `.workbench-event` from `44px` to `52px`. The time remains a single
line spanning the title and result rows. The second column stays
`minmax(0, 1fr)`, so event text still receives all remaining width and follows
the current truncation and line-clamp rules.

## Scope

- Modify `frontend/src/style.css` for the remote-phone event grid only.
- Modify `frontend/src/views/OverviewView.test.tsx` to assert the 52px time
  column in the existing responsive CSS contract.

## Non-Goals

- Do not change desktop layout, event data, event time formatting, or task
  text behavior.
- Do not modify unrelated mobile land-card work currently present in the
  working tree.

## Verification

Run the focused overview test before and after the CSS change, then run the
frontend production build. At a 360px remote-phone viewport, verify that a
recent event displaying `HH:MM:SS` is fully visible and that its title and
result do not overlap the time column.

## Self-Review

The change has one responsive CSS effect and one regression assertion. Its
breakpoint, selector, expected width, and visual acceptance condition are
explicit; no placeholders or deferred decisions remain.
