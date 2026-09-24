"use strict";

const fs = require("node:fs");
const path = require("node:path");

function requireWebSocket() {
  try {
    return require("ws");
  } catch (_) {
    return require(path.join(
      process.env.USERPROFILE,
      ".codex",
      "skills",
      "farm-protocol-capture",
      "scripts",
      "node_modules",
      "ws",
    ));
  }
}

const WebSocket = requireWebSocket();
const root = path.resolve(__dirname, "..", "..");
const outputDir = path.join(root, "data", "debug-captures");
fs.mkdirSync(outputDir, { recursive: true });

const ws = new WebSocket("ws://127.0.0.1:34115/wails/ipc");
const pending = new Map();
let sequence = 0;

function rpc(name, args, timeoutMs = 20_000) {
  return new Promise((resolve, reject) => {
    const callbackID = `svip-status-${Date.now()}-${++sequence}`;
    const timer = setTimeout(() => {
      pending.delete(callbackID);
      reject(new Error(`timeout ${name}`));
    }, timeoutMs);
    pending.set(callbackID, { resolve, reject, timer });
    ws.send("C" + JSON.stringify({ name, args, callbackID }));
  });
}

function diagnostic(method, args) {
  return rpc("main.App.RunDiagnostic", [method, { args }]);
}

ws.on("message", (raw) => {
  const text = raw.toString();
  if (!text.startsWith("c")) return;
  const message = JSON.parse(text.slice(1));
  const callback = pending.get(message.callbackid);
  if (!callback) return;
  clearTimeout(callback.timer);
  pending.delete(message.callbackid);
  if (message.error) callback.reject(new Error(String(message.error)));
  else callback.resolve(message.result);
});

ws.on("open", async () => {
  try {
    await diagnostic("gameCtl.startRuntimeSpies", [{ silent: true, includeFrames: false, limit: 20 }]);
    await diagnostic("gameCtl.resetRuntimeSpyEvents", [{ silent: true, keepProfiles: true }]);

    const attempts = [];
    for (let index = 0; index < 2; index += 1) {
      const response = await diagnostic("gameCtl.requestSvipRewardsStatus", [{
        silent: true,
        waitMs: 5000,
        pollMs: 80,
      }]);
      attempts.push(response);
      if (response && response.result && response.result.callbackCalled) break;
    }
    const snapshot = await diagnostic("gameCtl.getRuntimeSpySnapshot", [{
      silent: true,
      limit: 120,
      includeFrames: false,
    }]);
    const stamp = new Date().toISOString().replace(/[:.]/g, "-");
    const outputPath = path.join(outputDir, `svip-status-${stamp}.json`);
    fs.writeFileSync(outputPath, JSON.stringify({ attempts, snapshot }, null, 2));
    console.log(outputPath);
    console.log(JSON.stringify(attempts[attempts.length - 1], null, 2));
    ws.close();
  } catch (error) {
    console.error(error && error.stack ? error.stack : error);
    ws.close();
    process.exitCode = 1;
  }
});

ws.on("error", (error) => {
  console.error(error && error.stack ? error.stack : error);
  process.exit(1);
});
