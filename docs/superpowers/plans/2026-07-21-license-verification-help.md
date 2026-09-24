# License Verification Help Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an unauthenticated KAuth connection diagnostic and a “无法验证？” dialog to the card-license gate, including DNS comparison and a non-mutating recommendation.

**Architecture:** Keep DNS lookup, Windows ping parsing, concurrent checks, and recommendation ranking in a focused `internal/license` service with injected functions for deterministic tests. Expose one fixed-scope Wails method from `App`, pass it into `LicenseGate`, and render all help, loading, partial-failure, and recommendation states inside the gate dialog.

**Tech Stack:** Go 1.25, `net.Resolver`, `os/exec`, Wails v2, React 18, TypeScript, Vitest, react-test-renderer, lucide-react.

**Design Reference:** `docs/superpowers/specs/2026-07-21-license-verification-help-design.md`

---

## File Map

- Create `internal/license/connection_diagnostic.go`: diagnostic DTOs, ping-output parser, DNS resolver, Windows ping runner, concurrent orchestration, and recommendation ranking.
- Create `internal/license/connection_diagnostic_test.go`: parser, failure, timeout, concurrency, and ranking tests using injected lookup and ping functions.
- Modify `license_api.go`: expose `CheckLicenseConnection()` without an authorization guard.
- Modify `license_api_test.go`: verify the diagnostic is callable while the app is locked and delegates to an injected checker.
- Modify `app.go`: hold the diagnostic function dependency initialized by `NewApp`.
- Modify `frontend/wailsjs/go/main/App.js` and `App.d.ts`: add the generated-style binding without regenerating unrelated files.
- Modify `frontend/wailsjs/go/models.ts`: add only the new `license.Connection*` models while preserving the existing user whitespace changes.
- Modify `frontend/src/App.tsx`: import the binding, normalize the Wails result, and pass an `onCheckConnection` callback into `LicenseGate`.
- Modify `frontend/src/LicenseGate.tsx`: add the entry, accessible dialog, diagnostics state, result table, and recommendation.
- Modify `frontend/src/LicenseGate.test.tsx`: cover dialog behavior and every user-visible result state.
- Modify `frontend/src/style.css`: style the feedback-row entry, dialog, responsive table, and reduced-motion loading state.

### Task 1: Ping Parsing And Recommendation Logic

**Files:**
- Create: `internal/license/connection_diagnostic_test.go`
- Create: `internal/license/connection_diagnostic.go`

- [ ] **Step 1: Write failing parser and ranking tests**

Add table tests that call the wished-for APIs:

```go
func TestParseWindowsPingOutput(t *testing.T) {
    cases := []struct {
        name string
        output string
        want PingResult
    }{
        {"english", "Reply from 1.2.3.4: bytes=32 time=20ms TTL=55\nReply from 1.2.3.4: bytes=32 time<1ms TTL=55\n", PingResult{Sent: 4, Received: 2, AverageMS: 11}},
        {"chinese", "来自 1.2.3.4 的回复: 字节=32 时间=31ms TTL=55\n来自 1.2.3.4 的回复: 字节=32 时间=33ms TTL=55\n", PingResult{Sent: 4, Received: 2, AverageMS: 32}},
        {"timeout", "Request timed out.\n", PingResult{Sent: 4}},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            if got := parseWindowsPingOutput([]byte(tc.output), 4); !reflect.DeepEqual(got, tc.want) {
                t.Fatalf("parseWindowsPingOutput() = %#v, want %#v", got, tc.want)
            }
        })
    }
}

func TestRecommendConnectionResult(t *testing.T) {
    results := []ConnectionResult{
        {ID: "system", Reachable: true, AveragePingMS: 35, ResolveMS: 20},
        {ID: "aliyun", Reachable: true, AveragePingMS: 30, ResolveMS: 25},
        {ID: "dns114", Reachable: false, AveragePingMS: 5, ResolveMS: 5},
    }
    if got := recommendConnectionResult(results); got != "aliyun" {
        t.Fatalf("recommendConnectionResult() = %q, want aliyun", got)
    }
}
```

Also add ranking cases for equal ping resolved by lower DNS time, a complete tie preserving candidate order, and no reachable candidates returning an empty ID.

- [ ] **Step 2: Run tests and confirm RED**

Run: `go test ./internal/license -run 'Test(ParseWindowsPingOutput|RecommendConnectionResult)' -count=1`

Expected: FAIL because `PingResult`, `ConnectionResult`, `parseWindowsPingOutput`, and `recommendConnectionResult` do not exist.

- [ ] **Step 3: Implement the minimal parser and selector**

Define JSON DTOs with stable camel-case fields:

