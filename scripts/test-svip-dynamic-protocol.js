const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const root = path.join(__dirname, "..");
const buttonJs = fs.readFileSync(path.join(root, "resources", "wmpf", "button.js"), "utf8");

function loadGameCtl(netWebSocket, qqvipModule) {
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
    globalThis: null,
    setTimeout,
  };
  context.globalThis = context;
  context.cc = {
    Button: function Button() {},
    director: { getScene() { return null; } },
    find() { return null; },
    game: { canvas: {} },
  };
  context.System = {
    get(moduleId) {
      if (qqvipModule && String(moduleId).includes("qqvippb")) return qqvipModule;
      return { oops: { netWebSocket } };
    },
  };
  vm.createContext(context);
  vm.runInContext(buttonJs, context, { filename: "button.js", timeout: 5000 });
  return context.gameCtl;
}

function qqvipModuleForStatus(statusProvider) {
  return {
    gamepb: {
      qqvippb: {
        GetQQVipRewardsStatusReply: {
          decode() { return statusProvider(); },
          toObject(value) { return value; },
        },
      },
    },
  };
}

async function runClaimScenario(status, options = {}) {
  const calls = [];
  const gameCtl = loadGameCtl({
    sendMsg(bytes, methodName, callback, serviceName) {
      calls.push({ bytes: Array.from(bytes), methodName, serviceName });
      if (methodName === "GetQQVipRewardsStatus") {
        if (options.statusTimeout) return;
        if (options.statusFailure) {
          callback({ meta: { error_code: 500, error_message: options.statusFailure }, body: {} });
          return;
        }
        callback({ meta: { error_code: 0 }, body: Uint8Array.from([8, 1]) });
        return;
      }
      callback(options.claimReply || { ok: true });
    },
  }, qqvipModuleForStatus(() => status));
  const result = await gameCtl.claimSvipDailyGift({
    silent: true,
    dryRun: options.dryRun === true,
    waitMs: options.statusTimeout ? 0 : 20,
    pollMs: 30,
  });
  return { calls, result };
}

