# Activity Crop Land Stage Images Design

## Goal

Land details must resolve a local stage image for activity crops even when the
runtime supplies only `displayPlantName` rather than `plantName` or a usable
seed ID.

## Scope

- Use the display name as a final crop-identity candidate while building a
  land-detail image URL.
- Keep the existing resolution order: matching stage image, activity main
  image, generic seed image when explicitly requested, then the global image
  fallback.
- Cover a runtime payload with an empty primary plant name and a known activity
  display name at a non-seed stage.

## Design

`BuildRuntimeLandDetailsForRoot` already receives both runtime names. It will
derive one image lookup name from `plantName` and `displayPlantName`, preferring
the primary name when it is available. The resolver will use that identity for
both plant metadata lookup and activity-crop mapping lookup.

The change remains in the Go resource resolver so desktop and LAN clients use
the same local image URL. It does not add frontend-specific lookup logic or
per-crop aliases.

Activity crops that have a matching stage asset return that exact stage image.
Those with only an activity main image continue to return that main image for
non-seed stages instead of reaching the global default portrait.

## Testing

Add a table-style regression case to `gameconfig_land_assets_test.go` that
creates an activity mapping and a non-seed stage image, passes only
`displayPlantName` in the runtime grid, and asserts that the resulting URL is
the local URL for the expected stage file. Run that targeted Go test and the
full `internal/farm` package tests.
