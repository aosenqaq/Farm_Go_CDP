# QQ Native Add-Friend Capture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Record QQ native add-friend, profile, card, and deep-link API calls made by a manual stranger-farm add-friend click.

**Architecture:** Keep the existing `shareApiEvents` artifact contract so the generic protocol capture listener needs no changes. Add a native API wrapper family in the embedded Cocos runtime script; it discovers QQ platform roots, wraps only matching callable methods, records values with `apiKind: "nativeFriend"`, and delegates unchanged to the original API.

**Tech Stack:** Go test, embedded JavaScript (`resources/wmpf/button.js`), Wails runtime diagnostics, Node-based protocol capture listener.

---

### Task 1: Define the Native API Spy Contract With a Failing Test

**Files:**
- Modify: `runtime_spy_read_test.go:47` (append a runtime-script regression test)
- Test: `runtime_spy_read_test.go`

- [ ] **Step 1: Write the failing regression test**

Append this test after `TestRuntimeButtonExplicitSpyStartStillInstallsSpies`:

```go
func TestRuntimeButtonNativeFriendSpyCapturesQQPlatformCalls(t *testing.T) {
	for _, marker := range []string{
		"function installNativeFriendApiSpies()",
		"function wrapNativeFriendApiMethod(",
		"apiKind: 'nativeFriend'",
		"G.qq",
		"G.GameGlobal && safeReadKey(G.GameGlobal, 'qq')",
		"G.BK",
		"safeReadKey(G.BK, 'QQ')",
		"safeReadKey(safeReadKey(G.GameGlobal, 'BK'), 'QQ')",
		"/friend|add|profile|card|open|url|launch|scheme|contact/i",
	} {
		if !strings.Contains(runtimeButtonScript, marker) {
			t.Fatalf("native friend spy marker missing: %s", marker)
		}
	}

	if body := runtimeButtonFunctionBody(t, "installRuntimeSpies"); !strings.Contains(body, "installNativeFriendApiSpies()") {
		t.Fatal("runtime spy installer must install native friend API spies")
	}
}
```

- [ ] **Step 2: Run the test and observe RED**

Run:

```powershell
go test . -run '^TestRuntimeButtonNativeFriendSpyCapturesQQPlatformCalls$' -count=1
```

Expected: FAIL with `native friend spy marker missing: function installNativeFriendApiSpies()`. Do not edit the production script until this expected failure is observed.

### Task 2: Add Non-Invasive QQ Native API Wrappers

**Files:**
- Modify: `resources/wmpf/button.js:8396` (new native wrappers and installer)
- Modify: `resources/wmpf/button.js:8482` (install native wrappers on initial and repeated spy installation)
- Test: `runtime_spy_read_test.go:47-65`

- [ ] **Step 1: Add native callback and method wrappers**

Insert after `wrapShareApiMethod` and before `installShareApiSpies`:

```js
  function wrapNativeFriendCallbackOption(options, key, platformName, methodName) {
    if (!options || typeof options !== 'object') return;
    const original = safeReadKey(options, key);
    if (typeof original !== 'function' || original.__qqFarmNativeFriendSpyWrapped) return;
    const wrapped = function () {
      const args = Array.prototype.slice.call(arguments);
      pushShareApiEvent({
        apiKind: 'nativeFriend', action: 'callback', platform: platformName,
        method: methodName, callback: key,
        args: args.slice(0, 4).map(function (arg) { return summarizeSpyValue(arg, 1); }),
      });
      return original.apply(this, arguments);
    };
    wrapped.__qqFarmNativeFriendSpyWrapped = true;
    wrapped.__qqFarmNativeFriendSpyOriginal = original;
    options[key] = wrapped;
  }

  function wrapNativeFriendApiMethod(host, platformName, methodName) {
    if (!host || (typeof host !== 'object' && typeof host !== 'function')) return false;
    const original = safeReadKey(host, methodName);
    if (typeof original !== 'function' || original.__qqFarmNativeFriendSpyWrapped) return false;
    const wrapped = function () {
      const args = Array.prototype.slice.call(arguments);
      args.forEach(function (arg) {
        if (!arg || typeof arg !== 'object') return;
        wrapNativeFriendCallbackOption(arg, 'success', platformName, methodName);
        wrapNativeFriendCallbackOption(arg, 'fail', platformName, methodName);
        wrapNativeFriendCallbackOption(arg, 'complete', platformName, methodName);
      });
      pushShareApiEvent({
        apiKind: 'nativeFriend', action: 'call', platform: platformName,
        method: methodName,
        args: args.slice(0, 4).map(function (arg) { return summarizeSpyValue(arg, 1); }),
      });
      return original.apply(this, arguments);
    };
    wrapped.__qqFarmNativeFriendSpyWrapped = true;
    wrapped.__qqFarmNativeFriendSpyOriginal = original;
    try { host[methodName] = wrapped; } catch (_) { return false; }
    return safeReadKey(host, methodName) === wrapped;
  }
```

