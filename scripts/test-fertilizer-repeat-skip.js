const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const root = path.resolve(__dirname, '..');
const buttonPath = path.join(root, 'resources', 'wmpf', 'button.js');
let source = fs.readFileSync(buttonPath, 'utf8');

source = source.replace(
  '  G.gameCtl = {',
  [
    '  G.__testFertilizerSeasonSkipKey = fertilizerSeasonSkipKey;',
    '  G.__testPartitionFertilizerSeasonSkips = partitionFertilizerSeasonSkips;',
    '  G.__testRememberFertilizerSeasonSkip = rememberFertilizerSeasonSkip;',
    '  G.__testBuildFertilizerProtocolFailureDiagnostics = buildFertilizerProtocolFailureDiagnostics;',
    '  G.__testDispatchFertilizerProtocolBatch = dispatchFertilizerProtocolBatch;',
    '  G.gameCtl = {',
  ].join('\n'),
);

const nativeFertilizerEvents = [];
const nativeFertilizerTimeline = [];
const testMessageBus = {
  dispatchEvent(name, payload) {
    nativeFertilizerTimeline.push(`dispatch:${payload && payload.land_id}`);
    nativeFertilizerEvents.push({
      name,
      payload: JSON.parse(JSON.stringify(payload)),
    });
    return payload;
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
  setTimeout,
};
context.globalThis = context;
context.System = {
  get(moduleID) {
    if (moduleID === 'chunks:///_virtual/Oops.ts' || moduleID === './Oops.ts') {
      return { oops: { message: testMessageBus } };
    }
    return null;
  },
};
context.cc = {
  Button: function Button() {},
  Component: function Component() {},
  Node: function Node() {},
  director: { getScene() { return null; } },
  find() { return null; },
  game: { canvas: null },
};

vm.runInNewContext(source, context, { filename: buttonPath, timeout: 5000 });

const seasonSkipKey = context.__testFertilizerSeasonSkipKey;
const partition = context.__testPartitionFertilizerSeasonSkips;
const remember = context.__testRememberFertilizerSeasonSkip;
const buildDiagnostics = context.__testBuildFertilizerProtocolFailureDiagnostics;
const dispatchFertilizerBatch = context.__testDispatchFertilizerProtocolBatch;

assert.strictEqual(typeof seasonSkipKey, 'function');
assert.strictEqual(typeof partition, 'function');
assert.strictEqual(typeof remember, 'function');
assert.strictEqual(typeof buildDiagnostics, 'function');
assert.strictEqual(typeof dispatchFertilizerBatch, 'function');

const land7 = { index: 0, landId: 7, before: { landId: 7, plantId: 1020003, currentSeason: 1 } };
const land8 = { index: 1, landId: 8, before: { landId: 8, plantId: 1020004, currentSeason: 1 } };

assert.deepStrictEqual(Array.from(partition([land7, land8]).pending, (entry) => entry.landId), [7, 8]);
remember(land7);
assert.deepStrictEqual(Array.from(partition([land7, land8]).skipped, (entry) => entry.landId), [7]);
assert.deepStrictEqual(
  Array.from(partition([{ ...land7, before: { ...land7.before, currentSeason: 2 } }]).pending, (entry) => entry.landId),
  [7],
);

const diagnostics = buildDiagnostics({
  landIds: [7, 8],
  resolvedMode: 'organic',
  before: [{ landId: 7, plantId: 1020003, currentSeason: 1 }],
  selectedBucketCountBefore: 5,
  selectedBucketCountAfter: 5,
  selectedBucketDeltaCount: 0,
  managerStateAfterPerform: { currentDetailType: 'fertilizer' },
}, {
  captured: false,
  dispatches: [],
  error: 'fertilizer protocol event was not dispatched',
  eventName: null,
  result: null,
});

assert.strictEqual(diagnostics.reason, 'fertilizer protocol event was not dispatched');
assert.deepStrictEqual(Array.from(diagnostics.requestedLandIds), [7, 8]);
assert.strictEqual(diagnostics.mode, 'organic');
assert.strictEqual(diagnostics.protocol.captured, false);
assert.strictEqual(diagnostics.protocol.dispatchCount, 0);
assert.strictEqual(diagnostics.bucket.selectedBefore, 5);
assert.deepStrictEqual(diagnostics.manager, { currentDetailType: 'fertilizer' });

async function verifyBatchUsesNativePerLandPayloads() {
  const performCalls = [];
  const manager = {
    multiLand: false,
    mutiLandGridData: null,
    performFertilizing(landId) {
      performCalls.push(landId);
      nativeFertilizerTimeline.push(`perform:${landId}`);
      testMessageBus.dispatchEvent('plant.fertilize', {
        land_id: landId,
        activityCrop: {
          landId,
          ticket: `activity-${landId}`,
        },
      });
      return { landId };
    },
  };

  nativeFertilizerEvents.length = 0;
  nativeFertilizerTimeline.length = 0;
  const result = await dispatchFertilizerBatch(manager, [101, 102], {
    betweenLandWait: 0,
    mode: 'organic',
    syncTargets: false,
  });

  assert.deepStrictEqual(performCalls, [101, 102]);
  assert.deepStrictEqual(
    nativeFertilizerEvents.map((event) => event.payload.activityCrop.ticket),
    ['activity-101', 'activity-102'],
  );
  assert.deepStrictEqual(nativeFertilizerTimeline, [
    'perform:101',
    'perform:102',
    'dispatch:101',
    'dispatch:102',
  ]);
  assert.strictEqual(result.error, null);
}

async function verifySerialBatchDispatchesOneLandAtATime() {
  const performCalls = [];
  const manager = {
    multiLand: false,
    mutiLandGridData: null,
    performFertilizing(landId) {
      performCalls.push(landId);
      nativeFertilizerTimeline.push(`perform:${landId}`);
      testMessageBus.dispatchEvent('plant.fertilize', {
        land_id: landId,
        activityCrop: {
          landId,
          ticket: `activity-${landId}`,
        },
      });
      return { landId };
    },
  };

  nativeFertilizerEvents.length = 0;
  nativeFertilizerTimeline.length = 0;
  const result = await dispatchFertilizerBatch(manager, [201, 202], {
    betweenLandWait: 0,
    fertilizerSubmissionMode: 'serial',
    mode: 'organic',
    syncTargets: false,
  });

  assert.deepStrictEqual(performCalls, [201, 202]);
  assert.deepStrictEqual(
    nativeFertilizerEvents.map((event) => event.payload.activityCrop.ticket),
    ['activity-201', 'activity-202'],
  );
  assert.deepStrictEqual(nativeFertilizerTimeline, [
    'perform:201',
    'dispatch:201',
    'perform:202',
    'dispatch:202',
  ]);
  assert.strictEqual(result.error, null);
}

verifyBatchUsesNativePerLandPayloads()
  .then(verifySerialBatchDispatchesOneLandAtATime)
  .then(() => {
    console.log('[fertilizer-repeat-skip] season cache and native per-land batch and serial dispatch pass');
  })
  .catch((error) => {
    console.error(error);
    process.exitCode = 1;
  });
