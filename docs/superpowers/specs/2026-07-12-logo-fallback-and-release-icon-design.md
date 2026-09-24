# Logo fallback and release icon design

## Goal

Use `logo/logo.png` as the Windows release icon source and as the common
fallback for any frontend image that fails to load.

## Scope

- Copy the logo into frontend static assets so the bundled WebView can load it
  independently of the source checkout.
- Give each current frontend `<img>` rendering path the same one-shot error
  fallback, including account avatars, crop images, mutation icons, and
  automation option images.
- Replace the Wails PNG icon source and generate the Windows ICO used by both
  the executable and NSIS installer.

## Design

The frontend will expose a small reusable image component that renders a normal
`img`. If its original source fails, it changes the source to the bundled logo
and disables further error handling. Existing `alt`, CSS classes, conditional
rendering, and layout remain unchanged.

The release assets are updated from the supplied PNG: `build/appicon.png`
remains the Wails source image, and `build/windows/icon.ico` is regenerated as
a multi-resolution Windows icon. This preserves the Wails build and installer
configuration without changing packaging scripts.

## Validation

- Add a frontend test that proves a failed original image displays the logo.
- Run the focused frontend test suite and the production frontend build.
- Inspect the ICO metadata to confirm it is a readable multi-size icon.

## Non-goals

- No layout, styling, copy, image-selection logic, or packaging workflow
  changes beyond the requested icon and image fallback behavior.
