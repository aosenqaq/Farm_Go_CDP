# Read-Only Runtime Spy Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure account and profile reads do not install runtime Spy hooks while retaining the explicit diagnostic Spy entry point.

**Architecture:** Keep the existing embedded `resources/wmpf/button.js` API intact. Remove the two implicit Spy installer calls from profile-reading functions, and protect that boundary with a root-package test that inspects the exact embedded JavaScript function bodies. Explicit `startRuntimeSpies()` remains unchanged.

**Tech Stack:** Go 1.25, Go `embed`, standard-library `strings` and `testing`, embedded JavaScript runtime script.

---

## File Structure

- Create: `runtime_spy_read_test.go` — regression tests for the embedded runtime script's read-only and explicit-debug boundaries.
- Modify: `resources/wmpf/button.js:1930-1940` — remove Spy installation from `getPlayerProfile`.
- Modify: `resources/wmpf/button.js:8810-8816` — remove Spy installation from `getProtocolAccountProfile`.

### Task 1: Lock the Read-Only Boundary

**Files:**
- Create: `runtime_spy_read_test.go`
- Test: `runtime_spy_read_test.go`

- [ ] **Step 1: Write the failing test**

Create `runtime_spy_read_test.go` in package `main` with the following contents:

```go
package main

import (
	"strings"
	"testing"
)

func runtimeButtonFunctionBody(t *testing.T, name string) string {
	t.Helper()
	marker := "function " + name + "("
	start := strings.Index(runtimeButtonScript, marker)
	if start < 0 {
		t.Fatalf("function %s not found in runtime button script", name)
	}
	open := strings.Index(runtimeButtonScript[start:], "{")
	if open < 0 {
		t.Fatalf("function %s has no body", name)
	}
	open += start
	depth := 0
	for index := open; index < len(runtimeButtonScript); index++ {
		switch runtimeButtonScript[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return runtimeButtonScript[open+1 : index]
			}
		}
	}
	t.Fatalf("function %s body is not closed", name)
	return ""
}

func TestRuntimeButtonReadOnlyProfileFunctionsDoNotInstallSpies(t *testing.T) {
	for _, name := range []string{"getPlayerProfile", "getProtocolAccountProfile"} {
		if body := runtimeButtonFunctionBody(t, name); strings.Contains(body, "installRuntimeSpies()") {
			t.Fatalf("read-only function %s must not install runtime spies", name)
		}
	}
}

func TestRuntimeButtonExplicitSpyStartStillInstallsSpies(t *testing.T) {
	if body := runtimeButtonFunctionBody(t, "startRuntimeSpies"); !strings.Contains(body, "installRuntimeSpies()") {
		t.Fatal("explicit Spy start must retain the installer call")
	}
}
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run:

```powershell
go test . -run 'TestRuntimeButton(ReadOnlyProfileFunctionsDoNotInstallSpies|ExplicitSpyStartStillInstallsSpies)$' -count=1
```

Expected: FAIL with `read-only function getPlayerProfile must not install runtime spies`.

### Task 2: Remove Implicit Spy Installation From Reads

**Files:**
- Modify: `resources/wmpf/button.js:1930-1940`
- Modify: `resources/wmpf/button.js:8810-8816`
- Test: `runtime_spy_read_test.go`

- [ ] **Step 1: Remove the profile-read installer**

Change `getPlayerProfile` from:

```javascript
function getPlayerProfile(opts) {
  opts = opts || {};
  installRuntimeSpies();
  const farmModel = getFarmModel(opts);
```

to:

```javascript
function getPlayerProfile(opts) {
  opts = opts || {};
  const farmModel = getFarmModel(opts);
```

- [ ] **Step 2: Remove the protocol-profile installer**

Change `getProtocolAccountProfile` from:

```javascript
function getProtocolAccountProfile() {
  installRuntimeSpies();
  const oops = resolveOops();
```

to:

```javascript
function getProtocolAccountProfile() {
  const oops = resolveOops();
```

- [ ] **Step 3: Run the focused regression tests**

Run:

```powershell
go test . -run 'TestRuntimeButton(ReadOnlyProfileFunctionsDoNotInstallSpies|ExplicitSpyStartStillInstallsSpies)$' -count=1
```

Expected: PASS.

- [ ] **Step 4: Run the existing runtime bridge regression test**

Run:

```powershell
go test ./internal/runtime/wmpf -run '^TestCDPLinkInstallsRuntimeEventBridgeOnEveryConnection$' -count=1
```

Expected: PASS. This confirms the unrelated CDP bridge behavior remains unchanged.

- [ ] **Step 5: Review the final diff and commit**

Run:

```powershell
git diff --check
git diff -- resources/wmpf/button.js runtime_spy_read_test.go
```

Expected: no whitespace errors; only the two removed implicit calls and the regression test.

Commit:

```powershell
git add -- resources/wmpf/button.js runtime_spy_read_test.go
git commit -m "fix: keep runtime spies out of read paths"
```
