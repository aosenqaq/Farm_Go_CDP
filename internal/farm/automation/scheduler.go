package automation

import (
	"context"
	"reflect"
	"sort"
	"sync"
	"time"
)

type Lane string

const (
	LaneMainline Lane = "mainline"
	LaneProtocol Lane = "protocol"
)

type Resource string

const (
	ResourceScene    Resource = "scene"
	ResourceProtocol Resource = "protocol"
	ResourceReward   Resource = "reward"
	ResourceFriend   Resource = "friend"
	ResourceShop     Resource = "shop"
)

type TaskSpec struct {
	ID        string
	Label     string
	Lane      Lane
	Resources []Resource
}

type SchedulerLog struct {
	Type        string
	TaskID      string
	Trigger     string
	OK          bool
	Status      ActionStatus
	Message     string
	StartedAt   string
	FinishedAt  string
	DurationMs  int64
	ActionCount int
}

type SchedulerOptions struct {
	Now    func() time.Time
	Runner func(context.Context, string) ActionResult
	OnLog  func(SchedulerLog)
}

type Scheduler struct {
	mu         sync.Mutex
	settings   Settings
	state      State
	runtime    map[string]schedulerTaskRuntime
	acquired   map[Resource]int
	now        func() time.Time
	runner     func(context.Context, string) ActionResult
	onLog      func(SchedulerLog)
	cancel     context.CancelFunc
	started    bool
	minTick    time.Duration
	waitGroup  sync.WaitGroup
	wake       chan struct{}
	configured bool
}

type schedulerTaskRuntime struct {
	LastStartedAt     string
	LastFinishedAt    string
	LastSuccessAt     string
	LastError         string
	LastResultSummary string
	NextRunAt         time.Time
	Running           bool
	FailureCount      int
}

func NewScheduler(options SchedulerOptions) *Scheduler {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	runner := options.Runner
	if runner == nil {
		runner = func(context.Context, string) ActionResult {
			return ActionResult{OK: false, Status: StatusNotMigrated, Message: "scheduler runner is not configured"}
		}
	}
	s := &Scheduler{
		settings: DefaultSettings(),
		state:    DefaultState(),
		runtime:  map[string]schedulerTaskRuntime{},
		acquired: map[Resource]int{},
		now:      now,
		runner:   runner,
		onLog:    options.OnLog,
		minTick:  500 * time.Millisecond,
		wake:     make(chan struct{}, 1),
	}
	s.Configure(s.settings)
	return s
}

func (s *Scheduler) Configure(settings Settings) {
	merged := mergeSettingsWithDefaults(settings)
	s.mu.Lock()
	if s.configured && reflect.DeepEqual(s.settings, merged) {
		s.mu.Unlock()
		return
	}

	previousSettings := s.settings
	previousTasks := make(map[string]SchedulerTask, len(s.state.Scheduler.Tasks))
	for _, task := range s.state.Scheduler.Tasks {
		previousTasks[task.ID] = task
	}
	s.settings = merged
	s.state = StateFromSettings(s.settings)
	now := s.now()
	for _, task := range s.state.Scheduler.Tasks {
		rt := s.runtime[task.ID]
		previousTask := previousTasks[task.ID]
		if nextRunAt, ok := nextRewardSpecifiedRun(s.settings.Config, task.ID, now); ok {
			rt.NextRunAt = nextRunAt
		} else if _, wasSpecified := nextRewardSpecifiedRun(previousSettings.Config, task.ID, now); rt.NextRunAt.IsZero() || wasSpecified {
			rt.NextRunAt = now
		} else if task.Enabled && !previousTask.Enabled {
			rt.NextRunAt = now
		} else if task.Enabled && previousTask.IntervalSec != task.IntervalSec && !rt.Running {
			rt.NextRunAt = now.Add(time.Duration(task.IntervalSec) * time.Second)
		}
		s.runtime[task.ID] = rt
	}
	s.configured = true
	s.applyRuntimeToStateLocked()
	s.mu.Unlock()
	s.signalWake()
}

func (s *Scheduler) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := StateFromSettings(s.settings)
	s.state = state
	s.applyRuntimeToStateLocked()
	return s.state
}

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.started = true
	s.applyRuntimeToStateLocked()
	s.mu.Unlock()

	go s.loop(runCtx)
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.started = false
	s.applyRuntimeToStateLocked()
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Scheduler) Wait() {
	s.waitGroup.Wait()
}

