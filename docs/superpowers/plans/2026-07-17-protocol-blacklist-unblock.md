# Protocol Blacklist Unblock Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add row-level and checked batch unblock controls to the protocol blacklist dialog.

**Architecture:** Reuse the existing `gameCtl.unblockFriendByProtocol` runtime method. Add serial batch orchestration to the Go social service with per-gid results, then keep selection, pending state, and list refresh behavior local to `SocialView`.

**Tech Stack:** Go 1.25, Wails v2, React 18, TypeScript, Vitest, react-test-renderer, lucide-react, Vite.

---

## File Map

- Modify `internal/farm/social/service_test.go`: let the shared fake runtime return a sequence of values for repeated calls to one method.
- Modify `internal/farm/social/actions_test.go`: prove normalized serial batch unblock behavior and empty-selection rejection.
- Modify `internal/farm/social/actions.go`: route and execute `unblock_friend_batch` with partial-failure reporting.
- Modify `frontend/src/views/SocialView.test.tsx`: prove single unblock, selection controls, pending state, batch payload, and refresh behavior.
- Modify `frontend/src/views/SocialView.tsx`: render and coordinate the protocol blacklist controls.
- Modify `frontend/src/style.css`: add stable desktop/mobile layout for protocol blacklist rows and toolbar.
- Generate `frontend/dist/*`: rebuild the Wails-embedded frontend; generated output is ignored by Git but must be verified.

### Task 1: Go Protocol Batch Unblock

**Files:**
- Modify: `internal/farm/social/service_test.go:11-29`
- Modify: `internal/farm/social/actions_test.go:20-48`
- Modify: `internal/farm/social/actions.go:12-80`
- Modify: `internal/farm/social/actions.go:269-279`

- [ ] **Step 1: Write the failing batch-action tests**

Extend `fakeRuntimeCaller` in `internal/farm/social/service_test.go` so a test can provide different outcomes for repeated calls:

```go
type fakeRuntimeCaller struct {
	responses     map[string]any
	responseQueue map[string][]any
	calls         []runtimeCall
}

func (f *fakeRuntimeCaller) Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error) {
	f.calls = append(f.calls, runtimeCall{method: method, args: args, timeout: timeout})
	value := f.responses[method]
	if queued := f.responseQueue[method]; len(queued) > 0 {
		value = queued[0]
		f.responseQueue[method] = queued[1:]
	}
	if err, ok := value.(error); ok {
		return nil, err
	}
	return value, nil
}
```

Add these tests near `TestFriendProtocolBlockListCallsRuntime` in `internal/farm/social/actions_test.go`:

```go
func TestFriendActionUnblocksSelectedProtocolFriends(t *testing.T) {
	caller := &fakeRuntimeCaller{responseQueue: map[string][]any{
		"gameCtl.unblockFriendByProtocol": {
			map[string]any{"ok": true},
			errors.New("denied"),
			map[string]any{"ok": true},
		},
	}}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{
		Action:  "unblock_friend_batch",
		Targets: []string{" 10002 ", "bad", "10001", "10002", "0", "10003"},
	})

	if result.OK || result.Status != StatusFailed || !strings.Contains(result.Message, "成功 2 个，失败 1 个") {
		t.Fatalf("result = %#v", result)
	}
	if len(caller.calls) != 3 {
		t.Fatalf("calls = %#v", caller.calls)
	}
	wantGIDs := []int{10002, 10001, 10003}
	for index, gid := range wantGIDs {
		if caller.calls[index].method != "gameCtl.unblockFriendByProtocol" {
			t.Fatalf("call %d method = %q", index, caller.calls[index].method)
		}
		wantArgs := []any{map[string]any{"friendGid": gid, "dryRun": false, "silent": true}}
		if !reflect.DeepEqual(caller.calls[index].args, wantArgs) {
			t.Fatalf("call %d args = %#v, want %#v", index, caller.calls[index].args, wantArgs)
		}
	}
	data := result.Data.(map[string]any)
	if data["successCount"] != 2 || data["failureCount"] != 1 || !reflect.DeepEqual(data["targets"], wantGIDs) {
		t.Fatalf("data = %#v", data)
	}
	results := data["results"].([]map[string]any)
	if len(results) != 3 || results[1]["gid"] != 10001 || results[1]["ok"] != false {
		t.Fatalf("results = %#v", results)
	}
}

func TestFriendActionRejectsEmptyProtocolUnblockBatch(t *testing.T) {
	caller := &fakeRuntimeCaller{}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{
		Action:  "unblock_friend_batch",
		Targets: []string{"", "bad", "0", "-1"},
	})

	if result.OK || result.Status != StatusFailed || len(caller.calls) != 0 {
		t.Fatalf("result = %#v calls = %#v", result, caller.calls)
	}
}
```

