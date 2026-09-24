# Friend Social Menu Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the friend row's individual actions with clear `操作` and `配置` menus, distinguish local lists from the system blacklist, and make the same interactions compact and usable in LAN phone mode.

**Architecture:** Keep `SocialView` as the owner of friend-menu, confirmation, and dialog state because it already owns row actions, protocol list state, and action-result feedback. Render an anchored fixed desktop menu outside the scroll-clipped friend table; when the same view is inside `.app-shell-remote` at phone width, render the identical command items in the existing `Dialog` bottom-sheet contract. Reuse all existing Wails actions, but replace the unsupported front-end `protocol_block` action with the established `block_friend` action.

**Tech Stack:** React 18, TypeScript, lucide-react, Vitest/react-test-renderer, CSS media queries, Go social service regression tests.

---

## File Structure

- Modify: `frontend/src/views/SocialView.tsx`
  - Own active desktop/mobile command menu state, system-block confirmation state, compact row triggers, all local/system user-facing terminology, and the reusable command-item renderer.
- Modify: `frontend/src/views/social/SocialRankingDialog.tsx`
  - Rename the ranking action and pass its display name with the GID to the parent confirmation flow.
- Modify: `frontend/src/style.css`
  - Replace old five-icon row styles with desktop menu styles and LAN compact two-button controls.
- Modify: `frontend/src/views/SocialView.test.tsx`
  - Cover menu contents, dynamic local-list labels, confirmation behavior, terminology, system blacklist management wording, and LAN CSS invariants.
- Modify: `frontend/src/views/social/SocialRankingDialog.test.tsx`
  - Cover the renamed ranking action and its `{ gid, displayName }` callback payload.
- Modify: `internal/farm/social/actions_test.go`
  - Lock the existing `block_friend` protocol invocation as the canonical system-blacklist action.

### Task 1: Lock The Canonical System-Blacklist Action

**Files:**
- Modify: `internal/farm/social/actions_test.go`

- [ ] **Step 1: Add the failing service test for the canonical action**

  Add this test after `TestFriendActionEnterCallsRuntime`:

  ```go
  func TestFriendActionBlocksSystemFriend(t *testing.T) {
      caller := &fakeRuntimeCaller{responses: map[string]any{
          "gameCtl.blockFriendByProtocol": map[string]any{"ok": true},
      }}
      service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

      result := service.Action(context.Background(), FriendActionRequest{
          Action:  "block_friend",
          Target:  "10001",
          DryRun:  true,
      })

      if !result.OK || result.Status != StatusOK {
          t.Fatalf("result = %#v", result)
      }
      if len(caller.calls) != 1 || caller.calls[0].method != "gameCtl.blockFriendByProtocol" {
          t.Fatalf("calls = %#v", caller.calls)
      }
      want := []any{map[string]any{"friendGid": 10001, "dryRun": true, "silent": true}}
      if !reflect.DeepEqual(caller.calls[0].args, want) {
          t.Fatalf("args = %#v, want %#v", caller.calls[0].args, want)
      }
  }
  ```

- [ ] **Step 2: Run the focused Go test**

  Run: `go test ./internal/farm/social -run TestFriendActionBlocksSystemFriend -count=1`

  Expected: PASS. The existing `block_friend` service branch already invokes the correct protocol method; the test records this required contract before UI callers are changed.

- [ ] **Step 3: Run the social package regression suite**

  Run: `go test ./internal/farm/social -count=1`

  Expected: PASS.

- [ ] **Step 4: Commit the contract test**

  ```bash
  git add internal/farm/social/actions_test.go
  git commit -m "test: cover system blacklist action"
  ```

### Task 2: Replace Friend Row Actions With Menus And Local/System Terminology

**Files:**
- Modify: `frontend/src/views/SocialView.tsx`
- Modify: `frontend/src/views/SocialView.test.tsx`

