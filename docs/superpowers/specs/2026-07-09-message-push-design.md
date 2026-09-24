# Message Push Migration Design

## Goal

完善 Farm_Go 的消息推送模块，将旧项目 `message-push-manager.js` 的核心配置、测试发送和日报测试能力迁入当前 Wails/Go 架构，并用新的 A 版“控制台式”页面替换占位 UI。

本阶段不把自动化任务执行结果作为单独推送类型，避免通知过于频繁。

## Scope

包含：

- 持久化消息推送配置和运行状态。
- 支持单渠道推送配置。
- 支持测试推送和日报测试入口。
- 支持按 `dailyTime` 自动发送每日资产日报提醒。
- 支持消息类型级独立开关。
- 显示渠道状态、日报状态、最近推送记录和错误信息。

不包含：

- 多渠道同时推送。
- 自动化任务完成/失败独立通知。
- 完整日志扫描守护循环的复杂迁移。
- 旧 WebUI 的 Material UI 页面迁移。

## Message Types

消息推送有一个总开关，并提供以下独立类型开关：

- `abnormalEnabled`: 异常告警，覆盖超时、连接异常、运行时错误等。
- `suspectedEnabled`: 疑似异常，覆盖疑似未生效、疑似业务异常等。
- `recoveryEnabled`: 恢复通知，覆盖运行时恢复 ready 或异常恢复。
- `restartEnabled`: 重启通知，覆盖进程守护自动/定时重启开始、完成、失败。
- `dailyEnabled`: 每日资产日报。
- `logMonitorEnabled`: 日志监控入口开关，用于后续日志扫描能力。

旧项目已有字段继续保留：`enabled`, `abnormalEnabled`, `dailyEnabled`, `dailyMarkdownCardEnabled`, `dailyTime`, `logMonitorEnabled`, `abnormalTimeoutThreshold`, `logScanIntervalSec`, `httpTimeoutMs`, `pushRetryCount`, `selectedChannels`, `channelFormats`, `channels`。

新增字段：`suspectedEnabled`, `recoveryEnabled`。默认均为 `true`，与旧版本“异常类通知统一由 abnormalEnabled 控制”的行为兼容。

## Channels

保持旧项目支持的渠道名和单渠道选择模型：

- `serverchan`
- `pushplus`
- `qmsg`
- `wecom`
- `dingtalk`
- `feishu`
- `telegram`
- `bark`
- `ntfy`
- `webhook`

UI 允许选择一个渠道。发送时如果选中的渠道缺少必要参数，则返回明确错误；如果没有选中渠道但存在已配置渠道，可沿用旧逻辑选择第一个可用渠道。

## Backend Design

新增 `internal/messagepush` 包，负责配置规范化、状态管理和 HTTP 发送。

核心类型：

- `Config`: 消息推送配置。
- `Channels`: 各渠道密钥、Webhook、Topic 和格式配置。
- `State`: 运行状态，包括日报检查结果和最近推送记录。
- `Payload`: 待发送消息，包含 `kind`, `title`, `lines`, `meta`。
- `SendResult`: 每次发送的渠道结果、重试次数和摘要。
- `Service`: 加载配置、保存配置、构造状态、测试推送、日报测试、检查并发送到期日报。

`Service` 依赖：

- 存储层读写配置和状态。
- `http.Client` 发送渠道请求，便于测试注入。
- 可选的日报数据提供器。本阶段先提供简洁日报 payload；后续可接入更完整的每日统计。

HTTP 发送实现覆盖旧项目测试入口需要的主要渠道。所有渠道统一走 `httpTimeoutMs` 和 `pushRetryCount`。

日报调度采用轻量 Go ticker：

- App 启动后创建消息推送服务并启动后台检查循环。
- 每分钟检查一次配置和状态。
- 仅当 `enabled && dailyEnabled`、当前时间已达到 `dailyTime`、目标日期尚未发送时发送日报。
- 发送成功后写入 `lastDailySummaryDateKey` 和 `lastDailySummaryAt`。
- 发送失败不标记已发送，并记录 `lastDailySummarySkipReason=send_failed`。
- 前端状态中的 `daily.nextRunAt` 根据当前配置和状态计算。

## Storage Design

在 `settings` 表中存储消息推送配置，使用一个 JSON 字符串键：

- `messagePush.config`

运行状态使用另一个 JSON 字符串键：

- `messagePush.state`

这样不需要新增表，符合当前 settings 存储模式，也便于配置整体保存。

状态保留最近 20 条推送记录。记录字段包括：

- `time`
- `kind`
- `title`
- `ok`
- `channels`
- `error`

## Wails API

在 `App` 暴露以下方法：

