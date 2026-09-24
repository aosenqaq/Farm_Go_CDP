package automation

import (
	"Farm_Go/internal/farm/social"
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordingStealWriter struct {
	accountKey string
	records    []social.StealRecord
}

func (w *recordingStealWriter) SaveStealRecords(_ context.Context, accountKey string, records []social.StealRecord) error {
	w.accountKey = accountKey
	w.records = append(w.records, records...)
	return nil
}

func (w *recordingStealWriter) ListStealRecords(_ context.Context, _ string, _ []string) ([]social.StealRecord, error) {
	return w.records, nil
}

type recordingDogGuardReader struct {
	accountKey string
	state      social.DogGuardState
	err        error
}

func (r *recordingDogGuardReader) LoadDogGuardState(_ context.Context, accountKey string) (social.DogGuardState, error) {
	r.accountKey = accountKey
	return r.state, r.err
}

type recordingFriendRuleReader struct {
	accountKey string
	rules      social.FriendRules
	err        error
}

func (r *recordingFriendRuleReader) LoadFriendRules(_ context.Context, accountKey string) (social.FriendRules, error) {
	r.accountKey = accountKey
	return r.rules, r.err
}

type runtimeSocialStoreStub struct {
	rules       social.FriendRules
	err         error
	state       social.DogGuardState
	accountKeys []string
}

var _ RuntimeSocialStore = (*runtimeSocialStoreStub)(nil)

func (s *runtimeSocialStoreStub) SaveStealRecords(_ context.Context, _ string, _ []social.StealRecord) error {
	return nil
}

func (s *runtimeSocialStoreStub) LoadDogGuardState(_ context.Context, _ string) (social.DogGuardState, error) {
	return s.state, nil
}

func (s *runtimeSocialStoreStub) LoadFriendRules(_ context.Context, accountKey string) (social.FriendRules, error) {
	s.accountKeys = append(s.accountKeys, accountKey)
	return s.rules, s.err
}

type blockingFriendHelpCaller struct {
	mu          sync.Mutex
	friends     []any
	release     chan struct{}
	started     chan int
	active      int
	maxActive   int
	failGID     int
	emptyGID    int
	helpGIDs    []int
	inspectGIDs []int
}

type blockingFriendStealCaller struct {
	friends []any
	started chan int
	release chan struct{}
}

type friendOutcomeCaller struct {
	friends          []any
	inspectFailures  map[int]string
	harvestFailures  map[int]string
	helpFailures     map[int]string
	mischiefFailures map[int]string
}

func (c *friendOutcomeCaller) Call(_ context.Context, method string, args []any, _ time.Duration) (any, error) {
	if method == "gameCtl.getFriendList" {
		return c.friends, nil
	}
	gid := intFromAny(mapFromAny(args[0])["hostGid"])
	if method == "gameCtl.inspectFriendFarmByProtocol" {
		if reason := c.inspectFailures[gid]; reason != "" {
			return map[string]any{"ok": false, "reason": reason}, nil
		}
		return map[string]any{"ok": true, "workLandIds": map[string]any{
			"collect": []any{gid},
			"water":   []any{gid},
			"bug":     []any{gid},
		}}, nil
	}
	if method == "gameCtl.friendHarvestLandsByProtocol" {
		if reason := c.harvestFailures[gid]; reason != "" {
			return map[string]any{"ok": false, "reason": reason}, nil
		}
	}
	if method == "gameCtl.friendFarmingByProtocol" {
		if reason := c.helpFailures[gid]; reason != "" {
			return map[string]any{"ok": false, "reason": reason}, nil
		}
	}
	if method == "gameCtl.friendMischiefLandsBatch" {
		if reason := c.mischiefFailures[gid]; reason != "" {
			return map[string]any{"ok": false, "reason": reason}, nil
		}
	}
	return map[string]any{"ok": true}, nil
}

func (c *blockingFriendStealCaller) Call(ctx context.Context, method string, args []any, _ time.Duration) (any, error) {
	switch method {
	case "gameCtl.getFriendList":
		return c.friends, nil
	case "gameCtl.inspectFriendFarmByProtocol":
		gid := intFromAny(mapFromAny(args[0])["hostGid"])
		c.started <- gid
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.release:
		}
		return map[string]any{"ok": true, "workLandIds": map[string]any{"collect": []any{gid}}}, nil
	case "gameCtl.friendHarvestLandsByProtocol":
		return map[string]any{"ok": true}, nil
	default:
		return nil, errors.New("unexpected method: " + method)
	}
}

func friendStealTestFriends(gids ...int) []any {
	friends := make([]any, 0, len(gids))
	for _, gid := range gids {
		friends = append(friends, map[string]any{"gid": gid, "workCounts": map[string]any{"collect": 1}})
	}
	return friends
}

func (c *blockingFriendHelpCaller) Call(_ context.Context, method string, args []any, _ time.Duration) (any, error) {
	switch method {
	case "gameCtl.getFriendList":
		return c.friends, nil
	case "gameCtl.inspectFriendFarmByProtocol":
		gid := intFromAny(mapFromAny(args[0])["hostGid"])
		c.mu.Lock()
		c.active++
		if c.active > c.maxActive {
			c.maxActive = c.active
		}
		c.inspectGIDs = append(c.inspectGIDs, gid)
		c.mu.Unlock()
		if c.started != nil {
			c.started <- gid
		}
		if c.release != nil {
			<-c.release
		}
		c.mu.Lock()
		c.active--
		c.mu.Unlock()
		if gid == c.failGID {
			return map[string]any{"ok": false, "reason": "inspect_failed"}, nil
		}
		if gid == c.emptyGID {
			return map[string]any{"ok": true, "workLandIds": map[string]any{"water": []any{}}}, nil
		}
		return map[string]any{"ok": true, "workLandIds": map[string]any{"water": []any{gid}}}, nil
	case "gameCtl.friendFarmingByProtocol":
		gid := intFromAny(mapFromAny(args[0])["hostGid"])
		c.mu.Lock()
		c.helpGIDs = append(c.helpGIDs, gid)
		c.mu.Unlock()
		return map[string]any{"ok": true}, nil
	default:
		return nil, errors.New("unexpected method: " + method)
	}
}

func friendHelpTestFriends(gids ...int) []any {
	friends := make([]any, 0, len(gids))
	for _, gid := range gids {
		friends = append(friends, map[string]any{"gid": gid, "workCounts": map[string]any{"water": 1}})
	}
	return friends
}

func friendRuleTestFriends(taskID string, firstLevel, secondLevel int) []any {
	workKey := map[string]string{
		"friend_steal":    "collect",
		"friend_help":     "water",
		"friend_mischief": "mischief",
	}[taskID]
	return []any{
		map[string]any{"gid": float64(10001), "level": float64(firstLevel), "workCounts": map[string]any{workKey: float64(1)}},
		map[string]any{"gid": float64(10002), "level": float64(secondLevel), "workCounts": map[string]any{workKey: float64(1)}},
	}
}

func friendRuleTestResponses(friends []any) map[string]any {
	return map[string]any{
		"gameCtl.getFriendList":                friends,
		"gameCtl.inspectFriendFarmByProtocol":  map[string]any{"ok": true, "workLandIds": map[string]any{"collect": []any{float64(7)}, "water": []any{float64(7)}, "bug": []any{float64(7)}}},
		"gameCtl.friendHarvestLandsByProtocol": map[string]any{"ok": true},
		"gameCtl.friendFarmingByProtocol":      map[string]any{"ok": true},
		"gameCtl.friendMischiefLandsBatch":     map[string]any{"ok": true},
	}
}

