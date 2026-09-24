package automation

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestRuntimeFacadeFertilizerFillCallsRuntime(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.autoFillFertilizerBuckets": map[string]any{"ok": true, "totalFilledCount": float64(6)},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "fertilizer_fill")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("fertilizer_fill should report OK, got %#v", result)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("runtime call count = %d, want 1", len(caller.calls))
	}
	if caller.calls[0].method != "gameCtl.autoFillFertilizerBuckets" {
		t.Fatalf("method = %q, want autoFillFertilizerBuckets", caller.calls[0].method)
	}
	wantArgs := []any{map[string]any{
		"fillNormal":  true,
		"fillOrganic": true,
		"closeAfter":  true,
		"silent":      true,
		"source":      "farm_go_auto_fertilizer_fill",
	}}
	if !reflect.DeepEqual(caller.calls[0].args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", caller.calls[0].args, wantArgs)
	}
}

func TestRuntimeFacadeFertilizerFillReportsSkip(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.autoFillFertilizerBuckets": map[string]any{"ok": true, "skipped": true, "reason": "no_entries"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "fertilizer_fill")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("fertilizer_fill skipped runtime result should be OK, got %#v", result)
	}
}

func TestRuntimeFacadeFertilizerFillHandlesNilRuntimeCaller(t *testing.T) {
	facade := NewRuntimeFacade(nil)

	result := facade.RunTask(context.Background(), "fertilizer_fill")

	if result.OK || result.Status != StatusRuntimeNotReady {
		t.Fatalf("nil runtime should be runtime_not_ready, got %#v", result)
	}
}

func TestRuntimeFacadeFertilizerFillReportsRuntimeError(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.autoFillFertilizerBuckets": errors.New("netWebSocket.sendMsg not found"),
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "fertilizer_fill")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("runtime error should fail, got %#v", result)
	}
}

func TestRuntimeFacadeFertilizerFillReportsRuntimeNotOKResult(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.autoFillFertilizerBuckets": map[string]any{"ok": false, "reason": "warehouse_snapshot_failed"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "fertilizer_fill")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("runtime ok=false should fail, got %#v", result)
	}
}

func TestRuntimeFacadeRewardClaimCallsRuntime(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.claimTaskRewardsByProtocol": map[string]any{"ok": true, "claimedCount": float64(3)},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "reward_claim")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("reward_claim should report OK, got %#v", result)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("runtime call count = %d, want 1", len(caller.calls))
	}
	if caller.calls[0].method != "gameCtl.claimTaskRewardsByProtocol" {
		t.Fatalf("method = %q, want claimTaskRewardsByProtocol", caller.calls[0].method)
	}
	wantArgs := []any{map[string]any{
		"silent":                  true,
		"maxRounds":               8,
		"claimDailyActiveRewards": true,
		"source":                  "farm_go_auto_reward_claim",
	}}
	if !reflect.DeepEqual(caller.calls[0].args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", caller.calls[0].args, wantArgs)
	}
}

func TestRuntimeFacadeRewardClaimReportsSkip(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.claimTaskRewardsByProtocol": map[string]any{"ok": true, "skipped": true, "reason": "no_claimable_task_reward"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "reward_claim")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("reward_claim skipped runtime result should be OK, got %#v", result)
	}
}

func TestRuntimeFacadeRewardClaimReportsNoClaimableRewards(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.claimTaskRewardsByProtocol": map[string]any{
			"ok":           true,
			"success":      true,
			"reason":       "no_claimable_task_reward",
			"claimedCount": float64(0),
		},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "reward_claim")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("reward_claim no-claimable result should be OK, got %#v", result)
	}
	if result.Message != "自动领取任务奖励：无可领取奖励。" {
		t.Fatalf("message = %q, want no-claimable reward message", result.Message)
	}
}

func TestRuntimeFacadeRewardClaimHandlesNilRuntimeCaller(t *testing.T) {
	facade := NewRuntimeFacade(nil)

	result := facade.RunTask(context.Background(), "reward_claim")

	if result.OK || result.Status != StatusRuntimeNotReady {
		t.Fatalf("nil runtime should be runtime_not_ready, got %#v", result)
	}
}

