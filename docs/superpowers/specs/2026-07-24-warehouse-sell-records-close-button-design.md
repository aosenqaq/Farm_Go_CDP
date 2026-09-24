# Warehouse Sell Records Close Button Design

## Goal

Restore a consistently visible and clickable close button in the top-right corner of the warehouse sell-records dialog without changing other dialogs or warehouse behavior.

## Root Cause

The dialog already renders an accessible button containing the Lucide `X` icon and wires it to `onClose`. On narrow CSS viewports, the records table's `min-width` expands the dialog grid's implicit column beyond the clipped dialog width, which pushes the otherwise visible close button outside the viewport. The button also relies only on the shared `icon-button` class. The fix must constrain the dialog grid column and give this close control an explicit, stable presentation contract.

## Implementation

- Add a dedicated class to the existing sell-records close button.
- Constrain the sell-records dialog to a `minmax(0, 1fr)` grid column so the table overflows only inside its scroll container.
- Keep the existing `aria-label`, `type="button"`, Lucide `X` icon, and `onClose` handler.
- Define dialog-specific styles that keep the control visible, non-shrinking, aligned to the header's top-right corner, and visually consistent with the current warm surface and lime action color.
- Do not use absolute positioning; the existing two-column header grid remains responsible for placement.
- Do not change the shared `icon-button` class or other dialogs.

## Responsive Behavior

The dedicated close control keeps a stable square hit area on both desktop and remote/mobile layouts. It remains in the header's second grid column and cannot be compressed by a long title or narrow viewport.

## Testing

- Add a frontend regression assertion for the dedicated close-button class and accessible label.
- Add a CSS regression assertion covering stable dimensions, non-shrinking behavior, and visible foreground/background colors.
- Run the focused `AssetsLandView` tests and the full frontend test suite.
- Rebuild `frontend/dist` and confirm `frontend/dist/index.html` references the newly generated assets.
- Visually verify that the sell-records dialog shows the close icon at desktop and narrow viewports and that clicking it closes the dialog.

## Non-Goals

- Redesigning the sell-records dialog.
- Changing data loading, filtering, table rendering, or record content.
- Refactoring shared dialog or button components.
- Changing close controls elsewhere in the application.
