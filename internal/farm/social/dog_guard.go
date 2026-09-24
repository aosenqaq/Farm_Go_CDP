package social

import (
	"context"
	"strings"
	"sync"
	"time"
)

const (
	GuardDogID                    = 90021
	defaultDogGuardScanIntervalMS = 300
	maxDogGuardScanIntervalMS     = 60000
)

type DogGuardStore interface {
	LoadDogGuardState(ctx context.Context, accountKey string) (DogGuardState, error)
	SaveDogGuardState(ctx context.Context, accountKey string, state DogGuardState) error
}

type DogGuardOptions struct {
	AccountKey string
	Sleep      func(time.Duration)
}

type DogGuardScanner struct {
	store      DogGuardStore
	caller     RuntimeCaller
	accountKey string
	sleep      func(time.Duration)

	mu     sync.Mutex
	state  DogGuardState
	active chan struct{}
	stop   chan struct{}
}

func NewDogGuardScanner(store DogGuardStore, caller RuntimeCaller, opts DogGuardOptions) *DogGuardScanner {
	accountKey := opts.AccountKey
	if accountKey == "" {
		accountKey = "default"
	}
	scanner := &DogGuardScanner{
		store:      store,
		caller:     caller,
		accountKey: accountKey,
		sleep:      opts.Sleep,
		state:      DogGuardState{Results: []DogGuardRow{}},
	}
	if store != nil {
		if cached, err := store.LoadDogGuardState(context.Background(), accountKey); err == nil && len(cached.Results) > 0 {
			scanner.state = normalizeDogGuardState(cached)
		}
	}
	return scanner
}

func (s *DogGuardScanner) State() DogGuardState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneDogGuardState(s.state)
}

func (s *DogGuardScanner) Start(ctx context.Context, req DogGuardScanRequest) DogGuardState {
	req = normalizeDogGuardScanRequest(req)
	s.mu.Lock()
	if s.state.Running {
		defer s.mu.Unlock()
		return cloneDogGuardState(s.state)
	}
	s.state.Running = true
	s.state.StopRequested = false
	s.state.StartedAt = time.Now().Format(time.RFC3339Nano)
	s.state.FinishedAt = ""
	s.state.Error = ""
	s.state.Current = nil
	s.state.Total = 0
	s.state.Scanned = 0
	s.active = make(chan struct{})
	s.stop = make(chan struct{})
	active := s.active
	stop := s.stop
	s.mu.Unlock()

	go func() {
		defer close(active)
		s.run(ctx, req, stop)
	}()
	return s.State()
}

func (s *DogGuardScanner) Stop() DogGuardState {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requestStopLocked()
	return cloneDogGuardState(s.state)
}

func (s *DogGuardScanner) Clear(ctx context.Context) DogGuardState {
	s.mu.Lock()
	if s.state.Running {
		s.requestStopLocked()
		defer s.mu.Unlock()
		return cloneDogGuardState(s.state)
	}
	s.state = DogGuardState{Results: []DogGuardRow{}}
	next := cloneDogGuardState(s.state)
	s.mu.Unlock()
	s.persist(ctx, next)
	return next
}

func (s *DogGuardScanner) Wait(ctx context.Context) DogGuardState {
	s.mu.Lock()
	active := s.active
	s.mu.Unlock()
	if active != nil {
		select {
		case <-active:
		case <-ctx.Done():
		}
	}
	return s.State()
}

func (s *DogGuardScanner) run(ctx context.Context, req DogGuardScanRequest, stop <-chan struct{}) {
	if s.caller == nil {
		s.finish(ctx, "游戏运行时尚未连接，无法读取护主犬。")
		return
	}
	friendValue, err := s.caller.Call(ctx, "gameCtl.getFriendList", []any{map[string]any{
		"refresh":     req.Refresh,
		"sort":        true,
		"includeSelf": false,
		"waitRefresh": req.Refresh,
		"silent":      true,
	}}, 30*time.Second)
	if err != nil {
		s.finish(ctx, "读取好友列表失败："+err.Error())
		return
	}
	friends := s.filterDogGuardScanFriends(enrichFriendRows(friendValue, FriendRules{}), req)
	s.mu.Lock()
	s.state.Total = len(friends)
	s.mu.Unlock()

	for index, friend := range friends {
		if s.shouldStop() {
			break
		}
		row := DogGuardRow{GID: friend.GID, Name: friend.DisplayName, DisplayName: friend.DisplayName, Remark: friend.Remark, Level: friend.Level}
		s.setCurrent(row)
		scanned := s.scanFriend(ctx, friend.GID)
		row.Scanned = true
		row.HasGuardDog = scanned.dogID == GuardDogID
		row.DogID = scanned.dogID
		row.DogName = scanned.dogName
		row.DogConfig = scanned.dogConfig
		row.Error = scanned.errText
		row.ScannedAt = time.Now().Format(time.RFC3339Nano)
		s.upsertRow(ctx, row)
		if index+1 < len(friends) && !s.shouldStop() {
			if !s.waitForScanInterval(ctx, stop, time.Duration(req.ScanIntervalMS)*time.Millisecond) {
				break
			}
		}
	}
	s.finish(ctx, "")
}

func normalizeDogGuardScanRequest(req DogGuardScanRequest) DogGuardScanRequest {
	if req.ScanIntervalMS <= 0 {
		req.ScanIntervalMS = defaultDogGuardScanIntervalMS
	}
	if req.ScanIntervalMS > maxDogGuardScanIntervalMS {
		req.ScanIntervalMS = maxDogGuardScanIntervalMS
	}
	return req
}