func TestRuntimeFacadeFriendStealWaitsOnceBeforeStartingSharedBatch(t *testing.T) {
	const expectedDelay = 120 * time.Millisecond
	caller := &blockingFriendStealCaller{
		friends: friendStealTestFriends(10001, 10002, 10003),
		started: make(chan int, 3),
		release: make(chan struct{}),
	}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFriendStealConcurrency":        3,
		"autoFarmFriendStealRandomDelayEnabled": true,
		"autoFarmFriendStealRandomDelayMinMs":   int(expectedDelay / time.Millisecond),
		"autoFarmFriendStealRandomDelayMaxMs":   int(expectedDelay / time.Millisecond),
	})
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() { close(caller.release) })
	}
	defer release()

	startedAt := time.Now()
	resultChannel := make(chan ActionResult, 1)
	go func() {
		resultChannel <- facade.RunTask(context.Background(), "friend_steal")
	}()

	select {
	case gid := <-caller.started:
		t.Fatalf("friend %d inspection started before the shared random delay", gid)
	case <-time.After(expectedDelay / 2):
	}

	startedGIDs := make(map[int]bool)
	for len(startedGIDs) < 3 {
		select {
		case gid := <-caller.started:
			startedGIDs[gid] = true
		case <-time.After(time.Second):
			t.Fatalf("expected all friend inspections after the shared delay, got %#v", startedGIDs)
		}
	}
	if elapsed := time.Since(startedAt); elapsed < expectedDelay-20*time.Millisecond {
		t.Fatalf("first friend inspection started after %s, want at least %s", elapsed, expectedDelay)
	}

	release()
	select {
	case result := <-resultChannel:
		if !result.OK {
			t.Fatalf("friend_steal result = %#v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("friend_steal did not finish after releasing all inspections")
	}
}

func TestRuntimeFacadeFriendStealDoesNotStartProtocolsWhenDelayIsCanceled(t *testing.T) {
	caller := &blockingFriendStealCaller{
		friends: friendStealTestFriends(10001),
		started: make(chan int, 1),
		release: make(chan struct{}),
	}
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() { close(caller.release) })
	}
	defer release()
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFriendStealRandomDelayEnabled": true,
		"autoFarmFriendStealRandomDelayMinMs":   1000,
		"autoFarmFriendStealRandomDelayMaxMs":   1000,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultChannel := make(chan ActionResult, 1)
	go func() {
		resultChannel <- facade.RunTask(ctx, "friend_steal")
	}()

	select {
	case gid := <-caller.started:
		t.Fatalf("friend %d inspection started before the random delay could be canceled", gid)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()

	select {
	case result := <-resultChannel:
		if result.OK || result.Status != StatusFailed || !strings.Contains(result.Message, "随机偷菜延迟已取消") {
			t.Fatalf("canceled random delay result = %#v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("friend_steal did not stop after canceling the random delay")
	}
	select {
	case gid := <-caller.started:
		t.Fatalf("friend %d inspection started after delay cancellation", gid)
	default:
	}
}

func TestRuntimeFacadeFriendStealSkipsRandomDelayWhenDisabled(t *testing.T) {
	caller := &blockingFriendStealCaller{
		friends: friendStealTestFriends(10001),
		started: make(chan int, 1),
		release: make(chan struct{}),
	}
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() { close(caller.release) })
	}
	defer release()
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFriendStealRandomDelayEnabled": false,
		"autoFarmFriendStealRandomDelayMinMs":   1000,
		"autoFarmFriendStealRandomDelayMaxMs":   1000,
	})
	resultChannel := make(chan ActionResult, 1)
	go func() {
		resultChannel <- facade.RunTask(context.Background(), "friend_steal")
	}()

	select {
	case gid := <-caller.started:
		if gid != 10001 {
			t.Fatalf("started gid = %d, want 10001", gid)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("disabled random delay did not start the friend inspection promptly")
	}
	release()
	select {
	case result := <-resultChannel:
		if !result.OK {
			t.Fatalf("friend_steal result = %#v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("friend_steal did not finish with random delay disabled")
	}
}

func TestRuntimeFacadeSafeModeStealSerializesFriendsAndUsesFullFarmHarvestRequests(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": friendStealTestFriends(10001, 10002),
		"gameCtl.inspectFriendFarmByProtocol": fakeRuntimeResponseSequence{
			map[string]any{"ok": true, "workLandIds": map[string]any{"collect": []any{float64(1), float64(2)}}},
			map[string]any{"ok": true, "workLandIds": map[string]any{"collect": []any{float64(3)}}},
		},
		"gameCtl.friendHarvestLandsByProtocol": fakeRuntimeResponseSequence{
			map[string]any{"ok": false, "reason": "ssapi.go:323 failed"},
			map[string]any{"ok": true},
		},
	}}
	var waits []time.Duration
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFriendStealConcurrency":        3,
		"autoFarmFriendStealRandomDelayEnabled": true,
		"autoFarmFriendStealRandomDelayMinMs":   5000,
		"autoFarmFriendStealRandomDelayMaxMs":   5000,
	}).WithRunMode(RunModeSafe).WithExecutionTiming(
		func(_ context.Context, duration time.Duration) error {
			waits = append(waits, duration)
			return nil
		},
		func(n int) int { return n - 1 },
	)

	result := facade.RunTask(context.Background(), "friend_steal")

	if !result.OK || result.AttemptedFriends != 2 || result.SuccessfulFriends != 1 || result.FailedFriends != 1 || !strings.Contains(result.Message, "偷菜地块 1") {
		t.Fatalf("safe steal result = %#v", result)
	}
	if got := inspectHostGIDs(caller.calls); !reflect.DeepEqual(got, []int{10001, 10002}) {
		t.Fatalf("safe steal inspect order = %#v, want 10001 then 10002", got)
	}
	if got := friendLandCallIDs(caller.calls, "gameCtl.friendHarvestLandsByProtocol", "landIds"); !reflect.DeepEqual(got, [][]int{{1, 2}, {3}}) {
		t.Fatalf("safe steal harvest requests = %#v, want one full-farm request per friend", got)
	}
	if !reflect.DeepEqual(waits, []time.Duration{time.Second}) {
		t.Fatalf("safe steal waits = %#v, want one friend wait", waits)
	}
}

func TestFriendStealRandomDelaySettingsNormalizesMissingAndInvalidRanges(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		want   friendStealRandomDelaySettings
	}{
		{
			name: "missing config uses disabled defaults",
			want: friendStealRandomDelaySettings{minMs: 1000, maxMs: 5000},
		},
		{
			name: "negative values clamp to zero",
			config: map[string]any{
				"autoFarmFriendStealRandomDelayEnabled": true,
				"autoFarmFriendStealRandomDelayMinMs":   -50,
				"autoFarmFriendStealRandomDelayMaxMs":   -1,
			},
			want: friendStealRandomDelaySettings{enabled: true},
		},
		{
			name: "maximum below minimum clamps to minimum",
			config: map[string]any{
				"autoFarmFriendStealRandomDelayEnabled": true,
				"autoFarmFriendStealRandomDelayMinMs":   1200,
				"autoFarmFriendStealRandomDelayMaxMs":   200,
			},
			want: friendStealRandomDelaySettings{enabled: true, minMs: 1200, maxMs: 1200},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := friendStealRandomDelaySettingsFromConfig(test.config); got != test.want {
				t.Fatalf("settings = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestRuntimeFacadeFriendStealFiltersBlacklistedFriendsBeforeLimit(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "workCounts": map[string]any{"collect": float64(1)}},
			map[string]any{"gid": float64(10002), "workCounts": map[string]any{"collect": float64(1)}},
		},
		"gameCtl.inspectFriendFarmByProtocol":  map[string]any{"ok": true, "workLandIds": map[string]any{"collect": []any{float64(7)}}},
		"gameCtl.friendHarvestLandsByProtocol": map[string]any{"ok": true},
	}}
	store := &runtimeSocialStoreStub{rules: social.FriendRules{
		BlacklistEnabled: true,
		BlacklistScopes:  []string{"steal"},
		Blacklist:        []string{"10001"},
	}}
	facade := NewRuntimeFacadeWithConfigAndSocialStore(caller, map[string]any{"autoFarmFriendStealMaxFriends": 1}, store, "account-a")

	result := facade.RunTask(context.Background(), "friend_steal")

	if !result.OK {
		t.Fatalf("friend_steal should inspect the allowed candidate, got %#v", result)
	}
	if got := inspectHostGIDs(caller.calls); !reflect.DeepEqual(got, []int{10002}) {
		t.Fatalf("inspect host gids = %#v, want only non-blacklisted 10002", got)
	}
}

func TestRuntimeFacadeFriendHelpFiltersBlacklistedGuardDogBeforeLimit(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList":               friendHelpTestFriends(10001, 10002),
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": true, "workLandIds": map[string]any{"water": []any{float64(7)}}},
		"gameCtl.friendFarmingByProtocol":     map[string]any{"ok": true},
	}}
	store := &runtimeSocialStoreStub{
		rules: social.FriendRules{
			BlacklistEnabled: true,
			BlacklistScopes:  []string{"help"},
			Blacklist:        []string{"10001"},
		},
		state: social.DogGuardState{Results: []social.DogGuardRow{
			{GID: 10001, HasGuardDog: true},
			{GID: 10002, HasGuardDog: true},
		}},
	}
	facade := NewRuntimeFacadeWithConfigAndSocialStore(caller, map[string]any{
		"autoFarmFriendHelpGuardDogOnly": true,
		"autoFarmFriendHelpMaxFriends":   1,
	}, store, "account-a")

	result := facade.RunTask(context.Background(), "friend_help")

	if !result.OK {
		t.Fatalf("friend_help should inspect the allowed guarded candidate, got %#v", result)
	}
	if got := inspectHostGIDs(caller.calls); !reflect.DeepEqual(got, []int{10002}) {
		t.Fatalf("inspect host gids = %#v, want only non-blacklisted guarded friend 10002", got)
	}
	if got := helpHostGIDs(caller.calls); !reflect.DeepEqual(got, []int{10002}) {
		t.Fatalf("help host gids = %#v, want only non-blacklisted guarded friend 10002", got)
	}
}

func TestRuntimeFacadeFriendMischiefFiltersBlacklistedFriends(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "workCounts": map[string]any{"mischief": float64(1)}},
			map[string]any{"gid": float64(10002), "workCounts": map[string]any{"mischief": float64(1)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": true, "workLandIds": map[string]any{"bug": []any{float64(7)}}},
		"gameCtl.friendMischiefLandsBatch":    map[string]any{"ok": true},
	}}
	store := &runtimeSocialStoreStub{rules: social.FriendRules{
		BlacklistEnabled: true,
		BlacklistScopes:  []string{"mischief"},
		Blacklist:        []string{"10001"},
	}}
	facade := NewRuntimeFacadeWithConfigAndSocialStore(caller, nil, store, "account-a")

	result := facade.RunTask(context.Background(), "friend_mischief")

	if !result.OK {
		t.Fatalf("friend_mischief should inspect the allowed candidate, got %#v", result)
	}
	if got := inspectHostGIDs(caller.calls); !reflect.DeepEqual(got, []int{10002}) {
		t.Fatalf("inspect host gids = %#v, want only non-blacklisted 10002", got)
	}
	if got := mischiefHostGIDs(caller.calls); !reflect.DeepEqual(got, []int{10002}) {
		t.Fatalf("mischief host gids = %#v, want only non-blacklisted 10002", got)
	}
}

func TestRuntimeFacadeFriendTasksRespectScopeSpecificWhitelist(t *testing.T) {
	for _, testCase := range []struct {
		taskID string
		scope  string
	}{
		{taskID: "friend_steal", scope: "steal"},
		{taskID: "friend_help", scope: "help"},
		{taskID: "friend_mischief", scope: "mischief"},
	} {
		t.Run(testCase.taskID, func(t *testing.T) {
			store := &runtimeSocialStoreStub{rules: social.FriendRules{
				WhitelistEnabled: true,
				WhitelistScopes:  []string{testCase.scope},
				Whitelist:        []string{"10002"},
			}}
			caller := &fakeRuntimeCaller{responses: friendRuleTestResponses(friendRuleTestFriends(testCase.taskID, 4, 6))}
			accountKey := "account-" + testCase.taskID
			facade := NewRuntimeFacadeWithConfigAndSocialStore(caller, nil, store, accountKey)

			result := facade.RunTask(context.Background(), testCase.taskID)

			if !result.OK {
				t.Fatalf("%s should run for the whitelisted candidate, got %#v", testCase.taskID, result)
			}
			if got := inspectHostGIDs(caller.calls); !reflect.DeepEqual(got, []int{10002}) {
				t.Fatalf("inspect host gids = %#v, want only whitelisted 10002", got)
			}
			if got := store.accountKeys; !reflect.DeepEqual(got, []string{accountKey}) {
				t.Fatalf("rule reader account keys = %#v, want %#v", got, []string{accountKey})
			}
		})
	}
}

