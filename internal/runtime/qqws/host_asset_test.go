package qqws

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestQQHostAssetIsAdaptedForFarmGo(t *testing.T) {
	content, err := os.ReadFile("../../../resources/qq/qq-host.js")
	if err != nil {
		t.Fatalf("read qq host asset: %v", err)
	}
	text := string(content)

	for _, placeholder := range []string{
		"__QQ_FARM_HOST_RPC_PATHS__",
		"__QQ_FARM_HOST_WS_URL__",
		"__QQ_FARM_BUNDLE_HASH__",
		"__QQ_FARM_HOST_VERSION__",
	} {
		if strings.Contains(text, placeholder) {
			t.Fatalf("asset still contains placeholder %s", placeholder)
		}
	}

	if !strings.Contains(text, `url: "ws://127.0.0.1:8787/runtime/qqws"`) {
		t.Fatal("asset should point to Farm_Go local qqws URL")
	}
	if !strings.Contains(text, `var hostPaths = ["host.ping", "host.describe", "gameCtl.probe"];`) {
		t.Fatal("asset should limit first-stage RPC paths")
	}
	if !strings.Contains(text, `root.__farmGoQqHost = farmGoHost;`) {
		t.Fatal("asset should use a Farm_Go-specific runtime singleton")
	}
	if strings.Contains(text, `root.__qqFarmHost && root.__qqFarmHost.__installed`) {
		t.Fatal("asset should not be blocked by the reference project's singleton")
	}
	if !strings.Contains(text, `hostVersion: "farm-go-host-1"`) {
		t.Fatal("asset should report Farm_Go host version")
	}
}

func TestQQHostQueuesTSDKInitLogUntilSocketOpen(t *testing.T) {
	assetPath, err := filepath.Abs("../../../resources/qq/qq-host.js")
	if err != nil {
		t.Fatalf("resolve QQ host asset: %v", err)
	}
	script := `
const fs = require("node:fs");
const vm = require("node:vm");
const source = fs.readFileSync(process.argv[1], "utf8");
const sent = [];
let open;
const socket = {
  onOpen(fn) { open = fn; }, onMessage() {}, onError() {}, onClose() {},
  send(value) { sent.push(JSON.parse(value.data)); }, close() {}
};
const sandbox = {
  qq: {
    connectSocket() { return socket; }, request() {},
    getSystemInfoSync() { return { AppPlatform: "qq" }; }
  },
  console: { log() {} }, Date, Math, JSON,
  setTimeout() { return 1; }, clearTimeout() {},
  setInterval() { return 1; }, clearInterval() {}
};
vm.runInNewContext(source, sandbox);
const initLogs = () => sent.filter((packet) =>
  packet.type === "log" && packet.payload.message.includes("TSDK-BLOCK v2 init")
);
if (initLogs().length !== 0) throw new Error("init log sent before socket open");
open();
if (initLogs().length !== 1) throw new Error("init log count after open: " + initLogs().length);
`
	command := exec.Command("node", "-e", script, assetPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("QQ host queue behavior failed: %v\n%s", err, output)
	}
}
