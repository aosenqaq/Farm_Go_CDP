# Restore Fertilizer Fill Controls Design

## Goal

Remove the unimplemented fertilizer auto-buy controls from the automation settings page and restore the independent fertilizer-fill controls.

## Scope

- Remove the `autoFarmFertilizerAutoBuyEnabled`, `autoFarmFertilizerAutoBuyType`, and `autoFarmFertilizerAutoBuyMaxCount` controls from the fertilizer settings group.
- Restore the `autoFarmFertilizerFillEnabled` checkbox labelled "自动填充肥料".
- Restore the `autoFarmFertilizerFillIntervalSec` numeric input labelled "填充肥料间隔(秒)".
- Update the component test to assert the restored controls are present and the removed controls are absent.

## Compatibility

The backend default configuration retains the removed auto-buy keys. Existing saved settings remain readable, but the inactive controls are no longer displayed or editable.

## Verification

Run the focused AutomationView test, the frontend test suite, and the frontend build. Verify the generated bundle is referenced by `public/app/index.html`.
