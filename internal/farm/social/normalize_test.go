package social

import "testing"

func TestNormalizeFriendRulesDedupesListsAndScopesWithoutChoosingListPriority(t *testing.T) {
	rules := NormalizeFriendRules(FriendRules{
		WhitelistEnabled: true,
		WhitelistScopes:  []string{"steal", "help", "steal"},
		Whitelist:        []string{"10001", "10001", " Alice "},
		BlacklistEnabled: true,
		BlacklistScopes:  []string{"help", "mischief"},
		Blacklist:        []string{"10002", "10002"},
		MaskedBlacklist:  true,
		MaskedMaxLevel:   0,
	})

	if len(rules.WhitelistScopes) != 2 || rules.WhitelistScopes[0] != "steal" || rules.WhitelistScopes[1] != "help" {
		t.Fatalf("whitelist scopes = %#v", rules.WhitelistScopes)
	}
	if len(rules.BlacklistScopes) != 2 || rules.BlacklistScopes[0] != "help" || rules.BlacklistScopes[1] != "mischief" {
		t.Fatalf("blacklist scopes = %#v", rules.BlacklistScopes)
	}
	if len(rules.Whitelist) != 2 || rules.Whitelist[0] != "10001" || rules.Whitelist[1] != "Alice" {
		t.Fatalf("whitelist = %#v", rules.Whitelist)
	}
	if rules.MaskedMaxLevel != 1 {
		t.Fatalf("masked max level = %d, want 1", rules.MaskedMaxLevel)
	}
}

func TestNormalizeStringListParsesPastedRuleText(t *testing.T) {
	got := NormalizeStringList("10001, 10002，10001; Bob；Alice、Bob|Carol\nDave")
	want := []string{"10001", "10002", "Bob", "Alice", "Carol", "Dave"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("item %d = %q, want %q in %#v", index, got[index], want[index], got)
		}
	}
}

func TestProtectedFriendGIDsAreHardSkipped(t *testing.T) {
	for _, gid := range []any{1184649322, "1142601927"} {
		if !IsProtectedFriendGID(gid) {
			t.Fatalf("gid %v should be protected", gid)
		}
	}
	if IsProtectedFriendGID("10001") {
		t.Fatal("normal gid should not be protected")
	}
}
