/*
 * 原创脚本开发/抄袭/Faker/倒卖全家三日内暴毙，一辈子发不了财，开源项目地址：[aosenqaq/QQ_Farm_CDP_Manger](https://github.com/aosenqaq/QQ_Farm_CDP_Manger)
 * 你是自己不会写脚本？？天天抄袭我？？
 */

// 水印：二开倒卖先别急，README 都没看明白就上链接，属实有点绷不住。
(function () {
  var root = typeof globalThis !== "undefined" ? globalThis : Function("return this")();
  if (root.__farmGoQqHost && root.__farmGoQqHost.__installed) {
    return;
  }

  var mini = root.wx || root.qq || null;
  var hostPaths = ["host.ping", "host.describe", "gameCtl.probe"];
  var hostPathMap = {};
  for (var i = 0; i < hostPaths.length; i += 1) {
    hostPathMap[hostPaths[i]] = true;
  }

  var defaults = {
    url: "ws://127.0.0.1:8787/runtime/qqws",
    reconnectMs: 3000,
    heartbeatMs: 15000,
    callTimeoutMs: 15000,
    readyPollMs: 2000,
    autoStart: true,
    expectedAppPlatform: "qq"
  };

  var pendingLogLimit = 50;

  var state = {
    url: defaults.url,
    phase: "idle",
    seq: 0,
    socket: null,
    socketKind: null,
    reconnectTimer: null,
    heartbeatTimer: null,
    readyPollTimer: null,
    manualStop: false,
    lastHelloAck: null,
    lastGameCtlReady: null,
    lastError: null,
    clientId: "qq-farm-" + Math.random().toString(36).slice(2, 10),
    pendingLogs: []
  };

  function now() {
    return Date.now();
  }

  function nextId(prefix) {
    state.seq += 1;
    return prefix + "-" + state.seq;
  }

  function logLocal(level, message, extra) {
    var text = "[qq-host][" + level + "] " + message;
    try {
      if (extra === undefined) {
        console.log(text);
      } else {
        console.log(text, extra);
      }
    } catch (_) {}
  }

  function setPhase(phase) {
    state.phase = phase;
  }

  function clearTimer(name) {
    if (!state[name]) return;
    clearTimeout(state[name]);
    clearInterval(state[name]);
    state[name] = null;
  }

  function getGameCtl() {
    var ctl = root.gameCtl || (root.GameGlobal && root.GameGlobal.gameCtl);
    return ctl && typeof ctl === "object" ? ctl : null;
  }

  function getSystemInfo() {
    if (!mini || typeof mini.getSystemInfoSync !== "function") {
      return null;
    }
    try {
      return mini.getSystemInfoSync();
    } catch (_) {
      return null;
    }
  }

  function getAppPlatform(systemInfo) {
    var info = systemInfo || getSystemInfo();
    if (info && typeof info.AppPlatform === "string" && info.AppPlatform) {
      return info.AppPlatform;
    }
    if (root.qq) return "qq";
    if (root.wx) return "wx";
    return "unknown";
  }

  function getScriptHash() {
    var ctl = getGameCtl();
    if (ctl && typeof ctl.__scriptHash === "string" && ctl.__scriptHash) {
      return ctl.__scriptHash;
    }
    var meta = root.__qqFarmBundleMeta;
    if (meta && typeof meta.scriptHash === "string" && meta.scriptHash) {
      return meta.scriptHash;
    }
    return "farm-go-debug-link";
  }

  function sanitizeSystemInfo(systemInfo) {
    var info = systemInfo || getSystemInfo();
    if (!info || typeof info !== "object") return null;
    return {
      brand: info.brand || null,
      model: info.model || null,
      platform: info.platform || null,
      AppPlatform: info.AppPlatform || null,
      windowWidth: info.windowWidth || null,
      windowHeight: info.windowHeight || null,
      pixelRatio: info.pixelRatio || null,
      SDKVersion: info.SDKVersion || null,
      MiniAppVersion: info.MiniAppVersion || null
    };
  }

  function collectAvailableMethods() {
    var list = hostPaths.slice();
    var ctl = getGameCtl();
    if (!ctl) return list;
    var names = typeof Object.keys === "function" ? Object.keys(ctl) : [];
    for (var i = 0; i < names.length; i += 1) {
      var methodName = names[i];
      if (typeof ctl[methodName] === "function") {
        list.push("gameCtl." + methodName);
      }
    }
    return list;
  }

  function getStatus() {
    var ctl = getGameCtl();
    var systemInfo = getSystemInfo();
    return {
      clientId: state.clientId,
      url: state.url,
      phase: state.phase,
      socketKind: state.socketKind,
      transportKind: state.socketKind,
      gameCtlReady: !!ctl,
      availableMethods: collectAvailableMethods(),
      lastHelloAck: state.lastHelloAck,
      appPlatform: getAppPlatform(systemInfo),
      systemInfo: sanitizeSystemInfo(systemInfo),
      scriptHash: getScriptHash(),
      hostVersion: "farm-go-host-1"
    };
  }

  function normalizeMessageData(raw) {
    if (typeof raw === "string") return raw;
    if (raw && typeof raw.data === "string") return raw.data;
    if (raw && raw.data && typeof TextDecoder === "function" && raw.data instanceof ArrayBuffer) {
      try {
        return new TextDecoder("utf-8").decode(new Uint8Array(raw.data));
      } catch (_) {}
    }
    if (raw && raw.data && typeof ArrayBuffer !== "undefined" && raw.data instanceof ArrayBuffer) {
      try {
        return String.fromCharCode.apply(null, new Uint8Array(raw.data));
      } catch (_) {}
    }
    return String(raw && raw.data != null ? raw.data : raw);
  }

  function parsePacket(raw) {
    return JSON.parse(normalizeMessageData(raw));
  }

  function sendPacket(packet) {
    if (!state.socket) return false;
    var text = JSON.stringify(packet);
    try {
      if (state.socketKind === "websocket") {
        if (state.socket.readyState !== 1) return false;
        state.socket.send(text);
        return true;
      }
      if (state.socketKind === "socketTask") {
        state.socket.send({ data: text });
        return true;
      }
    } catch (error) {
      state.lastError = error && error.message ? error.message : String(error);
      logLocal("error", "send failed", state.lastError);
    }
    return false;
  }

  function sendTyped(type, payload, id) {
    return sendPacket({
      id: id || nextId(type),
      type: type,
      ts: now(),
      payload: payload || {}
    });
  }

  function sendLog(level, message, extra) {
    logLocal(level, message, extra);
    var payload = {
      level: level,
      message: message,
      extra: extra === undefined ? null : extra
    };
    if (sendTyped("log", payload)) return true;
    if (state.pendingLogs.length >= pendingLogLimit) state.pendingLogs.shift();
    state.pendingLogs.push(payload);
    return false;
  }

  function flushPendingLogs() {
    while (state.pendingLogs.length > 0) {
      if (!sendTyped("log", state.pendingLogs[0])) return;
      state.pendingLogs.shift();
    }
  }

  function sendHello() {
    var systemInfo = getSystemInfo();
    return sendTyped("hello", {
      client: "qq-miniapp",
      app: "qq-farm",
      version: "farm-go-host-1",
      gameCtlReady: !!getGameCtl(),
      availableMethods: collectAvailableMethods(),
      socketKind: state.socketKind,
      transportKind: state.socketKind,
      appPlatform: getAppPlatform(systemInfo),
      systemInfo: sanitizeSystemInfo(systemInfo),
      scriptHash: getScriptHash()
    });
  }

  function sendPong(requestId) {
    return sendTyped("pong", {}, requestId || nextId("pong"));
  }

  function sendReadyEvent(ready) {
    return sendTyped("event", {
      name: "gameCtlReadyChanged",
      ready: !!ready,
      availableMethods: collectAvailableMethods(),
      scriptHash: getScriptHash()
    });
  }

  function sendEvent(name, payload) {
    var data = payload && typeof payload === "object" ? payload : {};
    data.name = String(name || data.name || "runtimeEvent");
    return sendTyped("event", data);
  }

  function withTimeout(promise, timeoutMs) {
    return new Promise(function (resolve, reject) {
      var done = false;
      var timer = setTimeout(function () {
        if (done) return;
        done = true;
        reject(new Error("timeout"));
      }, Math.max(1, Number(timeoutMs) || defaults.callTimeoutMs));

      Promise.resolve(promise).then(function (value) {
        if (done) return;
        done = true;
        clearTimeout(timer);
        resolve(value);
      }, function (error) {
        if (done) return;
        done = true;
        clearTimeout(timer);
        reject(error);
      });
    });
  }

  function invokeAllowed(pathName, args) {
    pathName = String(pathName || "");
    if (hostPathMap[pathName]) {
      if (pathName === "host.ping") {
        return {
          pong: true,
          now: new Date().toISOString(),
          gameCtlReady: !!getGameCtl(),
          phase: state.phase
        };
      }
      if (pathName === "host.describe") {
        return getStatus();
      }
    }

    if (pathName.indexOf("gameCtl.") !== 0) {
      throw new Error("call_path_not_allowed: " + pathName);
    }

    var ctl = getGameCtl();
    if (!ctl) {
      throw new Error("gameCtl_not_ready");
    }

    var methodName = pathName.slice("gameCtl.".length);
    if (!methodName || typeof ctl[methodName] !== "function") {
      throw new Error("call_path_not_ready: " + pathName);
    }
    return ctl[methodName].apply(ctl, Array.isArray(args) ? args : []);
  }

  function sendResult(requestId, pathName, ok, data, error) {
    return sendTyped("result", {
      ok: !!ok,
      path: pathName || null,
      data: ok ? data : null,
      error: ok ? null : String(error || "unknown_error")
    }, requestId);
  }

  function handleCall(packet) {
    var payload = packet && packet.payload && typeof packet.payload === "object" ? packet.payload : {};
    var pathName = String(payload.path || "");
    var args = Array.isArray(payload.args) ? payload.args : [];
    var timeoutMs = Math.max(1000, Number(payload.timeoutMs) || defaults.callTimeoutMs);

    withTimeout(Promise.resolve().then(function () {
      return invokeAllowed(pathName, args);
    }), timeoutMs).then(function (result) {
      sendResult(packet.id, pathName, true, result, null);
    }, function (error) {
      sendLog("warn", "call failed", {
        path: pathName,
        error: error && error.message ? error.message : String(error)
      });
      sendResult(
        packet.id,
        pathName,
        false,
        null,
        error && error.message ? error.message : String(error)
      );
    });
  }

  function scheduleReconnect(reason) {
    clearTimer("reconnectTimer");
    if (state.manualStop) {
      return;
    }
    state.reconnectTimer = setTimeout(function () {
      sendLog("info", "reconnecting", { reason: reason || "unknown" });
      connect(state.url);
    }, defaults.reconnectMs);
  }

  function closeCurrentSocket() {
    if (!state.socket) return;
    try {
      if (state.socketKind === "websocket" && typeof state.socket.close === "function") {
        state.socket.close();
      } else if (state.socketKind === "socketTask" && typeof state.socket.close === "function") {
        state.socket.close({});
      }
    } catch (_) {}
    state.socket = null;
    state.socketKind = null;
  }

  function handleOpen(kind, rawSocket) {
    clearTimer("reconnectTimer");
    state.socket = rawSocket;
    state.socketKind = kind;
    state.lastError = null;
    setPhase("connected");
    sendLog("info", "socket connected", { kind: kind, url: state.url });
    sendHello();
    flushPendingLogs();
    startHeartbeat();
  }

  function handleClose(kind, detail) {
    setPhase("disconnected");
    stopHeartbeat();
    state.socket = null;
    state.socketKind = kind || state.socketKind;
    sendLog("warn", "socket closed", detail || null);
    scheduleReconnect("closed");
  }

  function handleError(kind, error) {
    state.lastError = error && error.message ? error.message : String(error);
    sendLog("error", "socket error", {
      kind: kind,
      error: state.lastError
    });
  }

  function handleIncoming(packet) {
    if (!packet || typeof packet !== "object") {
      return;
    }
    if (packet.type === "helloAck") {
      state.lastHelloAck = packet.payload || {};
      setPhase("ready");
      sendLog("info", "hello ack", state.lastHelloAck);
      return;
    }
    if (packet.type === "ping") {
      sendPong(packet.id);
      return;
    }
    if (packet.type === "pong") {
      return;
    }
    if (packet.type === "call") {
      handleCall(packet);
      return;
    }
  }

  function openWithWebSocket(url) {
    if (typeof root.WebSocket !== "function") {
      return false;
    }

    var ws;
    try {
      ws = new root.WebSocket(url);
    } catch (error) {
      handleError("websocket", error);
      return false;
    }

    ws.onopen = function () {
      handleOpen("websocket", ws);
    };
    ws.onmessage = function (event) {
      try {
        handleIncoming(parsePacket(event.data));
      } catch (error) {
        handleError("websocket", error);
      }
    };
    ws.onerror = function (error) {
      handleError("websocket", error);
    };
    ws.onclose = function (event) {
      handleClose("websocket", {
        code: event && event.code,
        reason: event && event.reason
      });
    };
    return true;
  }

  function openWithMiniSocket(url) {
    if (!mini || typeof mini.connectSocket !== "function") {
      return false;
    }

    var task;
    try {
      task = mini.connectSocket({ url: url });
    } catch (error) {
      handleError("socketTask", error);
      return false;
    }

    if (!task || typeof task.onOpen !== "function") {
      handleError("socketTask", "connectSocket returned invalid task");
      return false;
    }

    task.onOpen(function () {
      handleOpen("socketTask", task);
    });
    task.onMessage(function (event) {
      try {
        handleIncoming(parsePacket(event));
      } catch (error) {
        handleError("socketTask", error);
      }
    });
    task.onError(function (error) {
      handleError("socketTask", error);
    });
    task.onClose(function (event) {
      handleClose("socketTask", event || null);
    });
    return true;
  }

  function canStartOnCurrentPlatform(force) {
    if (force) return true;
    if (!defaults.expectedAppPlatform) return true;
    var platform = getAppPlatform();
    if (!platform || platform === "unknown") return true;
    return platform === defaults.expectedAppPlatform;
  }

  function connect(url, force) {
    if (url) {
      state.url = String(url);
    }
    if (!canStartOnCurrentPlatform(!!force)) {
      setPhase("platform_mismatch");
      logLocal("warn", "skip start on platform", getAppPlatform());
      return false;
    }

    closeCurrentSocket();
    setPhase("connecting");

    // QQ/微信小游戏环境：优先走运行时提供的 connectSocket，避免全局 WebSocket 被安全策略拦截导致连不上本机网关
    if (mini && typeof mini.connectSocket === "function" && openWithMiniSocket(state.url)) {
      return true;
    }
    if (openWithWebSocket(state.url)) {
      return true;
    }
    if (openWithMiniSocket(state.url)) {
      return true;
    }

    setPhase("unavailable");
    sendLog("error", "no websocket api available");
    return false;
  }

  function startHeartbeat() {
    stopHeartbeat();
    state.heartbeatTimer = setInterval(function () {
      sendTyped("ping", {});
    }, defaults.heartbeatMs);
  }

  function stopHeartbeat() {
    clearTimer("heartbeatTimer");
  }

  function startReadyPoll() {
    clearTimer("readyPollTimer");
    state.lastGameCtlReady = !!getGameCtl();
    state.readyPollTimer = setInterval(function () {
      var ready = !!getGameCtl();
      if (ready === state.lastGameCtlReady) {
        return;
      }
      state.lastGameCtlReady = ready;
      sendReadyEvent(ready);
      if (ready) {
        sendHello();
      }
    }, defaults.readyPollMs);
  }

  function stop() {
    state.manualStop = true;
    clearTimer("reconnectTimer");
    stopHeartbeat();
    clearTimer("readyPollTimer");
    closeCurrentSocket();
    setPhase("stopped");
    logLocal("info", "stopped");
  }

  function start(url, opts) {
    opts = opts && typeof opts === "object" ? opts : {};
    state.manualStop = false;
    if (opts.url) {
      state.url = String(opts.url);
    } else if (url) {
      state.url = String(url);
    }
    if (!state.readyPollTimer) {
      startReadyPoll();
    }
    return connect(state.url, !!opts.force);
  }

  function configure(next) {
    if (!next || typeof next !== "object") return getStatus();
    if (next.url) {
      state.url = String(next.url);
    }
    if (next.reconnectMs != null) {
      defaults.reconnectMs = Math.max(500, Number(next.reconnectMs) || defaults.reconnectMs);
    }
    if (next.heartbeatMs != null) {
      defaults.heartbeatMs = Math.max(1000, Number(next.heartbeatMs) || defaults.heartbeatMs);
    }
    if (next.callTimeoutMs != null) {
      defaults.callTimeoutMs = Math.max(1000, Number(next.callTimeoutMs) || defaults.callTimeoutMs);
    }
    if (next.readyPollMs != null) {
      defaults.readyPollMs = Math.max(300, Number(next.readyPollMs) || defaults.readyPollMs);
      if (state.readyPollTimer) {
        startReadyPoll();
      }
    }
    if (typeof next.expectedAppPlatform === "string") {
      defaults.expectedAppPlatform = next.expectedAppPlatform || null;
    }
    return getStatus();
  }

  var farmGoHost = {
    __installed: true,
    __hostVersion: "farm-go-host-1",
    __bundleHash: "farm-go-debug-link",
    defaults: defaults,
    start: start,
    stop: stop,
    connect: connect,
    configure: configure,
    status: getStatus,
    sendHello: sendHello,
    sendEvent: sendEvent,
    invokeLocal: invokeAllowed,
    logLocal: logLocal,
    // Exposed so sub-IIFEs (e.g. TSDK-BLOCK) can route logs through the
    // WebSocket link to the Farm_Go debug terminal.
    log: sendLog
  };
  root.__farmGoQqHost = farmGoHost;
  root.__qqFarmHost = farmGoHost;

  startReadyPoll();
  if (defaults.autoStart) {
    start(defaults.url);
  }
})();

