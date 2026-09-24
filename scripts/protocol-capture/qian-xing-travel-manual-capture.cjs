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

function rpc(name, args, timeoutMs = 30_000) {
  return new Promise((resolve, reject) => {
    const callbackID = `qian-xing-travel-${Date.now()}-${++sequence}`;
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

function unwrapPrimitive(value) {
  return value && value.summary && value.summary.primitive !== undefined
    ? value.summary.primitive
    : value;
}

function readMethod(event) {
  const args = Array.isArray(event && event.args) ? event.args : [];
  return String(unwrapPrimitive(args[1]) || "");
}

function isBackgroundMethod(method) {
  return /^(?:ping|pong|heartbeat|keepalive|report|upload|login|sync|notice|antidata)$/i.test(method);
}

function requestBytes(event) {
  const args = Array.isArray(event && event.args) ? event.args : [];
  const primitive = unwrapPrimitive(args[0]) || {};
  return Object.keys(primitive)
    .sort((a, b) => Number(a) - Number(b))
    .map((key) => Number(primitive[key]) & 255);
}

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
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
    const startedAt = Date.now();
    await diagnostic("gameCtl.startRuntimeSpies", [{
      silent: true,
      includeFrames: false,
      limit: 20,
    }]);
    await diagnostic("gameCtl.resetRuntimeSpyEvents", [{
      silent: true,
      keepProfiles: true,
    }]);

    console.log("[ready] 千星游记一键领取监听已挂起");
    console.log("[ready] 请只点击一次：千星游记 -> 一键领取");

    let snapshot = {};
    let matched = null;
    const deadline = Date.now() + 300_000;
    while (Date.now() < deadline && !matched) {
      const response = await diagnostic("gameCtl.getRuntimeSpySnapshot", [{
        silent: true,
        limit: 500,
        includeFrames: false,
      }]);
      snapshot = (response && response.result) || {};
      const sends = Array.isArray(snapshot.sendEvents) ? snapshot.sendEvents : [];
      matched = sends.find((event) => {
        const method = readMethod(event);
        return method !== "" && !isBackgroundMethod(method);
      }) || null;
      if (!matched) await delay(400);
    }

    if (matched) {
      const args = Array.isArray(matched.args) ? matched.args : [];
      const bytes = requestBytes(matched);
      console.log(
        `[observed] service=${unwrapPrimitive(args[3]) || ""} method=${readMethod(matched)} bytes=${bytes
          .map((value) => value.toString(16).padStart(2, "0"))
          .join(" ")}`,
      );
      await delay(1_800);
      const response = await diagnostic("gameCtl.getRuntimeSpySnapshot", [{
        silent: true,
        limit: 500,
        includeFrames: false,
      }]);
      snapshot = (response && response.result) || snapshot;
    }

    const stamp = new Date().toISOString().replace(/[:.]/g, "-");
    const outputBase = path.join(outputDir, `qian-xing-travel-claim-${stamp}`);
    const raw = {
      startedAt,
      finishedAt: Date.now(),
      matched: Boolean(matched),
      snapshot,
    };
    fs.writeFileSync(`${outputBase}-raw.json`, JSON.stringify(raw, null, 2));

    const events = [
      ...(snapshot.clickEvents || []).map((event) => ({ kind: "click", event })),
      ...(snapshot.messageEvents || []).map((event) => ({ kind: "message", event })),
      ...(snapshot.sendEvents || []).map((event) => ({ kind: "send", event })),
    ];
    const extracted = {
      matched: Boolean(matched),
      sends: (snapshot.sendEvents || []).map((event) => {
        const args = Array.isArray(event.args) ? event.args : [];
        return {
          method: readMethod(event),
          service: unwrapPrimitive(args[3]) || null,
          bytesHex: requestBytes(event).map((value) => value.toString(16).padStart(2, "0")).join(" "),
          event,
        };
      }),
      clicks: snapshot.clickEvents || [],
      messages: snapshot.messageEvents || [],
      events,
    };
    const extractedFile = `${outputBase}-extracted.json`;
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