- [ ] **Step 1: Write failing SocialView behavior tests**

  Replace the old `renders blacklist and whitelist row actions with explicit rule labels` test with the following coverage. Add `type SocialActionRequest` to the existing `SocialView` type import. Keep the test-local `findButton` helper and add `clickRect()` beside it so command triggers have a deterministic desktop anchor.

  ```tsx
  function clickRect() {
    return {
      currentTarget: {
        getBoundingClientRect: () => ({ top: 20, right: 300, bottom: 52, left: 220, width: 80, height: 32 }),
      },
    };
  }

  it('groups each friend into operation and configuration menus', () => {
    const requests: SocialActionRequest[] = [];
    const renderer = TestRenderer.create(
      <SocialView state={state} onRefresh={() => undefined} onAction={(request) => { requests.push(request); }} />,
    );

    expect(findButton(renderer.root, '操作')).toBeDefined();
    expect(findButton(renderer.root, '配置')).toBeDefined();

    act(() => findButton(renderer.root, '操作').props.onClick(clickRect()));
    expect(textFromChildren(renderer.root)).toContain('查看');
    expect(textFromChildren(renderer.root)).toContain('偷菜');
    expect(textFromChildren(renderer.root)).toContain('捣乱');
    expect(textFromChildren(renderer.root)).toContain('帮助');
    act(() => findButton(renderer.root, '查看').props.onClick());
    expect(requests).toEqual([{ action: 'enter', target: '10001' }]);
  });

  it('uses local-list configuration labels without removing the other local list', () => {
    const requests: SocialActionRequest[] = [];
    const renderer = TestRenderer.create(
      <SocialView state={state} onRefresh={() => undefined} onAction={(request) => { requests.push(request); }} />,
    );

    act(() => findButton(renderer.root, '配置').props.onClick(clickRect()));
    expect(textFromChildren(renderer.root)).toContain('加入本地黑名单');
    expect(textFromChildren(renderer.root)).toContain('移出本地白名单');
    expect(textFromChildren(renderer.root)).toContain('加入系统黑名单');
    act(() => findButton(renderer.root, '加入本地黑名单').props.onClick());
    expect(requests).toEqual([{ action: 'blacklist_toggle', target: '10001' }]);
  });

  it('confirms before submitting a system blacklist request', async () => {
    const requests: SocialActionRequest[] = [];
    const renderer = TestRenderer.create(
      <SocialView state={state} onRefresh={() => undefined} onAction={(request) => { requests.push(request); return { ok: true, status: 'ok', message: '已加入系统黑名单' }; }} />,
    );

    act(() => findButton(renderer.root, '配置').props.onClick(clickRect()));
    act(() => findButton(renderer.root, '加入系统黑名单').props.onClick());
    expect(requests).toEqual([]);
    expect(textFromChildren(renderer.root)).toContain('确认加入系统黑名单');
    await act(async () => findButton(renderer.root, '确认加入').props.onClick());
    expect(requests).toEqual([{ action: 'block_friend', target: '10001' }]);
  });
  ```

  Update wording assertions for the feature dialog, local rule settings, local import/export actions, protocol list dialog, summary/filter chips, and the static stale-rule test. Assertions must expect `本地黑名单` / `本地白名单` and `系统黑名单` / `系统拉黑`, not the old ambiguous labels.

- [ ] **Step 2: Run the new tests to verify the old UI fails**

  Run: `npm --prefix frontend test -- src/views/SocialView.test.tsx`

  Expected: FAIL because the old view has individual `IconAction` buttons, lacks `操作`/`配置` triggers, lacks the confirmation dialog, and exposes legacy wording.

