# Land Artwork Geometric Centering Design

## Goal

Center the visible outline of each crop image in the mobile LAN land-card artwork area. The full crop, including its ground and effects, must remain inside the existing safe padding.

## Root Cause

`landArtworkDrawPlacement` currently places the alpha-weighted pixel centroid at the canvas center. For asymmetrical artwork, the weighted centroid does not match the midpoint of the visible alpha bounds. The image element is centered, while the crop outline appears shifted within it.

## Design

Keep the existing mobile-only canvas path, alpha-bound detection, crop rectangle, 148px canvas, and safe clipping margin. Change the placement anchor from `alphaCenterX` and `alphaCenterY` to the geometric midpoint of `minX`/`maxX` and `minY`/`maxY`.

The scale continues to be constrained by the farthest visible bound from that geometric midpoint, so no opaque crop pixels are clipped. The desktop fallback image, land-card dimensions, card actions, local asset URL policy, and LAN HTTP interface remain unchanged.

## Verification

- Add a regression test using asymmetric alpha-weight data whose visible bounds are centered at the canvas midpoint after placement.
- Confirm that all visible alpha bounds remain inside the existing padding.
- Run the focused `AssetsLandView` test suite and the frontend production build.
- Open the LAN WebUI at a mobile viewport and verify the crop outline is centered in a land card.
