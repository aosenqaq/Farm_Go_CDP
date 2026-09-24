const assert = require('assert');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');
const buttonPath = path.join(root, 'resources', 'wmpf', 'button.js');
let source = fs.readFileSync(buttonPath, 'utf8');

source = source.replace(
  '  G.gameCtl = {',
  '  G.__testExtractProtocolFailureText = extractProtocolFailureText;\n  G.__testSelectFriendMischiefDispatchError = selectFriendMischiefDispatchError;\n  G.gameCtl = {'
);

source = source.replace(
  '  G.__testExtractProtocolFailureText = extractProtocolFailureText;',
  '  G.__testExtractProtocolFailureText = extractProtocolFailureText;\n  G.__testFindProtocolPayloadFailureReason = findProtocolPayloadFailureReason;\n  G.__testIsProtocolAlreadyClaimedReason = isProtocolAlreadyClaimedReason;'
);

const context = {
  console: { dir() {}, log() {}, warn() {}, error() {} },
  setTimeout,
  clearTimeout,
  Uint8Array,
  ArrayBuffer,
  TextDecoder,
  decodeURIComponent,
  escape,
  Date,
  Promise,
  Map,
  Set,
};

context.globalThis = context;
context.cc = {
  game: { canvas: null },
  director: { getScene: () => null },
  find: () => null,
  Node: function Node() {},
  Component: function Component() {},
  Button: function Button() {},
};

vm.runInNewContext(source, context, { filename: buttonPath });

const extract = context.__testExtractProtocolFailureText;
assert.strictEqual(typeof extract, 'function');
const selectDispatchError = context.__testSelectFriendMischiefDispatchError;
assert.strictEqual(typeof selectDispatchError, 'function');
const findProtocolPayloadFailureReason = context.__testFindProtocolPayloadFailureReason;
assert.strictEqual(typeof findProtocolPayloadFailureReason, 'function');
const isProtocolAlreadyClaimedReason = context.__testIsProtocolAlreadyClaimedReason;
assert.strictEqual(typeof isProtocolAlreadyClaimedReason, 'function');

assert.strictEqual(
  extract([
    { error: 'rpc error: code = Internal desc = protocol dispatch failed' },
    { response: { toast: '今日捣乱次数已达上限' } },
  ]),
  '今日捣乱次数已达上限'
);

assert.strictEqual(
  extract([
    { message: 'rpc error: code = Unknown desc = 今日捣乱次数已达上限' },
  ]),
  '今日捣乱次数已达上限'
);

assert.strictEqual(
  selectDispatchError({
    callbackError: 'rpc error: code = Unknown',
    failureText: '今日捣乱次数已达上限',
    callbackCalled: true,
  }),
  '今日捣乱次数已达上限'
);

const alreadyClaimedReason = findProtocolPayloadFailureReason([
  { meta: { error_code: 1009001, error_message: '分享奖励已经领取' } },
]);
assert.strictEqual(alreadyClaimedReason, '分享奖励已经领取');
assert.strictEqual(isProtocolAlreadyClaimedReason(alreadyClaimedReason), true);

const mallLimitReason = findProtocolPayloadFailureReason([
  { meta: { error_code: 1009002, error_message: '限购次数已用完' } },
]);
assert.strictEqual(mallLimitReason, '限购次数已用完');
assert.strictEqual(isProtocolAlreadyClaimedReason(mallLimitReason), true);

const mallFunctionMatch = source.match(/async function claimMallDailyFertilizerGift\(opts\) \{[\s\S]*?\n  \}/);
assert.ok(mallFunctionMatch, 'claimMallDailyFertilizerGift function should exist');
assert.ok(
  mallFunctionMatch[0].includes('let callbackRaw = null;'),
  'claimMallDailyFertilizerGift should define callbackRaw in function scope'
);
assert.ok(
  mallFunctionMatch[0].includes('let callbackCalled = false;'),
  'claimMallDailyFertilizerGift should define callbackCalled in function scope'
);
