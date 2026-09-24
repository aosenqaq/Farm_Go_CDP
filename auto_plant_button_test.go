package main

import (
	"strings"
	"testing"
)

func TestAutoPlantPreservesIncompleteFourGridReasonBeforeVerification(t *testing.T) {
	script := readWMPFButtonScript(t)
	start := strings.Index(script, "async function plantSeedsOnLandsVerified")
	end := strings.Index(script[start:], "async function autoPlant")
	if start < 0 || end < 0 {
		t.Fatalf("expected verified planting block")
	}
	block := script[start : start+end]
	guard := strings.Index(block, "plantResult.reason === 'no_complete_multi_land_group'")
	waitAfterDispatch := strings.Index(block, "await wait(waitAfterPlantMs)")
	if guard < 0 || waitAfterDispatch < 0 || guard > waitAfterDispatch {
		t.Fatalf("incomplete four-grid reason must return before verification")
	}
	if !strings.Contains(block[guard:waitAfterDispatch], "reason: plantResult.reason") {
		t.Fatalf("expected structural planting reason to be preserved")
	}
	if !strings.Contains(block, "plantResult = await plantSeedsOnLands(candidate, targetLandIds, opts);") {
		t.Fatalf("verified planting must await the paced dispatcher")
	}
}

func TestAutoPlantStillReportsPostDispatchVerificationFailure(t *testing.T) {
	if !strings.Contains(readWMPFButtonScript(t), "reason: 'plant_verify_failed'") {
		t.Fatalf("expected post-dispatch verification failure to remain")
	}
}
