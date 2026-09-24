const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const root = path.join(__dirname, '..');
const buttonPath = path.join(root, 'resources', 'wmpf', 'button.js');
const source = fs.readFileSync(buttonPath, 'utf8');
const exportMarker = '  G.gameCtl = {';

assert.ok(source.includes(exportMarker), 'button.js gameCtl export marker not found');

const instrumented = source.replace(
  exportMarker,
  `  globalThis.__warehouseSalePriceTest = {
    readSaleUnitPrice: readWarehouseModelSaleUnitPrice,
    canSell: readWarehouseModelCanSell,
    readRuntimeSaleMeta: readRuntimeConfigSaleMeta,
  };
${exportMarker}`,
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
vm.runInContext(instrumented, context, { filename: 'button.js', timeout: 5000 });

const { readSaleUnitPrice, canSell, readRuntimeSaleMeta } = context.__warehouseSalePriceTest;
const qingmei = { _tempData: { price: 240, price_id: 0 }, locked: false };

assert.strictEqual(readSaleUnitPrice(qingmei), 240);
assert.strictEqual(canSell(qingmei), true);
assert.strictEqual(canSell({ ...qingmei, locked: true }), false);
assert.strictEqual(canSell({ canSell: false, price: 240, locked: false }), false);
assert.strictEqual(canSell({ canSell: false, sellItemId: 1001, locked: false }), false);
assert.strictEqual(
  readSaleUnitPrice({ price: 240, saleRewards: [{ itemId: 1, amount: 300 }] }),
  300,
);

const conditionalQingmei = {
  id: 41221,
  name: '青梅',
  price: 240,
  cond_sells: '1001:240',
  sell_cond: '活动结束后:2026080102',
  sells: null,
};
const beforeActivityEnd = readRuntimeSaleMeta(conditionalQingmei, new Date(2026, 6, 11, 10).getTime());
assert.strictEqual(JSON.stringify(beforeActivityEnd.rewards), JSON.stringify([{ itemId: 1001, amount: 240 }]));
assert.strictEqual(beforeActivityEnd.unitPrice, 240);
assert.strictEqual(beforeActivityEnd.currencyId, 1001);
assert.strictEqual(beforeActivityEnd.canSell, true);
assert.strictEqual(beforeActivityEnd.conditionLabel, null);

const afterActivityEnd = readRuntimeSaleMeta(conditionalQingmei, new Date(2026, 7, 1, 2).getTime());
assert.strictEqual(afterActivityEnd.canSell, true);
assert.strictEqual(readRuntimeSaleMeta({ canSell: false, price: 240 }).canSell, false);

console.log('[warehouse-sale-price] Qingmei conditional sale price and availability are recognized');
