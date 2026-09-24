package automation

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"
)

type countingRuntimeCaller struct {
	mu      sync.Mutex
	calls   int
	values  []any
	errors  []error
	delay   time.Duration
	started chan struct{}
	release chan struct{}
}

func TestRuntimeMutationGroupsDoesNotSpecialCaseGoldenBug(t *testing.T) {
	if got := runtimeMutationGroups("gameCtl.cleanGoldenBugsByProtocol"); len(got) != 0 {
		t.Fatalf("golden-bug method groups = %#v, want none", got)
	}
}

func (c *countingRuntimeCaller) Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error) {
	c.mu.Lock()
	c.calls++
	callIndex := c.calls - 1
	started := c.started
	delay := c.delay
	var value any = map[string]any{"call": c.calls, "method": method}
	if callIndex < len(c.values) {
		value = c.values[callIndex]
	}
	var callErr error
	if callIndex < len(c.errors) {
		callErr = c.errors[callIndex]
	}
	release := c.release
	c.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	if release != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-release:
		}
	}
	return value, callErr
}

func (c *countingRuntimeCaller) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func TestRuntimeCallCacheReusesReadWithinTTLAndExpires(t *testing.T) {
	now := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	base := &countingRuntimeCaller{}
	cache := NewRuntimeCallCache(base, RuntimeCallCacheOptions{
		Now:   func() time.Time { return now },
		Scope: func() string { return "account-a/runtime-1" },
	})
	args := []any{map[string]any{"silent": true}}

	first, err := cache.Call(context.Background(), "gameCtl.getFarmStatus", args, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.Call(context.Background(), "gameCtl.getFarmStatus", args, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if base.callCount() != 1 || !reflect.DeepEqual(first, second) {
		t.Fatalf("calls=%d first=%#v second=%#v, want one shared read", base.callCount(), first, second)
	}

	now = now.Add(2 * time.Second)
	if _, err := cache.Call(context.Background(), "gameCtl.getFarmStatus", args, time.Second); err != nil {
		t.Fatal(err)
	}
	if base.callCount() != 2 {
		t.Fatalf("calls=%d, want expired read to hit runtime", base.callCount())
	}
}

func TestRuntimeCallCacheCoalescesConcurrentFriendListReadsIgnoringAutomationSource(t *testing.T) {
	base := &countingRuntimeCaller{delay: 40 * time.Millisecond, started: make(chan struct{}, 2)}
	cache := NewRuntimeCallCache(base, RuntimeCallCacheOptions{Scope: func() string { return "account-a/runtime-1" }})
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make([]any, 2)
	errs := make([]error, 2)
	for i, source := range []string{"farm_go_auto_friend_steal", "farm_go_auto_friend_help"} {
		i, source := i, source
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = cache.Call(ctx, "gameCtl.getFriendList", []any{map[string]any{
				"refresh":     true,
				"waitRefresh": true,
				"silent":      true,
				"source":      source,
			}}, time.Second)
		}()
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("errors=%v", errs)
	}
	if base.callCount() != 1 || !reflect.DeepEqual(results[0], results[1]) {
		t.Fatalf("calls=%d results=%#v, want one singleflight result", base.callCount(), results)
	}
}

func TestRuntimeCallCacheInvalidatesFarmReadsAfterWrite(t *testing.T) {
	base := &countingRuntimeCaller{}
	cache := NewRuntimeCallCache(base, RuntimeCallCacheOptions{Scope: func() string { return "account-a/runtime-1" }})
	ctx := context.Background()
	args := []any{map[string]any{"silent": true}}

	if _, err := cache.Call(ctx, "gameCtl.getFarmStatus", args, time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Call(ctx, "gameCtl.harvestLandsBatchByProtocol", []any{map[string]any{"landIds": []int{1}}}, time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Call(ctx, "gameCtl.getFarmStatus", args, time.Second); err != nil {
		t.Fatal(err)
	}
	if base.callCount() != 3 {
		t.Fatalf("calls=%d, want read/write/read after invalidation", base.callCount())
	}
}

func TestRuntimeCallCacheSeparatesScopesAndClear(t *testing.T) {
	scope := "account-a/runtime-1"
	base := &countingRuntimeCaller{}
	cache := NewRuntimeCallCache(base, RuntimeCallCacheOptions{Scope: func() string { return scope }})
	ctx := context.Background()
	args := []any{map[string]any{"silent": true}}

	if _, err := cache.Call(ctx, "gameCtl.getPlayerProfile", args, time.Second); err != nil {
		t.Fatal(err)
	}
	scope = "account-b/runtime-1"
	if _, err := cache.Call(ctx, "gameCtl.getPlayerProfile", args, time.Second); err != nil {
		t.Fatal(err)
	}
	cache.Clear()
	if _, err := cache.Call(ctx, "gameCtl.getPlayerProfile", args, time.Second); err != nil {
		t.Fatal(err)
	}
	if base.callCount() != 3 {
		t.Fatalf("calls=%d, want account isolation plus explicit reconnect clear", base.callCount())
	}
}

func TestRuntimeCallCacheExplicitRefreshInvalidatesOlderSnapshot(t *testing.T) {
	base := &countingRuntimeCaller{values: []any{"cached", "refreshed", "after-refresh"}}
	cache := NewRuntimeCallCache(base, RuntimeCallCacheOptions{Scope: func() string { return "account-a/runtime-1" }})
	ctx := context.Background()
	readArgs := []any{map[string]any{"silent": true}}

	if _, err := cache.Call(ctx, "gameCtl.getFarmStatus", readArgs, time.Second); err != nil {
		t.Fatal(err)
	}
	refreshed, err := cache.Call(ctx, "gameCtl.getFarmStatus", []any{map[string]any{
		"silent":  true,
		"refresh": true,
	}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed != "refreshed" {
		t.Fatalf("refresh result=%#v, want refreshed runtime value", refreshed)
	}
	afterRefresh, err := cache.Call(ctx, "gameCtl.getFarmStatus", readArgs, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if base.callCount() != 3 || afterRefresh != "after-refresh" {
		t.Fatalf("calls=%d result=%#v, want stale snapshot invalidated after explicit refresh", base.callCount(), afterRefresh)
	}
}

func TestRuntimeCallCacheDoesNotCacheErrors(t *testing.T) {
	base := &countingRuntimeCaller{
		values: []any{nil, "recovered"},
		errors: []error{context.DeadlineExceeded, nil},
	}
	cache := NewRuntimeCallCache(base, RuntimeCallCacheOptions{Scope: func() string { return "account-a/runtime-1" }})

	if _, err := cache.Call(context.Background(), "gameCtl.getFarmStatus", nil, time.Second); err == nil {
		t.Fatal("first call error=nil, want runtime error")
	}
	value, err := cache.Call(context.Background(), "gameCtl.getFarmStatus", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if base.callCount() != 2 || value != "recovered" {
		t.Fatalf("calls=%d value=%#v, want failed read retried", base.callCount(), value)
	}
}

func TestRuntimeCallCacheClearDetachesInflightRead(t *testing.T) {
	release := make(chan struct{})
	base := &countingRuntimeCaller{
		values:  []any{"old-runtime", "new-runtime"},
		started: make(chan struct{}, 2),
		release: release,
	}
	cache := NewRuntimeCallCache(base, RuntimeCallCacheOptions{Scope: func() string { return "account-a/runtime-1" }})
	ctx := context.Background()

	firstDone := make(chan any, 1)
	go func() {
		value, _ := cache.Call(ctx, "gameCtl.getFarmStatus", nil, time.Second)
		firstDone <- value
	}()
	<-base.started
	cache.Clear()

	secondDone := make(chan any, 1)
	go func() {
		value, _ := cache.Call(ctx, "gameCtl.getFarmStatus", nil, time.Second)
		secondDone <- value
	}()
	<-base.started
	close(release)

	if first, second := <-firstDone, <-secondDone; first != "old-runtime" || second != "new-runtime" {
		t.Fatalf("first=%#v second=%#v, want Clear to detach old in-flight request", first, second)
	}
	if base.callCount() != 2 {
		t.Fatalf("calls=%d, want a new runtime call after Clear", base.callCount())
	}
}

func TestRuntimeCallCacheInvalidatesFarmAfterCompositeFarmMutation(t *testing.T) {
	base := &countingRuntimeCaller{}
	cache := NewRuntimeCallCache(base, RuntimeCallCacheOptions{Scope: func() string { return "account-a/runtime-1" }})
	ctx := context.Background()

	if _, err := cache.Call(ctx, "gameCtl.getFarmStatus", nil, time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Call(ctx, "gameCtl.triggerOneClickOperation", nil, time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Call(ctx, "gameCtl.getFarmStatus", nil, time.Second); err != nil {
		t.Fatal(err)
	}
	if base.callCount() != 3 {
		t.Fatalf("calls=%d, want composite farm mutation to invalidate farm snapshot", base.callCount())
	}
}

func TestRuntimeCallCacheHonorsNoCacheIntent(t *testing.T) {
	base := &countingRuntimeCaller{values: []any{"cached", "no-cache", "after-no-cache"}}
	cache := NewRuntimeCallCache(base, RuntimeCallCacheOptions{Scope: func() string { return "account-a/runtime-1" }})
	ctx := context.Background()

	if _, err := cache.Call(ctx, "gameCtl.getSeedList", nil, time.Second); err != nil {
		t.Fatal(err)
	}
	value, err := cache.Call(ctx, "gameCtl.getSeedList", []any{map[string]any{"noCache": true}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if value != "no-cache" {
		t.Fatalf("noCache result=%#v, want direct runtime value", value)
	}
	value, err = cache.Call(ctx, "gameCtl.getSeedList", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if base.callCount() != 3 || value != "after-no-cache" {
		t.Fatalf("calls=%d value=%#v, want noCache to bypass and invalidate older snapshot", base.callCount(), value)
	}
}
