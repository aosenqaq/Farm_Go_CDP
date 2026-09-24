# Mobile Auto-Sell Settings Design

## Goal

Make the `自动出售设置` dialog compact and easy to scan on the remote LAN WebUI phone layout while retaining its current settings model and save behavior.

## Approved Direction

Use the approved grouped compact form layout.

- The remote dialog remains a bottom Sheet with a fixed header and two-button footer.
- The scrollable body contains two compact, bordered groups: `执行方式` and `出售类型`.
- `启用自动出售` becomes a full-width row with a semantic checkbox styled as an on/off switch.
- `出售间隔` follows in the same group as a numeric input with the visible `分钟` unit; direct numeric input remains available.
- The four existing category checkboxes stay in a two-column grid under `出售类型`.

## Scope

- Update `WarehouseAutoSellDialog` markup only as needed to introduce form-group and toggle class names.
- Add remote-phone CSS under the existing `.app-shell-remote` breakpoint for compact grouped layout, touch sizing, the checkbox switch appearance, field/unit alignment, and category-grid spacing.
- Preserve the existing close, cancel, save, busy, error, category-toggle, minimum interval, and Wails save semantics.

## Non-Goals

- No changes to `WarehouseAutoSellSettings`, category keys, backend validation, persistence, scheduler configuration, or desktop dialog layout.
- Do not add preset intervals, new categories, or automatic immediate-sale actions.

## Interaction And Accessibility

- The real checkbox remains keyboard and screen-reader accessible; styling changes only its visual appearance.
- Labels remain associated with their inputs and retain their existing text.
- The footer continues to expose `取消` and `保存设置` as separate full-width mobile actions.

## Testing

- Add focused frontend tests asserting the grouped form markers and remote CSS rules.
- Run the affected Assets/Land frontend tests, the full frontend suite, and the production frontend build.
- Inspect the built dialog at 360px and 412px widths to ensure no empty vertical gaps, clipped footer, or overlapping category controls.

## Self-Review

- Completeness: all existing controls are accounted for.
- Consistency: layout changes are remote-phone scoped and reuse the established warehouse Sheet structure.
- Scope: the task is limited to one frontend dialog and its test coverage.
- Ambiguity: the interval remains a direct numeric input in minutes, with no new value semantics.
