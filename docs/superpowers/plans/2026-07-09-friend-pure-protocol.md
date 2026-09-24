# Friend Pure Protocol Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move Farm_Go's existing friend steal, help, mischief, and dog-guard scan flows onto direct QQ Farm protobuf RPC calls exposed through the existing WMPF runtime bridge.

**Architecture:** Keep Go/Wails as the scheduler, persistence, and UI layer. Add a focused protocol surface in `resources/wmpf/button.js` that calls `netWebSocket.sendMsg` for `VisitService` and `PlantService`, returns normalized friend farm summaries to Go, and leaves the existing QQ/WMPF runtime connection model intact. Do not implement a new Go-side encrypted game gateway in this plan.

**Tech Stack:** Go 1.25, Wails v2, React/Vite, Cocos/WMPF runtime JavaScript, protobuf wire-format byte builders, existing `gameCtl` runtime caller, SQLite-backed social cache.

---

## Scope Check

This plan covers one subsystem: friend farm protocol integration. It intentionally does not replace the whole QQ login, heartbeat, encryption, or `GateMessage` transport in Go. The current WMPF runtime already has `oops.netWebSocket.sendMsg`, so the shortest stable path is to expose direct protocol methods through `gameCtl` and let Go call those methods.

The plan must preserve existing user-facing controls:

- automatic friend steal;
- automatic friend help;
- automatic friend mischief;
- manual social row actions for steal/help/mischief;
- dog-guard scan and cache;
- local blacklist/whitelist and protected friend behavior.

## File Structure

- Modify `resources/wmpf/button.js`: add direct protocol request builders, dispatch wrappers, friend farm visit analyzer, dog info parser, and exported `gameCtl` methods.
- Create `scripts/test-friend-pure-protocol.js`: VM-load `button.js`, expose private helpers, and test byte builders plus mock `netWebSocket.sendMsg` calls.
- Modify `internal/farm/automation/runtime_friend.go`: switch automatic friend tasks from scene/button methods to the new protocol methods.
- Modify `internal/farm/automation/runtime_helpers.go`: consume protocol summary maps such as `workLandIds.collect`, `workLandIds.farming`, `workLandIds.bug`, and `workLandIds.grass`.
- Modify `internal/farm/automation/runtime_friend_test.go`: update expected method names and arguments.
- Modify `internal/farm/social/actions.go`: switch manual friend actions to the same protocol surface.
- Modify `internal/farm/social/actions_test.go`: update expected method names and argument shapes.
- Modify `internal/farm/social/dog_guard.go`: remove runtime spy dependency and use protocol visit dog info.
- Modify `internal/farm/social/dog_guard_test.go`: assert the scanner uses protocol visit calls and no longer calls spy methods.
- Do not modify `internal/farm/automation/scheduler.go` in this plan: keep friend tasks on `LaneMainline` because they still enter friend farms through the live runtime bridge.

---

### Task 1: Protocol Byte Builder Tests

**Files:**
- Create: `scripts/test-friend-pure-protocol.js`
- Modify: `package.json` only if a new npm script is desired; otherwise run with `node scripts/test-friend-pure-protocol.js`
- Test target: private helper functions inside `resources/wmpf/button.js`

- [ ] **Step 1: Write the failing VM test**

Create `scripts/test-friend-pure-protocol.js`:

```js
const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const root = path.resolve(__dirname, "..");
const buttonPath = path.join(root, "resources", "wmpf", "button.js");
let source = fs.readFileSync(buttonPath, "utf8");

source = source.replace(
  "  G.gameCtl = {",
  [
    "  G.__testEncodeFarmVarint = encodeFarmVarint;",
    "  G.__testBuildVisitEnterRequestBytes = buildVisitEnterRequestBytes;",
    "  G.__testBuildHarvestRequestBytes = buildHarvestRequestBytes;",
    "  G.__testBuildFarmingRequestBytes = buildFarmingRequestBytes;",
    "  G.__testBuildCheckCanOperateRequestBytes = buildCheckCanOperateRequestBytes;",
    "  G.__testParseBriefDogInfoBytes = parseBriefDogInfoBytesProtocol;",
    "  G.gameCtl = {",
  ].join("\\n")
);

const context = {
  ArrayBuffer,
  Date,
  Map,
  Promise,
  Set,
  TextDecoder,
  TextEncoder,
  Uint8Array,
  clearTimeout,
  console: { dir() {}, error() {}, info() {}, log() {}, warn() {} },
  decodeURIComponent,
  setTimeout,
};
context.globalThis = context;
context.cc = {
  Button: function Button() {},
  Component: function Component() {},
  Node: function Node() {},
  director: { getScene() { return null; } },
  find() { return null; },
  game: { canvas: {} },
};

vm.runInNewContext(source, context, { filename: buttonPath, timeout: 5000 });

assert.deepStrictEqual(context.__testEncodeFarmVarint(10001), [145, 78]);
assert.deepStrictEqual(context.__testBuildVisitEnterRequestBytes(10001, 2), [8, 145, 78, 16, 2]);
assert.deepStrictEqual(context.__testBuildCheckCanOperateRequestBytes(10001, 10004), [8, 145, 78, 16, 148, 78]);
assert.deepStrictEqual(context.__testBuildHarvestRequestBytes(10001, [3, 9], false), [10, 2, 3, 9, 16, 145, 78, 24, 0]);
assert.deepStrictEqual(context.__testBuildFarmingRequestBytes(10001, [3, 9], 0), [10, 2, 3, 9, 16, 145, 78, 24, 0]);

const dogInfo = context.__testParseBriefDogInfoBytes([8, 165, 191, 5, 16, 1, 24, 60]);
assert.strictEqual(dogInfo.dogId, 90021);
assert.strictEqual(dogInfo.dogName, "护主犬");

console.log("[friend-pure-protocol] byte builders pass");
```

