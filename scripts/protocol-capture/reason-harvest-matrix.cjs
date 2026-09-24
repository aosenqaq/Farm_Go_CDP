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
const targetArg = process.argv.find((arg) => arg.startsWith("--target-gid="));
const targetGid = targetArg == null ? 0 : Number(targetArg.slice("--target-gid=".length));
const sendHarvest = process.argv.includes("--confirm-single-land");
const startReason = integerOption("--start-reason=", 1);
const endReason = integerOption("--end-reason=", 10);
const stamp = new Date().toISOString().replace(/[:.]/g, "-");

if (!Number.isSafeInteger(targetGid) || targetGid <= 0) {
  throw new Error("provide --target-gid=<authorized GID>");
}
if (!Number.isInteger(startReason) || !Number.isInteger(endReason) || startReason < 0 || endReason > 127 || startReason > endReason) {
  throw new Error("reason range must satisfy 0 <= start <= end <= 127");
}

fs.mkdirSync(outputDir, { recursive: true });

function integerOption(prefix, fallback) {
  const arg = process.argv.find((value) => value.startsWith(prefix));
  return arg == null ? fallback : Number(arg.slice(prefix.length));
}

class WailsClient {
  constructor() {
    this.ws = new WebSocket("ws://127.0.0.1:34115/wails/ipc");
    this.pending = new Map();
    this.sequence = 0;
    this.open = new Promise((resolve, reject) => {
      this.ws.once("open", resolve);
      this.ws.once("error", reject);
    });
    this.ws.on("message", (raw) => this.receive(raw));
  }

  receive(raw) {
    const text = raw.toString();
    if (!text.startsWith("c")) return;
    let packet;
    try {
      packet = JSON.parse(text.slice(1));
    } catch (_) {
      return;
    }
    const pending = this.pending.get(packet.callbackid);
    if (!pending) return;
    clearTimeout(pending.timer);
    this.pending.delete(packet.callbackid);
    if (packet.error) pending.reject(new Error(String(packet.error)));
    else pending.resolve(packet.result);
  }

  async diagnostic(method, args, timeoutMs = 45_000) {
    await this.open;
    return await new Promise((resolve, reject) => {
      const callbackID = `reason-harvest-${Date.now()}-${++this.sequence}`;
      const timer = setTimeout(() => {
        this.pending.delete(callbackID);
        reject(new Error(`timeout ${method}`));
      }, timeoutMs);
      this.pending.set(callbackID, { resolve, reject, timer });
      this.ws.send("C" + JSON.stringify({
        name: "main.App.RunDiagnostic",
        args: [method, { args }],
        callbackID,
      }));
    });
  }

  close() {
    try {
      this.ws.close();
    } catch (_) {}
  }
}

function resultOrThrow(response, method) {
  if (response && response.ok === true) return response.result;
  throw new Error((response && response.error) || `diagnostic failed: ${method}`);
}

function positiveInteger(value) {
  const number = Number(value);
  return Number.isSafeInteger(number) && number > 0 ? number : null;
}

function firstCollectibleLand(inspection) {
  const values = inspection && inspection.workLandIds && inspection.workLandIds.collect;
  if (!Array.isArray(values)) return null;
  for (const value of values) {
    const landID = positiveInteger(value);
    if (landID != null) return landID;
  }
  return null;
}

function sendEvents(snapshot, methodName, serviceName) {
  const events = Array.isArray(snapshot && snapshot.sendEvents) ? snapshot.sendEvents : [];
  return events.filter((event) => {
    const args = Array.isArray(event && event.args) ? event.args : [];
    return args[1] === methodName && args[3] === serviceName;
  });
}

function summary(value) {
  if (!value || typeof value !== "object") return value;
  return {
    ok: value.ok === true,
    action: value.action || null,
    hostGid: positiveInteger(value.hostGid),
    landIds: Array.isArray(value.landIds) ? value.landIds.map(positiveInteger).filter(Boolean) : [],
    serviceName: value.serviceName || null,
    methodName: value.methodName || null,
    callbackCalled: value.callbackCalled === true,
    callbackError: value.callbackError || null,
    failureText: value.failureText || null,
  };
}

