# Farm Go sidebar brand design

## Goal

Replace the sidebar's two-line diagnostic label with a single, distinctive
Farm Go brand mark that feels like an active desktop control console.

## Scope

- Render `Farm Go` using a space, replacing the current `Farm_Go` text.
- Remove the `本机诊断站` subtitle completely.
- Use the selected "sweeping metal" wordmark: a pale metallic text fill with
  a low-frequency lime highlight, a short lime line above the wordmark, and a
  compact `//` suffix.
- Keep the current sidebar width, padding, navigation layout, colors, and all
  non-brand behavior unchanged.

## Design

The wordmark is a single accessible text node inside the existing brand block.
CSS supplies the visual treatment: a multi-stop background clipped to the
letters and a 3.5-second linear background-position animation create the
metallic sweep. The treatment uses the existing near-black sidebar,
warm-white base text, and lime accent so it reads as part of the established
interface rather than a separate product identity.

The decorative line and `//` suffix are CSS pseudo-elements. They are not
additional spoken content. Reduced-motion users receive the static metallic
fill with no sweep animation.

## Testing and validation

- Add a focused `AppShell` rendering test that confirms `Farm Go` is present
  and `本机诊断站` is absent.
- Run the frontend test suite.
- Run `pnpm run frontend:build` so the static bundle served from `public/app`
  includes the change.
- Confirm the generated `public/app/index.html` points at current bundle
  assets and that the latest generated asset timestamp changes.

## Non-goals

- No new image assets, logo files, dependencies, or JavaScript animation.
- No changes to the license screen, release icon, navigation behavior, or
  responsive sidebar-collapse behavior.