```go
type PingResult struct {
    Sent      int `json:"sent"`
    Received  int `json:"received"`
    AverageMS int `json:"averageMs"`
}

type ConnectionResult struct {
    ID            string `json:"id"`
    Name          string `json:"name"`
    DNSAddress    string `json:"dnsAddress,omitempty"`
    IPAddress     string `json:"ipAddress,omitempty"`
    ResolveMS     int `json:"resolveMs,omitempty"`
    Sent          int `json:"sent"`
    Received      int `json:"received"`
    AveragePingMS int `json:"averagePingMs,omitempty"`
    Reachable     bool `json:"reachable"`
    ErrorCode     string `json:"errorCode,omitempty"`
    Message       string `json:"message,omitempty"`
}

type ConnectionReport struct {
    Host          string             `json:"host"`
    CheckedAt     string             `json:"checkedAt"`
    Results       []ConnectionResult `json:"results"`
    RecommendedID string             `json:"recommendedId,omitempty"`
    Message       string             `json:"message,omitempty"`
}
```

Parse only lines containing case-insensitive `ttl=`. Extract `[=<]\s*(\d+)ms`; count `<1ms` as 1 ms. Compute an integer rounded average from successful replies. Sort only a copied slice of reachable candidates by average ping, resolve time, then original order.

- [ ] **Step 4: Run tests and confirm GREEN**

Run: `go test ./internal/license -run 'Test(ParseWindowsPingOutput|RecommendConnectionResult)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the pure diagnostic logic**

Run:

```powershell
git add internal/license/connection_diagnostic.go internal/license/connection_diagnostic_test.go
git commit -m "feat: parse license connection diagnostics"
```

### Task 2: DNS And Ping Diagnostic Orchestration

**Files:**
- Modify: `internal/license/connection_diagnostic.go`
- Modify: `internal/license/connection_diagnostic_test.go`

- [ ] **Step 1: Write failing orchestration tests**

Create injected function types and test `checkConnections` with four fixed candidates. The test lookup returns deterministic IPs and durations; ping returns success for three candidates and an error for one. Assert all four results return in candidate order, one error does not abort the report, and the fastest reachable result is recommended.

```go
type DNSCandidate struct {
    ID      string
    Name    string
    Address string
}

type lookupIPv4Func func(context.Context, DNSCandidate, string) (string, int, error)
type pingIPv4Func func(context.Context, string) (PingResult, error)

lookup := func(_ context.Context, candidate DNSCandidate, _ string) (string, int, error) {
    if candidate.ID == "dns114" { return "", 9, context.DeadlineExceeded }
    return map[string]string{"system":"1.1.1.1", "aliyun":"2.2.2.2", "dnspod":"3.3.3.3"}[candidate.ID], 10, nil
}
ping := func(_ context.Context, ip string) (PingResult, error) {
    latency := map[string]int{"1.1.1.1":40, "2.2.2.2":25, "3.3.3.3":30}[ip]
    return PingResult{Sent:4, Received:4, AverageMS:latency}, nil
}
report := checkConnections(context.Background(), defaultDNSCandidates(), lookup, ping)
```

Add a timeout test whose lookup blocks until `ctx.Done()`, and a validation test proving the real ping runner rejects a non-IP string before invoking a command.

- [ ] **Step 2: Run tests and confirm RED**

Run: `go test ./internal/license -run 'Test(CheckConnections|RunWindowsPing)' -count=1`

Expected: FAIL because the orchestration APIs do not exist.

- [ ] **Step 3: Implement real diagnostics**

Add these fixed candidates in order:

```go
[]DNSCandidate{
    {ID:"system", Name:"系统 DNS"},
    {ID:"aliyun", Name:"阿里公共 DNS", Address:"223.5.5.5"},
    {ID:"dns114", Name:"114DNS", Address:"114.114.114.114"},
    {ID:"dnspod", Name:"腾讯 DNSPod", Address:"119.29.29.29"},
}
```

Implement system lookup with `net.DefaultResolver.LookupIP(ctx, "ip4", host)`. Implement fixed-server lookup with `net.Resolver{PreferGo:true, Dial: ...}` dialing UDP `<address>:53`. Measure lookup duration, return the first valid IPv4, and classify deadline, lookup, empty-result, ping-unavailable, and ping-timeout failures into fixed Chinese messages.

Run Windows ping with `exec.CommandContext(ctx, "ping", "-4", "-n", "4", "-w", "1000", ip)`. Validate `net.ParseIP(ip).To4() != nil` before command creation. Treat a non-zero exit with at least one parsed reply as partial success; otherwise return a timeout result. Run candidates concurrently into indexed slots under an 8-second report context.

Export:

```go
func CheckKAuthConnection(ctx context.Context) ConnectionReport
```

It always targets `api.kauth.cn`, sets `CheckedAt` in RFC3339 format, and returns the recommendation ID calculated from completed results.

- [ ] **Step 4: Run tests and confirm GREEN**

Run: `go test ./internal/license -run 'Test(CheckConnections|RunWindowsPing|ParseWindowsPingOutput|RecommendConnectionResult)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the diagnostic orchestration**

