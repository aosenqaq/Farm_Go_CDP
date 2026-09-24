# 安全隧道 HTTP 会话提示 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent an HTTP-accessed secure tunnel from submitting its access password and explain that the FRP endpoint must use HTTPS.

**Architecture:** The Go LAN server adds a tunnel-only flag to its existing external remote bootstrap script. The React remote password gate reads that flag with the browser protocol, displays a transport-specific message, and returns before issuing a login request for insecure tunnel pages. The existing tunnel `Secure` cookie policy remains unchanged.

**Tech Stack:** Go `net/http` and `httptest`; React 18; TypeScript; Vitest; react-test-renderer.

---

### Task 1: Publish Tunnel State in the Remote Bootstrap

**Files:**
- Modify: `internal/lanaccess/server.go:22-23,90-96`
- Test: `internal/lanaccess/server_test.go:206-231`

- [ ] **Step 1: Write the failing bootstrap-mode test**

Replace `TestServerServesCSPCompatibleRemoteBootstrap` with this table-driven assertion so both modes have an explicit expected script:

```go
func TestServerServesCSPCompatibleRemoteBootstrap(t *testing.T) {
	for _, tc := range []struct {
		mode Mode
		want string
	}{
		{mode: ModeLAN, want: "window.__FARM_GO_REMOTE__=true;\n"},
		{mode: ModeTunnel, want: "window.__FARM_GO_REMOTE__=true;\nwindow.__FARM_GO_TUNNEL__=true;\n"},
	} {
		t.Run(string(tc.mode), func(t *testing.T) {
			server := newTestServer(t, tc.mode, nil, nil)
			get := httptest.NewRecorder()
			server.ServeHTTP(get, newServerRequest(http.MethodGet, "/farm-go-remote.js", "127.0.0.1:3210", nil))

			if get.Code != http.StatusOK {
				t.Fatalf("GET status=%d", get.Code)
			}
			if got := get.Header().Get("Content-Type"); got != "application/javascript; charset=utf-8" {
				t.Fatalf("Content-Type=%q", got)
			}
			if got := get.Body.String(); got != tc.want {
				t.Fatalf("body=%q, want %q", got, tc.want)
			}
		})
	}

	server := newTestServer(t, ModeLAN, nil, nil)
	post := httptest.NewRecorder()
	server.ServeHTTP(post, newServerRequest(http.MethodPost, "/farm-go-remote.js", "127.0.0.1:3210", nil))
	if post.Code != http.StatusMethodNotAllowed || post.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("POST status=%d allow=%q", post.Code, post.Header().Get("Allow"))
	}
}
```

- [ ] **Step 2: Run the focused test and confirm it fails for tunnel mode**

Run: `go test ./internal/lanaccess -run TestServerServesCSPCompatibleRemoteBootstrap -count=1`

Expected: FAIL because tunnel mode currently returns only `window.__FARM_GO_REMOTE__=true;`.

- [ ] **Step 3: Add the minimal tunnel-only bootstrap flag**

Add a helper next to `serveStatic` and use it in the bootstrap route:

```go
func (s *server) remoteBootstrapScript() string {
	script := remoteBootstrapScript
	if s.config.Mode == ModeTunnel {
		script += "window.__FARM_GO_TUNNEL__=true;\n"
	}
	return script
}
```

```go
w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
_, _ = io.WriteString(w, s.remoteBootstrapScript())
```

Do not alter `remoteBootstrapScript`, the cookie attributes, proxy-header policy, or source-address filtering.

- [ ] **Step 4: Run the focused Go test and confirm it passes**

