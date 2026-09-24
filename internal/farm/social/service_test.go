package social

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type fakeRuntimeCaller struct {
	responses     map[string]any
	responseQueue map[string][]any
	calls         []runtimeCall
}

type runtimeCall struct {
	method  string
	args    []any
	timeout time.Duration
}

func (f *fakeRuntimeCaller) Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error) {
	f.calls = append(f.calls, runtimeCall{method: method, args: args, timeout: timeout})
	value := f.responses[method]
	if queued := f.responseQueue[method]; len(queued) > 0 {
		value = queued[0]
		f.responseQueue[method] = queued[1:]
	}
	if err, ok := value.(error); ok {
		return nil, err
	}
	return value, nil
}

type memoryStore struct {
	rules        FriendRules
	preferences  RankingPreferences
	stealRecords []StealRecord
	visitors     []VisitorRecord
	dogGuard     DogGuardState
	rankingPage  RankingPageData
	rankingErr   error
	rankingCalls []RankingQuery
}

func TestFilterFriendCandidatesByRules(t *testing.T) {
	candidates := []map[string]any{
		{"gid": float64(10001), "name": "GID match", "level": float64(12)},
		{"gid": float64(10002), "name": "Name match", "level": float64(12)},
		{"gid": float64(10003), "name": "Remark match", "remark": "VIP", "level": float64(12)},
		{"gid": float64(10004), "name": "Masked", "level": float64(2)},
		{"gid": float64(10005), "name": "Allowed", "level": float64(12)},
		{"gid": float64(10006), "name": "Outside whitelist", "level": float64(12)},
		{"gid": float64(10007), "name": "Whitelisted", "level": float64(12)},
	}
	rules := FriendRules{
		BlacklistEnabled: true,
		BlacklistScopes:  []string{"help"},
		Blacklist:        []string{"10001", "Name match", "VIP", "Allowed"},
		MaskedBlacklist:  true,
		MaskedMaxLevel:   2,
		WhitelistEnabled: true,
		WhitelistScopes:  []string{"help"},
		Whitelist:        []string{"Allowed", "Whitelisted"},
	}

	for _, test := range []struct {
		name  string
		scope string
		want  []int
	}{
		{name: "help applies blacklist before whitelist", scope: "help", want: []int{10007}},
		{name: "inactive steal scope retains candidates", scope: "steal", want: []int{10001, 10002, 10003, 10004, 10005, 10006, 10007}},
	} {
		t.Run(test.name, func(t *testing.T) {
			filtered := FilterFriendCandidatesByRules(candidates, rules, test.scope)
			got := make([]int, 0, len(filtered))
			for _, candidate := range filtered {
				got = append(got, PositiveInt(candidate["gid"]))
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("FilterFriendCandidatesByRules(%q) = %v, want %v", test.scope, got, test.want)
			}
		})
	}
}

type failingRankingPreferencesStore struct {
	*memoryStore
}

type failingDogGuardStore struct {
	*memoryStore
}

func (s *failingRankingPreferencesStore) SaveRankingPreferences(context.Context, string, RankingPreferences) error {
	return errors.New("save failed")
}

func (s *failingDogGuardStore) LoadDogGuardState(context.Context, string) (DogGuardState, error) {
	return DogGuardState{}, errors.New("load failed")
}

func (m *memoryStore) LoadFriendRules(ctx context.Context, accountKey string) (FriendRules, error) {
	return m.rules, nil
}

func (m *memoryStore) SaveFriendRules(ctx context.Context, accountKey string, rules FriendRules) error {
	m.rules = NormalizeFriendRules(rules)
	return nil
}

func (m *memoryStore) LoadRankingPreferences(ctx context.Context, accountKey string) (RankingPreferences, error) {
	return NormalizeRankingPreferences(m.preferences), nil
}

func (m *memoryStore) SaveRankingPreferences(ctx context.Context, accountKey string, preferences RankingPreferences) error {
	m.preferences = NormalizeRankingPreferences(preferences)
	return nil
}

func (m *memoryStore) ListStealRecords(ctx context.Context, accountKey string, dateKeys []string) ([]StealRecord, error) {
	return m.stealRecords, nil
}

func (m *memoryStore) ListVisitorRecords(ctx context.Context, accountKey string) ([]VisitorRecord, error) {
	return m.visitors, nil
}

func (m *memoryStore) QueryRankingPage(ctx context.Context, accountKey string, query RankingQuery) (RankingPageData, error) {
	m.rankingCalls = append(m.rankingCalls, query)
	return m.rankingPage, m.rankingErr
}

