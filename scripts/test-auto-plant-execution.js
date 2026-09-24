const assert = require('assert');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');
const buttonPath = path.join(root, 'resources', 'wmpf', 'button.js');
let source = fs.readFileSync(buttonPath, 'utf8');

source = source.replace(
  '  G.gameCtl = {',
  '  G.__testNormalizeAutoPlantExecutionSettings = normalizeAutoPlantExecutionSettings;\n' +
    '  G.__testOrderAutoPlantExecutionUnits = orderAutoPlantExecutionUnits;\n' +
    '  G.__testDispatchAutoPlantExecutionUnits = dispatchAutoPlantExecutionUnits;\n' +
    '  G.gameCtl = {'
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

const normalize = context.__testNormalizeAutoPlantExecutionSettings;
const order = context.__testOrderAutoPlantExecutionUnits;
const dispatch = context.__testDispatchAutoPlantExecutionUnits;
assert.strictEqual(typeof normalize, 'function');
assert.strictEqual(typeof order, 'function');
assert.strictEqual(typeof dispatch, 'function');

const plain = (value) => JSON.parse(JSON.stringify(value));

async function run() {
  const stable = plain(normalize({
    autoPlantRandomizedEnabled: true,
    autoPlantRandomizedDelayMinMs: 20,
    autoPlantRandomizedDelayMaxMs: 20,
  }));
  assert.deepStrictEqual(stable, {
    randomOrderEnabled: false,
    randomDelayEnabled: true,
    minDelayMs: 20,
    maxDelayMs: 20,
    deadlineAtMs: 0,
  });
  assert.deepStrictEqual(plain(normalize({
    autoPlantRandomizedEnabled: true,
    autoPlantRandomizedDelayMinMs: -1,
    autoPlantRandomizedDelayMaxMs: 99_999,
  })), {
    randomOrderEnabled: false,
    randomDelayEnabled: true,
    minDelayMs: 0,
    maxDelayMs: 10_000,
    deadlineAtMs: 0,
  });

  const units = [
    { kind: 'single', landIds: [1] },
    { kind: 'single', landIds: [2] },
    { kind: 'single', landIds: [3] },
  ];
  assert.deepStrictEqual(plain(order(units, plain(normalize({})), () => 0)), units);

  const fourGridUnits = [
    { kind: 'multi', landIds: [1, 2, 5, 6] },
    { kind: 'single', landIds: [8] },
  ];
  assert.deepStrictEqual(
    plain(order(fourGridUnits, plain(normalize({ autoPlantRandomOrderEnabled: true })), () => 0)),
    [{ kind: 'single', landIds: [8] }, { kind: 'multi', landIds: [1, 2, 5, 6] }],
  );

  let now = 0;
  const waits = [];
  const dispatched = [];
  const paced = await dispatch(
    units,
    stable,
    (unit) => {
      dispatched.push(unit.landIds[0]);
      return { landId: unit.landIds[0] };
    },
    () => now,
    async (ms) => {
      waits.push(ms);
      now += ms;
    },
    () => 0,
  );
  assert.deepStrictEqual(plain(paced), {
    ok: true,
    units,
    payloads: [{ landId: 1 }, { landId: 2 }, { landId: 3 }],
    delaysMs: [20, 20],
  });
  assert.deepStrictEqual(dispatched, [1, 2, 3]);
  assert.deepStrictEqual(waits, [20, 20]);

  now = 95;
  const deadlineDispatches = [];
  const timedOut = await dispatch(
    units,
    {
      randomOrderEnabled: false,
      randomDelayEnabled: true,
      minDelayMs: 10,
      maxDelayMs: 10,
      deadlineAtMs: 100,
    },
    (unit) => {
      deadlineDispatches.push(unit.landIds[0]);
      return { landId: unit.landIds[0] };
    },
    () => now,
    async (ms) => { now += ms; },
    () => 0,
  );
  assert.strictEqual(timedOut.ok, false);
  assert.strictEqual(timedOut.reason, 'plant_execution_timeout');
  assert.deepStrictEqual(deadlineDispatches, [1]);
}

run().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
