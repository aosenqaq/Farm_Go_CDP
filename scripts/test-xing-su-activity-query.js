const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const root = path.join(__dirname, "..");
const buttonJs = fs.readFileSync(path.join(root, "resources", "wmpf", "button.js"), "utf8");

function loadGameCtl(netWebSocket, replyCodec) {
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
      { namespace: { gamepb: { activitypb: { ListReply: replyCodec } } } },
    ]]),
  };
  vm.createContext(context);
  vm.runInContext(buttonJs, context, { filename: "button.js", timeout: 5000 });
  return context.gameCtl;
}

(async () => {
  const calls = [];
  const replyCodec = {
    decode(bytes) {
      assert.deepStrictEqual(Array.from(bytes), [10, 0]);
      return { activities: [{ id: "2026072901", status: 1 }] };
    },
    toObject(value) { return value; },
  };
  const gameCtl = loadGameCtl({
    sendMsg(bytes, methodName, callback, serviceName) {
      calls.push({ bytes: Array.from(bytes), methodName, serviceName });
      callback({
        meta: { error_code: 0 },
        body: Uint8Array.from([10, 0]),
        activity_description: "单人邀新上限10人，达上限后无额外奖励",
      });
    },
  }, replyCodec);

  const result = await gameCtl.requestXingSuActivityStatus({ silent: true, waitMs: 0 });
  assert.strictEqual(result.ok, true);
  assert.strictEqual(result.callbackCalled, true);
  assert.deepStrictEqual(result.decoded, { activities: [{ id: "2026072901", status: 1 }] });
  assert.deepStrictEqual(calls, [{
    bytes: [],
    methodName: "List",
    serviceName: "gamepb.activitypb.ActivityService",
  }]);
  assert.strictEqual(calls.some((call) => call.methodName === "Operate"), false);

  const failedCalls = [];
  const failedGameCtl = loadGameCtl({
    sendMsg(bytes, methodName, callback, serviceName) {
      failedCalls.push({ bytes: Array.from(bytes), methodName, serviceName });
      callback({ meta: { error_code: 400, error_message: "活动查询失败" } });
    },
  }, replyCodec);
  const failedResult = await failedGameCtl.requestXingSuActivityStatus({ silent: true, waitMs: 0 });
  assert.strictEqual(failedResult.ok, false);
  assert.match(failedResult.failureText, /活动查询失败/);
  assert.deepStrictEqual(failedCalls.map((call) => call.methodName), ["List"]);

  console.log("[xing-su-activity-query] List uses the named codec and never dispatches Operate");
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
