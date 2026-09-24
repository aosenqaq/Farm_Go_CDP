package lanaccess

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

const serverTestPassword = "correct horse battery staple"

var (
	serverTestHashOnce sync.Once
	serverTestHash     string
	serverTestHashErr  error
)

type testRPCDispatcher struct {
	err      error
	method   string
	args     []json.RawMessage
	called   bool
	response any
}

func (d *testRPCDispatcher) Dispatch(_ context.Context, method string, args []json.RawMessage) (any, error) {
	d.called = true
	d.method = method
	d.args = args
	if d.err != nil {
		return nil, d.err
	}
	if d.response != nil {
		return d.response, nil
	}
	return map[string]bool{"ok": true}, nil
}

func testServerHash(t *testing.T) string {
	t.Helper()
	serverTestHashOnce.Do(func() {
		serverTestHash, serverTestHashErr = HashPassword(serverTestPassword)
	})
	if serverTestHashErr != nil {
		t.Fatal(serverTestHashErr)
	}
	return serverTestHash
}

func newTestServer(t *testing.T, mode Mode, dispatcher *testRPCDispatcher, poll http.Handler) http.Handler {
	t.Helper()
	auth := NewAuthenticator(nil)
	hash := testServerHash(t)
	auth.SetPasswordHash(hash)
	if dispatcher == nil {
		dispatcher = &testRPCDispatcher{}
	}
	return NewServer(Options{
		Config: Config{Enabled: true, Mode: mode, Port: 8788, PasswordHash: hash},
		Auth:   auth,
		Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("remote password page")}},
		RPC:    dispatcher,
		Poll:   poll,
	})
}

func newAuthenticatedTestServer(t *testing.T, mode Mode, dispatcher *testRPCDispatcher) (http.Handler, Session) {
	t.Helper()
	auth := NewAuthenticator(nil)
	hash := testServerHash(t)
	auth.SetPasswordHash(hash)
	session, err := auth.Login(serverTestPassword, "192.168.1.10:3210", mode)
	if err != nil {
		t.Fatal(err)
	}
	if dispatcher == nil {
		dispatcher = &testRPCDispatcher{}
	}
	return NewServer(Options{
		Config: Config{Enabled: true, Mode: mode, Port: 8788, PasswordHash: hash},
		Auth:   auth,
		Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("remote password page")}},
		RPC:    dispatcher,
	}), session
}

func newServerRequest(method, target, remote string, body []byte) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	req.RemoteAddr = remote
	return req
}

func addAuthenticatedHeaders(req *http.Request, session Session) {
	req.Host = "farm.test"
	req.Header.Set("Origin", "http://farm.test")
	req.Header.Set("X-Farm-Go-CSRF", session.CSRF)
	req.AddCookie(&http.Cookie{Name: "farm_go_lan", Value: session.ID})
}

