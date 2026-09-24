const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const root = path.join(__dirname, '..');
let source = fs.readFileSync(path.join(root, 'resources', 'wmpf', 'button.js'), 'utf8');

source = source.replace(
  '  G.gameCtl = {',
  '  G.__testIsDeadShovelTarget = isDeadShovelTarget;\n  G.__testIsGridShovelObserved = isGridShovelObserved;\n  G.gameCtl = {',
);

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
context.System = { get() { return { oops: {} }; } };

vm.createContext(context);
vm.runInContext(source, context, { filename: 'button.js', timeout: 5000 });

const isDeadShovelTarget = context.__testIsDeadShovelTarget;
const isGridShovelObserved = context.__testIsGridShovelObserved;
assert.strictEqual(typeof isDeadShovelTarget, 'function');
assert.strictEqual(typeof isGridShovelObserved, 'function');

const actionSets = { eraseDead: new Set([8]) };
assert.strictEqual(
  isDeadShovelTarget({ stageKind: 'growing', isDead: false, canEraseDead: false }, actionSets, 8),
  true,
  'manager eraseDead membership must survive the shovel safety check',
);
assert.strictEqual(
  isDeadShovelTarget({ stageKind: 'dead', isDead: true }, null, 9),
  true,
  'direct runtime dead flags must remain supported',
);
assert.strictEqual(
  isDeadShovelTarget({ stageKind: 'growing', isDead: false, canEraseDead: false }, actionSets, 9),
  false,
  'healthy lands outside eraseDead must remain protected',
);
assert.strictEqual(
  isGridShovelObserved(
    { landId: 8, hasPlant: false, stageKind: 'empty' },
    { landId: 8, hasPlant: false, stageKind: 'empty' },
    actionSets,
    { eraseDead: new Set() },
    8,
  ),
  true,
  'removal from eraseDead must confirm shovel success when plant runtime is absent',
);
assert.strictEqual(
  isGridShovelObserved(
    { landId: 8, hasPlant: false, stageKind: 'empty' },
    { landId: 8, hasPlant: false, stageKind: 'empty' },
    actionSets,
    actionSets,
    8,
  ),
  false,
  'a persistent eraseDead member must not be reported as shoveled',
);

const batchStart = source.indexOf('  async function shovelLandsBatch(opts) {');
const batchEnd = source.indexOf('\n  function findShovelConfirmButton()', batchStart);
assert.ok(batchStart >= 0 && batchEnd > batchStart, 'shovelLandsBatch source should be found');
const batchSource = source.slice(batchStart, batchEnd);
assert.match(batchSource, /getFarmWorkSummary/, 'shovel batch must read the current work summary');
assert.match(batchSource, /actionSets/, 'shovel batch must carry action sets into state validation');
assert.match(batchSource, /isDeadShovelTarget/, 'shovel batch must use the shared dead predicate');
assert.match(batchSource, /afterWorkSummary/, 'shovel batch must refresh dead state after dispatch');
assert.ok(
  batchSource.indexOf("onlyDead && !isDeadShovelTarget") < batchSource.indexOf("before.hasPlant !== true"),
  'onlyDead validation must use manager state before rejecting a missing plant runtime',
);

console.log('[shovel-dead-action-sets] manager dead state reaches shovel validation');
