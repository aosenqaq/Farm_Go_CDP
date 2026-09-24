package social

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func (m *memoryStore) LoadDogGuardState(ctx context.Context, accountKey string) (DogGuardState, error) {
	return m.dogGuard, nil
}

func (m *memoryStore) SaveDogGuardState(ctx context.Context, accountKey string, state DogGuardState) error {
	m.dogGuard = state
	return nil
}

func TestDogGuardScanScansAllFriendsAndSleepsBetweenCandidates(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{"list": []any{
			map[string]any{"gid": float64(10001), "name": "A"},
			map[string]any{"gid": float64(10002), "name": "B"},
			map[string]any{"gid": float64(10003), "name": "C"},
		}},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": true, "briefDogInfo": map[string]any{
			"dogId":   float64(90021),
			"dogName": "护主犬",
		}},
	}}
	store := &memoryStore{}
	var sleeps []time.Duration
	scanner := NewDogGuardScanner(store, caller, DogGuardOptions{
		AccountKey: "account-a",
		Sleep: func(duration time.Duration) {
			sleeps = append(sleeps, duration)
		},
	})

	state := scanner.Start(context.Background(), DogGuardScanRequest{Refresh: true})
	if !state.Running {
		t.Fatalf("state after start = %#v", state)
	}
	state = scanner.Wait(context.Background())

	if state.Running || state.Total != 3 || state.Scanned != 3 || state.HasGuardDogCount != 3 || len(state.Results) != 3 {
		t.Fatalf("state = %#v", state)
	}
	if store.dogGuard.HasGuardDogCount != 3 {
		t.Fatalf("cache not persisted: %#v", store.dogGuard)
	}
	wantSleeps := []time.Duration{300 * time.Millisecond, 300 * time.Millisecond}
	if !reflect.DeepEqual(sleeps, wantSleeps) {
		t.Fatalf("sleeps = %#v, want %#v", sleeps, wantSleeps)
	}
	for _, call := range caller.calls {
		switch call.method {
		case "gameCtl.startRuntimeSpies", "gameCtl.resetRuntimeSpyEvents", "gameCtl.inspectDogGuardSignals", "gameCtl.enterFriendFarm":
			t.Fatalf("unexpected runtime spy call: %#v", call)
		}
	}
}

func TestDogGuardScanUsesCustomScanInterval(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{"list": []any{
			map[string]any{"gid": float64(10001), "name": "A"},
			map[string]any{"gid": float64(10002), "name": "B"},
		}},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": true, "briefDogInfo": map[string]any{}},
	}}
	var sleeps []time.Duration
	scanner := NewDogGuardScanner(&memoryStore{}, caller, DogGuardOptions{
		Sleep: func(duration time.Duration) {
			sleeps = append(sleeps, duration)
		},
	})

	scanner.Start(context.Background(), DogGuardScanRequest{ScanIntervalMS: 125})
	scanner.Wait(context.Background())

	wantSleeps := []time.Duration{125 * time.Millisecond}
	if !reflect.DeepEqual(sleeps, wantSleeps) {
		t.Fatalf("sleeps = %#v, want %#v", sleeps, wantSleeps)
	}
}

func TestDogGuardScanStopInterruptsIntervalBeforeNextCandidate(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{"list": []any{
			map[string]any{"gid": float64(10001), "name": "A"},
			map[string]any{"gid": float64(10002), "name": "B"},
		}},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": true, "briefDogInfo": map[string]any{}},
	}}
	intervalStarted := make(chan struct{})
	releaseInterval := make(chan struct{})
	scanner := NewDogGuardScanner(&memoryStore{}, caller, DogGuardOptions{
		Sleep: func(time.Duration) {
			close(intervalStarted)
			<-releaseInterval
		},
	})

	scanner.Start(context.Background(), DogGuardScanRequest{ScanIntervalMS: 60000})
	<-intervalStarted
	scanner.Stop()
	waitDone := make(chan DogGuardState, 1)
	go func() {
		waitDone <- scanner.Wait(context.Background())
	}()

	var stopped DogGuardState
	returnedPromptly := false
	select {
	case stopped = <-waitDone:
		returnedPromptly = true
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseInterval)
	if !returnedPromptly {
		stopped = scanner.Wait(context.Background())
		t.Fatal("stop did not interrupt the scan interval")
	}
	if stopped.Running || stopped.Scanned != 1 {
		t.Fatalf("state after stop = %#v", stopped)
	}
	inspectCalls := 0
	for _, call := range caller.calls {
		if call.method == "gameCtl.inspectFriendFarmByProtocol" {
			inspectCalls++
		}
	}
	if inspectCalls != 1 {
		t.Fatalf("inspect calls = %d, want one call", inspectCalls)
	}
}

