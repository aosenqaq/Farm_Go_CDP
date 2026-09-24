package social

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestFriendActionProtectedGIDSkipsWithoutRuntimeCall(t *testing.T) {
	for _, action := range []string{"steal", "help", "mischief"} {
		caller := &fakeRuntimeCaller{responses: map[string]any{}}
		service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

		result := service.Action(context.Background(), FriendActionRequest{Action: action, Target: "1184649322"})

		if !result.OK || result.Status != StatusSkipped {
			t.Fatalf("action %s result = %#v", action, result)
		}
		if len(caller.calls) != 0 {
			t.Fatalf("protected action %s should not call runtime: %#v", action, caller.calls)
		}
	}
}

func TestFriendActionViewQQCallsGuardedRuntimeOperation(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.addFriendByGidDiagnostic": map[string]any{
			"ok": true, "invoked": true, "hostGid": float64(10001),
			"resolvedFrom": "basic.open_id",
		},
	}}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "view_qq", Target: "10001"})

	if !result.OK || result.Status != StatusOK || result.Message != "已请求打开 QQ 原生好友对话框。" {
		t.Fatalf("result = %#v", result)
	}
	wantArgs := []any{map[string]any{"hostGid": 10001, "verifyMsg": "来自QQ农场", "silent": true}}
	if len(caller.calls) != 1 || caller.calls[0].method != "gameCtl.addFriendByGidDiagnostic" || !reflect.DeepEqual(caller.calls[0].args, wantArgs) {
		t.Fatalf("calls = %#v, want args = %#v", caller.calls, wantArgs)
	}
	data := result.Data.(map[string]any)
	if !reflect.DeepEqual(data, map[string]any{"action": "view_qq", "gid": 10001, "invoked": true}) {
		t.Fatalf("data = %#v", data)
	}
}

func TestFriendActionViewQQDoesNotExposeRuntimePayloadOnFailure(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.addFriendByGidDiagnostic": map[string]any{
			"ok":     false,
			"reason": "reply_open_id_missing",
			"basic":  map[string]any{"open_id": "A1B2C3D4E5F60708192A3B4C5D6E7F80"},
		},
	}}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "view_qq", Target: "10001"})

	if result.OK || result.Status != StatusFailed || result.Message != "QQ 目标信息不完整，已阻止打开对话框。" || result.Data != nil {
		t.Fatalf("result = %#v", result)
	}
}

func TestFriendProtocolBlockListCallsRuntime(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendBlockListByProtocol": map[string]any{"list": []any{map[string]any{"gid": float64(10001), "name": "Blocked A"}}},
	}}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	result := service.ProtocolBlockList(context.Background(), false)

	if !result.OK || len(result.Friends) != 1 || result.Friends[0].GID != 10001 {
		t.Fatalf("block list = %#v", result)
	}
	if len(caller.calls) != 1 || caller.calls[0].method != "gameCtl.getFriendBlockListByProtocol" {
		t.Fatalf("calls = %#v", caller.calls)
	}
	wantArgs := []any{map[string]any{"dryRun": false, "debug": false, "silent": true}}
	if !reflect.DeepEqual(caller.calls[0].args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", caller.calls[0].args, wantArgs)
	}
}

func TestFriendActionUnblocksSelectedProtocolFriends(t *testing.T) {
	caller := &fakeRuntimeCaller{responseQueue: map[string][]any{
		"gameCtl.unblockFriendByProtocol": {
			map[string]any{"ok": true},
			errors.New("denied"),
			map[string]any{"ok": true},
		},
	}}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{
		Action:  "unblock_friend_batch",
		Targets: []string{" 10002 ", "bad", "10001", "10002", "0", "10003"},
	})

	if result.OK || result.Status != StatusFailed || !strings.Contains(result.Message, "成功 2 个，失败 1 个") {
		t.Fatalf("result = %#v", result)
	}
	if len(caller.calls) != 3 {
		t.Fatalf("calls = %#v", caller.calls)
	}
	wantGIDs := []int{10002, 10001, 10003}
	for index, gid := range wantGIDs {
		if caller.calls[index].method != "gameCtl.unblockFriendByProtocol" {
			t.Fatalf("call %d method = %q", index, caller.calls[index].method)
		}
		wantArgs := []any{map[string]any{"friendGid": gid, "dryRun": false, "silent": true}}
		if !reflect.DeepEqual(caller.calls[index].args, wantArgs) {
			t.Fatalf("call %d args = %#v, want %#v", index, caller.calls[index].args, wantArgs)
		}
	}
	data := result.Data.(map[string]any)
	if data["successCount"] != 2 || data["failureCount"] != 1 || !reflect.DeepEqual(data["targets"], wantGIDs) {
		t.Fatalf("data = %#v", data)
	}
	results := data["results"].([]map[string]any)
	if len(results) != 3 || results[1]["gid"] != 10001 || results[1]["ok"] != false {
		t.Fatalf("results = %#v", results)
	}
}

