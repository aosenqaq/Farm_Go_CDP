package automation

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSchedulerPicksHighestPriorityDueTask(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(Settings{
		SchedulerEnabled:  true,
		SchedulerMinGapMs: 10,
		Tasks: []TaskSettings{
			{ID: "friend_steal", Enabled: true, Priority: 70, IntervalSec: 90},
			{ID: "own_base", Enabled: true, Priority: 100, IntervalSec: 60},
		},
		Config: DefaultConfig(),
	})

	task, ok := s.nextDueTask(now)

	if !ok || task.ID != "own_base" {
		t.Fatalf("next task = %#v, %v; want own_base", task, ok)
	}
}

func TestSchedulerSkipsDailyOnceTaskMarkedDoneToday(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	config := DefaultConfig()
	config = MarkDailyOnceTaskDone(config, "svip_daily_gift", now)
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(Settings{
		SchedulerEnabled:  true,
		SchedulerMinGapMs: 10,
		Tasks: []TaskSettings{
			{ID: "svip_daily_gift", Enabled: true, Priority: 120, IntervalSec: 60},
			{ID: "own_base", Enabled: true, Priority: 119, IntervalSec: 60},
		},
		Config: config,
	})

	task, ok := s.nextDueTask(now)

	if !ok || task.ID != "own_base" {
		t.Fatalf("next task = %#v, %v; want own_base because svip_daily_gift is done today", task, ok)
	}
}

func TestSchedulerStateMarksDailyOnceTaskDoneTodayAndNextRun(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	config := DefaultConfig()
	config["autoFarmSvipDailyGiftScheduleMode"] = "daily_time"
	config["autoFarmSvipDailyGiftScheduleTime"] = "09:30"
	config = MarkDailyOnceTaskDone(config, "svip_daily_gift", now)
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(Settings{
		SchedulerEnabled: true,
		Tasks:            []TaskSettings{{ID: "svip_daily_gift", Enabled: true, Priority: 100, IntervalSec: 60}},
		Config:           config,
	})

	state := s.State()
	task := SchedulerTask{}
	for _, item := range state.Scheduler.Tasks {
		if item.ID == "svip_daily_gift" {
			task = item
			break
		}
	}

	if !task.DailyDoneToday {
		t.Fatalf("DailyDoneToday = false, want true: %#v", task)
	}
	if task.NextRunAt != "2026-07-09T09:30:00Z" {
		t.Fatalf("NextRunAt = %q, want next day at configured time", task.NextRunAt)
	}
}

func TestSchedulerSpecifiedRewardUsesNextDailyTimeAfterResult(t *testing.T) {
	for _, resultOK := range []bool{true, false} {
		name := "failure"
		status := StatusFailed
		if resultOK {
			name = "success"
			status = StatusOK
		}
		t.Run(name, func(t *testing.T) {
			now := time.Date(2026, 7, 10, 9, 30, 0, 0, time.UTC)
			s := NewScheduler(SchedulerOptions{
				Now: func() time.Time { return now },
				Runner: func(ctx context.Context, taskID string) ActionResult {
					return ActionResult{OK: resultOK, Status: status, TaskID: taskID, Message: "result"}
				},
			})
			settings := mailRewardSchedulerSettingsForTest("daily_time", "09:30")
			s.Configure(settings)
			s.mu.Lock()
			rt := s.runtime["mail_reward"]
			rt.NextRunAt = now
			s.runtime["mail_reward"] = rt
			s.mu.Unlock()

			s.RunDue(context.Background())

			task := findSchedulerTaskForTest(s.State().Scheduler.Tasks, "mail_reward")
			if task == nil || task.NextRunAt != "2026-07-11T09:30:00Z" {
				t.Fatalf("mail_reward = %#v, want next specified occurrence after result", task)
			}
		})
	}
}

func TestSchedulerAllowsProtocolTaskBesideMainlineTask(t *testing.T) {
	s := NewScheduler(SchedulerOptions{})
	if !s.tryAcquire(taskSpecByID("own_base")) {
		t.Fatal("own_base should acquire resources")
	}
	if !s.tryAcquire(taskSpecByID("reward_claim")) {
		t.Fatal("reward_claim should run beside own_base")
	}
}

func TestSchedulerAllowsFriendStealBesideAnyRunningTask(t *testing.T) {
	for _, runningTaskID := range []string{"own_base", "reward_claim", "friend_help"} {
		t.Run(runningTaskID, func(t *testing.T) {
			s := NewScheduler(SchedulerOptions{})
			if !s.tryAcquire(taskSpecByID(runningTaskID)) {
				t.Fatalf("%s should acquire resources", runningTaskID)
			}
			if !s.tryAcquire(taskSpecByID("friend_steal")) {
				t.Fatalf("friend_steal should run beside %s", runningTaskID)
			}
		})
	}
}