func (s *DogGuardScanner) filterDogGuardScanFriends(friends []FriendRow, req DogGuardScanRequest) []FriendRow {
	if !req.SkipScanned && !req.ExcludeGuardDog {
		return friends
	}
	s.mu.Lock()
	previous := make(map[int]DogGuardRow, len(s.state.Results))
	for _, row := range s.state.Results {
		if row.GID > 0 {
			previous[row.GID] = row
		}
	}
	s.mu.Unlock()
	filtered := make([]FriendRow, 0, len(friends))
	for _, friend := range friends {
		row, exists := previous[friend.GID]
		if exists && req.SkipScanned {
			continue
		}
		if exists && req.ExcludeGuardDog && row.HasGuardDog {
			continue
		}
		filtered = append(filtered, friend)
	}
	return filtered
}

type dogSignal struct {
	dogID     int
	dogName   string
	dogConfig map[string]any
	errText   string
}

func (s *DogGuardScanner) scanFriend(ctx context.Context, gid int) dogSignal {
	result, err := s.caller.Call(ctx, "gameCtl.inspectFriendFarmByProtocol", []any{map[string]any{
		"hostGid":      gid,
		"silent":       true,
		"includeLands": false,
		"leaveAfter":   true,
		"waitReplyMs":  5000,
	}}, 90*time.Second)
	if err != nil {
		return dogSignal{errText: err.Error()}
	}
	if failed, reason := runtimeResultFailed(result); failed {
		return dogSignal{errText: reason}
	}
	info := mapFromAny(mapFromAny(result)["briefDogInfo"])
	return dogSignal{
		dogID:   PositiveInt(firstExistingAny(info["dogId"], info["dog_id"])),
		dogName: firstText(info["dogName"], info["dog_name"]),
	}
}

func pickDogSignal(payload any) dogSignal {
	var signal dogSignal
	for _, scan := range sliceFromAny(mapFromAny(payload)["sourceScans"]) {
		for _, match := range sliceFromAny(mapFromAny(scan)["matches"]) {
			item := mapFromAny(match)
			path := strings.ToLower(firstText(item["path"]))
			if strings.HasSuffix(path, ".pet_dog.id") || strings.HasSuffix(path, ".brief_dog_info.dog_id") {
				if id := PositiveInt(item["value"]); id > 0 {
					signal.dogID = id
				}
			}
			if strings.HasSuffix(path, ".pet_dog.config") {
				if primitive := mapFromAny(mapFromAny(item["summary"])["primitive"]); len(primitive) > 0 {
					signal.dogConfig = primitive
					if name := firstText(primitive["name"]); name != "" {
						signal.dogName = name
					}
					if id := PositiveInt(primitive["id"]); id > 0 && signal.dogID == 0 {
						signal.dogID = id
					}
				}
			}
		}
	}
	return signal
}

func (s *DogGuardScanner) shouldStop() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.StopRequested
}

func (s *DogGuardScanner) requestStopLocked() {
	if s.state.StopRequested {
		return
	}
	s.state.StopRequested = true
	if s.stop != nil {
		close(s.stop)
	}
}

func (s *DogGuardScanner) waitForScanInterval(ctx context.Context, stop <-chan struct{}, duration time.Duration) bool {
	if s.sleep == nil {
		timer := time.NewTimer(duration)
		defer timer.Stop()
		select {
		case <-timer.C:
			return true
		case <-stop:
			return false
		case <-ctx.Done():
			return false
		}
	}

	done := make(chan struct{})
	go func() {
		s.sleep(duration)
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-stop:
		return false
	case <-ctx.Done():
		return false
	}
}

func (s *DogGuardScanner) setCurrent(row DogGuardRow) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Current = &row
}

func (s *DogGuardScanner) upsertRow(ctx context.Context, row DogGuardRow) {
	s.mu.Lock()
	replaced := false
	for index := range s.state.Results {
		if s.state.Results[index].GID == row.GID {
			s.state.Results[index] = row
			replaced = true
			break
		}
	}
	if !replaced {
		s.state.Results = append(s.state.Results, row)
	}
	s.state.Scanned++
	s.state.HasGuardDogCount = 0
	for _, item := range s.state.Results {
		if item.HasGuardDog {
			s.state.HasGuardDogCount++
		}
	}
	next := cloneDogGuardState(s.state)
	s.mu.Unlock()
	s.persist(ctx, next)
}

func (s *DogGuardScanner) finish(ctx context.Context, errText string) {
	s.mu.Lock()
	s.state.Running = false
	s.stop = nil
	s.state.Current = nil
	s.state.FinishedAt = time.Now().Format(time.RFC3339Nano)
	if errText != "" {
		s.state.Error = errText
	}
	next := cloneDogGuardState(s.state)
	s.mu.Unlock()
	s.persist(ctx, next)
}

func (s *DogGuardScanner) persist(ctx context.Context, state DogGuardState) {
	if s.store == nil {
		return
	}
	_ = s.store.SaveDogGuardState(ctx, s.accountKey, state)
}

func normalizeDogGuardState(state DogGuardState) DogGuardState {
	if state.Results == nil {
		state.Results = []DogGuardRow{}
	}
	state.HasGuardDogCount = 0
	for _, row := range state.Results {
		if row.HasGuardDog {
			state.HasGuardDogCount++
		}
	}
	return state
}

func cloneDogGuardState(state DogGuardState) DogGuardState {
	if len(state.Results) == 0 {
		state.Results = []DogGuardRow{}
	} else {
		state.Results = append([]DogGuardRow(nil), state.Results...)
	}
	if state.Current != nil {
		current := *state.Current
		state.Current = &current
	}
	return state
}