func TestRuntimeFacadeFriendTasksRespectScopeSpecificMaskedBlacklist(t *testing.T) {
	for _, testCase := range []struct {
		taskID string
		scope  string
	}{
		{taskID: "friend_steal", scope: "steal"},
		{taskID: "friend_help", scope: "help"},
		{taskID: "friend_mischief", scope: "mischief"},
	} {
		t.Run(testCase.taskID, func(t *testing.T) {
			store := &runtimeSocialStoreStub{rules: social.FriendRules{
				BlacklistEnabled: true,
				BlacklistScopes:  []string{testCase.scope},
				MaskedBlacklist:  true,
				MaskedMaxLevel:   3,
			}}
			caller := &fakeRuntimeCaller{responses: friendRuleTestResponses(friendRuleTestFriends(testCase.taskID, 3, 6))}
			accountKey := "account-" + testCase.taskID
			facade := NewRuntimeFacadeWithConfigAndSocialStore(caller, nil, store, accountKey)

			result := facade.RunTask(context.Background(), testCase.taskID)

			if !result.OK {
				t.Fatalf("%s should run for the non-masked candidate, got %#v", testCase.taskID, result)
			}
			if got := inspectHostGIDs(caller.calls); !reflect.DeepEqual(got, []int{10002}) {
				t.Fatalf("inspect host gids = %#v, want only non-masked 10002", got)
			}
			if got := store.accountKeys; !reflect.DeepEqual(got, []string{accountKey}) {
				t.Fatalf("rule reader account keys = %#v, want %#v", got, []string{accountKey})
			}
		})
	}
}

func TestRuntimeFacadeFriendTasksFailBeforeProtocolWhenFriendRulesCannotLoad(t *testing.T) {
	for _, taskID := range []string{"friend_steal", "friend_help", "friend_mischief"} {
		t.Run(taskID, func(t *testing.T) {
			caller := &fakeRuntimeCaller{responses: map[string]any{
				"gameCtl.getFriendList": []any{map[string]any{
					"gid":        float64(10001),
					"workCounts": map[string]any{"collect": float64(1), "help": float64(1), "mischief": float64(1)},
				}},
			}}
			facade := NewRuntimeFacadeWithConfigAndSocialStore(caller, nil, &runtimeSocialStoreStub{err: errors.New("rules unavailable")}, "account-a")

			result := facade.RunTask(context.Background(), taskID)

			if result.OK || result.Status != StatusFailed || !strings.Contains(result.Message, "读取好友规则失败：rules unavailable") {
				t.Fatalf("%s should fail with friend-rule error, got %#v", taskID, result)
			}
			if got := inspectHostGIDs(caller.calls); len(got) != 0 {
				t.Fatalf("%s should not inspect friends after friend-rule error, got gids %#v", taskID, got)
			}
			if got := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(got, []string{"gameCtl.getFriendList"}) {
				t.Fatalf("%s should only read the friend list before friend-rule error, calls %#v", taskID, got)
			}
		})
	}
}

func TestRuntimeFacadeFriendTasksSkipBeforeRuleReadWithoutCandidates(t *testing.T) {
	tests := []struct {
		name     string
		taskID   string
		config   map[string]any
		friends  []any
		state    social.DogGuardState
		contains string
	}{
		{
			name:     "steal has no candidates",
			taskID:   "friend_steal",
			friends:  []any{},
			contains: "没有检测到可偷菜好友",
		},
		{
			name:    "help guard filter removes all candidates",
			taskID:  "friend_help",
			config:  map[string]any{"autoFarmFriendHelpGuardDogOnly": true},
			friends: friendHelpTestFriends(10001),
			state: social.DogGuardState{Results: []social.DogGuardRow{
				{GID: 10002, HasGuardDog: true},
			}},
			contains: "护主犬好友中没有检测到可帮忙对象",
		},
		{
			name:     "mischief has no candidates",
			taskID:   "friend_mischief",
			friends:  []any{},
			contains: "没有检测到可捣乱好友",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := &fakeRuntimeCaller{responses: map[string]any{"gameCtl.getFriendList": tt.friends}}
			store := &runtimeSocialStoreStub{err: errors.New("rules unavailable"), state: tt.state}
			facade := NewRuntimeFacadeWithConfigAndSocialStore(caller, tt.config, store, "account-a")

			result := facade.RunTask(context.Background(), tt.taskID)

			if !result.OK || result.Status != StatusOK || !strings.Contains(result.Message, tt.contains) {
				t.Fatalf("%s should keep its no-candidate skip, got %#v", tt.taskID, result)
			}
			if len(store.accountKeys) != 0 {
				t.Fatalf("%s should not read friend rules without candidates, account keys %#v", tt.taskID, store.accountKeys)
			}
		})
	}
}

func TestRuntimeFacadeFriendTasksSkipWithoutProtocolWhenRulesFilterAllCandidates(t *testing.T) {
	for _, testCase := range []struct {
		taskID string
		scope  string
	}{
		{taskID: "friend_steal", scope: "steal"},
		{taskID: "friend_help", scope: "help"},
		{taskID: "friend_mischief", scope: "mischief"},
	} {
		t.Run(testCase.taskID, func(t *testing.T) {
			caller := &fakeRuntimeCaller{responses: map[string]any{
				"gameCtl.getFriendList": []any{map[string]any{
					"gid":        float64(10001),
					"workCounts": map[string]any{"collect": float64(1), "help": float64(1), "mischief": float64(1)},
				}},
			}}
			store := &runtimeSocialStoreStub{rules: social.FriendRules{
				BlacklistEnabled: true,
				BlacklistScopes:  []string{testCase.scope},
				Blacklist:        []string{"10001"},
			}}
			facade := NewRuntimeFacadeWithConfigAndSocialStore(caller, nil, store, "account-a")

			result := facade.RunTask(context.Background(), testCase.taskID)

			if !result.OK || result.Status != StatusOK {
				t.Fatalf("%s should skip after all candidates are filtered, got %#v", testCase.taskID, result)
			}
			if got := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(got, []string{"gameCtl.getFriendList"}) {
				t.Fatalf("%s should not call a protocol after all candidates are filtered, calls %#v", testCase.taskID, got)
			}
		})
	}
}

func TestRuntimeFacadeFriendStealHarvestsFirstCollectableFriend(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "name": "A", "workCounts": map[string]any{"collect": float64(2)}},
			map[string]any{"gid": float64(10002), "name": "B", "workCounts": map[string]any{"collect": float64(0)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"hostGid":     float64(10001),
			"workLandIds": map[string]any{"collect": []any{float64(9), "3"}},
		},
		"gameCtl.friendHarvestLandsByProtocol": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_steal")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("friend_steal should report OK, got %#v", result)
	}
	if len(caller.calls) != 3 {
		t.Fatalf("runtime call count = %d, want 3: %#v", len(caller.calls), caller.calls)
	}
	wantMethods := []string{
		"gameCtl.getFriendList",
		"gameCtl.inspectFriendFarmByProtocol",
		"gameCtl.friendHarvestLandsByProtocol",
	}
	if !reflect.DeepEqual(calledRuntimeMethods(caller.calls), wantMethods) {
		t.Fatalf("methods = %#v, want %#v", calledRuntimeMethods(caller.calls), wantMethods)
	}
	wantHarvestArgs := []any{map[string]any{
		"hostGid":     10001,
		"landIds":     []int{3, 9},
		"isAll":       false,
		"silent":      true,
		"waitReplyMs": 1200,
		"source":      "farm_go_auto_friend_steal",
	}}
	if !reflect.DeepEqual(caller.calls[2].args, wantHarvestArgs) {
		t.Fatalf("harvest args = %#v, want %#v", caller.calls[2].args, wantHarvestArgs)
	}
}

func TestRuntimeFacadeFriendTasksRefreshFriendWorkCountsBeforeSelectingCandidates(t *testing.T) {
	tests := []struct {
		taskID string
		source string
	}{
		{taskID: "friend_steal", source: "farm_go_auto_friend_steal"},
		{taskID: "friend_help", source: "farm_go_auto_friend_help"},
		{taskID: "friend_mischief", source: "farm_go_auto_friend_mischief"},
	}
	for _, tt := range tests {
		t.Run(tt.taskID, func(t *testing.T) {
			caller := &fakeRuntimeCaller{responses: map[string]any{
				"gameCtl.getFriendList": []any{},
			}}
			facade := NewRuntimeFacade(caller)

			result := facade.RunTask(context.Background(), tt.taskID)

			if !result.OK {
				t.Fatalf("%s should skip cleanly with empty friend list, got %#v", tt.taskID, result)
			}
			if len(caller.calls) != 1 || caller.calls[0].method != "gameCtl.getFriendList" {
				t.Fatalf("calls = %#v, want one getFriendList call", caller.calls)
			}
			options := mapFromAny(caller.calls[0].args[0])
			checks := map[string]any{
				"refresh":            true,
				"waitRefresh":        true,
				"refreshPlantStatus": true,
				"silent":             true,
				"source":             tt.source,
			}
			for key, want := range checks {
				if got := options[key]; got != want {
					t.Fatalf("%s getFriendList option %s = %#v, want %#v; options=%#v", tt.taskID, key, got, want, options)
				}
			}
		})
	}
}

func TestRuntimeFacadeFriendStealTriesNextCandidateWhenInspectHasNoCollectableLands(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "name": "A", "workCounts": map[string]any{"collect": float64(1)}},
			map[string]any{"gid": float64(10002), "name": "B", "workCounts": map[string]any{"collect": float64(1)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": fakeRuntimeResponseSequence{
			map[string]any{"ok": true, "hostGid": float64(10001), "workLandIds": map[string]any{"collect": []any{}}},
			map[string]any{"ok": true, "hostGid": float64(10002), "workLandIds": map[string]any{"collect": []any{float64(7)}}},
		},
		"gameCtl.friendHarvestLandsByProtocol": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_steal")

	if !result.OK {
		t.Fatalf("friend_steal should continue to next candidate: %#v", result)
	}
	if got := inspectHostGIDs(caller.calls); !reflect.DeepEqual(got, []int{10001, 10002}) {
		t.Fatalf("inspect host gids = %#v, want 10001 then 10002; calls=%#v", got, caller.calls)
	}
	harvest := caller.calls[len(caller.calls)-1]
	if harvest.method != "gameCtl.friendHarvestLandsByProtocol" {
		t.Fatalf("last method = %q, want harvest", harvest.method)
	}
	if got := harvest.args[0].(map[string]any)["hostGid"]; got != 10002 {
		t.Fatalf("harvest hostGid = %#v, want 10002", got)
	}
}