- [ ] **Step 2: Run the test to verify RED**

Run:

```powershell
node scripts\test-friend-pure-protocol.js
```

Expected: FAIL with `ReferenceError: encodeFarmVarint is not defined` or equivalent missing helper names.

- [ ] **Step 3: Commit the failing test**

```powershell
git add scripts/test-friend-pure-protocol.js
git commit -m "test: cover friend protocol byte builders"
```

---

### Task 2: Add Direct Friend Protocol Surface In `button.js`

**Files:**
- Modify: `resources/wmpf/button.js`
- Test: `scripts/test-friend-pure-protocol.js`

- [ ] **Step 1: Add pure helper functions near existing protocol helpers**

Add these helpers near `encodeFriendMischiefVarint` so protocol byte building has one local style:

```js
  const FRIEND_PROTOCOL_DOG_NAMES = {
    90001: '田园犬',
    90002: '牧羊犬',
    90003: '斑点狗',
    90011: '柯基',
    90021: '护主犬',
  };

  function encodeFarmVarint(value) {
    let n = Number(value);
    n = Number.isFinite(n) && n > 0 ? Math.floor(n) : 0;
    const bytes = [];
    while (n >= 0x80) {
      bytes.push((n & 0x7f) | 0x80);
      n = Math.floor(n / 0x80);
    }
    bytes.push(n);
    return bytes;
  }

  function buildPackedFarmInts(values) {
    const out = [];
    normalizeLandIds(values).forEach(function (value) {
      const bytes = encodeFarmVarint(value);
      for (let i = 0; i < bytes.length; i += 1) out.push(bytes[i]);
    });
    return out;
  }

  function buildVisitEnterRequestBytes(hostGid, reason) {
    const gid = Number(hostGid);
    if (!Number.isFinite(gid) || gid <= 0) throw new Error('hostGid required');
    return [8].concat(encodeFarmVarint(gid)).concat([16]).concat(encodeFarmVarint(reason == null ? 2 : reason));
  }

  function buildVisitLeaveRequestBytes(hostGid) {
    const gid = Number(hostGid);
    if (!Number.isFinite(gid) || gid <= 0) throw new Error('hostGid required');
    return [8].concat(encodeFarmVarint(gid));
  }

  function buildCheckCanOperateRequestBytes(hostGid, operationId) {
    const gid = Number(hostGid);
    const op = Number(operationId);
    if (!Number.isFinite(gid) || gid <= 0) throw new Error('hostGid required');
    if (!Number.isFinite(op) || op <= 0) throw new Error('operationId required');
    return [8].concat(encodeFarmVarint(gid)).concat([16]).concat(encodeFarmVarint(op));
  }

  function buildHarvestRequestBytes(hostGid, landIds, isAll) {
    const gid = Number(hostGid);
    const packed = buildPackedFarmInts(landIds);
    if (!Number.isFinite(gid) || gid <= 0) throw new Error('hostGid required');
    if (packed.length === 0) throw new Error('landIds required');
    return [10]
      .concat(encodeFarmVarint(packed.length))
      .concat(packed)
      .concat([16])
      .concat(encodeFarmVarint(gid))
      .concat([24])
      .concat(encodeFarmVarint(isAll === true ? 1 : 0));
  }

  function buildFarmingRequestBytes(hostGid, landIds, source) {
    const gid = Number(hostGid);
    const packed = buildPackedFarmInts(landIds);
    if (!Number.isFinite(gid) || gid <= 0) throw new Error('hostGid required');
    if (packed.length === 0) throw new Error('landIds required');
    return [10]
      .concat(encodeFarmVarint(packed.length))
      .concat(packed)
      .concat([16])
      .concat(encodeFarmVarint(gid))
      .concat([24])
      .concat(encodeFarmVarint(source == null ? 0 : source));
  }
```

- [ ] **Step 2: Add callback dispatch wrapper**

Add this wrapper below the builders:

```js
  async function sendFriendProtocolRequest(serviceName, methodName, requestBytes, opts) {
    opts = opts || {};
    const net = getNetWebSocket();
    if (!net || typeof net.sendMsg !== 'function') throw new Error('netWebSocket.sendMsg not found');
    const waitMs = Math.max(0, Number(opts.waitReplyMs == null ? 1200 : opts.waitReplyMs) || 0);
    let callbackCalled = false;
    let callbackRaw = null;
    let callbackPayload = null;
    let callbackError = null;
    const callback = function () {
      callbackCalled = true;
      callbackRaw = Array.prototype.slice.call(arguments);
      callbackPayload = callbackRaw.map(function (item) {
        return summarizeSpyValue(item, 3);
      });
    };
    try {
      net.sendMsg(Uint8Array.from(requestBytes), methodName, callback, serviceName);
    } catch (error) {
      callbackError = error && error.message ? error.message : String(error || 'sendMsg failed');
      throw error;
    }
    if (waitMs > 0) {
      await waitForProtocolCallback(function () { return callbackCalled || callbackError; }, waitMs);
    }
    const failureText = extractProtocolFailureText(callbackRaw);
    return {
      ok: !callbackError && !failureText,
      serviceName,
      methodName,
      requestBytes,
      callbackCalled,
      callbackPayload,
      callbackError,
      failureText,
      raw: callbackRaw,
    };
  }
```

