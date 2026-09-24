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
    const callbackID = `xing-su-status-${Date.now()}-${++sequence}`;
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

function protocolSends(snapshot, methodName) {
  const sends = Array.isArray(snapshot && snapshot.sendEvents) ? snapshot.sendEvents : [];
  return sends.filter((event) => {
    const args = Array.isArray(event && event.args) ? event.args : [];
    const method = args[1] && args[1].summary && args[1].summary.primitive !== undefined
      ? args[1].summary.primitive
      : args[1];
    const service = args[3] && args[3].summary && args[3].summary.primitive !== undefined
      ? args[3].summary.primitive
      : args[3];
    return method === methodName && service === "gamepb.activitypb.ActivityService";
  });
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
    await diagnostic("gameCtl.startRuntimeSpies", [{ silent: true, includeFrames: false, limit: 40 }]);
    await diagnostic("gameCtl.resetRuntimeSpyEvents", [{ silent: true, keepProfiles: true }]);
    const status = await diagnostic("gameCtl.requestXingSuActivityStatus", [{
      silent: true,
      waitMs: 5000,
    }]);
    const snapshotResponse = await diagnostic("gameCtl.getRuntimeSpySnapshot", [{
      silent: true,
      includeFrames: false,
      limit: 200,
    }]);
    const result = status && status.result ? status.result : {};
    const snapshot = snapshotResponse && snapshotResponse.result ? snapshotResponse.result : {};
    const listSends = protocolSends(snapshot, "List");
    const operateSends = protocolSends(snapshot, "Operate");
    const verification = {
      queryOK: result.ok === true,
      codecFound: result.codecFound === true,
      listSendCount: listSends.length,
      operateSendCount: operateSends.length,
      clickCount: Array.isArray(snapshot.clickEvents) ? snapshot.clickEvents.length : -1,
    };
    const stamp = new Date().toISOString().replace(/[:.]/g, "-");
    const outputPath = path.join(outputDir, `xing-su-activity-status-live-${stamp}.json`);
    fs.writeFileSync(outputPath, JSON.stringify({ status, snapshot, verification }, null, 2));
    console.log(JSON.stringify(verification));
    console.log(outputPath);
    if (!verification.queryOK || !verification.codecFound || verification.listSendCount < 1 || verification.operateSendCount !== 0 || verification.clickCount !== 0) {
      process.exitCode = 1;
    }
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