func TestRuntimeFacadeRewardClaimReportsRuntimeError(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.claimTaskRewardsByProtocol": errors.New("task service unavailable"),
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "reward_claim")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("runtime error should fail, got %#v", result)
	}
}

func TestRuntimeFacadeRewardClaimReportsRuntimeNotOKResult(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.claimTaskRewardsByProtocol": map[string]any{"ok": false, "reason": "task_info_not_decoded"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "reward_claim")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("runtime ok=false should fail, got %#v", result)
	}
}

func TestRuntimeFacadeSvipDailyGiftCallsRuntime(t *testing.T) {
	testDirectRuntimeTaskBehavior(t, directRuntimeTaskTestCase{
		taskID: "svip_daily_gift",
		method: "gameCtl.claimSvipDailyGift",
		args: []any{map[string]any{
			"silent": true,
			"waitMs": 800,
			"source": "farm_go_auto_svip_daily_gift",
		}},
	})
}

func TestRuntimeFacadeMonthlyCardRewardCallsRuntime(t *testing.T) {
	testDirectRuntimeTaskBehavior(t, directRuntimeTaskTestCase{
		taskID: "monthly_card_reward",
		method: "gameCtl.claimMonthlyCardReward",
		args: []any{map[string]any{
			"silent": true,
			"waitMs": 800,
			"source": "farm_go_auto_monthly_card_reward",
		}},
	})
}

func TestRuntimeFacadeMallDailyFertilizerCallsRuntime(t *testing.T) {
	testDirectRuntimeTaskBehavior(t, directRuntimeTaskTestCase{
		taskID: "mall_daily_fertilizer",
		method: "gameCtl.claimMallDailyFertilizerGift",
		args: []any{map[string]any{
			"silent": true,
			"waitMs": 800,
			"source": "farm_go_auto_mall_daily_fertilizer",
		}},
	})
}

func TestRuntimeFacadeShareRewardCallsRuntime(t *testing.T) {
	testDirectRuntimeTaskBehavior(t, directRuntimeTaskTestCase{
		taskID: "share_reward",
		method: "gameCtl.claimShareRewardByProtocol",
		args: []any{map[string]any{
			"silent":       true,
			"waitMs":       300,
			"claimDelayMs": 3200,
			"source":       "farm_go_auto_share_reward",
		}},
	})
}

func TestRuntimeFacadeRewardTasksReportAlreadyClaimedAsNoClaimable(t *testing.T) {
	tests := []struct {
		name    string
		taskID  string
		method  string
		action  string
		message string
	}{
		{
			name:    "svip_daily_gift",
			taskID:  "svip_daily_gift",
			method:  "gameCtl.claimSvipDailyGift",
			action:  "svip_daily_gift",
			message: "SVIP每日礼包：今日已领取或无可领取奖励。",
		},
		{
			name:    "monthly_card_reward",
			taskID:  "monthly_card_reward",
			method:  "gameCtl.claimMonthlyCardReward",
			action:  "monthly_card_reward",
			message: "月卡奖励：今日已领取或无可领取奖励。",
		},
		{
			name:    "mall_daily_fertilizer",
			taskID:  "mall_daily_fertilizer",
			method:  "gameCtl.claimMallDailyFertilizerGift",
			action:  "claim_mall_daily_fertilizer",
			message: "商城每日肥料：今日已领取或无可领取奖励。",
		},
		{
			name:    "share_reward",
			taskID:  "share_reward",
			method:  "gameCtl.claimShareRewardByProtocol",
			action:  "claim_share_reward",
			message: "分享奖励：今日已领取或无可领取奖励。",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := &fakeRuntimeCaller{responses: map[string]any{
				tt.method: map[string]any{
					"ok":      true,
					"success": true,
					"skipped": true,
					"reason":  "already_claimed",
					"action":  tt.action,
				},
			}}
			facade := NewRuntimeFacade(caller)

			result := facade.RunTask(context.Background(), tt.taskID)

			if !result.OK || result.Status != StatusOK {
				t.Fatalf("%s already-claimed result should be OK, got %#v", tt.taskID, result)
			}
			if result.Message != tt.message {
				t.Fatalf("message = %q, want %q", result.Message, tt.message)
			}
		})
	}
}