// >>> TSDK-BLOCK START >>>
// Layer1a: intercept wx/qq .request to block TSDK telemetry uploads.
// Layer1b: intercept wx.connectSocket/uploadFile/downloadFile (ACEVM bypass routes).
// Layer2:  selectively disable TSDK detection features (IDs 1,2,3,6,7).
//          IDs 4=TOKEN_GET_CHECK and 5=DATA_ENCRYPT are kept at 100 (auth-critical).
//          IDs 1001-1005 are always-enabled and silently ignore set calls.
// Safe-to-disable IDs confirmed from tsdk.decrypted.wasm binary (0x6140 table):
//   1=SPEED_CHECK  2=VARIABLE_CHECK  3=MINI_GAME_ENV_CHECK
//   6=TUCH_CHECK   7=STACK_HASH_REPORT
;(function () {
  var root = typeof globalThis !== "undefined" ? globalThis : Function("return this")();

  // Route log both to console AND through the Farm_Go WebSocket link so it
  // appears in the Wails dev terminal. Falls back to console-only if the
  // host isn't ready yet.
  function tsdkLog(level, message, extra) {
    try { console.log("[TSDK-BLOCK][" + level + "] " + message, extra !== undefined ? extra : ""); } catch (_) {}
    try {
      var host = root.__farmGoQqHost;
      if (host && typeof host.log === "function") {
        host.log(level, "[TSDK-BLOCK] " + message, extra !== undefined ? extra : undefined);
      }
    } catch (_) {}
  }

  // All known Tencent security / telemetry hosts.  Keep in sync with
  // tsdkHosts in internal/runtime/wmpf/link.go.
  var TSDK_HOSTS = [
    "anticheatexpert",
    "tss.qq.com",
    "btrace.qq.com",
    "beacon.qq.com",
    "bcs.qq.com",
    "aegis.qq.com",
    "hisvc.qq.com",
    "speed.qq.com",
    "trace.qq.com"
  ];
  function isTsdkUrl(url) {
    for (var i = 0; i < TSDK_HOSTS.length; i++) {
      if (url.indexOf(TSDK_HOSTS[i]) !== -1) return true;
    }
    return false;
  }

  // --- Layer1a: wx/qq .request ---
  // Blocks all known TSDK telemetry upload domains; all other
  // traffic (login, heartbeat, game data) passes through untouched.
  function interceptRequest(obj, name) {
    if (!obj || typeof obj.request !== "function" || obj.__tsdkBlocked) return;
    var _orig = obj.request;
    obj.request = function (opts) {
      if (opts && typeof opts.url === "string" && isTsdkUrl(opts.url)) {
        tsdkLog("warn", "upload blocked on " + name + ".request", { url: opts.url });
        // Return a silent success so TSDK doesn't retry or flag env as broken
        if (typeof opts.success === "function") { try { opts.success({ data: {} }); } catch (_) {} }
        return;
      }
      return _orig.apply(this, arguments);
    };
    obj.__tsdkBlocked = true;
  }

  // --- Layer1b: wx secondary egress (ACEVM bypass routes) ---
  // ACEVM bytecode runs with wx in scope and could call connectSocket/uploadFile/
  // downloadFile to exfiltrate data outside the .request path.
  function interceptWxEgress(obj) {
    if (!obj || obj.__tsdkEgressBlocked) return;
    var _origConnect = obj.connectSocket;
    if (typeof _origConnect === "function") {
      obj.connectSocket = function (opts) {
        if (opts && typeof opts.url === "string" && isTsdkUrl(opts.url)) {
          tsdkLog("warn", "connectSocket blocked", { url: opts.url });
          if (typeof opts.fail === "function") { try { opts.fail({ errMsg: "blocked" }); } catch (_) {} }
          return {};
        }
        return _origConnect.apply(this, arguments);
      };
    }
    var _origUpload = obj.uploadFile;
    if (typeof _origUpload === "function") {
      obj.uploadFile = function (opts) {
        if (opts && typeof opts.url === "string" && isTsdkUrl(opts.url)) {
          tsdkLog("warn", "uploadFile blocked", { url: opts.url });
          if (typeof opts.fail === "function") { try { opts.fail({ errMsg: "blocked" }); } catch (_) {} }
          return {};
        }
        return _origUpload.apply(this, arguments);
      };
    }
    var _origDownload = obj.downloadFile;
    if (typeof _origDownload === "function") {
      obj.downloadFile = function (opts) {
        if (opts && typeof opts.url === "string" && isTsdkUrl(opts.url)) {
          tsdkLog("warn", "downloadFile blocked", { url: opts.url });
          if (typeof opts.fail === "function") { try { opts.fail({ errMsg: "blocked" }); } catch (_) {} }
          return {};
        }
        return _origDownload.apply(this, arguments);
      };
    }
    obj.__tsdkEgressBlocked = true;
  }

  interceptRequest(root.wx, "wx");
  interceptRequest(root.qq, "qq");
  try { interceptWxEgress(root.wx); } catch (_interceptErr) {}

  // Immediate startup confirmation — fires as soon as the script is injected.
  // Appears in the TSDK panel within milliseconds; absence means injection failed.
  tsdkLog("info", "TSDK-BLOCK v2 init", {
    layer1a: !!(root.wx && root.wx.__tsdkBlocked),
    layer1b: !!(root.wx && root.wx.__tsdkEgressBlocked),
    layer2: "pending"
  });

  // --- Layer2: selective FeatureManager disable ---
  var TSDK_SAFE_DISABLE_IDS = [1, 2, 3, 6, 7];
  var _layer2Applied = false;
  function applyLayer2() {
    if (_layer2Applied) return;
    var tsdk = root.globalThis && root.globalThis.tsdk;
    if (!tsdk) tsdk = root.tsdk;
    if (!tsdk || typeof tsdk._set_feature_gray_value !== "function") return;
    try {
      for (var i = 0; i < TSDK_SAFE_DISABLE_IDS.length; i++) {
        tsdk._set_feature_gray_value(TSDK_SAFE_DISABLE_IDS[i], 0);
      }
      _layer2Applied = true;
      tsdkLog("info", "Layer2 applied: detection features disabled", {
        disabled: TSDK_SAFE_DISABLE_IDS,
        kept: [4, 5]
      });
    } catch (e) {
      tsdkLog("warn", "Layer2 apply failed", { err: String(e) });
    }
  }

  // Poll until globalThis.tsdk is ready (WASM loads asynchronously).
  var _layer2Attempts = 0;
  var _layer2MaxAttempts = 60; // 60 × 500 ms = 30 s
  var _layer2Timer = setInterval(function () {
    _layer2Attempts++;
    applyLayer2();
    if (_layer2Applied || _layer2Attempts >= _layer2MaxAttempts) {
      clearInterval(_layer2Timer);
      if (!_layer2Applied) {
        tsdkLog("warn", "Layer2 gave up after " + _layer2Attempts + " attempts");
      }
    }
  }, 500);

  // Defer the "ready" notification — at this point the host IIFE has just
  // started its WebSocket dial; give it ~2 s to establish before logging.
  setTimeout(function () {
    var wxOk = !!(root.wx && root.wx.__tsdkBlocked);
    var qqOk = !!(root.qq && root.qq.__tsdkBlocked);
    var wxEgressOk = !!(root.wx && root.wx.__tsdkEgressBlocked);
    tsdkLog("info", "Layer1 interceptors ready", { wx: wxOk, qq: qqOk, wxEgress: wxEgressOk });
  }, 2000);
})();
// <<< TSDK-BLOCK END <<<