func TestFriendActionRejectsEmptyProtocolUnblockBatch(t *testing.T) {
	caller := &fakeRuntimeCaller{}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{
		Action:  "unblock_friend_batch",
		Targets: []string{"", "bad", "0", "-1"},
	})

	if result.OK || result.Status != StatusFailed || len(caller.calls) != 0 {
		t.Fatalf("result = %#v calls = %#v", result, caller.calls)
	}
}

func TestFriendActionTogglesLocalBlacklist(t *testing.T) {
	store := &memoryStore{rules: FriendRules{Blacklist: []string{"10001"}, MaskedBlacklist: true, MaskedMaxLevel: 1}}
	service := NewService(store, nil, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "blacklist_toggle", Target: "10002"})

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("result = %#v", result)
	}
	if len(store.rules.Blacklist) != 2 || store.rules.Blacklist[1] != "10002" {
		t.Fatalf("blacklist = %#v", store.rules.Blacklist)
	}
}

func TestFriendActionRemovesSelectedBlacklistRules(t *testing.T) {
	store := &memoryStore{rules: FriendRules{Blacklist: []string{"10001", "10002", "name-rule"}, Whitelist: []string{"10003"}, MaskedBlacklist: true, MaskedMaxLevel: 1}}
	service := NewService(store, nil, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "blacklist_remove_batch", Targets: []string{"10002", "missing"}})

	if !result.OK || !reflect.DeepEqual(store.rules.Blacklist, []string{"10001", "name-rule"}) || !reflect.DeepEqual(store.rules.Whitelist, []string{"10003"}) {
		t.Fatalf("result = %#v rules = %#v", result, store.rules)
	}
	data := result.Data.(map[string]any)
	if data["removedCount"] != 1 || data["missingCount"] != 1 {
		t.Fatalf("batch result = %#v", data)
	}
}

func TestFriendActionRemovesSelectedWhitelistRules(t *testing.T) {
	store := &memoryStore{rules: FriendRules{Blacklist: []string{"10001"}, Whitelist: []string{"10002", "10003"}, MaskedBlacklist: true, MaskedMaxLevel: 1}}
	service := NewService(store, nil, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "whitelist_remove_batch", Targets: []string{"10002"}})

	if !result.OK || !reflect.DeepEqual(store.rules.Blacklist, []string{"10001"}) || !reflect.DeepEqual(store.rules.Whitelist, []string{"10003"}) {
		t.Fatalf("result = %#v rules = %#v", result, store.rules)
	}
}

func TestFriendActionSavesRuleSettings(t *testing.T) {
	store := &memoryStore{rules: FriendRules{Blacklist: []string{"old"}, MaskedBlacklist: true, MaskedMaxLevel: 1}}
	service := NewService(store, nil, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{
		Action: "rules_save",
		Rules: &FriendRules{
			WhitelistEnabled: true,
			WhitelistScopes:  []string{"steal", "help"},
			Whitelist:        []string{"10001"},
			BlacklistEnabled: true,
			BlacklistScopes:  []string{"help", "mischief"},
			Blacklist:        []string{"10002"},
			MaskedBlacklist:  true,
			MaskedMaxLevel:   3,
		},
	})

	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	if !store.rules.WhitelistEnabled || !reflect.DeepEqual(store.rules.WhitelistScopes, []string{"steal", "help"}) {
		t.Fatalf("whitelist settings = %#v", store.rules)
	}
	if !reflect.DeepEqual(store.rules.BlacklistScopes, []string{"help", "mischief"}) {
		t.Fatalf("blacklist scopes = %#v", store.rules.BlacklistScopes)
	}
}

func TestFriendActionCleansInvalidNumericRuleItems(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{"list": []any{
			map[string]any{"gid": float64(10001), "name": "A"},
		}},
	}}
	store := &memoryStore{rules: FriendRules{Blacklist: []string{"10001", "99999", "name-rule"}, Whitelist: []string{"88888"}}}
	service := NewService(store, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "cleanup_invalid_rules"})

	if !result.OK || !reflect.DeepEqual(store.rules.Blacklist, []string{"10001", "name-rule"}) || len(store.rules.Whitelist) != 0 {
		t.Fatalf("result = %#v rules = %#v", result, store.rules)
	}
}