func TestRuntimeFacadeMallDailyFertilizerReportsPurchaseLimitUsedAsNoClaimable(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.claimMallDailyFertilizerGift": map[string]any{
			"ok":      true,
			"success": true,
			"skipped": true,
			"reason":  "限购次数已用完",
			"action":  "claim_mall_daily_fertilizer",
		},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "mall_daily_fertilizer")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("mall_daily_fertilizer purchase-limit result should be OK, got %#v", result)
	}
	if result.Message != "商城每日肥料：今日已领取或无可领取奖励。" {
		t.Fatalf("message = %q, want no-claimable mall fertilizer message", result.Message)
	}
}

func TestRuntimeFacadeMailRewardCallsRuntime(t *testing.T) {
	testDirectRuntimeTaskBehavior(t, directRuntimeTaskTestCase{
		taskID: "mail_reward",
		method: "gameCtl.claimMailRewardsByProtocol",
		args: []any{map[string]any{
			"silent": true,
			"types":  []any{1, 2},
			"waitMs": 1600,
			"source": "farm_go_auto_mail_reward",
		}},
	})
}

func TestRuntimeFacadeMailRewardReportsNoClaimableRewards(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.claimMailRewardsByProtocol": map[string]any{
			"ok":           true,
			"success":      true,
			"reason":       "no_claimable_mail",
			"action":       "claim_mail_rewards",
			"claimedCount": float64(0),
		},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "mail_reward")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("mail_reward no-claimable result should be OK, got %#v", result)
	}
	if result.Message != "邮件奖励：今日已领取或无可领取奖励。" {
		t.Fatalf("message = %q, want no-claimable mail message", result.Message)
	}
}

func TestRuntimeFacadeHeFengTravelRewardCallsRuntime(t *testing.T) {
	testDirectRuntimeTaskBehavior(t, directRuntimeTaskTestCase{
		taskID: "he_feng_travel_reward",
		method: "gameCtl.claimHeFengTravelRewards",
		args: []any{map[string]any{
			"silent": true,
			"waitMs": 800,
			"source": "farm_go_auto_he_feng_travel_reward",
		}},
	})
}

func TestRuntimeFacadeQianXingTravelRewardCallsCurrentProtocol(t *testing.T) {
	testDirectRuntimeTaskBehavior(t, directRuntimeTaskTestCase{
		taskID: "qian_xing_travel_reward",
		method: "gameCtl.claimQianXingTravelRewards",
		args: []any{map[string]any{
			"silent": true,
			"waitMs": 1500,
			"source": "farm_go_auto_qian_xing_travel_reward",
		}},
	})
}

func TestRuntimeFacadeXingSuAutoLightUpCallsCurrentProtocol(t *testing.T) {
	testDirectRuntimeTaskBehavior(t, directRuntimeTaskTestCase{
		taskID:      "xing_su_auto_light_up",
		method:      "gameCtl.autoLightUpXingSuByProtocol",
		successText: "已确认自动点亮星宿。本次处理 1 项。",
		args: []any{map[string]any{
			"silent": true,
			"waitMs": 1500,
			"source": "farm_go_auto_xing_su_auto_light_up",
		}},
	})
}

func TestRuntimeFacadeLimitedSeedDrawCallsRuntime(t *testing.T) {
	testDirectRuntimeTaskBehavior(t, directRuntimeTaskTestCase{
		taskID: "limited_seed_draw",
		method: "gameCtl.claimLimitedSeedDraw",
		args: []any{map[string]any{
			"silent":   true,
			"waitMs":   800,
			"source":   "farm_go_auto_limited_seed_draw",
			"drawKind": "free",
		}},
	})
}