Add `"errors"` to the `internal/farm/social/actions_test.go` import block.

- [ ] **Step 2: Run the focused Go tests and verify RED**

Run:

```powershell
go test ./internal/farm/social -run 'TestFriendAction(UnblocksSelectedProtocolFriends|RejectsEmptyProtocolUnblockBatch)' -count=1
```

Expected: FAIL because `unblock_friend_batch` currently falls through to the single-target gid validation.

- [ ] **Step 3: Implement serial batch orchestration**

In the first action switch in `internal/farm/social/actions.go`, route the new action before single-target validation:

```go
	case "unblock_batch", "unblock_friend_batch":
		return s.unblockFriendsByProtocol(ctx, action, req.Targets, req.DryRun)
```

Add this helper beside `callRuntime`:

```go
func (s *Service) unblockFriendsByProtocol(ctx context.Context, action string, targets []string, dryRun bool) ActionResult {
	gids := make([]int, 0, len(targets))
	seen := map[int]bool{}
	for _, target := range NormalizeStringList(targets) {
		gid := PositiveInt(target)
		if gid <= 0 || seen[gid] {
			continue
		}
		seen[gid] = true
		gids = append(gids, gid)
	}
	if len(gids) == 0 {
		return ActionResult{OK: false, Status: StatusFailed, Message: "请选择要解除协议拉黑的好友。"}
	}
	if s.caller == nil {
		return ActionResult{OK: false, Status: StatusRuntimeNotReady, Message: "游戏运行时尚未连接，无法解除协议拉黑。"}
	}

	results := make([]map[string]any, 0, len(gids))
	successCount := 0
	failureCount := 0
	for _, gid := range gids {
		result := s.callRuntime(ctx, "gameCtl.unblockFriendByProtocol", []any{map[string]any{
			"friendGid": gid,
			"dryRun":    dryRun,
			"silent":    true,
		}}, 30*time.Second, "已提交协议解除拉黑请求。", "协议解除拉黑失败：")
		entry := map[string]any{"gid": gid, "ok": result.OK, "status": result.Status, "message": result.Message}
		if result.Data != nil {
			entry["data"] = result.Data
		}
		results = append(results, entry)
		if result.OK {
			successCount++
		} else {
			failureCount++
		}
	}

	ok := failureCount == 0
	status := StatusOK
	if !ok {
		status = StatusFailed
	}
	return ActionResult{
		OK:      ok,
		Status:  status,
		Message: fmt.Sprintf("协议批量解除完成：成功 %d 个，失败 %d 个。", successCount, failureCount),
		Data: map[string]any{
			"action":       action,
			"targets":      gids,
			"results":      results,
			"successCount": successCount,
			"failureCount": failureCount,
		},
	}
}
```

- [ ] **Step 4: Format and verify GREEN**

Run:

```powershell
gofmt -w internal/farm/social/actions.go internal/farm/social/actions_test.go internal/farm/social/service_test.go
go test ./internal/farm/social -run 'TestFriendAction(UnblocksSelectedProtocolFriends|RejectsEmptyProtocolUnblockBatch)' -count=1
go test ./internal/farm/social -count=1
```

Expected: both commands PASS; the package suite reports zero failures.

- [ ] **Step 5: Commit the backend behavior**

```powershell
git add -- internal/farm/social/actions.go internal/farm/social/actions_test.go internal/farm/social/service_test.go
git commit -m "feat: add protocol blacklist batch unblock action"
```

### Task 2: Protocol Blacklist Dialog Controls

**Files:**
- Modify: `frontend/src/views/SocialView.test.tsx:1-140`
- Modify: `frontend/src/views/SocialView.test.tsx:990-1225`
- Modify: `frontend/src/views/SocialView.tsx:1-18`
- Modify: `frontend/src/views/SocialView.tsx:244-278`
- Modify: `frontend/src/views/SocialView.tsx:476-540`
- Modify: `frontend/src/views/SocialView.tsx:1027-1040`
- Modify: `frontend/src/style.css:2764-2880`
- Modify: `frontend/src/style.css:2948-2975`