func TestRuntimeFacadeFriendStealRunsMultipleCollectableFriendsPerRound(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "name": "A", "workCounts": map[string]any{"collect": float64(1)}},
			map[string]any{"gid": float64(10002), "name": "B", "workCounts": map[string]any{"collect": float64(1)}},
			map[string]any{"gid": float64(10003), "name": "C", "workCounts": map[string]any{"collect": float64(1)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"collect": []any{float64(7)}},
		},
		"gameCtl.friendHarvestLandsByProtocol": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFriendStealConcurrency": 3,
		"autoFarmFriendStealMaxFriends":  3,
	})

	result := facade.RunTask(context.Background(), "friend_steal")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("friend_steal should report OK, got %#v", result)
	}
	got := harvestHostGIDs(caller.calls)
	sort.Ints(got)
	if !reflect.DeepEqual(got, []int{10001, 10002, 10003}) {
		t.Fatalf("harvest host gids = %#v, want all collectable friends; calls=%#v", got, caller.calls)
	}
	if result.ActionCount != 3 {
		t.Fatalf("action count = %d, want 3 successful friends", result.ActionCount)
	}
}

func TestRuntimeFacadeFriendStealDoesNotSkipRecentlyStolenFriendOnRepeatedManualRuns(t *testing.T) {
	writer := &recordingStealWriter{}
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "name": "A", "workCounts": map[string]any{"collect": float64(2)}},
			map[string]any{"gid": float64(10002), "name": "B", "workCounts": map[string]any{"collect": float64(2)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"collect": []any{float64(9)}},
			"lands": []any{
				map[string]any{"landId": float64(9), "plantInfo": map[string]any{"id": float64(2001), "name": "百香果"}},
			},
		},
		"gameCtl.friendHarvestLandsByProtocol": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfigAndStealRecorder(caller, map[string]any{
		"automationTrigger":                         "manual",
		"autoFarmFriendStealIntervalSec":            90,
		"autoFarmFriendBlacklistCooldownMin":        10,
		"autoFarmFriendStealPlantBlacklistEnabled":  true,
		"autoFarmFriendStealPlantListMode":          "blacklist",
		"autoFarmFriendStealPlantBlacklist":         []int{9999},
		"autoFarmFriendStealPlantBlacklistStrategy": 1,
	}, writer, "account-a")

	first := facade.RunTask(context.Background(), "friend_steal")
	second := facade.RunTask(context.Background(), "friend_steal")

	if !first.OK || !second.OK {
		t.Fatalf("repeated friend_steal should report OK, got first=%#v second=%#v", first, second)
	}
	if len(caller.calls) != 6 {
		t.Fatalf("runtime call count = %d, want 6: %#v", len(caller.calls), caller.calls)
	}
	secondInspectArgs := caller.calls[4].args
	wantSecondInspectArgs := []any{map[string]any{
		"hostGid":      10001,
		"silent":       true,
		"includeLands": true,
		"leaveAfter":   false,
		"source":       "farm_go_auto_friend_steal",
	}}
	if !reflect.DeepEqual(secondInspectArgs, wantSecondInspectArgs) {
		t.Fatalf("second inspect args = %#v, want %#v", secondInspectArgs, wantSecondInspectArgs)
	}
}

func TestRuntimeFacadeFriendStealSkipsRecentBlacklistOnlyFriendWithoutEnteringFarm(t *testing.T) {
	writer := &recordingStealWriter{}
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "name": "A", "workCounts": map[string]any{"collect": float64(2)}},
			map[string]any{"gid": float64(10002), "name": "B", "workCounts": map[string]any{"collect": float64(2)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"collect": []any{float64(9)}},
			"lands": []any{
				map[string]any{"landId": float64(9), "plantInfo": map[string]any{"id": float64(2001), "name": "百香果"}},
			},
		},
		"gameCtl.friendHarvestLandsByProtocol": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfigAndStealRecorder(caller, map[string]any{
		"autoFarmFriendBlacklistCooldownMin":        10,
		"autoFarmFriendStealPlantBlacklistEnabled":  true,
		"autoFarmFriendStealPlantListMode":          "blacklist",
		"autoFarmFriendStealPlantBlacklist":         []int{2001},
		"autoFarmFriendStealPlantBlacklistStrategy": 1,
	}, writer, "account-a")

	first := facade.RunTask(context.Background(), "friend_steal")
	second := facade.RunTask(context.Background(), "friend_steal")

	if !first.OK || !strings.Contains(first.Message, "黑名单") {
		t.Fatalf("blacklist-only friend should be skipped with blacklist message, got %#v", first)
	}
	if !second.OK {
		t.Fatalf("second friend_steal should report OK, got %#v", second)
	}
	if len(writer.records) != 2 || writer.records[0].Action != "steal_blacklist_skip" || writer.records[0].GID != 10001 {
		t.Fatalf("blacklist skip record not stored: %#v", writer.records)
	}
	if len(caller.calls) != 4 {
		t.Fatalf("runtime call count = %d, want 4: %#v", len(caller.calls), caller.calls)
	}
	secondInspectArgs := caller.calls[3].args
	wantSecondInspectArgs := []any{map[string]any{
		"hostGid":      10002,
		"silent":       true,
		"includeLands": true,
		"leaveAfter":   false,
		"source":       "farm_go_auto_friend_steal",
	}}
	if !reflect.DeepEqual(secondInspectArgs, wantSecondInspectArgs) {
		t.Fatalf("second inspect args = %#v, want %#v", secondInspectArgs, wantSecondInspectArgs)
	}
}

func TestRuntimeFacadeFriendStealFarmStrategySkipsWholeFarmWhenAnyCropBlacklisted(t *testing.T) {
	writer := &recordingStealWriter{}
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "name": "A", "workCounts": map[string]any{"collect": float64(2)}},
		},
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
	facade := NewRuntimeFacadeWithConfigAndStealRecorder(caller, map[string]any{
		"autoFarmFriendStealPlantBlacklistEnabled":  true,
		"autoFarmFriendStealPlantListMode":          "blacklist",
		"autoFarmFriendStealPlantBlacklist":         []int{2001},
		"autoFarmFriendStealPlantBlacklistStrategy": 1,
	}, writer, "account-a")

	result := facade.RunTask(context.Background(), "friend_steal")

	if !result.OK || !strings.Contains(result.Message, "黑名单") {
		t.Fatalf("blacklist farm strategy should skip the whole farm, got %#v", result)
	}
	if gotMethods := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(gotMethods, []string{
		"gameCtl.getFriendList",
		"gameCtl.inspectFriendFarmByProtocol",
	}) {
		t.Fatalf("methods = %#v", gotMethods)
	}
	if len(writer.records) != 1 || writer.records[0].Action != "steal_blacklist_skip" || !reflect.DeepEqual(writer.records[0].LandIDs, []int{3, 9}) {
		t.Fatalf("blacklist skip record = %#v", writer.records)
	}
}

func TestRuntimeFacadeFriendStealCropStrategyFiltersOnlyBlacklistedCrops(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "name": "A", "workCounts": map[string]any{"collect": float64(2)}},
		},
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
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFriendStealPlantBlacklistEnabled":  true,
		"autoFarmFriendStealPlantListMode":          "blacklist",
		"autoFarmFriendStealPlantBlacklist":         []int{2001},
		"autoFarmFriendStealPlantBlacklistStrategy": 2,
	})

	result := facade.RunTask(context.Background(), "friend_steal")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("friend_steal should report OK, got %#v", result)
	}
	wantHarvestArgs := []any{map[string]any{
		"hostGid":     10001,
		"landIds":     []int{9},
		"isAll":       false,
		"silent":      true,
		"waitReplyMs": 1200,
		"source":      "farm_go_auto_friend_steal",
	}}
	if !reflect.DeepEqual(caller.calls[2].args, wantHarvestArgs) {
		t.Fatalf("harvest args = %#v, want %#v", caller.calls[2].args, wantHarvestArgs)
	}
}

func TestRuntimeFacadeFriendStealWhitelistFarmStrategySkipsWholeFarmWhenAnyCropIsNotListed(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "name": "A", "workCounts": map[string]any{"collect": float64(2)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"collect": []any{float64(3), float64(9)}},
			"lands": []any{
				map[string]any{"landId": float64(3), "plantInfo": map[string]any{"id": float64(2001)}},
				map[string]any{"landId": float64(9), "plantInfo": map[string]any{"id": float64(3001)}},
			},
		},
		"gameCtl.friendHarvestLandsByProtocol": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFriendStealPlantBlacklistEnabled":  true,
		"autoFarmFriendStealPlantListMode":          "whitelist",
		"autoFarmFriendStealPlantWhitelist":         []int{3001},
		"autoFarmFriendStealPlantBlacklistStrategy": 1,
	})

	result := facade.RunTask(context.Background(), "friend_steal")

	if !result.OK || !strings.Contains(result.Message, "白名单") {
		t.Fatalf("whitelist farm strategy should skip the whole farm, got %#v", result)
	}
	if gotMethods := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(gotMethods, []string{
		"gameCtl.getFriendList",
		"gameCtl.inspectFriendFarmByProtocol",
	}) {
		t.Fatalf("methods = %#v", gotMethods)
	}
}

func TestRuntimeFacadeFriendStealWhitelistCropStrategyFiltersOnlyUnlistedCrops(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "name": "A", "workCounts": map[string]any{"collect": float64(2)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"collect": []any{float64(3), float64(9)}},
			"lands": []any{
				map[string]any{"landId": float64(3), "plantInfo": map[string]any{"id": float64(2001)}},
				map[string]any{"landId": float64(9), "plantInfo": map[string]any{"id": float64(3001)}},
			},
		},
		"gameCtl.friendHarvestLandsByProtocol": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFriendStealPlantBlacklistEnabled":  true,
		"autoFarmFriendStealPlantListMode":          "whitelist",
		"autoFarmFriendStealPlantWhitelist":         []int{3001},
		"autoFarmFriendStealPlantBlacklistStrategy": 2,
	})

	result := facade.RunTask(context.Background(), "friend_steal")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("friend_steal should report OK, got %#v", result)
	}
	wantHarvestArgs := []any{map[string]any{
		"hostGid":     10001,
		"landIds":     []int{9},
		"isAll":       false,
		"silent":      true,
		"waitReplyMs": 1200,
		"source":      "farm_go_auto_friend_steal",
	}}
	if !reflect.DeepEqual(caller.calls[2].args, wantHarvestArgs) {
		t.Fatalf("harvest args = %#v, want %#v", caller.calls[2].args, wantHarvestArgs)
	}
}