func (s *Scheduler) RunDue(ctx context.Context) {
	var tasks []SchedulerTask
	for {
		now := s.now()
		s.mu.Lock()
		task, ok := s.nextDueTaskLocked(now)
		if !ok {
			s.mu.Unlock()
			break
		}
		spec := taskSpecByID(task.ID)
		if !s.tryAcquireLocked(spec) {
			rt := s.runtime[task.ID]
			rt.NextRunAt = now.Add(s.minTick)
			s.runtime[task.ID] = rt
			s.mu.Unlock()
			continue
		}
		rt := s.runtime[task.ID]
		rt.Running = true
		rt.LastStartedAt = now.Format(time.RFC3339)
		s.runtime[task.ID] = rt
		s.applyRuntimeToStateLocked()
		tasks = append(tasks, task)
		safeMode := s.settings.RunMode.IsSafe()
		s.mu.Unlock()
		if safeMode {
			break
		}
	}

	var pass sync.WaitGroup
	for _, task := range tasks {
		task := task
		pass.Add(1)
		s.waitGroup.Add(1)
		go func() {
			defer pass.Done()
			defer s.waitGroup.Done()
			s.runTask(ctx, task)
		}()
	}
	pass.Wait()
}

func (s *Scheduler) nextDueTask(now time.Time) (SchedulerTask, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nextDueTaskLocked(now)
}

