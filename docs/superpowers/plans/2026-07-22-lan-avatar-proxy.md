# LAN WebUI Avatar Proxy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render real account avatar images in the LAN WebUI while retaining the existing same-origin CSP and session boundaries.

**Architecture:** A new avatarProxy owns session-bound opaque avatar tokens and fetches registered HTTPS images only after validating their destination and response. server.go rewrites avatarUrl fields in authenticated RPC JSON responses and serves token-backed images at /farm-avatars/<token>; React continues using its existing FallbackImage components unchanged.

**Tech Stack:** Go 1.25, net/http, net/netip, net/url, mime, httptest, existing LAN session authentication and Go testing.

---

## File Structure

- Create: internal/lanaccess/avatar_proxy.go - session-bound token registry, JSON rewrite, source validation, and bounded image fetch.
- Create: internal/lanaccess/avatar_proxy_test.go - unit coverage for recursive rewrites, token ownership, image type, and size checks.
- Modify: internal/lanaccess/server.go - initialize the proxy, rewrite authenticated RPC results, and serve /farm-avatars/<token>.
- Modify: internal/lanaccess/server_test.go - cover the HTTP route and its session boundary.

### Task 1: Add The Session-Bound Avatar Proxy

**Files:**
- Create: internal/lanaccess/avatar_proxy.go
- Create: internal/lanaccess/avatar_proxy_test.go

- [x] **Step 1: Write the failing proxy tests**

Create tests with the following cases:

~~~go
func TestAvatarProxyRewritesNestedAvatarURLsAndBindsTokensToSession(t *testing.T) {
	proxy := newAvatarProxy(systemClock{})
	owner := Session{ID: "owner", ExpiresAt: time.Now().Add(time.Hour)}
	value := map[string]any{
		"account": map[string]any{"avatarUrl": "https://avatar.example/self.png"},
		"social": map[string]any{"friends": []any{map[string]any{"avatarUrl": "https://avatar.example/friend.png"}}},
	}
	rewritten := proxy.rewriteJSON(owner, value).(map[string]any)
	accountURL := rewritten["account"].(map[string]any)["avatarUrl"].(string)
	friendURL := rewritten["social"].(map[string]any)["friends"].([]any)[0].(map[string]any)["avatarUrl"].(string)
	for _, got := range []string{accountURL, friendURL} {
		if !strings.HasPrefix(got, avatarProxyPathPrefix) || strings.Contains(got, "avatar.example") {
			t.Fatalf("rewritten avatar URL = %q", got)
		}
	}
	if _, err := proxy.fetch(context.Background(), Session{ID: "other", ExpiresAt: owner.ExpiresAt}, strings.TrimPrefix(accountURL, avatarProxyPathPrefix)); !errors.Is(err, errAvatarUnauthorized) {
		t.Fatalf("other session error = %v", err)
	}
}

func TestAvatarProxyFetchAcceptsOnlyBoundedImageResponses(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/image":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png"))
		case "/text":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("not an image"))
		default:
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(make([]byte, maxAvatarBytes+1))
		}
	}))
	defer upstream.Close()

	proxy := newAvatarProxy(systemClock{})
	proxy.client = upstream.Client()
	proxy.lookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	owner := Session{ID: "owner", ExpiresAt: time.Now().Add(time.Hour)}
	for _, tc := range []struct{ path string; want error }{
		{"/image", nil},
		{"/text", errAvatarInvalidResponse},
		{"/large", errAvatarInvalidResponse},
	} {
		t.Run(tc.path, func(t *testing.T) {
			path := proxy.register(owner, upstream.URL+tc.path)
			_, err := proxy.fetch(context.Background(), owner, strings.TrimPrefix(path, avatarProxyPathPrefix))
			if !errors.Is(err, tc.want) {
				t.Fatalf("fetch error = %v, want %v", err, tc.want)
			}
		})
	}
}
~~~

Imports: context, errors, net/http, net/http/httptest, net/netip, strings, testing, and time.

- [x] **Step 2: Run the tests to verify RED**

Run: go test ./internal/lanaccess -run TestAvatarProxy -count=1

Expected: FAIL because the proxy types, methods, and constants do not exist.

- [x] **Step 3: Implement the proxy**

Create avatar_proxy.go with these package-private contracts:

~~~go
const (
	avatarProxyPathPrefix = "/farm-avatars/"
	maxAvatarBytes        = 2 << 20
)

var (
	errAvatarNotFound        = errors.New("avatar token not found")
	errAvatarUnauthorized    = errors.New("avatar token is not owned by this session")
	errAvatarInvalidResponse = errors.New("avatar upstream response is invalid")
)

type avatarProxy struct {
	mu          sync.Mutex
	entries     map[string]avatarEntry
	now         func() time.Time
	client      *http.Client
	lookupNetIP func(context.Context, string, string) ([]netip.Addr, error)
}

type avatarEntry struct {
	sourceURL string
	sessionID string
	expiresAt time.Time
}

type avatarImage struct {
	contentType string
	bytes       []byte
}
~~~

Implement newAvatarProxy, rewriteJSON, register, fetch, pruneExpiredLocked, rewriteAvatarURLs, parseAvatarSource, validatePublicHTTPSURL, and isPublicAvatarAddress with these rules:

