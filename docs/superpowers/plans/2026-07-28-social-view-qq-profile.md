# Social View QQ Profile Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a QQ-only `查看好友QQ` row action that resolves a friend through the visitor protocol and immediately asks QQ to display its native friend dialog with `来自QQ农场` as the verification text.

**Architecture:** The React workspace passes its already-loaded runtime target into `SocialView`, which renders the command only for `qq_ws`. The social service owns the fixed, guarded runtime call and returns a sanitized result; `App.FarmSocialAction` independently rejects `view_qq` for any non-QQ target.

**Tech Stack:** Go 1.25, Wails v2, React 18, TypeScript, Vitest, react-test-renderer.

---

## File Structure

- `internal/farm/social/actions.go`: dispatch and sanitize the focused `view_qq` runtime operation.
- `internal/farm/social/actions_test.go`: service-level contract tests for request arguments and sanitized results.
- `app.go`: prevent `view_qq` from being dispatched outside the active QQ runtime.
- `app_test.go`: exercise the App-boundary QQ target restriction.
- `frontend/src/views/FarmWorkspaceView.tsx`: derive the QQ capability from the existing runtime status and pass it to `SocialView`.
- `frontend/src/views/FarmWorkspaceView.test.tsx`: prove the workspace derives that capability from `status.target`.
- `frontend/src/views/SocialView.tsx`: render the command only for QQ and keep it disabled while its request is in flight.
- `frontend/src/views/SocialView.test.tsx`: verify visibility, payload, and duplicate-submission protection.

### Task 1: Add the Sanitized Social Service Action

**Files:**
- Modify: `internal/farm/social/actions_test.go`
- Modify: `internal/farm/social/actions.go`

- [ ] **Step 1: Write the failing service tests**

Append these tests after `TestFriendActionProtectedGIDSkipsWithoutRuntimeCall`:

```go
func TestFriendActionViewQQCallsGuardedRuntimeOperation(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.addFriendByGidDiagnostic": map[string]any{
			"ok": true, "invoked": true, "hostGid": float64(10001),
			"resolvedFrom": "basic.open_id",
		},
	}}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "view_qq", Target: "10001"})

	if !result.OK || result.Status != StatusOK || result.Message != "已请求打开 QQ 原生好友对话框。" {
		t.Fatalf("result = %#v", result)
	}
	wantArgs := []any{map[string]any{"hostGid": 10001, "verifyMsg": "来自QQ农场", "silent": true}}
	if len(caller.calls) != 1 || caller.calls[0].method != "gameCtl.addFriendByGidDiagnostic" || !reflect.DeepEqual(caller.calls[0].args, wantArgs) {
		t.Fatalf("calls = %#v, want args = %#v", caller.calls, wantArgs)
	}
	data := result.Data.(map[string]any)
	if !reflect.DeepEqual(data, map[string]any{"action": "view_qq", "gid": 10001, "invoked": true}) {
		t.Fatalf("data = %#v", data)
	}
}

func TestFriendActionViewQQDoesNotExposeRuntimePayloadOnFailure(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.addFriendByGidDiagnostic": map[string]any{
			"ok": false,
			"reason": "reply_open_id_missing",
			"basic": map[string]any{"open_id": "A1B2C3D4E5F60708192A3B4C5D6E7F80"},
		},
	}}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "view_qq", Target: "10001"})

	if result.OK || result.Status != StatusFailed || result.Message != "QQ 目标信息不完整，已阻止打开对话框。" || result.Data != nil {
		t.Fatalf("result = %#v", result)
	}
}
```

- [ ] **Step 2: Run the focused service tests and observe RED**

Run:

```powershell
go test ./internal/farm/social -run '^(TestFriendActionViewQQCallsGuardedRuntimeOperation|TestFriendActionViewQQDoesNotExposeRuntimePayloadOnFailure)$' -count=1
```

Expected: FAIL because `view_qq` is still an unknown social action.

- [ ] **Step 3: Implement the minimal guarded action**

