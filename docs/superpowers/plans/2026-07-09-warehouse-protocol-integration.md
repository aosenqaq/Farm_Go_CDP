# Warehouse Protocol Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade Farm_Go warehouse refresh and sell flows to use QQ Farm `gamepb.itempb.ItemService` Bag/Sell protocol data while preserving the current Wails UI contract.

**Architecture:** Keep the existing Wails surface: React calls Go, Go calls `gameCtl`, and injected runtime JS talks to the mini-game. Move warehouse data authority from UI component snapshots toward protocol snapshots, keep UI snapshot as fallback, and retain original bag entries so sell requests can be built from real `id/count/uid/is_locked` values.

**Tech Stack:** Go/Wails, React/TypeScript, injected mini-game JavaScript in `resources/wmpf/button.js`, QQ Farm protobuf runtime codecs exposed inside the mini-game, Go unit tests and frontend Vitest tests.

---

## Current Findings

The external analysis document at `E:\desktop\qq-farm - 副本\仓库数据与出售链路解析.md` applies at the protocol layer, not at the HTTP/worker architecture layer.

Farm_Go currently uses:

- `frontend/src/views/AssetsLandView.tsx`: calls `FarmWarehouseRefresh()` and `FarmWarehouseSell({ itemKeys })`.
- `app.go`: `FarmWarehouseRefresh()` calls `gameCtl.refreshWarehouseSnapshot`; `FarmWarehouseSell()` calls `gameCtl.sellWarehouseItems`.
- `internal/farm/gameconfig.go`: normalizes runtime warehouse items into `WarehousePayload`.
- `resources/wmpf/button.js`: reads warehouse items mostly from UI/component `dataList`, and sells through `message.dispatchEvent('WarehouseItemsSell', requestPayload)`.

Farm_Go already has useful protocol groundwork:

- `resources/wmpf/button.js` contains `requestBagSnapshotByProtocol()`.
- That function sends `net.sendMsg(Uint8Array.from([]), 'Bag', callback, 'gamepb.itempb.ItemService')`.
- It can find and use `gamepb.itempb.BagReply` decode support.

The missing pieces are:

- Bag protocol output does not yet preserve complete original item entries for warehouse sell.
- Warehouse refresh does not yet prefer protocol data.
- There is no direct `SellRequest` encoder / `SellReply` decoder path in `button.js`.
- Go and React types expose only aggregated warehouse rows, not original protocol evidence.

---

## Target Data Model

Runtime JS should return this shape from warehouse refresh:

```json
{
  "ok": true,
  "source": "protocol",
  "items": [
    {
      "warehouseKey": "40002:open:123456",
      "itemId": 40002,
      "count": 5,
      "uid": 123456,
      "name": "白萝卜",
      "saleUnitPrice": 2,
      "saleRewards": [{ "itemId": 1001, "amount": 2 }],
      "canSell": true,
      "locked": false,
      "sourcePath": "ItemService.Bag.callback"
    }
  ],
  "originalItems": [
    {
      "id": 40002,
      "itemId": 40002,
      "count": 5,
      "uid": 123456,
      "isLocked": false,
      "is_locked": false,
      "expireTime": 0,
      "sourcePath": "ItemService.Bag.callback"
    }
  ],
  "capacity": {
    "used": 12,
    "max": 210
  },
  "fallbackSource": null
}
```

The UI can continue to consume `items`. The sell implementation should use `originalItems` or sell-ready entries associated with selected `warehouseKey` values.

---

## File Structure

- Modify `resources/wmpf/button.js`
  - Add protocol Bag normalization for full warehouse entries.
  - Add protocol Sell request encoding and reply decoding.
  - Make `refreshWarehouseSnapshot()` prefer protocol results and fall back to UI snapshots.
  - Make `sellWarehouseItems()` prefer direct protocol Sell and fall back to `WarehouseItemsSell`.

- Modify `internal/farm/gameconfig.go`
  - Preserve `uid`, `source`, and protocol-derived sale metadata where useful.
  - Keep existing JSON shape stable for current React code.

- Modify `app.go`
  - Pass protocol-preference flags into `gameCtl.refreshWarehouseSnapshot` and `gameCtl.sellWarehouseItems`.
  - Surface protocol/fallback failures in `WarehouseSellPayload.Sell`.

- Modify `app_test.go`
  - Cover protocol-source warehouse refresh.
  - Cover protocol sell payload forwarding and fallback behavior.

- Modify `frontend/src/views/AssetsLandView.tsx`
  - Keep current UI contract.
  - Optionally display protocol/fallback status in existing warehouse message text.

