package main

import (
	"os"
	"strings"
	"testing"
)

func TestPlayerProfileReaderCollectsUnderscoreExp(t *testing.T) {
	source, err := os.ReadFile("resources/wmpf/button.js")
	if err != nil {
		t.Fatalf("read player profile runtime source: %v", err)
	}
	if !strings.Contains(string(source), "['exp', '_exp', 'curExp', 'currentExp'") {
		t.Fatal("player profile reader must include the runtime _exp experience field")
	}
}
