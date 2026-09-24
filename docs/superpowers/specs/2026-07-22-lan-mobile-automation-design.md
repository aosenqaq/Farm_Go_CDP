# LAN Mobile Automation Design

## Goal

Make the remote WebUI automation page practical at phone widths without
changing automation state, scheduler persistence, task execution, or the
recommended-configuration API. The mobile page must make feature groups easy
to scan and operate, and its scheduler and recommendation dialogs must always
leave the user a visible way to close, cancel, or confirm.

## Scope

This change applies only inside the remote shell at `max-width: 760px`.
Desktop automation layout and desktop dialogs retain their present table and
centered-dialog layouts. Existing callbacks, state normalization, save
semantics, scheduler task ordering, and recommended-configuration coverage
remain unchanged.

## Automation Page

The mobile page stays a single page with no nested menu. It presents:

1. The `农场自动化` heading and two clearly labelled secondary actions:
   `应用推荐配置` and `调度中心`. Labels stay visible; icons supplement labels
   rather than replacing them.
2. A compact running summary that keeps status, enabled-task count, current
   task, and next task readable at mobile width. Its right-aligned start/stop
   action is the page's third direct action, visually tied to the operating
   state instead of competing with the title.
3. A single-column feature list grouped into `自己的农场` (`own_base`,
   `planting`, `fertilizer`), `好友互动` (`friends`), and `奖励与商店`
   (`rewards`, `mystery_shop`). Unknown future feature IDs remain visible in a final
   `其他功能` group, preserving their input order and preventing data loss.
4. Every feature row keeps the existing bot icon, one-line title, one-line
   summary, ON/OFF control, and labelled settings entry. Long titles and
   summaries truncate within the row; no row creates horizontal scrolling.

Desktop feature cards keep their two-column grid. The same React data drives
both layouts: on desktop the grouping wrappers are visually transparent and
their headings are hidden; only the remote mobile breakpoint renders the
single-column grouped presentation.

## Mobile Sheets

The generic remote rule that forces dialogs to `100dvh` does not apply to the
automation scheduler, settings, or recommendation dialogs. These dialogs use
bottom-aligned sheets on remote mobile screens, leaving a small, dimmed part
of the automation page visible above the sheet.

### Scheduler

The scheduler sheet has a bounded height of approximately `88dvh`, with a
visible rounded top edge. Its title and close control occupy a fixed header;
the close button has an explicit `aria-label`. The global scheduler enabled
control and minimum-gap input remain directly below that header. The task
list is the only scrollable region.

Scheduler rows retain every existing editable field and action, but no longer
use the seven-column desktop table at mobile widths:

- task label and daily-complete badge appear first;
- priority and interval are labelled number fields on one compact row;
- last and next execution times appear as a two-part metadata row;
- the enable control and immediate-run command remain in the row;
- task-table headings are hidden on mobile because every value is locally
  labelled.

A fixed footer provides `取消` and `保存调度`. Cancel and the close button
discard unsaved scheduler edits exactly as the present close action does.
Save uses the existing `saveScheduler` handler, disabled/loading state, and
success/error message. The desktop save control remains in its current
location and the desktop footer is not rendered there.

### Recommendation Confirmation

The recommendation confirmation uses the same bottom-sheet treatment but is
shorter, capped near `70dvh`. Its header contains an always-visible close
button; its content body is the only scrollable region; its footer always
shows `取消` and `确认应用`.

The body explicitly distinguishes the two covered areas, `农场自动化` and
`仓库自动出售`, from the non-covered area, `不会覆盖`. Existing request,
success, loading, disabled, and backend-error behaviour is retained. Closing
or cancelling never invokes the apply API.

Feature-settings dialogs receive the shared bounded mobile sheet layout so
their own close and save controls are reachable. Their existing detailed
settings form and fieldset-disabled behavior are otherwise out of scope.

## Implementation Boundaries

- `frontend/src/views/AutomationView.tsx`: derive the stable mobile display
  sections from `featureGroups`; render grouping wrappers and mobile-local
  labels for scheduler row values; add explicit close labels and the
  scheduler footer while preserving existing handlers and desktop markup.
- `frontend/src/style.css`: add scoped `.app-shell-remote` mobile rules for
  the page action layout, run summary, grouped feature list, and the three
  automation sheets. Override the generic remote full-viewport dialog rule
  only for these automation dialog classes.
- `frontend/src/views/AutomationView.test.tsx`: cover group allocation and
  fallback ordering; assert grouped section markup, accessible close labels,
  scheduler task field labels/footer controls, and unchanged recommendation
  confirmation semantics. Add scoped CSS-source assertions where this test
  file already verifies responsive layout contracts.

## Accessibility And Error Handling

All controls remain semantic buttons, labels, inputs, and dialog surfaces.
The close controls receive explicit Chinese accessible names; the direct page
actions retain their existing accessible names. Scrollable interiors use
`min-height: 0` and vertical overflow containment so their content does not
scroll the sheet header or footer away. Long task names, timestamps, backend
errors, and recommendation descriptions may wrap or truncate but cannot
produce horizontal page scrolling.

No new network or persistence operation is introduced. Scheduler save and
recommended-configuration failures retain their current visible error state;
both sheets remain open and their retryable controls become usable again.

## Verification

1. Run focused `AutomationView` tests, then the complete frontend test suite
   and production build.
2. At a 360px-wide remote viewport, verify page actions have visible labels,
   feature groups are one column in the declared order, and toggles/settings
   remain touchable without horizontal overflow.
3. Open the scheduler with enough tasks to require scrolling. Verify the
   header close control and footer cancel/save controls remain visible while
   only tasks scroll; change a priority, interval, enabled flag, and run a
   task to confirm existing handlers are still used.
4. Open the recommendation confirmation with an error state and verify its
   close/cancel/confirm controls remain reachable, the body scrolls
   independently, and cancellation does not invoke the apply handler.
5. At a desktop viewport, verify the original two-column feature grid and
   desktop scheduler table are unchanged.

## Non-Goals

- No change to scheduler priorities, intervals, execution order, or task
  synchronization.
- No user-selectable recommendation subsets or new automation settings.
- No changes to desktop automation UI, API contracts, or LAN transport.
