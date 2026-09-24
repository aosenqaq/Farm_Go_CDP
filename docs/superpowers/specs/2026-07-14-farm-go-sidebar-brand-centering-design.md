# Farm Go sidebar brand centering design

## Goal

Center the complete Farm Go wordmark horizontally in the desktop sidebar brand
area while retaining its current vertical placement above navigation.

## Design

The existing `.brand-block` becomes a flex container with
`justify-content: center`. Its existing minimum height and bottom margin stay
unchanged, so the navigation starts at the same vertical position. The
`.brand-name` remains an inline flex item; its text, `//` pseudo-element, and
anchored top rule keep their current visual treatment and move together as one
wordmark.

The narrow-sidebar media rule continues to hide `.brand-block`, so the compact
icon-only navigation remains unchanged.

## Validation

- Add a CSS source contract test that verifies the desktop brand block has
  both `display: flex` and `justify-content: center`.
- Run the full frontend test suite.
- Run the frontend production build and confirm the latest generated CSS asset
  contains the centered-brand rule.

## Non-goals

- No change to the Farm Go copy, animation, sidebar width, vertical spacing,
  navigation behavior, or collapsed-sidebar behavior.