func TestRuntimeFacadeFriendStealWhitelistEmptyListSkipsFarm(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "name": "A", "workCounts": map[string]any{"collect": float64(1)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"collect": []any{float64(3)}},
			"lands":       []any{map[string]any{"landId": float64(3), "plantInfo": map[string]any{"id": float64(2001)}}},
		},
		"gameCtl.friendHarvestLandsByProtocol": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFriendStealPlantBlacklistEnabled": true,
		"autoFarmFriendStealPlantListMode":         "whitelist",
		"autoFarmFriendStealPlantWhitelist":        []int{},
	})

	result := facade.RunTask(context.Background(), "friend_steal")

	if !result.OK || !strings.Contains(result.Message, "白名单") {
		t.Fatalf("empty whitelist should skip the farm, got %#v", result)
	}
	if gotMethods := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(gotMethods, []string{
		"gameCtl.getFriendList",
		"gameCtl.inspectFriendFarmByProtocol",
	}) {
		t.Fatalf("methods = %#v", gotMethods)
	}
}

func TestRuntimeFacadeFriendStealIgnoresLegacyStrictModeWhenCropIDMissing(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "name": "A", "workCounts": map[string]any{"collect": float64(2)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"collect": []any{float64(9)}},
			"lands": []any{
				map[string]any{"landId": float64(9)},
			},
		},
		"gameCtl.friendHarvestLandsByProtocol": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFriendStealPlantBlacklistEnabled":           true,
		"autoFarmFriendStealPlantListMode":                   "blacklist",
		"autoFarmFriendStealPlantBlacklist":                  []int{2001},
		"autoFarmFriendStealPlantBlacklistStrictModeEnabled": true,
	})

	result := facade.RunTask(context.Background(), "friend_steal")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("legacy strict mode should not block protocol steal, got %#v", result)
	}
	if gotMethods := calledRuntimeMethods(caller.calls); !reflect.DeepEqual(gotMethods, []string{
		"gameCtl.getFriendList",
		"gameCtl.inspectFriendFarmByProtocol",
		"gameCtl.friendHarvestLandsByProtocol",
	}) {
		t.Fatalf("methods = %#v", gotMethods)
	}
}

func TestRuntimeFacadeFriendStealStoresStealRecordWithItems(t *testing.T) {
	writer := &recordingStealWriter{}
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "name": "A", "displayName": "A", "workCounts": map[string]any{"collect": float64(2)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":     true,
			"friend": map[string]any{"gid": float64(10001), "displayName": "A", "level": float64(12)},
			"workLandIds": map[string]any{
				"collect": []any{float64(3), float64(9)},
			},
			"lands": []any{
				map[string]any{"landId": float64(3), "plantInfo": map[string]any{"id": float64(2001), "name": "百香果"}},
				map[string]any{"landId": float64(9), "plantInfo": map[string]any{"id": float64(2001), "name": "百香果"}},
			},
		},
		"gameCtl.friendHarvestLandsByProtocol": map[string]any{
			"ok": true,
			"items": []any{
				map[string]any{"itemId": float64(2001), "name": "百香果", "count": float64(6), "landIds": []any{float64(3), float64(9)}},
			},
		},
	}}
	facade := NewRuntimeFacadeWithConfigAndStealRecorder(caller, nil, writer, "account-a")

	result := facade.RunTask(context.Background(), "friend_steal")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("friend_steal should report OK, got %#v", result)
	}
	if writer.accountKey != "account-a" || len(writer.records) != 1 {
		t.Fatalf("steal record was not stored: account=%q records=%#v", writer.accountKey, writer.records)
	}
	record := writer.records[0]
	if record.GID != 10001 || record.DisplayName != "A" || record.Action != "steal_auto" {
		t.Fatalf("stored record identity/action = %#v", record)
	}
	if !reflect.DeepEqual(record.LandIDs, []int{3, 9}) {
		t.Fatalf("stored land ids = %#v", record.LandIDs)
	}
	if len(record.Items) != 1 || record.Items[0].Name != "百香果" || record.Items[0].Count != 6 {
		t.Fatalf("stored items = %#v", record.Items)
	}
}

func TestRuntimeFacadeFriendStealHarvestsCollectableFriendFromPayloadList(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{
			"count": float64(1),
			"list": []any{
				map[string]any{"gid": float64(10001), "name": "A", "workCounts": map[string]any{"collect": float64(2)}},
			},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"collect": []any{float64(9)}},
		},
		"gameCtl.friendHarvestLandsByProtocol": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_steal")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("friend_steal should use payload.list friends, got %#v", result)
	}
	if len(caller.calls) != 3 {
		t.Fatalf("runtime call count = %d, want 3: %#v", len(caller.calls), caller.calls)
	}
	wantMethods := []string{
		"gameCtl.getFriendList",
		"gameCtl.inspectFriendFarmByProtocol",
		"gameCtl.friendHarvestLandsByProtocol",
	}
	if !reflect.DeepEqual(calledRuntimeMethods(caller.calls), wantMethods) {
		t.Fatalf("methods = %#v, want %#v", calledRuntimeMethods(caller.calls), wantMethods)
	}
}

func TestRuntimeFacadeFriendStealSkipsWhenNoCollectableFriends(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{map[string]any{"gid": float64(10002), "workCounts": map[string]any{"collect": float64(0)}}},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_steal")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("no collectable friends should be OK skip, got %#v", result)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("skip should only list friends, got %#v", caller.calls)
	}
}

func TestRuntimeFacadeFriendStealFailsWhenProtocolInspectNotOK(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList":               []any{map[string]any{"gid": float64(10001), "workCounts": map[string]any{"collect": float64(1)}}},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": false, "reason": "enter_failed"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_steal")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("protocol inspect ok=false should fail friend_steal, got %#v", result)
	}
	if !strings.Contains(result.Message, "协议进入好友农场") {
		t.Fatalf("message should mention protocol enter, got %q", result.Message)
	}
}

func TestRuntimeResultFailedUsesProtocolFailureText(t *testing.T) {
	failed, reason := runtimeResultFailed(map[string]any{
		"ok":          false,
		"failureText": "地块已被收取，无法操作",
	})

	if !failed || reason != "地块已被收取，无法操作" {
		t.Fatalf("runtime failure = (%v, %q), want protocol failure text", failed, reason)
	}
}

func TestRuntimeFacadeFriendStealSummarizesMixedBatchOutcome(t *testing.T) {
	caller := &friendOutcomeCaller{
		friends:         friendStealTestFriends(10001, 10002),
		harvestFailures: map[int]string{10001: "land_not_operable"},
	}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmFriendStealConcurrency": 2,
		"autoFarmFriendStealMaxFriends":  2,
	})

	result := facade.RunTask(context.Background(), "friend_steal")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("mixed friend steal should be successful overall, got %#v", result)
	}
	if result.AttemptedFriends != 2 || result.SuccessfulFriends != 1 || result.FailedFriends != 1 {
		t.Fatalf("friend steal counts = %#v, want attempted=2 success=1 failed=1", result)
	}
	for _, text := range []string{"已执行 2 个好友", "成功 1", "失败 1"} {
		if !strings.Contains(result.Message, text) {
			t.Fatalf("message %q should contain %q", result.Message, text)
		}
	}
}

func TestRuntimeFacadeFriendStealHandlesNilRuntimeCaller(t *testing.T) {
	facade := NewRuntimeFacade(nil)

	result := facade.RunTask(context.Background(), "friend_steal")

	if result.OK || result.Status != StatusRuntimeNotReady {
		t.Fatalf("nil runtime should be runtime_not_ready, got %#v", result)
	}
}

func TestRuntimeFacadeFriendStealReportsRuntimeError(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": errors.New("friend list unavailable"),
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_steal")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("runtime error should fail, got %#v", result)
	}
}

func TestRuntimeFacadeFriendStealReportsRuntimeNotOKResult(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList":               []any{map[string]any{"gid": float64(10001), "workCounts": map[string]any{"collect": float64(1)}}},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": false, "reason": "enter_failed"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_steal")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("runtime ok=false should fail, got %#v", result)
	}
}

func TestRuntimeFacadeFriendHelpUsesFarmingProtocol(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "workCounts": map[string]any{"help": float64(2)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"farming": []any{float64(1), float64(2)}},
		},
		"gameCtl.friendFarmingByProtocol": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_help")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("friend_help should report OK, got %#v", result)
	}
	wantMethods := []string{"gameCtl.getFriendList", "gameCtl.inspectFriendFarmByProtocol", "gameCtl.friendFarmingByProtocol"}
	if !reflect.DeepEqual(calledRuntimeMethods(caller.calls), wantMethods) {
		t.Fatalf("methods = %#v, want %#v", calledRuntimeMethods(caller.calls), wantMethods)
	}
	wantArgs := []any{map[string]any{
		"hostGid":     10001,
		"landIds":     []int{1, 2},
		"source":      0,
		"silent":      true,
		"waitReplyMs": 1200,
	}}
	if !reflect.DeepEqual(caller.calls[2].args, wantArgs) {
		t.Fatalf("help args = %#v, want %#v", caller.calls[2].args, wantArgs)
	}
}

func TestRuntimeFacadeFriendHelpGuardDogOnlyFiltersCurrentCandidatesAndUsesAccountKey(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": friendHelpTestFriends(10001, 10002),
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"water": []any{float64(4)}},
		},
		"gameCtl.friendFarmingByProtocol": map[string]any{"ok": true},
	}}
	reader := &recordingDogGuardReader{state: social.DogGuardState{Results: []social.DogGuardRow{
		{GID: 10001, HasGuardDog: false},
		{GID: 10002, HasGuardDog: true},
	}}}
	facade := NewRuntimeFacadeWithConfigAndDogGuardReader(caller, map[string]any{
		"autoFarmFriendHelpGuardDogOnly": true,
	}, reader, "account-b")

	result := facade.RunTask(context.Background(), "friend_help")

	if !result.OK {
		t.Fatalf("guard-only friend help should succeed, got %#v", result)
	}
	if reader.accountKey != "account-b" {
		t.Fatalf("dog guard reader account key = %q, want account-b", reader.accountKey)
	}
	if got := inspectHostGIDs(caller.calls); !reflect.DeepEqual(got, []int{10002}) {
		t.Fatalf("inspect host gids = %#v, want only guarded friend 10002", got)
	}
}

