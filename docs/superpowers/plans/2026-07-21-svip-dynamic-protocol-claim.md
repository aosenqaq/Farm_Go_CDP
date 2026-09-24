# SVIP Dynamic Protocol Claim Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the fixed SVIP reward IDs with a pure-protocol status query that discovers the current claim IDs before claiming.

**Architecture:** `claimSvipDailyGift` first calls a focused `requestSvipRewardsStatus` helper with an empty protobuf request. A strict parser reads only the field path proven by the live callback capture, and a general uint32 varint encoder builds the packed repeated field for `ClaimQQVipRewards`; ambiguous or failed status responses never trigger a claim.

**Tech Stack:** JavaScript, Node.js `vm` integration tests, QQ mini-game `netWebSocket.sendMsg`, protobuf wire encoding, Go automation regression tests.

**Execution Status:** Completed on 2026-07-21. The live response codec is `GetQQVipRewardsStatusReply`; the frozen extraction paths are `active_configs[].id`, `active_configs[].is_enable`, and `claimed_today`. Live evidence is saved in `data/debug-captures/svip-dynamic-live-2026-07-21T02-00-31-187Z.json`.

---

### Task 1: Capture the real status callback without UI

**Files:**
- Create: `scripts/test-svip-dynamic-protocol.js`
- Modify: `resources/wmpf/button.js:20363-20366,20903-20961,26348`
- Evidence: `data/debug-captures/svip-status-<timestamp>.json`

- [x] **Step 1: Write the failing status-query test**

Create a VM test that loads the real `button.js`, provides a fake `netWebSocket`, calls `gameCtl.requestSvipRewardsStatus({ silent: true, waitMs: 20 })`, and asserts the only send is:

```js
assert.deepStrictEqual(calls, [{
  bytes: [],
  methodName: "GetQQVipRewardsStatus",
  serviceName: "gamepb.qqvippb.QQVipService",
}]);
assert.strictEqual(result.ok, true);
assert.strictEqual(result.callbackCalled, true);
```

- [x] **Step 2: Run the test and verify RED**

Run: `node scripts/test-svip-dynamic-protocol.js`

Expected: FAIL because `gameCtl.requestSvipRewardsStatus` is not a function.

- [x] **Step 3: Implement the minimal status-query helper**

Add constants and a helper that sends an empty request, captures callback arguments internally, returns a bounded summary, extracts protocol failure text, and treats timeout as failure:

```js
const SVIP_REWARDS_STATUS_METHOD_NAME = 'GetQQVipRewardsStatus';
const SVIP_REWARDS_STATUS_SERVICE_NAME = 'gamepb.qqvippb.QQVipService';

async function requestSvipRewardsStatus(opts) {
  opts = opts || {};
  const net = getNetWebSocket();
  if (!net || typeof net.sendMsg !== 'function') throw new Error('netWebSocket.sendMsg not found');
  const waitMs = Math.max(0, Number(opts.waitMs == null ? 1500 : opts.waitMs) || 0);
  const pollMs = Math.max(30, Number(opts.pollMs == null ? 80 : opts.pollMs) || 80);
  let callbackRaw = null;
  let callbackCalled = false;
  let callbackError = null;
  const callback = function () {
    callbackCalled = true;
    callbackRaw = Array.prototype.slice.call(arguments);
  };
  try {
    net.sendMsg(Uint8Array.from([]), SVIP_REWARDS_STATUS_METHOD_NAME, callback, SVIP_REWARDS_STATUS_SERVICE_NAME);
  } catch (error) {
    callbackError = error && error.message ? error.message : String(error || 'sendMsg failed');
  }
  const deadlineAt = Date.now() + waitMs;
  while (!callbackCalled && !callbackError && Date.now() < deadlineAt) {
    await wait(Math.min(pollMs, Math.max(0, deadlineAt - Date.now())));
  }
  const failureReason = findProtocolPayloadFailureReason(callbackRaw);
  const ok = !callbackError && callbackCalled && !failureReason;
  const payload = {
    ok: ok,
    success: ok,
    methodName: SVIP_REWARDS_STATUS_METHOD_NAME,
    serviceName: SVIP_REWARDS_STATUS_SERVICE_NAME,
    callbackCalled: callbackCalled,
    callbackPayload: callbackRaw ? callbackRaw.map(function (item) { return summarizeSpyValue(item, 4); }) : null,
    callbackError: callbackError,
    reason: ok ? null : (callbackError || failureReason || 'svip_rewards_status_timeout')
  };
  Object.defineProperty(payload, 'callbackRaw', { value: callbackRaw, enumerable: false });
  return opts.silent ? payload : out(payload);
}
```

