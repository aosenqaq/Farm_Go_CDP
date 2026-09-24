# Account Scoped Storage Design

## Goal

将 Farm_Go 的运行数据按 QQ 农场账号 GId 隔离保存。启动完成小程序注入并识别到账号后，用户确认识别弹窗，随后本次运行产生的日志、自动化配置、社交缓存和消息推送配置都写入该 GId 对应的 SQLite 作用域。

## Scope

包含：

- 启动链路 ready 后查询 `gameCtl.getPlayerProfile` 识别当前运行账号。
- 新增账号确认弹窗，展示头像、昵称、GId 和文案“已识别到运行账户，请点击确认”。
- 用户确认前用弹窗阻止进入主界面。
- 按 GId 隔离运行日志、自动化配置、社交规则/缓存和消息推送配置/状态。
- 侧栏左下角新增账号/授权预留信息块，风格参考用户提供的截图。

不包含：

- 卡密授权系统的真实校验。
- 多账号同时在线。
- 迁移历史全局数据到具体 GId。
- 改变注入链路、端口和守护进程等启动前必需配置的全局存储方式。

## Storage Boundary

保留全局存储：

- 默认运行目标和当前运行目标。
- CDP、WMPF、QQ WS 端口和启动配置。
- 进程守护配置。
- 启动注入状态。

按账号隔离：

- `runtime_events`
- `autoFarm.*`
- `messagePush.config`
- `messagePush.state`
- `social_*` 现有 `account_key` 数据

账号作用域使用字符串 `gid:<number>`。没有确认账号前，后端读取和写入使用 `default` 作为临时作用域；确认后切换到当前 GId 作用域并刷新前端状态。

## Backend Design

新增 `RuntimeAccount` DTO：

- `gid`
- `accountKey`
- `nickname`
- `avatarUrl`
- `confirmed`
- `identifiedAt`

`App` 增加当前账号状态：

- `currentAccountKey` 默认 `default`。
- `currentAccount` 保存最近识别并确认的账号。
- `IdentifyRuntimeAccount()` 调用 `gameCtl.getPlayerProfile`，从 `gid/name/nick/nickname/avatarUrl/avatar` 提取账号信息。
- `ConfirmRuntimeAccount(account)` 校验 GId，保存账号元数据，切换 `currentAccountKey`，重建依赖账号作用域的服务实例。
- `CurrentRuntimeAccount()` 返回当前确认状态。

服务使用账号作用域：

- `recordEvent` 写入 `runtime_events.account_key`。
- `RuntimeEvents` 只读取当前账号作用域。
- `FarmAutomationState` 和 `SaveFarmAutomationState` 使用当前账号作用域的设置键。
- `socialService` 和 `socialDogGuardScanner` 使用当前 `accountKey`。
- `messagepush.Service` 通过包装 store 使用当前账号作用域。

## Database Design

保留一个 SQLite 文件 `farm_go.db`，用账号字段实现隔离。

表结构调整：

- `runtime_events` 新增 `account_key TEXT NOT NULL DEFAULT 'default'`。
- `settings` 新增 `account_key TEXT NOT NULL DEFAULT 'global'`，主键改为 `(account_key, key)`。
- 新增 `runtime_accounts` 保存已识别账号元数据。

兼容策略：

- 旧 `settings` 表如果是 `key` 单主键，迁移为新表并将旧数据写入 `global` 作用域。
- 旧 `runtime_events` 表补充 `account_key='default'`。
- 全局设置继续读写 `global`。
- 账号设置读写传入的 `accountKey`。

## Frontend Design

启动流程：

1. 保持现有注入状态弹窗。
2. 当 `RuntimeStatus.ready === true` 且注入弹窗不需要显示时，调用 `IdentifyRuntimeAccount()`。
3. 识别成功后显示账号确认弹窗。
4. 用户点击确认，调用 `ConfirmRuntimeAccount()`。
5. 确认成功后刷新运行日志、自动化、社交、消息推送状态并进入主界面。

账号确认弹窗：

- 居中 modal，不能通过关闭按钮跳过。
- 展示圆形头像。
- 展示昵称和 `GId: <gid>`。
- 文案固定为“已识别到运行账户，请点击确认”。
- 确认按钮显示 loading 状态。
- 识别失败时显示重试按钮和错误文案，不进入主界面。

侧栏左下角预留块：

- 固定在 sidebar 底部。
- 左侧展示头像或默认图标。
- 中间展示昵称/未识别状态和本次运行时间。
- 右侧展示“时长卡 2027-07-13”样式的授权占位 badge。
- 不实现真实授权逻辑。

## Error Handling

- 运行时未 ready：不查询账号。
- 查询失败：账号弹窗显示错误和重试，不确认、不切换作用域。
- GId 缺失或非正数：返回“未识别到有效 GId”。
- 头像缺失：前端使用默认头像占位。
- 切换账号后依赖账号状态的服务缓存全部失效，避免把上个账号的数据继续写入新账号。

## Testing

Go 测试：

- settings 可同时保存 `global` 和两个不同 `gid:*` 作用域。
- runtime events 只返回指定账号作用域的数据。
- account key 规范化为 `gid:<number>`。
- `IdentifyRuntimeAccount` 能从 profile 提取 GId、昵称和头像。
- `ConfirmRuntimeAccount` 切换账号作用域。

前端测试：

- 账号确认弹窗展示头像、昵称、GId 和固定文案。
- 点击确认调用后端确认方法。
- App ready 后触发账号识别，确认前显示弹窗。
- AppShell 左下角渲染账号/授权预留块。

验证命令：

- `go test ./...`
- `cd frontend && npm test -- AccountIdentityDialog AppShell`
- `cd frontend && npm run build`