func TestServerServesStaticPasswordPage(t *testing.T) {
	server := newTestServer(t, ModeLAN, nil, nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, newServerRequest(http.MethodGet, "/", "192.168.1.10:3210", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := rec.Body.String(); got != "remote password page" {
		t.Fatalf("body=%q", got)
	}
}

func TestServerInjectsRemoteFlagIntoIndexOnly(t *testing.T) {
	hash := testServerHash(t)
	assets := fstest.MapFS{
		"index.html":     &fstest.MapFile{Data: []byte("<html><head><title>Farm</title></head><body></body></html>")},
		"assets/app.bin": &fstest.MapFile{Data: []byte{0x00, 0x01, 0xff, 0x02}},
	}
	server := NewServer(Options{
		Config: Config{Enabled: true, Mode: ModeLAN, Port: 8788, PasswordHash: hash},
		Assets: assets,
	})

	for _, target := range []string{"/", "/index.html"} {
		t.Run(target, func(t *testing.T) {
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, newServerRequest(http.MethodGet, target, "192.168.1.10:3210", nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d", rec.Code)
			}
			want := "<html><head><title>Farm</title><script src=\"/farm-go-remote.js\"></script></head><body></body></html>"
			if got := rec.Body.String(); got != want {
				t.Fatalf("body=%q", got)
			}
			if strings.Contains(rec.Body.String(), "window.__FARM_GO_REMOTE__") {
				t.Fatalf("index contains a CSP-blocked inline remote marker: %q", rec.Body.String())
			}
			if got := rec.Header().Get("Content-Security-Policy"); got != "default-src 'self'; base-uri 'self'; frame-ancestors 'self'; form-action 'self'" {
				t.Fatalf("CSP=%q", got)
			}
		})
	}

	assetRec := httptest.NewRecorder()
	server.ServeHTTP(assetRec, newServerRequest(http.MethodGet, "/assets/app.bin", "192.168.1.10:3210", nil))
	if assetRec.Code != http.StatusOK {
		t.Fatalf("asset status=%d", assetRec.Code)
	}
	if got, want := assetRec.Body.Bytes(), []byte{0x00, 0x01, 0xff, 0x02}; !bytes.Equal(got, want) {
		t.Fatalf("asset bytes=%v", got)
	}
}

func TestServerServesGameAssetsWithStaticSecurityBoundary(t *testing.T) {
	called := 0
	server := NewServer(Options{
		Config: Config{Enabled: true, Mode: ModeLAN, Port: 8788, PasswordHash: testServerHash(t)},
		Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("remote password page")}},
		GameAssets: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called++
			if r.URL.Path != "/farm-assets/registered" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png"))
		}),
	})

	get := httptest.NewRecorder()
	server.ServeHTTP(get, newServerRequest(http.MethodGet, "/farm-assets/registered", "192.168.1.10:3210", nil))
	if get.Code != http.StatusOK || get.Body.String() != "png" || get.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("unexpected game asset response: code=%d body=%q type=%q", get.Code, get.Body.String(), get.Header().Get("Content-Type"))
	}
	if get.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("cache control=%q", get.Header().Get("Cache-Control"))
	}
	if get.Header().Get("Content-Security-Policy") != "default-src 'self'; base-uri 'self'; frame-ancestors 'self'; form-action 'self'" {
		t.Fatalf("CSP=%q", get.Header().Get("Content-Security-Policy"))
	}

	for _, request := range []*http.Request{
		newServerRequest(http.MethodPost, "/farm-assets/registered", "192.168.1.10:3210", nil),
		newServerRequest(http.MethodGet, "/farm-assets/registered/nested", "192.168.1.10:3210", nil),
		newServerRequest(http.MethodGet, "/farm-assets/registered", "8.8.8.8:3210", nil),
	} {
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, request)
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
			t.Fatalf("unexpected rejected response for %s %s: %d", request.Method, request.URL.Path, rec.Code)
		}
	}
	if called != 1 {
		t.Fatalf("game asset handler calls=%d, want 1", called)
	}
}

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

func TestServerRejectsRPCWithoutSession(t *testing.T) {
	server := newTestServer(t, ModeLAN, nil, nil)
	req := newServerRequest(http.MethodPost, "/api/rpc/FarmAutomationState", "192.168.1.10:3210", []byte(`{"args":[]}`))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestServerAcceptsAutomationSavePayloadAboveOneMiB(t *testing.T) {
	dispatcher := &testRPCDispatcher{}
	server, session := newAuthenticatedTestServer(t, ModeLAN, dispatcher)
	payload := `{"args":[{"config":{"autoFarmFriendStealCropOptions":"` + strings.Repeat("x", 3<<20) + `"}}]}`
	request := newServerRequest(http.MethodPost, "/api/rpc/SaveFarmAutomationState", "192.168.1.10:3210", []byte(payload))
	addAuthenticatedHeaders(request, session)
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if !dispatcher.called || dispatcher.method != "SaveFarmAutomationState" {
		t.Fatalf("dispatcher call = %#v, want SaveFarmAutomationState", dispatcher)
	}
}

func TestServerRejectsForgedCookieBeforeOriginAndCSRF(t *testing.T) {
	server := newTestServer(t, ModeLAN, nil, nil)
	req := newServerRequest(http.MethodPost, "/api/rpc/FarmAutomationState", "192.168.1.10:3210", []byte(`{"args":[]}`))
	req.AddCookie(&http.Cookie{Name: "farm_go_lan", Value: "forged"})
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestServerRejectsRPCWithoutExactOriginOrCSRF(t *testing.T) {
	server, session := newAuthenticatedTestServer(t, ModeLAN, nil)

	for _, tc := range []struct {
		name    string
		prepare func(*http.Request)
	}{
		{
			name: "missing origin",
			prepare: func(req *http.Request) {
				req.AddCookie(&http.Cookie{Name: "farm_go_lan", Value: session.ID})
			},
		},
		{
			name: "missing csrf",
			prepare: func(req *http.Request) {
				req.Host = "farm.test"
				req.Header.Set("Origin", "http://farm.test")
				req.AddCookie(&http.Cookie{Name: "farm_go_lan", Value: session.ID})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := newServerRequest(http.MethodPost, "/api/rpc/FarmAutomationState", "192.168.1.10:3210", []byte(`{"args":[]}`))
			tc.prepare(req)
			rec := httptest.NewRecorder()

			server.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status=%d", rec.Code)
			}
		})
	}
}

func TestServerMapsMethodNotAllowedToNotFound(t *testing.T) {
	dispatcher := &testRPCDispatcher{err: ErrMethodNotAllowed}
	server, session := newAuthenticatedTestServer(t, ModeLAN, dispatcher)
	req := newServerRequest(http.MethodPost, "/api/rpc/ExitApplication", "192.168.1.10:3210", []byte(`{"args":[]}`))
	addAuthenticatedHeaders(req, session)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestServerRejectsPublicLANAndNonLoopbackTunnelSources(t *testing.T) {
	for _, tc := range []struct {
		mode   Mode
		remote string
	}{
		{mode: ModeLAN, remote: "8.8.8.8:3210"},
		{mode: ModeTunnel, remote: "192.168.1.5:3210"},
	} {
		t.Run(string(tc.mode), func(t *testing.T) {
			server := newTestServer(t, tc.mode, nil, nil)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, newServerRequest(http.MethodGet, "/api/auth/session", tc.remote, nil))

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status=%d", rec.Code)
			}
		})
	}
}

