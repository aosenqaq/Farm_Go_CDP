# Basic Task Default Interval Design

## Goal

Make the default intervals for the three basic scheduler tasks consistent at 60 seconds:

- `own_base` (one-click farming)
- `own_collect` (automatic harvest)
- `land_upgrade` (automatic land upgrade)

Land upgrade remains disabled by default.

## Scope

Update the default interval values in both sources that form a new automation state:

- `internal/farm/automation/config.go`: the three interval config values in `DefaultConfig`.
- `internal/farm/automation/catalog.go`: the `own_collect` and `land_upgrade` task definitions. `own_base` already uses 60 seconds.

Update the default-state regression test to assert all three task intervals and all three configuration values are 60 seconds.

## Behavior

`DefaultConfig` and `DefaultState` will agree on the same 60-second default. Existing saved account settings are not migrated or modified; their explicitly stored intervals continue to take precedence.

## Verification

Add a focused failing test before production changes. Then run the focused automation package tests and the broader relevant Go package suite.
