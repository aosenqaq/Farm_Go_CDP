# 恢复千星游记奖励领取设计

## 目标

在自动化的“奖励领取”设置和调度中心恢复本期活动任务“千星游记奖励领取”。

该任务使用独立于已结束荷风活动的任务 ID 与配置键，默认关闭；用户开启后按默认的 120 分钟间隔调度。2026-07-29 已通过 QQ 小程序“千星游记 -> 一键领取”捕获当前协议，并接入本期专属运行时方法。

## 范围

新增并公开以下任务：

- 任务 ID：`qian_xing_travel_reward`
- 展示名称：`千星游记奖励领取`
- 启用配置：`autoFarmQianXingTravelRewardEnabled`
- 间隔配置：`autoFarmQianXingTravelRewardIntervalSec`、`autoFarmQianXingTravelRewardIntervalMin`
- 定时模式配置：`autoFarmQianXingTravelRewardScheduleMode`、`autoFarmQianXingTravelRewardScheduleTime`

默认值如下：

- 默认关闭。
- 默认执行模式为间隔执行。
- 默认间隔为 120 分钟（7200 秒）。
- 默认指定时间保留为 `08:00`，供用户切换到指定时间模式后使用。

前端将该设置卡置于“邮件奖励”之后；调度中心使用同一名称显示该任务，并允许按现有任务行修改开关、优先级和间隔。

## 非目标

- 不恢复荷风游记任务或抽奖任务。
- 不复用 `gameCtl.claimHeFengTravelRewards`；即使本期协议的服务和方法名相同，也使用本期专属常量和运行时方法。
- 不硬编码或复用旧活动 ID；捕获证实当前一键领取请求体为空。
- 不迁移已保存的荷风任务设置到本期任务；本期任务以默认关闭状态开始。

## 架构与数据流

后端任务目录、调度规格、奖励定时映射和默认配置均以 `qian_xing_travel_reward` 为唯一标识。现有退役清单继续保留荷风任务的配置键，保证旧任务不再出现在公开状态或自动调度中。

`StateFromSettings` 向前端和调度器公开新任务。前端复用现有奖励设置卡的通用开关、间隔和指定时间控件，因此保存后的配置按既有状态转换流程写回。

本期任务注册独立的奖励运行时规格，调用 `gameCtl.claimQianXingTravelRewards`。该方法直接下发空请求体到 `gamepb.seasonpb.SeasonService.ClaimBattlePassRewards`，不打开、点击或关闭游戏 UI。手动捕获的按钮证据为 `S2BattlepassUI.onAllGetHandler()`，节点为 `btn_oneKeyGet`；同窗成功证据为 `BattlePassChange` 及 `ITEM_UPDATE`。

运行时以回调、`BattlePassChange` 或 `ITEM_UPDATE` 作为成功证据；明确的“已领取”协议失败文本视为安全跳过。现场无 UI 验收已确认新方法发送正确服务和方法，且 runtime spy 的点击数为零。

## 错误处理

协议调用异常、未观察到回调或同窗成功事件时，`qian_xing_travel_reward` 返回失败，不将单纯发包视为成功。明确的“已领取”返回为成功跳过。默认关闭保持不影响未配置用户。

## 测试与验收

- 默认状态在调度中心包含 `qian_xing_travel_reward`，标签正确、默认关闭且间隔为 7200 秒。
- 默认配置包含本期专用的开关、间隔和定时模式键，间隔分钟数为 120。
- 旧荷风任务仍不在公开调度状态中，旧开关继续归一化为关闭。
- 奖励定时映射包含本期任务的专用定时键。
- 前端渲染“千星游记奖励领取”设置卡，位置紧随“邮件奖励”，并使用 120 分钟作为回退间隔。
- 手动调用新任务使用 `gameCtl.claimQianXingTravelRewards`、`gamepb.seasonpb.SeasonService`、`ClaimBattlePassRewards` 和空请求体。
- 回归测试覆盖空请求体、服务/方法名和“已领取”安全跳过；现场验收确认无 UI 点击。
- 自动化 Go 包测试、相关前端测试和前端生产构建均通过。