- [ ] **Step 3: Add explicit menu state and action helpers in `SocialView.tsx`**

  Add `ChevronDown` and `Bug` to the lucide imports. Remove `IconAction` after its callers are removed. Define these local types immediately above `SocialView`:

  ```tsx
  type FriendCommandGroup = 'actions' | 'config';

  type FriendCommandMenuState = {
    friend: FriendRow;
    group: FriendCommandGroup;
    top: number;
    right: number;
  };

  type SystemBlockTarget = {
    gid: string;
    displayName: string;
  };

  function localRuleBlockReason(friend: FriendRow, rules: FriendRules, scope: 'steal' | 'help' | 'mischief'): string | null {
    const blacklisted = isFriendBlacklistedByRules(friend, rules);
    const whitelisted = isFriendWhitelistedByRules(friend, rules);
    if (rules.blacklistEnabled && rules.blacklistScopes.includes(scope) && (blacklisted || friend.maskedBlocked)) {
      return friend.maskedBlocked ? '低等级屏蔽已跳过' : '本地黑名单已跳过';
    }
    if (rules.whitelistEnabled && rules.whitelistScopes.includes(scope) && !whitelisted) {
      return '不在本地白名单';
    }
    return null;
  }

  function friendOperationDisabledReason(friend: FriendRow, rules: FriendRules, action: 'steal' | 'help' | 'mischief'): string | null {
    const ruleReason = localRuleBlockReason(friend, rules, action);
    if (ruleReason) return ruleReason;
    if (action === 'steal' && !friend.stealable) return '当前无可偷作物';
    if (action === 'help' && !friend.helpable) return '当前无可帮忙地块';
    if (action === 'mischief' && !friend.mischiefable) return '当前无可捣乱地块';
    return null;
  }
  ```

  Add `rootRef`, `friendMenu`, `systemBlockTarget`, and `systemBlockPending` state in `SocialView`. Add an effect that closes `friendMenu` on document `pointerdown`, `Escape`, window resize, or captured scroll; the handler must ignore pointer events inside `rootRef` elements marked `[data-friend-command-menu]` and `[data-friend-command-trigger]`. Add `openFriendMenu(event, friend, group)` that derives `top` from `event.currentTarget.getBoundingClientRect().bottom + 6` and `right` from `(typeof window === 'undefined' ? rect.right : window.innerWidth) - rect.right`. Add `submitFriendAction(action, friend)` that closes a normal menu before awaiting `onAction`, but stores a `SystemBlockTarget` instead of sending `block_friend` immediately.

  Add `confirmSystemBlock()` that sends exactly `{ action: 'block_friend', target: systemBlockTarget.gid }`, disables the confirmation buttons while pending, invokes `onProtocolBlockList?.()` after completion, and finally clears the pending state and confirmation target.