- [ ] **Step 2: Add root discovery and a bounded name filter**

Insert after the wrappers from Step 1:

```js
  function getNativeFriendApiMethodNames(root) {
    const names = [];
    const seen = {};
    const addName = function (name) {
      const text = String(name || '');
      if (!text || seen[text]) return;
      seen[text] = true;
      if (/friend|add|profile|card|open|url|launch|scheme|contact/i.test(text)) names.push(text);
    };
    let current = root;
    for (let depth = 0; current && depth < 3; depth += 1) {
      safeCall(function () { Object.getOwnPropertyNames(current).forEach(addName); }, null);
      current = safeCall(function () { return Object.getPrototypeOf(current); }, null);
    }
    return names;
  }

  function installNativeFriendApiSpies() {
    const gameGlobal = safeReadKey(G, 'GameGlobal');
    const gameGlobalBK = gameGlobal && safeReadKey(gameGlobal, 'BK');
    const roots = [
      { platform: 'qq', value: G.qq },
      { platform: 'GameGlobal.qq', value: G.GameGlobal && safeReadKey(G.GameGlobal, 'qq') },
      { platform: 'BK', value: G.BK },
      { platform: 'GameGlobal.BK', value: gameGlobalBK },
      { platform: 'BK.QQ', value: G.BK && safeReadKey(G.BK, 'QQ') },
      { platform: 'GameGlobal.BK.QQ', value: safeReadKey(safeReadKey(G.GameGlobal, 'BK'), 'QQ') },
    ];
    let installedCount = 0;
    roots.forEach(function (root) {
      if (!root.value || (typeof root.value !== 'object' && typeof root.value !== 'function')) return;
      getNativeFriendApiMethodNames(root.value).forEach(function (methodName) {
        if (wrapNativeFriendApiMethod(root.value, root.platform, methodName)) installedCount += 1;
      });
    });
    return installedCount;
  }
```

- [ ] **Step 3: Install native wrappers with the existing spies**

In `installRuntimeSpies`, put `installNativeFriendApiSpies();` before every `installShareApiSpies();` call. The repeated-install branch must be:

```js
    if (runtimeSpyState.installed) {
      installRuntimeSendSpies();
      installNativeFriendApiSpies();
      installShareApiSpies();
      return runtimeSpyState;
    }
```

The initial branch begins with `installNativeFriendApiSpies();` followed by `installShareApiSpies();`, and the later refresh call retains the same ordering after `installRuntimeSendSpies();`.

- [ ] **Step 4: Run the focused regression tests and observe GREEN**

Run:

```powershell
go test . -run '^TestRuntimeButton' -count=1
```

Expected: `ok Farm_Go` with the new contract test and the existing read-only installer tests passing.

- [ ] **Step 5: Commit only the implementation and test**

Run:

```powershell
git add -- resources/wmpf/button.js runtime_spy_read_test.go
git commit -m "feat: capture native QQ add-friend APIs"
```

Do not stage `resources/gameConfig.bundle.zip`.

### Task 3: Validate the Captured Runtime Event

**Files:**
- Create: `data/debug-captures/friend-farm-native-add-friend-<timestamp>-raw.json` (generated)
- Create: `data/debug-captures/friend-farm-native-add-friend-<timestamp>-events.ndjson` (generated)
- Create: `data/debug-captures/friend-farm-native-add-friend-<timestamp>-extracted.json` (generated)

- [ ] **Step 1: Arm a clean capture after the Wails development runtime reloads**

Run:

```powershell
node C:/Users/奥森/.codex/skills/farm-protocol-capture/scripts/capture-runtime-protocol.cjs --label friend-farm-native-add-friend --keyword "nativeFriend|addFriend|Friend|Profile|Card|mqqapi|Open|Url|Launch"
```

Wait for both `[ready]` lines.

- [ ] **Step 2: Perform exactly one manual action**

Open the shared friend's farm until the stranger prompt appears, then click `添加QQ好友` once. Do not click other controls in the capture window.

- [ ] **Step 3: Read the native event without constructing data that was not captured**

Run:

```powershell
$latest = Get-ChildItem 'data/debug-captures' -Filter 'friend-farm-native-add-friend-*-raw.json' |
  Sort-Object LastWriteTime -Descending |
  Select-Object -First 1
$capture = Get-Content -Raw $latest.FullName | ConvertFrom-Json
$capture.events |
  Where-Object { $_.kind -eq 'shareApi' -and $_.event.apiKind -eq 'nativeFriend' } |
  ConvertTo-Json -Depth 20
```

Expected: one or more native API records with `platform`, `method`, and `args`. Report an URL or `mqqapi` only when it is present in those captured values; an empty result means the QQ action remains below the JavaScript API boundary.

- [ ] **Step 4: Run the complete Go suite with enough time to finish**

Run:

```powershell
go test ./...
```

Expected: every package exits successfully. The initial baseline attempt exceeded 64 seconds without returning a test failure, so allow this final command to complete rather than treating the shell timeout as a failed test.
