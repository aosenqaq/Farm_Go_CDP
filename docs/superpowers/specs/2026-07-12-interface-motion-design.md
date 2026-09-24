# Interface Motion Design

## Scope

Add restrained CSS transitions to the authorized Farm Go interface so navigation changes and modal openings communicate hierarchy without delaying operational work. This is a presentation-only change: API calls, account state, authorization checks, focus behavior, and action handlers remain unchanged.

## Motion Language

The system uses three complementary motion patterns, all with a 150-220ms duration and an ease-out curve:

1. The primary workspace and standalone application views fade in while moving upward by at most 4px.
2. Same-level workspace tabs, including the assets/land tabs, leave by 12px and the replacement enters from the opposite side. The direction is fixed rather than inferred from tab position, so it remains predictable when views are opened programmatically.
3. Modal backdrops fade in first and dialog surfaces enter with a subtle 8px rise and 0.98 to 1 scale. Closing waits for the matching exit animation before unmounting.

The implementation honors `prefers-reduced-motion: reduce` by eliminating transforms and reducing all transition duration to effectively immediate. Loading states and continuous progress indicators are out of scope.

## Architecture

Create a small reusable React presence helper in `frontend/src/components` that retains the last child during an exit window and provides stable class names for entering and exiting content. It receives a key that identifies the visible view or dialog and calls its removal callback only after the CSS transition ends. This avoids an animation library and preserves the existing React 18/Vite dependency footprint.

`AuthorizedApp` wraps the main conditional view composition in that helper. `AssetsLandView` wraps its secondary tab content with the same helper. A modal wrapper applies the same lifecycle to each existing dialog owner; it does not introduce a new global modal registry or alter the existing z-index hierarchy.

## Dialog Coverage

Apply the modal lifecycle and shared CSS classes to the common dialog surfaces currently owned by startup injection, account identity, update details, social, automation, warehouse settings, warehouse records, and the settings dialog. The land-rush side drawer uses the same backdrop timing but a horizontal 16px transform rather than dialog scale so its origin remains visually correct.

Each dialog keeps its current `role`, `aria-modal`, labels, keyboard handling, and close callback. During an exit, it remains mounted and non-interactive; opening it again cancels the pending exit.

## Error Handling

If a `transitionend` event does not fire, a timeout slightly longer than the declared CSS duration completes the exit. Reduced-motion users do not incur this delay. An unknown child key is treated as a normal content replacement rather than an error.

## Verification

- Unit tests exercise the presence helper's enter, exit, cancellation, and reduced-motion paths with fake timers.
- Targeted existing view tests confirm primary and secondary view wrappers render without changing their data or callbacks.
- Targeted dialog tests confirm a close request keeps the dialog mounted until its exit completes, then removes it, without changing ARIA attributes.
- Run `npm test` and `npm run build` from `frontend`.