func (s *Scheduler) nextDueTaskLocked(now time.Time) (SchedulerTask, bool) {
	if !s.state.Scheduler.Enabled {
		return SchedulerTask{}, false
	}
	candidates := make([]SchedulerTask, 0, len(s.state.Scheduler.Tasks))
	for _, task := range s.state.Scheduler.Tasks {
		rt := s.runtime[task.ID]
		if !task.Enabled || rt.Running {
			continue
		}
		if decision := friendQuietHoursDecision(s.settings.Config, task.ID, now); !decision.Allowed {
			continue
		}
		if automationTaskDoneToday(s.settings.Config, task.ID, now) {
			continue
		}
		if rt.NextRunAt.IsZero() || !rt.NextRunAt.After(now) {
			candidates = append(candidates, task)
		}
	}
	if len(candidates) == 0 {
		return SchedulerTask{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left := s.taskScore(candidates[i], now)
		right := s.taskScore(candidates[j], now)
		if left != right {
			return left > right
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates[0], true
}

func (s *Scheduler) taskScore(task SchedulerTask, now time.Time) int64 {
	score := int64(task.Priority * 1000)
	rt := s.runtime[task.ID]
	if !rt.NextRunAt.IsZero() && now.After(rt.NextRunAt) {
		interval := task.IntervalSec
		if interval <= 0 {
			interval = 1
		}
		score += int64(now.Sub(rt.NextRunAt).Seconds()/float64(interval)) * 100
	}
	score -= int64(rt.FailureCount * 50)
	return score
}

func (s *Scheduler) tryAcquire(spec TaskSpec) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tryAcquireLocked(spec)
}

func (s *Scheduler) tryAcquireLocked(spec TaskSpec) bool {
	for _, resource := range spec.Resources {
		if s.acquired[resource] >= resourceCapacity(resource) {
			return false
		}
	}
	for _, resource := range spec.Resources {
		s.acquired[resource]++
	}
	return true
}

func resourceCapacity(resource Resource) int {
	if resource == ResourceProtocol {
		return 2
	}
	return 1
}

func (s *Scheduler) release(spec TaskSpec) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, resource := range spec.Resources {
		if s.acquired[resource] <= 1 {
			delete(s.acquired, resource)
			continue
		}
		s.acquired[resource]--
	}
}

func (s *Scheduler) runTask(ctx context.Context, task SchedulerTask) {
	started := s.now()
	startedText := started.Format(time.RFC3339)
	s.emit(SchedulerLog{
		Type:      "task.start",
		TaskID:    task.ID,
		Trigger:   "auto",
		StartedAt: startedText,
	})

	result := s.runner(ctx, task.ID)
	finished := s.now()
	if result.TaskID == "" {
		result.TaskID = task.ID
	}
	eventType := "task.done"
	if !result.OK {
		eventType = "task.failed"
	}
	log := SchedulerLog{
		Type:        eventType,
		TaskID:      result.TaskID,
		Trigger:     "auto",
		OK:          result.OK,
		Status:      result.Status,
		Message:     result.Message,
		StartedAt:   startedText,
		FinishedAt:  finished.Format(time.RFC3339),
		DurationMs:  finished.Sub(started).Milliseconds(),
		ActionCount: result.ActionCount,
	}

	s.mu.Lock()
	rt := s.runtime[task.ID]
	configuredTask := task
	for _, currentTask := range s.state.Scheduler.Tasks {
		if currentTask.ID == task.ID {
			configuredTask = currentTask
			break
		}
	}
	rt.Running = false
	rt.LastFinishedAt = log.FinishedAt
	rt.LastResultSummary = result.Message
	if result.OK {
		rt.LastSuccessAt = log.FinishedAt
		rt.LastError = ""
		rt.FailureCount = 0
		rt.NextRunAt = finished.Add(time.Duration(configuredTask.IntervalSec) * time.Second)
	} else {
		rt.LastError = result.Message
		rt.FailureCount++
		rt.NextRunAt = finished.Add(s.failureBackoff(configuredTask, rt.FailureCount))
	}
	if nextRunAt, ok := nextRewardSpecifiedRun(s.settings.Config, task.ID, finished); ok {
		rt.NextRunAt = nextRunAt
	}
	s.runtime[task.ID] = rt
	s.applyRuntimeToStateLocked()
	s.mu.Unlock()

	s.release(taskSpecByID(task.ID))
	s.signalWake()
	s.emit(log)
}

func (s *Scheduler) failureBackoff(task SchedulerTask, failures int) time.Duration {
	if failures <= 0 {
		failures = 1
	}
	backoff := time.Duration(failures*10) * time.Second
	interval := time.Duration(task.IntervalSec) * time.Second
	if interval > 0 && backoff > interval {
		return interval
	}
	return backoff
}

func (s *Scheduler) applyRuntimeToStateLocked() {
	runningTaskID := ""
	now := s.now()
	for index := range s.state.Scheduler.Tasks {
		task := &s.state.Scheduler.Tasks[index]
		rt := s.runtime[task.ID]
		task.LastStartedAt = rt.LastStartedAt
		task.LastFinishedAt = rt.LastFinishedAt
		task.LastSuccessAt = rt.LastSuccessAt
		task.LastError = rt.LastError
		task.LastResultSummary = rt.LastResultSummary
		task.DailyDoneToday = false
		task.NextRunAt = ""
		if !rt.NextRunAt.IsZero() {
			task.NextRunAt = rt.NextRunAt.Format(time.RFC3339)
		}
		applyDailyOnceTaskState(task, s.settings.Config, now)
		displayNextRunAt := time.Time{}
		if task.NextRunAt != "" {
			displayNextRunAt, _ = time.Parse(time.RFC3339, task.NextRunAt)
		}
		decision := friendQuietHoursDecision(s.settings.Config, task.ID, now)
		if !decision.Allowed {
			if decision.NextAllowedAt.IsZero() {
				displayNextRunAt = time.Time{}
			} else if displayNextRunAt.IsZero() || decision.NextAllowedAt.After(displayNextRunAt) {
				displayNextRunAt = decision.NextAllowedAt
			}
		}
		task.NextRunAt = ""
		if !displayNextRunAt.IsZero() {
			task.NextRunAt = displayNextRunAt.Format(time.RFC3339)
		}
		if rt.Running && runningTaskID == "" {
			runningTaskID = task.ID
		}
	}
	s.state.Running = s.started
	s.state.Scheduler.RunningTaskID = runningTaskID
}

func (s *Scheduler) emit(log SchedulerLog) {
	if s.onLog != nil {
		s.onLog(log)
	}
}

func (s *Scheduler) loop(ctx context.Context) {
	for {
		s.RunDue(ctx)
		delay := s.nextWakeDelay(s.now())
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			stopAndDrainTimer(timer)
			return
		case <-s.wake:
			stopAndDrainTimer(timer)
		case <-timer.C:
		}
	}
}

func stopAndDrainTimer(timer *time.Timer) {
	if timer == nil || timer.Stop() {
		return
	}
	select {
	case <-timer.C:
	default:
	}
}

