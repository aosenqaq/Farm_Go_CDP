# Steal Crop Blacklist Strategy Design

## Goal

Make crop blacklist behavior consistent across automatic friend stealing, scheduler-center manual friend stealing, and the social friend-list manual steal action.

## Behavior

When `autoFarmFriendStealPlantBlacklistEnabled` is true and `autoFarmFriendStealPlantListMode` is `blacklist`, the selected crop IDs in `autoFarmFriendStealPlantBlacklist` affect friend stealing:

- Strategy `1` (`autoFarmFriendStealPlantBlacklistStrategy = 1`): if any collectable land contains a blacklisted crop, skip the whole friend farm and write a `steal_blacklist_skip` cooldown record.
- Strategy `2` (`autoFarmFriendStealPlantBlacklistStrategy = 2`): filter out blacklisted crop lands and steal the remaining lands. If every collectable land is blacklisted, skip the whole friend farm and write a `steal_blacklist_skip` cooldown record.

If the crop blacklist is disabled, not in blacklist mode, empty, or a collectable land has no detectable crop ID, existing behavior is preserved for that land.

## Architecture

Add a small shared package for crop-rule decisions so `automation` and `social` can use the same logic without creating an import cycle. `automation.RuntimeFacade.runFriendSteal` and `social.Service.runSteal` will both apply the shared decision after protocol inspection and before `friendHarvestLandsByProtocol`.

`social.Service` will receive the current automation config through `social.Options`, populated by `App.socialService()` from `FarmAutomationState().Config`.

## Tests

Cover strategy `1` and `2` in the automation runtime, and cover social-list manual stealing with the same config. Existing tests cover the disabled/no-rule paths and friend blacklist rules.