async function runReason(client, reason) {
  const row = {
    reason,
    status: "not_started",
    query: null,
    harvest: null,
    selectedLandID: null,
    evidence: null,
    error: null,
  };

  try {
    resultOrThrow(await client.diagnostic("gameCtl.resetRuntimeSpyEvents", [{
      silent: true,
      keepProfiles: true,
    }]), "gameCtl.resetRuntimeSpyEvents");

    const inspection = resultOrThrow(await client.diagnostic("gameCtl.inspectFriendFarmByProtocol", [{
      hostGid: targetGid,
      reason,
      silent: true,
      includeLands: false,
      leaveAfter: false,
      waitReplyMs: 5_000,
      source: "reason_harvest_matrix_query",
    }]), "gameCtl.inspectFriendFarmByProtocol");
    row.query = summary(inspection && inspection.visit ? inspection.visit : inspection);

    if (!inspection || inspection.ok !== true) {
      row.status = "query_returned_failure_action_withheld";
      return row;
    }
    const replyGid = positiveInteger(inspection.friend && inspection.friend.gid);
    if (replyGid !== targetGid) {
      row.status = "reply_gid_mismatch_action_withheld";
      return row;
    }
    const landID = firstCollectibleLand(inspection);
    if (landID == null) {
      row.status = "no_current_collectible_land_action_withheld";
      return row;
    }
    row.selectedLandID = landID;
    if (!sendHarvest) {
      row.status = "dry_run_single_land_selected_action_withheld";
      return row;
    }

    const harvest = resultOrThrow(await client.diagnostic("gameCtl.friendHarvestLandsByProtocol", [{
      hostGid: targetGid,
      landIds: [landID],
      isAll: false,
      silent: true,
      waitReplyMs: 1_500,
      source: "reason_harvest_matrix_single_land",
    }]), "gameCtl.friendHarvestLandsByProtocol");
    row.harvest = summary(harvest);
    row.status = harvest && harvest.ok === true
      ? "single_land_harvest_returned_success"
      : "single_land_harvest_returned_failure";
  } catch (error) {
    row.status = "diagnostic_error_action_withheld_or_failed";
    row.error = error && error.message ? error.message : String(error);
  } finally {
    try {
      const snapshot = resultOrThrow(await client.diagnostic("gameCtl.getRuntimeSpySnapshot", [{
        silent: true,
        includeFrames: false,
        limit: 300,
      }]), "gameCtl.getRuntimeSpySnapshot") || {};
      row.evidence = {
        clickCount: Array.isArray(snapshot.clickEvents) ? snapshot.clickEvents.length : 0,
        enterSendCount: sendEvents(snapshot, "Enter", "gamepb.visitpb.VisitService").length,
        harvestSendCount: sendEvents(snapshot, "Harvest", "gamepb.plantpb.PlantService").length,
      };
    } catch (error) {
      row.evidence = { snapshotError: error && error.message ? error.message : String(error) };
    }
  }
  return row;
}

async function main() {
  const client = new WailsClient();
  const report = {
    targetGid,
    sendHarvest,
    startReason,
    endReason,
    reasons: [],
    startedAt: new Date().toISOString(),
    finishedAt: null,
  };

  try {
    resultOrThrow(await client.diagnostic("gameCtl.startRuntimeSpies", [{
      silent: true,
      includeFrames: false,
      limit: 20,
    }]), "gameCtl.startRuntimeSpies");

    for (let reason = startReason; reason <= endReason; reason += 1) {
      report.reasons.push(await runReason(client, reason));
    }
  } finally {
    report.finishedAt = new Date().toISOString();
    const outputPath = path.join(outputDir, `reason-harvest-matrix-${targetGid}-${stamp}.json`);
    fs.writeFileSync(outputPath, JSON.stringify(report, null, 2), "utf8");
    client.close();
    console.log(JSON.stringify({
      targetGid,
      sendHarvest,
      startReason,
      endReason,
      reasons: report.reasons.map((row) => ({
        reason: row.reason,
        status: row.status,
        selectedLandID: row.selectedLandID,
        evidence: row.evidence,
      })),
      outputPath,
    }, null, 2));
  }
}

main().catch((error) => {
  console.error(error && error.stack ? error.stack : String(error));
  process.exitCode = 1;
});
