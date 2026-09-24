# Warehouse Auto-Sell Interval Ownership

## Goal

Keep the warehouse auto-sell interval selected in the warehouse settings dialog from being reset to the default 60 minutes by an unrelated automation settings save.

## Cause

The warehouse dialog persists `WarehouseAutoSellSettings` directly. The generic automation save endpoint also derives warehouse settings from its complete scheduler-state payload and persists them. An already-open automation dialog can therefore submit its stale default `autoWarehouseSellIntervalSec` value of 3600 and overwrite the warehouse dialog's newer value.

## Design

`WarehouseAutoSellSettings` is the sole persisted owner of the warehouse auto-sell enabled flag, interval, and categories. `SaveFarmAutomationState` will load that persisted value and overlay it onto the automation scheduler settings before saving or reconfiguring the scheduler. It will not derive or save warehouse settings from the generic automation payload.

The scheduler retains the warehouse task so it can execute the sale, but the generic automation save path cannot mutate that task's persisted warehouse configuration. The warehouse dialog remains the supported configuration entry point.

## Regression Coverage

Add an application test that saves a non-default warehouse interval, submits an automation state containing the old 3600-second warehouse task value, and verifies both stored warehouse settings and the live scheduler retain the non-default interval.