func TestServerLoginCookieUsesSecureOnlyForTunnel(t *testing.T) {
	for _, mode := range []Mode{ModeLAN, ModeTunnel} {
		t.Run(string(mode), func(t *testing.T) {
			server := newTestServer(t, mode, nil, nil)
			remote := "192.168.1.10:3210"
			if mode == ModeTunnel {
				remote = "127.0.0.1:3210"
			}
			req := newServerRequest(http.MethodPost, "/api/auth/login", remote, []byte(`{"password":"correct horse battery staple"}`))
			rec := httptest.NewRecorder()

			server.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d", rec.Code)
			}
			cookies := rec.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("cookies=%d", len(cookies))
			}
			cookie := cookies[0]
			if cookie.Name != "farm_go_lan" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge != 43200 || cookie.Secure != (mode == ModeTunnel) {
				t.Fatalf("cookie=%+v", cookie)
			}
		})
	}
}

func TestServerProtectsPollHandler(t *testing.T) {
	pollCalled := false
	poll := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		pollCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	server := newTestServer(t, ModeLAN, nil, poll)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, newServerRequest(http.MethodGet, "/farm-api/poll/dashboard", "192.168.1.10:3210", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
	if pollCalled {
		t.Fatal("poll handler was called without a session")
	}
}

func TestServerEnforcesSecurityHeadersOnPollResponses(t *testing.T) {
	auth := NewAuthenticator(nil)
	hash := testServerHash(t)
	auth.SetPasswordHash(hash)
	session, err := auth.Login(serverTestPassword, "192.168.1.10:3210", ModeLAN)
	if err != nil {
		t.Fatal(err)
	}
	poll := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "https://untrusted.example")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.WriteHeader(http.StatusNoContent)
	})
	server := NewServer(Options{
		Config: Config{Enabled: true, Mode: ModeLAN, Port: 8788, PasswordHash: hash},
		Auth:   auth,
		Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("remote password page")}},
		Poll:   poll,
	})
	req := newServerRequest(http.MethodGet, "/farm-api/poll/dashboard", "192.168.1.10:3210", nil)
	req.AddCookie(&http.Cookie{Name: "farm_go_lan", Value: session.ID})
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control=%q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("cors header=%q", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("frame-options=%q", got)
	}
}

func TestServerReturnsSessionCSRFTokenAndProtectsLogout(t *testing.T) {
	server, session := newAuthenticatedTestServer(t, ModeLAN, nil)
	sessionReq := newServerRequest(http.MethodGet, "/api/auth/session", "192.168.1.10:3210", nil)
	sessionReq.AddCookie(&http.Cookie{Name: "farm_go_lan", Value: session.ID})
	sessionRec := httptest.NewRecorder()

	server.ServeHTTP(sessionRec, sessionReq)

	if sessionRec.Code != http.StatusOK {
		t.Fatalf("session status=%d", sessionRec.Code)
	}
	var payload struct {
		Authorized bool   `json:"authorized"`
		CSRFToken  string `json:"csrfToken"`
	}
	if err := json.Unmarshal(sessionRec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Authorized || payload.CSRFToken != session.CSRF {
		t.Fatalf("payload=%+v", payload)
	}

	logoutReq := newServerRequest(http.MethodPost, "/api/auth/logout", "192.168.1.10:3210", nil)
	addAuthenticatedHeaders(logoutReq, session)
	logoutRec := httptest.NewRecorder()
	server.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout status=%d", logoutRec.Code)
	}
}

