# Friend List Level Sort Design

## Goal

Add user-selectable ordering to the `好友社交` friend table without changing the default runtime order.

## UI

Place a compact sorting select immediately after the friend search input in `SocialView`'s existing toolbar. Its options are:

- `默认`
- `等级升序`
- `等级倒序`

The selected option uses the existing warm, compact toolbar-control styling. It is independent from the action filters (`全部`, `可偷`, `可帮`, and so on).

## Behavior

1. The table first applies the active action filter and search term.
2. The resulting rows are then ordered according to the selected mode.
3. `默认` preserves the original `current.friends` order.
4. `等级升序` orders numeric `friend.level` from low to high.
5. `等级倒序` orders numeric `friend.level` from high to low.
6. Friends with a missing or non-numeric level appear after friends with a numeric level in both level sort modes.
7. Rows with equal sortable levels retain their original relative order.
8. The selected sort mode remains active when the friend data refreshes during the open application session. It is component-local state and resets to `默认` when the application restarts.

## Architecture

Keep sorting entirely in `frontend/src/views/SocialView.tsx`. The backend friend payload, saved user preferences, Wails bindings, and friend-action behavior remain unchanged. A small exported pure helper may be used to make ordering deterministic and directly testable.

## Testing

Extend `frontend/src/views/SocialView.test.tsx` with a friend fixture containing distinct, equal, and missing levels. Verify:

- default mode keeps the original list order;
- ascending and descending modes order numeric levels correctly;
- missing levels are last;
- ties keep their source order;
- sorting applies to the list after search/filter selection;
- the visible selector exposes all three Chinese labels.

Run the focused frontend test, the complete frontend test suite, and the frontend production build.

## Out Of Scope

- Persisting sort mode to storage or account preferences.
- Changing backend APIs or data normalization.
- Adding sort controls to rankings, dog guard scans, or other social dialogs.