In `internal/farm/social/actions.go`, add this action to the final action switch:

```go
case "view_qq":
	return s.viewQQFriend(ctx, gid)
```

Add the following helpers near `callRuntime`. Do not use `callRuntime`, because
it returns the complete runtime payload through `ActionResult.Data`.

```go
const qqFriendVerificationMessage = "来自QQ农场"

func (s *Service) viewQQFriend(ctx context.Context, gid int) ActionResult {
	value, err := s.caller.Call(ctx, "gameCtl.addFriendByGidDiagnostic", []any{map[string]any{
		"hostGid":   gid,
		"verifyMsg": qqFriendVerificationMessage,
		"silent":    true,
	}}, 30*time.Second)
	if err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "打开 QQ 好友对话框失败：" + err.Error()}
	}
	result := mapFromAny(value)
	if result["ok"] != true || result["invoked"] != true {
		return ActionResult{OK: false, Status: StatusFailed, Message: viewQQFailureMessage(fmt.Sprint(result["reason"]))}
	}
	return ActionResult{
		OK: true, Status: StatusOK, Message: "已请求打开 QQ 原生好友对话框。",
		Data: map[string]any{"action": "view_qq", "gid": gid, "invoked": true},
	}
}

func viewQQFailureMessage(reason string) string {
	switch reason {
	case "visit_query_failed":
		return "无法读取该好友的 QQ 信息。"
	case "reply_gid_mismatch":
		return "QQ 目标校验失败，已阻止打开对话框。"
	case "reply_open_id_missing":
		return "QQ 目标信息不完整，已阻止打开对话框。"
	case "qq_add_friend_api_unavailable":
		return "当前 QQ 客户端不支持打开好友对话框。"
	default:
		return "打开 QQ 好友对话框失败。"
	}
}
```

Keep the existing nil-caller check before this switch. It continues to return
the established `runtime_not_ready` result before any runtime request.

- [ ] **Step 4: Run the focused service tests and observe GREEN**

Run:

```powershell
go test ./internal/farm/social -run '^(TestFriendActionViewQQCallsGuardedRuntimeOperation|TestFriendActionViewQQDoesNotExposeRuntimePayloadOnFailure)$' -count=1
```

Expected: PASS with both tests green.

- [ ] **Step 5: Run the social package regression suite**

Run:

```powershell
go test ./internal/farm/social -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the service slice**

```powershell
git add -- internal/farm/social/actions.go internal/farm/social/actions_test.go
git commit -m "feat: add QQ friend profile social action"
```

### Task 2: Reject the Action Outside the QQ Runtime

**Files:**
- Modify: `app_test.go`
- Modify: `app.go`

- [ ] **Step 1: Write the failing App-boundary test**

Append this after `TestFarmSocialActionIsExposed`:

```go
func TestFarmSocialActionViewQQRejectsNonQQRuntime(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.addFriendByGidDiagnostic": map[string]any{"ok": true, "invoked": true},
	})
	app.manager.SetStatus(farmruntime.Status{Target: string(farmruntime.RuntimeTargetWeChatCDP)})

	result := app.FarmSocialAction(social.FriendActionRequest{Action: "view_qq", Target: "10001"})

	if result.OK || result.Status != social.StatusUnsupported || result.Message != "查看好友QQ仅支持 QQ 运行链路。" {
		t.Fatalf("result = %#v", result)
	}
	if link.called("gameCtl.addFriendByGidDiagnostic") {
		t.Fatalf("non-QQ request must not reach runtime: %#v", link.calls)
	}
}
```

- [ ] **Step 2: Run the focused App test and observe RED**

Run:

```powershell
go test . -run '^TestFarmSocialActionViewQQRejectsNonQQRuntime$' -count=1
```

Expected: FAIL because `FarmSocialAction` still delegates `view_qq` without a
runtime-target restriction.

- [ ] **Step 3: Add the App target guard**

In `app.go`, before the current social-service delegation in
`FarmSocialAction`, add:

```go
if strings.TrimSpace(input.Action) == "view_qq" && a.manager.Status().Target != string(farmruntime.RuntimeTargetQQWS) {
	return social.ActionResult{
		OK: false, Status: social.StatusUnsupported,
		Message: "查看好友QQ仅支持 QQ 运行链路。",
	}
}
```

Leave the existing authorization check first. This guard rejects handcrafted
Wails requests while normal QQ requests continue to reach the focused service
action from Task 1.

- [ ] **Step 4: Run the focused App test and observe GREEN**

Run:

```powershell
go test . -run '^TestFarmSocialActionViewQQRejectsNonQQRuntime$' -count=1
```

Expected: PASS.

- [ ] **Step 5: Run the App social exposure regression tests**

Run:

```powershell
go test . -run '^TestFarmSocial(State|Action|ProtocolBlockList).*' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the App guard**

