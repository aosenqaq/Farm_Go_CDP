# 自动点亮星宿设计

## 目标

在“自动领取奖励”中新增“自动点亮星宿”任务。该任务默认关闭；用户开启后按默认 120 分钟间隔运行，并出现在“千星游记奖励领取”之后。

星宿活动的活动 ID 每日变化。因此，运行时必须先从当前服务器状态获得当天有效活动及可点亮状态，再编码点亮操作。不得复用捕获当天的 ID，也不得从游戏 UI 读取或驱动状态。

## 已确认协议证据

2026-07-29 手动执行“千星游记 -> 星宿 -> 点亮”已捕获如下证据：

- 按钮处理器：`S2BigEvent::S2BigEventUI.onOneKeyLine()`。
- 节点：`startup/root/ui/LayerPopUp/S2MainUI/node_content/S2BigEvent/node_bottom/S2BigEventBtn/btn_oneKeyLine`。
- 服务：`gamepb.activitypb.ActivityService`。
- 方法：`Operate`。
- 请求字节：`08 fd d4 8d c6 07 10 15 ba 07 00`。
- 请求字段：活动 ID（字段 1，本次值 `2026072701`，动态）、命令（字段 2，值 `21`）和空的字段 119 负载。
- 成功同窗证据：`ACTIVITY_OPERATE_CHANGE`、两项 `ITEM_UPDATE`，随后出现奖励弹窗事件。

原始证据保留在 `data/debug-captures/xing-su-light-up-2026-07-29T02-28-51-892Z-raw.json`，不纳入提交。

## 配置与调度

任务使用本期专用标识，避免与历史活动或千星游记奖励领取共享配置：

- 任务 ID：`xing_su_auto_light_up`。
- 展示名称：`自动点亮星宿`。
- 启用配置：`autoFarmXingSuAutoLightUpEnabled`。
- 间隔配置：`autoFarmXingSuAutoLightUpIntervalSec`、`autoFarmXingSuAutoLightUpIntervalMin`。
- 定时模式配置：`autoFarmXingSuAutoLightUpScheduleMode`、`autoFarmXingSuAutoLightUpScheduleTime`。

默认值为关闭、间隔模式、120 分钟（7200 秒）；指定时间默认 `08:00`。任务属于奖励协议通道，使用既有 `protocol` 和 `reward` 资源锁，避免与其他奖励请求并发。

前端奖励设置卡按现有通用开关、间隔和指定时间控件渲染，位置固定在“千星游记奖励领取”之后。调度中心显示同一名称、开关、优先级、间隔和下一次运行时间。

## 运行时数据流

运行时先执行 `gamepb.activitypb.ActivityService.List` 的空请求。查询回调使用 `{ meta, body }` 封装：只在 `meta.error_code === 0` 时继续；`body` 必须用运行时 `gamepb.activitypb.ListReply` codec 解码。已于 2026-07-29 现场验证该查询（`List` 双层传输观测 2 次、`Operate` 0 次、点击 0 次）。

已冻结的回包契约如下：

- 星宿活动集合：`groups[].children[]`，仅选择含 `mega_event` 的子活动。
- 动态活动 ID：`groups[].children[].head.id`，要求为安全正整数；`head.status === 1` 才能操作。
- 可点亮状态：`groups[].children[].mega_event.rewards[]` 中存在 `unlocked === true && claimed !== true`。
- 已完成或尚未开放：星宿活动存在但不存在上述可点亮奖励时，返回成功跳过 `no_claimable_xing_su_reward`；无开放星宿活动时返回成功跳过 `xing_su_activity_not_open`。
- 冲突、缺失或无效字段：返回失败且不发送 `Operate`。

查询成功且返回一个有效、可点亮的星宿活动时，运行时从同一回复的活动 ID 编码 `ActivityService.Operate`：字段 1 为活动 ID、字段 2 为 `21`、字段 119 为空负载。操作完成后，以零错误的操作回调作为成功证据；运行时绝不保留或回退使用历史活动 ID。

查询明确表示已完成、活动未开放或可点亮集合为空时，任务返回成功跳过。查询超时、回调失败、codec 缺失、字段缺失、活动冲突或状态不明时，任务返回失败且绝不发送 `Operate`。

为发现并冻结该状态查询，先加入受限的只读运行时诊断：它只检查协议服务、活动管理器和已加载 System 模块的命名导出；不打开、点击、关闭或读取星宿 UI。诊断结果仅保存到忽略的捕获目录，并用于锁定查询方法和回复 codec。该诊断不会暴露为调度任务，也不会发送点亮操作。

## 非目标

- 不硬编码 `2026072701` 或任何历史活动 ID。
- 不用 `ACTIVITY_OPERATE_CHANGE` 的旧事件、缓存事件或最近一次 UI 状态替代本次服务器查询。
- 不打开、点击、关闭或抓取星宿界面。
- 不改动千星游记奖励领取和已退役荷风活动的行为。

## 测试与验收

- VM 测试先验证状态查询失败、超时、回包字段缺失或冲突时不会发送 `Operate`。
- 以冻结的真实回包 fixture 测试 ID 提取、已完成跳过、无活动跳过和可点亮操作编码；测试 ID 和长度大于 127 的 protobuf varint。
- Go 测试验证新任务、专用配置、默认关闭、120 分钟、奖励定时映射、协议资源锁和运行时调用参数。
- 前端测试验证设置位置紧随“千星游记奖励领取”、名称及专用配置键。
- 现场验收先证明状态查询，再证明由同一回包的 ID 生成操作，或得到合法跳过；全程点击事件为零，且没有星宿 UI 打开或关闭事件。2026-07-29 的现场验收得到 `no_claimable_xing_su_reward` 合法跳过，`List` 2 次、`Operate` 0 次、点击 0 次。
