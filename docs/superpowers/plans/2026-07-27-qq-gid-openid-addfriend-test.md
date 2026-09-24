# QQ GID-to-OpenID Add-Friend Test Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add one diagnostic operation that queries an authorized farm GID, validates `basic.open_id`, and invokes QQ's native add-friend API once.

**Architecture:** Reuse `visitFriendFarmByProtocol` and its decoded reply. The new operation accepts a GID and verification message, but never an OpenID; it validates the response before resolving `qq.addFriendByOpenId`.

**Tech Stack:** Embedded JavaScript, Node VM tests, Wails diagnostics, QQ runtime Spy capture.

---

### Task 1: Define the Guarded Invocation Contract

**Files:**
- Modify: `scripts/test-friend-pure-protocol.js:73`
- Test: `scripts/test-friend-pure-protocol.js`

- [x] **Step 1: Extend the VM helper with an optional QQ root**

```js
async function loadGameCtlWithNet(netWebSocket, qq) {
  const localContext = {
    ArrayBuffer, Date, Map, Promise, Set, TextDecoder, TextEncoder, Uint8Array,
    clearTimeout, decodeURIComponent, setTimeout,
    console: { dir() {}, error() {}, info() {}, log() {}, warn() {} },
  };
  localContext.globalThis = localContext;
  localContext.__testOops = { netWebSocket };
  localContext.qq = qq || null;
  // Preserve the existing cc test stub and vm.runInNewContext call.
}
```

- [x] **Step 2: Add a failing query-then-native invocation test**

Append this inside `main()` before its success log:

```js
const nativeCalls = [];
const qqRoot = {
  addFriendByOpenId(options) {
    nativeCalls.push({ receiver: this, options });
    options.success({ errMsg: "addFriendByOpenId:ok" });
  },
};
const addFriendGameCtl = await loadGameCtlWithNet({
  sendMsg(_bytes, methodName, callback) {
    assert.strictEqual(methodName, "Enter");
    callback({
      basic: {
        gid: 10001,
        open_id: "A1B2C3D4E5F60708192A3B4C5D6E7F80",
        binded_communities: [{ openid: "NOT_THE_QQ_OPENID" }],
      },
      lands: [],
    });
  },
}, qqRoot);
const addFriendResult = await addFriendGameCtl.addFriendByGidDiagnostic({
  hostGid: 10001,
  verifyMsg: "test verification",
  silent: true,
  waitReplyMs: 0,
});
assert.strictEqual(addFriendResult.ok, true);
assert.strictEqual(addFriendResult.invoked, true);
assert.strictEqual(addFriendResult.resolvedFrom, "basic.open_id");
assert.strictEqual(nativeCalls.length, 1);
assert.strictEqual(nativeCalls[0].receiver, qqRoot);
assert.strictEqual(nativeCalls[0].options.openId, "A1B2C3D4E5F60708192A3B4C5D6E7F80");
assert.strictEqual(nativeCalls[0].options.verifyMsg, "test verification");
assert.deepStrictEqual(addFriendResult.callbackEvents.map((item) => item.kind), ["success"]);
```

- [x] **Step 3: Add the failing mismatched-GID case**

```js
let rejectedNativeCalls = 0;
const mismatchGameCtl = await loadGameCtlWithNet({
  sendMsg(_bytes, methodName, callback) {
    assert.strictEqual(methodName, "Enter");
    callback({ basic: { gid: 10002, open_id: "A1B2C3D4E5F60708192A3B4C5D6E7F80" }, lands: [] });
  },
}, { addFriendByOpenId() { rejectedNativeCalls += 1; } });
const mismatchResult = await mismatchGameCtl.addFriendByGidDiagnostic({
  hostGid: 10001,
  verifyMsg: "test verification",
  silent: true,
  waitReplyMs: 0,
});
assert.strictEqual(mismatchResult.ok, false);
assert.strictEqual(mismatchResult.reason, "reply_gid_mismatch");
assert.strictEqual(rejectedNativeCalls, 0);
```

- [x] **Step 4: Add the failing missing-OpenID case**

```js
let missingOpenIdNativeCalls = 0;
const missingOpenIdGameCtl = await loadGameCtlWithNet({
  sendMsg(_bytes, methodName, callback) {
    assert.strictEqual(methodName, "Enter");
    callback({ basic: { gid: 10001 }, lands: [] });
  },
}, { addFriendByOpenId() { missingOpenIdNativeCalls += 1; } });
const missingOpenIdResult = await missingOpenIdGameCtl.addFriendByGidDiagnostic({
  hostGid: 10001,
  verifyMsg: "test verification",
  silent: true,
  waitReplyMs: 0,
});
assert.strictEqual(missingOpenIdResult.ok, false);
assert.strictEqual(missingOpenIdResult.reason, "reply_open_id_missing");
assert.strictEqual(missingOpenIdNativeCalls, 0);
```

- [x] **Step 5: Run the VM test and observe RED**

Run `node scripts/test-friend-pure-protocol.js`.

Expected: non-zero exit with `addFriendByGidDiagnostic is not a function`.

### Task 2: Implement the Single Native Invocation

**Files:**
- Modify: `resources/wmpf/button.js:26592` (new helpers below `visitFriendFarmByProtocol`)
- Modify: `resources/wmpf/button.js:26713` (runtime export)
- Test: `scripts/test-friend-pure-protocol.js`

- [x] **Step 1: Add field and native-method resolvers**

