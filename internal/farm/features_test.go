package farm

import "testing"

func TestCatalogCoversScreenshotBlocks(t *testing.T) {
	catalog := Catalog()
	want := []string{
		"auto_farm",
		"scheduler",
		"crop_analytics",
		"lands",
		"warehouse",
		"friends",
		"rankings",
		"guard",
		"atlas",
		"account",
		"logs",
		"message_push",
		"settings",
	}
	seen := map[string]bool{}
	for _, group := range catalog.Groups {
		for _, feature := range group.Features {
			seen[feature.ID] = true
		}
	}
	for _, id := range want {
		if !seen[id] {
			t.Fatalf("catalog missing %s", id)
		}
	}
}

func TestCatalogUsesGroupedNavigation(t *testing.T) {
	catalog := Catalog()
	got := make([]string, 0, len(catalog.Groups))
	for _, group := range catalog.Groups {
		got = append(got, group.ID)
	}
	want := []string{"workspace", "automation", "assets", "social", "system"}
	if len(got) != len(want) {
		t.Fatalf("group count = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("group[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
