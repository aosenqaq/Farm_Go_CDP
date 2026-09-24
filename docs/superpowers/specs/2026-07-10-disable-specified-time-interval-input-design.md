# Disable Specified-Time Interval Input Design

## Goal

Prevent accidental interval edits in the scheduler center when a reward task is configured to run at a specified daily time.

## Behavior

- For the eight reward tasks that support `ScheduleMode`, disable the scheduler-center interval input when the corresponding mode is `daily_time`.
- Keep the existing interval value visible and unchanged while the input is disabled.
- Restore editing automatically when the mode changes back to `interval`.
- Keep interval inputs enabled for non-reward tasks and reward tasks in interval mode.
- Priority, enabled state, manual execution, and timestamp columns remain unchanged.

## Implementation

Keep the behavior in `AutomationView.tsx`, where both scheduler task IDs and reward configuration are available.

Add an explicit task-to-schedule-mode-key mapping for the eight reward tasks, matching the existing interval and enabled-key maps. A small pure helper reads the mapped config value and returns `true` only for `daily_time`. The scheduler row uses this result for the interval input's native `disabled` property.

Do not derive the mode from `nextRunAt`: both interval and specified-time tasks expose that timestamp. Do not add a backend field or change the Wails contract for a presentation-only safety control.

Use native disabled behavior for semantics and keyboard protection. Add focused CSS only if the current browser styling does not make the disabled state sufficiently visible.

## State Handling

Disabling the input must not clear or rewrite `intervalSec`. Existing scheduler/config synchronization remains unchanged. Because a disabled input cannot emit changes, switching back to interval mode reveals the previously stored value and restores editing.

## Tests

Frontend tests should verify:

- A reward task with `ScheduleMode: daily_time` renders its interval input disabled.
- The same reward task with `ScheduleMode: interval` renders its interval input enabled.
- A non-reward task remains enabled even when unrelated reward configuration uses `daily_time`.
- The disabled input retains its interval value.

Run the focused `AutomationView` suite, the complete frontend suite, and the production frontend build.

## Non-Goals

- Do not change scheduler execution behavior or persistence.
- Do not remove or reset interval values.
- Do not disable priority, task enabled state, or manual execution.
- Do not redesign the scheduler-center table.