- [ ] **Step 1: Write failing dialog interaction tests**

Add `initialProtocolOpen?: boolean` to the wished-for `SocialView` test API, then append these tests to `frontend/src/views/SocialView.test.tsx`:

```tsx
  it('submits one protocol unblock and refreshes the runtime list', async () => {
    const requests: Record<string, unknown>[] = [];
    const refresh = vi.fn();
    const renderer = TestRenderer.create(
      <SocialView
        state={state}
        protocolBlockList={state.friends}
        onRefresh={() => undefined}
        onAction={(request) => {
          requests.push(request);
          return { ok: true, status: 'ok', message: '已解除' };
        }}
        onProtocolBlockList={refresh}
        initialProtocolOpen
      />,
    );

    await act(async () => {
      await findButton(renderer.root, '解除拉黑').props.onClick();
    });

    expect(requests).toEqual([{ action: 'unblock_friend', target: '10001' }]);
    expect(refresh).toHaveBeenCalledTimes(1);
  });

  it('selects protocol friends and submits a batch unblock payload', async () => {
    const requests: Record<string, unknown>[] = [];
    const refresh = vi.fn();
    const renderer = TestRenderer.create(
      <SocialView
        state={state}
        protocolBlockList={state.friends}
        onRefresh={() => undefined}
        onAction={(request) => {
          requests.push(request);
          return { ok: true, status: 'ok', message: '批量解除完成' };
        }}
        onProtocolBlockList={refresh}
        initialProtocolOpen
      />,
    );

    act(() => findButton(renderer.root, '全选当前列表').props.onClick());
    expect(renderer.root.findAllByType('input').filter((input) => input.props.type === 'checkbox' && input.props.checked)).toHaveLength(2);

    await act(async () => {
      await findButton(renderer.root, '解除选中').props.onClick();
    });

    expect(requests).toEqual([{ action: 'unblock_friend_batch', targets: ['10001', '10002'] }]);
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(findButton(renderer.root, '解除选中').props.disabled).toBe(true);
  });

  it('clears selected protocol friends without submitting', () => {
    const renderer = TestRenderer.create(
      <SocialView
        state={state}
        protocolBlockList={state.friends}
        onRefresh={() => undefined}
        onAction={() => undefined}
        initialProtocolOpen
      />,
    );

    act(() => findButton(renderer.root, '全选当前列表').props.onClick());
    act(() => findButton(renderer.root, '清空选择').props.onClick());

    expect(renderer.root.findAllByType('input').filter((input) => input.props.type === 'checkbox' && input.props.checked)).toHaveLength(0);
    expect(findButton(renderer.root, '解除选中').props.disabled).toBe(true);
  });

  it('keeps protocol unblock controls disabled while a request is pending and refreshes after failure', async () => {
    const pending = deferred<SocialActionResult>();
    const refresh = vi.fn();
    const renderer = TestRenderer.create(
      <SocialView
        state={state}
        protocolBlockList={state.friends}
        onRefresh={() => undefined}
        onAction={() => pending.promise}
        onProtocolBlockList={refresh}
        initialProtocolOpen
      />,
    );

    let request!: Promise<void>;
    await act(async () => {
      request = findButton(renderer.root, '解除拉黑').props.onClick();
      await Promise.resolve();
    });
    expect(findButton(renderer.root, '解除中').props.disabled).toBe(true);
    expect(findButton(renderer.root, '解除选中').props.disabled).toBe(true);

    await act(async () => {
      pending.resolve({ ok: false, status: 'failed', message: '解除失败' });
      await request;
    });
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(findButton(renderer.root, '解除拉黑').props.disabled).toBe(false);
  });
```

- [ ] **Step 2: Run the focused Vitest file and verify RED**

Run from `frontend`:

```powershell
npm test -- src/views/SocialView.test.tsx
```

Expected: FAIL because `initialProtocolOpen`, selection controls, and unblock handlers do not exist.

- [ ] **Step 3: Add dialog state and unblock handlers**

In `frontend/src/views/SocialView.tsx`:

1. Import `Unlock` from `lucide-react`.
2. Add `initialProtocolOpen?: boolean` to `SocialViewProps`, default it to `false`, and initialize `protocolOpen` from it.
3. Add these states and handlers inside `SocialView`:

```tsx
  const [selectedProtocolGIDs, setSelectedProtocolGIDs] = useState<string[]>([]);
  const [protocolUnblockPending, setProtocolUnblockPending] = useState<string | null>(null);
  const protocolListKey = protocolBlockList.map((friend) => String(friend.gid)).join('|');

  useEffect(() => {
    const current = new Set(protocolBlockList.map((friend) => String(friend.gid)));
    setSelectedProtocolGIDs((selected) => selected.filter((gid) => current.has(gid)));
  }, [protocolListKey]);

  function toggleProtocolSelection(gid: string) {
    setSelectedProtocolGIDs((selected) =>
      selected.includes(gid) ? selected.filter((item) => item !== gid) : [...selected, gid],
    );
  }

  async function refreshProtocolBlockList() {
    await onProtocolBlockList?.();
  }

  async function submitProtocolUnblock(gid: string) {
    setProtocolUnblockPending(gid);
    try {
      await onAction({ action: 'unblock_friend', target: gid });
    } finally {
      try {
        await refreshProtocolBlockList();
      } finally {
        setProtocolUnblockPending(null);
      }
    }
  }

  async function submitProtocolUnblockBatch() {
    if (selectedProtocolGIDs.length === 0) return;
    const targets = [...selectedProtocolGIDs];
    setProtocolUnblockPending('__batch__');
    try {
      await onAction({ action: 'unblock_friend_batch', targets });
    } finally {
      try {
        setSelectedProtocolGIDs([]);
        await refreshProtocolBlockList();
      } finally {
        setProtocolUnblockPending(null);
      }
    }
  }
```

- [ ] **Step 4: Replace the protocol dialog markup**

Replace the current `protocolOpen` block with:

```tsx
      {protocolOpen && (
        <Dialog title="协议拉黑好友" onClose={() => setProtocolOpen(false)}>
          <div className="social-dialog-toolbar social-protocol-toolbar">
            <span className="social-muted">已选择 {selectedProtocolGIDs.length} / {protocolBlockList.length}</span>
            <div className="social-protocol-toolbar-actions">
              <button
                className="secondary-button social-action-button compact"
                type="button"
                disabled={protocolUnblockPending !== null || protocolBlockList.length === 0}
                onClick={() => setSelectedProtocolGIDs(protocolBlockList.map((friend) => String(friend.gid)))}
              >
                全选当前列表
              </button>
              <button
                className="secondary-button social-action-button compact"
                type="button"
                disabled={protocolUnblockPending !== null || selectedProtocolGIDs.length === 0}
                onClick={() => setSelectedProtocolGIDs([])}
              >
                清空选择
              </button>
              <button
                className="primary-button social-action-button compact"
                type="button"
                disabled={protocolUnblockPending !== null || selectedProtocolGIDs.length === 0}
                onClick={submitProtocolUnblockBatch}
              >
                <Unlock size={14} />
                {protocolUnblockPending === '__batch__' ? '解除中' : '解除选中'}
              </button>
            </div>
          </div>
          <div className="social-mini-table">
            {protocolBlockList.map((friend) => {
              const gid = String(friend.gid);
              const rowPending = protocolUnblockPending === gid;
              return (
                <div className="social-mini-row social-protocol-row" key={friend.gid}>
                  <label className="social-protocol-select">
                    <input
                      aria-label={`选择 ${friend.displayName || friend.name || gid}`}
                      type="checkbox"
                      checked={selectedProtocolGIDs.includes(gid)}
                      disabled={protocolUnblockPending !== null}
                      onChange={() => toggleProtocolSelection(gid)}
                    />
                  </label>
                  <strong>{friend.displayName || friend.name || friend.gid}</strong>
                  <span>{friend.gid}</span>
                  <em>协议拉黑</em>
                  <button
                    className="secondary-button social-action-button compact"
                    type="button"
                    disabled={protocolUnblockPending !== null}
                    onClick={() => submitProtocolUnblock(gid)}
                  >
                    <Unlock size={14} />
                    {rowPending ? '解除中' : '解除拉黑'}
                  </button>
                </div>
              );
            })}
            {protocolBlockList.length === 0 && <div className="social-empty">暂无协议拉黑好友</div>}
          </div>
        </Dialog>
      )}
```