- [ ] **Step 4: Replace the row markup and render shared command items**

  Change the friend table header to `好友`、`状态与名单`、`可执行`、`操作`; delete the duplicate standalone list column. Replace the five `IconAction` calls with this trigger pair:

  ```tsx
  <div className="social-row-commands">
    <button
      className="social-command-hit-area"
      type="button"
      data-friend-command-trigger
      aria-haspopup="menu"
      aria-expanded={friendMenu?.friend.gid === friend.gid && friendMenu.group === 'actions'}
      onClick={(event) => openFriendMenu(event, friend, 'actions')}
    >
      <span className="social-command-trigger primary">操作 <ChevronDown size={14} /></span>
    </button>
    <button
      className="social-command-hit-area"
      type="button"
      data-friend-command-trigger
      aria-haspopup="menu"
      aria-expanded={friendMenu?.friend.gid === friend.gid && friendMenu.group === 'config'}
      onClick={(event) => openFriendMenu(event, friend, 'config')}
    >
      <span className="social-command-trigger config">配置 <ChevronDown size={14} /></span>
    </button>
  </div>
  ```

  Define the shared menu body before `Dialog`, then render it in both desktop and LAN containers:

  ```tsx
  function FriendCommandItems({
    friend,
    group,
    rules,
    onSelect,
  }: {
    friend: FriendRow;
    group: FriendCommandGroup;
    rules: FriendRules;
    onSelect: (action: 'enter' | 'steal' | 'help' | 'mischief' | 'blacklist_toggle' | 'whitelist_toggle' | 'block_friend', friend: FriendRow) => void;
  }) {
    if (group === 'actions') {
      const items = [
        { action: 'enter' as const, label: '查看', icon: <Eye size={15} />, reason: null },
        { action: 'steal' as const, label: '偷菜', icon: <Swords size={15} />, reason: friendOperationDisabledReason(friend, rules, 'steal') },
        { action: 'mischief' as const, label: '捣乱', icon: <Bug size={15} />, reason: friendOperationDisabledReason(friend, rules, 'mischief') },
        { action: 'help' as const, label: '帮助', icon: <HeartHandshake size={15} />, reason: friendOperationDisabledReason(friend, rules, 'help') },
      ];
      return <div className="social-command-menu-items">{items.map((item) => (
        <button className="social-command-menu-item" type="button" disabled={Boolean(item.reason)} key={item.action} onClick={() => onSelect(item.action, friend)}>
          {item.icon}<span>{item.label}</span>{item.reason && <span className="social-command-menu-reason">{item.reason}</span>}
        </button>
      ))}</div>;
    }
    const blacklisted = isFriendBlacklistedByRules(friend, rules);
    const whitelisted = isFriendWhitelistedByRules(friend, rules);
    const items = [
      { action: 'blacklist_toggle' as const, label: blacklisted ? '移出本地黑名单' : '加入本地黑名单', icon: <Ban size={15} />, system: false },
      { action: 'whitelist_toggle' as const, label: whitelisted ? '移出本地白名单' : '加入本地白名单', icon: <Check size={15} />, system: false },
      { action: 'block_friend' as const, label: '加入系统黑名单', icon: <Ban size={15} />, system: true },
    ];
    return <div className="social-command-menu-items">{items.map((item) => (
      <button className={item.system ? 'social-command-menu-item system' : 'social-command-menu-item'} type="button" key={item.action} onClick={() => onSelect(item.action, friend)}>
        {item.icon}<span>{item.label}</span>
      </button>
    ))}</div>;
  }
  ```

  Render a fixed `.social-command-popover` after `.social-workbench` when `friendMenu` is set. Give it `role="menu"`, `data-friend-command-menu`, and its saved `top`/`right` values. Pass its selected `friend`, `group`, `current.rules`, and `submitFriendAction` to `FriendCommandItems`.

  Add a `Dialog` titled `确认加入系统黑名单` after the other dialogs. Its body must display `${systemBlockTarget.displayName} · ${systemBlockTarget.gid}` and state that the action affects the game-side relationship. Its footer contains `取消` and a disabled-while-pending primary `确认加入` button. Extend the local `Dialog` component with an optional `className?: string`, preserving the existing `wide` behavior.

  Rename all SocialView-facing local/system text exactly as follows:

  ```tsx
  <Metric label="本地黑名单" value={current.summary.blacklisted + current.summary.maskedBlocked} tone="amber" />
  <Metric label="本地白名单" value={current.summary.whitelisted} />
  ['blacklist', '本地黑名单'],
  ['whitelist', '本地白名单'],
  <Pill label="本地黑名单" tone="dark" />
  <Pill label="本地白名单" tone="green" />
  ```

  Change the feature dialog labels to `查看系统拉黑`, `清理无效本地黑名单`, `批量移除本地黑名单`, `批量移除本地白名单`, and `本地名单设置`. Change all local rule settings and import/export controls to `本地黑名单` / `本地白名单`. Change the protocol dialog title, empty state, status, and unblock controls to `系统黑名单`, `暂无系统拉黑好友`, `系统拉黑`, and `解除系统拉黑`.

- [ ] **Step 5: Run SocialView tests and fix only failures in this task**

  Run: `npm --prefix frontend test -- src/views/SocialView.test.tsx`

  Expected: PASS, including existing ranking, rule import/export, dog guard, and protocol-unblock cases updated only for the required labels.

- [ ] **Step 6: Commit the row-menu implementation**

  ```bash
  git add frontend/src/views/SocialView.tsx frontend/src/views/SocialView.test.tsx
  git commit -m "feat: organize friend row commands"
  ```

### Task 3: Route Ranking System-Blacklist Requests Through The Same Confirmation Flow

**Files:**
- Modify: `frontend/src/views/social/SocialRankingDialog.tsx`
- Modify: `frontend/src/views/social/SocialRankingDialog.test.tsx`
- Modify: `frontend/src/views/SocialView.tsx`

- [ ] **Step 1: Update the ranking dialog test first**

  In `renders page summary and O(page) row fields, rank, items, actions, and dialog commands`, rename the spy to `onSystemBlock`, pass it in the dialog props, then replace the final assertion with:

  ```tsx
  act(() => button(renderer.root, '加入系统黑名单').props.onClick());
  expect(onSystemBlock).toHaveBeenCalledWith({ gid: '10002', displayName: '目标好友' });
  ```

  Update `dialogProps` to require `onSystemBlock: () => undefined` instead of `onProtocolBlock`.

