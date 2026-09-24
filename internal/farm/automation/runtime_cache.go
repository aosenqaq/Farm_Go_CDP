package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// RuntimeCallCache provides short-lived, account/runtime-scoped reuse for
// read-only runtime calls. Concurrent identical reads share one underlying
// protocol request, while mutations invalidate the affected snapshots.
type RuntimeCallCache struct {
	base  RuntimeCaller
	now   func() time.Time
	scope func() string

	mu       sync.Mutex
	entries  map[string]runtimeCacheEntry
	inflight map[string]*runtimeCacheFlight
	epoch    uint64
}

type RuntimeCallCacheOptions struct {
	Now   func() time.Time
	Scope func() string
}

type runtimeCachePolicy struct {
	ttl   time.Duration
	group string
}

type runtimeCacheEntry struct {
	value     any
	expiresAt time.Time
	group     string
}

type runtimeCacheFlight struct {
	done  chan struct{}
	value any
	err   error
	epoch uint64
	group string
}

var runtimeCachePolicies = map[string]runtimeCachePolicy{
	"gameCtl.getFarmStatus":                {ttl: 900 * time.Millisecond, group: "farm"},
	"gameCtl.getFriendList":                {ttl: 4 * time.Second, group: "friends"},
	"gameCtl.getFriendListRaw":             {ttl: 4 * time.Second, group: "friends"},
	"gameCtl.getPlayerProfile":             {ttl: 2 * time.Second, group: "profile"},
	"gameCtl.getFertilizerContainerStatus": {ttl: 1200 * time.Millisecond, group: "fertilizer"},
	"gameCtl.getSeedList":                  {ttl: 10 * time.Second, group: "inventory"},
	"gameCtl.getWarehouseItems":            {ttl: 2 * time.Second, group: "inventory"},
	"gameCtl.getShopGoodsList":             {ttl: 10 * time.Second, group: "shop"},
	"gameCtl.getShopSeedList":              {ttl: 10 * time.Second, group: "shop"},
	"gameCtl.getGodRankList":               {ttl: 3 * time.Second, group: "social"},
	"gameCtl.getVisitorRecords":            {ttl: 3 * time.Second, group: "social"},
	"gameCtl.getFriendBlockListByProtocol": {ttl: 3 * time.Second, group: "social"},
	"gameCtl.getAtlasVisibleUnlockRows":    {ttl: 15 * time.Second, group: "atlas"},
	"gameCtl.collectAtlasUnlockRows":       {ttl: 15 * time.Second, group: "atlas"},
}

func NewRuntimeCallCache(base RuntimeCaller, options RuntimeCallCacheOptions) *RuntimeCallCache {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	scope := options.Scope
	if scope == nil {
		scope = func() string { return "default" }
	}
	return &RuntimeCallCache{
		base:     base,
		now:      now,
		scope:    scope,
		entries:  map[string]runtimeCacheEntry{},
		inflight: map[string]*runtimeCacheFlight{},
	}
}

func (c *RuntimeCallCache) Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error) {
	if c == nil || c.base == nil {
		return nil, fmt.Errorf("runtime caller is not configured")
	}
	policy, cacheable := runtimeCachePolicies[method]
	if !cacheable || shouldBypassRuntimeCache(args) {
		value, err := c.base.Call(ctx, method, args, timeout)
		if err == nil {
			if cacheable {
				c.invalidateGroups([]string{policy.group})
			} else {
				c.invalidateGroups(runtimeMutationGroups(method))
			}
		}
		return value, err
	}

	key, err := runtimeCacheKey(c.scope(), method, args)
	if err != nil {
		return c.base.Call(ctx, method, args, timeout)
	}
	now := c.now()

	c.mu.Lock()
	if entry, ok := c.entries[key]; ok {
		if now.Before(entry.expiresAt) {
			c.mu.Unlock()
			return entry.value, nil
		}
		delete(c.entries, key)
	}
	if flight := c.inflight[key]; flight != nil {
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-flight.done:
			return flight.value, flight.err
		}
	}
	flight := &runtimeCacheFlight{done: make(chan struct{}), epoch: c.epoch, group: policy.group}
	c.inflight[key] = flight
	c.mu.Unlock()

	value, callErr := c.base.Call(ctx, method, args, timeout)
	c.mu.Lock()
	flight.value = value
	flight.err = callErr
	if c.inflight[key] == flight {
		delete(c.inflight, key)
	}
	if callErr == nil && flight.epoch == c.epoch {
		c.entries[key] = runtimeCacheEntry{
			value:     value,
			expiresAt: c.now().Add(policy.ttl),
			group:     policy.group,
		}
	}
	close(flight.done)
	c.mu.Unlock()
	return value, callErr
}