func TestSchedulerBlocksTasksSharingRewardResource(t *testing.T) {
	s := NewScheduler(SchedulerOptions{})
	if !s.tryAcquire(taskSpecByID("reward_claim")) {
		t.Fatal("reward_claim should acquire resources")
	}
	if s.tryAcquire(taskSpecByID("mail_reward")) {
		t.Fatal("mail_reward should wait for reward resource")
	}
}

func TestSchedulerAllowsIndependentProtocolResources(t *testing.T) {
	s := NewScheduler(SchedulerOptions{})
	if !s.tryAcquire(taskSpecByID("reward_claim")) {
		t.Fatal("reward_claim should acquire resources")
	}
	if !s.tryAcquire(taskSpecByID("mystery_shop_auto_buy")) {
		t.Fatal("mystery shop protocol task should run beside reward protocol task")
	}
}

func TestWarehouseAutoSellUsesProtocolResource(t *testing.T) {
	spec := taskSpecByID("auto_warehouse_sell")
	if spec.Lane != LaneProtocol {
		t.Fatalf("lane = %q, want protocol", spec.Lane)
	}
	if len(spec.Resources) != 1 || spec.Resources[0] != ResourceProtocol {
		t.Fatalf("resources = %#v, want protocol", spec.Resources)
	}
}

func TestSchedulerMakesWarehouseAutoSellImmediatelyDueWhenEnabled(t *testing.T) {
	now := time.Date(2026, 7, 11, 9, 0, 0, 0, time.UTC)
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(schedulerSettingsForSingleTask("auto_warehouse_sell", false, 3600))
	s.Configure(schedulerSettingsForSingleTask("auto_warehouse_sell", true, 3600))

	task, ok := s.nextDueTask(now)
	if !ok || task.ID != "auto_warehouse_sell" {
		t.Fatalf("next task = %#v, %v; want auto_warehouse_sell", task, ok)
	}
}

func TestSchedulerDispatchesDueTaskAndRecordsLog(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	var logs []SchedulerLog
	runner := func(ctx context.Context, taskID string) ActionResult {
		return ActionResult{OK: true, Status: StatusOK, TaskID: taskID, Message: "done", ActionCount: 2}
	}
	s := NewScheduler(SchedulerOptions{
		Now:    func() time.Time { return now },
		Runner: runner,
		OnLog:  func(log SchedulerLog) { logs = append(logs, log) },
	})
	s.Configure(Settings{
		SchedulerEnabled:  true,
		SchedulerMinGapMs: 1,
		Tasks:             []TaskSettings{{ID: "own_base", Enabled: true, Priority: 100, IntervalSec: 60}},
		Config:            DefaultConfig(),
	})

	s.RunDue(context.Background())

	state := s.State()
	if state.Scheduler.Tasks[0].LastSuccessAt == "" {
		t.Fatalf("LastSuccessAt not recorded: %#v", state.Scheduler.Tasks[0])
	}
	if len(logs) < 2 || logs[0].Type != "task.start" || logs[len(logs)-1].Type != "task.done" {
		t.Fatalf("logs = %#v, want start and done", logs)
	}
	if got := logs[len(logs)-1].ActionCount; got != 2 {
		t.Fatalf("scheduler action count = %d, want 2", got)
	}
}

func TestSchedulerSafeModeDispatchesOnlyOneDueTask(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name string
		mode RunMode
		want []string
	}{
		{name: "god", mode: RunModeGod, want: []string{"own_base", "reward_claim", "auto_warehouse_sell"}},
		{name: "safe", mode: RunModeSafe, want: []string{"own_base"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var mu sync.Mutex
			started := map[string]bool{}
			s := NewScheduler(SchedulerOptions{
				Now: func() time.Time { return now },
				Runner: func(_ context.Context, taskID string) ActionResult {
					mu.Lock()
					started[taskID] = true
					mu.Unlock()
					return ActionResult{OK: true, Status: StatusOK, TaskID: taskID, Message: "done"}
				},
			})
			settings := schedulerSettingsForSingleTask("own_base", true, 60)
			settings.RunMode = test.mode
			for index := range settings.Tasks {
				switch settings.Tasks[index].ID {
				case "reward_claim":
					settings.Tasks[index].Enabled = true
					settings.Tasks[index].Priority = 98
				case "auto_warehouse_sell":
					settings.Tasks[index].Enabled = true
					settings.Tasks[index].Priority = 64
				}
			}
			settings.Config["autoRewardClaimEnabled"] = true
			settings.Config["autoWarehouseSellEnabled"] = true
			s.Configure(settings)
			s.RunDue(context.Background())

			for _, taskID := range test.want {
				if !started[taskID] {
					t.Errorf("%s was not dispatched: %#v", taskID, started)
				}
			}
			if len(started) != len(test.want) {
				t.Fatalf("dispatched tasks = %#v, want %#v", started, test.want)
			}
		})
	}
}

