# Friend Steal Crop Whitelist Design

## Goal

Make the existing friend-steal crop whitelist mode enforce its selected crops, and include the missing `红云飞片` and `艾草` crop choices.

## Behavior

The shared crop-rule decision applies to automatic friend stealing and the social friend-list manual steal action.

- Blacklist mode retains its current behavior: strategy `1` skips a farm when any selected land contains a listed crop; strategy `2` removes only listed crops.
- Whitelist mode uses `autoFarmFriendStealPlantWhitelist`: strategy `1` skips a farm when any selected land is not listed; strategy `2` removes only unlisted crops.
- With whitelist mode enabled, an empty whitelist yields no eligible land and skips the farm. This prevents an enabled but unconfigured whitelist from stealing unintended crops.
- Lands without a detectable crop ID retain existing behavior and are not treated as whitelist misses.

## Crop Options

`BuildStealCropOptions` continues to use `Plant.json` as its primary source and supplements it with the existing crop-level mapping. This makes the two missing mapped crops available with their runtime plant IDs:

- `红云飞片`: plant `1020193`, seed `20193`
- `艾草`: plant `1021135`, seed `21135`

The existing image resolver supplies their local images from the bundled crop assets.

## Verification

Regression tests cover both whitelist strategies, an empty whitelist, retained blacklist behavior, and the presence and image URLs of both added options.
