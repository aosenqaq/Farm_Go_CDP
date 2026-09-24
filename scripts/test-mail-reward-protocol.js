const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const root = path.join(__dirname, "..");
const buttonJs = fs.readFileSync(path.join(root, "resources", "wmpf", "button.js"), "utf8");

async function loadGameCtlWithNet(netWebSocket, protobufDefault) {
  const oops = { netWebSocket, protobufDefault };
  const context = {
    ArrayBuffer,
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
      return { oops };
    },
  };
  vm.createContext(context);
  vm.runInContext(buttonJs, context, { filename: "button.js", timeout: 5000 });
  return context.gameCtl;
}

(async () => {
  const snakeCaseCalls = [];
  const snakeCaseNetWebSocket = {
    sendMsg(bytes, methodName, callback, serviceName) {
      snakeCaseCalls.push({ bytes: Array.from(bytes), methodName, serviceName });
      if (methodName === "GetEmailList") {
        callback({
          emails: [
            { email_id: "serverWideMail123", can_claim: true },
          ],
        });
        return;
      }
      callback({ ok: true });
    },
  };

  const snakeCaseGameCtl = await loadGameCtlWithNet(snakeCaseNetWebSocket);
  const snakeCaseResult = await snakeCaseGameCtl.claimMailRewardsByProtocol({
    silent: true,
    types: [1],
  });

  assert.strictEqual(snakeCaseResult.claimedCount, 1, "snake_case email ids should be claimed");
  assert.deepStrictEqual(Array.from(snakeCaseResult.typeResults[0].ids), ["serverWideMail123"]);
  assert.ok(
    snakeCaseCalls.some((call) => call.methodName === "BatchClaimEmail"),
    "BatchClaimEmail should be sent after reading claimable mail",
  );

  const failingClaimNetWebSocket = {
    sendMsg(_bytes, methodName, callback) {
      if (methodName === "GetEmailList") {
        callback({ emails: [{ email_id: "serverWideMail456", can_claim: true }] });
        return;
      }
      callback({ ok: false, error_message: "邮件奖励领取失败" });
    },
  };

  const failingClaimGameCtl = await loadGameCtlWithNet(failingClaimNetWebSocket);
  const failingClaimResult = await failingClaimGameCtl.claimMailRewardsByProtocol({
    silent: true,
    types: [1],
  });

  assert.strictEqual(failingClaimResult.ok, false, "failed claim callbacks should fail the mail task");
  assert.strictEqual(failingClaimResult.claimedCount, 0, "failed claims must not be counted as claimed");
  assert.match(failingClaimResult.typeResults[0].reason, /邮件奖励领取失败/);

  const mixedFieldCalls = [];
  const mixedFieldNetWebSocket = {
    sendMsg(bytes, methodName, callback) {
      mixedFieldCalls.push({ bytes: Array.from(bytes), methodName });
      if (methodName === "GetEmailList" && bytes[1] === 1) {
        callback({ emails: [] });
        return;
      }
      if (methodName === "GetEmailList") {
        callback({ emails: [{ emailID: 987654321, canClaim: true }] });
        return;
      }
      callback({ success: true });
    },
  };
  const mixedFieldGameCtl = await loadGameCtlWithNet(mixedFieldNetWebSocket);
  const mixedFieldResult = await mixedFieldGameCtl.claimMailRewardsByProtocol({
    silent: true,
    types: [1, 2],
  });
  assert.strictEqual(mixedFieldResult.ok, true, "a later mail type should still be claimed");
  assert.strictEqual(mixedFieldResult.claimedCount, 1);
  assert.deepStrictEqual(Array.from(mixedFieldResult.typeResults[1].ids), ["987654321"]);

  const listFailureCalls = [];
  const listFailureGameCtl = await loadGameCtlWithNet({
    sendMsg(_bytes, methodName, callback) {
      listFailureCalls.push(methodName);
      callback({ ok: false, error_message: "邮件列表读取失败" });
    },
  });
  const listFailureResult = await listFailureGameCtl.claimMailRewardsByProtocol({
    silent: true,
    types: [1],
  });
  assert.strictEqual(listFailureResult.ok, false, "mail list failures must not be reported as no mail");
  assert.match(listFailureResult.reason, /邮件列表读取失败/);
  assert.deepStrictEqual(listFailureCalls, ["GetEmailList"], "claim must not run after list failure");

  function fakeMessageType(fieldsArray, encode, decode) {
    return {
      fieldsArray,
      create(value) { return value; },
      encode(value) {
        return { finish() { return Uint8Array.from(encode(value)); } };
      },
      decode: decode || (() => ({})),
    };
  }
  const reflectedCalls = [];
  const protobufDefault = {
    gamepb: {
      emailpb: {
        GetEmailListRequest: fakeMessageType(
          [{ name: "emailType", id: 7, type: "int32", repeated: false }],
          (value) => [56, value.emailType],
        ),
        GetEmailListReply: fakeMessageType([], () => [], () => ({
          emails: [{ mailID: "mail-uuid-550e8400-e29b-41d4-a716-446655440000", canClaim: true }],
        })),
        BatchClaimEmailRequest: fakeMessageType(
          [
            { name: "emailIDs", id: 1, type: "string", repeated: true },
            { name: "emailType", id: 3, type: "int32", repeated: false },
          ],
          (value) => {
            const idBytes = Array.from(new TextEncoder().encode(value.emailIDs[0]));
            return [10, idBytes.length, ...idBytes, 24, value.emailType];
          },
        ),
        BatchClaimEmailReply: fakeMessageType([], () => [], () => ({ success: true })),
      },
    },
  };
  const reflectedGameCtl = await loadGameCtlWithNet({
    sendMsg(bytes, methodName, callback) {
      reflectedCalls.push({ bytes: Array.from(bytes), methodName });
      callback(Uint8Array.from([1]));
    },
  }, protobufDefault);
  const reflectedResult = await reflectedGameCtl.claimMailRewardsByProtocol({
    silent: true,
    types: [2],
  });
  assert.strictEqual(reflectedResult.ok, true);
  assert.strictEqual(reflectedResult.claimedCount, 1);
  assert.deepStrictEqual(reflectedCalls[0], { methodName: "GetEmailList", bytes: [56, 2] });
  assert.strictEqual(reflectedCalls[1].methodName, "BatchClaimEmail");
  assert.deepStrictEqual(reflectedCalls[1].bytes.slice(-2), [24, 2]);

  const callbackEnvelopeCalls = [];
  const callbackEnvelopeGameCtl = await loadGameCtlWithNet({
    sendMsg(bytes, methodName, callback) {
      callbackEnvelopeCalls.push({ bytes: Array.from(bytes), methodName });
      callback({
        meta: { error_code: 0 },
        body: { 0: 1 },
      });
    },
  }, protobufDefault);
  const callbackEnvelopeResult = await callbackEnvelopeGameCtl.claimMailRewardsByProtocol({
    silent: true,
    types: [2],
  });
  assert.strictEqual(
    callbackEnvelopeResult.claimedCount,
    1,
    "QQ callback envelopes with numeric-object body bytes should be decoded and claimed",
  );
  assert.deepStrictEqual(
    Array.from(callbackEnvelopeResult.typeResults[0].ids),
    ["mail-uuid-550e8400-e29b-41d4-a716-446655440000"],
  );
  assert.ok(
    callbackEnvelopeCalls.some((call) => call.methodName === "BatchClaimEmail"),
    "BatchClaimEmail should be sent for claimable mail decoded from callback.body",
  );

  const alreadyClaimedId = "serverWideMail999";
  const alreadyClaimedIdBytes = Array.from(new TextEncoder().encode(alreadyClaimedId));
  const alreadyClaimedBodyBytes = [10, alreadyClaimedIdBytes.length, ...alreadyClaimedIdBytes];
  const alreadyClaimedBody = Object.fromEntries(
    alreadyClaimedBodyBytes.map((byte, index) => [String(index), byte]),
  );
  const alreadyClaimedProtobufDefault = {
    gamepb: {
      emailpb: {
        ...protobufDefault.gamepb.emailpb,
        GetEmailListReply: fakeMessageType([], () => [], () => ({
          emails: [{ email_id: alreadyClaimedId, is_claimed: true }],
        })),
      },
    },
  };
  const alreadyClaimedCalls = [];
  const alreadyClaimedGameCtl = await loadGameCtlWithNet({
    sendMsg(bytes, methodName, callback) {
      alreadyClaimedCalls.push({ bytes: Array.from(bytes), methodName });
      callback({ meta: { error_code: 0 }, body: alreadyClaimedBody });
    },
  }, alreadyClaimedProtobufDefault);
  const alreadyClaimedResult = await alreadyClaimedGameCtl.claimMailRewardsByProtocol({
    silent: true,
    types: [2],
  });
  assert.strictEqual(
    alreadyClaimedResult.claimedCount,
    0,
    "raw callback bytes must not re-add ids that the decoded mail marks as already claimed",
  );
  assert.deepStrictEqual(
    alreadyClaimedCalls.map((call) => call.methodName),
    ["GetEmailList"],
    "already claimed decoded mail must not trigger BatchClaimEmail",
  );

  const partialClaimIds = ["serverWideMail701", "serverWideMail702"];
  const partialClaimProtobufDefault = {
    gamepb: {
      emailpb: {
        ...protobufDefault.gamepb.emailpb,
        GetEmailListReply: fakeMessageType([], () => [], () => ({
          emails: partialClaimIds.map((email_id) => ({ email_id, has_reward: true })),
        })),
        BatchClaimEmailReply: fakeMessageType([], () => [], () => ({
          success: true,
          unclaimed_email_ids: [partialClaimIds[1]],
        })),
      },
    },
  };
  const partialClaimGameCtl = await loadGameCtlWithNet({
    sendMsg(_bytes, _methodName, callback) {
      callback({ meta: { error_code: 0 }, body: { 0: 1 } });
    },
  }, partialClaimProtobufDefault);
  const partialClaimResult = await partialClaimGameCtl.claimMailRewardsByProtocol({
    silent: true,
    types: [2],
  });
  assert.strictEqual(
    partialClaimResult.claimedCount,
    1,
    "server-reported unclaimed_email_ids must not be counted as successfully claimed",
  );
  assert.deepStrictEqual(
    Array.from(partialClaimResult.typeResults[0].unclaimedIds),
    [partialClaimIds[1]],
  );

  const noClaimableGameCtl = await loadGameCtlWithNet({
    sendMsg(_bytes, methodName, callback) {
      if (methodName === "GetEmailList") {
        callback({ emails: [{ email_id: "serverWideMail703", has_reward: true }] });
        return;
      }
      callback({ meta: { error_code: 1001, error_message: "没有可领取奖励的邮件" } });
    },
  });
  const noClaimableResult = await noClaimableGameCtl.claimMailRewardsByProtocol({
    silent: true,
    types: [2],
  });
  assert.strictEqual(noClaimableResult.ok, true, "an idempotent no-claimable reply should not fail automation");
  assert.strictEqual(noClaimableResult.claimedCount, 0);
  assert.strictEqual(noClaimableResult.reason, "no_claimable_mail");

  console.log("[mail-reward-protocol] list decoding, reflected schemas, callback envelopes, claimed-state filtering, partial claims, ids, and failures are handled");
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