```powershell
git add -- app.go app_test.go
git commit -m "feat: guard QQ friend dialog by runtime target"
```

### Task 3: Wire QQ Visibility Into the Social Operation Menu

**Files:**
- Modify: `frontend/src/views/FarmWorkspaceView.test.tsx`
- Modify: `frontend/src/views/FarmWorkspaceView.tsx`
- Modify: `frontend/src/views/SocialView.test.tsx`
- Modify: `frontend/src/views/SocialView.tsx`

- [ ] **Step 1: Write the failing workspace and menu tests**

Add this test to `frontend/src/views/FarmWorkspaceView.test.tsx`:

```tsx
it('passes the QQ runtime capability to the social view', () => {
  const qq = renderToStaticMarkup(
    <FarmWorkspaceView area="social" status={status} guardStatus={guardStatus} bindingStatus={null} events={[]}
      onRefresh={noop} onLaunch={noop} onRestart={noop} onToggleGuard={noop} />,
  );
  const nonQQ = renderToStaticMarkup(
    <FarmWorkspaceView area="social" status={{ ...status, target: 'wechat_cdp' }} guardStatus={guardStatus} bindingStatus={null} events={[]}
      onRefresh={noop} onLaunch={noop} onRestart={noop} onToggleGuard={noop} />,
  );

  expect(qq).toContain('data-qq-runtime="true"');
  expect(nonQQ).toContain('data-qq-runtime="false"');
});
```

Add these tests to `frontend/src/views/SocialView.test.tsx` inside the existing
`describe('SocialView', ...)` block:

```tsx
it('shows 查看好友QQ only for the QQ runtime', () => {
  const qqRenderer = TestRenderer.create(<SocialView state={state} isQQRuntime onRefresh={() => undefined} onAction={() => undefined} />);
  act(() => findButton(qqRenderer.root, '操作').props.onClick(clickRect()));
  expect(findButton(qqRenderer.root, '查看好友QQ')).toBeTruthy();

  const nonQQRenderer = TestRenderer.create(<SocialView state={state} isQQRuntime={false} onRefresh={() => undefined} onAction={() => undefined} />);
  act(() => findButton(nonQQRenderer.root, '操作').props.onClick(clickRect()));
  expect(JSON.stringify(nonQQRenderer.toJSON())).not.toContain('查看好友QQ');
});

it('submits 查看好友QQ once and keeps it disabled while pending', async () => {
  const pending = deferred<SocialActionResult>();
  const requests: SocialActionRequest[] = [];
  const renderer = TestRenderer.create(
    <SocialView state={state} isQQRuntime onRefresh={() => undefined} onAction={(request) => {
      requests.push(request);
      return pending.promise;
    }} />,
  );
  act(() => findButton(renderer.root, '操作').props.onClick(clickRect()));

  await act(async () => {
    void findButton(renderer.root, '查看好友QQ').props.onClick();
    await Promise.resolve();
  });

  expect(requests).toEqual([{ action: 'view_qq', target: '10001' }]);
  expect(findButton(renderer.root, '打开QQ中').props.disabled).toBe(true);
  await act(async () => { pending.resolve({ ok: true, status: 'ok', message: '已请求打开 QQ 原生好友对话框。' }); });
});
```

