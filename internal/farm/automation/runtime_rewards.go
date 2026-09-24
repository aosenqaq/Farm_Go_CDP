package automation

import (
	"context"
	"time"
)

var rewardRuntimeTaskSpecs = map[string]directRuntimeTaskSpec{
	"fertilizer_fill": {
		taskID:       "fertilizer_fill",
		method:       "gameCtl.autoFillFertilizerBuckets",
		timeout:      60 * time.Second,
		notReadyText: "游戏运行时尚未连接，无法执行自动填充化肥。",
		errorPrefix:  "自动填充化肥调用失败：",
		failedPrefix: "自动填充化肥返回失败：",
		successText:  "已提交自动填充化肥请求。",
		args: []any{map[string]any{
			"fillNormal":  true,
			"fillOrganic": true,
			"closeAfter":  true,
			"silent":      true,
			"source":      "farm_go_auto_fertilizer_fill",
		}},
	},
	"reward_claim": {
		taskID:       "reward_claim",
		method:       "gameCtl.claimTaskRewardsByProtocol",
		timeout:      90 * time.Second,
		notReadyText: "游戏运行时尚未连接，无法执行自动领取任务奖励。",
		errorPrefix:  "自动领取任务奖励调用失败：",
		failedPrefix: "自动领取任务奖励返回失败：",
		successText:  "已提交自动领取任务奖励请求。",
		args: []any{map[string]any{
			"silent":                  true,
			"maxRounds":               8,
			"claimDailyActiveRewards": true,
			"source":                  "farm_go_auto_reward_claim",
		}},
	},
	"svip_daily_gift": {
		taskID:       "svip_daily_gift",
		method:       "gameCtl.claimSvipDailyGift",
		timeout:      45 * time.Second,
		notReadyText: "游戏运行时尚未连接，无法领取 SVIP 每日礼包。",
		errorPrefix:  "领取 SVIP 每日礼包调用失败：",
		failedPrefix: "领取 SVIP 每日礼包返回失败：",
		successText:  "已提交领取 SVIP 每日礼包请求。",
		args: []any{map[string]any{
			"silent": true,
			"waitMs": 800,
			"source": "farm_go_auto_svip_daily_gift",
		}},
	},
	"monthly_card_reward": {
		taskID:       "monthly_card_reward",
		method:       "gameCtl.claimMonthlyCardReward",
		timeout:      45 * time.Second,
		notReadyText: "游戏运行时尚未连接，无法领取月卡奖励。",
		errorPrefix:  "领取月卡奖励调用失败：",
		failedPrefix: "领取月卡奖励返回失败：",
		successText:  "已提交领取月卡奖励请求。",
		args: []any{map[string]any{
			"silent": true,
			"waitMs": 800,
			"source": "farm_go_auto_monthly_card_reward",
		}},
	},
	"mall_daily_fertilizer": {
		taskID:       "mall_daily_fertilizer",
		method:       "gameCtl.claimMallDailyFertilizerGift",
		timeout:      45 * time.Second,
		notReadyText: "游戏运行时尚未连接，无法领取商城每日肥料。",
		errorPrefix:  "领取商城每日肥料调用失败：",
		failedPrefix: "领取商城每日肥料返回失败：",
		successText:  "已提交领取商城每日肥料请求。",
		args: []any{map[string]any{
			"silent": true,
			"waitMs": 800,
			"source": "farm_go_auto_mall_daily_fertilizer",
		}},
	},
	"share_reward": {
		taskID:       "share_reward",
		method:       "gameCtl.claimShareRewardByProtocol",
		timeout:      60 * time.Second,
		notReadyText: "游戏运行时尚未连接，无法领取分享奖励。",
		errorPrefix:  "领取分享奖励调用失败：",
		failedPrefix: "领取分享奖励返回失败：",
		successText:  "已提交领取分享奖励请求。",
		args: []any{map[string]any{
			"silent":       true,
			"waitMs":       300,
			"claimDelayMs": 3200,
			"source":       "farm_go_auto_share_reward",
		}},
	},
	"mail_reward": {
		taskID:       "mail_reward",
		method:       "gameCtl.claimMailRewardsByProtocol",
		timeout:      90 * time.Second,
		notReadyText: "游戏运行时尚未连接，无法领取邮件奖励。",
		errorPrefix:  "领取邮件奖励调用失败：",
		failedPrefix: "领取邮件奖励返回失败：",
		successText:  "已提交领取邮件奖励请求。",
		args: []any{map[string]any{
			"silent": true,
			"types":  []any{1, 2},
			"waitMs": 1600,
			"source": "farm_go_auto_mail_reward",
		}},
	},
	"qian_xing_travel_reward": {
		taskID:       "qian_xing_travel_reward",
		method:       "gameCtl.claimQianXingTravelRewards",
		timeout:      60 * time.Second,
		notReadyText: "游戏运行时尚未连接，无法领取千星游记奖励。",
		errorPrefix:  "领取千星游记奖励调用失败：",
		failedPrefix: "领取千星游记奖励返回失败：",
		successText:  "已提交千星游记奖励领取请求。",
		args: []any{map[string]any{
			"silent": true,
			"waitMs": 1500,
			"source": "farm_go_auto_qian_xing_travel_reward",
		}},
	},
	"xing_su_auto_light_up": {
		taskID:       "xing_su_auto_light_up",
		method:       "gameCtl.autoLightUpXingSuByProtocol",
		timeout:      60 * time.Second,
		notReadyText: "游戏运行时尚未连接，无法自动点亮星宿。",
		errorPrefix:  "自动点亮星宿调用失败：",
		failedPrefix: "自动点亮星宿返回失败：",
		successText:  "已确认自动点亮星宿。",
		args: []any{map[string]any{
			"silent": true,
			"waitMs": 1500,
			"source": "farm_go_auto_xing_su_auto_light_up",
		}},
	},
	"he_feng_travel_reward": {
		taskID:       "he_feng_travel_reward",
		method:       "gameCtl.claimHeFengTravelRewards",
		timeout:      60 * time.Second,
		notReadyText: "游戏运行时尚未连接，无法领取荷风游记奖励。",
		errorPrefix:  "领取荷风游记奖励调用失败：",
		failedPrefix: "领取荷风游记奖励返回失败：",
		successText:  "已提交领取荷风游记奖励请求。",
		args: []any{map[string]any{
			"silent": true,
			"waitMs": 800,
			"source": "farm_go_auto_he_feng_travel_reward",
		}},
	},
	"limited_seed_draw": {
		taskID:       "limited_seed_draw",
		method:       "gameCtl.claimLimitedSeedDraw",
		timeout:      60 * time.Second,
		notReadyText: "游戏运行时尚未连接，无法执行荷风游记抽奖。",
		errorPrefix:  "荷风游记抽奖调用失败：",
		failedPrefix: "荷风游记抽奖返回失败：",
		successText:  "已提交荷风游记抽奖请求。",
		args: []any{map[string]any{
			"silent":   true,
			"waitMs":   800,
			"source":   "farm_go_auto_limited_seed_draw",
			"drawKind": "free",
		}},
	},
}

func (r RuntimeFacade) runRewardTask(ctx context.Context, taskID string) (ActionResult, bool) {
	spec, ok := rewardRuntimeTaskSpecs[taskID]
	if !ok {
		return ActionResult{}, false
	}
	if taskID == "limited_seed_draw" {
		return r.runLimitedSeedDraw(ctx, spec), true
	}
	return r.runDirectRuntimeTask(ctx, spec), true
}

func (r RuntimeFacade) runLimitedSeedDraw(ctx context.Context, freeSpec directRuntimeTaskSpec) ActionResult {
	freeResult := r.runDirectRuntimeTask(ctx, freeSpec)
	if !freeResult.OK || !boolConfigAny(r.config["autoFarmLimitedSeedDrawPaidEnabled"]) {
		return freeResult
	}

	paidSpec := freeSpec
	paidSpec.args = []any{map[string]any{
		"silent":   true,
		"waitMs":   800,
		"source":   "farm_go_auto_limited_seed_draw",
		"drawKind": "paid",
	}}
	paidResult := r.runDirectRuntimeTask(ctx, paidSpec)
	if !paidResult.OK {
		return paidResult
	}
	paidResult.Message = "已提交荷风游记免费与付费抽奖请求。"
	return paidResult
}