Run:

```powershell
git add internal/license/connection_diagnostic.go internal/license/connection_diagnostic_test.go
git commit -m "feat: compare KAuth DNS connectivity"
```

### Task 3: Expose The Unauthenticated Wails Method

**Files:**
- Modify: `app.go`
- Modify: `license_api.go`
- Modify: `license_api_test.go`
- Modify: `frontend/wailsjs/go/main/App.js`
- Modify: `frontend/wailsjs/go/main/App.d.ts`
- Modify: `frontend/wailsjs/go/models.ts`

- [ ] **Step 1: Write a failing App API test**

Add an injected checker to a locked `App` and call the wished-for method without setting authorization:

```go
func TestCheckLicenseConnectionDoesNotRequireAuthorization(t *testing.T) {
    called := false
    app := &App{checkLicenseConnection: func(context.Context) license.ConnectionReport {
        called = true
        return license.ConnectionReport{Host:"api.kauth.cn", RecommendedID:"aliyun"}
    }}
    got := app.CheckLicenseConnection()
    if !called || got.RecommendedID != "aliyun" {
        t.Fatalf("CheckLicenseConnection() = %#v, called=%v", got, called)
    }
}
```

- [ ] **Step 2: Run the App test and confirm RED**

Run: `go test . -run TestCheckLicenseConnectionDoesNotRequireAuthorization -count=1`

Expected: FAIL because the dependency and method do not exist.

- [ ] **Step 3: Implement App delegation and bindings**

Add `checkLicenseConnection func(context.Context) license.ConnectionReport` to `App`, initialize it to `license.CheckKAuthConnection` in `NewApp`, and expose:

```go
func (a *App) CheckLicenseConnection() license.ConnectionReport {
    checker := a.checkLicenseConnection
    if checker == nil { checker = license.CheckKAuthConnection }
    return checker(a.contextOrBackground())
}
```

Do not call `requireAuthorized`. Add generated-style `CheckLicenseConnection()` wrappers to `App.js` and `App.d.ts`. Incrementally add `ConnectionReport` and `ConnectionResult` classes to the existing `license` namespace in `models.ts`; preserve all pre-existing whitespace changes and inspect the focused diff before staging.

- [ ] **Step 4: Run tests and type generation checks**

Run:

```powershell
go test . -run TestCheckLicenseConnectionDoesNotRequireAuthorization -count=1
go test ./internal/license -count=1
```

Expected: both PASS.

- [ ] **Step 5: Commit the API surface**

Run:

```powershell
git add app.go license_api.go license_api_test.go frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/models.ts
git commit -m "feat: expose license connection check"
```

### Task 4: Build The Help Dialog With TDD

**Files:**
- Modify: `frontend/src/LicenseGate.test.tsx`
- Modify: `frontend/src/LicenseGate.tsx`
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Write failing entry and dialog tests**

Update every existing `LicenseGate` render with `onCheckConnection={async () => emptyReport}` through a shared test factory, defining the fixture before the tests:

```tsx
const emptyReport: ConnectionReportDto = {
  host: 'api.kauth.cn',
  checkedAt: '2026-07-21T12:00:00+08:00',
  results: [],
};
```

Add tests asserting:

```tsx
const help = renderer.root.findByProps({ name: 'open-license-help' });
expect(help.children.join('')).toContain('无法验证？');
act(() => help.props.onClick());
const dialog = renderer.root.findByProps({ 'aria-label': '卡密验证帮助' });
expect(JSON.stringify(dialog.toJSON())).toContain('检查卡密是否正确');
expect(JSON.stringify(dialog.toJSON())).toContain('检查本机时区与时间');
expect(JSON.stringify(dialog.toJSON())).toContain('检查验证服务器连通状态');
expect(JSON.stringify(dialog.toJSON())).toContain('比较公共 DNS');
```

Add close-button, backdrop, and Escape tests. Add an async test holding a pending promise and asserting the check button is disabled with “正在检查”.

- [ ] **Step 2: Run component tests and confirm RED**

Run: `Set-Location frontend; npm test -- src/LicenseGate.test.tsx`

Expected: FAIL because the new prop, entry, and dialog do not exist.

- [ ] **Step 3: Implement the dialog shell and App callback**

Define local frontend DTO interfaces matching the Wails JSON names. Add `onCheckConnection: () => Promise<ConnectionReportDto>` to `LicenseGateProps`. Keep `helpOpen`, `checking`, `report`, and `checkError` state. Render the chosen B layout with `.license-feedback-row`, a left `role="status"` paragraph, and a right help button using Lucide `CircleHelp`.