func TestRuntimeFacadeFriendHelpGuardDogOnlySkipsWhenCacheHasNoGuardedFriends(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": friendHelpTestFriends(10001, 10002),
	}}
	reader := &recordingDogGuardReader{state: social.DogGuardState{Results: []social.DogGuardRow{}}}
	facade := NewRuntimeFacadeWithConfigAndDogGuardReader(caller, map[string]any{
		"autoFarmFriendHelpGuardDogOnly": true,
	}, reader, "account-empty")

	result := facade.RunTask(context.Background(), "friend_help")

	if !result.OK || result.Status != StatusOK || !strings.Contains(result.Message, "请先读取护主犬好友") {
		t.Fatalf("empty guard cache should skip with guidance, got %#v", result)
	}
	if got := inspectHostGIDs(caller.calls); len(got) != 0 {
		t.Fatalf("empty guard cache must not inspect all friends, got gids %#v", got)
	}
}

func TestRuntimeFacadeFriendHelpGuardDogOnlyFailsWithoutReader(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": friendHelpTestFriends(10001),
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{"autoFarmFriendHelpGuardDogOnly": true})

	result := facade.RunTask(context.Background(), "friend_help")

	if result.OK || result.Status != StatusFailed || !strings.Contains(result.Message, "护主犬好友") {
		t.Fatalf("missing guard reader should fail clearly, got %#v", result)
	}
	if got := inspectHostGIDs(caller.calls); len(got) != 0 {
		t.Fatalf("missing guard reader must not inspect all friends, got gids %#v", got)
	}
}

func TestRuntimeFacadeFriendHelpGuardDogOnlyFailsOnCacheReadError(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": friendHelpTestFriends(10001),
	}}
	reader := &recordingDogGuardReader{err: errors.New("cache unavailable")}
	facade := NewRuntimeFacadeWithConfigAndDogGuardReader(caller, map[string]any{
		"autoFarmFriendHelpGuardDogOnly": true,
	}, reader, "account-error")

	result := facade.RunTask(context.Background(), "friend_help")

	if result.OK || result.Status != StatusFailed || !strings.Contains(result.Message, "cache unavailable") {
		t.Fatalf("guard cache read error should fail clearly, got %#v", result)
	}
	if got := inspectHostGIDs(caller.calls); len(got) != 0 {
		t.Fatalf("guard cache read error must not inspect all friends, got gids %#v", got)
	}
}

func TestRuntimeFacadeFriendHelpRespectsMaxFriends(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": friendHelpTestFriends(10001, 10002, 10003),
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"water": []any{float64(1)}},
		},
		"gameCtl.friendFarmingByProtocol": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"automationTrigger":               "auto",
		"autoFarmFriendHelpMaxFriends":    2,
		"autoFarmFriendHelpDailyLimit":    10,
		FriendHelpDailyCountDateConfigKey: time.Now().Format("2006-01-02"),
		FriendHelpDailyCountConfigKey:     1,
	})

	result := facade.RunTask(context.Background(), "friend_help")

	if !result.OK {
		t.Fatalf("limited friend help should succeed, got %#v", result)
	}
	gids := inspectHostGIDs(caller.calls)
	sort.Ints(gids)
	if !reflect.DeepEqual(gids, []int{10001, 10002}) {
		t.Fatalf("inspected gids = %#v, want first two candidates", gids)
	}
	if result.SuccessfulFriends != 2 {
		t.Fatalf("successful friends = %d, want 2", result.SuccessfulFriends)
	}
	if result.ActionCount != 2 {
		t.Fatalf("action count = %d, want 2 successful friends", result.ActionCount)
	}
}

func TestRuntimeFacadeFriendHelpLimitsAutomaticBatchToDailyRemaining(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": friendHelpTestFriends(10001, 10002, 10003),
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"water": []any{float64(1)}},
		},
		"gameCtl.friendFarmingByProtocol": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"automationTrigger":               "auto",
		"autoFarmFriendHelpMaxFriends":    5,
		"autoFarmFriendHelpDailyLimit":    3,
		FriendHelpDailyCountDateConfigKey: time.Now().Format("2006-01-02"),
		FriendHelpDailyCountConfigKey:     2,
	})

	result := facade.RunTask(context.Background(), "friend_help")

	if !result.OK || result.SuccessfulFriends != 1 {
		t.Fatalf("daily-limited result = %#v", result)
	}
	if got := inspectHostGIDs(caller.calls); !reflect.DeepEqual(got, []int{10001}) {
		t.Fatalf("inspected gids = %#v, want one remaining friend", got)
	}
}

func TestRuntimeFacadeFriendHelpSkipsAutomaticRunAtDailyLimit(t *testing.T) {
	facade := NewRuntimeFacadeWithConfig(nil, map[string]any{
		"automationTrigger":               "auto",
		"autoFarmFriendHelpDailyLimit":    2,
		FriendHelpDailyCountDateConfigKey: time.Now().Format("2006-01-02"),
		FriendHelpDailyCountConfigKey:     2,
	})

	result := facade.RunTask(context.Background(), "friend_help")

	if !result.OK || result.Status != StatusOK || !strings.Contains(result.Message, "每日上限") {
		t.Fatalf("exhausted daily limit should skip without runtime: %#v", result)
	}
}

func TestRuntimeFacadeFriendHelpManualRunIgnoresDailyLimit(t *testing.T) {
	caller := &blockingFriendHelpCaller{friends: friendHelpTestFriends(10001)}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"automationTrigger":               "manual",
		"autoFarmFriendHelpDailyLimit":    1,
		FriendHelpDailyCountDateConfigKey: time.Now().Format("2006-01-02"),
		FriendHelpDailyCountConfigKey:     1,
	})

	result := facade.RunTask(context.Background(), "friend_help")

	if !result.OK || result.SuccessfulFriends != 1 {
		t.Fatalf("manual run should ignore daily limit: %#v", result)
	}
}

func TestRuntimeFacadeFriendHelpZeroDailyLimitIsUnlimited(t *testing.T) {
	caller := &blockingFriendHelpCaller{friends: friendHelpTestFriends(10001, 10002, 10003)}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"automationTrigger":            "auto",
		"autoFarmFriendHelpDailyLimit": 0,
		"autoFarmFriendHelpMaxFriends": 0,
	})

	result := facade.RunTask(context.Background(), "friend_help")

	if !result.OK || result.SuccessfulFriends != 3 {
		t.Fatalf("zero daily limit should be unlimited: %#v", result)
	}
}

func TestRuntimeFacadeFriendHelpRunsThreeWorkersAndContinuesAfterFailure(t *testing.T) {
	caller := &blockingFriendHelpCaller{
		friends: friendHelpTestFriends(10001, 10002, 10003, 10004, 10005),
		release: make(chan struct{}),
		started: make(chan int, 5),
		failGID: 10002,
	}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{"autoFarmFriendHelpMaxFriends": 0})
	resultChannel := make(chan ActionResult, 1)
	go func() {
		resultChannel <- facade.RunTask(context.Background(), "friend_help")
	}()

	for index := 0; index < 3; index++ {
		select {
		case <-caller.started:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for three concurrent friend inspections")
		}
	}
	select {
	case gid := <-caller.started:
		t.Fatalf("fourth inspection %d started before a worker was released", gid)
	case <-time.After(50 * time.Millisecond):
	}
	caller.mu.Lock()
	maxActive := caller.maxActive
	caller.mu.Unlock()
	if maxActive != 3 {
		t.Fatalf("max active calls = %d, want exactly 3", maxActive)
	}
	close(caller.release)

	var result ActionResult
	select {
	case result = <-resultChannel:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for friend help batch")
	}
	if !result.OK || result.Status != StatusOK {
		t.Fatalf("partial-success friend help should be OK, got %#v", result)
	}
	if result.SuccessfulFriends != 4 {
		t.Fatalf("successful friends = %d, want 4", result.SuccessfulFriends)
	}
	for _, text := range []string{"成功 4", "帮忙项 4", "跳过 0", "失败 1"} {
		if !strings.Contains(result.Message, text) {
			t.Fatalf("message %q should contain %q", result.Message, text)
		}
	}
	caller.mu.Lock()
	helpGIDs := append([]int(nil), caller.helpGIDs...)
	inspectGIDs := append([]int(nil), caller.inspectGIDs...)
	maxActive = caller.maxActive
	caller.mu.Unlock()
	if maxActive > 3 {
		t.Fatalf("max active calls = %d, want <= 3", maxActive)
	}
	sort.Ints(helpGIDs)
	if !reflect.DeepEqual(helpGIDs, []int{10001, 10003, 10004, 10005}) {
		t.Fatalf("help gids = %#v, want all successful candidates including later friends; inspected=%#v", helpGIDs, inspectGIDs)
	}
}

func TestRuntimeFacadeSafeModeHelpSerializesFriendsButKeepsOneClickHelp(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": friendHelpTestFriends(10001, 10002),
		"gameCtl.inspectFriendFarmByProtocol": fakeRuntimeResponseSequence{
			map[string]any{"ok": true, "workLandIds": map[string]any{"water": []any{float64(1), float64(2)}}},
			map[string]any{"ok": true, "workLandIds": map[string]any{"water": []any{float64(3)}}},
		},
		"gameCtl.friendFarmingByProtocol": map[string]any{"ok": true},
	}}
	var waits []time.Duration
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{}).WithRunMode(RunModeSafe).WithExecutionTiming(
		func(_ context.Context, duration time.Duration) error {
			waits = append(waits, duration)
			return nil
		},
		func(n int) int { return n - 1 },
	)

	result := facade.RunTask(context.Background(), "friend_help")

	if !result.OK || result.SuccessfulFriends != 2 || result.ActionCount != 2 {
		t.Fatalf("safe help result = %#v", result)
	}
	if got := inspectHostGIDs(caller.calls); !reflect.DeepEqual(got, []int{10001, 10002}) {
		t.Fatalf("safe help inspect order = %#v, want 10001 then 10002", got)
	}
	if got := friendLandCallIDs(caller.calls, "gameCtl.friendFarmingByProtocol", "landIds"); !reflect.DeepEqual(got, [][]int{{1, 2}, {3}}) {
		t.Fatalf("safe help batches = %#v, want one full help call per friend", got)
	}
	if !reflect.DeepEqual(waits, []time.Duration{time.Second}) {
		t.Fatalf("safe help waits = %#v, want one friend wait", waits)
	}
}

