const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const root = path.join(__dirname, '..');
const source = fs.readFileSync(path.join(root, 'resources', 'wmpf', 'button.js'), 'utf8');

function loadGameCtl() {
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
  return context.gameCtl;
}

(async () => {
  const gameCtl = loadGameCtl();
  assert.strictEqual(typeof gameCtl.waitForLinkedHarvestReadiness, 'function');

  const reads = new Map();
  const result = await gameCtl.waitForLinkedHarvestReadiness(
    [{ landId: 1, node: { id: 1 } }, { landId: 2, node: { id: 2 } }],
    {
      initialDelayMs: 0,
      pollIntervalMs: 1,
      timeoutMs: 80,
      readState(entry) {
        const count = (reads.get(entry.landId) || 0) + 1;
        reads.set(entry.landId, count);
        if (entry.landId === 1 || count >= 3) {
          return { hasPlant: true, stageKind: 'mature', matureInSec: 0, canHarvest: true };
        }
        return { hasPlant: true, stageKind: 'growing', matureInSec: 1, canHarvest: false };
      },
    },
  );

  assert.deepStrictEqual(Array.from(result.readyLandIds), [1, 2]);
  assert.deepStrictEqual(Array.from(result.pendingLandIds), []);
  assert.ok(result.attempts >= 3, 'delayed land should be polled until mature');

  const timeoutStartedAt = Date.now();
  const timeoutResult = await gameCtl.waitForLinkedHarvestReadiness(
    [{ landId: 3, node: { id: 3 } }],
    {
      initialDelayMs: 0,
      pollIntervalMs: 2,
      timeoutMs: 12,
      readState() {
        return { hasPlant: true, stageKind: 'growing', matureInSec: 30 };
      },
    },
  );
  assert.deepStrictEqual(Array.from(timeoutResult.readyLandIds), []);
  assert.deepStrictEqual(Array.from(timeoutResult.pendingLandIds), [3]);
  assert.ok(Date.now() - timeoutStartedAt < 200, 'readiness timeout must remain bounded');

  console.log('[fertilizer-linked-harvest] readiness polling handles delayed maturity and timeout');
})().catch((error) => {
  console.error(error);
  process.exit(1);
});