- [ ] **Step 3: Add exported protocol action methods**

Add these methods before `G.gameCtl = {`:

```js
  async function checkFriendCanOperateByProtocol(opts) {
    opts = opts || {};
    const hostGid = Number(opts.hostGid || opts.gid || opts.friendGid);
    const operationId = Number(opts.operationId || opts.operation_id);
    const bytes = buildCheckCanOperateRequestBytes(hostGid, operationId);
    const dispatch = await sendFriendProtocolRequest('gamepb.plantpb.PlantService', 'CheckCanOperate', bytes, opts);
    return opts.silent ? dispatch : out(dispatch);
  }

  async function friendHarvestLandsByProtocol(opts) {
    opts = opts || {};
    const hostGid = Number(opts.hostGid || opts.gid || opts.friendGid);
    const landIds = normalizeLandIds(opts.landIds || []);
    const bytes = buildHarvestRequestBytes(hostGid, landIds, opts.isAll === true);
    const dispatch = await sendFriendProtocolRequest('gamepb.plantpb.PlantService', 'Harvest', bytes, opts);
    const result = {
      ok: dispatch.ok,
      action: 'friend_harvest_protocol',
      executionSource: 'direct_protocol_dispatch',
      hostGid,
      landIds,
      serviceName: dispatch.serviceName,
      methodName: dispatch.methodName,
      requestBytes: bytes,
      callbackCalled: dispatch.callbackCalled,
      callbackPayload: dispatch.callbackPayload,
      callbackError: dispatch.callbackError,
      failureText: dispatch.failureText,
    };
    return opts.silent ? result : out(result);
  }

  async function friendFarmingByProtocol(opts) {
    opts = opts || {};
    const hostGid = Number(opts.hostGid || opts.gid || opts.friendGid);
    const landIds = normalizeLandIds(opts.landIds || []);
    const bytes = buildFarmingRequestBytes(hostGid, landIds, opts.source == null ? 0 : opts.source);
    const dispatch = await sendFriendProtocolRequest('gamepb.plantpb.PlantService', 'Farming', bytes, opts);
    const result = {
      ok: dispatch.ok,
      action: 'friend_farming_protocol',
      executionSource: 'direct_protocol_dispatch',
      hostGid,
      landIds,
      serviceName: dispatch.serviceName,
      methodName: dispatch.methodName,
      requestBytes: bytes,
      callbackCalled: dispatch.callbackCalled,
      callbackPayload: dispatch.callbackPayload,
      callbackError: dispatch.callbackError,
      failureText: dispatch.failureText,
    };
    return opts.silent ? result : out(result);
  }
```

- [ ] **Step 4: Export the methods in `G.gameCtl`**

Add the new methods to the `G.gameCtl` object:

```js
    checkFriendCanOperateByProtocol,
    friendHarvestLandsByProtocol,
    friendFarmingByProtocol,
```

- [ ] **Step 5: Run byte builder test to verify GREEN**

Run:

```powershell
node scripts\test-friend-pure-protocol.js
```

Expected: PASS with `[friend-pure-protocol] byte builders pass`.

- [ ] **Step 6: Commit**

```powershell
git add resources/wmpf/button.js scripts/test-friend-pure-protocol.js
git commit -m "feat: add friend plant protocol methods"
```

---

### Task 3: Add Protocol Visit Summary And Dog Parser

**Files:**
- Modify: `resources/wmpf/button.js`
- Modify: `scripts/test-friend-pure-protocol.js`

- [ ] **Step 1: Extend the VM test for protocol visit methods**

Append this mock-net test to `scripts/test-friend-pure-protocol.js`:

```js
async function loadGameCtlWithNet(netWebSocket) {
  const oops = { netWebSocket };
  const localContext = {
    ArrayBuffer,
    Date,
    Map,
    Promise,
    Set,
    TextDecoder,
    TextEncoder,
    Uint8Array,
    clearTimeout,
    console: { dir() {}, error() {}, info() {}, log() {}, warn() {} },
    decodeURIComponent,
    setTimeout,
  };
  localContext.globalThis = localContext;
  localContext.cc = {
    Button: function Button() {},
    Component: function Component() {},
    Node: function Node() {},
    director: { getScene() { return null; } },
    find() { return null; },
    game: { canvas: {} },
  };
  localContext.System = { get() { return { oops }; } };
  vm.runInNewContext(source, localContext, { filename: buttonPath, timeout: 5000 });
  return localContext.gameCtl;
}

(async () => {
  const calls = [];
  const gameCtl = await loadGameCtlWithNet({
    sendMsg(bytes, methodName, callback, serviceName) {
      calls.push({ bytes: Array.from(bytes), methodName, serviceName });
      if (methodName === "Enter") {
        callback({
          basic: { gid: 10001, name: "A" },
          brief_dog_info: Uint8Array.from([8, 165, 191, 5, 16, 1, 24, 60]),
          lands: [
            { id: 3, plant: { id: 2001, phases: [{ phase: 6 }], stealable: true } },
            { id: 9, plant: { id: 2002, phases: [{ phase: 4, dry_time: 1 }], dry_num: 1, weed_owners: [], insect_owners: [] } },
          ],
        });
        return;
      }
      callback({});
    },
  });

  const summary = await gameCtl.inspectFriendFarmByProtocol({ hostGid: 10001, silent: true, leaveAfter: true });
  assert.strictEqual(summary.ok, true);
  assert.strictEqual(summary.briefDogInfo.dogId, 90021);
  assert.deepStrictEqual(summary.workLandIds.collect, [3]);
  assert.deepStrictEqual(summary.workLandIds.farming, [9]);
  assert.strictEqual(calls[0].methodName, "Enter");
  assert.strictEqual(calls[0].serviceName, "gamepb.visitpb.VisitService");
  assert.strictEqual(calls[calls.length - 1].methodName, "Leave");
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
```

