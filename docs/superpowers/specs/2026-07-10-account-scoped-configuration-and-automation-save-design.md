# Account-Scoped Configuration and Automation Save Design

## Goal

修复农场自动化详细设置连续保存时开关恢复旧值的问题，并统一 Farm_Go 的配置持久化边界：机器级配置保存在 SQLite 的 `global` 作用域，农场业务配置和运行数据按已识别账号的 `gid:<number>` 作用域隔离。

## Scope

包含：

- 修复详细设置中的启用开关与调度任务启用状态不一致的问题。
- 修复自动化每日完成状态误写到 `default` 作用域的问题。
- 将仓库自动出售设置从全局运行设置中拆分为按 GID 隔离的业务配置。
- 审计并约束自动化、仓库、社交、消息推送和运行记录的账户作用域。
- 为旧 `global/default` 业务配置提供一次性复制迁移。
- 增加跨账户隔离、迁移和保存回归测试。

不包含：

- 将运行链路、端口、自动启动或进程守护配置绑定到 GID。
- 将静态游戏资源或 Frida 地址数据导入 SQLite。
- 重构为每项配置一个独立数据库列或专用表。
- 多账号同时在线或跨设备同步。

## Root Cause

自动化状态同时保存两份相关数据：

- `state.config[<featureEnabledKey>]`
- `state.scheduler.tasks[].enabled`

详细设置面板只更新 `config`，调度中心只更新任务状态。后端 `SettingsFromState` 为兼容两个入口，会在部分情况下用 `scheduler.tasks[].enabled` 回写配置。当一个默认关闭的功能已经开启并保存后，用户在详细面板关闭它时，`config` 变为 `false`，但任务状态仍为旧的 `true`。转换逻辑将旧任务状态写回配置，保存返回后开关重新显示为开启。

该问题由状态双写缺少明确的数据来源规则导致，不是 SQLite 事务丢失。

## Storage Boundary

### Global Machine Settings

以下设置在识别账号前就必须读取，并且描述当前电脑或运行环境，继续保存到 SQLite `settings.account_key='global'`：

- 默认和当前运行链路。
- 自动启动。
- CDP、WMPF 和 QQ WS 连接参数。
- 进程守护、重启阈值和定时重启设置。
- 注入、补丁和本机运行目标相关设置。
- 后续增加的纯 UI 机器偏好，例如主题或窗口布局。

### Account-Scoped Business Settings

以下数据保存到当前确认账号的 `settings.account_key='gid:<number>'` 或已有带 `account_key` 的业务表：

- `autoFarm.*` 自动化配置、调度配置和每日完成状态。
- 仓库自动出售开关、间隔、类别和刷新策略。
- 仓库出售记录。
- 社交好友规则、排行榜偏好、偷取记录、访客记录和护主犬缓存。
- 消息推送配置、每日发送状态和最近推送记录。
- 运行日志和自动化统计来源事件。
- 其他由玩家选择、农场进度或账号运行结果决定的数据。

### JSON Files

JSON 文件不作为应用运行配置的第二持久化源，只保留以下用途：

- `resources/gameConfig` 下的只读游戏数据和图片映射。
- `resources/wmpf/frida/config` 下按客户端版本匹配的地址表。
- `package.json`、`tsconfig.json`、`wails.json` 和构建清单。
- 用户主动导出或导入的备份文件。

用户修改的运行配置不得直接写入项目 JSON 文件。结构化配置可以序列化为 JSON 字符串存入 SQLite，但 SQLite 是唯一权威来源。

## Architecture

### Explicit Storage APIs

存储层保留明确的两类入口：

- 全局入口只处理机器设置，例如 `LoadGlobalRuntimeSettings` 和 `SaveGlobalRuntimeSettings`。
- 账户入口必须接收 `accountKey`，例如 `LoadAccountPreferences`、`SaveAccountPreferences` 和已有的 `*ForAccount` 方法。

生产代码不得使用会隐式落到 `default` 的自动化、消息推送或事件快捷方法。兼容方法可以暂时保留给旧测试或迁移逻辑，但不再被应用服务调用。

### Runtime Settings Split

当前 `storage.RuntimeSettings` 混合了机器字段和仓库业务字段。实现时将其拆分为：

- `GlobalRuntimeSettings`：运行链路、端口、自动启动和进程守护。
- `AccountPreferences` 或专用 `WarehouseAutoSellSettings`：仓库自动出售字段。

