# LAN WebUI CSP Remote Bootstrap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Start the LAN WebUI in remote browser mode while retaining the strict CSP that blocks inline scripts.

**Architecture:** `internal/lanaccess/server.go` supplies a fixed same-origin JavaScript bootstrap resource and inserts an external `<script>` reference into the LAN index response. The React entrypoint remains unchanged: the bootstrap sets its existing remote-mode flag before the module script executes.

**Tech Stack:** Go `net/http`, `testing`, `testing/fstest`, React/Vite build verification.

---

### Task 1: Specify CSP-safe remote bootstrap responses

**Files:**

- Modify: `internal/lanaccess/server_test.go:117-151`
- Modify: `internal/lanaccess/server.go:72-125`

- [x] **Step 1: Write the failing tests**

Replace the current inline-marker assertion and add the fixed bootstrap route test:

```go
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
```

Add a second test:

```go
func TestServerServesCSPCompatibleRemoteBootstrap(t *testing.T) {
    server := newTestServer(t, ModeLAN, nil, nil)
    get := httptest.NewRecorder()
    server.ServeHTTP(get, newServerRequest(http.MethodGet, "/farm-go-remote.js", "192.168.1.10:3210", nil))

    if get.Code != http.StatusOK {
        t.Fatalf("GET status=%d", get.Code)
    }
    if got := get.Header().Get("Content-Type"); got != "application/javascript; charset=utf-8" {
        t.Fatalf("Content-Type=%q", got)
    }
    if got := get.Body.String(); got != "window.__FARM_GO_REMOTE__=true;\n" {
        t.Fatalf("body=%q", got)
    }

    post := httptest.NewRecorder()
    server.ServeHTTP(post, newServerRequest(http.MethodPost, "/farm-go-remote.js", "192.168.1.10:3210", nil))
    if post.Code != http.StatusMethodNotAllowed || post.Header().Get("Allow") != http.MethodGet {
        t.Fatalf("POST status=%d allow=%q", post.Code, post.Header().Get("Allow"))
    }
}
```

Add `strings` to the test imports.

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/lanaccess -run 'TestServerInjectsRemoteFlagIntoIndexOnly|TestServerServesCSPCompatibleRemoteBootstrap' -count=1`

Expected: FAIL because the index still contains an inline script and the bootstrap route is not defined.

- [x] **Step 3: Write minimal implementation**

Declare fixed constants near the existing server constants:

```go
const (
    remoteBootstrapPath   = "/farm-go-remote.js"
    remoteBootstrapScript = "window.__FARM_GO_REMOTE__=true;\n"
)
```

Handle the route before API dispatch:

```go
case r.URL.Path == remoteBootstrapPath:
    if r.Method != http.MethodGet {
        s.methodNotAllowed(w, http.MethodGet)
        return
    }
    w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
    _, _ = io.WriteString(w, remoteBootstrapScript)
```

Replace the injected inline tag with:

```go
[]byte("<script src=\"" + remoteBootstrapPath + "\"></script></head>")
```

- [x] **Step 4: Run focused tests to verify they pass**

Run: `go test ./internal/lanaccess -run 'TestServerInjectsRemoteFlagIntoIndexOnly|TestServerServesCSPCompatibleRemoteBootstrap' -count=1`

Expected: PASS.

- [x] **Step 5: Commit**

```powershell
git add internal/lanaccess/server.go internal/lanaccess/server_test.go
git commit -m "fix: bootstrap LAN remote mode under CSP"
```

### Task 2: Verify the running LAN WebUI response

**Files:**

- Verify: `internal/lanaccess/server.go`
- Verify: `internal/lanaccess/server_test.go`

- [x] **Step 1: Run the LAN server package suite**

Run: `go test ./internal/lanaccess -count=1`

Expected: PASS.

- [x] **Step 2: Build frontend assets**

Run: `npm run build`

Expected: PASS with TypeScript and Vite output.

- [x] **Step 3: Smoke the active LAN listener**

Run: `curl.exe -sS http://192.168.110.75:8788/` and `curl.exe -sS -D - http://192.168.110.75:8788/farm-go-remote.js -o NUL`

Expected: the index references `/farm-go-remote.js` with no inline marker; the bootstrap returns `200` as JavaScript under the unchanged strict CSP.

- [x] **Step 4: Commit the plan record and inspect the final worktree**

```powershell
git add docs/superpowers/plans/2026-07-22-lan-csp-remote-bootstrap.md
git commit -m "docs: plan LAN CSP bootstrap repair"
git status --short
git log --oneline -4
```