func TestServerRejectsMalformedRPCArgs(t *testing.T) {
	server, session := newAuthenticatedTestServer(t, ModeLAN, nil)
	req := newServerRequest(http.MethodPost, "/api/rpc/FarmAutomationState", "192.168.1.10:3210", []byte(`{"args":{}}`))
	addAuthenticatedHeaders(req, session)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestServerDoesNotTrustForwardedProto(t *testing.T) {
	server, session := newAuthenticatedTestServer(t, ModeLAN, nil)
	req := newServerRequest(http.MethodPost, "/api/rpc/FarmAutomationState", "192.168.1.10:3210", []byte(`{"args":[]}`))
	addAuthenticatedHeaders(req, session)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestServerAcceptsHTTPSOriginAfterTunnelTLSIsTerminated(t *testing.T) {
	server, session := newAuthenticatedTestServer(t, ModeTunnel, nil)
	req := newServerRequest(http.MethodPost, "/api/rpc/FarmAutomationState", "127.0.0.1:3210", []byte(`{"args":[]}`))
	req.Host = "farm.example"
	req.Header.Set("Origin", "https://farm.example")
	req.Header.Set("X-Forwarded-Proto", "http")
	req.Header.Set("X-Farm-Go-CSRF", session.CSRF)
	req.AddCookie(&http.Cookie{Name: "farm_go_lan", Value: session.ID})
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestServerRewritesNestedRPCAvatarURLsAsSameOriginPaths(t *testing.T) {
	dispatcher := &testRPCDispatcher{response: map[string]any{
		"account": map[string]any{"avatarUrl": "https://avatar.example/account.png"},
		"social":  map[string]any{"friends": []any{map[string]any{"avatarUrl": "https://avatar.example/friend.png"}}},
	}}
	server, session := newAuthenticatedTestServer(t, ModeLAN, dispatcher)
	req := newServerRequest(http.MethodPost, "/api/rpc/FarmSocialState", "192.168.1.10:3210", []byte(`{"args":[]}`))
	addAuthenticatedHeaders(req, session)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "avatar.example") {
		t.Fatalf("external avatar URL leaked: %s", rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	accountURL := payload["account"].(map[string]any)["avatarUrl"].(string)
	friendURL := payload["social"].(map[string]any)["friends"].([]any)[0].(map[string]any)["avatarUrl"].(string)
	for _, avatarURL := range []string{accountURL, friendURL} {
		if !strings.HasPrefix(avatarURL, "/farm-avatars/") {
			t.Fatalf("avatar URL = %q", avatarURL)
		}
	}
}

func TestServerServesAvatarOnlyToTheOwningSession(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png"))
	}))
	defer upstream.Close()

	dispatcher := &testRPCDispatcher{response: map[string]any{"avatarUrl": upstream.URL + "/account"}}
	handler, owner := newAuthenticatedTestServer(t, ModeLAN, dispatcher)
	server := handler.(*server)
	server.avatars.client = upstream.Client()
	server.avatars.lookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	rpc := newServerRequest(http.MethodPost, "/api/rpc/FarmAccountStatus", "192.168.1.10:3210", []byte(`{"args":[]}`))
	addAuthenticatedHeaders(rpc, owner)
	rpcRec := httptest.NewRecorder()
	server.ServeHTTP(rpcRec, rpc)
	var payload map[string]any
	if err := json.Unmarshal(rpcRec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	avatarPath := payload["avatarUrl"].(string)

	for _, tc := range []struct {
		name    string
		method  string
		prepare func(*http.Request)
		want    int
	}{
		{
			name:   "owner",
			method: http.MethodGet,
			prepare: func(req *http.Request) {
				req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: owner.ID})
			},
			want: http.StatusOK,
		},
		{name: "anonymous", method: http.MethodGet, prepare: func(*http.Request) {}, want: http.StatusUnauthorized},
		{
			name:   "other session",
			method: http.MethodGet,
			prepare: func(req *http.Request) {
				other, err := server.auth.Login(serverTestPassword, "192.168.1.11:3210", ModeLAN)
				if err != nil {
					t.Fatal(err)
				}
				req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: other.ID})
			},
			want: http.StatusNotFound,
		},
		{
			name:   "post",
			method: http.MethodPost,
			prepare: func(req *http.Request) {
				req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: owner.ID})
			},
			want: http.StatusMethodNotAllowed,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := newServerRequest(tc.method, avatarPath, "192.168.1.10:3210", nil)
			tc.prepare(req)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status=%d", rec.Code)
			}
			if tc.name == "owner" && (rec.Header().Get("Content-Type") != "image/png" || rec.Body.String() != "png") {
				t.Fatalf("image response = %q %q", rec.Header().Get("Content-Type"), rec.Body.String())
			}
			if got := rec.Header().Get("Content-Security-Policy"); got != "default-src 'self'; base-uri 'self'; frame-ancestors 'self'; form-action 'self'" {
				t.Fatalf("CSP=%q", got)
			}
		})
	}
}

var _ RPCDispatcher = (*testRPCDispatcher)(nil)