Export `requestSvipRewardsStatus` on `gameCtl` so the existing Wails diagnostic RPC can invoke it.

- [x] **Step 4: Run the focused test and verify GREEN**

Run: `node scripts/test-svip-dynamic-protocol.js`

Expected: PASS for empty request, callback capture, query failure, and query timeout cases.

- [x] **Step 5: Install the updated QQ debug bundle and capture the callback**

Patch/reload the connected QQ mini-game using the repository's QQ patch workflow, then invoke only:

```json
{
  "name": "main.App.RunDiagnostic",
  "args": ["gameCtl.requestSvipRewardsStatus", {"args": [{"silent": true, "waitMs": 3000}]}],
  "callbackID": "svip-status-<timestamp>"
}
```

Save the returned `callbackPayload` to `data/debug-captures/svip-status-<timestamp>.json`. Confirm runtime spies contain `GetQQVipRewardsStatus` and do not contain `ClaimQQVipRewards`, UI click events, or `QQVIPGiftUI` open/close events in this window.

- [x] **Step 6: Freeze the verified response contract before parser work**

Record the exact callback argument index, envelope keys, reward collection path, ID key, and claimed-state key found in Step 5. Do not continue if the capture contains only a summary without the necessary nested keys or if two fields could plausibly be the claim IDs; improve the bounded callback serializer and repeat Step 5 instead.

### Task 2: Strict extraction and protobuf encoding

**Files:**
- Modify: `scripts/test-svip-dynamic-protocol.js`
- Modify: `resources/wmpf/button.js:20363-20366,20903-20961`

- [x] **Step 1: Add failing tests based on the frozen callback fixture**

Use the exact captured shape as the test fixture. Assert that `claimSvipDailyGift`:

```js
assert.deepStrictEqual(claimCall.bytes, [0x0a, 0x03, 0x01, 0x04, 0x03]);
assert.deepStrictEqual(Array.from(result.claimIds), [1, 4, 3]);
assert.deepStrictEqual(calls.map((call) => call.methodName), [
  "GetQQVipRewardsStatus",
  "ClaimQQVipRewards",
]);
```

Add separate fixtures at the same verified field path for duplicate IDs, an empty list, explicit already-claimed state, missing field, conflicting supported aliases if the capture proves aliases exist, non-integer/zero/negative IDs, and values above `0xffffffff`.

- [x] **Step 2: Add failing encoder boundary tests**

Instrument the real source before `G.gameCtl = {` to expose `encodeSvipClaimRequest` only inside the VM test. Assert:

```js
assert.deepStrictEqual(Array.from(encode([1, 4, 3])), [0x0a, 0x03, 0x01, 0x04, 0x03]);
assert.deepStrictEqual(Array.from(encode([128, 300])), [0x0a, 0x04, 0x80, 0x01, 0xac, 0x02]);
assert.strictEqual(encode(Array.from({ length: 128 }, (_, i) => i + 1))[1], 0x81);
assert.strictEqual(encode(Array.from({ length: 128 }, (_, i) => i + 1))[2], 0x01);
```

- [x] **Step 3: Run the tests and verify RED**

Run: `node scripts/test-svip-dynamic-protocol.js`

Expected: FAIL because dynamic extraction/encoding is not yet used and the production encoder is absent.

- [x] **Step 4: Implement uint32 varint and packed repeated encoding**

Implement without assuming one-byte IDs or payload lengths:

```js
function encodeSvipUint32Varint(value) {
  const normalized = Number(value);
  if (!Number.isInteger(normalized) || normalized <= 0 || normalized > 0xffffffff) {
    throw new Error('invalid_svip_claim_id');
  }
  let remaining = normalized;
  const bytes = [];
  while (remaining >= 0x80) {
    bytes.push((remaining % 0x80) | 0x80);
    remaining = Math.floor(remaining / 0x80);
  }
  bytes.push(remaining);
  return bytes;
}

function encodeSvipClaimRequest(claimIds) {
  const packed = [];
  claimIds.forEach(function (claimId) {
    packed.push.apply(packed, encodeSvipUint32Varint(claimId));
  });
  return [0x0a].concat(encodeProtocolUnsignedVarint(packed.length), packed);
}
```

Reuse the existing unsigned-varint helper if its verified range covers the packed length; otherwise add the equivalent local length encoder next to the SVIP helper.