func TestFriendActionEnterCallsRuntime(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.enterFriendFarm": map[string]any{"ok": true},
	}}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "enter", Target: "10001"})

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("result = %#v", result)
	}
	if len(caller.calls) != 1 || caller.calls[0].method != "gameCtl.enterFriendFarm" {
		t.Fatalf("calls = %#v", caller.calls)
	}
}

func TestFriendActionBlocksSystemFriend(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.blockFriendByProtocol": map[string]any{"ok": true},
	}}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{
		Action: "block_friend",
		Target: "10001",
		DryRun: true,
	})

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("result = %#v", result)
	}
	if len(caller.calls) != 1 || caller.calls[0].method != "gameCtl.blockFriendByProtocol" {
		t.Fatalf("calls = %#v", caller.calls)
	}
	want := []any{map[string]any{"friendGid": 10001, "dryRun": true, "silent": true}}
	if !reflect.DeepEqual(caller.calls[0].args, want) {
		t.Fatalf("args = %#v, want %#v", caller.calls[0].args, want)
	}
}

func TestFriendActionStealUsesProtocolInspectAndHarvest(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"collect": []any{float64(9), "3", float64(9)}},
		},
		"gameCtl.friendHarvestLandsByProtocol": map[string]any{"ok": true},
	}}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "steal", Target: "10001"})

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("result = %#v", result)
	}
	gotMethods := calledMethods(caller.calls)
	wantMethods := []string{"gameCtl.inspectFriendFarmByProtocol", "gameCtl.friendHarvestLandsByProtocol"}
	if !reflect.DeepEqual(gotMethods, wantMethods) {
		t.Fatalf("methods = %#v, want %#v", gotMethods, wantMethods)
	}
	wantInspectArgs := []any{map[string]any{
		"hostGid":      10001,
		"silent":       true,
		"includeLands": true,
		"leaveAfter":   false,
		"source":       "farm_go_social_steal",
	}}
	if !reflect.DeepEqual(caller.calls[0].args, wantInspectArgs) {
		t.Fatalf("inspect args = %#v, want %#v", caller.calls[0].args, wantInspectArgs)
	}
	wantArgs := []any{map[string]any{
		"hostGid":     10001,
		"landIds":     []int{3, 9},
		"isAll":       false,
		"silent":      true,
		"waitReplyMs": 1200,
		"source":      "farm_go_social_steal",
	}}
	if !reflect.DeepEqual(caller.calls[1].args, wantArgs) {
		t.Fatalf("harvest args = %#v, want %#v", caller.calls[1].args, wantArgs)
	}
}

func TestFriendActionStealFarmStrategySkipsWholeFarmWhenAnyCropBlacklisted(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"collect": []any{float64(3), float64(9)}},
			"lands": []any{
				map[string]any{"landId": float64(3), "plantInfo": map[string]any{"id": float64(2001), "name": "百香果"}},
				map[string]any{"landId": float64(9), "plantInfo": map[string]any{"id": float64(3001), "name": "胡萝卜"}},
			},
		},
		"gameCtl.friendHarvestLandsByProtocol": map[string]any{"ok": true},
	}}
	store := &memoryStore{}
	service := NewService(store, caller, Options{
		AccountKey: "account-a",
		AutomationConfig: map[string]any{
			"autoFarmFriendStealPlantBlacklistEnabled":  true,
			"autoFarmFriendStealPlantListMode":          "blacklist",
			"autoFarmFriendStealPlantBlacklist":         []int{2001},
			"autoFarmFriendStealPlantBlacklistStrategy": 1,
		},
	})

	result := service.Action(context.Background(), FriendActionRequest{Action: "steal", Target: "10001"})

	if !result.OK || !strings.Contains(result.Message, "黑名单") {
		t.Fatalf("blacklist farm strategy should skip the whole farm, got %#v", result)
	}
	if gotMethods := calledMethods(caller.calls); !reflect.DeepEqual(gotMethods, []string{
		"gameCtl.inspectFriendFarmByProtocol",
	}) {
		t.Fatalf("methods = %#v", gotMethods)
	}
	if len(store.stealRecords) != 1 || store.stealRecords[0].Action != "steal_blacklist_skip" || !reflect.DeepEqual(store.stealRecords[0].LandIDs, []int{3, 9}) {
		t.Fatalf("blacklist skip record = %#v", store.stealRecords)
	}
}