func TestServiceStateHidesProtectedFriendGIDs(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{"list": []any{
			map[string]any{"gid": float64(10001), "name": "Normal", "workCounts": map[string]any{"collect": float64(1)}},
			map[string]any{"gid": float64(1184649322), "name": "Protected", "workCounts": map[string]any{"collect": float64(9)}},
			map[string]any{"gid": "1142601927", "name": "AlsoProtected", "workCounts": map[string]any{"help": float64(3)}},
		}},
	}}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	state := service.State(context.Background(), StateRequest{Refresh: true})

	if !state.OK || len(state.Friends) != 1 {
		t.Fatalf("state = %#v", state)
	}
	if state.Friends[0].GID != 10001 {
		t.Fatalf("visible friend = %#v", state.Friends[0])
	}
	if state.Summary.TotalFriends != 1 || state.Summary.Protected != 0 {
		t.Fatalf("summary = %#v", state.Summary)
	}
}

func TestServiceStateLoadsRuntimeFriendsAndAppliesRules(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{"list": []any{
			map[string]any{"gid": float64(10001), "name": "A", "level": float64(12), "workCounts": map[string]any{"collect": float64(2)}},
			map[string]any{"gid": float64(10002), "name": "B", "level": float64(1), "workCounts": map[string]any{"help": float64(1)}},
		}},
	}}
	store := &memoryStore{rules: FriendRules{
		Blacklist:       []string{"10002"},
		Whitelist:       []string{"10001"},
		MaskedBlacklist: true,
		MaskedMaxLevel:  1,
	}}
	service := NewService(store, caller, Options{AccountKey: "account-a"})

	state := service.State(context.Background(), StateRequest{Refresh: true})

	if !state.OK || len(state.Friends) != 2 {
		t.Fatalf("state = %#v", state)
	}
	if !state.Friends[0].Whitelisted || !state.Friends[0].Stealable {
		t.Fatalf("first friend not enriched: %#v", state.Friends[0])
	}
	if !state.Friends[1].Blacklisted || !state.Friends[1].MaskedBlocked || !state.Friends[1].Helpable {
		t.Fatalf("second friend not marked: %#v", state.Friends[1])
	}
	if state.Summary.TotalFriends != 2 || state.Summary.StealableFriends != 1 || state.Summary.HelpableFriends != 1 {
		t.Fatalf("summary = %#v", state.Summary)
	}
}

func TestServiceStateMarksOnlyCurrentFriendsFromDogGuardCache(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{"list": []any{
			map[string]any{"gid": float64(10001), "name": "A"},
			map[string]any{"gid": float64(10002), "name": "B"},
		}},
	}}
	store := &memoryStore{dogGuard: DogGuardState{Results: []DogGuardRow{
		{GID: 10001, Scanned: true, HasGuardDog: true},
		{GID: 99999, Scanned: true, HasGuardDog: true},
	}}}
	service := NewService(store, caller, Options{AccountKey: "gid:10000"})

	state := service.State(context.Background(), StateRequest{})

	if !state.Friends[0].HasGuardDog || state.Friends[1].HasGuardDog {
		t.Fatalf("friend marks = %#v", state.Friends)
	}
	if state.Summary.DogGuardCount != 1 {
		t.Fatalf("dog guard count = %d, want 1", state.Summary.DogGuardCount)
	}
}

func TestServiceStateReturnsStructuredDogGuardCacheError(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{"list": []any{
			map[string]any{"gid": float64(10001), "name": "A"},
		}},
	}}
	store := &failingDogGuardStore{memoryStore: &memoryStore{}}
	service := NewService(store, caller, Options{AccountKey: "gid:10000"})

	state := service.State(context.Background(), StateRequest{})

	if state.OK || state.Status != StatusFailed {
		t.Fatalf("state = %#v", state)
	}
	if len(state.Friends) != 0 {
		t.Fatalf("friends should be empty on dog guard cache error: %#v", state.Friends)
	}
}

func TestServiceStateReturnsStructuredRuntimeError(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": errors.New("runtime disconnected"),
	}}
	service := NewService(&memoryStore{}, caller, Options{AccountKey: "account-a"})

	state := service.State(context.Background(), StateRequest{Refresh: true})

	if state.OK || state.Status != StatusFailed {
		t.Fatalf("state = %#v", state)
	}
	if len(state.Friends) != 0 {
		t.Fatalf("friends should be empty on runtime error: %#v", state.Friends)
	}
}

func TestServiceStateHandlesNilRuntimeCaller(t *testing.T) {
	service := NewService(&memoryStore{}, nil, Options{AccountKey: "account-a"})

	state := service.State(context.Background(), StateRequest{})

	if state.OK || state.Status != StatusRuntimeNotReady {
		t.Fatalf("state = %#v", state)
	}
}

