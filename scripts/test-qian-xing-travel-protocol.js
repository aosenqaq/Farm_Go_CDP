const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const root = path.join(__dirname, "..");
const buttonJs = fs.readFileSync(path.join(root, "resources", "wmpf", "button.js"), "utf8");

function loadGameCtl(netWebSocket) {
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
    get() {
      return { oops: { netWebSocket } };
    },
  };
  vm.createContext(context);
  vm.runInContext(buttonJs, context, { filename: "button.js", timeout: 5000 });
  return context.gameCtl;
}

(async () => {
  const calls = [];
  const gameCtl = loadGameCtl({
    sendMsg(bytes, methodName, callback, serviceName) {
      calls.push({ bytes: Array.from(bytes), methodName, serviceName });
      callback({ meta: { error_code: 0 }, body: Uint8Array.from([]) });
    },
  });

  const result = await gameCtl.claimQianXingTravelRewards({ silent: true, waitMs: 20 });

  assert.deepStrictEqual(calls, [{
    bytes: [],
    methodName: "ClaimBattlePassRewards",
    serviceName: "gamepb.seasonpb.SeasonService",
  }]);
  assert.strictEqual(result.ok, true);
  assert.strictEqual(result.success, true);
  assert.strictEqual(result.skipped, false);
  assert.strictEqual(result.action, "qian_xing_travel_reward");
  assert.strictEqual(result.requestHex, "");

  const failingGameCtl = loadGameCtl({
    sendMsg(_bytes, _methodName, callback) {
      callback({ meta: { error_code: 1001, error_message: "千星游记奖励已领取" } });
    },
  });
  const skipped = await failingGameCtl.claimQianXingTravelRewards({ silent: true, waitMs: 20 });
  assert.strictEqual(skipped.ok, true);
  assert.strictEqual(skipped.skipped, true);
  assert.strictEqual(skipped.reason, "already_claimed");

  console.log("[qian-xing-travel-protocol] empty request, current service, and already-claimed skip are handled");
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