func (s *Scheduler) signalWake() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Scheduler) nextWakeDelay(now time.Time) time.Duration {
	const idleWakeDelay = 30 * time.Second
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.Scheduler.Enabled {
		return idleWakeDelay
	}
	earliest := time.Time{}
	for _, task := range s.state.Scheduler.Tasks {
		rt := s.runtime[task.ID]
		if !task.Enabled || rt.Running || automationTaskDoneToday(s.settings.Config, task.ID, now) {
			continue
		}
		decision := friendQuietHoursDecision(s.settings.Config, task.ID, now)
		if !decision.Allowed && decision.NextAllowedAt.IsZero() {
			continue
		}
		next := rt.NextRunAt
		if !decision.Allowed && (next.IsZero() || decision.NextAllowedAt.After(next)) {
			next = decision.NextAllowedAt
		}
		if next.IsZero() || !next.After(now) {
			return s.minTick
		}
		if earliest.IsZero() || next.Before(earliest) {
			earliest = next
		}
	}
	if earliest.IsZero() {
		return idleWakeDelay
	}
	delay := earliest.Sub(now)
	if delay < s.minTick {
		return s.minTick
	}
	if delay > idleWakeDelay {
		return idleWakeDelay
	}
	return delay
}

func taskSpecByID(taskID string) TaskSpec {
	for _, spec := range schedulerTaskSpecs {
		if spec.ID == taskID {
			return spec
		}
	}
	return TaskSpec{ID: taskID, Lane: LaneMainline, Resources: []Resource{ResourceScene}}
}

var schedulerTaskSpecs = []TaskSpec{
	{ID: "own_base", Label: "一键务农", Lane: LaneMainline, Resources: []Resource{ResourceScene}},
	{ID: "land_upgrade", Label: "土地自动升级", Lane: LaneMainline, Resources: []Resource{ResourceScene}},
	{ID: "own_collect", Label: "自动收获", Lane: LaneMainline, Resources: []Resource{ResourceScene}},
	{ID: "own_plant", Label: "自动种植", Lane: LaneMainline, Resources: []Resource{ResourceScene}},
	{ID: "fertilizer_fill", Label: "自动填充化肥", Lane: LaneMainline, Resources: []Resource{ResourceScene}},
	{ID: "own_fertilizer", Label: "自动施肥", Lane: LaneMainline, Resources: []Resource{ResourceScene}},
	{ID: "friend_steal", Label: "好友偷菜", Lane: LaneProtocol, Resources: nil},
	{ID: "friend_help", Label: "好友帮忙", Lane: LaneMainline, Resources: []Resource{ResourceScene, ResourceFriend}},
	{ID: "friend_mischief", Label: "好友捣乱", Lane: LaneMainline, Resources: []Resource{ResourceScene, ResourceFriend}},
	{ID: "auto_warehouse_sell", Label: "仓库自动出售", Lane: LaneProtocol, Resources: []Resource{ResourceProtocol}},
	{ID: "reward_claim", Label: "自动领取任务奖励", Lane: LaneProtocol, Resources: []Resource{ResourceProtocol, ResourceReward}},
	{ID: "svip_daily_gift", Label: "SVIP每日礼包", Lane: LaneProtocol, Resources: []Resource{ResourceProtocol, ResourceReward}},
	{ID: "monthly_card_reward", Label: "月卡奖励", Lane: LaneProtocol, Resources: []Resource{ResourceProtocol, ResourceReward}},
	{ID: "mall_daily_fertilizer", Label: "商城每日肥料", Lane: LaneProtocol, Resources: []Resource{ResourceProtocol, ResourceReward}},
	{ID: "share_reward", Label: "自动领取分享奖励", Lane: LaneProtocol, Resources: []Resource{ResourceProtocol, ResourceReward}},
	{ID: "mail_reward", Label: "自动领取邮件奖励", Lane: LaneProtocol, Resources: []Resource{ResourceProtocol, ResourceReward}},
	{ID: "qian_xing_travel_reward", Label: "千星游记奖励领取", Lane: LaneProtocol, Resources: []Resource{ResourceProtocol, ResourceReward}},
	{ID: "xing_su_auto_light_up", Label: "自动点亮星宿", Lane: LaneProtocol, Resources: []Resource{ResourceProtocol, ResourceReward}},
	{ID: "limited_seed_draw", Label: "荷风游记抽奖", Lane: LaneProtocol, Resources: []Resource{ResourceProtocol, ResourceReward}},
	{ID: "mystery_shop_auto_buy", Label: "神秘商店自动购买", Lane: LaneProtocol, Resources: []Resource{ResourceProtocol, ResourceShop}},
	{ID: "mystery_shop_read", Label: "读取神秘商店", Lane: LaneProtocol, Resources: []Resource{ResourceProtocol, ResourceShop}},
}