func TestRuntimeFacadeFriendHelpReportsStructuredMixedBatchOutcome(t *testing.T) {
	caller := &friendOutcomeCaller{
		friends:      friendHelpTestFriends(10001, 10002),
		helpFailures: map[int]string{10001: "land_already_collected"},
	}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_help")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("mixed friend help should be successful overall, got %#v", result)
	}
	if result.AttemptedFriends != 2 || result.SuccessfulFriends != 1 || result.FailedFriends != 1 {
		t.Fatalf("friend help counts = %#v, want attempted=2 success=1 failed=1", result)
	}
	for _, text := range []string{"已执行 2 个好友", "成功 1", "失败 1"} {
		if !strings.Contains(result.Message, text) {
			t.Fatalf("message %q should contain %q", result.Message, text)
		}
	}
}

func TestRuntimeFacadeFriendMischiefContinuesAfterFailureAndSummarizesBatch(t *testing.T) {
	caller := &friendOutcomeCaller{
		friends:          friendRuleTestFriends("friend_mischief", 1, 1),
		inspectFailures:  map[int]string{10001: "land_not_operable"},
		mischiefFailures: map[int]string{},
	}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_mischief")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("mixed friend mischief should be successful overall, got %#v", result)
	}
	if result.AttemptedFriends != 2 || result.SuccessfulFriends != 1 || result.FailedFriends != 1 {
		t.Fatalf("friend mischief counts = %#v, want attempted=2 success=1 failed=1", result)
	}
	for _, text := range []string{"已执行 2 个好友", "成功 1", "失败 1"} {
		if !strings.Contains(result.Message, text) {
			t.Fatalf("message %q should contain %q", result.Message, text)
		}
	}
}

func TestRuntimeFacadeFriendHelpCancellationStopsQueuedCandidatesAndHelpMutations(t *testing.T) {
	caller := &blockingFriendHelpCaller{
		friends: friendHelpTestFriends(10001, 10002, 10003, 10004, 10005, 10006),
		release: make(chan struct{}),
		started: make(chan int, 6),
	}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{"autoFarmFriendHelpMaxFriends": 0})
	ctx, cancel := context.WithCancel(context.Background())
	resultChannel := make(chan ActionResult, 1)
	go func() {
		resultChannel <- facade.RunTask(ctx, "friend_help")
	}()

	for index := 0; index < friendHelpConcurrency; index++ {
		select {
		case <-caller.started:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for initial friend inspections")
		}
	}
	cancel()
	close(caller.release)

	select {
	case <-resultChannel:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for canceled friend help batch")
	}
	caller.mu.Lock()
	inspectGIDs := append([]int(nil), caller.inspectGIDs...)
	helpGIDs := append([]int(nil), caller.helpGIDs...)
	caller.mu.Unlock()
	sort.Ints(inspectGIDs)
	if !reflect.DeepEqual(inspectGIDs, []int{10001, 10002, 10003}) {
		t.Fatalf("inspect gids after cancellation = %#v, want only initial three candidates", inspectGIDs)
	}
	if len(helpGIDs) != 0 {
		t.Fatalf("help mutations after cancellation = %#v, want none", helpGIDs)
	}
}

func TestRuntimeFacadeFriendHelpTriesNextCandidateWhenInspectHasNoHelpLands(t *testing.T) {
	caller := &blockingFriendHelpCaller{
		friends:  friendHelpTestFriends(10001, 10002),
		emptyGID: 10001,
	}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_help")

	if !result.OK {
		t.Fatalf("friend_help should continue to next candidate: %#v", result)
	}
	caller.mu.Lock()
	inspected := append([]int(nil), caller.inspectGIDs...)
	helpGIDs := append([]int(nil), caller.helpGIDs...)
	caller.mu.Unlock()
	sort.Ints(inspected)
	if !reflect.DeepEqual(inspected, []int{10001, 10002}) {
		t.Fatalf("inspect host gids = %#v, want both candidates", inspected)
	}
	if !reflect.DeepEqual(helpGIDs, []int{10002}) {
		t.Fatalf("help host gids = %#v, want only 10002", helpGIDs)
	}
}

func TestRuntimeFacadeFriendHelpTreatsCareWorkCountsAsHelpable(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "workCounts": map[string]any{"water": float64(1), "eraseGrass": float64(1), "killBug": float64(0)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"water": []any{float64(9)}, "eraseGrass": []any{float64(5)}},
		},
		"gameCtl.friendFarmingByProtocol": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_help")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("friend_help should run for water/grass/bug work counts, got %#v", result)
	}
	gotMethods := calledRuntimeMethods(caller.calls)
	wantMethods := []string{"gameCtl.getFriendList", "gameCtl.inspectFriendFarmByProtocol", "gameCtl.friendFarmingByProtocol"}
	if !reflect.DeepEqual(gotMethods, wantMethods) {
		t.Fatalf("methods = %#v, want %#v", gotMethods, wantMethods)
	}
}

func TestRuntimeFacadeFriendHelpSkipsWhenNoHelpableFriends(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{map[string]any{"gid": float64(10002), "workCounts": map[string]any{"help": float64(0)}}},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_help")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("no helpable friends should be OK skip, got %#v", result)
	}
}

func TestRuntimeFacadeLoadFriendRulesReturnsReaderError(t *testing.T) {
	reader := &recordingFriendRuleReader{err: errors.New("rules unavailable")}
	facade := RuntimeFacade{friendRuleReader: reader}

	_, err := facade.loadFriendRules(context.Background())

	if err == nil || !strings.Contains(err.Error(), "rules unavailable") {
		t.Fatalf("loadFriendRules error = %v, want rules unavailable", err)
	}
	if reader.accountKey != "default" {
		t.Fatalf("reader account key = %q, want default", reader.accountKey)
	}
}

func TestRuntimeFacadeLoadFriendRulesUsesAccountKeyAndNormalizes(t *testing.T) {
	reader := &recordingFriendRuleReader{rules: social.FriendRules{
		BlacklistScopes: []string{"HELP", "help", "unknown"},
		Blacklist:       []string{" 10001 ", "10001", ""},
		MaskedMaxLevel:  0,
	}}
	facade := RuntimeFacade{accountKey: "account-42", friendRuleReader: reader}

	rules, err := facade.loadFriendRules(context.Background())

	if err != nil {
		t.Fatalf("loadFriendRules error = %v", err)
	}
	if reader.accountKey != "account-42" {
		t.Fatalf("reader account key = %q, want account-42", reader.accountKey)
	}
	if !reflect.DeepEqual(rules.BlacklistScopes, []string{"help"}) {
		t.Fatalf("blacklist scopes = %#v, want normalized help scope", rules.BlacklistScopes)
	}
	if !reflect.DeepEqual(rules.Blacklist, []string{"10001"}) {
		t.Fatalf("blacklist = %#v, want normalized item", rules.Blacklist)
	}
	if rules.MaskedMaxLevel != 1 {
		t.Fatalf("masked max level = %d, want 1", rules.MaskedMaxLevel)
	}
}

func TestRuntimeFacadeLoadFriendRulesUsesNormalizedEmptyRulesWithoutReader(t *testing.T) {
	facade := RuntimeFacade{}

	rules, err := facade.loadFriendRules(context.Background())

	if err != nil {
		t.Fatalf("loadFriendRules error = %v", err)
	}
	if !reflect.DeepEqual(rules, social.NormalizeFriendRules(social.FriendRules{})) {
		t.Fatalf("rules = %#v, want normalized empty rules", rules)
	}
}

func TestRuntimeFacadeFriendHelpFailsWhenProtocolInspectNotOK(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList":               []any{map[string]any{"gid": float64(10001), "workCounts": map[string]any{"help": float64(1)}}},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": false, "reason": "enter_failed"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_help")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("protocol inspect ok=false should fail friend_help, got %#v", result)
	}
}

func TestRuntimeFacadeFriendHelpHandlesNilRuntimeCaller(t *testing.T) {
	facade := NewRuntimeFacade(nil)

	result := facade.RunTask(context.Background(), "friend_help")

	if result.OK || result.Status != StatusRuntimeNotReady {
		t.Fatalf("nil runtime should be runtime_not_ready, got %#v", result)
	}
}

func TestRuntimeFacadeFriendHelpReportsRuntimeError(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": errors.New("friend list unavailable"),
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_help")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("runtime error should fail, got %#v", result)
	}
}

func TestRuntimeFacadeFriendHelpReportsRuntimeNotOKResult(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList":               []any{map[string]any{"gid": float64(10001), "workCounts": map[string]any{"help": float64(1)}}},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": true, "workLandIds": map[string]any{"water": []any{float64(1)}}},
		"gameCtl.friendFarmingByProtocol":     map[string]any{"ok": false, "reason": "farming_missing"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_help")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("runtime ok=false should fail, got %#v", result)
	}
}

func calledRuntimeMethods(calls []runtimeCall) []string {
	methods := make([]string, 0, len(calls))
	for _, call := range calls {
		methods = append(methods, call.method)
	}
	return methods
}

func inspectHostGIDs(calls []runtimeCall) []int {
	gids := []int{}
	for _, call := range calls {
		if call.method != "gameCtl.inspectFriendFarmByProtocol" || len(call.args) == 0 {
			continue
		}
		gids = append(gids, intFromAny(mapFromAny(call.args[0])["hostGid"]))
	}
	return gids
}

func helpHostGIDs(calls []runtimeCall) []int {
	return actionHostGIDs(calls, "gameCtl.friendFarmingByProtocol")
}

func mischiefHostGIDs(calls []runtimeCall) []int {
	return actionHostGIDs(calls, "gameCtl.friendMischiefLandsBatch")
}

func actionHostGIDs(calls []runtimeCall, method string) []int {
	gids := []int{}
	for _, call := range calls {
		if call.method != method || len(call.args) == 0 {
			continue
		}
		gids = append(gids, intFromAny(mapFromAny(call.args[0])["hostGid"]))
	}
	return gids
}

func harvestHostGIDs(calls []runtimeCall) []int {
	gids := []int{}
	for _, call := range calls {
		if call.method != "gameCtl.friendHarvestLandsByProtocol" || len(call.args) == 0 {
			continue
		}
		gids = append(gids, intFromAny(mapFromAny(call.args[0])["hostGid"]))
	}
	return gids
}

