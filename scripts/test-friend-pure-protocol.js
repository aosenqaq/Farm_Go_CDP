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
  ].join("\n")
);
source = source.replace(
  "  function resolveOops() {",
  "  function resolveOops() { if (G.__testOops) return G.__testOops;"
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

assert.deepStrictEqual(Array.from(context.__testEncodeFarmVarint(10001)), [145, 78]);
assert.deepStrictEqual(Array.from(context.__testBuildVisitEnterRequestBytes(10001, 2)), [8, 145, 78, 16, 2]);
assert.deepStrictEqual(Array.from(context.__testBuildCheckCanOperateRequestBytes(10001, 10004)), [8, 145, 78, 16, 148, 78]);
assert.deepStrictEqual(Array.from(context.__testBuildHarvestRequestBytes(10001, [3, 9], false)), [10, 2, 3, 9, 16, 145, 78, 24, 0]);
assert.deepStrictEqual(Array.from(context.__testBuildHarvestRequestBytes(10001, [0, null, 3, -1, 3], false)), [10, 1, 3, 16, 145, 78, 24, 0]);
assert.throws(() => context.__testBuildHarvestRequestBytes(10001, [0, null, -1], false), /landIds required/);
assert.deepStrictEqual(Array.from(context.__testBuildFarmingRequestBytes(10001, [3, 9], 0)), [10, 2, 3, 9, 16, 145, 78, 24, 0]);

const dogInfo = context.__testParseBriefDogInfoBytes([8, 165, 191, 5, 16, 1, 24, 60]);
assert.strictEqual(dogInfo.dogId, 90021);
assert.strictEqual(dogInfo.dogName, "护主犬");
assert.strictEqual(dogInfo.dogLevel, 1);
assert.strictEqual(dogInfo.duration, 60);

let sentMessage = null;
context.__testOops = {
  netWebSocket: {
    sendMsg(request, methodName, callback, serviceName) {
      sentMessage = {
        request: Array.from(request),
        methodName,
        callbackType: typeof callback,
        serviceName,
      };
    },
  },
};

async function loadGameCtlWithNet(netWebSocket, qq) {
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
  localContext.__testOops = { netWebSocket };
  localContext.qq = qq || null;
  localContext.cc = {
    Button: function Button() {},
    Component: function Component() {},
    Node: function Node() {},
    director: { getScene() { return null; } },
    find() { return null; },
    game: { canvas: {} },
  };
  vm.runInNewContext(source, localContext, { filename: buttonPath, timeout: 5000 });
  return localContext.gameCtl;
}

function encVarint(value) {
  let n = Number(value) || 0;
  const out = [];
  while (n >= 0x80) {
    out.push((n & 0x7f) | 0x80);
    n = Math.floor(n / 0x80);
  }
  out.push(n);
  return out;
}

function protoVarint(field, value) {
  return encVarint((field * 8) + 0).concat(encVarint(value));
}

function protoBytes(field, bytes) {
  const list = Array.from(bytes || []);
  return encVarint((field * 8) + 2).concat(encVarint(list.length), list);
}

function protoText(field, text) {
  return protoBytes(field, Array.from(new TextEncoder().encode(text)));
}

async function main() {
  const checkResult = await context.gameCtl.checkFriendCanOperateByProtocol({
    hostGid: 10001,
    operationId: 10004,
    waitReplyMs: 0,
    silent: true,
  });
  assert.deepStrictEqual(sentMessage, {
    request: [8, 145, 78, 16, 148, 78],
    methodName: "CheckCanOperate",
    callbackType: "function",
    serviceName: "gamepb.plantpb.PlantService",
  });
  assert.strictEqual(checkResult.callbackCalled, false);
  assert.strictEqual(checkResult.ok, false);

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
  assert.deepStrictEqual(Array.from(summary.workLandIds.collect), [3]);
  assert.deepStrictEqual(Array.from(summary.workLandIds.farming), [9]);
  assert.strictEqual(calls[0].methodName, "Enter");
  assert.strictEqual(calls[0].serviceName, "gamepb.visitpb.VisitService");
  assert.strictEqual(calls[calls.length - 1].methodName, "Leave");

  const slimCalls = [];
  const slimGameCtl = await loadGameCtlWithNet({
    sendMsg(bytes, methodName, callback, serviceName) {
      slimCalls.push({ bytes: Array.from(bytes), methodName, serviceName });
      if (methodName === "Leave") {
        throw new Error("leave unavailable");
      }
      callback({
        basic: { gid: 10001, name: "A" },
        brief_dog_info: Uint8Array.from([8, 165, 191, 5, 16, 1, 24, 60]),
        lands: [
          { id: 3, plant: { id: 2001, phases: [{ phase: 6 }], stealable: true } },
          { id: 9, plant: { id: 2002, phases: [{ phase: 4, dry_time: 1 }], dry_num: 1, weed_owners: [], insect_owners: [] } },
        ],
      });
    },
  });
  const slimSummary = await slimGameCtl.inspectFriendFarmByProtocol({
    hostGid: 10001,
    silent: true,
    leaveAfter: true,
    includeLands: false,
  });
  assert.strictEqual(slimSummary.ok, true);
  assert.deepStrictEqual(Array.from(slimSummary.lands), []);
  assert.strictEqual(slimSummary.visit.lands, undefined);
  assert.strictEqual(slimSummary.visit.rawReply, undefined);
  assert.strictEqual(slimSummary.visit.dispatch, undefined);
  assert.strictEqual(slimSummary.leave.ok, false);
  assert.strictEqual(slimSummary.leave.callbackError, "leave unavailable");
  assert.strictEqual(slimCalls[slimCalls.length - 1].methodName, "Leave");

  const camelGameCtl = await loadGameCtlWithNet({
    sendMsg(_bytes, methodName, callback) {
      if (methodName === "Enter") {
        callback({
          basic: { gid: 10001, name: "Camel" },
          briefDogInfo: { dogId: 90021, dogName: "护主犬", dogLevel: 2, duration: 120 },
          lands: [
            { landId: 4, plantInfo: { id: 2003, phase: 6, canSteal: true } },
            { landId: 5, plantInfo: { id: 2004, phase: 4, dryNum: 1, weedOwners: [888], insectOwners: [777] } },
          ],
        });
        return;
      }
      callback({});
    },
  });
  const camelSummary = await camelGameCtl.inspectFriendFarmByProtocol({ hostGid: 10001, silent: true });
  assert.strictEqual(camelSummary.briefDogInfo.dogId, 90021);
  assert.strictEqual(camelSummary.briefDogInfo.dogLevel, 2);
  assert.deepStrictEqual(Array.from(camelSummary.workLandIds.collect), [4]);
  assert.deepStrictEqual(Array.from(camelSummary.workLandIds.farming), [5]);
  assert.deepStrictEqual(Array.from(camelSummary.workLandIds.eraseGrass), [5]);
  assert.deepStrictEqual(Array.from(camelSummary.workLandIds.killBug), [5]);

  const nestedGameCtl = await loadGameCtlWithNet({
    sendMsg(_bytes, methodName, callback) {
      if (methodName === "Enter") {
        callback(null, {
          data: {
            basic: { gid: 10001, name: "Nested" },
            briefDogInfo: { dogId: 90021, dogName: "护主犬" },
            lands: [
              { landId: 6, plantInfo: { phase: 6, canHarvest: 1 } },
            ],
          },
        });
        return;
      }
      callback({});
    },
  });
  const nestedSummary = await nestedGameCtl.inspectFriendFarmByProtocol({ hostGid: 10001, silent: true });
  assert.strictEqual(nestedSummary.friend.name, "Nested");
  assert.deepStrictEqual(Array.from(nestedSummary.workLandIds.collect), [6]);

  const wireBasic = []
    .concat(protoVarint(1, 10001))
    .concat(protoText(2, "Wire"))
    .concat(protoVarint(3, 88));
  const wireMaturePlant = []
    .concat(protoVarint(1, 2003))
    .concat(protoText(2, "Mature"));
  const wireGrowingPlant = []
    .concat(protoVarint(1, 2004))
    .concat(protoText(2, "Growing"))
    .concat(protoBytes(20, protoVarint(1, 1)));
  const wireMatureLand = []
    .concat(protoVarint(1, 4))
    .concat(protoVarint(3, 5))
    .concat(protoBytes(10, wireMaturePlant))
    .concat(protoVarint(16, 5));
  const wireGrowingLand = []
    .concat(protoVarint(1, 5))
    .concat(protoVarint(3, 4))
    .concat(protoBytes(10, wireGrowingPlant))
    .concat(protoVarint(16, 4));
  const wireReply = []
    .concat(protoBytes(1, wireBasic))
    .concat(protoBytes(2, wireMatureLand))
    .concat(protoBytes(2, wireGrowingLand))
    .concat(protoBytes(3, [8, 165, 191, 5, 16, 2, 24, 120]));
  const wireGameCtl = await loadGameCtlWithNet({
    sendMsg(_bytes, methodName, callback) {
      if (methodName === "Enter") {
        callback({ meta: { method_name: "Enter" }, body: Uint8Array.from(wireReply) });
        return;
      }
      callback({});
    },
  });
  const wireSummary = await wireGameCtl.inspectFriendFarmByProtocol({ hostGid: 10001, silent: true });
  assert.strictEqual(wireSummary.friend.name, "Wire");
  assert.strictEqual(wireSummary.briefDogInfo.dogId, 90021);
  assert.deepStrictEqual(Array.from(wireSummary.workLandIds.collect), [4]);
  assert.deepStrictEqual(Array.from(wireSummary.workLandIds.farming), [5]);
  assert.deepStrictEqual(Array.from(wireSummary.workLandIds.water), [5]);
  assert.deepStrictEqual(Array.from(wireSummary.workLandIds.grass), [5]);
  assert.deepStrictEqual(Array.from(wireSummary.workLandIds.bug), [5]);

  const timedGameCtl = await loadGameCtlWithNet({
    sendMsg(_bytes, methodName, callback) {
      if (methodName === "Enter") {
        callback({
          basic: { gid: 10001, name: "Timed" },
          unix_milli: 100000,
          lands: [
            {
              id: 7,
              plant: {
                id: 2005,
                stealable: true,
                phases: [
                  { phase: 2, begin_time: 90, dry_time: 95, weeds_time: 96, insect_time: 97 },
                  { phase: 6, begin_time: 200 },
                ],
              },
            },
            {
              id: 8,
              plant: {
                id: 2006,
                stealable: true,
                phases: [
                  { phase: 6, begin_time: 50 },
                ],
              },
            },
          ],
        });
        return;
      }
      callback({});
    },
  });
  const timedSummary = await timedGameCtl.inspectFriendFarmByProtocol({ hostGid: 10001, silent: true });
  assert.deepStrictEqual(Array.from(timedSummary.workLandIds.collect), [8]);
  assert.deepStrictEqual(Array.from(timedSummary.workLandIds.farming), [7]);
  assert.deepStrictEqual(Array.from(timedSummary.workLandIds.water), [7]);
  assert.deepStrictEqual(Array.from(timedSummary.workLandIds.eraseGrass), [7]);
  assert.deepStrictEqual(Array.from(timedSummary.workLandIds.killBug), [7]);

  const mischiefCalls = [];
  const mischiefGameCtl = await loadGameCtlWithNet({
    sendMsg(bytes, methodName, callback, serviceName) {
      mischiefCalls.push({ bytes: Array.from(bytes), methodName, serviceName });
      callback({});
    },
  });
  const mischiefResult = await mischiefGameCtl.friendMischiefLandsBatch({
    hostGid: 10001,
    grassLandIds: [5],
    bugLandIds: [7],
    dryRun: false,
    silent: true,
    waitReplyMs: 0,
    waitAfterAction: 100,
  });
  assert.strictEqual(mischiefResult.ok, true);
  assert.strictEqual(mischiefResult.processedCount, 2);
  assert.strictEqual(mischiefResult.skippedCount, 0);
  assert.deepStrictEqual(mischiefCalls.map((item) => item.methodName), ["PutWeeds", "PutInsects"]);
  assert.deepStrictEqual(mischiefCalls.map((item) => item.serviceName), ["gamepb.plantpb.PlantService", "gamepb.plantpb.PlantService"]);

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
  assert.deepStrictEqual(Array.from(addFriendResult.callbackEvents, (item) => item.kind), ["success"]);

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

  console.log("[friend-pure-protocol] byte builders pass");
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