- Modify `frontend/src/views/AssetsLandView.test.tsx`
  - Cover current `itemKeys` sell path with protocol-source payloads.

---

## Task 1: Preserve Full Bag Protocol Entries

**Files:**
- Modify: `resources/wmpf/button.js`

- [ ] **Step 1: Add failing runtime-shape test notes**

Manual verification will use the injected console because `button.js` runs inside QQ/WMPF runtime.

Run after implementation:

```js
await gameCtl.requestBagSnapshotByProtocol({ silent: true, waitMs: 1200 })
```

Expected protocol result after this task:

```json
{
  "ok": true,
  "success": true,
  "items": [{ "itemId": 40002, "count": 5 }],
  "originalItems": [{ "id": 40002, "count": 5, "uid": 123456, "isLocked": false }]
}
```

- [ ] **Step 2: Extend Bag item normalization**

In `resources/wmpf/button.js`, update `normalizeBagReplyRuntimeItem(raw)` so it reads these fields:

```js
const uid = Number(
  safeReadKey(raw, 'uid') ||
  safeReadKey(raw, 'itemUid') ||
  safeReadKey(raw, 'sourceId')
) || 0;
const expireTime = Number(
  safeReadKey(raw, 'expire_time') != null ? safeReadKey(raw, 'expire_time') :
  safeReadKey(raw, 'expireTime')
) || 0;
const isLocked = !!(
  safeReadKey(raw, 'is_locked') != null ? safeReadKey(raw, 'is_locked') :
  safeReadKey(raw, 'isLocked')
);
```

Return these fields on the normalized item:

```js
uid: uid,
sourceId: uid > 0 ? uid : itemId,
protocolItemId: uid > 0 && uid !== itemId ? uid : 0,
expireTime: expireTime,
isLocked: isLocked,
is_locked: isLocked,
locked: isLocked,
canSell: isLocked ? false : readRuntimeConfigCanSell(cfg),
saleUnitPrice: readRuntimeConfigSaleUnitPrice(cfg),
saleCurrencyId: readRuntimeConfigSaleCurrencyId(cfg),
saleRewards: readRuntimeConfigSaleRewards(cfg),
```

- [ ] **Step 3: Add small helpers for runtime config sale fields**

Add helpers near the existing warehouse sale helper functions:

```js
function readRuntimeConfigCanSell(cfg) {
  if (!cfg || typeof cfg !== 'object') return false;
  if (safeReadKey(cfg, 'canSell') === true) return true;
  if (Number(safeReadKey(cfg, 'price')) > 0) return true;
  const sells = safeReadKey(cfg, 'sells') || safeReadKey(cfg, 'saleRewards');
  return normalizeWarehouseSaleRewards(sells).length > 0;
}

function readRuntimeConfigSaleUnitPrice(cfg) {
  if (!cfg || typeof cfg !== 'object') return 0;
  return Number(
    safeReadKey(cfg, 'saleUnitPrice') ||
    safeReadKey(cfg, 'sellPrice') ||
    safeReadKey(cfg, 'price')
  ) || 0;
}

function readRuntimeConfigSaleCurrencyId(cfg) {
  if (!cfg || typeof cfg !== 'object') return 0;
  return Number(
    safeReadKey(cfg, 'saleCurrencyId') ||
    safeReadKey(cfg, 'priceId') ||
    safeReadKey(cfg, 'price_id')
  ) || 0;
}

function readRuntimeConfigSaleRewards(cfg) {
  if (!cfg || typeof cfg !== 'object') return [];
  return normalizeWarehouseSaleRewards(
    safeReadKey(cfg, 'saleRewards') ||
    safeReadKey(cfg, 'sells') ||
    safeReadKey(cfg, 'sellRewards')
  );
}
```

- [ ] **Step 4: Return `originalItems` from `requestBagSnapshotByProtocol()`**

Inside `requestBagSnapshotByProtocol()`, after `items` has been collected, build:

```js
const originalItems = items.map(function (item) {
  return {
    id: Number(item && (item.id != null ? item.id : item.itemId)) || 0,
    itemId: Number(item && (item.itemId != null ? item.itemId : item.id)) || 0,
    count: Number(item && item.count) || 0,
    uid: Number(item && item.uid) || 0,
    isLocked: item && item.isLocked === true || item && item.is_locked === true || item && item.locked === true,
    is_locked: item && item.isLocked === true || item && item.is_locked === true || item && item.locked === true,
    expireTime: Number(item && item.expireTime) || 0,
    sourcePath: item && item.sourcePath || 'ItemService.Bag.callback'
  };
}).filter(function (item) {
  return item.id > 0 && item.count > 0;
});
```