- [x] **Step 5: Implement strict extraction from only the frozen paths**

Return `{ ok, claimIds, alreadyClaimed, reason }`. Deduplicate with a `Set` while preserving service order. Empty valid collections return `reason: 'no_claimable_rewards'`; explicit claimed state returns `reason: 'already_claimed'`; missing, conflicting, or invalid fields return `ok: false` and never guess IDs from unrelated numbers.

- [x] **Step 6: Run the focused test and verify GREEN**

Run: `node scripts/test-svip-dynamic-protocol.js`

Expected: PASS for the captured fixture, order, deduplication, empty/already-claimed skips, invalid data, query failures/timeouts, and encoder boundaries.

### Task 3: Replace fixed-ID claim flow

**Files:**
- Modify: `resources/wmpf/button.js:20903-20961`
- Test: `scripts/test-svip-dynamic-protocol.js`

- [x] **Step 1: Add failing orchestration assertions**

Assert query failure, timeout, malformed response, and encoder failure produce `ok: false` and exactly one status call. Assert already-claimed and valid empty results produce `{ ok: true, skipped: true }`, with reasons `already_claimed` and `no_claimable_rewards`, and no claim call.

- [x] **Step 2: Run the focused test and verify RED**

Run: `node scripts/test-svip-dynamic-protocol.js`

Expected: FAIL because `claimSvipDailyGift` still sends the fixed request directly.

- [x] **Step 3: Implement query-extract-encode-claim orchestration**

Remove `SVIP_DAILY_GIFT_REQUEST_BYTES`. Have `claimSvipDailyGift` await `requestSvipRewardsStatus`, parse its non-enumerable raw callback, return safe skip/failure results before dispatch when appropriate, encode the returned IDs, then retain the existing claim callback, `ITEM_UPDATE`, send-event, failure-reason, and already-claimed success behavior.

The returned payload must include:

```js
{
  claimIds: claimIds,
  statusMethodName: SVIP_REWARDS_STATUS_METHOD_NAME,
  statusCallbackCalled: statusResult.callbackCalled,
  requestBytes: requestBytes,
  requestHex: bytesToHex(requestBytes)
}
```

Do not expose the complete raw status response in the normal result or logs.

- [x] **Step 4: Run the focused test and verify GREEN**

Run: `node scripts/test-svip-dynamic-protocol.js`

Expected: PASS with every scenario and no hard-coded claim ID list in `button.js`.

### Task 4: Regression and live pure-protocol verification

**Files:**
- Verify: `internal/farm/automation/runtime_rewards.go`
- Verify: `internal/farm/automation/runtime_rewards_test.go`
- Verify: `app_test.go`
- Evidence: `data/debug-captures/svip-dynamic-live-<timestamp>.json`

- [x] **Step 1: Verify the Go facade contract**

Run: `go test ./internal/farm/automation -run "Svip|SVIP|DailyReward" -count=1`

Expected: PASS and the facade still calls `gameCtl.claimSvipDailyGift` with the same arguments.

- [x] **Step 2: Run protocol-adjacent JavaScript regressions**

Run:

```powershell
node scripts/test-svip-dynamic-protocol.js
node scripts/test-mail-reward-protocol.js
node scripts/test-protocol-failure-text.js
```

Expected: all commands exit 0.

- [x] **Step 3: Run the full Go suite**

Run: `go test ./... -count=1`

Expected: PASS with zero failing packages.

- [x] **Step 4: Install the verified bundle and invoke the real claim method**

After patch/reload, reset runtime spies and invoke only `gameCtl.claimSvipDailyGift({ silent: true, waitMs: 3000 })` through Wails RPC. Save the result and spy snapshot to `data/debug-captures/svip-dynamic-live-<timestamp>.json`.

- [x] **Step 5: Verify the live acceptance criteria**

The evidence must show `GetQQVipRewardsStatus` before any `ClaimQQVipRewards`. If claimable IDs are present, decode the claim bytes and confirm they match the same status response. If already claimed or empty, confirm the result is a successful skip and no claim request was sent. Confirm no click event or `QQVIPGiftUI` open/close event occurred during the test.

- [x] **Step 6: Review the final diff**

Run: `git diff --check` and `git diff -- resources/wmpf/button.js scripts/test-svip-dynamic-protocol.js docs/superpowers/plans/2026-07-21-svip-dynamic-protocol-claim.md`.

Expected: no whitespace errors, no unrelated changes, no fixed `[1, 2]` or captured `[1, 4, 3]` claim fallback in production code.
