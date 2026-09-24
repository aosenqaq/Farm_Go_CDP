package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readWMPFButtonScript(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("resources", "wmpf", "button.js"))
	if err != nil {
		t.Fatalf("read button.js: %v", err)
	}
	return string(raw)
}

func TestWarehouseProtocolSellRequiresUIDBeforeDirectDispatch(t *testing.T) {
	script := readWMPFButtonScript(t)
	uidGuard := strings.Index(script, "payload.protocolError = 'protocol_sell_uid_missing'")
	dispatch := strings.Index(script, "const protocolSell = await sellWarehouseItemsByProtocol")

	if uidGuard < 0 {
		t.Fatalf("expected protocol sell uid guard")
	}
	if dispatch < 0 {
		t.Fatalf("expected protocol sell dispatch")
	}
	if uidGuard > dispatch {
		t.Fatalf("uid guard should run before direct protocol sell dispatch")
	}
	if !strings.Contains(script, "Number(entry.request.uid) > 0") {
		t.Fatalf("uid guard should require a positive request uid")
	}
}

func TestWarehouseProtocolSellDoesNotTreatCallbackAsSuccess(t *testing.T) {
	script := readWMPFButtonScript(t)
	start := strings.Index(script, "payload.ok = payload.observedChange === true")
	end := strings.Index(script[start:], "if (!payload.ok) payload.reason = 'protocol_sell_result_not_observed'")

	if start < 0 || end < 0 {
		t.Fatalf("expected protocol sell success predicate")
	}
	predicate := script[start : start+end]
	if strings.Contains(predicate, "callbackCalled") {
		t.Fatalf("protocol sell success should not rely on a bare callback")
	}
	if !strings.Contains(predicate, "payload.protocolRewards") || !strings.Contains(predicate, "payload.protocolSoldItems") {
		t.Fatalf("protocol sell success should require rewards or sold items when no stock diff is observed")
	}
}

func TestBackpackSeedFallbackSkipsProtocolWarehouseRefresh(t *testing.T) {
	script := readWMPFButtonScript(t)
	start := strings.Index(script, "async function getWarehouseBackpackSeeds")
	end := strings.Index(script[start:], "async function getAllSeeds")

	if start < 0 || end < 0 {
		t.Fatalf("expected getWarehouseBackpackSeeds block")
	}
	block := script[start : start+end]
	if !strings.Contains(block, "preferProtocol: false") {
		t.Fatalf("seed UI fallback should bypass protocol warehouse refresh")
	}
}