Set:

```js
payload.originalItems = originalItems;
```

- [ ] **Step 5: Manually verify in runtime**

Run:

```js
await gameCtl.requestBagSnapshotByProtocol({ silent: true, waitMs: 1200 })
```

Expected:

- `ok === true`
- `items.length > 0` or a clear `reason`
- `originalItems` exists
- each sellable item has `id`, `count`, and preferably `uid`

---

## Task 2: Prefer Protocol Data in Warehouse Refresh

**Files:**
- Modify: `resources/wmpf/button.js`
- Modify: `app.go`
- Test: `app_test.go`

- [ ] **Step 1: Add Go test for protocol-source refresh payload**

In `app_test.go`, add:

```go
func TestAppWarehouseRefreshAcceptsProtocolSourceSnapshot(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.refreshWarehouseSnapshot": map[string]any{
			"ok":     true,
			"source": "protocol",
			"items": []any{
				map[string]any{
					"itemId":        float64(40002),
					"count":         float64(5),
					"name":          "白萝卜",
					"warehouseKey":  "40002:open:123456",
					"saleUnitPrice": float64(2),
					"uid":           float64(123456),
				},
			},
			"originalItems": []any{
				map[string]any{"id": float64(40002), "count": float64(5), "uid": float64(123456)},
			},
		},
	})

	payload := app.FarmWarehouseRefresh()

	if payload.Status != "runtime" || len(payload.Items) != 1 {
		t.Fatalf("expected protocol warehouse payload, got %#v", payload)
	}
	if payload.Items[0].ID != "40002:open:123456" || !payload.Items[0].CanSell {
		t.Fatalf("unexpected protocol item %#v", payload.Items[0])
	}
	if !link.called("gameCtl.refreshWarehouseSnapshot") {
		t.Fatalf("expected runtime call, got %#v", link.calls)
	}
}
```

- [ ] **Step 2: Run failing test**

Run:

```powershell
go test ./... -run TestAppWarehouseRefreshAcceptsProtocolSourceSnapshot -count=1
```

Expected before implementation: it may already pass if `BuildRuntimeWarehouse` accepts the shape. If it passes, keep it as regression coverage.

- [ ] **Step 3: Pass protocol preference from Go**

In `app.go`, update the argument map in `FarmWarehouseRefresh()`:

```go
value, err := a.supervisor.Call(a.contextOrBackground(), "gameCtl.refreshWarehouseSnapshot", []any{map[string]any{
	"silent":         true,
	"preferProtocol": true,
	"protocolWaitMs": 1200,
	"closeAfter":     true,
	"openTimeoutMs":  2600,
	"readTimeoutMs":  3200,
	"allowEmpty":     true,
}}, 15*time.Second)
```

- [ ] **Step 4: Prefer protocol in JS refresh**

At the start of `refreshWarehouseSnapshot(opts)` in `resources/wmpf/button.js`, before opening the warehouse UI:

```js
if (opts.preferProtocol !== false) {
  const protocolSnapshot = await requestBagSnapshotByProtocol({
    silent: true,
    waitMs: opts.protocolWaitMs == null ? 1200 : opts.protocolWaitMs,
    pollMs: opts.pollMs,
    sortMode: opts.sortMode
  });
  if (protocolSnapshot && protocolSnapshot.success === true && Array.isArray(protocolSnapshot.items) && protocolSnapshot.items.length > 0) {
    const protocolItems = aggregateWarehouseRuntimeItems(protocolSnapshot.items);
    return opts.silent ? {
      ok: true,
      action: 'refreshWarehouseSnapshot',
      source: 'protocol',
      items: protocolItems,
      originalItems: Array.isArray(protocolSnapshot.originalItems) ? protocolSnapshot.originalItems : [],
      itemCount: protocolItems.length,
      capacity: protocolSnapshot.capacity || null,
      protocol: {
        methodName: protocolSnapshot.methodName,
        serviceName: protocolSnapshot.serviceName,
        callbackItemCount: protocolSnapshot.callbackItemCount,
        callbackSeedCount: protocolSnapshot.callbackSeedCount
      }
    } : out({
      ok: true,
      action: 'refreshWarehouseSnapshot',
      source: 'protocol',
      items: protocolItems,
      originalItems: Array.isArray(protocolSnapshot.originalItems) ? protocolSnapshot.originalItems : [],
      itemCount: protocolItems.length,
      capacity: protocolSnapshot.capacity || null
    });
  }
}
```

