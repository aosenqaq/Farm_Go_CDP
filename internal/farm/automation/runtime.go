package automation

import (
	"Farm_Go/internal/farm/social"
	"context"
	"math/rand"
	"time"
)

type RuntimeCaller interface {
	Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error)
}

type RuntimeFacade struct {
	caller                          RuntimeCaller
	config                          map[string]any
	stealRecordWriter               StealRecordWriter
	dogGuardStateReader             DogGuardStateReader
	friendRuleReader                FriendRuleReader
	mysteryShopPurchaseRecordWriter MysteryShopPurchaseRecordWriter
	accountKey                      string
	runMode                         RunMode
	wait                            func(context.Context, time.Duration) error
	intn                            func(int) int
}

type StealRecordWriter interface {
	SaveStealRecords(ctx context.Context, accountKey string, records []social.StealRecord) error
}

type StealRecordReader interface {
	ListStealRecords(ctx context.Context, accountKey string, dateKeys []string) ([]social.StealRecord, error)
}

type DogGuardStateReader interface {
	LoadDogGuardState(ctx context.Context, accountKey string) (social.DogGuardState, error)
}

type FriendRuleReader interface {
	LoadFriendRules(ctx context.Context, accountKey string) (social.FriendRules, error)
}

type MysteryShopPurchaseRecord struct {
	ID           string
	OccurredAt   string
	GoodsID      int
	ItemID       int
	ItemName     string
	Count        int
	UnitPrice    int
	CurrencyID   int
	CurrencyName string
	Discount     int
	Payload      map[string]any
}

type MysteryShopPurchaseRecordWriter interface {
	SaveMysteryShopPurchaseRecord(ctx context.Context, accountKey string, record MysteryShopPurchaseRecord) error
}

type RuntimeSocialStore interface {
	StealRecordWriter
	DogGuardStateReader
	FriendRuleReader
}

func NewRuntimeFacade(caller RuntimeCaller) RuntimeFacade {
	return newRuntimeFacade(caller, nil)
}

func NewRuntimeFacadeWithConfig(caller RuntimeCaller, config map[string]any) RuntimeFacade {
	return newRuntimeFacade(caller, config)
}

func NewRuntimeFacadeWithConfigAndStealRecorder(caller RuntimeCaller, config map[string]any, writer StealRecordWriter, accountKey string) RuntimeFacade {
	facade := newRuntimeFacade(caller, config)
	facade.stealRecordWriter = writer
	facade.accountKey = accountKey
	return facade
}

func NewRuntimeFacadeWithConfigAndDogGuardReader(caller RuntimeCaller, config map[string]any, reader DogGuardStateReader, accountKey string) RuntimeFacade {
	facade := newRuntimeFacade(caller, config)
	facade.dogGuardStateReader = reader
	facade.accountKey = accountKey
	return facade
}

func NewRuntimeFacadeWithConfigAndSocialStore(caller RuntimeCaller, config map[string]any, store RuntimeSocialStore, accountKey string) RuntimeFacade {
	facade := newRuntimeFacade(caller, config)
	facade.stealRecordWriter = store
	facade.dogGuardStateReader = store
	facade.friendRuleReader = store
	facade.accountKey = accountKey
	return facade
}

func newRuntimeFacade(caller RuntimeCaller, config map[string]any) RuntimeFacade {
	return RuntimeFacade{
		caller:  caller,
		config:  config,
		runMode: RunModeGod,
		wait:    sleepWithContext,
		intn:    rand.Intn,
	}
}

func (r RuntimeFacade) WithMysteryShopPurchaseRecordWriter(writer MysteryShopPurchaseRecordWriter, accountKey string) RuntimeFacade {
	r.mysteryShopPurchaseRecordWriter = writer
	r.accountKey = accountKey
	return r
}

func (r RuntimeFacade) WithRunMode(mode RunMode) RuntimeFacade {
	r.runMode = NormalizeRunMode(mode)
	return r
}

func (r RuntimeFacade) WithExecutionTiming(wait func(context.Context, time.Duration) error, intn func(int) int) RuntimeFacade {
	if wait != nil {
		r.wait = wait
	}
	if intn != nil {
		r.intn = intn
	}
	return r
}

func (r RuntimeFacade) loadFriendRules(ctx context.Context) (social.FriendRules, error) {
	if r.friendRuleReader == nil {
		return social.NormalizeFriendRules(social.FriendRules{}), nil
	}
	accountKey := r.accountKey
	if accountKey == "" {
		accountKey = "default"
	}
	rules, err := r.friendRuleReader.LoadFriendRules(ctx, accountKey)
	if err != nil {
		return social.FriendRules{}, err
	}
	return social.NormalizeFriendRules(rules), nil
}

func (r RuntimeFacade) RunTask(ctx context.Context, taskID string) ActionResult {
	switch taskID {
	case "own_base":
		return r.runOwnBase(ctx)
	case "own_collect":
		return r.runOwnCollect(ctx)
	case "own_plant":
		return r.runOwnPlant(ctx)
	case "own_fertilizer":
		return r.runOwnFertilizer(ctx)
	case "land_upgrade":
		return r.runLandUpgrade(ctx)
	case "mystery_shop_read":
		return r.runMysteryShopRead(ctx)
	case "mystery_shop_auto_buy":
		return r.runMysteryShopAutoBuy(ctx)
	case "friend_steal":
		return r.runFriendSteal(ctx)
	case "friend_help":
		return r.runFriendHelp(ctx)
	case "friend_mischief":
		return r.runFriendMischief(ctx)
	}
	if result, ok := r.runRewardTask(ctx, taskID); ok {
		return result
	}
	return RunTask(taskID)
}

type directRuntimeTaskSpec struct {
	taskID       string
	method       string
	args         []any
	timeout      time.Duration
	notReadyText string
	errorPrefix  string
	failedPrefix string
	successText  string
}

func (r RuntimeFacade) runDirectRuntimeTask(ctx context.Context, spec directRuntimeTaskSpec) ActionResult {
	if r.caller == nil {
		return ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  spec.taskID,
			Message: spec.notReadyText,
		}
	}
	value, err := r.caller.Call(ctx, spec.method, spec.args, spec.timeout)
	if err != nil {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  spec.taskID,
			Message: spec.errorPrefix + err.Error(),
		}
	}
	if failed, reason := runtimeResultFailed(value); failed {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  spec.taskID,
			Message: spec.failedPrefix + reason,
		}
	}
	return ActionResult{
		OK:      true,
		Status:  StatusOK,
		TaskID:  spec.taskID,
		Message: runtimeSuccessMessage(value, spec.successText),
	}
}