```js
function getVisitBasicOpenId(basic) {
  const value = basic && safeReadKey(basic, 'open_id');
  const openId = typeof value === 'string' ? value.trim() : '';
  return /^[0-9a-f]{32}$/i.test(openId) ? openId : null;
}

function getQQAddFriendByOpenId() {
  const qq = safeReadKey(G, 'qq');
  const method = qq && safeReadKey(qq, 'addFriendByOpenId');
  return typeof method === 'function' ? { receiver: qq, method: method } : null;
}
```

- [x] **Step 2: Add the diagnostic operation**

```js
async function addFriendByGidDiagnostic(opts) {
  opts = opts || {};
  const hostGid = Number(opts.hostGid || opts.gid || opts.friendGid);
  const verifyMsg = typeof opts.verifyMsg === 'string' ? opts.verifyMsg : '';
  const result = {
    ok: false,
    action: 'native_add_friend_by_gid',
    hostGid: Number.isSafeInteger(hostGid) && hostGid > 0 ? hostGid : null,
    invoked: false,
    resolvedFrom: null,
    callbackEvents: [],
    reason: null,
  };
  if (result.hostGid == null) {
    result.reason = 'invalid_host_gid';
    return opts.silent ? result : out(result);
  }
  if (!verifyMsg) {
    result.reason = 'verify_message_required';
    return opts.silent ? result : out(result);
  }
  const visit = await visitFriendFarmByProtocol({
    hostGid: result.hostGid,
    reason: 5,
    silent: true,
    waitReplyMs: opts.waitReplyMs,
  });
  const basic = visit && visit.basic;
  const replyGid = Number(basic && safeReadKey(basic, 'gid'));
  const openId = getVisitBasicOpenId(basic);
  if (!visit || !visit.ok) {
    result.reason = 'visit_query_failed';
    return opts.silent ? result : out(result);
  }
  if (!Number.isSafeInteger(replyGid) || replyGid !== result.hostGid) {
    result.reason = 'reply_gid_mismatch';
    return opts.silent ? result : out(result);
  }
  if (!openId) {
    result.reason = 'reply_open_id_missing';
    return opts.silent ? result : out(result);
  }
  const nativeApi = getQQAddFriendByOpenId();
  if (!nativeApi) {
    result.reason = 'qq_add_friend_api_unavailable';
    return opts.silent ? result : out(result);
  }
  const recordCallback = function (kind, payload) {
    result.callbackEvents.push({ kind: kind, payload: summarizeNativeFriendArgument(payload) });
  };
  try {
    nativeApi.method.call(nativeApi.receiver, {
      openId: openId,
      verifyMsg: verifyMsg,
      success: function (payload) { recordCallback('success', payload); },
      fail: function (payload) { recordCallback('fail', payload); },
    });
    result.ok = true;
    result.invoked = true;
    result.resolvedFrom = 'basic.open_id';
  } catch (error) {
    result.reason = error && error.message ? error.message : String(error || 'qq_add_friend_call_failed');
  }
  return opts.silent ? result : out(result);
}
```

- [x] **Step 3: Export the diagnostic operation**

Add `addFriendByGidDiagnostic,` to the existing `G.gameCtl` object. Do not
export a generic raw-OpenID operation.

- [x] **Step 4: Run the VM test and observe GREEN**

Run `node scripts/test-friend-pure-protocol.js`.

Expected: `[friend-pure-protocol] byte builders pass`.

- [x] **Step 5: Run focused validation**

Run `node --check resources/wmpf/button.js` and
`go test . -run '^TestRuntimeButton' -count=1`.

- [x] **Step 6: Commit the implementation**

Run `git add -- resources/wmpf/button.js scripts/test-friend-pure-protocol.js runtime_spy_read_test.go`, then run `git commit -m "feat: add guarded gid add-friend diagnostic"`.

Do not stage `resources/gameConfig.bundle.zip`, captures, or unrelated plan files.

### Task 3: Verify the Authorized Live Experiment

**Files:**
- Create: `data/debug-captures/gid-openid-addfriend-<timestamp>-raw.json`
- Create: `data/debug-captures/gid-openid-addfriend-<timestamp>-extracted.json`

- [x] **Step 1: Restart the user-controlled Wails development session**

Ask the user to restart Wails after the embedded-script change. Do not terminate
the session from the agent.

- [x] **Step 2: Arm a clean capture**

Call `gameCtl.startRuntimeSpies({ silent: true, includeFrames: false })`, then
`gameCtl.resetRuntimeSpyEvents({ silent: true, keepProfiles: true })` over the
Wails IPC bridge.

- [x] **Step 3: Invoke exactly one diagnostic operation**

```json
{
  "hostGid": 1005494359,
  "verifyMsg": "\u201c145\u7ea7\uff0c\u6709\u72d7\u968f\u4fbf\u5077\u201c",
  "silent": true,
  "waitReplyMs": 5000
}
```

Do not repeat the call or invoke `addFriendByOpenId` directly when the result
is not `ok: true`.

- [x] **Step 4: Persist and inspect the capture**

The extracted result must show:

```text
VisitService.Enter reason=5 -> basic.gid == 1005494359
basic.open_id -> qq.addFriendByOpenId openId
one nativeFriend call
zero clickEvents
```

Report callback evidence only when captured; a missing callback is not a
successful friend relationship.

- [x] **Step 5: Run final regression checks**

Run `node scripts/test-friend-pure-protocol.js`, then `go test ./... -count=1`,
then `git diff --check`.

Expected: all commands exit `0` before reporting the experiment result.