- [ ] **Step 2: Run the test to verify RED**

Run:

```powershell
node scripts\test-friend-pure-protocol.js
```

Expected: FAIL with `gameCtl.inspectFriendFarmByProtocol is not a function`.

- [ ] **Step 3: Add dog info parser and land analyzer**

Add this code near the protocol helpers:

```js
  function parseProtoVarintsLoose(bytes, limit) {
    const data = Array.from(bytes || []);
    const outArr = [];
    for (let i = 0; i < data.length && outArr.length < (limit || 64); i += 1) {
      let shift = 0;
      let value = 0;
      let pos = i;
      while (pos < data.length && shift <= 35) {
        const b = data[pos];
        value += (b & 0x7f) * Math.pow(2, shift);
        pos += 1;
        if ((b & 0x80) === 0) {
          if (value > 0) outArr.push(value);
          break;
        }
        shift += 7;
      }
    }
    return outArr;
  }

  function parseBriefDogInfoBytesProtocol(bytes) {
    const values = parseProtoVarintsLoose(bytes, 80);
    const dogId = values.find(function (value) {
      return Object.prototype.hasOwnProperty.call(FRIEND_PROTOCOL_DOG_NAMES, value);
    }) || 0;
    return {
      dogId,
      dogName: FRIEND_PROTOCOL_DOG_NAMES[dogId] || '',
      numbers: values.slice(0, 16),
      rawLen: bytes && bytes.length ? bytes.length : 0,
    };
  }

  function normalizeVisitLandId(land) {
    return normalizeLandId(land && (land.id != null ? land.id : land.land_id));
  }

  function getVisitLandPlant(land) {
    return land && (land.plant || land.plant_info || land.plantInfo) || null;
  }

  function visitPlantCurrentPhase(plant) {
    const phases = Array.isArray(plant && plant.phases) ? plant.phases : [];
    if (phases.length === 0) return 0;
    let best = phases[0];
    for (let i = 1; i < phases.length; i += 1) {
      const phase = Number(phases[i] && phases[i].phase) || 0;
      if (phase > (Number(best && best.phase) || 0)) best = phases[i];
    }
    return Number(best && best.phase) || 0;
  }

  function analyzeVisitFriendLands(lands, opts) {
    opts = opts || {};
    const selfGid = Number(opts.selfGid || getSelfGid() || 0);
    const summary = {
      collect: [],
      water: [],
      eraseGrass: [],
      killBug: [],
      farming: [],
      grass: [],
      bug: [],
    };
    (Array.isArray(lands) ? lands : []).forEach(function (land) {
      const landId = normalizeVisitLandId(land);
      const plant = getVisitLandPlant(land);
      if (landId == null || !plant) return;
      const phase = visitPlantCurrentPhase(plant);
      if (phase === 6) {
        if (plant.stealable === true || plant.canSteal === true) summary.collect.push(landId);
        return;
      }
      if (phase === 7) return;
      const dry = Number(plant.dry_num || plant.dryNum || 0) > 0;
      const weedOwners = Array.isArray(plant.weed_owners) ? plant.weed_owners : [];
      const insectOwners = Array.isArray(plant.insect_owners) ? plant.insect_owners : [];
      if (dry) summary.water.push(landId);
      if (weedOwners.length > 0) summary.eraseGrass.push(landId);
      if (insectOwners.length > 0) summary.killBug.push(landId);
      if (dry || weedOwners.length > 0 || insectOwners.length > 0) summary.farming.push(landId);
      const hasSelfWeed = weedOwners.some(function (gid) { return Number(gid) === selfGid; });
      const hasSelfBug = insectOwners.some(function (gid) { return Number(gid) === selfGid; });
      if (weedOwners.length < 2 && !hasSelfWeed) summary.grass.push(landId);
      if (insectOwners.length < 2 && !hasSelfBug) summary.bug.push(landId);
    });
    Object.keys(summary).forEach(function (key) {
      summary[key] = normalizeLandIds(summary[key]);
    });
    return summary;
  }
```

- [ ] **Step 4: Add visit protocol methods**

Add these methods:

```js
  async function visitFriendFarmByProtocol(opts) {
    opts = opts || {};
    const hostGid = Number(opts.hostGid || opts.gid || opts.friendGid);
    const bytes = buildVisitEnterRequestBytes(hostGid, opts.reason == null ? 2 : opts.reason);
    const dispatch = await sendFriendProtocolRequest('gamepb.visitpb.VisitService', 'Enter', bytes, opts);
    const reply = dispatch.raw && dispatch.raw.length > 0 ? dispatch.raw[0] : null;
    const dogBytes = reply && (reply.brief_dog_info || reply.briefDogInfo);
    const dogInfo = parseBriefDogInfoBytesProtocol(dogBytes);
    return {
      ok: dispatch.ok || !!reply,
      action: 'visit_friend_protocol',
      hostGid,
      serviceName: dispatch.serviceName,
      methodName: dispatch.methodName,
      requestBytes: bytes,
      callbackCalled: dispatch.callbackCalled,
      callbackPayload: dispatch.callbackPayload,
      callbackError: dispatch.callbackError,
      failureText: dispatch.failureText,
      basic: reply && (reply.basic || reply.user) || null,
      lands: reply && Array.isArray(reply.lands) ? reply.lands : [],
      briefDogInfo: dogInfo,
    };
  }

  async function leaveFriendFarmByProtocol(opts) {
    opts = opts || {};
    const hostGid = Number(opts.hostGid || opts.gid || opts.friendGid);
    const bytes = buildVisitLeaveRequestBytes(hostGid);
    const dispatch = await sendFriendProtocolRequest('gamepb.visitpb.VisitService', 'Leave', bytes, opts);
    return opts.silent ? dispatch : out(dispatch);
  }

  async function inspectFriendFarmByProtocol(opts) {
    opts = opts || {};
    const hostGid = Number(opts.hostGid || opts.gid || opts.friendGid);
    const visit = await visitFriendFarmByProtocol({ ...opts, hostGid, silent: true });
    const workLandIds = analyzeVisitFriendLands(visit.lands, opts);
    let leave = null;
    if (opts.leaveAfter === true) {
      leave = await leaveFriendFarmByProtocol({ hostGid, silent: true, waitReplyMs: opts.leaveWaitReplyMs || 400 });
    }
    const result = {
      ok: visit.ok,
      action: 'inspect_friend_farm_protocol',
      hostGid,
      friend: visit.basic,
      lands: opts.includeLands === false ? [] : visit.lands,
      workLandIds,
      briefDogInfo: visit.briefDogInfo,
      visit,
      leave,
    };
    return opts.silent ? result : out(result);
  }
```

- [ ] **Step 5: Export visit methods**

Add these to `G.gameCtl`:

```js
    visitFriendFarmByProtocol,
    leaveFriendFarmByProtocol,
    inspectFriendFarmByProtocol,
```

- [ ] **Step 6: Run protocol tests**

Run:

```powershell
node scripts\test-friend-pure-protocol.js
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add resources/wmpf/button.js scripts/test-friend-pure-protocol.js
git commit -m "feat: inspect friend farms by protocol"
```

---

### Task 4: Switch Automatic Friend Tasks To Protocol Methods

**Files:**
- Modify: `internal/farm/automation/runtime_friend.go`
- Modify: `internal/farm/automation/runtime_helpers.go`
- Modify: `internal/farm/automation/runtime_friend_test.go`

- [ ] **Step 1: Update failing automation tests**

In `internal/farm/automation/runtime_friend_test.go`, update the first friend steal test response map to:

```go
caller := &fakeRuntimeCaller{responses: map[string]any{
	"gameCtl.getFriendList": []any{
		map[string]any{"gid": float64(10001), "name": "A", "workCounts": map[string]any{"collect": float64(2)}},
		map[string]any{"gid": float64(10002), "name": "B", "workCounts": map[string]any{"collect": float64(0)}},
	},
	"gameCtl.inspectFriendFarmByProtocol": map[string]any{
		"ok":          true,
		"hostGid":     float64(10001),
		"workLandIds": map[string]any{"collect": []any{float64(9), "3"}},
	},
	"gameCtl.checkFriendCanOperateByProtocol": map[string]any{"ok": true, "callbackCalled": true},
	"gameCtl.friendHarvestLandsByProtocol":    map[string]any{"ok": true},
}}
```

Change expected methods to:

```go
wantMethods := []string{
	"gameCtl.getFriendList",
	"gameCtl.inspectFriendFarmByProtocol",
	"gameCtl.checkFriendCanOperateByProtocol",
	"gameCtl.friendHarvestLandsByProtocol",
}
if !reflect.DeepEqual(calledRuntimeMethods(caller.calls), wantMethods) {
	t.Fatalf("methods = %#v, want %#v", calledRuntimeMethods(caller.calls), wantMethods)
}
```

Add a new help assertion:

```go
func TestRuntimeFacadeFriendHelpUsesFarmingProtocol(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "workCounts": map[string]any{"help": float64(2)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"farming": []any{float64(1), float64(2)}},
		},
		"gameCtl.friendFarmingByProtocol": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_help")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("friend_help should report OK, got %#v", result)
	}
	wantMethods := []string{"gameCtl.getFriendList", "gameCtl.inspectFriendFarmByProtocol", "gameCtl.friendFarmingByProtocol"}
	if !reflect.DeepEqual(calledRuntimeMethods(caller.calls), wantMethods) {
		t.Fatalf("methods = %#v, want %#v", calledRuntimeMethods(caller.calls), wantMethods)
	}
}
```

- [ ] **Step 2: Run tests to verify RED**

Run:

```powershell
go test ./internal/farm/automation -run "TestRuntimeFacadeFriend"
```

Expected: FAIL because Go still calls `gameCtl.enterFriendFarm`, `gameCtl.getFarmStatus`, and `gameCtl.triggerOneClickOperation`.

- [ ] **Step 3: Add helper to read protocol work IDs**

In `internal/farm/automation/runtime_helpers.go`, add:

```go
func collectProtocolWorkLandIDs(status map[string]any, keys ...string) []int {
	seen := map[int]bool{}
	workLandIDs := mapFromAny(status["workLandIds"])
	for _, key := range keys {
		for _, item := range sliceFromAny(workLandIDs[key]) {
			id := intFromAny(item)
			if id > 0 {
				seen[id] = true
			}
		}
	}
	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}
```

- [ ] **Step 4: Replace friend steal flow**

In `runFriendSteal`, replace enter/status/harvest block with:

```go
	inspectValue, err := r.caller.Call(ctx, "gameCtl.inspectFriendFarmByProtocol", []any{map[string]any{
		"hostGid":      gid,
		"silent":       true,
		"includeLands": false,
		"leaveAfter":   false,
		"source":       "farm_go_auto_friend_steal",
	}}, 45*time.Second)
	if err != nil {
		return ActionResult{OK: false, Status: StatusFailed, TaskID: taskID, Message: "协议进入好友农场失败：" + err.Error()}
	}
	if failed, reason := runtimeResultFailed(inspectValue); failed {
		return ActionResult{OK: false, Status: StatusFailed, TaskID: taskID, Message: "协议进入好友农场返回失败：" + reason}
	}
	inspect := mapFromAny(inspectValue)
	landIDs := collectProtocolWorkLandIDs(inspect, "collect")
	if len(landIDs) == 0 {
		return ActionResult{OK: true, Status: StatusOK, TaskID: taskID, Message: "协议检查后没有检测到可偷菜地块，本轮好友偷菜跳过。"}
	}
	checkValue, err := r.caller.Call(ctx, "gameCtl.checkFriendCanOperateByProtocol", []any{map[string]any{
		"hostGid":      gid,
		"operationId":  10004,
		"silent":       true,
		"waitReplyMs":  800,
		"source":       "farm_go_auto_friend_steal",
	}}, 15*time.Second)
	if err == nil {
		if failed, reason := runtimeResultFailed(checkValue); failed {
			return ActionResult{OK: false, Status: StatusFailed, TaskID: taskID, Message: "偷菜次数检查失败：" + reason}
		}
	}
	harvestResult, err := r.caller.Call(ctx, "gameCtl.friendHarvestLandsByProtocol", []any{map[string]any{
		"hostGid":     gid,
		"landIds":     landIDs,
		"isAll":       false,
		"silent":      true,
		"waitReplyMs": 1200,
		"source":      "farm_go_auto_friend_steal",
	}}, 45*time.Second)
```

- [ ] **Step 5: Replace friend help flow**

In `runFriendHelp`, replace enter/status/one-click block with:

```go
	inspectValue, err := r.caller.Call(ctx, "gameCtl.inspectFriendFarmByProtocol", []any{map[string]any{
		"hostGid":      gid,
		"silent":       true,
		"includeLands": false,
		"leaveAfter":   false,
		"source":       "farm_go_auto_friend_help",
	}}, 45*time.Second)
	if err != nil {
		return ActionResult{OK: false, Status: StatusFailed, TaskID: taskID, Message: "协议进入好友农场失败：" + err.Error()}
	}
	if failed, reason := runtimeResultFailed(inspectValue); failed {
		return ActionResult{OK: false, Status: StatusFailed, TaskID: taskID, Message: "协议进入好友农场返回失败：" + reason}
	}
	landIDs := collectProtocolWorkLandIDs(mapFromAny(inspectValue), "farming", "water", "eraseGrass", "killBug")
	if len(landIDs) == 0 {
		return ActionResult{OK: true, Status: StatusOK, TaskID: taskID, Message: "协议检查后没有检测到可帮忙地块，本轮好友帮忙跳过。"}
	}
	helpResult, err := r.caller.Call(ctx, "gameCtl.friendFarmingByProtocol", []any{map[string]any{
		"hostGid":     gid,
		"landIds":     landIDs,
		"source":      0,
		"silent":      true,
		"waitReplyMs": 1200,
	}}, 45*time.Second)
```

- [ ] **Step 6: Replace friend mischief status source**

In `runFriendMischief`, keep `gameCtl.friendMischiefLandsBatch` but replace `enterFriendFarm` and `getFarmStatus` with `inspectFriendFarmByProtocol`. Collect bug and grass separately:

```go
	inspectValue, err := r.caller.Call(ctx, "gameCtl.inspectFriendFarmByProtocol", []any{map[string]any{
		"hostGid":      gid,
		"silent":       true,
		"includeLands": false,
		"leaveAfter":   false,
		"source":       "farm_go_auto_friend_mischief",
	}}, 45*time.Second)
	if err != nil {
		return ActionResult{OK: false, Status: StatusFailed, TaskID: taskID, Message: "协议进入好友农场失败：" + err.Error()}
	}
	inspect := mapFromAny(inspectValue)
	bugLandIDs := collectProtocolWorkLandIDs(inspect, "bug")
	grassLandIDs := collectProtocolWorkLandIDs(inspect, "grass")
	if len(bugLandIDs) == 0 && len(grassLandIDs) == 0 {
		return ActionResult{OK: true, Status: StatusOK, TaskID: taskID, Message: "协议检查后没有检测到可捣乱地块，本轮好友捣乱跳过。"}
	}
	mischiefResult, err := r.caller.Call(ctx, "gameCtl.friendMischiefLandsBatch", []any{map[string]any{
		"hostGid":      gid,
		"bugLandIds":   bugLandIDs,
		"grassLandIds": grassLandIDs,
		"dryRun":       false,
		"silent":       true,
		"source":       "farm_go_auto_friend_mischief",
	}}, 45*time.Second)
```

- [ ] **Step 7: Run automation tests**

Run:

```powershell
go test ./internal/farm/automation -run "TestRuntimeFacadeFriend"
```

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add internal/farm/automation/runtime_friend.go internal/farm/automation/runtime_helpers.go internal/farm/automation/runtime_friend_test.go
git commit -m "feat: use protocol methods for automatic friend tasks"
```

---

### Task 5: Switch Manual Social Actions To Protocol Methods

**Files:**
- Modify: `internal/farm/social/actions.go`
- Modify: `internal/farm/social/actions_test.go`

- [ ] **Step 1: Update action tests**

In `internal/farm/social/actions_test.go`, update steal/help/mischief runtime response maps to use:

```go
"gameCtl.inspectFriendFarmByProtocol": map[string]any{
	"ok":          true,
	"workLandIds": map[string]any{"collect": []any{float64(9)}},
},
"gameCtl.friendHarvestLandsByProtocol": map[string]any{"ok": true},
```

For help:

```go
"gameCtl.inspectFriendFarmByProtocol": map[string]any{
	"ok":          true,
	"workLandIds": map[string]any{"farming": []any{float64(1)}},
},
"gameCtl.friendFarmingByProtocol": map[string]any{"ok": true},
```

For mischief:

```go
"gameCtl.inspectFriendFarmByProtocol": map[string]any{
	"ok":          true,
	"workLandIds": map[string]any{"bug": []any{float64(1)}, "grass": []any{float64(2)}},
},
"gameCtl.friendMischiefLandsBatch": map[string]any{"ok": true},
```

- [ ] **Step 2: Run tests to verify RED**

Run:

```powershell
go test ./internal/farm/social -run "TestFriendAction"
```

Expected: FAIL because social actions still call `enterFriendFarm`, `getFarmStatus`, and `triggerOneClickOperation`.

- [ ] **Step 3: Add social helper for protocol IDs**

Add to `internal/farm/social/actions.go`:

```go
func collectProtocolWorkLandIDs(status map[string]any, keys ...string) []int {
	seen := map[int]bool{}
	workLandIDs := mapFromAny(status["workLandIds"])
	for _, key := range keys {
		for _, item := range sliceFromAny(workLandIDs[key]) {
			id := PositiveInt(item)
			if id > 0 {
				seen[id] = true
			}
		}
	}
	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}
```

If `sort` is not already imported in `actions.go`, add it to the import block.

- [ ] **Step 4: Replace `runSteal`**

Replace the enter/status block in `runSteal` with:

```go
	statusValue, err := s.caller.Call(ctx, "gameCtl.inspectFriendFarmByProtocol", []any{map[string]any{
		"hostGid":      gid,
		"silent":       true,
		"includeLands": false,
		"leaveAfter":   false,
		"source":       "farm_go_social_steal",
	}}, 45*time.Second)
	if err != nil {
		return ActionResult{OK: false, Status: StatusFailed, Message: "协议进入好友农场失败：" + err.Error()}
	}
	status := mapFromAny(statusValue)
	landIDs := collectProtocolWorkLandIDs(status, "collect")
```

Replace the harvest call with:

```go
	result := s.callRuntime(ctx, "gameCtl.friendHarvestLandsByProtocol", []any{map[string]any{
		"hostGid":     gid,
		"landIds":     landIDs,
		"isAll":       false,
		"silent":      true,
		"waitReplyMs": 1200,
		"source":      "farm_go_social_steal",
	}}, 45*time.Second, fmt.Sprintf("已提交好友 %d 的 %d 块土地偷菜请求。", gid, len(landIDs)), "好友偷菜失败：")
```

- [ ] **Step 5: Replace `runHelp`**

Replace the one-click call with:

```go
	result := s.callRuntime(ctx, "gameCtl.friendFarmingByProtocol", []any{map[string]any{
		"hostGid":     gid,
		"landIds":     landIDs,
		"source":      0,
		"silent":      true,
		"waitReplyMs": 1200,
	}}, 45*time.Second, fmt.Sprintf("已提交好友 %d 的帮忙请求。", gid), "好友帮忙失败：")
```

The `landIDs` variable must come from:

```go
landIDs := collectProtocolWorkLandIDs(status, "farming", "water", "eraseGrass", "killBug")
```

- [ ] **Step 6: Replace `runMischief`**

Use protocol inspect and pass separate ID groups:

```go
	bugLandIDs := collectProtocolWorkLandIDs(status, "bug")
	grassLandIDs := collectProtocolWorkLandIDs(status, "grass")
	if len(bugLandIDs) == 0 && len(grassLandIDs) == 0 {
		return ActionResult{OK: true, Status: StatusOK, Message: "没有检测到可捣乱地块，本次跳过。"}
	}
	return s.callRuntime(ctx, "gameCtl.friendMischiefLandsBatch", []any{map[string]any{
		"hostGid":      gid,
		"bugLandIds":   bugLandIDs,
		"grassLandIds": grassLandIDs,
		"dryRun":       false,
		"silent":       true,
		"source":       "farm_go_social_mischief",
	}}, 45*time.Second, fmt.Sprintf("已提交好友 %d 的 %d 块土地捣乱请求。", gid, len(bugLandIDs)+len(grassLandIDs)), "好友捣乱失败：")
```

- [ ] **Step 7: Run social action tests**

Run:

```powershell
go test ./internal/farm/social -run "TestFriendAction"
```

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add internal/farm/social/actions.go internal/farm/social/actions_test.go
git commit -m "feat: use protocol methods for social friend actions"
```

---

### Task 6: Switch Dog Guard Scan To `VisitService.Enter`

**Files:**
- Modify: `internal/farm/social/dog_guard.go`
- Modify: `internal/farm/social/dog_guard_test.go`

- [ ] **Step 1: Update dog scanner tests**

In `TestDogGuardScanStartScansFriendsAndPersistsRows`, replace spy responses with:

```go
caller := &fakeRuntimeCaller{responses: map[string]any{
	"gameCtl.getFriendList": map[string]any{"list": []any{
		map[string]any{"gid": float64(10001), "name": "A"},
	}},
	"gameCtl.inspectFriendFarmByProtocol": map[string]any{
		"ok": true,
		"briefDogInfo": map[string]any{
			"dogId":   float64(90021),
			"dogName": "护主犬",
		},
	},
}}
```