- [ ] **Step 2: Run the focused ranking test and verify failure**

  Run: `npm --prefix frontend test -- src/views/social/SocialRankingDialog.test.tsx`

  Expected: FAIL because the current prop is `onProtocolBlock`, the button label is `协议拉黑`, and it only passes a string.

- [ ] **Step 3: Change the ranking dialog's callback contract**

  In `SocialRankingDialog.tsx`, replace the prop with the following type and thread it through `SocialRankingDialog` and `RankingRow`:

  ```tsx
  onSystemBlock: (target: { gid: string; displayName: string }) => void | Promise<void>;
  ```

  Replace the ranking row action with:

  ```tsx
  <button
    className="secondary-button social-action-button compact"
    type="button"
    onClick={() => onSystemBlock({ gid: row.actionTarget || '', displayName: row.displayName })}
  >
    <Ban size={14} />
    加入系统黑名单
  </button>
  ```

  In `SocialView.tsx`, pass `onSystemBlock={(target) => setSystemBlockTarget(target)}`. Do not call `onAction` from the ranking child; only `confirmSystemBlock()` may send `block_friend`.

- [ ] **Step 4: Add a parent integration test, then run ranking and SocialView tests**

  Add this test to `SocialView.test.tsx` and add `type RankingPage` to its ranking type import:

  ```tsx
  it('confirms a ranking system-blacklist request through the parent action flow', async () => {
    const requests: SocialActionRequest[] = [];
    const rankingPage: RankingPage = {
      ok: true,
      status: 'ok',
      message: '',
      tab: 'stolenByMe',
      viewMode: 'ranking',
      dateRange: 'current',
      summary: { visitorCount: 0, stolenFromMeCount: 0, stolenByMeCount: 1, stolenByMeRecordCount: 1 },
      rows: [{
        key: 'ranking:10002', kind: 'stolenRanking', timeMS: 0, displayName: '排行榜目标', rank: 1,
        eventCount: 1, stealCount: 1, items: [], actionTarget: '10002',
      }],
      hasMore: false,
    };
    const renderer = TestRenderer.create(
      <SocialView
        state={state}
        initialRankingOpen
        rankingPreferences={{ stolenByMeViewMode: 'ranking', stolenFromMeViewMode: 'timeline' }}
        onRefresh={() => undefined}
        onAction={(request) => { requests.push(request); return { ok: true, status: 'ok', message: '已加入系统黑名单' }; }}
        onRankings={async () => rankingPage}
      />,
    );

    await act(async () => { await Promise.resolve(); });
    act(() => findButton(renderer.root, '加入系统黑名单').props.onClick());
    expect(requests).toEqual([]);
    await act(async () => findButton(renderer.root, '确认加入').props.onClick());
    expect(requests).toEqual([{ action: 'block_friend', target: '10002' }]);
  });
  ```

  Then run:

  Run: `npm --prefix frontend test -- src/views/social/SocialRankingDialog.test.tsx src/views/SocialView.test.tsx`

  Expected: PASS. The ranking action should open the same confirmation layer in its parent and no test or source text should contain the outbound action `protocol_block`.

- [ ] **Step 5: Commit the ranking integration**

  ```bash
  git add frontend/src/views/social/SocialRankingDialog.tsx frontend/src/views/social/SocialRankingDialog.test.tsx frontend/src/views/SocialView.tsx
  git commit -m "fix: unify system blacklist entry points"
  ```

### Task 4: Add Desktop Menus And Compact LAN Phone Controls

**Files:**
- Modify: `frontend/src/style.css`
- Modify: `frontend/src/views/SocialView.tsx`
- Modify: `frontend/src/views/SocialView.test.tsx`

- [ ] **Step 1: Add failing CSS contract assertions**

  Replace the old `.social-icon-action-label` assertion in `defines compact remote social rows and bounded child sheets` with the following assertions:

  ```tsx
  expect(css).toContain('.social-row-commands');
  expect(css).toContain('.social-command-trigger');
  expect(css).toContain('.social-command-popover');
  expect(css).toContain('.app-shell-remote .social-row-commands');
  expect(css).toContain('grid-template-columns: repeat(2, minmax(0, 1fr));');
  expect(css).toContain('min-height: 42px;');
  expect(css).toContain('min-height: 32px;');
  expect(css).toContain('.app-shell-remote .social-command-trigger svg');
  ```

  Add a source assertion that `SocialView.tsx` contains `data-friend-command-menu` and `className="social-command-dialog"` so LAN mode has a bottom-sheet counterpart rather than a clipped popover.