Run: `go test ./internal/lanaccess -run TestServerServesCSPCompatibleRemoteBootstrap -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the bootstrap change**

```powershell
git add -- internal/lanaccess/server.go internal/lanaccess/server_test.go
git commit -m "fix: identify secure tunnel browser sessions"
```

### Task 2: Block Password Submission from HTTP Tunnel Pages

**Files:**
- Modify: `frontend/src/lib/remoteBridge.ts:10-16`
- Modify: `frontend/src/RemoteApp.tsx:9-70`
- Test: `frontend/src/RemoteApp.test.tsx:35-152`

- [ ] **Step 1: Write failing React tests for HTTP rejection and supported protocols**

Add the following test after the initial password-form test:

```tsx
it('does not send an access password from an HTTP secure tunnel page', async () => {
  vi.stubGlobal('window', {
    __FARM_GO_TUNNEL__: true,
    location: { protocol: 'http:' },
  });
  vi.mocked(syncRemoteSession).mockResolvedValue({ authorized: false, csrfToken: '' });
  const fetchImpl = vi.fn();
  vi.stubGlobal('fetch', fetchImpl);
  let view!: ReturnType<typeof create>;

  await act(async () => {
    view = create(<RemoteApp />);
    await flushEffects();
  });
  act(() => {
    view.root.findByProps({ id: 'remote-password' }).props.onChange({ target: { value: 'correct horse battery staple' } });
  });
  await act(async () => {
    view.root.findByType('form').props.onSubmit({ preventDefault() {} });
    await flushEffects();
  });

  expect(fetchImpl).not.toHaveBeenCalled();
  expect(JSON.stringify(view.toJSON())).toContain('安全隧道必须通过 HTTPS FRP 入口访问');
  view.unmount();
});
```

Before the existing successful-login submission test, install an HTTPS tunnel browser global:

```tsx
vi.stubGlobal('window', {
  __FARM_GO_TUNNEL__: true,
  location: { protocol: 'https:' },
});
```

Add this complete LAN HTTP regression test after the HTTPS tunnel test:

```tsx
it('continues to post the access password from a LAN HTTP page', async () => {
  vi.stubGlobal('window', { location: { protocol: 'http:' } });
  vi.mocked(syncRemoteSession)
    .mockResolvedValueOnce({ authorized: false, csrfToken: '' })
    .mockResolvedValueOnce({ authorized: true, csrfToken: 'refreshed-csrf' });
  const fetchImpl = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
  vi.stubGlobal('fetch', fetchImpl);
  let view!: ReturnType<typeof create>;

  await act(async () => {
    view = create(<RemoteApp />);
    await flushEffects();
  });
  act(() => {
    view.root.findByProps({ id: 'remote-password' }).props.onChange({ target: { value: 'correct horse battery staple' } });
  });
  await act(async () => {
    view.root.findByType('form').props.onSubmit({ preventDefault() {} });
    await flushEffects();
  });

  expect(fetchImpl).toHaveBeenCalledWith('/api/auth/login', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password: 'correct horse battery staple' }),
  });
  expect(JSON.stringify(view.toJSON())).toContain('authorized remote console');
  view.unmount();
});
```

This proves only tunnel HTTP is blocked.

- [ ] **Step 2: Run the RemoteApp test and confirm the new HTTP tunnel test fails**

Run: `npm test -- src/RemoteApp.test.tsx`

Expected: FAIL because the current component posts the password without inspecting the tunnel flag or browser protocol.

- [ ] **Step 3: Add the tunnel browser type and minimal submission guard**

Extend `RemoteBridgeWindow`:

```ts
export type RemoteBridgeWindow = {
  __FARM_GO_REMOTE__?: boolean;
  __FARM_GO_TUNNEL__?: boolean;
  go?: {
```

In `RemoteApp.tsx`, import `RemoteBridgeWindow`, define the message and helper before `RemoteApp`, and guard the submission:

```tsx
const tunnelHTTPSMessage = '安全隧道必须通过 HTTPS FRP 入口访问';

function insecureTunnelPage() {
  if (typeof window === 'undefined') return false;
  const remoteWindow = window as RemoteBridgeWindow;
  return remoteWindow.__FARM_GO_TUNNEL__ === true && window.location.protocol !== 'https:';
}
```

```tsx
  const requiresTunnelHTTPS = insecureTunnelPage();
```

At the start of `submit`, before `setSubmitting(true)`, add:

```tsx
    if (requiresTunnelHTTPS) {
      setAuthorized(false);
      setError(tunnelHTTPSMessage);
      return;
    }
```

Pass the transport message to the gate when it is known:

```tsx
      error={requiresTunnelHTTPS ? tunnelHTTPSMessage : error}
```

The post-login session recheck, generic password-failure message, and fetch request options must remain unchanged.

- [ ] **Step 4: Run the focused React tests and confirm they pass**

Run: `npm test -- src/RemoteApp.test.tsx`

Expected: PASS, including the HTTP tunnel no-fetch assertion, HTTPS tunnel login path, and LAN HTTP login path.

- [ ] **Step 5: Commit the remote password guard**

```powershell
git add -- frontend/src/lib/remoteBridge.ts frontend/src/RemoteApp.tsx frontend/src/RemoteApp.test.tsx
git commit -m "fix: explain HTTPS requirement for secure tunnels"
```

### Task 3: Clarify the Desktop Mode Label

**Files:**
- Modify: `frontend/src/components/LANAccessSettingsPanel.tsx:171-174`
- Test: `frontend/src/components/LANAccessSettingsPanel.test.tsx:51-65`

- [ ] **Step 1: Write the failing label assertion**

In `uses the LAN access and secure tunnel labels required by the desktop settings`, add:

```tsx
expect(markup).toContain('安全隧道（仅 HTTPS）');
expect(markup).not.toContain('安全隧道（127.0.0.1）');
```

- [ ] **Step 2: Run the panel test and confirm it fails**

Run: `npm test -- src/components/LANAccessSettingsPanel.test.tsx`

Expected: FAIL because the option currently describes the local `127.0.0.1` FRP target instead of its HTTPS-only access requirement.

- [ ] **Step 3: Replace only the tunnel option text**

```tsx
<option value="tunnel">安全隧道（仅 HTTPS）</option>
```

Keep the `FRP 目标` label and `127.0.0.1:<port>` target calculation unchanged.

- [ ] **Step 4: Run the focused panel test and confirm it passes**

Run: `npm test -- src/components/LANAccessSettingsPanel.test.tsx`

Expected: PASS.

- [ ] **Step 5: Commit the settings clarification**

```powershell
git add -- frontend/src/components/LANAccessSettingsPanel.tsx frontend/src/components/LANAccessSettingsPanel.test.tsx
git commit -m "fix: clarify HTTPS tunnel configuration"
```

### Task 4: Verify the Integrated Change

**Files:**
- Verify only: `internal/lanaccess/server_test.go`
- Verify only: `frontend/src/RemoteApp.test.tsx`
- Verify only: `frontend/src/components/LANAccessSettingsPanel.test.tsx`

- [ ] **Step 1: Run the relevant Go package tests**

Run: `go test ./internal/lanaccess -count=1`

Expected: PASS with the secure-cookie test still confirming tunnel cookies remain `Secure`.

- [ ] **Step 2: Run all frontend tests**

Run: `npm test`

Expected: PASS with no failed tests.

- [ ] **Step 3: Build the frontend**

Run: `npm run build`

Expected: PASS; TypeScript compilation and Vite production build both complete successfully.

- [ ] **Step 4: Run the full Go suite and inspect the final diff**

Run: `go test ./... -count=1; git diff --check; git status --short`

Expected: all Go tests PASS, no whitespace errors, and only the planned source/test changes are present before the final commit.