- [ ] **Step 5: Run Go tests**

Run:

```powershell
go test ./... -run "TestAppWarehouseRefresh|TestAppWarehouseReadsRuntimeSnapshot" -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add app.go app_test.go resources/wmpf/button.js
git commit -m "feat: prefer protocol warehouse snapshots"
```

---

## Task 3: Add Direct Protocol Sell

**Files:**
- Modify: `resources/wmpf/button.js`
- Modify: `app.go`
- Test: `app_test.go`

- [ ] **Step 1: Add Go regression test for protocol sell result**

In `app_test.go`, add:

```go
func TestAppWarehouseSellAcceptsProtocolSellResult(t *testing.T) {
	app, link := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.sellWarehouseItems": map[string]any{
			"ok":              true,
			"source":          "protocol",
			"requestPayload":  []any{map[string]any{"id": float64(40002), "count": float64(5), "uid": float64(123456)}},
			"protocolRewards": []any{map[string]any{"itemId": float64(1001), "amount": float64(10)}},
			"afterItems":      []any{},
		},
	})

	sold := app.FarmWarehouseSell(map[string]any{"itemKeys": []any{"40002:open:123456"}})

	if !sold.OK || sold.Sell["source"] != "protocol" {
		t.Fatalf("expected protocol sell payload, got %#v", sold)
	}
	if !link.called("gameCtl.sellWarehouseItems") {
		t.Fatalf("expected runtime sell call, got %#v", link.calls)
	}
}
```

- [ ] **Step 2: Run failing or regression test**

Run:

```powershell
go test ./... -run TestAppWarehouseSellAcceptsProtocolSellResult -count=1
```

Expected: PASS once the fake runtime payload is accepted. This protects the Go result shape.

- [ ] **Step 3: Pass protocol sell preference from Go**

In `buildWarehouseSellRuntimeArgs()` in `app.go`, add:

```go
"preferProtocol": true,
"protocolWaitMs": 1200,
```

The returned map should include:

```go
return map[string]any{
	"silent":          true,
	"preferProtocol":  true,
	"protocolWaitMs":  1200,
	"itemIds":         itemIDs,
	"itemKeys":        itemKeys,
	"closeAfter":      true,
	"forceCloseAfter": false,
	"openTimeoutMs":   2600,
	"readTimeoutMs":   2600,
	"sellTimeoutMs":   9000,
	"pollMs":          140,
}, nil
```

- [ ] **Step 4: Add protobuf Sell codecs**

In `resources/wmpf/button.js`, add near `getBagReplyCodec()`:

```js
function getSellRequestCodec() {
  const systemItempb = safeCall(function () {
    return getSystemExportRuntime(
      ['chunks:///_virtual/itempb.ts', './itempb.ts'],
      'gamepb'
    );
  }, null);
  const systemGamepb = systemItempb && systemItempb.value ? systemItempb.value : null;
  const systemCodec = systemGamepb ? safeGetNested(systemGamepb, 'itempb.SellRequest') : null;
  if (systemCodec && typeof systemCodec.encode === 'function' && typeof systemCodec.create === 'function') return systemCodec;

  const protobufRoot = safeCall(function () { return getProtobufDefault(); }, null);
  if (!protobufRoot) return null;
  const direct = safeGetNested(protobufRoot, 'gamepb.itempb.SellRequest');
  if (direct && typeof direct.encode === 'function' && typeof direct.create === 'function') return direct;
  const found = findNestedValueByKey(protobufRoot, 'SellRequest', 6);
  const value = found && found.value ? found.value : null;
  return value && typeof value.encode === 'function' && typeof value.create === 'function' ? value : null;
}

function getSellReplyCodec() {
  const systemItempb = safeCall(function () {
    return getSystemExportRuntime(
      ['chunks:///_virtual/itempb.ts', './itempb.ts'],
      'gamepb'
    );
  }, null);
  const systemGamepb = systemItempb && systemItempb.value ? systemItempb.value : null;
  const systemCodec = systemGamepb ? safeGetNested(systemGamepb, 'itempb.SellReply') : null;
  if (systemCodec && typeof systemCodec.decode === 'function') return systemCodec;

  const protobufRoot = safeCall(function () { return getProtobufDefault(); }, null);
  if (!protobufRoot) return null;
  const direct = safeGetNested(protobufRoot, 'gamepb.itempb.SellReply');
  if (direct && typeof direct.decode === 'function') return direct;
  const found = findNestedValueByKey(protobufRoot, 'SellReply', 6);
  const value = found && found.value ? found.value : null;
  return value && typeof value.decode === 'function' ? value : null;
}
```