func TestSchedulerUsesDetailedIntervalConfigForNextRun(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	s := NewScheduler(SchedulerOptions{
		Now: func() time.Time { return now },
		Runner: func(ctx context.Context, taskID string) ActionResult {
			return ActionResult{OK: true, Status: StatusOK, TaskID: taskID, Message: "done"}
		},
	})
	s.Configure(Settings{
		SchedulerEnabled:  true,
		SchedulerMinGapMs: 1,
		Tasks:             []TaskSettings{{ID: "friend_steal", Enabled: true, Priority: 70, IntervalSec: 90}},
		Config: map[string]any{
			"autoFarmFriendEnabled":          true,
			"autoFarmFriendStealIntervalSec": 5,
		},
	})

	s.RunDue(context.Background())

	task := SchedulerTask{}
	for _, item := range s.State().Scheduler.Tasks {
		if item.ID == "friend_steal" {
			task = item
			break
		}
	}
	if task.IntervalSec != 5 || task.NextRunAt != "2026-07-08T12:00:05Z" {
		t.Fatalf("friend_steal interval/next = %d/%q, want 5/2026-07-08T12:00:05Z", task.IntervalSec, task.NextRunAt)
	}
}

func TestSchedulerConfigureSkipsUnchangedSettings(t *testing.T) {
	nowCalls := 0
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	s := NewScheduler(SchedulerOptions{Now: func() time.Time {
		nowCalls++
		return now
	}})
	settings := DefaultSettings()
	before := nowCalls

	s.Configure(settings)

	if nowCalls != before {
		t.Fatalf("unchanged Configure called clock %d extra times", nowCalls-before)
	}
}

func TestSchedulerConfigureReschedulesChangedInterval(t *testing.T) {
	now := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
	s := NewScheduler(SchedulerOptions{
		Now: func() time.Time { return now },
		Runner: func(ctx context.Context, taskID string) ActionResult {
			return ActionResult{OK: true, Status: StatusOK, TaskID: taskID, Message: "done"}
		},
	})
	settings := schedulerSettingsForSingleTask("own_collect", true, 30)
	s.Configure(settings)
	s.RunDue(context.Background())

	now = now.Add(5 * time.Second)
	settings = schedulerSettingsForSingleTask("own_collect", true, 120)
	s.Configure(settings)

	task := findSchedulerTaskForTest(s.State().Scheduler.Tasks, "own_collect")
	want := now.Add(120 * time.Second).Format(time.RFC3339)
	if task == nil || task.IntervalSec != 120 || task.NextRunAt != want {
		t.Fatalf("own_collect after interval change = %#v, want interval 120 next %s", task, want)
	}
}

func TestSchedulerConfigureMakesNewlyEnabledTaskDue(t *testing.T) {
	now := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(schedulerSettingsForSingleTask("own_collect", false, 30))
	s.mu.Lock()
	runtime := s.runtime["own_collect"]
	runtime.NextRunAt = now.Add(time.Hour)
	s.runtime["own_collect"] = runtime
	s.mu.Unlock()

	now = now.Add(5 * time.Second)
	s.Configure(schedulerSettingsForSingleTask("own_collect", true, 30))

	task, ok := s.nextDueTask(now)
	if !ok || task.ID != "own_collect" {
		t.Fatalf("newly enabled task = %#v, %v; want own_collect due immediately", task, ok)
	}
}

func TestSchedulerRunningTaskUsesReconfiguredIntervalAfterFinish(t *testing.T) {
	now := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
	s := NewScheduler(SchedulerOptions{
		Now: func() time.Time { return now },
		Runner: func(ctx context.Context, taskID string) ActionResult {
			return ActionResult{OK: true, Status: StatusOK, TaskID: taskID, Message: "done"}
		},
	})
	s.Configure(schedulerSettingsForSingleTask("own_collect", true, 30))
	startedTask := findSchedulerTaskForTest(s.State().Scheduler.Tasks, "own_collect")
	if startedTask == nil {
		t.Fatal("own_collect task not found")
	}
	s.mu.Lock()
	runtime := s.runtime["own_collect"]
	runtime.Running = true
	s.runtime["own_collect"] = runtime
	s.mu.Unlock()

	now = now.Add(5 * time.Second)
	s.Configure(schedulerSettingsForSingleTask("own_collect", true, 120))
	s.runTask(context.Background(), *startedTask)

	task := findSchedulerTaskForTest(s.State().Scheduler.Tasks, "own_collect")
	want := now.Add(120 * time.Second).Format(time.RFC3339)
	if task == nil || task.IntervalSec != 120 || task.NextRunAt != want {
		t.Fatalf("running own_collect after interval change = %#v, want interval 120 next %s", task, want)
	}
}