func TestRuntimeFacadeLimitedSeedDrawRunsPaidDrawAfterFreeWhenEnabled(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.claimLimitedSeedDraw": map[string]any{"ok": true, "itemUpdateCount": float64(1)},
	}}
	config := DefaultConfig()
	config["autoFarmLimitedSeedDrawPaidEnabled"] = true
	facade := NewRuntimeFacadeWithConfig(caller, config)

	result := facade.RunTask(context.Background(), "limited_seed_draw")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("limited_seed_draw paid draw should report OK, got %#v", result)
	}
	if len(caller.calls) != 2 {
		t.Fatalf("runtime call count = %d, want 2", len(caller.calls))
	}
	wantFreeArgs := []any{map[string]any{
		"silent":   true,
		"waitMs":   800,
		"source":   "farm_go_auto_limited_seed_draw",
		"drawKind": "free",
	}}
	if !reflect.DeepEqual(caller.calls[0].args, wantFreeArgs) {
		t.Fatalf("free args = %#v, want %#v", caller.calls[0].args, wantFreeArgs)
	}
	wantPaidArgs := []any{map[string]any{
		"silent":   true,
		"waitMs":   800,
		"source":   "farm_go_auto_limited_seed_draw",
		"drawKind": "paid",
	}}
	if !reflect.DeepEqual(caller.calls[1].args, wantPaidArgs) {
		t.Fatalf("paid args = %#v, want %#v", caller.calls[1].args, wantPaidArgs)
	}
}

type directRuntimeTaskTestCase struct {
	taskID      string
	method      string
	args        []any
	successText string
}

func testDirectRuntimeTaskBehavior(t *testing.T, tc directRuntimeTaskTestCase) {
	t.Helper()

	t.Run("success", func(t *testing.T) {
		caller := &fakeRuntimeCaller{responses: map[string]any{
			tc.method: map[string]any{"ok": true, "claimedCount": float64(1)},
		}}
		facade := NewRuntimeFacade(caller)

		result := facade.RunTask(context.Background(), tc.taskID)

		if !result.OK || result.Status != StatusOK {
			t.Fatalf("%s should report OK, got %#v", tc.taskID, result)
		}
		if len(caller.calls) != 1 {
			t.Fatalf("runtime call count = %d, want 1", len(caller.calls))
		}
		if caller.calls[0].method != tc.method {
			t.Fatalf("method = %q, want %s", caller.calls[0].method, tc.method)
		}
		if !reflect.DeepEqual(caller.calls[0].args, tc.args) {
			t.Fatalf("args = %#v, want %#v", caller.calls[0].args, tc.args)
		}
		if tc.successText != "" && result.Message != tc.successText {
			t.Fatalf("success message = %q, want %q", result.Message, tc.successText)
		}
	})

	t.Run("skip", func(t *testing.T) {
		caller := &fakeRuntimeCaller{responses: map[string]any{
			tc.method: map[string]any{"ok": true, "skipped": true, "reason": "already_claimed"},
		}}
		facade := NewRuntimeFacade(caller)

		result := facade.RunTask(context.Background(), tc.taskID)

		if !result.OK || result.Status != StatusOK {
			t.Fatalf("%s skipped runtime result should be OK, got %#v", tc.taskID, result)
		}
	})

	t.Run("nil_runtime", func(t *testing.T) {
		facade := NewRuntimeFacade(nil)

		result := facade.RunTask(context.Background(), tc.taskID)

		if result.OK || result.Status != StatusRuntimeNotReady {
			t.Fatalf("nil runtime should be runtime_not_ready, got %#v", result)
		}
	})

	t.Run("runtime_error", func(t *testing.T) {
		caller := &fakeRuntimeCaller{responses: map[string]any{
			tc.method: errors.New("runtime unavailable"),
		}}
		facade := NewRuntimeFacade(caller)

		result := facade.RunTask(context.Background(), tc.taskID)

		if result.OK || result.Status != StatusFailed {
			t.Fatalf("runtime error should fail, got %#v", result)
		}
	})

	t.Run("runtime_not_ok", func(t *testing.T) {
		caller := &fakeRuntimeCaller{responses: map[string]any{
			tc.method: map[string]any{"ok": false, "reason": "not_available"},
		}}
		facade := NewRuntimeFacade(caller)

		result := facade.RunTask(context.Background(), tc.taskID)

		if result.OK || result.Status != StatusFailed {
			t.Fatalf("runtime ok=false should fail, got %#v", result)
		}
	})
}
