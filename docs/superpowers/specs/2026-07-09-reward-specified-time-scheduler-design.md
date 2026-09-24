# Reward Specified-Time Scheduler Design

## Goal

Make reward tasks configured with "指定时间执行" use that schedule in the Go automation scheduler and expose the same next execution time in the scheduler center.

## Scope

This change covers the eight reward tasks that already expose interval and specified-time settings:

- `reward_claim`
- `svip_daily_gift`
- `monthly_card_reward`
- `mall_daily_fertilizer`
- `share_reward`
- `mail_reward`
- `he_feng_travel_reward`
- `limited_seed_draw`

No new frontend controls or scheduler task fields are required.

## Current State

The reward settings UI stores a schedule mode and time for every reward task by deriving these keys from the task interval key:

- `<Prefix>ScheduleMode`: `interval` or `daily_time`
- `<Prefix>ScheduleTime`: local `HH:mm`

The Go defaults contain these fields, but the scheduler currently ignores them. It initializes every task as immediately due, then calculates later runs from `IntervalSec` or failure backoff. The scheduler center only formats the backend-provided `SchedulerTask.NextRunAt`, so it cannot show the configured daily time when the backend does not calculate it.

## Design

### Schedule Metadata

Keep the schedule metadata inside the automation package. Add a task-to-prefix mapping for the eight reward tasks, or an equivalent helper that derives the following config keys for a task:

- schedule mode key
- schedule time key

The scheduler remains the single source of truth for execution and display time. The frontend must not independently calculate a daily timestamp.

### Next Specified Time

Add a focused helper that accepts configuration, task ID, and the scheduler's current time. It returns the next specified timestamp only when the task is a supported reward task and its mode is `daily_time`.

The timestamp uses the location of the supplied current time:

- If the configured time is later than the current local time, return today at that time.
- If the configured time is equal to or earlier than the current local time, return tomorrow at that time.
- Starting or reconfiguring the app after today's time does not trigger a catch-up run.
- Invalid or empty time text falls back to `08:00` for that task.

Equality belongs to the next day rule. This prevents an immediate run when configuration happens exactly at the selected minute.

### Scheduler Initialization And Reconfiguration

During `Scheduler.Configure`, align `runtime.NextRunAt` to the calculated specified timestamp for every enabled or disabled supported task in `daily_time` mode. This alignment must happen when the scheduler is first created and when settings change.

Interval-mode tasks keep the existing behavior. Switching from `daily_time` back to `interval` must clear the old daily timestamp and make the task immediately eligible, matching the scheduler's existing first-run behavior. Later interval runs continue to use completion time plus `IntervalSec` or the existing failure backoff.

### Execution Completion

After a specified-time reward task finishes, both success and failure schedule the next run at the next configured daily time. Interval-based success and failure retain their current interval and backoff calculations.

This prevents a successful run from drifting by `IntervalSec` and prevents failure retries from running outside the user-selected daily schedule.

### Scheduler Center

`SchedulerTask.NextRunAt` continues to be the scheduler-center contract. `AutomationView` keeps formatting the value as `MM-DD HH:mm` and displaying `-` only when the backend does not provide a timestamp.

Once a specified-time reward configuration is saved and the scheduler is reconfigured, the row must show today's configured time when it is still upcoming, otherwise tomorrow's configured time.

Existing open-dialog refresh merging must continue to update `nextRunAt` without overwriting unsaved priority, interval, or enabled edits.

## Daily Completion Interaction

Existing daily-once completion markers remain authoritative for supported daily-once tasks. If a task is already completed today, the scheduler may override its visible next run with the next valid day as it does today. The new specified-time calculation must preserve the configured hour and minute rather than forcing midnight.

Reward tasks without a daily completion marker still follow their configured daily timestamp and run once per scheduled occurrence.

## Error Handling

- Unsupported task IDs and `interval` mode return no specified schedule and use existing scheduling logic.
- Invalid schedule times use `08:00`; they must not make a task immediately due or enter a rapid loop.
- The calculation uses the scheduler's injected clock so behavior is deterministic in tests.
- No frontend parsing of schedule configuration is used as a fallback for missing backend state.

## Tests

Go tests should cover:

- Each of the eight reward task IDs resolves its schedule keys.
- An upcoming configured time resolves to today.
- An elapsed configured time resolves to tomorrow.
- A time equal to the current minute resolves to tomorrow.
- Invalid time falls back to the next `08:00` occurrence.
- Initial scheduler configuration exposes the specified `NextRunAt`.
- Reconfiguration updates `NextRunAt` after mode or time changes.
- Switching to interval mode removes the daily alignment and makes the task immediately eligible.
- Successful specified-time execution schedules the next configured daily occurrence.
- Failed specified-time execution also schedules the next configured daily occurrence.
- Daily-completed tasks retain the configured hour and minute in the visible next run.

Frontend tests should cover:

- A backend `nextRunAt` produced for a specified-time reward task renders as `MM-DD HH:mm` in the scheduler center.
- Existing runtime-state merging continues to accept refreshed `nextRunAt` values.

## Non-Goals

- Do not add a generic cron expression model.
- Do not change scheduler task persistence or the Wails data contract.
- Do not calculate the next daily timestamp separately in React.
- Do not catch up a missed daily execution after application startup.
- Do not redesign reward settings or scheduler-center layout.
