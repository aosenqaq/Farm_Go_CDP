# Retire He Feng Reward Controls

## Context

The He Feng Travel event has ended. Its two automatic reward features must no longer be configurable or run automatically, while their implementation remains available for a future event with similar behavior.

The affected features are:

- `he_feng_travel_reward`
- `limited_seed_draw`, including its paid-draw option

## Design

### Frontend

Remove the He Feng Travel reward and draw rows from the reward settings list in `AutomationView`. Because the paid-draw checkbox is rendered inside the draw row, removing that row also removes the paid-draw configuration entry.

Keep task labels, event labels, scheduler state, and runtime result rendering unchanged. They remain useful for stored history, logs, and future reactivation.

### Backend configuration

Treat these configuration keys as retired switches:

- `autoFarmHeFengTravelRewardEnabled`
- `autoFarmLimitedSeedDrawEnabled`
- `autoFarmLimitedSeedDrawPaidEnabled`

`MergeConfigWithDefaults` must force all three values to `false` after merging caller-provided configuration. This ensures existing accounts with persisted `true` values cannot continue running an ended event after the controls disappear.

The default configuration keeps the keys with `false` values for compatibility. The recommended automation configuration must also disable both scheduler tasks and all three switches so applying recommendations cannot reactivate them.

### Retained implementation

Keep scheduler task definitions, configuration-key mappings, schedule metadata, runtime reward handlers, protocol calls, daily-state support, event labels, and log labels. No protocol or feature implementation is deleted.

### Data flow

Stored or submitted settings pass through `MergeConfigWithDefaults`, which normalizes the retired switches to `false`. State construction then derives both task and UI state from the normalized configuration, so the scheduler does not select either task. The frontend independently omits their controls from the reward settings page.

### Error handling

No new runtime error path is introduced. Direct internal task execution remains supported by retained code, while automatic scheduling is prevented through normalized configuration.

## Verification

- A backend test supplies all three retired switches as `true` and verifies merged configuration returns `false` for each.
- Recommended-configuration tests verify both tasks and all three switches are disabled.
- The reward-settings render test verifies the two rows, paid-draw checkbox, and their schedule fields are absent while other reward controls remain visible.
- Focused Go and frontend tests pass, followed by the broader relevant test suites.

## Non-goals

- Deleting runtime or protocol code.
- Removing historical task or event labels.
- Removing stored schedule values or daily-state fields.
- Building a generic activity-retirement framework.