The dialog must have `role="dialog"`, `aria-modal="true"`, `aria-label="卡密验证帮助"`, an icon-only `X` close button with tooltip, the four approved items, and a primary “检查连接状态” button. Register Escape only while open and stop backdrop propagation from the surface.

In `App.tsx`, import `CheckLicenseConnection`, convert the generated Wails model to the DTO shape without changing values, and pass `onCheckConnection={() => CheckLicenseConnection()}`.

- [ ] **Step 4: Run component tests and confirm GREEN**

Run: `Set-Location frontend; npm test -- src/LicenseGate.test.tsx`

Expected: all `LicenseGate` tests PASS.

- [ ] **Step 5: Commit the dialog shell**

Run:

```powershell
git add frontend/src/LicenseGate.tsx frontend/src/LicenseGate.test.tsx frontend/src/App.tsx
git commit -m "feat: add license verification help dialog"
```

### Task 5: Render Diagnostic Results And Responsive Styling

**Files:**
- Modify: `frontend/src/LicenseGate.test.tsx`
- Modify: `frontend/src/LicenseGate.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write failing result-state tests**

Add successful, partial, and all-failed reports. Assert the successful view includes the recommended DNS and address, IP, resolve time, `4/4`, average ping, and the non-mutating disclaimer. Assert partial failure shows the failed row message while retaining the recommendation. Assert all-failed contains no recommendation and shows the overall network guidance. Add a CSS source test ensuring the feedback row uses `minmax(0, 1fr) auto` and the dialog width is constrained by `calc(100vw - 32px)`.

- [ ] **Step 2: Run tests and confirm RED**

Run: `Set-Location frontend; npm test -- src/LicenseGate.test.tsx`

Expected: FAIL because results and new selectors are absent.

- [ ] **Step 3: Implement results and styles**

Render one stable result row per backend candidate. Use semantic text for failure states instead of blank numeric cells. Mark the recommended row with a class and use a separate recommendation banner. Keep the exact normal-state button label “检查连接状态”; use `Loader2` only during the pending state.

Add scoped styles for:

```css
.license-feedback-row { display:grid; grid-template-columns:minmax(0,1fr) auto; gap:12px; align-items:start; }
.license-help-dialog { width:min(560px,calc(100vw - 32px)); max-height:calc(100vh - 32px); overflow:auto; border-radius:8px; }
.license-connection-grid { display:grid; grid-template-columns:minmax(110px,1.2fr) minmax(100px,1fr) 70px 88px; }
```

At narrow widths, hide the IP column visually while retaining DNS, status, and ping. Ensure long errors wrap inside the left feedback column and do not resize the help button.

- [ ] **Step 4: Run focused tests and build**

Run:

```powershell
Set-Location frontend
npm test -- src/LicenseGate.test.tsx
npm run build
```

Expected: tests PASS and the TypeScript/Vite build exits 0.

- [ ] **Step 5: Commit the completed UI**

Run:

```powershell
git add frontend/src/LicenseGate.tsx frontend/src/LicenseGate.test.tsx frontend/src/style.css
git commit -m "feat: show license connection recommendations"
```

### Task 6: Full Verification And Live Inspection

**Files:**
- Verify only; change code only in response to a reproduced failure and add a regression test first.

- [ ] **Step 1: Run formatting and focused tests**

Run:

```powershell
gofmt -w internal/license/connection_diagnostic.go internal/license/connection_diagnostic_test.go app.go license_api.go license_api_test.go
go test ./internal/license -count=1
go test . -run 'Test(CheckLicenseConnection|License)' -count=1
```

Expected: PASS with no formatting diff beyond the feature files.

- [ ] **Step 2: Run the complete automated suite**

Run:

```powershell
go test ./... -count=1
Set-Location frontend
npm test
npm run build
```

Expected: every command exits 0 with no failed tests.

- [ ] **Step 3: Exercise the real local diagnostic**

Run a focused Go integration test guarded to Windows or a temporary `go test` entry that calls `CheckKAuthConnection(context.Background())`. Confirm the report contains all four candidates and each successful candidate reports a validated IPv4 plus a non-negative average latency. Do not assert a specific winning DNS because network conditions vary.

- [ ] **Step 4: Inspect the actual Wails UI**

Start `wails dev`, open the locked license gate, click “无法验证？”, and verify desktop and narrow-window screenshots. Confirm no overlap, the selected B entry stays visible beside long errors, the spinner does not shift layout, the table remains readable, and a real check displays the current machine results.

- [ ] **Step 5: Review repository state**

Run:

```powershell
git status --short
git diff --check HEAD
git log --oneline -8
```

Expected: only intentional feature changes or explicitly preserved user changes remain; no whitespace errors; the design, plan, backend, API, dialog, and result commits are visible.