- [ ] **Step 2: Run the focused test and verify failure**

  Run: `npm --prefix frontend test -- src/views/SocialView.test.tsx`

  Expected: FAIL because the command-menu classes and phone-size invariants do not yet exist.

- [ ] **Step 3: Add the desktop command-menu CSS**

  Replace `.social-row-actions` and `.social-icon-action*` declarations with these focused rules near the existing friend row styles:

  ```css
  .social-friend-row {
    grid-template-columns: minmax(150px, 1.2fr) minmax(126px, 0.9fr) minmax(132px, 1fr) minmax(184px, auto);
  }

  .social-row-commands {
    display: flex;
    justify-content: flex-end;
    gap: 6px;
  }

  .social-command-hit-area {
    display: inline-grid;
    min-height: 32px;
    padding: 0;
    border: 0;
    background: transparent;
    cursor: pointer;
  }

  .social-command-trigger {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 5px;
    min-width: 76px;
    min-height: 32px;
    padding: 0 9px;
    border: 1px solid #d6c9ae;
    border-radius: 6px;
    color: #20271f;
    background: #fffaf0;
    font: inherit;
    font-size: 12px;
    font-weight: 900;
    cursor: pointer;
  }

  .social-command-trigger.primary {
    border-color: #20271f;
    color: #f8f3e8;
    background: #20271f;
  }

  .social-command-trigger.config {
    border-color: #a9c7a9;
    color: #1f5f26;
    background: #edf8d2;
  }

  .social-command-popover {
    position: fixed;
    z-index: 28;
    width: min(248px, calc(100vw - 24px));
    overflow: hidden;
    border: 1px solid #d6c9ae;
    border-radius: 8px;
    background: #fffdf7;
    box-shadow: 0 14px 36px rgba(31, 39, 31, 0.16);
  }

  .social-command-menu-item {
    display: grid;
    grid-template-columns: 18px minmax(0, 1fr);
    gap: 8px;
    width: 100%;
    min-height: 38px;
    padding: 7px 10px;
    border: 0;
    border-bottom: 1px solid #eee4d1;
    color: #20271f;
    background: transparent;
    font: inherit;
    font-size: 12px;
    font-weight: 850;
    text-align: left;
    cursor: pointer;
  }

  .social-command-menu-item:last-child { border-bottom: 0; }
  .social-command-menu-item:disabled { color: #968f7f; cursor: not-allowed; }
  .social-command-menu-item.system { color: #8a3523; }
  .social-command-menu-reason { grid-column: 2; color: #776e5b; font-size: 11px; font-weight: 700; }
  ```

- [ ] **Step 4: Render a LAN bottom sheet instead of the desktop popover**

  Add `isRemotePhone` state in `SocialView`, updated from `window.matchMedia('(max-width: 760px)')` and `rootRef.current?.closest('.app-shell-remote')`. Add `ref={rootRef}` to the root `<section>`. When `friendMenu && isRemotePhone`, render the same command list inside:

  ```tsx
  <Dialog
    title={friendMenu.group === 'actions' ? '操作' : '配置'}
    className="social-command-dialog"
    onClose={() => setFriendMenu(null)}
  >
    <div className="social-command-context">
      {friendMenu.friend.displayName} · {friendMenu.friend.gid}
    </div>
    <FriendCommandItems
      friend={friendMenu.friend}
      group={friendMenu.group}
      rules={current.rules}
      onSelect={submitFriendAction}
    />
  </Dialog>
  ```

  The desktop popover and the LAN dialog must use the same `FriendCommandItems` component so label order, disabled reasons, local toggle labels, and system confirmation behavior cannot drift.

