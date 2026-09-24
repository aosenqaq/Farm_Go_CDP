const assert = require('assert');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');
const buttonPath = path.join(root, 'resources', 'wmpf', 'button.js');
const source = fs.readFileSync(buttonPath, 'utf8');

const context = {
  console: { dir() {}, log() {}, warn() {}, error() {} },
  setTimeout,
  clearTimeout,
  Uint8Array,
  ArrayBuffer,
  TextDecoder,
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
context.FriendManager = {
  ins: {
    bReqFriendListed: true,
    sortClientFriendList() {},
    getClientFriendListExcludeSelf() {
      return [
        { gid: 10002, name: 'Beta', level: 8, rank: 2, plant: { steal_plant_num: 1 } },
        { gid: 10001, name: 'Alpha', level: 9, rank: 1, is_banned: true, plant: { steal_plant_num: 3 } },
      ];
    },
  },
};

vm.runInNewContext(source, context, { filename: buttonPath });

assert.strictEqual(
  typeof context.gameCtl.getGodRankList,
  'function',
  'gameCtl.getGodRankList should be exported for the social god-rank action'
);

const payload = context.gameCtl.getGodRankList({ silent: true });
assert.strictEqual(payload.ok, true);
assert.strictEqual(payload.action, 'get_god_rank_list');
assert.deepStrictEqual(
  payload.list.map((row) => row.gid),
  [10001],
  'god rank payload should include only friends explicitly marked is_banned'
);
assert.strictEqual(payload.list[0].isBanned, true);

console.log('[god-rank-method] gameCtl.getGodRankList is available');