func schedulerSettingsForSingleTask(taskID string, enabled bool, intervalSec int) Settings {
	settings := DefaultSettings()
	for index := range settings.Tasks {
		settings.Tasks[index].Enabled = false
		if settings.Tasks[index].ID == taskID {
			settings.Tasks[index].Enabled = enabled
			settings.Tasks[index].IntervalSec = intervalSec
		}
	}
	settings.Config = DefaultConfig()
	for _, key := range schedulerTaskConfigKeys {
		settings.Config[key] = false
	}
	if key := schedulerTaskConfigKeys[taskID]; key != "" {
		settings.Config[key] = enabled
	}
	if key := schedulerTaskIntervalConfigKeys[taskID]; key != "" {
		settings.Config[key] = intervalSec
	}
	return settings
}

func TestSchedulerConfigureAlignsMailRewardToSpecifiedTime(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 7, 10, 8, 15, 0, 0, location)
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})

	s.Configure(mailRewardSchedulerSettingsForTest("daily_time", "09:30"))

	task := findSchedulerTaskForTest(s.State().Scheduler.Tasks, "mail_reward")
	if task == nil || task.NextRunAt != "2026-07-10T09:30:00+08:00" {
		t.Fatalf("mail_reward = %#v, want next run at today's specified time", task)
	}
	if due, ok := s.nextDueTask(now); ok {
		t.Fatalf("nextDueTask() = %#v, true; want no due mail task", due)
	}
}

func TestSchedulerConfigureUpdatesMailRewardSpecifiedTime(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 7, 10, 8, 15, 0, 0, location)
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(mailRewardSchedulerSettingsForTest("daily_time", "09:30"))

	s.Configure(mailRewardSchedulerSettingsForTest("daily_time", "10:45"))

	task := findSchedulerTaskForTest(s.State().Scheduler.Tasks, "mail_reward")
	if task == nil || task.NextRunAt != "2026-07-10T10:45:00+08:00" {
		t.Fatalf("mail_reward = %#v, want updated next run at 10:45", task)
	}
}

func TestSchedulerConfigureSwitchesMailRewardFromSpecifiedTimeToInterval(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 7, 10, 8, 15, 0, 0, location)
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(mailRewardSchedulerSettingsForTest("daily_time", "09:30"))

	s.Configure(mailRewardSchedulerSettingsForTest("interval", "09:30"))

	task := findSchedulerTaskForTest(s.State().Scheduler.Tasks, "mail_reward")
	if task == nil || task.NextRunAt != "2026-07-10T08:15:00+08:00" {
		t.Fatalf("mail_reward = %#v, want interval mode reset to immediate first run", task)
	}
	due, ok := s.nextDueTask(now)
	if !ok || due.ID != "mail_reward" {
		t.Fatalf("nextDueTask() = %#v, %v; want immediately eligible mail_reward", due, ok)
	}
}

func mailRewardSchedulerSettingsForTest(mode, scheduleTime string) Settings {
	settings := DefaultSettings()
	for index := range settings.Tasks {
		settings.Tasks[index].Enabled = false
	}
	config := DefaultConfig()
	for _, key := range schedulerTaskConfigKeys {
		config[key] = false
	}
	config["autoFarmMailRewardEnabled"] = true
	config["autoFarmMailRewardScheduleMode"] = mode
	config["autoFarmMailRewardScheduleTime"] = scheduleTime
	settings.Config = config
	return settings
}

func TestSchedulerNextWakeDelayTracksEarliestTaskAndIdleBound(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(Settings{
		SchedulerEnabled: true,
		Tasks:            []TaskSettings{{ID: "own_base", Enabled: true, Priority: 100, IntervalSec: 60}},
		Config:           DefaultConfig(),
	})
	s.mu.Lock()
	for taskID, rt := range s.runtime {
		rt.NextRunAt = now.Add(20 * time.Second)
		s.runtime[taskID] = rt
	}
	rt := s.runtime["own_base"]
	rt.NextRunAt = now.Add(7 * time.Second)
	s.runtime["own_base"] = rt
	s.mu.Unlock()

	if delay := s.nextWakeDelay(now); delay != 7*time.Second {
		t.Fatalf("nextWakeDelay=%s, want 7s", delay)
	}

	s.Configure(Settings{SchedulerEnabled: false, Config: DefaultConfig()})
	if delay := s.nextWakeDelay(now); delay != 30*time.Second {
		t.Fatalf("idle nextWakeDelay=%s, want 30s", delay)
	}
}