为了控制 Wails API 改动，可以继续向前端返回组合视图，但后端读取时从 `global` 和当前 GID 合并，保存时按字段分别写入两个作用域。内部存储模型必须保持边界清晰。

### Automation State Synchronization

自动化启用状态采用按入口确定来源的同步规则：

- 详细设置更新某个映射开关时，同时更新对应 `scheduler.tasks[].enabled`。
- 调度中心更新任务启用状态时，同时更新对应配置键。
- 保存归一化后，两处值必须一致。
- 后端转换不再根据配置是否等于默认值来猜测哪个值更新得更晚。

前端同步用于即时保证草稿一致，后端转换仍进行确定性归一化，防止其他调用方提交不一致状态。

## Data Flow

### Account Confirmation

1. 应用启动时只加载 `global` 机器设置。
2. 运行时识别 GID，用户确认账号。
3. 后端切换 `currentAccountKey` 为 `gid:<number>`。
4. 执行该 GID 的一次性旧配置初始化检查。
5. 重建自动化、消息推送和社交服务，并刷新账户业务视图。

### Configuration Save

1. 前端生成一致的自动化或业务配置草稿。
2. `App` 在调用存储层前读取一次当前 `accountKey`，本次操作始终使用该固定作用域。
3. 存储层在事务中写入当前 GID。
4. 后端从已保存的归一化数据生成返回状态。
5. 前端用返回状态更新父级状态和当前草稿。

同一次保存过程中不得再次读取可能变化的账户键，避免账号切换与异步保存交错时跨账户写入。

## Legacy Migration

每个 GID 首次确认时执行幂等初始化：

1. 检查账户迁移标记，例如 `migration.accountConfigSeed.v1`。
2. 如果该 GID 已有对应业务配置，保留现有值，不覆盖。
3. 对缺失项依次检查旧 `default` 和旧 `global` 业务键。
4. 仅复制存在的旧值到当前 GID。
5. 在同一事务或成功完成后写入迁移标记。
6. 后续读取只访问当前 GID，不再动态回退到旧作用域。

首批迁移项包括：

- 自动化设置和任务设置。
- 仓库自动出售设置。
- 消息推送配置和状态。
- 已存在于 `default` 的其他账户业务设置。

历史事件和业务记录不复制，避免把无法确定归属的数据绑定到错误账号。旧数据可以保留在原作用域，不参与新账户查询。

## Error Handling

- 未确认有效 GID 时，业务配置只允许使用临时 `default` 作用域，不得写入 `global`。
- GID 初始化复制失败时，不写迁移完成标记，并返回可重试错误。
- 已有 GID 配置优先，迁移不得覆盖用户当前数据。
- 保存失败时前端保留草稿并显示失败状态，不显示保存成功提示。
- 数据库事务失败时不得返回未经持久化的成功状态。
- 账号切换时停止并重建持有旧账户状态的调度器和服务缓存。

## Testing

### Automation Save Regression

- 默认关闭功能先开启保存，再关闭保存，最终配置和任务状态都为 `false`。
- 默认开启功能关闭保存后保持关闭。
- 详细设置和调度中心分别修改同一开关时结果一致。
- 连续多次保存不同开关不会恢复旧值。

### Storage Scope

- 两个 GID 可以保存不同的自动化开关和调度设置。
- 两个 GID 可以保存不同的仓库自动出售设置。
- 自动化每日完成状态写入当前 GID，不写入 `default`。
- 消息推送、社交偏好、运行事件和仓库记录保持账户隔离。
- 保存机器设置不会改变任一 GID 的业务配置。
- 保存账户业务配置不会改变 `global` 机器设置。

### Migration

- 首次确认时仅复制缺失的旧业务配置。
- 已有 GID 配置不会被旧值覆盖。
- 迁移完成后修改旧 `default/global` 数据不会影响该 GID。
- 迁移中途失败可以重试且不会产生重复业务记录。

### Verification

- `go test ./...`
- `cd frontend && npm test`
- `cd frontend && npm run build`

## Implementation Order

1. 添加自动化连续保存失败测试并修复状态同步。
2. 修复自动化每日状态的账户作用域。
3. 引入明确的全局/账户存储模型和仓库设置拆分。
4. 添加一次性旧配置迁移。
5. 审计并移除生产代码中的隐式 `default` 存储调用。
6. 运行全量后端、前端和构建验证。
