# Warehouse UI Sale Price Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correctly recognize warehouse UI models such as Qingmei as sellable when their runtime sale price is stored in nested model configuration fields.

**Architecture:** Keep runtime data authoritative. Extend the existing warehouse model readers in `resources/wmpf/button.js` to use the shared nested-number traversal after reward parsing, and verify the real script behavior through a Node VM regression test without adding test-only exports to production code.

**Tech Stack:** JavaScript runtime injection, Node.js `vm`, Go test suite, React/Vitest regression suite.

---

### Task 1: Reproduce nested warehouse sale price parsing

**Files:**
- Create: `scripts/test-warehouse-sale-price.js`

- [ ] **Step 1: Add a failing VM regression test**

Load `resources/wmpf/button.js`, inject a test export immediately before `const gameCtl = {`, and assert:

```js
const qingmei = { _tempData: { price: 240, price_id: 0 }, locked: false };
assert.strictEqual(readSaleUnitPrice(qingmei), 240);
assert.strictEqual(canSell(qingmei), true);
assert.strictEqual(canSell({ ...qingmei, locked: true }), false);
assert.strictEqual(readSaleUnitPrice({ price: 240, saleRewards: [{ itemId: 1, amount: 300 }] }), 300);
```

- [ ] **Step 2: Run the test and verify RED**

Run: `node scripts/test-warehouse-sale-price.js`

Expected: FAIL because `_tempData.price` currently produces 0.

### Task 2: Read nested model price and currency fields

**Files:**
- Modify: `resources/wmpf/button.js`

- [ ] **Step 1: Implement the minimal sale-price fallback**

Keep reward parsing first, then use the existing nested reader:

```js
function readWarehouseModelSaleUnitPrice(model) {
  const rewards = readWarehouseModelSaleRewards(model);
  const primary = rewards && rewards.length > 0 ? rewards[0] : null;
  const rewardAmount = Number(primary && primary.amount) || 0;
  if (rewardAmount > 0) return rewardAmount;
  return readWarehouseModelNumber(model, ['saleUnitPrice', 'sellPrice', 'price']);
}
```

Apply the equivalent fallback for `saleCurrencyId`, `priceId`, and `price_id` in `readWarehouseModelSaleCurrencyId`.

- [ ] **Step 2: Run the VM regression test and verify GREEN**

Run: `node scripts/test-warehouse-sale-price.js`

Expected: PASS and print the Qingmei nested-price confirmation.

### Task 3: Full verification

**Files:**
- Verify all changed files

- [ ] **Step 1: Run Go tests**

Run: `go test ./... -count=1`

Expected: PASS.

- [ ] **Step 2: Run frontend tests**

Run: `npm test -- --run`

Working directory: `frontend`

Expected: PASS.

- [ ] **Step 3: Run frontend build**

Run: `npm run build`

Working directory: `frontend`

Expected: exit code 0.

- [ ] **Step 4: Inspect final diff**

Run: `git diff --check && git status --short && git diff --stat`

Expected: only the runtime script, regression test, and pre-existing user-owned dirty files are present.