- [ ] **Step 5: Add Sell request encoder**

Add:

```js
function buildSellRequestBytes(items) {
  const list = (Array.isArray(items) ? items : []).map(function (item) {
    return {
      id: Number(item && item.id) || 0,
      count: Number(item && item.count) || 0,
      uid: Number(item && item.uid) || 0
    };
  }).filter(function (item) {
    return item.id > 0 && item.count > 0;
  });
  if (list.length <= 0) return [];

  const codec = getSellRequestCodec();
  if (codec && typeof codec.encode === 'function' && typeof codec.create === 'function') {
    const message = codec.create({ items: list });
    const encoded = codec.encode(message).finish();
    return Array.prototype.slice.call(encoded);
  }

  const writer = protobufWriterCreate();
  if (!writer) throw new Error('SellRequest codec not found');
  list.forEach(function (item) {
    writer.uint32(10).fork();
    writer.uint32(8).int64(item.id);
    writer.uint32(16).int64(item.count);
    if (item.uid > 0) writer.uint32(48).int64(item.uid);
    writer.ldelim();
  });
  return Array.prototype.slice.call(writer.finish());
}
```

If there is no existing `protobufWriterCreate()` helper, add:

```js
function protobufWriterCreate() {
  const protobufRoot = safeCall(function () { return getProtobufDefault(); }, null);
  const Writer = safeReadKey(protobufRoot, 'Writer') || safeReadKey(G, 'protobuf') && safeReadKey(safeReadKey(G, 'protobuf'), 'Writer');
  return Writer && typeof Writer.create === 'function' ? Writer.create() : null;
}
```

- [ ] **Step 6: Add Sell reply decoding**

Add:

```js
function decodeSellReplyFromArgs(argsLike) {
  const codec = getSellReplyCodec();
  const rawArgs = Array.prototype.slice.call(argsLike || []);
  for (let i = 0; i < rawArgs.length; i += 1) {
    const body = rawArgs[i] && typeof rawArgs[i] === 'object' ? safeReadKey(rawArgs[i], 'body') || rawArgs[i] : rawArgs[i];
    const bytes = normalizeProtocolBytes(body);
    if (!bytes || bytes.length <= 0 || !codec || typeof codec.decode !== 'function') continue;
    const decoded = safeCall(function () { return codec.decode(bytes); }, null);
    if (decoded) return decoded;
  }
  return null;
}

function collectSellReplyRewards(decoded) {
  const rewards = [];
  const list = decoded && Array.isArray(safeReadKey(decoded, 'get_items'))
    ? safeReadKey(decoded, 'get_items')
    : (decoded && Array.isArray(safeReadKey(decoded, 'getItems')) ? safeReadKey(decoded, 'getItems') : []);
  list.forEach(function (item) {
    const itemId = Number(safeReadKey(item, 'id') || safeReadKey(item, 'itemId')) || 0;
    const amount = Number(safeReadKey(item, 'count') || safeReadKey(item, 'amount')) || 0;
    if (itemId > 0 && amount > 0) rewards.push({ itemId: itemId, amount: amount });
  });
  return rewards;
}
```

- [ ] **Step 7: Add protocol sell dispatcher**

Add:

```js
async function sellWarehouseItemsByProtocol(entries, opts) {
  opts = opts || {};
  const net = getNetWebSocket();
  if (!net || typeof net.sendMsg !== 'function') throw new Error('netWebSocket.sendMsg not found');
  const requestPayload = (Array.isArray(entries) ? entries : []).map(function (entry) {
    return {
      id: Number(entry && (entry.request && entry.request.id || entry.id || entry.itemId)) || 0,
      count: Number(entry && (entry.request && entry.request.count || entry.count)) || 0,
      uid: Number(entry && (entry.request && entry.request.uid || entry.uid)) || 0
    };
  }).filter(function (item) {
    return item.id > 0 && item.count > 0;
  });
  if (requestPayload.length <= 0) {
    return { ok: false, reason: 'protocol_sell_payload_empty', requestPayload: [] };
  }

  const bytes = buildSellRequestBytes(requestPayload);
  const waitMs = Math.max(0, Number(opts.protocolWaitMs == null ? 1200 : opts.protocolWaitMs) || 0);
  let callbackRaw = null;
  let callbackError = null;
  let decoded = null;
  const callback = function () {
    callbackRaw = Array.prototype.slice.call(arguments);
    decoded = decodeSellReplyFromArgs(arguments);
  };

  try {
    net.sendMsg(Uint8Array.from(bytes), 'Sell', callback, 'gamepb.itempb.ItemService');
  } catch (error) {
    callbackError = error && error.message ? error.message : String(error || 'sendMsg failed');
  }

  if (waitMs > 0) await wait(waitMs);
  const rewards = collectSellReplyRewards(decoded);
  return {
    ok: callbackError == null,
    source: 'protocol',
    methodName: 'Sell',
    serviceName: 'gamepb.itempb.ItemService',
    requestPayload: requestPayload,
    requestBytes: bytes,
    requestHex: bytesToHex(Uint8Array.from(bytes)),
    callbackCalled: callbackRaw != null,
    callbackError: callbackError,
    protocolRewards: rewards,
    decoded: decoded ? summarizeSpyValue(decoded, 2) : null
  };
}
```