func TestServiceRankingQueryDoesNotCallRuntime(t *testing.T) {
	store := &memoryStore{rankingPage: RankingPageData{Rows: []RankingPageRow{}, HasMore: false}}
	caller := &fakeRuntimeCaller{responses: map[string]any{"gameCtl.getVisitorRecords": errors.New("must not run")}}

	page := NewService(store, caller, Options{AccountKey: "gid:1"}).Rankings(
		context.Background(), RankingRequest{Tab: "visitors", ViewMode: "timeline", DateRange: "all"},
	)

	if !page.OK || len(caller.calls) != 0 {
		t.Fatalf("page=%#v calls=%#v", page, caller.calls)
	}
	if len(store.rankingCalls) != 1 {
		t.Fatalf("ranking calls = %#v", store.rankingCalls)
	}
}

func TestServiceRankingsReturnsStructuredValidationFailure(t *testing.T) {
	page := NewService(&memoryStore{}, nil, Options{AccountKey: "gid:1"}).Rankings(
		context.Background(), RankingRequest{Tab: "bad"},
	)

	if page.OK || page.Status != StatusFailed || page.Rows == nil {
		t.Fatalf("page = %#v", page)
	}
}

func TestServiceRankingsNormalizesRowsItemsAndNextCursor(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	store := &memoryStore{rankingPage: RankingPageData{
		Rows:    []RankingPageRow{{Kind: "stealRecord", Key: "row-1", TimeMS: 1234}},
		HasMore: true,
	}}
	service := NewService(store, nil, Options{AccountKey: "gid:1"})

	page := service.Rankings(context.Background(), RankingRequest{
		Tab: "stolenByMe", ViewMode: "timeline", DateRange: "all", Now: now,
	})

	if !page.OK || len(page.Rows) != 1 || page.Rows[0].Items == nil || page.NextCursor == "" {
		t.Fatalf("page = %#v", page)
	}
	store.rankingPage.HasMore = false
	page = service.Rankings(context.Background(), RankingRequest{
		Tab: "stolenByMe", ViewMode: "timeline", DateRange: "all", Now: now,
	})
	if page.NextCursor != "" {
		t.Fatalf("next cursor without more rows = %q", page.NextCursor)
	}
}

func TestServiceRankingPreferencesAreAccountScopedAndNormalized(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store, nil, Options{AccountKey: "account-a"})

	saved, err := service.SaveRankingPreferences(context.Background(), RankingPreferences{
		StolenByMeViewMode:   "ranking",
		StolenFromMeViewMode: "timeline",
	})
	if err != nil {
		t.Fatalf("save ranking preferences: %v", err)
	}
	got := service.RankingPreferences(context.Background())
	fallback, err := service.SaveRankingPreferences(context.Background(), RankingPreferences{
		StolenByMeViewMode:   "bad-value",
		StolenFromMeViewMode: "ranking",
	})
	if err != nil {
		t.Fatalf("save normalized ranking preferences: %v", err)
	}

	if saved.StolenByMeViewMode != "ranking" || saved.StolenFromMeViewMode != "timeline" ||
		got.StolenByMeViewMode != "ranking" || got.StolenFromMeViewMode != "timeline" {
		t.Fatalf("preferences were not persisted: saved=%#v got=%#v", saved, got)
	}
	if fallback.StolenByMeViewMode != "timeline" || fallback.StolenFromMeViewMode != "ranking" {
		t.Fatalf("preferences should normalize independently: %#v", fallback)
	}
}

func TestNormalizeRankingPreferencesDefaultsEachModeIndependently(t *testing.T) {
	tests := []struct {
		name  string
		input RankingPreferences
		want  RankingPreferences
	}{
		{name: "missing", want: RankingPreferences{StolenByMeViewMode: "timeline", StolenFromMeViewMode: "timeline"}},
		{name: "stolen by me ranking", input: RankingPreferences{StolenByMeViewMode: "ranking"}, want: RankingPreferences{StolenByMeViewMode: "ranking", StolenFromMeViewMode: "timeline"}},
		{name: "stolen from me ranking", input: RankingPreferences{StolenFromMeViewMode: "ranking"}, want: RankingPreferences{StolenByMeViewMode: "timeline", StolenFromMeViewMode: "ranking"}},
		{name: "invalid values", input: RankingPreferences{StolenByMeViewMode: "grid", StolenFromMeViewMode: "list"}, want: RankingPreferences{StolenByMeViewMode: "timeline", StolenFromMeViewMode: "timeline"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeRankingPreferences(test.input); got != test.want {
				t.Fatalf("NormalizeRankingPreferences(%#v) = %#v, want %#v", test.input, got, test.want)
			}
		})
	}
}

func TestServiceSaveRankingPreferencesReturnsStorageError(t *testing.T) {
	store := &failingRankingPreferencesStore{memoryStore: &memoryStore{}}
	service := NewService(store, nil, Options{AccountKey: "gid:10001"})

	saved, err := service.SaveRankingPreferences(
		context.Background(),
		RankingPreferences{StolenByMeViewMode: "ranking", StolenFromMeViewMode: "timeline"},
	)
	if err == nil {
		t.Fatal("expected ranking preference save error")
	}
	if saved != (RankingPreferences{}) {
		t.Fatalf("failed save returned successful preferences: %#v", saved)
	}
}

