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
const startedAt = Date.now();
const stamp = new Date().toISOString().replace(/[:.]/g, "-");
const outputBase = path.join(outputDir, `svip-claim-${stamp}`);
let sequence = 0;

function rpc(name, args, timeoutMs = 30_000) {
  return new Promise((resolve, reject) => {
    const callbackID = `svip-${Date.now()}-${++sequence}`;
    const timer = setTimeout(() => {
      pending.delete(callbackID);
      reject(new Error(`timeout ${name}`));
    }, timeoutMs);
    pending.set(callbackID, { resolve, reject, timer });
    ws.send("C" + JSON.stringify({ name, args, callbackID }));
  });
}

function unwrapPrimitive(value) {
  return value && value.summary && value.summary.primitive !== undefined
    ? value.summary.primitive
    : value;
}

function requestBytes(event) {
  const args = Array.isArray(event && event.args) ? event.args : [];
  const primitive = unwrapPrimitive(args[0]) || {};
  return Object.keys(primitive)
    .sort((a, b) => Number(a) - Number(b))
    .map((key) => Number(primitive[key]) & 255);
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
    await rpc("main.App.RunDiagnostic", [
      "gameCtl.startRuntimeSpies",
      { args: [{ silent: true, includeFrames: false, limit: 20 }] },
    ]);
    await rpc("main.App.RunDiagnostic", [
      "gameCtl.resetRuntimeSpyEvents",
      { args: [{ silent: true, keepProfiles: true }] },
    ]);

    console.log("[ready] SVIP claim listener armed");
    console.log("[ready] click the QQ SVIP 领取 button exactly once");

    let snapshot = {};
    let matchedAt = 0;
    const deadline = Date.now() + 180_000;
    while (Date.now() < deadline) {
      const response = await rpc("main.App.RunDiagnostic", [
        "gameCtl.getRuntimeSpySnapshot",
        { args: [{ silent: true, limit: 500, includeFrames: false }] },
      ]);
      snapshot = (response && response.result) || {};
      const sends = Array.isArray(snapshot.sendEvents) ? snapshot.sendEvents : [];
      const match = sends.find((event) => {
        const args = Array.isArray(event && event.args) ? event.args : [];
        return unwrapPrimitive(args[1]) === "ClaimQQVipRewards";
      });
      if (match && !matchedAt) {
        matchedAt = Date.now();
        const args = match.args || [];
        const bytes = requestBytes(match);
        console.log(
          `[send] service=${unwrapPrimitive(args[3])} method=${unwrapPrimitive(args[1])} bytes=${bytes
            .map((value) => value.toString(16).padStart(2, "0"))
            .join(" ")}`,
        );
      }
      if (matchedAt && Date.now() - matchedAt >= 1_800) break;
      await new Promise((resolve) => setTimeout(resolve, 500));
    }

    const events = [];
    for (const event of snapshot.clickEvents || []) events.push({ kind: "click", event });
    for (const event of snapshot.messageEvents || []) events.push({ kind: "message", event });
    for (const event of snapshot.sendEvents || []) events.push({ kind: "send", event });
    const payload = { ok: true, startedAt, finishedAt: Date.now(), events };
    const rawFile = outputBase + "-raw.json";
    const extractedFile = outputBase + "-extracted.json";
    fs.writeFileSync(rawFile, JSON.stringify(payload, null, 2));

    const sendEvents = events.filter(
      (item) => item.kind === "send" && JSON.stringify(item.event).includes("ClaimQQVipRewards"),
    );
    const extracted = {
      matched: sendEvents.length > 0,
      sendEvents,
      recentMessages: events.filter((item) => item.kind === "message").slice(-40),
      recentClicks: events.filter((item) => item.kind === "click").slice(-20),
    };
    fs.writeFileSync(extractedFile, JSON.stringify(extracted, null, 2));
    console.log(`[done] matched=${extracted.matched}`);
    console.log(`[capture] ${extractedFile}`);
    ws.close();
  } catch (error) {
    console.error("[fatal]", error && error.stack ? error.stack : error);
    ws.close();
    process.exitCode = 1;
  }
});

ws.on("error", (error) => {
  console.error("[fatal]", error && error.stack ? error.stack : error);
  process.exit(1);
});