func TestSchedulerSkipsFriendMischiefMarkedDoneToday(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	config := MarkFriendMischiefDailyDone(DefaultConfig(), now)
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(Settings{
		SchedulerEnabled: true,
		Tasks: []TaskSettings{
			{ID: "friend_mischief", Enabled: true, Priority: 120, IntervalSec: 60},
			{ID: "own_base", Enabled: true, Priority: 100, IntervalSec: 60},
		},
		Config: config,
	})

	task, ok := s.nextDueTask(now)
	if !ok || task.ID != "own_base" {
		t.Fatalf("next task=%#v ok=%v, want friend mischief skipped for today", task, ok)
	}
}

func TestSchedulerStateMarksFriendMischiefDoneToday(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	config := MarkFriendMischiefDailyDone(DefaultConfig(), now)
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(Settings{
		SchedulerEnabled: true,
		Tasks:            []TaskSettings{{ID: "friend_mischief", Enabled: true, Priority: 100, IntervalSec: 60}},
		Config:           config,
	})

	state := s.State()
	var task SchedulerTask
	for _, item := range state.Scheduler.Tasks {
		if item.ID == "friend_mischief" {
			task = item
			break
		}
	}
	if !task.DailyDoneToday {
		t.Fatalf("DailyDoneToday=false, want friend mischief completion badge: %#v", task)
	}
	if task.NextRunAt != "2026-07-11T00:00:00Z" {
		t.Fatalf("NextRunAt=%q, want next day boundary", task.NextRunAt)
	}
}

func TestSchedulerQuietHoursSkipsBlockedFriendTaskButRunsOtherDueTask(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 7, 21, 23, 30, 0, 0, location)
	settings := schedulerSettingsForSingleTask("own_base", true, 60)
	for index := range settings.Tasks {
		if settings.Tasks[index].ID == "friend_steal" {
			settings.Tasks[index].Enabled = true
			settings.Tasks[index].Priority = 200
			settings.Tasks[index].IntervalSec = 90
		}
	}
	for key, value := range quietHoursConfigForTest("sleep", "23:00", "07:00", []string{"steal", "help"}) {
		settings.Config[key] = value
	}
	settings.Config["autoFarmFriendEnabled"] = true
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(settings)

	task, ok := s.nextDueTask(now)
	if !ok || task.ID != "own_base" {
		t.Fatalf("next task = %#v, %v; want own_base", task, ok)
	}
}

func TestSchedulerQuietHoursExposesEffectiveNextRunAndWakeDelay(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 7, 21, 22, 59, 50, 0, location)
	settings := schedulerSettingsForSingleTask("friend_help", true, 90)
	for key, value := range quietHoursConfigForTest("work", "23:00", "07:00", []string{"help"}) {
		settings.Config[key] = value
	}
	settings.Config["autoFarmFriendHelpEnabled"] = true
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(settings)

	if _, ok := s.nextDueTask(now); ok {
		t.Fatal("friend_help should be blocked before work window")
	}
	if delay := s.nextWakeDelay(now); delay != 10*time.Second {
		t.Fatalf("nextWakeDelay = %s, want 10s", delay)
	}
	task := findSchedulerTaskForTest(s.State().Scheduler.Tasks, "friend_help")
	if task == nil || task.NextRunAt != "2026-07-21T23:00:00+08:00" {
		t.Fatalf("friend_help state = %#v, want effective 23:00 next run", task)
	}
}

func TestSchedulerQuietHoursMakesOverdueTaskEligibleAtAllowedBoundary(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 7, 21, 22, 59, 50, 0, location)
	settings := schedulerSettingsForSingleTask("friend_steal", true, 90)
	for key, value := range quietHoursConfigForTest("work", "23:00", "07:00", []string{"steal"}) {
		settings.Config[key] = value
	}
	settings.Config["autoFarmFriendEnabled"] = true
	s := NewScheduler(SchedulerOptions{Now: func() time.Time { return now }})
	s.Configure(settings)

	now = time.Date(2026, 7, 21, 23, 0, 0, 0, location)
	task, ok := s.nextDueTask(now)
	if !ok || task.ID != "friend_steal" {
		t.Fatalf("boundary task = %#v, %v; want friend_steal", task, ok)
	}
}