func TestFriendActionStealCropStrategyFiltersOnlyBlacklistedCrops(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"collect": []any{float64(3), float64(9)}},
			"lands": []any{
				map[string]any{"landId": float64(3), "plantInfo": map[string]any{"id": float64(2001), "name": "百香果"}},
				map[string]any{"landId": float64(9), "plantInfo": map[string]any{"id": float64(3001), "name": "胡萝卜"}},
			},
		},
		"gameCtl.friendHarvestLandsByProtocol": map[string]any{"ok": true},
	}}
	service := NewService(&memoryStore{}, caller, Options{
		AccountKey: "account-a",
		AutomationConfig: map[string]any{
			"autoFarmFriendStealPlantBlacklistEnabled":  true,
			"autoFarmFriendStealPlantListMode":          "blacklist",
			"autoFarmFriendStealPlantBlacklist":         []int{2001},
			"autoFarmFriendStealPlantBlacklistStrategy": 2,
		},
	})

	result := service.Action(context.Background(), FriendActionRequest{Action: "steal", Target: "10001"})

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("result = %#v", result)
	}
	wantArgs := []any{map[string]any{
		"hostGid":     10001,
		"landIds":     []int{9},
		"isAll":       false,
		"silent":      true,
		"waitReplyMs": 1200,
		"source":      "farm_go_social_steal",
	}}
	if !reflect.DeepEqual(caller.calls[1].args, wantArgs) {
		t.Fatalf("harvest args = %#v, want %#v", caller.calls[1].args, wantArgs)
	}
}

func TestFriendActionStealWhitelistFarmStrategyReportsWhitelist(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"collect": []any{float64(3), float64(9)}},
			"lands": []any{
				map[string]any{"landId": float64(3), "plantInfo": map[string]any{"id": float64(2001)}},
				map[string]any{"landId": float64(9), "plantInfo": map[string]any{"id": float64(3001)}},
			},
		},
	}}
	service := NewService(&memoryStore{}, caller, Options{
		AccountKey: "account-a",
		AutomationConfig: map[string]any{
			"autoFarmFriendStealPlantBlacklistEnabled":  true,
			"autoFarmFriendStealPlantListMode":          "whitelist",
			"autoFarmFriendStealPlantWhitelist":         []int{3001},
			"autoFarmFriendStealPlantBlacklistStrategy": 1,
		},
	})

	result := service.Action(context.Background(), FriendActionRequest{Action: "steal", Target: "10001"})

	if !result.OK || !strings.Contains(result.Message, "白名单") {
		t.Fatalf("whitelist farm strategy should report whitelist, got %#v", result)
	}
	if gotMethods := calledMethods(caller.calls); !reflect.DeepEqual(gotMethods, []string{
		"gameCtl.inspectFriendFarmByProtocol",
	}) {
		t.Fatalf("methods = %#v", gotMethods)
	}
}

func TestFriendActionStealSkipsBlacklistedFriendForStealScope(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{"list": []any{
			map[string]any{"gid": float64(10001), "name": "A", "level": float64(12)},
		}},
		"gameCtl.enterFriendFarm": map[string]any{"ok": true},
	}}
	store := &memoryStore{rules: FriendRules{
		BlacklistEnabled: true,
		BlacklistScopes:  []string{"steal"},
		Blacklist:        []string{"10001"},
		MaskedBlacklist:  true,
		MaskedMaxLevel:   1,
	}}
	service := NewService(store, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "steal", Target: "10001"})

	if !result.OK || result.Status != StatusSkipped {
		t.Fatalf("blacklisted friend should be skipped, got %#v", result)
	}
	if methods := calledMethods(caller.calls); reflect.DeepEqual(methods, []string{"gameCtl.getFriendList", "gameCtl.enterFriendFarm"}) {
		t.Fatalf("should not enter blacklisted friend farm, calls = %#v", methods)
	}
}

