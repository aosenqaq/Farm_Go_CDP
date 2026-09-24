const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");

const scriptDir = __dirname;
const scripts = [
  "svip-live-capture.cjs",
  "svip-status-capture.cjs",
  "svip-dynamic-live.cjs",
  "qian-xing-travel-manual-capture.cjs",
  "qian-xing-travel-live-acceptance.cjs",
];

for (const name of scripts) {
  const source = fs.readFileSync(path.join(scriptDir, name), "utf8");
  assert.ok(
    source.includes('const root = path.resolve(__dirname, "..", "..");'),
    `${name} should resolve the Farm_Go project root`,
  );
  assert.ok(
    source.includes('const outputDir = path.join(root, "data", "debug-captures");'),
    `${name} should write captures only under data/debug-captures`,
  );
  assert.ok(!source.includes("C:/Users/"), `${name} should not contain a user-specific dependency path`);
}

console.log("[protocol-capture-layout] reusable scripts keep captures in the ignored data directory");
