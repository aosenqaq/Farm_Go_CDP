# Land Mutation Icon Stack Design

## Goal

Show every mutation icon for a land crop in the land-details card. Icons appear at the card visual's right edge, stacked vertically in the same order as the runtime mutation types.

## Scope

The frontend uses the existing `mutationTypes` array. It will collect each non-empty `iconUrl` in array order and render an icon for each value inside a dedicated vertical overlay container.

For compatibility with older land payloads that only expose `mutationIconUrl`, the card renders that single URL when `mutationTypes` contains no usable icons. A single icon keeps the current size and right-top visual position.

## Layout And Behavior

- The overlay container is absolutely positioned at the visual's top-right corner.
- Its child icons use the existing 24px visual treatment and stack top-to-bottom with a small fixed gap.
- The crop visual and text fields remain unchanged.
- Icons with missing URLs are omitted; no empty placeholder is rendered.

## Verification

Add a static-render test with two ordered mutation types and assert that both icon sources occur in the rendered markup in their input order. Keep the existing single-icon test as compatibility coverage. Run the focused frontend test suite and the frontend production build.