// Clear discards every cached snapshot and prevents an in-flight request from
// being stored after an account switch or runtime reconnect.
func (c *RuntimeCallCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.entries = map[string]runtimeCacheEntry{}
	c.inflight = map[string]*runtimeCacheFlight{}
	c.epoch++
	c.mu.Unlock()
}

func (c *RuntimeCallCache) invalidateGroups(groups []string) {
	if c == nil || len(groups) == 0 {
		return
	}
	wanted := make(map[string]bool, len(groups))
	for _, group := range groups {
		wanted[group] = true
	}
	c.mu.Lock()
	for key, entry := range c.entries {
		if wanted[entry.group] {
			delete(c.entries, key)
		}
	}
	for key, flight := range c.inflight {
		if wanted[flight.group] {
			delete(c.inflight, key)
		}
	}
	c.epoch++
	c.mu.Unlock()
}

func runtimeCacheKey(scope string, method string, args []any) (string, error) {
	normalized := normalizeRuntimeCacheValue(args)
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return scope + "\x00" + method + "\x00" + string(encoded), nil
}

func normalizeRuntimeCacheValue(value any) any {
	switch typed := value.(type) {
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = normalizeRuntimeCacheValue(item)
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if key == "source" && strings.HasPrefix(fmt.Sprint(item), "farm_go_auto_") {
				continue
			}
			result[key] = normalizeRuntimeCacheValue(item)
		}
		return result
	default:
		return value
	}
}

func shouldBypassRuntimeCache(args []any) bool {
	refresh, automation := runtimeCacheRefreshIntent(args)
	return refresh && !automation
}

func runtimeCacheRefreshIntent(value any) (refresh bool, automation bool) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			itemRefresh, itemAutomation := runtimeCacheRefreshIntent(item)
			refresh = refresh || itemRefresh
			automation = automation || itemAutomation
		}
	case map[string]any:
		for key, item := range typed {
			switch key {
			case "refresh", "forceRefresh", "waitRefresh", "noCache":
				refresh = refresh || item == true
			case "source":
				automation = automation || strings.HasPrefix(fmt.Sprint(item), "farm_go_auto_")
			default:
				itemRefresh, itemAutomation := runtimeCacheRefreshIntent(item)
				refresh = refresh || itemRefresh
				automation = automation || itemAutomation
			}
		}
	}
	return refresh, automation
}

func runtimeMutationGroups(method string) []string {
	name := strings.ToLower(method)
	groups := map[string]bool{}
	add := func(items ...string) {
		for _, item := range items {
			groups[item] = true
		}
	}
	if strings.Contains(name, "harvest") || strings.Contains(name, "plant") || strings.Contains(name, "farm") ||
		strings.Contains(name, "fertiliz") || strings.Contains(name, "shovel") || strings.Contains(name, "landupgrade") ||
		strings.Contains(name, "upgradeland") || strings.Contains(name, "oneclick") {
		add("farm", "profile", "inventory", "fertilizer")
	}
	if strings.Contains(name, "fillfertilizer") {
		add("profile", "inventory", "fertilizer")
	}
	if strings.Contains(name, "refreshwarehousesnapshot") {
		add("inventory")
	}
	if strings.Contains(name, "requestshopdata") || strings.Contains(name, "requestmysterymerchant") {
		add("shop")
	}
	if strings.Contains(name, "requestatlasunlock") {
		add("atlas")
	}
	if strings.Contains(name, "friend") || strings.Contains(name, "visitor") || strings.Contains(name, "block") {
		add("friends", "social")
	}
	if strings.Contains(name, "buy") || strings.Contains(name, "sell") || strings.Contains(name, "claim") ||
		strings.Contains(name, "reward") || strings.Contains(name, "draw") {
		add("profile", "inventory", "fertilizer", "shop")
	}
	if strings.Contains(name, "atlas") {
		add("atlas")
	}
	result := make([]string, 0, len(groups))
	for group := range groups {
		result = append(result, group)
	}
	return result
}