func friendLandCallIDs(calls []runtimeCall, method string, key string) [][]int {
	landIDs := make([][]int, 0)
	for _, call := range calls {
		if call.method != method || len(call.args) == 0 {
			continue
		}
		payload := mapFromAny(call.args[0])
		landIDs = append(landIDs, collectProtocolWorkLandIDs(map[string]any{"workLandIds": map[string]any{"lands": payload[key]}}, "lands"))
	}
	return landIDs
}

func friendMischiefPayloads(calls []runtimeCall) []map[string][]int {
	payloads := make([]map[string][]int, 0)
	for _, call := range calls {
		if call.method != "gameCtl.friendMischiefLandsBatch" || len(call.args) == 0 {
			continue
		}
		payload := mapFromAny(call.args[0])
		payloads = append(payloads, map[string][]int{
			"bug":   collectProtocolWorkLandIDs(map[string]any{"workLandIds": map[string]any{"lands": payload["bugLandIds"]}}, "lands"),
			"grass": collectProtocolWorkLandIDs(map[string]any{"workLandIds": map[string]any{"lands": payload["grassLandIds"]}}, "lands"),
		})
	}
	return payloads
}

func TestRuntimeFacadeFriendMischiefRunsBatch(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "workCounts": map[string]any{"mischief": float64(2)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok": true,
			"workLandIds": map[string]any{
				"bug":   []any{float64(9), "3"},
				"grass": []any{"5"},
			},
		},
		"gameCtl.friendMischiefLandsBatch": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_mischief")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("friend_mischief should report OK, got %#v", result)
	}
	if len(caller.calls) != 3 {
		t.Fatalf("runtime call count = %d, want 3: %#v", len(caller.calls), caller.calls)
	}
	if caller.calls[2].method != "gameCtl.friendMischiefLandsBatch" {
		t.Fatalf("third method = %q, want friendMischiefLandsBatch", caller.calls[2].method)
	}
	wantArgs := []any{map[string]any{
		"hostGid":      10001,
		"bugLandIds":   []int{3, 9},
		"grassLandIds": []int{5},
		"dryRun":       false,
		"silent":       true,
		"source":       "farm_go_auto_friend_mischief",
	}}
	if !reflect.DeepEqual(caller.calls[2].args, wantArgs) {
		t.Fatalf("mischief args = %#v, want %#v", caller.calls[2].args, wantArgs)
	}
	if result.ActionCount != 1 {
		t.Fatalf("action count = %d, want 1 successful friend", result.ActionCount)
	}
}

func TestRuntimeFacadeSafeModeMischiefSerializesLandsAndContinuesToNextFriend(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "workCounts": map[string]any{"mischief": float64(1)}},
			map[string]any{"gid": float64(10002), "workCounts": map[string]any{"mischief": float64(1)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": fakeRuntimeResponseSequence{
			map[string]any{"ok": true, "workLandIds": map[string]any{"bug": []any{float64(1), float64(2)}, "grass": []any{float64(3)}}},
			map[string]any{"ok": true, "workLandIds": map[string]any{"grass": []any{float64(4)}}},
		},
		"gameCtl.friendMischiefLandsBatch": fakeRuntimeResponseSequence{
			map[string]any{"ok": true},
			map[string]any{"ok": false, "reason": "land_not_ready"},
			map[string]any{"ok": true},
		},
	}}
	var waits []time.Duration
	facade := NewRuntimeFacade(caller).WithRunMode(RunModeSafe).WithExecutionTiming(
		func(_ context.Context, duration time.Duration) error {
			waits = append(waits, duration)
			return nil
		},
		func(n int) int { return n - 1 },
	)

	result := facade.RunTask(context.Background(), "friend_mischief")

	if !result.OK || result.AttemptedFriends != 2 || result.SuccessfulFriends != 1 || result.FailedFriends != 1 || !strings.Contains(result.Message, "捣乱地块 2") {
		t.Fatalf("safe mischief result = %#v", result)
	}
	if got := inspectHostGIDs(caller.calls); !reflect.DeepEqual(got, []int{10001, 10002}) {
		t.Fatalf("safe mischief inspect order = %#v, want 10001 then 10002", got)
	}
	payloads := friendMischiefPayloads(caller.calls)
	wantPayloads := []map[string][]int{
		{"bug": []int{1}, "grass": []int{}},
		{"bug": []int{2}, "grass": []int{}},
		{"bug": []int{}, "grass": []int{4}},
	}
	if !reflect.DeepEqual(payloads, wantPayloads) {
		t.Fatalf("safe mischief payloads = %#v, want %#v", payloads, wantPayloads)
	}
	if !reflect.DeepEqual(waits, []time.Duration{200 * time.Millisecond, time.Second}) {
		t.Fatalf("safe mischief waits = %#v, want land then friend wait", waits)
	}
}

func TestRuntimeFacadeFriendMischiefTriesNextCandidateWhenInspectHasNoMischiefLands(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "workCounts": map[string]any{"mischief": float64(1)}},
			map[string]any{"gid": float64(10002), "workCounts": map[string]any{"mischief": float64(1)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": fakeRuntimeResponseSequence{
			map[string]any{"ok": true, "workLandIds": map[string]any{"bug": []any{}, "grass": []any{}}},
			map[string]any{"ok": true, "workLandIds": map[string]any{"bug": []any{float64(5)}, "grass": []any{}}},
		},
		"gameCtl.friendMischiefLandsBatch": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_mischief")

	if !result.OK {
		t.Fatalf("friend_mischief should continue to next candidate: %#v", result)
	}
	if got := inspectHostGIDs(caller.calls); !reflect.DeepEqual(got, []int{10001, 10002}) {
		t.Fatalf("inspect host gids = %#v, want 10001 then 10002; calls=%#v", got, caller.calls)
	}
	mischief := caller.calls[len(caller.calls)-1]
	if mischief.method != "gameCtl.friendMischiefLandsBatch" {
		t.Fatalf("last method = %q, want friendMischiefLandsBatch", mischief.method)
	}
	if got := mischief.args[0].(map[string]any)["hostGid"]; got != 10002 {
		t.Fatalf("mischief hostGid = %#v, want 10002", got)
	}
}

func TestRuntimeFacadeFriendMischiefTriesFriendWhenMischiefCountMissing(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{
			map[string]any{"gid": float64(10001), "workCounts": map[string]any{"collect": float64(0), "water": float64(0)}},
		},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{
			"ok":          true,
			"workLandIds": map[string]any{"bug": []any{float64(9)}},
		},
		"gameCtl.friendMischiefLandsBatch": map[string]any{"ok": true},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_mischief")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("friend_mischief should try friends when mischief count is missing, got %#v", result)
	}
	gotMethods := calledRuntimeMethods(caller.calls)
	wantMethods := []string{"gameCtl.getFriendList", "gameCtl.inspectFriendFarmByProtocol", "gameCtl.friendMischiefLandsBatch"}
	if !reflect.DeepEqual(gotMethods, wantMethods) {
		t.Fatalf("methods = %#v, want %#v", gotMethods, wantMethods)
	}
}

func TestRuntimeFacadeFriendMischiefSkipsWhenNoTargets(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": []any{map[string]any{"gid": float64(10002), "workCounts": map[string]any{"mischief": float64(0)}}},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_mischief")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("no mischief targets should be OK skip, got %#v", result)
	}
}

func TestRuntimeFacadeFriendMischiefFailsWhenProtocolInspectNotOK(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList":               []any{map[string]any{"gid": float64(10001), "workCounts": map[string]any{"mischief": float64(1)}}},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": false, "reason": "enter_failed"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_mischief")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("protocol inspect ok=false should fail friend_mischief, got %#v", result)
	}
}

func TestRuntimeFacadeFriendMischiefHandlesNilRuntimeCaller(t *testing.T) {
	facade := NewRuntimeFacade(nil)

	result := facade.RunTask(context.Background(), "friend_mischief")

	if result.OK || result.Status != StatusRuntimeNotReady {
		t.Fatalf("nil runtime should be runtime_not_ready, got %#v", result)
	}
}

func TestRuntimeFacadeFriendMischiefReportsRuntimeError(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": errors.New("friend list unavailable"),
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_mischief")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("runtime error should fail, got %#v", result)
	}
}

func TestRuntimeFacadeFriendMischiefReportsRuntimeNotOKResult(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList":               []any{map[string]any{"gid": float64(10001), "workCounts": map[string]any{"mischief": float64(1)}}},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": true, "workLandIds": map[string]any{"bug": []any{float64(1)}}},
		"gameCtl.friendMischiefLandsBatch":    map[string]any{"ok": false, "reason": "dispatch_failed", "results": []any{map[string]any{"error": "好友农场暂时不可捣乱"}}},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_mischief")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("runtime ok=false should fail, got %#v", result)
	}
	if !strings.Contains(result.Message, "好友农场暂时不可捣乱") {
		t.Fatalf("message should include runtime error detail, got %q", result.Message)
	}
}

func TestRuntimeFacadeFriendMischiefSkipsWhenDailyLimitMarkedToday(t *testing.T) {
	config := DefaultConfig()
	config[FriendMischiefDailyDoneDateConfigKey] = time.Now().Format("2006-01-02")
	caller := &fakeRuntimeCaller{responses: map[string]any{}}
	facade := NewRuntimeFacadeWithConfig(caller, config)

	result := facade.RunTask(context.Background(), "friend_mischief")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("marked daily limit should skip as OK, got %#v", result)
	}
	if !strings.Contains(result.Message, "今日") || !strings.Contains(result.Message, "跳过") {
		t.Fatalf("skip message should mention today's skip, got %q", result.Message)
	}
	if len(caller.calls) != 0 {
		t.Fatalf("daily-done skip should not call runtime, got %#v", caller.calls)
	}
}

func TestRuntimeFacadeFriendMischiefTreatsDailyLimitReplyAsCompleted(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList":               []any{map[string]any{"gid": float64(10001), "workCounts": map[string]any{"mischief": float64(1)}}},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": true, "workLandIds": map[string]any{"bug": []any{float64(1)}}},
		"gameCtl.friendMischiefLandsBatch":    map[string]any{"ok": false, "reason": "dispatch_failed", "results": []any{map[string]any{"error": "操作次数已达上限"}}},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "friend_mischief")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("daily limit reply should complete auto mischief, got %#v", result)
	}
	if !strings.Contains(result.Message, "操作次数已达上限") || !strings.Contains(result.Message, "已停止") {
		t.Fatalf("completion message should include limit detail and stop semantics, got %q", result.Message)
	}
}