Add assertion:

```go
for _, call := range caller.calls {
	if call.method == "gameCtl.startRuntimeSpies" || call.method == "gameCtl.inspectDogGuardSignals" {
		t.Fatalf("dog scanner should not use runtime spies after protocol migration: %#v", caller.calls)
	}
}
```

- [ ] **Step 2: Run dog scanner tests to verify RED**

Run:

```powershell
go test ./internal/farm/social -run "TestDogGuard"
```

Expected: FAIL because `scanFriend` still calls `startRuntimeSpies`, `resetRuntimeSpyEvents`, `enterFriendFarm`, and `inspectDogGuardSignals`.

- [ ] **Step 3: Replace `scanFriend`**

In `internal/farm/social/dog_guard.go`, replace `scanFriend` with:

```go
func (s *DogGuardScanner) scanFriend(ctx context.Context, gid int, req DogGuardScanRequest) dogSignal {
	payload, err := s.caller.Call(ctx, "gameCtl.inspectFriendFarmByProtocol", []any{map[string]any{
		"hostGid":      gid,
		"silent":       true,
		"includeLands": false,
		"leaveAfter":   true,
		"waitReplyMs":  maxInt(req.EnterWaitMS, 5000),
	}}, 90*time.Second)
	if err != nil {
		return dogSignal{errText: err.Error()}
	}
	result := mapFromAny(payload)
	if failed, reason := runtimeResultFailed(result); failed {
		return dogSignal{errText: reason}
	}
	info := mapFromAny(result["briefDogInfo"])
	dogID := PositiveInt(firstExistingAny(info["dogId"], info["dog_id"]))
	dogName := firstText(info["dogName"], info["dog_name"])
	return dogSignal{
		dogID:   dogID,
		dogName: dogName,
	}
}
```

- [ ] **Step 4: Run dog scanner tests**

Run:

```powershell
go test ./internal/farm/social -run "TestDogGuard"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/farm/social/dog_guard.go internal/farm/social/dog_guard_test.go
git commit -m "feat: scan dog guard through visit protocol"
```

---

### Task 7: Full Verification And Regression Sweep

**Files:**
- Modify only files required by failing tests from previous tasks.
- No new feature code should be introduced in this task.

- [ ] **Step 1: Run JavaScript protocol tests**

Run:

```powershell
node scripts\test-friend-pure-protocol.js
node scripts\test-protocol-failure-text.js
node scripts\test-mail-reward-protocol.js
```

Expected: all commands exit with code `0`.

- [ ] **Step 2: Run backend tests**

Run:

```powershell
go test -count=1 ./...
```

Expected: PASS.

- [ ] **Step 3: Run frontend tests**

Run:

```powershell
cd frontend
npm test
```

Expected: PASS.

- [ ] **Step 4: Build frontend**

Run:

```powershell
cd frontend
npm run build
```

Expected: PASS.

- [ ] **Step 5: Check for old non-protocol calls in friend paths**

Run:

```powershell
rg -n "triggerOneClickOperation|inspectDogGuardSignals|startRuntimeSpies|gameCtl.getFarmStatus|gameCtl.enterFriendFarm" internal/farm/automation internal/farm/social
```

Expected: no matches in friend steal/help/mischief/dog-guard paths. Matches in unrelated own-farm paths are acceptable only outside `runtime_friend.go`, `actions.go`, and `dog_guard.go`.

- [ ] **Step 6: Check diff hygiene**

Run:

```powershell
git diff --check
```

Expected: no whitespace errors.

- [ ] **Step 7: Commit verification updates**

```powershell
git add resources/wmpf/button.js scripts/test-friend-pure-protocol.js internal/farm/automation internal/farm/social
git commit -m "chore: verify friend pure protocol integration"
```

---

## Runtime Smoke Test

After automated tests pass, run these manually with QQ/WMPF connected:

1. Open Farm_Go and confirm runtime status is ready.
2. Run manual social dog-guard scan with `Limit = 1`.
3. Confirm the logs show `inspectFriendFarmByProtocol`, not `inspectDogGuardSignals`.
4. Run manual friend help on one known helpable friend.
5. Confirm runtime result includes `action: friend_farming_protocol`.
6. Run manual friend steal on one known stealable friend.
7. Confirm runtime result includes `methodName: Harvest`.
8. Run manual friend mischief with dry-run disabled on a safe test friend.
9. Confirm runtime result includes `methodName: PutInsects` or `methodName: PutWeeds`.

If a live protocol callback returns a decoded shape different from the VM mock, capture the callback payload using the existing `callbackPayload` fields and adjust only the normalizer functions in `button.js`.

## Rollback Plan

The plan keeps old `gameCtl` methods in `button.js`, so rollback is narrow:

- revert Go callers in `runtime_friend.go`, `actions.go`, and `dog_guard.go` to previous method names;
- keep the new protocol methods dormant in `button.js`;
- rerun `go test -count=1 ./...`.

## Self-Review

- Spec coverage: The plan covers all four requested features: steal via `Harvest`, help via `Farming`, mischief via `PutInsects`/`PutWeeds`, and dog guard via `VisitService.Enter` `brief_dog_info`.
- Placeholder scan: The plan contains concrete file paths, method names, request bytes, test commands, and expected outcomes.
- Type consistency: Go caller method names match exported `gameCtl` names: `inspectFriendFarmByProtocol`, `checkFriendCanOperateByProtocol`, `friendHarvestLandsByProtocol`, `friendFarmingByProtocol`, and `friendMischiefLandsBatch`.
