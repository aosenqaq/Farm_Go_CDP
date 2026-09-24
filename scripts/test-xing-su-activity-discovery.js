const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const root = path.join(__dirname, "..");
const buttonJs = fs.readFileSync(path.join(root, "resources", "wmpf", "button.js"), "utf8");

function loadGameCtl() {
  const calls = [];
  const netWebSocket = {
    sendMsg(...args) {
      calls.push(args);
    },
  };
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
    registry: new Map([[
      "chunks:///_virtual/activitypb.ts",
      {
        namespace: {
          gamepb: {
            activitypb: {
              ActivityService: {
                GetActivities: {},
                Operate: {},
                create() { return null; },
              },
              GetActivitiesReply: {},
            },
          },
        },
      },
    ], [
      "chunks:///_virtual/ActivityManager.ts",
      {
        namespace: {
          ActivityManager: class ActivityManager {
            listActivities() { return null; }
            requestAllActivity() { return null; }
          },
        },
      },
    ]]),
  };
  vm.createContext(context);
  vm.runInContext(buttonJs, context, { filename: "button.js", timeout: 5000 });
  return { calls, gameCtl: context.gameCtl };
}

const { calls, gameCtl } = loadGameCtl();
const result = gameCtl.inspectXingSuActivityProtocolRuntime({ silent: true });

assert.strictEqual(result.ok, true);
assert.strictEqual(result.registryEntryCount, 2);
assert.deepStrictEqual(
  Array.from(result.systemSummaries.map((summary) => summary.source)),
  ["System"],
);
assert.deepStrictEqual(
  Array.from(result.systemSummaries[0].containers.map((container) => container.source)),
  ["System.entries", "System.registry", "System._loader.modules", "System._loader.moduleRecords", "System._loader.registry"],
);
assert.strictEqual(result.systemSummaries[0].containers[1].entryCount, 2);
assert.deepStrictEqual(Array.from(result.moduleMatches[0].exports), ["gamepb"]);
assert.deepStrictEqual(Array.from(result.moduleMatches[0].gamePbExports), ["activitypb"]);
assert.strictEqual(result.moduleMatches[0].activityExports.includes("ActivityService"), true);
assert.deepStrictEqual(
  Array.from(result.moduleMatches[0].activityServiceExports),
  ["GetActivities", "Operate", "create"],
);
assert.match(result.moduleMatches[0].activityServiceCreateSource, /create/);
assert.strictEqual(result.activityManager.moduleId, "chunks:///_virtual/ActivityManager.ts");
assert.strictEqual(result.activityManager.methodNames.includes("listActivities"), true);
assert.match(result.activityManager.methodSources.requestAllActivity, /requestAllActivity/);
assert.deepStrictEqual(calls, []);

console.log("[xing-su-activity-discovery] named activity module discovery is read-only");