- [ ] **Step 5: Add LAN compact-control CSS in the existing final remote-phone media block**

  Delete the old remote `.social-row-actions` and `.social-icon-action*` rules. Add these declarations in the `@media (max-width: 760px)` `.app-shell-remote` section:

  ```css
  .app-shell-remote .social-friend-row {
    grid-template-columns: minmax(0, 1fr) 114px;
    grid-template-rows: auto auto;
    gap: 5px 7px;
    min-height: 70px;
    padding: 9px 10px;
  }

  .app-shell-remote .social-friend-row > :nth-child(2),
  .app-shell-remote .social-friend-row > :nth-child(3) {
    grid-column: 1;
  }

  .app-shell-remote .social-row-commands {
    grid-column: 2;
    grid-row: 1 / span 2;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    align-self: center;
    gap: 4px;
    width: 114px;
  }

  .app-shell-remote .social-command-hit-area {
    display: grid;
    min-height: 42px;
    place-items: center;
  }

  .app-shell-remote .social-command-trigger {
    width: 100%;
    min-width: 0;
    min-height: 32px;
    padding: 0 5px;
    font-size: 10px;
  }

  .app-shell-remote .social-command-trigger svg { display: none; }
  .app-shell-remote .social-command-popover { display: none; }
  .app-shell-remote .social-command-dialog .social-command-menu-item { min-height: 42px; }
  ```

  The trigger markup in Task 2 makes `.social-command-hit-area` the outer button, so every pixel in the `42px` high area activates the command. Keep its inner `.social-command-trigger` at `32px` high; do not increase the visual label height or restore vertical stacking.

- [ ] **Step 6: Run the focused frontend tests**

  Run: `npm --prefix frontend test -- src/views/SocialView.test.tsx src/views/social/SocialRankingDialog.test.tsx`

  Expected: PASS. The source and CSS assertions prove desktop menus have an escape route and LAN cards use a one-row compact pair with a `42px` touch region and `32px` visual button.

- [ ] **Step 7: Commit the responsive styling**

  ```bash
  git add frontend/src/style.css frontend/src/views/SocialView.tsx frontend/src/views/SocialView.test.tsx
  git commit -m "style: compact LAN friend controls"
  ```

### Task 5: Build And Regression Verify The Complete Feature

**Files:**
- Verify: `frontend/src/views/SocialView.tsx`
- Verify: `frontend/src/views/social/SocialRankingDialog.tsx`
- Verify: `frontend/src/style.css`
- Verify: `frontend/src/views/SocialView.test.tsx`
- Verify: `frontend/src/views/social/SocialRankingDialog.test.tsx`
- Verify: `internal/farm/social/actions_test.go`

- [ ] **Step 1: Run focused frontend and Go suites**

  Run: `npm --prefix frontend test -- src/views/SocialView.test.tsx src/views/social/SocialRankingDialog.test.tsx`

  Expected: PASS.

  Run: `go test ./internal/farm/social -count=1`

  Expected: PASS.

- [ ] **Step 2: Build the frontend**

  Run: `npm --prefix frontend run build`

  Expected: TypeScript completes with no errors and Vite reports a successful production build.

- [ ] **Step 3: Run repository regression tests**

  Run: `go test ./...`

  Expected: PASS.

  Run: `npm --prefix frontend test`

  Expected: PASS.

- [ ] **Step 4: Perform the responsive manual check**

  Start the normal development surface and inspect `好友社交` at desktop width, LAN `760px`, and LAN `360px`:

  1. Desktop: `操作` and `配置` open separately, do not clip inside the table, close on `Escape` and outside click, and include the exact approved item ordering.
  2. Desktop: select local membership and verify only the selected local entry changes to `移出`; the other local-list entry remains available.
  3. Desktop: initiate system block from a friend and from a ranking row; both require confirmation and send `block_friend` only after confirmation.
  4. LAN 360px: the two buttons are horizontal, visually compact, and the card has no horizontal page scroll; each button still responds when tapped across its 42px high hit area.
  5. LAN 360px: both command groups open as bottom sheets with the friend name/GID context and reachable close action; local and system blacklist management retain their existing actions.

- [ ] **Step 5: Inspect the final change set before handing off**

  Run: `git diff --check`

  Expected: no whitespace errors. Then run `git status --short` and confirm any unrelated pre-existing changes remain unstaged and untouched.