- [ ] **Step 5: Add stable responsive styles**

Add beside the existing social dialog and mini-row rules in `frontend/src/style.css`:

```css
.social-protocol-toolbar {
  align-items: center;
}

.social-protocol-toolbar-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.social-protocol-row {
  grid-template-columns: 24px minmax(0, 1fr) 92px 76px auto;
  min-height: 48px;
  padding-block: 6px;
}

.social-protocol-select {
  display: grid;
  width: 24px;
  height: 32px;
  place-items: center;
}

.social-protocol-select input {
  width: 16px;
  height: 16px;
  margin: 0;
  accent-color: #20271f;
}

.social-protocol-row .social-action-button {
  justify-self: end;
  white-space: nowrap;
}

@media (max-width: 760px) {
  .social-protocol-toolbar {
    align-items: stretch;
  }

  .social-protocol-toolbar-actions {
    width: 100%;
  }

  .social-protocol-toolbar-actions .social-action-button {
    flex: 1 1 auto;
  }

  .social-protocol-row {
    grid-template-columns: 24px minmax(0, 1fr) auto;
  }

  .social-protocol-row span,
  .social-protocol-row em {
    grid-column: 2;
    text-align: left;
  }

  .social-protocol-row .social-action-button {
    grid-column: 3;
    grid-row: 1 / span 3;
    align-self: center;
  }
}
```

- [ ] **Step 6: Run focused tests and verify GREEN**

Run from `frontend`:

```powershell
npm test -- src/views/SocialView.test.tsx
```

Expected: the entire `SocialView.test.tsx` file PASS with zero failed tests.

- [ ] **Step 7: Commit the frontend behavior**

```powershell
git add -- frontend/src/views/SocialView.tsx frontend/src/views/SocialView.test.tsx frontend/src/style.css
git commit -m "feat: add protocol blacklist unblock controls"
```

### Task 3: Full Verification and Frontend Build

**Files:**
- Verify: `internal/farm/social/actions.go`
- Verify: `internal/farm/social/actions_test.go`
- Verify: `internal/farm/social/service_test.go`
- Verify: `frontend/src/views/SocialView.tsx`
- Verify: `frontend/src/views/SocialView.test.tsx`
- Verify: `frontend/src/style.css`
- Generate and verify: `frontend/dist/index.html`
- Generate and verify: `frontend/dist/assets/*`

- [ ] **Step 1: Run all Go tests**

```powershell
go test ./... -count=1
```

Expected: all Go packages PASS with zero failures.

- [ ] **Step 2: Run all frontend tests**

Run from `frontend`:

```powershell
npm test
```

Expected: all Vitest files PASS with zero failures.

- [ ] **Step 3: Record the old embedded asset and build the frontend**

Run from the repository root:

```powershell
$oldIndex = Get-Content -Raw 'frontend\dist\index.html'
$oldAsset = [regex]::Match($oldIndex, 'assets/index-[^"'']+\.js').Value
Push-Location frontend
try { npm run build } finally { Pop-Location }
$newIndex = Get-Content -Raw 'frontend\dist\index.html'
$newAsset = [regex]::Match($newIndex, 'assets/index-[^"'']+\.js').Value
Write-Output "old=$oldAsset"
Write-Output "new=$newAsset"
Get-Item (Join-Path 'frontend\dist' $newAsset) | Select-Object FullName, Length, LastWriteTime
```

Expected: TypeScript and Vite exit `0`; `$newAsset` is non-empty, exists, and has the current build timestamp. The asset name normally changes because the source changed.

- [ ] **Step 4: Check formatting, scope, and repository state**

```powershell
git diff --check
git status --short
git log -5 --oneline
```

Expected: no whitespace errors; only intentional implementation commits are present; `frontend/dist` may remain absent from status because it is generated and ignored.

- [ ] **Step 5: Start the local frontend server for user validation**

From `frontend`, start Vite on an unused port:

```powershell
npm run dev -- --host 127.0.0.1 --port 5173
```

Expected: Vite prints a local URL. If `5173` is occupied, retry with `5174`. Keep the server running and report the URL, noting that the Wails-backed authorized social data requires the desktop runtime.