(async () => {
  const calls = [];
  const gameCtl = loadGameCtl({
    sendMsg(bytes, methodName, callback, serviceName) {
      calls.push({ bytes: Array.from(bytes), methodName, serviceName });
      callback({ rewards: [{ id: 1 }] });
    },
  });

  const result = await gameCtl.requestSvipRewardsStatus({ silent: true, waitMs: 20 });

  assert.deepStrictEqual(calls, [{
    bytes: [],
    methodName: "GetQQVipRewardsStatus",
    serviceName: "gamepb.qqvippb.QQVipService",
  }]);
  assert.strictEqual(result.ok, true);
  assert.strictEqual(result.callbackCalled, true);

  const envelopeGameCtl = loadGameCtl({
    sendMsg(_bytes, _methodName, callback) {
      callback({
        meta: { error_code: 0, method_name: "GetQQVipRewardsStatus" },
        body: { 0: 0x0a, 1: 0x01, 2: 0x04 },
      });
    },
  });
  const envelopeResult = await envelopeGameCtl.requestSvipRewardsStatus({ silent: true, waitMs: 20 });
  assert.deepStrictEqual(
    Array.from(envelopeResult.callbackPayload[0].bodyBytes),
    [0x0a, 0x01, 0x04],
    "status diagnostics should expose only the protobuf body bytes needed to freeze the parser contract",
  );
  assert.strictEqual(envelopeResult.callbackPayload[0].meta.error_code, 0);

  const decodedBodyGameCtl = loadGameCtl({
    sendMsg(_bytes, _methodName, callback) {
      callback({
        meta: { error_code: 0 },
        body: {
          reward_statuses: [
            { reward_id: 4, claimed: false },
          ],
        },
      });
    },
  });
  const decodedBodyResult = await decodedBodyGameCtl.requestSvipRewardsStatus({ silent: true, waitMs: 20 });
  assert.strictEqual(
    decodedBodyResult.callbackPayload[0].body.reward_statuses[0].reward_id,
    4,
    "decoded status bodies should retain bounded nested field names for protocol contract discovery",
  );

  const reflectedGameCtl = loadGameCtl({
    sendMsg(_bytes, _methodName, callback) {
      callback({ meta: { error_code: 0 }, body: Uint8Array.from([8, 1]) });
    },
  }, {
    gamepb: {
      qqvippb: {
        GetQQVipRewardsStatusReply: {
          decode(bytes) {
            assert.deepStrictEqual(Array.from(bytes), [8, 1]);
            return { reward_statuses: [{ reward_id: 4, claimed: false }] };
          },
          toObject(value) { return value; },
        },
      },
    },
  });
  const reflectedResult = await reflectedGameCtl.requestSvipRewardsStatus({ silent: true, waitMs: 20 });
  assert.strictEqual(reflectedResult.decodedPayload.reward_statuses[0].reward_id, 4);

  const dynamic = await runClaimScenario({
    claimed_today: false,
    active_configs: [{ id: 1 }, { id: 4 }, { id: 3 }],
  });
  assert.deepStrictEqual(dynamic.calls.map((call) => call.methodName), [
    "GetQQVipRewardsStatus",
    "ClaimQQVipRewards",
  ]);
  assert.deepStrictEqual(dynamic.calls[1].bytes, [0x0a, 0x03, 0x01, 0x04, 0x03]);
  assert.deepStrictEqual(Array.from(dynamic.result.claimIds), [1, 4, 3]);

  const duplicates = await runClaimScenario({
    claimed_today: false,
    active_configs: [{ id: 4 }, { id: 1 }, { id: 4 }, { id: 3 }, { id: 1 }],
  });
  assert.deepStrictEqual(Array.from(duplicates.result.claimIds), [4, 1, 3]);
  assert.deepStrictEqual(duplicates.calls[1].bytes, [0x0a, 0x03, 0x04, 0x01, 0x03]);

  const disabled = await runClaimScenario({
    claimed_today: false,
    active_configs: [{ id: 1, is_enable: true }, { id: 4, is_enable: false }, { id: 3 }],
  });
  assert.deepStrictEqual(Array.from(disabled.result.claimIds), [1, 3]);
  assert.deepStrictEqual(disabled.calls[1].bytes, [0x0a, 0x02, 0x01, 0x03]);

  const alreadyClaimed = await runClaimScenario({
    claimed_today: true,
    active_configs: [{ id: 1 }],
  });
  assert.strictEqual(alreadyClaimed.result.ok, true);
  assert.strictEqual(alreadyClaimed.result.skipped, true);
  assert.strictEqual(alreadyClaimed.result.reason, "already_claimed");
  assert.deepStrictEqual(alreadyClaimed.calls.map((call) => call.methodName), ["GetQQVipRewardsStatus"]);

  const empty = await runClaimScenario({ claimed_today: false, active_configs: [] });
  assert.strictEqual(empty.result.ok, true);
  assert.strictEqual(empty.result.skipped, true);
  assert.strictEqual(empty.result.reason, "no_claimable_rewards");
  assert.deepStrictEqual(empty.calls.map((call) => call.methodName), ["GetQQVipRewardsStatus"]);

  for (const invalidId of [0, -1, 1.5, 0x100000000, "4"]) {
    const invalid = await runClaimScenario({
      claimed_today: false,
      active_configs: [{ id: invalidId }],
    });
    assert.strictEqual(invalid.result.ok, false, `invalid id ${invalidId} should fail`);
    assert.strictEqual(invalid.calls.length, 1, `invalid id ${invalidId} must not be claimed`);
  }

  const missing = await runClaimScenario({ claimed_today: false });
  assert.strictEqual(missing.result.ok, false);
  assert.strictEqual(missing.result.reason, "svip_active_configs_missing");
  assert.strictEqual(missing.calls.length, 1);

  const invalidClaimedState = await runClaimScenario({
    claimed_today: "true",
    active_configs: [{ id: 1 }],
  });
  assert.strictEqual(invalidClaimedState.result.ok, false);
  assert.strictEqual(invalidClaimedState.result.reason, "svip_claimed_today_invalid");
  assert.strictEqual(invalidClaimedState.calls.length, 1);

  const queryFailure = await runClaimScenario(null, { statusFailure: "SVIP状态读取失败" });
  assert.strictEqual(queryFailure.result.ok, false);
  assert.match(queryFailure.result.reason, /SVIP状态读取失败/);
  assert.strictEqual(queryFailure.calls.length, 1);

  const queryTimeout = await runClaimScenario(null, { statusTimeout: true });
  assert.strictEqual(queryTimeout.result.ok, false);
  assert.strictEqual(queryTimeout.result.reason, "svip_rewards_status_timeout");
  assert.strictEqual(queryTimeout.calls.length, 1);

  const multibyteIds = await runClaimScenario({
    claimed_today: false,
    active_configs: [{ id: 128 }, { id: 300 }],
  });
  assert.deepStrictEqual(multibyteIds.calls[1].bytes, [0x0a, 0x04, 0x80, 0x01, 0xac, 0x02]);

  const dryRun = await runClaimScenario({
    claimed_today: false,
    active_configs: [{ id: 1 }, { id: 4 }, { id: 3 }],
  }, { dryRun: true });
  assert.strictEqual(dryRun.result.ok, true);
  assert.deepStrictEqual(dryRun.calls.map((call) => call.methodName), ["GetQQVipRewardsStatus"]);
  assert.deepStrictEqual(Array.from(dryRun.result.requestBytes), [0x0a, 0x03, 0x01, 0x04, 0x03]);

  const claimFailure = await runClaimScenario({
    claimed_today: false,
    active_configs: [{ id: 1 }],
  }, { claimReply: { ok: false, error_message: "SVIP礼包领取失败" } });
  assert.strictEqual(claimFailure.result.ok, false);
  assert.match(claimFailure.result.reason, /SVIP礼包领取失败/);

  const longPayload = await runClaimScenario({
    claimed_today: false,
    active_configs: Array.from({ length: 128 }, (_, index) => ({ id: index + 1 })),
  });
  assert.deepStrictEqual(longPayload.calls[1].bytes.slice(0, 3), [0x0a, 0x81, 0x01]);

  console.log("[svip-dynamic-protocol] status query, strict ids, skips, failures, and protobuf boundaries are handled");
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