- [ ] **Step 8: Prefer protocol inside `sellWarehouseItems()`**

After `payload.requestPayload` is built and before `WarehouseItemsSell` dispatch:

```js
if (opts.preferProtocol !== false) {
  const protocolSell = await sellWarehouseItemsByProtocol(matchedTargets, opts);
  if (protocolSell && protocolSell.ok === true) {
    const afterSnapshot = await refreshWarehouseSnapshot({
      silent: true,
      preferProtocol: true,
      protocolWaitMs: opts.protocolWaitMs,
      closeAfter: false,
      allowEmpty: true,
      pollMs: opts.pollMs
    });
    payload.source = 'protocol';
    payload.requestDispatched = true;
    payload.protocolRewards = protocolSell.protocolRewards || [];
    payload.protocol = protocolSell;
    payload.afterItems = afterSnapshot && Array.isArray(afterSnapshot.items) ? afterSnapshot.items : [];
    payload.soldDiff = buildWarehouseSellDiff(
      payload.beforeItems,
      payload.afterItems,
      payload.requestPayload.map(function (entry) { return entry.id; })
    );
    payload.ok = true;
    if (closeAfter) {
      payload.closeResult = await closeWarehouseUi({
        timeoutMs: opts.closeTimeoutMs == null ? 1800 : opts.closeTimeoutMs,
        pollMs: opts.pollMs
      });
    }
    return opts.silent ? payload : out(payload);
  }
  payload.protocolError = protocolSell && (protocolSell.callbackError || protocolSell.reason) || 'protocol_sell_failed';
}
```

The existing `WarehouseItemsSell` dispatch remains the fallback.

- [ ] **Step 9: Run Go tests**

Run:

```powershell
go test ./... -run "TestAppWarehouseSell|TestAppWarehouseRefreshAndSellUseRuntime" -count=1
```

Expected: PASS.

- [ ] **Step 10: Runtime dry verification**

In the mini-game runtime console, run a non-destructive refresh first:

```js
const snapshot = await gameCtl.refreshWarehouseSnapshot({ silent: true, preferProtocol: true, allowEmpty: true })
snapshot.items.filter(x => x.canSell && !x.locked).slice(0, 3)
```

Only after confirming the selected item is safe to sell, run:

```js
await gameCtl.sellWarehouseItems({
  silent: true,
  preferProtocol: true,
  itemKeys: [snapshot.items.find(x => x.canSell && !x.locked).warehouseKey],
  protocolWaitMs: 1200
})
```

Expected:

- `source === 'protocol'` or fallback path records `protocolError`
- `requestPayload` contains `id/count/uid`
- `afterItems` refreshes

- [ ] **Step 11: Commit**

```powershell
git add app.go app_test.go resources/wmpf/button.js
git commit -m "feat: sell warehouse items by protocol"
```

---

## Task 4: Preserve UI Compatibility and Surface Source

**Files:**
- Modify: `internal/farm/gameconfig.go`
- Modify: `frontend/src/views/AssetsLandView.tsx`
- Test: `internal/farm/gameconfig_test.go`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Add Go normalization test for protocol fields**

In `internal/farm/gameconfig_test.go`, add:

```go
func TestBuildRuntimeWarehouseKeepsProtocolWarehouseKey(t *testing.T) {
	itemMap := map[int]itemInfoConfigItem{
		40002: {Name: "白萝卜", Type: 6},
	}
	payload := BuildRuntimeWarehouse(map[string]any{
		"items": []any{
			map[string]any{
				"itemId":        float64(40002),
				"count":         float64(5),
				"name":          "白萝卜",
				"warehouseKey":  "40002:open:123456",
				"saleUnitPrice": float64(2),
				"uid":           float64(123456),
			},
		},
	}, itemMap)

	if len(payload.Items) != 1 {
		t.Fatalf("expected one item, got %#v", payload)
	}
	if payload.Items[0].ID != "40002:open:123456" {
		t.Fatalf("expected protocol warehouse key, got %#v", payload.Items[0])
	}
	if payload.Items[0].EstimatedSellPrice != 10 {
		t.Fatalf("expected estimate from protocol sale unit price, got %#v", payload.Items[0])
	}
}
```

- [ ] **Step 2: Run Go normalization test**

Run:

```powershell
go test ./internal/farm -run TestBuildRuntimeWarehouseKeepsProtocolWarehouseKey -count=1
```

Expected: PASS or fail with a precise field mismatch.

- [ ] **Step 3: Keep frontend payload contract unchanged**

Do not require frontend changes for selling. The table should still use:

```tsx
const selectedSellableKeys = selectedKeys.filter((key) =>
  items.some((item) => item.id === key && item.canSell && !item.locked)
);
```

and:

```tsx
FarmWarehouseSell({ itemKeys } as any)
```

- [ ] **Step 4: Add optional source text**

In `WarehousePanel`, compute:

```tsx
const statusText = payload?.status === 'runtime'
  ? payload?.message || '仓库已从游戏运行时读取。'
  : payload?.message || '手动刷新仓库；后台默认仅在出售时刷新仓库快照。';
```

Use:

```tsx
<p>{statusText}</p>
```

Do not add visible debug protocol JSON to the UI.

- [ ] **Step 5: Run frontend test suite**

Run:

```powershell
cd frontend
npm test -- --run AssetsLandView
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add internal/farm/gameconfig.go internal/farm/gameconfig_test.go frontend/src/views/AssetsLandView.tsx frontend/src/views/AssetsLandView.test.tsx
git commit -m "test: preserve warehouse protocol ui contract"
```

---

## Task 5: Add Fallback and Failure Evidence

**Files:**
- Modify: `resources/wmpf/button.js`
- Test: manual runtime checks

- [ ] **Step 1: Ensure refresh fallback reports protocol failure**

When protocol refresh fails and UI fallback runs, include:

```js
payload.source = 'ui';
payload.protocolError = protocolSnapshot && (protocolSnapshot.reason || protocolSnapshot.callbackError) || 'protocol_snapshot_failed';
```

Expected fallback result:

```json
{
  "ok": true,
  "source": "ui",
  "protocolError": "bag_items_not_observed",
  "items": []
}
```

- [ ] **Step 2: Ensure sell fallback reports protocol failure**

When protocol sell fails and `WarehouseItemsSell` fallback runs, include:

```js
payload.source = 'ui_event';
payload.protocolError = payload.protocolError || 'protocol_sell_failed';
```

Expected fallback result:

```json
{
  "ok": true,
  "source": "ui_event",
  "protocolError": "SellRequest codec not found",
  "requestDispatched": true
}
```

- [ ] **Step 3: Manual runtime fallback test**

Run with protocol disabled:

```js
await gameCtl.refreshWarehouseSnapshot({ silent: true, preferProtocol: false, allowEmpty: true })
```

Expected:

- `source` is absent or `ui`
- existing UI snapshot behavior still works

- [ ] **Step 4: Manual protocol-preferred fallback test**

Run:

```js
await gameCtl.refreshWarehouseSnapshot({ silent: true, preferProtocol: true, protocolWaitMs: 1, allowEmpty: true })
```

Expected:

- no thrown exception
- UI fallback still returns a payload
- `protocolError` explains why protocol was not used

- [ ] **Step 5: Commit**

```powershell
git add resources/wmpf/button.js
git commit -m "fix: keep warehouse ui fallback evidence"
```

---

## Task 6: Final Verification

**Files:**
- No code changes expected unless verification finds defects.

- [ ] **Step 1: Run Go tests**