- `MessagePushState()`: 返回配置、状态、可用渠道列表、已配置渠道和类型开关状态。
- `SaveMessagePushConfig(config)`: 规范化并保存配置，返回最新状态。
- `SendMessagePushTest(config)`: 使用当前草稿配置发送测试推送。
- `SendMessagePushDailyTest(config)`: 使用当前草稿配置发送日报测试。
- `RunDueMessagePushDaily()`: 手动触发一次到期日报检查，便于测试和故障排查。

前端不直接写 `settings`；全部通过这些 API。

## Frontend Design

`MessagePushView` 使用 A 版控制台式布局：

- 页面头部显示标题、说明、总开关状态和当前渠道状态。
- 顶部第一行是 6 个类型开关卡片：异常告警、疑似异常、恢复通知、重启通知、资产日报、日志监控。
- 主体左侧是单渠道选择和当前渠道参数表单。
- 主体左下是高级参数：日报时间、异常超时阈值、日志扫描秒数、HTTP 超时、重试次数、日报 Markdown 卡片。
- 主体右侧是测试发送区和最近推送记录。

交互细节：

- 切换任何开关后标记为有未保存更改。
- “保存配置”保存当前草稿。
- “测试推送”和“日报测试”使用当前草稿配置，不要求先保存。
- 发送中按钮显示 loading，错误显示在测试区和最近记录中。
- 选中不同渠道时，只显示该渠道所需字段。

视觉方向：

- 沿用 Farm_Go 当前深绿、米白、荧光绿的工具感，但减少占位卡片。
- 类型开关用窄色条区分语义，不使用大面积单色背景。
- 页面保持高信息密度，适合桌面反复操作。

## Data Flow

页面加载：

1. 前端调用 `MessagePushState()`。
2. 后端加载 `messagePush.config`，缺失时返回默认配置。
3. 后端加载 `messagePush.state`，缺失时返回空状态。
4. 前端初始化草稿表单和状态卡片。

保存配置：

1. 前端提交草稿配置到 `SaveMessagePushConfig`。
2. 后端规范化配置并保存。
3. 后端返回最新 state payload。
4. 前端清除 dirty 标记。

测试发送：

1. 前端提交草稿配置到 `SendMessagePushTest`。
2. 后端规范化配置，选择可用渠道，构造测试 payload。
3. 后端按重试策略发送。
4. 后端写入最近推送记录并返回结果。
5. 前端刷新右侧最近记录。

日报测试：

1. 前端提交草稿配置到 `SendMessagePushDailyTest`。
2. 后端构造简洁日报 payload。
3. 后端按相同发送流程执行。

定时日报：

1. 后台 ticker 调用服务的到期检查。
2. 服务加载最新配置和状态。
3. 如果未到时间、已发送、未启用或无可用渠道，则只更新检查状态和跳过原因。
4. 如果到期且可发送，则发送昨日资产日报并更新状态。

## Error Handling

配置错误：

- 未选择且未配置任何可用渠道：返回 `当前选择的推送渠道未配置或不可用`。
- Webhook URL、Token、Chat ID 等必要字段为空：返回渠道级错误。
- Webhook Headers JSON 非法：返回 `请求头 JSON 无效`。
- 时间格式非法：规范化回 `09:00`。
- 数值超出范围：按旧项目范围钳制。

发送错误：

- 每个渠道按 `pushRetryCount` 重试。
- 只有所有目标渠道成功时 `ok=true`。
- 最近推送记录写入最终摘要，包括失败渠道和尝试次数。
- HTTP 非 2xx 响应视为失败，并记录状态码。

## Testing

Go 测试：

- 默认配置规范化。
- 新增类型开关默认值。
- 单渠道选择与回退。
- 重试成功和重试失败。
- Webhook headers JSON 校验。
- 状态最近记录最多保留 20 条。
- 到期日报只发送一次，失败不标记已发送。
- Wails API 保存配置、测试推送、日报测试。

前端测试：

- 渲染 6 个类型开关。
- 切换类型开关更新草稿。
- 选择渠道显示对应字段。
- 测试推送调用草稿配置。
- 发送错误在页面显示。

验证命令：

- `go test ./...`
- `cd frontend && npm test -- MessagePushView`
- `cd frontend && npm run build`

## Open Decisions

已定：

- UI 采用 A 版控制台式布局。
- 类型开关独立展示。
- 自动化任务执行结果不作为单独推送类型。
- 本阶段保留单渠道推送。

后续可扩展：

- 将完整自动化日报统计接入 `SendMessagePushDailyTest`。
- 将日志扫描守护循环接入 `logMonitorEnabled`。
- 将进程守护重启事件接入 `restartEnabled`。