func TestNormalizeDogGuardScanRequestScanInterval(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{name: "zero uses default", in: 0, want: 300},
		{name: "negative uses default", in: -1, want: 300},
		{name: "custom interval is retained", in: 125, want: 125},
		{name: "over maximum is clamped", in: 60001, want: 60000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeDogGuardScanRequest(DogGuardScanRequest{ScanIntervalMS: tt.in})
			if got.ScanIntervalMS != tt.want {
				t.Fatalf("ScanIntervalMS = %d, want %d", got.ScanIntervalMS, tt.want)
			}
		})
	}
}

func TestDogGuardScanRequestContract(t *testing.T) {
	typeOfRequest := reflect.TypeOf(DogGuardScanRequest{})
	required := map[string]string{
		"Refresh":         `json:"refresh,omitempty"`,
		"SkipScanned":     `json:"skipScanned,omitempty"`,
		"ExcludeGuardDog": `json:"excludeGuardDog,omitempty"`,
		"ScanIntervalMS":  `json:"scanIntervalMs,omitempty"`,
	}
	for name, tag := range required {
		field, ok := typeOfRequest.FieldByName(name)
		if !ok {
			t.Fatalf("DogGuardScanRequest is missing %s", name)
		}
		if string(field.Tag) != tag {
			t.Fatalf("%s tag = %q, want %q", name, field.Tag, tag)
		}
	}
	for _, legacy := range []string{"Limit", "EnterWaitMS", "AfterEnterWaitMS"} {
		if _, ok := typeOfRequest.FieldByName(legacy); ok {
			t.Fatalf("DogGuardScanRequest still exposes legacy field %s", legacy)
		}
	}
}

func TestDogGuardScanUsesDefaultIntervalAndSkipsCachedRows(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.getFriendList": map[string]any{"list": []any{
			map[string]any{"gid": float64(10001), "name": "A"},
			map[string]any{"gid": float64(10002), "name": "B"},
			map[string]any{"gid": float64(10003), "name": "C"},
		}},
		"gameCtl.inspectFriendFarmByProtocol": map[string]any{"ok": true, "briefDogInfo": map[string]any{}},
	}}
	store := &memoryStore{dogGuard: DogGuardState{Results: []DogGuardRow{
		{GID: 10001, Name: "A", Scanned: true, HasGuardDog: false},
		{GID: 10002, Name: "B", Scanned: true, HasGuardDog: true, DogID: GuardDogID},
	}}}
	scanner := NewDogGuardScanner(store, caller, DogGuardOptions{
		AccountKey: "account-a",
		Sleep:      func(time.Duration) {},
	})

	scanner.Start(context.Background(), DogGuardScanRequest{SkipScanned: true, ExcludeGuardDog: true})
	state := scanner.Wait(context.Background())

	if state.Total != 1 || len(state.Results) != 3 {
		t.Fatalf("state = %#v", state)
	}
	var inspectCalls []runtimeCall
	for _, call := range caller.calls {
		switch call.method {
		case "gameCtl.startRuntimeSpies", "gameCtl.resetRuntimeSpyEvents", "gameCtl.inspectDogGuardSignals", "gameCtl.enterFriendFarm":
			t.Fatalf("unexpected runtime spy call: %#v", call)
		case "gameCtl.inspectFriendFarmByProtocol":
			inspectCalls = append(inspectCalls, call)
		}
	}
	if len(inspectCalls) != 1 {
		t.Fatalf("inspect calls = %#v, want one call", inspectCalls)
	}
	args := inspectCalls[0].args[0].(map[string]any)
	if args["hostGid"] != 10003 {
		t.Fatalf("hostGid = %#v, want 10003", args["hostGid"])
	}
	if args["waitReplyMs"] != 5000 {
		t.Fatalf("waitReplyMs = %#v, want 5000", args["waitReplyMs"])
	}
}
