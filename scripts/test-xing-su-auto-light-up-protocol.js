const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const root = path.join(__dirname, "..");
const buttonJs = fs.readFileSync(path.join(root, "resources", "wmpf", "button.js"), "utf8");

function encodeVarint(value) {
  const bytes = [];
  let current = value;
  while (current >= 0x80) {
    bytes.push((current & 0x7f) | 0x80);
    current = Math.floor(current / 128);
  }
  bytes.push(current);
  return bytes;
}

function loadGameCtl(netWebSocket, listReply) {
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
    get() { return { oops: { netWebSocket } }; },
    registry: new Map([[
      "chunks:///_virtual/activitypb.ts",
      {
        namespace: {
          gamepb: {
            activitypb: {
              ListReply: {
                decode() { return typeof listReply === "function" ? listReply() : listReply; },
                toObject(value) { return value; },
              },
            },
          },
        },
      },
    ]]),
  };
  vm.createContext(context);
  vm.runInContext(buttonJs, context, { filename: "button.js", timeout: 5000 });
  return context.gameCtl;
}

function claimableReply(id) {
  return {
    groups: [{
      children: [{
        head: { id, status: 1 },
        mega_event: { rewards: [{ unlocked: true, claimed: false }] },
      }],
    }],
  };
}

(async () => {
  const dynamicID = 2026072901;
  const calls = [];
  let successfulListReads = 0;
  const gameCtl = loadGameCtl({
    sendMsg(bytes, methodName, callback, serviceName) {
      calls.push({ bytes: Array.from(bytes), methodName, serviceName });
      callback({ meta: { error_code: 0 }, body: Uint8Array.from([1]) });
    },
  }, () => {
    successfulListReads += 1;
    if (successfulListReads === 1) return claimableReply(dynamicID);
    return {
      groups: [{
        children: [{
          head: { id: dynamicID, status: 1 },
          mega_event: { rewards: [{ unlocked: true, claimed: true }] },
        }],
      }],
    };
  });
  const result = await gameCtl.autoLightUpXingSuByProtocol({ silent: true, waitMs: 0 });
  assert.strictEqual(result.ok, true);
  assert.strictEqual(result.skipped, false);
  assert.strictEqual(result.activityID, dynamicID);
  assert.strictEqual(result.verified, true);
  assert.deepStrictEqual(calls, [
    { bytes: [], methodName: "List", serviceName: "gamepb.activitypb.ActivityService" },
    {
      bytes: [8, ...encodeVarint(dynamicID), 16, 21, 186, 7, 0],
      methodName: "Operate",
      serviceName: "gamepb.activitypb.ActivityService",
    },
    { bytes: [], methodName: "List", serviceName: "gamepb.activitypb.ActivityService" },
  ]);

  const skippedCalls = [];
  const skippedGameCtl = loadGameCtl({
    sendMsg(bytes, methodName, callback, serviceName) {
      skippedCalls.push({ bytes: Array.from(bytes), methodName, serviceName });
      callback({ meta: { error_code: 0 }, body: Uint8Array.from([1]) });
    },
  }, {
    groups: [{
      children: [{
        head: { id: dynamicID, status: 1 },
        mega_event: { rewards: [{ unlocked: true, claimed: true }] },
      }],
    }],
  });
  const skipped = await skippedGameCtl.autoLightUpXingSuByProtocol({ silent: true, waitMs: 0 });
  assert.strictEqual(skipped.ok, true);
  assert.strictEqual(skipped.skipped, true);
  assert.strictEqual(skipped.reason, "no_claimable_xing_su_reward");
  assert.deepStrictEqual(skippedCalls.map((call) => call.methodName), ["List"]);

  const failedCalls = [];
  const failedGameCtl = loadGameCtl({
    sendMsg(bytes, methodName, callback, serviceName) {
      failedCalls.push({ bytes: Array.from(bytes), methodName, serviceName });
      callback({ meta: { error_code: 400, error_message: "活动查询失败" } });
    },
  }, claimableReply(dynamicID));
  const failed = await failedGameCtl.autoLightUpXingSuByProtocol({ silent: true, waitMs: 0 });
  assert.strictEqual(failed.ok, false);
  assert.match(failed.reason, /活动查询失败/);
  assert.deepStrictEqual(failedCalls.map((call) => call.methodName), ["List"]);

  const unverifiedCalls = [];
  const unverifiedGameCtl = loadGameCtl({
    sendMsg(bytes, methodName, callback, serviceName) {
      unverifiedCalls.push({ bytes: Array.from(bytes), methodName, serviceName });
      callback({ meta: { error_code: 0 }, body: Uint8Array.from([1]) });
    },
  }, claimableReply(dynamicID));
  const unverified = await unverifiedGameCtl.autoLightUpXingSuByProtocol({ silent: true, waitMs: 0 });
  assert.strictEqual(unverified.ok, false);
  assert.strictEqual(unverified.reason, "xing_su_activity_verification_failed");
  assert.deepStrictEqual(unverifiedCalls.map((call) => call.methodName), ["List", "Operate", "List"]);

  console.log("[xing-su-auto-light-up] query-derived IDs require post-action server verification");
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