func TestFriendActionHelpSkipsFriendOutsideWhitelistForHelpScope(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{"list": []any{
			map[string]any{"gid": float64(10001), "name": "A", "level": float64(12)},
		}},
		"gameCtl.enterFriendFarm": map[string]any{"ok": true},
	}}
	store := &memoryStore{rules: FriendRules{
		WhitelistEnabled: true,
		WhitelistScopes:  []string{"help"},
		Whitelist:        []string{"10002"},
		MaskedBlacklist:  true,
		MaskedMaxLevel:   1,
	}}
	service := NewService(store, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "help", Target: "10001"})

	if !result.OK || result.Status != StatusSkipped {
		t.Fatalf("non-whitelisted friend should be skipped, got %#v", result)
	}
	if methods := calledMethods(caller.calls); reflect.DeepEqual(methods, []string{"gameCtl.getFriendList", "gameCtl.enterFriendFarm"}) {
		t.Fatalf("should not enter non-whitelisted friend farm, calls = %#v", methods)
	}
}

func TestFriendActionMischiefAllowsWhitelistedFriendForMischiefScope(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{"list": []any{
			map[string]any{"gid": float64(10001), "name": "A", "level": float64(12)},
		}},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"bug": []any{float64(9)}},
		},
		"gameCtl.friendMischiefLandsBatch": map[string]any{"ok": true},
	}}
	store := &memoryStore{rules: FriendRules{
		WhitelistEnabled: true,
		WhitelistScopes:  []string{"mischief"},
		Whitelist:        []string{"10001"},
		MaskedBlacklist:  true,
		MaskedMaxLevel:   1,
	}}
	service := NewService(store, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "mischief", Target: "10001"})

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("whitelisted friend should be allowed, got %#v", result)
	}
}

func TestFriendActionHelpChecksFriendFarmWorkBeforeTrigger(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"farming": []any{float64(9), "3", float64(9)}},
		},
		"gameCtl.friendFarmingByProtocol": map[string]any{"ok": true},
	}}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "help", Target: "10001"})

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("result = %#v", result)
	}
	gotMethods := calledMethods(caller.calls)
	wantMethods := []string{"gameCtl.inspectFriendFarmByProtocol", "gameCtl.friendFarmingByProtocol"}
	if !reflect.DeepEqual(gotMethods, wantMethods) {
		t.Fatalf("methods = %#v, want %#v", gotMethods, wantMethods)
	}
	wantArgs := []any{map[string]any{
		"hostGid":     10001,
		"landIds":     []int{3, 9},
		"source":      0,
		"silent":      true,
		"waitReplyMs": 1200,
	}}
	if !reflect.DeepEqual(caller.calls[1].args, wantArgs) {
		t.Fatalf("help args = %#v, want %#v", caller.calls[1].args, wantArgs)
	}
}

func TestFriendActionMischiefPassesRuntimeBatchArguments(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"bug": []any{float64(9), "3"}, "grass": []any{float64(5), "3"}},
		},
		"gameCtl.friendMischiefLandsBatch": map[string]any{"ok": true},
	}}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "mischief", Target: "10001"})

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("result = %#v", result)
	}
	wantArgs := []any{map[string]any{
		"hostGid":      10001,
		"bugLandIds":   []int{3, 9},
		"grassLandIds": []int{3, 5},
		"dryRun":       false,
		"silent":       true,
		"source":       "farm_go_social_mischief",
	}}
	if !reflect.DeepEqual(caller.calls[1].args, wantArgs) {
		t.Fatalf("mischief args = %#v, want %#v", caller.calls[1].args, wantArgs)
	}
}

func TestFriendActionMischiefSkipsWhenProtocolSummaryHasNoTargets(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": true, "workLandIds": map[string]any{}},
	}}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "mischief", Target: "10001"})

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("result = %#v", result)
	}
	if gotMethods := calledMethods(caller.calls); !reflect.DeepEqual(gotMethods, []string{"gameCtl.inspectFriendFarmByProtocol"}) {
		t.Fatalf("methods = %#v", gotMethods)
	}
}

func TestFriendActionMischiefSurfacesRuntimeErrorDetail(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"bug": []any{float64(9)}},
		},
		"gameCtl.friendMischiefLandsBatch": map[string]any{"ok": false, "reason": "dispatch_failed", "results": []any{map[string]any{"error": "今日捣乱次数已达上限"}}},
	}}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "mischief", Target: "10001"})

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("runtime failure should fail action, got %#v", result)
	}
	if !strings.Contains(result.Message, "今日捣乱次数已达上限") {
		t.Fatalf("message should include runtime error detail, got %q", result.Message)
	}
}

func calledMethods(calls []runtimeCall) []string {
	methods := make([]string, 0, len(calls))
	for _, call := range calls {
		methods = append(methods, call.method)
	}
	return methods
}
