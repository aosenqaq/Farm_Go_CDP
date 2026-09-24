# Friend Social Migration Design

## Goal

Migrate the reference project's `好友社交` capabilities into Farm_Go as working Go/Wails features, not static UI. The page must center on live friend management and move complex tools into focused second-level dialogs.

## Reference Sources

- Reference project: `E:\desktop\farm-tauri-core-copy-20260707-155223\farm-tauri`
- Friend list and action API: `core/src/gateway.js`
- Friend rule storage: `core/src/friend-rule-config-store.js`
- Protocol block request: `core/src/friend-block-protocol.js`
- Manual friend mischief protocol: `core/src/friend-mischief-protocol.js`
- Steal ranking store: `core/src/friend-steal-ranking-store.js`
- Visitor record store: `core/src/visitor-record-store.js`
- Dog guard scanner and cache: `core/src/dog-guard-scan.js`, `core/src/dog-guard-scan-cache.js`
- Protected friend GIDs: `core/src/protected-friend-gids.js`
- Current Farm_Go placeholder: `frontend/src/views/FarmWorkspaceView.tsx`
- Existing Farm_Go friend runtime slice: `internal/farm/automation/runtime_friend.go`

## Approved UI Direction

Use the refined A layout from the visual companion.

- The `好友社交` page is a friend-management workbench.
- Main content shows a live friend list with search, filters, status badges, quick row actions, and summary metrics.
- Top actions include `刷新好友`, `好友功能`, `排行榜`, and `导入导出`.
- `好友功能` opens a compact menu matching the reference-style feature list:
  - `查看封神榜`
  - `查看协议拉黑好友`
  - `清理无效黑名单`
  - `批量移除黑名单`
  - `批量移除白名单`
  - `读取护主犬`
- Complex flows use second-level dialogs instead of expanding the page:
  - blacklist and whitelist settings,
  - import and export,
  - protocol block list,
  - batch removal,
  - dog guard scan,
  - ranking views,
  - raw record viewers.

The UI must keep Farm_Go's current visual language: dark green sidebar, warm light surface, lime primary actions, compact 8px cards/dialogs, and dense operational tables.

## Functional Scope

### Friend List

The main page must call the live runtime method equivalent to reference `/api/friends`, using `gameCtl.getFriendList` through Farm_Go's runtime caller.

The returned state must include:

- friend rows with normalized `gid`, display name, remark, level, avatar if available, raw record, and work counts;
- action counts for steal, help, and mischief;
- rule marks for local blacklist, local whitelist, masked blacklist, protected friend, protocol blocked if known, and dog guard scan result if cached;
- summary metrics for total friends, actionable friends, rule counts, dog guard count, and refresh status.

The page must support filters for all friends, stealable, helpable, mischiefable, blacklist, whitelist, protected, and dog guard. Search must match gid, name, display name, and remark.

### Friend Actions

Row actions must execute real runtime operations:

- enter friend farm;
- manual steal;
- manual help;
- manual mischief;
- protocol block;
- local blacklist toggle;
- local whitelist toggle.

The manual steal path must append ranking records when a steal actually occurs, matching the reference project's `appendFriendStealRecordsFromVisits` behavior. Protected GIDs `1184649322` and `1142601927` must silently skip steal/help/mischief and must not write logs, counters, or rankings.

### Local Rule Dialog

The blacklist/whitelist dialog must edit account-scoped friend rules:

- whitelist enabled flag;
- whitelist scopes: `steal`, `help`, `mischief`;
- whitelist items;
- blacklist enabled flag;
- blacklist scopes excluding scopes already owned by whitelist;
- blacklist items;
- masked blacklist enabled flag;
- masked blacklist max level.

The dialog must normalize pasted lists by newline, comma, Chinese comma, semicolon, Chinese semicolon, enumeration comma, and pipe. Duplicate entries must be removed.

### Friend Feature Menu

The reference menu functions must be migrated as working features:

- `查看封神榜`: open a second-level dialog backed by the same social record/risk data used by rankings and protocol block evidence. If the reference bundle's exact presentation cannot be recovered from source, Farm_Go must provide a concrete risk-list dialog with real data and state the implementation basis in the plan.
- `查看协议拉黑好友`: call `gameCtl.getFriendBlockListByProtocol` and display the protocol-side block list.
- `清理无效黑名单`: refresh friends, remove numeric local blacklist/whitelist entries whose gids no longer exist in the current friend list, and report removed items.
- `批量移除黑名单`: remove selected items from local blacklist.
- `批量移除白名单`: remove selected items from local whitelist.
- `读取护主犬`: run the dog guard scanner as an asynchronous task with progress, stop, clear, and cached results.