```powershell
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run frontend tests**

```powershell
cd frontend
npm test -- --run
```

Expected: PASS.

- [ ] **Step 3: Build frontend**

```powershell
cd frontend
npm run build
```

Expected: build completes without TypeScript or Vite errors.

- [ ] **Step 4: Runtime smoke test**

In an attached QQ Farm runtime:

```js
await gameCtl.refreshWarehouseSnapshot({ silent: true, preferProtocol: true, allowEmpty: true })
```

Expected:

- protocol source if Bag callback decodes successfully
- UI fallback with `protocolError` if protocol cannot decode

- [ ] **Step 5: Safe sell smoke test**

Pick one low-value sellable item from the refreshed snapshot:

```js
const snapshot = await gameCtl.refreshWarehouseSnapshot({ silent: true, preferProtocol: true, allowEmpty: true })
const target = snapshot.items.find(x => x.canSell && !x.locked)
target
```

If `target` is safe to sell:

```js
await gameCtl.sellWarehouseItems({
  silent: true,
  preferProtocol: true,
  itemKeys: [target.warehouseKey],
  protocolWaitMs: 1200
})
```

Expected:

- `ok === true`
- `requestPayload` contains real `id/count/uid`
- `afterItems` reflects reduced count or item removal

- [ ] **Step 6: Commit final fixes**

If verification required fixes:

```powershell
git add app.go app_test.go internal/farm/gameconfig.go internal/farm/gameconfig_test.go frontend/src/views/AssetsLandView.tsx frontend/src/views/AssetsLandView.test.tsx resources/wmpf/button.js
git commit -m "fix: verify warehouse protocol integration"
```

If no fixes were needed, do not create an empty commit.

---

## Risks and Guardrails

- Direct protocol Sell is destructive. Only run sell smoke tests on explicitly chosen low-value items.
- Keep UI-event sell fallback until protocol Sell is proven across QQ/WMPF versions.
- Do not remove `WarehouseItemsSell` fallback in this integration.
- Do not change the React API from `itemKeys` to raw protocol entries; keep raw entries inside runtime JS.
- If `SellRequest` codec cannot be found in runtime exports, use the manual protobuf writer fallback.
- If manual writer fallback fails because no writer exists, keep fallback to `WarehouseItemsSell` and report `protocolError`.

## Impact on Protocol Planting

Current protocol planting depends on the same Bag protocol helper, but through a narrower seed-only contract:

- Go automatic planting calls `gameCtl.getSeedList` with `protocolOnly: true` in `internal/farm/automation/runtime_own.go`.
- The runtime `getSeedList()` path calls `getAllSeeds()` -> `getWarehouseBackpackSeeds()` -> `requestBagSnapshotByProtocol()`.
- Planting consumes `seeds`, not `refreshWarehouseSnapshot().items`.
- Plant execution uses resolved `seedId` / `runtimeSeedId` and empty land IDs, then dispatches planting through `autoPlant()`.

Guardrails for this integration:

- Do not change the shape of `requestBagSnapshotByProtocol().seeds`.
- Do not change `getSeedList()` output keys: `itemId`, `seedId`, `name`, `count`, `level`, `rarity`, `layer`.
- Do not make `getWarehouseBackpackSeeds({ protocolOnly: true })` depend on opening the warehouse UI.
- Keep `writeWarehouseBackpackSeedCache(seeds, 'ItemService.Bag')` behavior intact.
- Add regression coverage for backpack-first planting and analytics planting before switching warehouse refresh to protocol-first.

Required planting regression checks:

```powershell
go test ./internal/farm/automation -run "TestRuntimeFacadeOwnPlantPassesBackpackFirstConfigToRuntime|TestRuntimeFacadeOwnPlantUsesConfiguredStrategySeedInsteadOfDefaultSeed" -count=1
go test ./internal/farm -run "TestBuildBackpackSeedOptions" -count=1
```

Runtime smoke check:

```js
await gameCtl.getSeedList({ silent: true, sortMode: 3, protocolOnly: true, noCache: true })
```

Expected:

- returns an array of seed rows
- each row keeps `itemId` / `seedId` / `count`
- no warehouse UI is required for the protocol-only path
- `autoPlant({ silent: true, protocolOnly: true, mode: 'backpack_first', emptyLandIds: [...] })` still resolves a seed from the Bag protocol list

---

## Self-Review

Spec coverage:

- Warehouse Bag protocol applicability is covered by Tasks 1 and 2.
- Sell protocol applicability is covered by Task 3.
- Current UI compatibility is covered by Task 4.
- Runtime fallback safety is covered by Task 5.
- Full verification is covered by Task 6.

Placeholder scan:

- No `TBD`, open-ended TODOs, or unspecified test commands remain.

Type consistency:

- The plan consistently uses current public UI keys: `itemKeys`, `WarehousePayload.items`, `WarehouseItem.id`.
- Runtime protocol entries consistently use `id`, `itemId`, `count`, `uid`, `isLocked`, `warehouseKey`.
