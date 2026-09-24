# LAN Mobile Land Card Design

## Goal

Redesign the LAN WebUI land-details cards at the mobile breakpoint to use the two-column operational layout approved from the reference image. Use the same per-land fertilizer chooser on LAN mobile and the normal desktop page while retaining the existing shovel action.

## Scope

The two-column card layout applies only inside `.app-shell-remote` at the existing mobile breakpoint. Desktop card dimensions and columns remain unchanged, but its per-land action row changes to the same `施肥` and `铲除` controls. Land polling, bulk actions, and game-runtime commands remain unchanged.

## Card Layout

The remote mobile land grid renders two equal columns. Each card has a stable vertical layout:

1. Land number and compact runtime status.
2. A fixed crop-art area that retains the existing image trimming and mutation indicators.
3. Crop name and maturity text.
4. Existing land-type, season, mutation, and operational tags, wrapping without changing the card width.
5. A bottom action row with `施肥` and `铲除`.

Land-type coloring remains data-driven through the existing status and tag classes. Growing, mature, empty, locked, dead, and mutation states retain their current data and visual cues. Cards use fixed or constrained dimensions for art, buttons, and status regions so dynamic crop names and busy icons cannot cause layout shift.

## Fertilizer Selection

Clicking a card's `施肥` action opens the shared fertilizer chooser for that land. It identifies the target land and crop, offers `无机肥` and `有机肥`, and has a cancel action. On LAN mobile it is a bottom sheet; on the normal desktop page it is a centered modal, capped at `440px` wide with at least `16px` viewport margins.

Choosing a type invokes the existing per-land fertilizer handler with the corresponding mode. While an action is in progress, the existing busy key disables competing land actions and presents the existing loading state. The sheet closes once a fertilizer action is submitted; success and failure continue through the current refresh and error handling path.

`铲除` stays directly available as the card's destructive action and keeps its existing disabled and busy behavior.

## Component Boundary

`LandDetailsPanel` owns the selected fertilizer target and renders a focused fertilizer chooser component. The chooser accepts the target land, open state, busy state, cancel callback, and mode-selection callback. It has no transport logic. `AssetsLandView` remains the owner of runtime command submission and post-action refreshes.

## Accessibility and Interaction

The bottom sheet uses dialog semantics, is labelled with the target land and crop, and exposes descriptive labels for each fertilizer choice. The background is visually dimmed; Cancel and the close affordance dismiss without dispatching a command. Buttons retain the current disabled semantics.

## Testing and Verification

Frontend tests cover the two-column mobile selector, shared desktop/mobile `施肥` trigger, rendered land-card information, opening and closing the fertilizer chooser, dispatching normal and organic fertilizer selections, and busy or disabled states. The focused land view suite and full frontend suite run before building the production frontend bundle. The bundle is rebuilt with `pnpm --dir frontend run build`, which updates the Wails-embedded `frontend/dist` assets.