func TestServiceRefreshVisitorsPersistsWithoutReturningRankingRows(t *testing.T) {
	store := &memoryStore{}
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getVisitorRecords": map[string]any{
			"ok": true,
			"records": []any{
				map[string]any{
					"playerId":      float64(10002),
					"displayName":   "B",
					"actionType":    float64(1),
					"time":          float64(1783497600),
					"stealItemName": "白萝卜",
					"stealItemNum":  float64(4),
				},
			},
		},
	}}
	service := NewService(store, caller, Options{AccountKey: "account-a"})

	result := service.RefreshVisitors(context.Background())

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("refresh result = %#v", result)
	}
	if len(store.visitors) != 1 || store.visitors[0].PlayerID != 10002 {
		t.Fatalf("runtime visitor records were not stored: %#v", store.visitors)
	}
}

type failingVisitorSaveStore struct {
	*memoryStore
}

func (s *failingVisitorSaveStore) SaveVisitorRecords(context.Context, string, []VisitorRecord) error {
	return errors.New("save failed")
}

func TestServiceRefreshVisitorsReturnsSaveFailure(t *testing.T) {
	store := &failingVisitorSaveStore{memoryStore: &memoryStore{}}
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getVisitorRecords": map[string]any{"records": []any{map[string]any{
			"playerId": float64(7), "displayName": "A", "actionType": float64(1), "time": float64(1784534400),
		}}},
	}}

	result := NewService(store, caller, Options{AccountKey: "gid:1"}).RefreshVisitors(context.Background())

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("refresh result = %#v", result)
	}
}

type nonRecordWriterStore struct{}

func (nonRecordWriterStore) LoadFriendRules(context.Context, string) (FriendRules, error) {
	return FriendRules{}, nil
}

func (nonRecordWriterStore) SaveFriendRules(context.Context, string, FriendRules) error {
	return nil
}

func TestServiceRefreshVisitorsFailsWhenStoreCannotPersistRecords(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getVisitorRecords": map[string]any{"records": []any{map[string]any{
			"playerId": float64(7), "displayName": "A", "actionType": float64(1), "time": float64(1784534400),
		}}},
	}}

	result := NewService(nonRecordWriterStore{}, caller, Options{AccountKey: "gid:1"}).RefreshVisitors(context.Background())

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("refresh result = %#v", result)
	}
}

type blockingRuntimeCaller struct {
	started chan struct{}
	release chan struct{}
}

func (c *blockingRuntimeCaller) Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error) {
	close(c.started)
	select {
	case <-c.release:
		return map[string]any{"records": []any{}}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestServiceRuntimeRefreshCanBlockWhileLocalPageQueryCompletes(t *testing.T) {
	store := &memoryStore{rankingPage: RankingPageData{Rows: []RankingPageRow{}}}
	caller := &blockingRuntimeCaller{started: make(chan struct{}), release: make(chan struct{})}
	service := NewService(store, caller, Options{AccountKey: "gid:1"})
	refreshDone := make(chan ActionResult, 1)
	go func() { refreshDone <- service.RefreshVisitors(context.Background()) }()
	<-caller.started

	pageDone := make(chan RankingPage, 1)
	go func() {
		pageDone <- service.Rankings(context.Background(), RankingRequest{Tab: "visitors", DateRange: "all"})
	}()
	select {
	case page := <-pageDone:
		if !page.OK {
			t.Fatalf("page = %#v", page)
		}
	case <-time.After(time.Second):
		t.Fatal("local page query blocked behind runtime refresh")
	}
	close(caller.release)
	<-refreshDone
}

func TestServiceStealActionStoresRankingRecord(t *testing.T) {
	store := &memoryStore{}
	caller := &fakeRuntimeCaller{responses: map[string]any{
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
	service := NewService(store, caller, Options{AccountKey: "account-a"})

	result := service.Action(context.Background(), FriendActionRequest{Action: "steal", Target: "10001"})

	if !result.OK {
		t.Fatalf("steal action should succeed: %#v", result)
	}
	if len(store.stealRecords) != 1 {
		t.Fatalf("steal action should write one ranking record: %#v", store.stealRecords)
	}
	if store.stealRecords[0].GID != 10001 || store.stealRecords[0].DisplayName != "A" {
		t.Fatalf("stored steal record should identify the friend: %#v", store.stealRecords[0])
	}
	if len(store.stealRecords[0].Items) != 1 || store.stealRecords[0].Items[0].Name != "百香果" || store.stealRecords[0].Items[0].Count != 6 {
		t.Fatalf("stored steal record should include item details: %#v", store.stealRecords[0].Items)
	}
}