- rewriteJSON marshal/unmarshals the result and recursively changes only exact avatarUrl keys. Valid HTTPS values become avatarProxyPathPrefix plus randomToken; invalid values become an empty string, ensuring no original external source reaches the browser.
- register stores source URL, owner Session.ID, and Session.ExpiresAt; it removes expired entries before adding an entry.
- fetch verifies owner and expiry before network access, limits redirects to three, validates the initial URL and every redirect as HTTPS with a hostname resolving exclusively to public addresses, then accepts only 2xx image/* responses with at most maxAvatarBytes bytes.
- isPublicAvatarAddress rejects non-global, loopback, private, link-local, multicast, unspecified, and IPv6 addresses.
- Copy the configured http.Client before assigning CheckRedirect; the callback calls validatePublicHTTPSURL for every redirect. Parse Content-Type with mime.ParseMediaType, and read with io.LimitReader(response.Body, maxAvatarBytes+1).

- [x] **Step 4: Run the focused proxy tests to verify GREEN**

Run: go test ./internal/lanaccess -run TestAvatarProxy -count=1

Expected: PASS.

- [x] **Step 5: Commit the proxy unit**

Run: git add internal/lanaccess/avatar_proxy.go internal/lanaccess/avatar_proxy_test.go

Run: git commit -m "feat: add session-bound LAN avatar proxy"

### Task 2: Wire Avatar Rewriting And Serving Into The LAN Server

**Files:**
- Modify: internal/lanaccess/server.go:31-49, 91-125, 219-257
- Modify: internal/lanaccess/server_test.go

- [x] **Step 1: Write failing server integration tests**

Add TestServerRewritesRPCAvatarURLsAndServesTheOwnerImage. It must construct a TLS upstream image handler, return nested account and social avatarUrl values from testRPCDispatcher, and POST an authenticated /api/rpc/FarmSocialState request. Decode the JSON response and assert that it does not contain upstream.URL, its account and friend URL fields begin with /farm-avatars/, and an owner-cookie GET of the account path returns 200, Content-Type image/png, body png, and the unchanged CSP header.

Inject the TLS client and a lookup function returning netip.MustParseAddr("8.8.8.8") on handler.(*server).avatars. This isolates image-proxy logic from test-server loopback DNS.

Add TestServerRejectsAvatarTokenOutsideTheOwnerSession. It must verify an anonymous GET is 401, a POST with the owner cookie is 405 and Allow: GET, and an authenticated token generated for a different session is inaccessible.

- [x] **Step 2: Run the server tests to verify RED**

Run: go test ./internal/lanaccess -run 'TestServer(RewritesRPCAvatarURLsAndServesTheOwnerImage|RejectsAvatarTokenOutsideTheOwnerSession)' -count=1

Expected: FAIL because server has neither avatar proxy wiring nor a /farm-avatars/ route.

- [x] **Step 3: Wire the minimal server changes**

Add avatars *avatarProxy to server and initialize it as newAvatarProxy(auth.clock) in NewServer. Handle the route before the generic /api/ fallback:

~~~go
case strings.HasPrefix(r.URL.Path, avatarProxyPathPrefix):
	s.serveAvatar(w, r)
~~~

Preserve the session returned by requireSessionAndCSRF in rpcCall and change its successful response to:

~~~go
s.writeJSON(w, http.StatusOK, s.avatars.rewriteJSON(session, result))
~~~

Add this handler:

~~~go
func (s *server) serveAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, http.MethodGet)
		return
	}
	session, ok := s.sessionFromRequest(r)
	if !ok {
		s.writeError(w, http.StatusUnauthorized, "authorization required")
		return
	}
	token := strings.TrimPrefix(r.URL.Path, avatarProxyPathPrefix)
	if token == "" || strings.ContainsAny(token, "/\\") {
		s.writeError(w, http.StatusNotFound, "avatar not found")
		return
	}
	image, err := s.avatars.fetch(r.Context(), session, token)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "avatar not found")
		return
	}
	w.Header().Set("Content-Type", image.contentType)
	_, _ = w.Write(image.bytes)
}
~~~

- [x] **Step 4: Run LAN package regression tests**

Run: go test ./internal/lanaccess -count=1

Expected: PASS.

- [x] **Step 5: Commit the server wiring**

Run: git add internal/lanaccess/server.go internal/lanaccess/server_test.go

Run: git commit -m "fix: proxy LAN WebUI avatar images"

### Task 3: Verify The Full Application Contract

**Files:**
- Verify: internal/lanaccess/avatar_proxy.go
- Verify: internal/lanaccess/server.go
- Verify: frontend/src/components/AppShell.tsx
- Verify: frontend/src/components/AccountIdentityDialog.tsx

- [x] **Step 1: Run the complete Go suite**

Run: go test ./...

Expected: PASS.

- [x] **Step 2: Run the frontend test suite**

Run: npm test

Working directory: frontend

Expected: PASS. Existing components continue to consume avatarUrl, now supplied as a same-origin value in LAN mode.

- [x] **Step 3: Build frontend assets**

Run: npm run build

Working directory: frontend

Expected: exit code 0.

- [x] **Step 4: Inspect the final diff**

Run: git diff --check

Expected: no output. Confirm the pre-existing resources/gameConfig.bundle.zip modification remains untouched.

- [x] **Step 5: Confirm the final worktree**

Run: git status --short

Expected: avatar proxy and LAN server changes are committed; leave the pre-existing resource bundle modification unmodified.

## Plan Self-Review

- Spec coverage: Task 1 creates opaque session-bound rewriting plus destination, redirect, response type, and body-size validation. Task 2 exposes only the guarded same-origin route and preserves CSP. Task 3 checks the Go and frontend contracts.
- Scope: no friend-avatar UI is added; friend payload URLs are merely made safe for any existing or future image use.
- Type consistency: avatarProxyPathPrefix, maxAvatarBytes, newAvatarProxy, rewriteJSON, register, and fetch are introduced before the server task uses them.