- [ ] **Step 2: Run the focused frontend tests and observe RED**

Run:

```powershell
Set-Location frontend
npm test -- src/views/FarmWorkspaceView.test.tsx src/views/SocialView.test.tsx
```

Expected: FAIL because neither `isQQRuntime` nor the `查看好友QQ` command
exists.

- [ ] **Step 3: Implement the capability wiring and pending command state**

In `FarmWorkspaceView.tsx`, pass the exact target-derived flag and a stable
capability to `SocialView`:

```tsx
<SocialView
  isQQRuntime={status.target === 'qq_ws'}
  // retain every existing SocialView prop
/>
```

Extend `SocialViewProps` with `isQQRuntime?: boolean`, default it to `false`,
and render a stable marker on the existing root section:

```tsx
<section className="view-stack fill social-view" data-qq-runtime={isQQRuntime ? 'true' : 'false'}>
```

Add `const [qqProfilePendingGID, setQQProfilePendingGID] = useState<number | null>(null);`.
Update `submitFriendCommand` so `view_qq` keeps its menu open and blocks its
own duplicate request while awaiting `onAction`:

```tsx
async function submitFriendCommand(action: string, friend: FriendRow) {
  if (action === 'view_qq') {
    if (qqProfilePendingGID === friend.gid) return;
    setQQProfilePendingGID(friend.gid);
    try {
      await onAction({ action, target: String(friend.gid) });
    } finally {
      setQQProfilePendingGID(null);
      setFriendCommandMenu(null);
    }
    return;
  }
  setFriendCommandMenu(null);
  await onAction({ action, target: String(friend.gid) });
}
```

At the beginning of the existing action-menu item list, conditionally add the
following item. Reuse the existing `Eye` icon; do not add a new icon package.

```tsx
...(isQQRuntime ? [{
  label: qqProfilePendingGID === friend.gid ? '打开QQ中' : '查看好友QQ',
  icon: <Eye size={16} />,
  disabled: qqProfilePendingGID === friend.gid,
  reason: null,
  onSelect: () => submitFriendCommand('view_qq', friend),
}] : []),
```

Keep the existing `查看` entry and all other action/config items unchanged.

- [ ] **Step 4: Run the focused frontend tests and observe GREEN**

Run:

```powershell
Set-Location frontend
npm test -- src/views/FarmWorkspaceView.test.tsx src/views/SocialView.test.tsx
```

Expected: PASS.

- [ ] **Step 5: Run the frontend production build**

Run:

```powershell
Set-Location frontend
npm run build
```

Expected: `tsc && vite build` exits `0`.

- [ ] **Step 6: Commit the frontend slice**

```powershell
git add -- frontend/src/views/FarmWorkspaceView.tsx frontend/src/views/FarmWorkspaceView.test.tsx frontend/src/views/SocialView.tsx frontend/src/views/SocialView.test.tsx
git commit -m "feat: show QQ friend profile action in social view"
```

### Task 4: Run Cross-Layer Regression Verification

**Files:**
- Verify: `internal/farm/social/actions.go`
- Verify: `app.go`
- Verify: `frontend/src/views/FarmWorkspaceView.tsx`
- Verify: `frontend/src/views/SocialView.tsx`

- [ ] **Step 1: Run the embedded QQ protocol regression**

Run:

```powershell
node scripts/test-friend-pure-protocol.js
```

Expected: `[friend-pure-protocol] byte builders pass`.

- [ ] **Step 2: Run the full Go suite**

Run:

```powershell
go test ./... -count=1
```

Expected: every package exits `0`.

- [ ] **Step 3: Run all frontend tests**

Run:

```powershell
Set-Location frontend
npm test
```

Expected: Vitest exits `0` with no failed tests.

- [ ] **Step 4: Validate the working tree**

Run:

```powershell
Set-Location ..
git diff --check
git status --short
```

Expected: no whitespace errors; after the three implementation-slice commits,
only this implementation-plan document remains uncommitted.