### Import And Export

The import/export dialog must support selected data groups:

- local blacklist/whitelist rules;
- friend list snapshot;
- ranking records for stolen-by-me;
- visitor records and stolen-from-me records;
- dog guard scan cache.

Export writes JSON with a version, exported timestamp, account key, selected groups, and normalized payloads. Import validates version and selected groups before writing any local data. Runtime-only lists such as protocol block list are not imported directly; they can be exported for inspection.

### Rankings Dialog

The `排行榜` button opens one second-level dialog with top tabs:

- `偷取记录`;
- `被偷记录`;
- `访客记录`.

`偷取记录` must support:

- raw record view;
- ranking view;
- date ranges: today/current, 3 days, 7 days, 30 days, all;
- optional current-friend filtering.

`被偷记录` must support:

- raw record view;
- recent stolen-from-me ranking based on visitor records;
- the same date range controls, constrained by available visitor retention.

`访客记录` must support:

- live refresh from `gameCtl.getVisitorRecords`;
- normalized raw record view;
- action labels matching the reference project's visitor formatter.

The three tabs share one entry button and one dialog, as approved.

### Dog Guard Scan

Dog guard scanning must be implemented as an asynchronous runtime task:

- load cached state on dialog open;
- start scan with refresh, limit, skip scanned, and exclude known guard-dog options;
- stop scan by setting stop requested;
- clear cached results when not running;
- show total, scanned, current friend, guard dog count, errors, and per-friend rows;
- persist cache per account.

The scanner may pause automatic farming while it runs, matching the reference runtime behavior, because it enters friend farms and inspects runtime signals.

## Architecture

Go owns all business logic and persistence. React owns presentation, local filters, dialog state, and action dispatch.

Add focused Go packages under `internal/farm/social` for:

- friend normalization and rule config;
- friend action service;
- ranking and visitor record storage;
- dog guard scan state and cache;
- import/export payloads.

`app.go` exposes Wails methods that delegate to the social service. The frontend imports generated Wails bindings and never fabricates successful runtime results.

## Data Flow

1. React calls `FarmSocialState(refresh bool)`.
2. `App` builds a runtime caller from the current runtime manager and calls the social service.
3. The social service calls runtime methods such as `gameCtl.getFriendList`, reads local rule and cache data, normalizes rows, and returns a structured state.
4. React renders the friend table, filters locally, and opens dialogs for complex tools.
5. Mutating actions call focused Wails methods and then refresh social state or the affected dialog state.

## Error Handling

All actions return structured results with:

- `ok`;
- `status`: `ok`, `runtime_not_ready`, `unsupported_target`, `busy`, `skipped`, or `failed`;
- `message`;
- optional `data`.

The UI must show the result message and keep the page usable after failures. No button may claim an operation succeeded unless the Go service received a real successful runtime result or completed a local persisted change.

## Testing

Backend tests must cover:

- friend rule normalization and scope conflicts;
- friend list enrichment with blacklist, whitelist, masked blacklist, protected friend, and dog guard flags;
- protected GID skip behavior;
- protocol block and unblock request argument shape;
- batch blacklist and whitelist removal;
- invalid rule cleanup;
- steal record normalization, date ranges, and ranking summaries;
- visitor record normalization and stolen-from-me summaries;
- dog guard cache normalization and scanner state transitions with fake runtime caller;
- import/export validation and selected group handling.

Frontend tests must cover:

- the social page renders the friend table as the primary content;
- top actions open the correct dialogs/menus;
- friend filters and search reduce visible rows;
- ranking dialog top tabs, view mode toggle, and date range controls;
- rule dialog renders blacklist/whitelist controls;
- dog guard dialog renders progress and result rows;
- row action callbacks call the provided handlers with the expected action payloads.

Full verification before completion requires:

- `go test ./...`;
- `cd frontend && npm test`;
- `cd frontend && npm run build`;
- generated Wails bindings refreshed if new Wails methods are added.

## Migration Policy

The migration is not complete if it only renders UI or returns placeholder data. Each visible operation must be backed by one of:

- a real runtime call;
- persisted local state;
- a documented cache read;
- a structured error explaining why the runtime is unavailable.

Use TDD for each backend behavior and frontend interaction. Keep commits small by migration slice:

- design and plan docs;
- backend social models and storage;
- friend list and rule UI;
- friend actions and protocol block list;
- rankings and visitor records;
- dog guard scan;
- import/export;
- final integration and verification.
