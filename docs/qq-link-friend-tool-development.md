# QQ 链接好友工具开发文档

**状态：** 可进入实现设计  
**依据：** 本次受控 QQ 小程序运行时实验；本文不记录测试账号标识、OpenID、昵称、头像、验证文案或原始报文。

## 1. 目标与范围

开发一个独立的小型 Wails 桌面工具，专门处理 **QQ 分享链接进入好友农场** 这一条链路，并移植必要的好友信息能力。

首个版本需包含：

1. 连接并显示 QQ 小程序运行时状态。
2. 支持用户手动输入Gid查询农场信息目标农场的公开访问结果、土地摘要和护主犬状态。
3. 支持外部导入gid和结果导出等批量工作。
4. 第一步查询筛选完成后，自动根据筛选项加入添加好友队列。
5. 筛选结果需要包含用户名/等级/是否有护主犬，支持导出结果需要附带openid。

### 明确不做

- 不支持微信、应用宝或其他运行时链路。
- 不移植偷取、帮助、捣乱、收获等好友农场操作。

## 2. 已确认事实

### 2.1 QQ 分享链接进入链路

用户手动打开 QQ 分享链接后，QQ 小程序运行时观察到：

1. `GAME_ENTER` 与 `LaunchOptions`，场景值为 `10001`。
2. `gamepb.userpb.UserService.ReportArkClick`。
3. `gamepb.visitpb.VisitService.Enter`。
4. 在手动链接实验中，随后出现 `FARM_READY`、`FARM_ENTER`、`FARM_MAP_INITIALIZED` 与土地更新事件。

`ReportArkClick` 比该次 `Enter` 早约 174 ms。这个时序来自一次人工入口观察，作为链路识别证据，不应被实现为严格时序断言。

### 2.2 VisitService.Enter 协议

已确认请求中的字段语义如下：

| 字段 | 已确认含义 | 备注 |
| --- | --- | --- |
| field 1 | `hostGid`，目标农场 GID | 已由手动链接和直接实验交叉确认 |
| field 2 | 访问路径/原因 `reason` | 已验证 `2` 与 `5` 的行为不同 |
| field 3 | 手动链接样本中为 `0` | 语义与必要性未证明，不能依赖 |
| field 7 | 手动链接样本中为 32 字节不透明值 | 名称、生成规则和必要性均未证明，不能伪造 |

受控实验的结果：

| 请求 | 观察结果 | 产品解释 |
| --- | --- | --- |
| `Enter(hostGid, reason=2)`，测试目标不在好友列表 | 服务端返回 `1002002`，提示“不是好友无法拜访” | 普通好友访问受关系限制 |
| `Enter(hostGid, reason=5)`，同一测试目标 | 回调成功，返回基础资料、土地、护主犬摘要与服务器时间 | 可用于本工具的受控检查，但不代表前端场景切换 |
| 手动 QQ 分享链接 | `reason=5`，并有前端场景事件 | 真实链接入口的观察基线 |

`reason=5` 的成功回包已观察到：`basic`、`lands`、`briefDogInfo`、`serverUnixMilli`。直接请求时没有 UI 点击事件，也没有 `FARM_ENTER`，因此它只能标记为“协议检查成功”，不能显示为“已进入对方农场”。

### 2.3 GID 到 OpenID 的受限解析

`reason=5` 的当前回复中可以取得：

- `basic.gid`
- `basic.open_id`
- `basic.binded_communities[].openid`

只有 `basic.open_id` 适用于本次观察到的 QQ 原生加好友 API。`binded_communities[].openid` 属于不同字段，禁止替代使用。

加好友前必须在同一次实时回复内同时满足：

1. 回复的 `basic.gid` 与本次目标 GID 完全相等。
2. `basic.open_id` 符合 32 位十六进制格式。
3. QQ 运行时存在 `qq.addFriendByOpenId`。

不满足任一条件时立即结束，不能重试、猜测、回退到缓存值或接受前端传入的 OpenID。

### 2.4 QQ 原生加好友

手动点击陌生人农场“添加 QQ 好友”按钮时，已定位到：

```text
UI 路径：startup/root/ui/LayerUI/main_ui_v2/foot/uiStangerAddFriendRoot/
        UIStangerAddFriend/btn_addFriend
处理器：UIStangerAddFriend::UIFriendApply.onClick()
运行时调用：qq.addFriendByOpenId({ openId, verifyMsg, success, fail })
```

这条操作会打开 QQ 外部原生加好友弹窗。观察窗口内没有捕获到游戏业务 WebSocket 请求、分享 URL、`mqqapi` scheme 或成功/失败回调。因而本工具只能记录“已发起原生调用”，最终关系状态必须由用户在 QQ 弹窗内自行确认。

已有两次用户授权的受控账户实验均符合：一次 `VisitService.Enter(reason=5)`、一次 `addFriendByOpenId`、零次 UI 点击事件；短观察窗口内未收到原生回调。该结果验证调用链，未验证 QQ 对加好友请求的最终处理结果。

### 2.5 护主犬

`reason=5` 的访问回复包含 `briefDogInfo`。受控回复中已出现“护主犬”信息。现有 Go 护主犬扫描器已能消费 `gameCtl.inspectFriendFarmByProtocol` 的 `briefDogInfo`，不需要进入前端农场场景。

## 3. 产品边界与操作流

```text
用户手动打开 QQ 真实分享链接
        |
        v
QQ 小程序运行时：scene=10001 / ReportArkClick / VisitService.Enter(reason=5)
        |
        v
观察器锁定本次 hostGid，写入短生命周期会话
        |
        +--> 检查：Enter(reason=5) -> 资料摘要、土地摘要、briefDogInfo
        |
        +--> 用户二次确认 -> 校验同次回复 basic.gid + basic.open_id
                                    |
                                    v
                             qq.addFriendByOpenId -> QQ 原生弹窗
```

### 3.1 目标来源

生产界面的目标必须来自当前运行时捕获到的 QQ 分享会话；不提供任意 GID 输入框。这样工具只处理用户已经实际打开的 QQ 链接，且加好友动作能与当前会话绑定。

开发环境可保留显式的单目标诊断入口，但必须同时满足以下条件：仅本地构建启用、明显标示“开发诊断”、一次只允许一个目标、不可被正式 UI 或自动化任务调用。它用于协议回归测试，不属于产品功能。

### 3.2 状态机

| 状态 | 允许操作 | 退出条件 |
| --- | --- | --- |
| `Disconnected` | 重连 | QQ 运行时连通 |
| `Observing` | 停止监听 | 捕获到有效分享访问，或用户停止 |
| `TargetReady` | 检查农场 | 新分享会话、运行时断开、会话过期 |
| `Inspected` | 查看摘要、请求添加好友 | 新分享会话、运行时断开、会话过期 |
| `ConfirmAddFriend` | 取消或明确确认一次 | 取消、确认、会话失效 |
| `NativePromptOpened` | 查看调用审计 | 记录原生调用后结束；不等待业务成功 |

会话有效期建议为 5 分钟，且在捕获到新的分享目标、运行时重启或连接断开时立即失效。加好友确认只能使用当前 `Inspected` 会话中实时解析出的值。

## 4. 推荐架构

保留现有 QQ 运行时注入脚本和 Go runtime manager；新工具在其上增加受限、强类型的业务层。前端不得通过通用诊断接口执行任意运行时方法。

```text
Wails React UI
    -> App 绑定的强类型方法
        -> internal/qqfriendlink Service
            -> internal/runtime Manager / diagnostics transport
                -> QQ WS runtime host
                    -> resources/wmpf/button.js
                        -> VisitService.Enter / qq.addFriendByOpenId
```

### 4.1 复用点

| 层级 | 现有能力 | 新工具的使用方式 |
| --- | --- | --- |
| QQ 运行时 | `resources/wmpf/button.js` 的 `visitFriendFarmByProtocol`、`inspectFriendFarmByProtocol`、`addFriendByGidDiagnostic` | 只增加稳定、脱敏的工具专用返回包装；保留现有协议编码逻辑 |
| QQ 运行时 | 原生好友 API spy 与 `getRuntimeSpySnapshot()` | 用于手动链接和原生调用的观察证据 |
| Go | `internal/runtime` 与 `internal/diagnostics` | 作为受认证的 WS 调用通道，不能直接暴露给 UI |
| Go | `internal/farm/social/dog_guard.go` | 复用 `briefDogInfo` 的判定逻辑或抽出纯函数 |
| Wails | 根 `App` 的授权检查与事件记录 | 只绑定新的强类型工具方法 |

### 4.2 新模块建议

```text
internal/qqfriendlink/
  types.go          请求、脱敏响应、审计记录和状态枚举
  service.go        会话状态机与用例编排
  observer.go       将 runtime spy 事件归并为一次 QQ 分享会话
  inspector.go      受限的 reason=5 检查及结果净化
  add_friend.go     二次确认、同次回复校验、单次原生调用
  audit.go          内存审计与脱敏持久化策略
  service_test.go   状态机、校验和失败路径
```

在根 `App` 中提供领域方法，例如：

- `QQLinkFriendStatus() QQLinkFriendStatus`
- `StartQQLinkObservation() error`
- `StopQQLinkObservation() error`
- `InspectCurrentQQLinkFriend() (FriendInspection, error)`
- `PrepareQQNativeAddFriend() (AddFriendPreparation, error)`
- `ConfirmQQNativeAddFriend(confirmationID string) (NativeAddFriendResult, error)`

`ConfirmQQNativeAddFriend` 只接受一次性 `confirmationID`，由 `Prepare...` 生成并与会话版本绑定；不接受 GID、OpenID 或验证文案的自由组合参数。

## 5. 数据契约

所有响应都应为 Go 强类型结构，再由 Wails 生成绑定。以下字段是前端允许看到的最小集合。

### 5.1 分享会话

```go
type ShareLinkSession struct {
    ID          string    `json:"id"`
    State       string    `json:"state"` // Observing, TargetReady, Inspected, Expired
    TargetGID   int64     `json:"targetGid"`
    Scene       int       `json:"scene"` // 仅作证据展示，例如 10001
    Reason      int       `json:"reason"` // 已观察到 5
    CapturedAt  time.Time `json:"capturedAt"`
    ExpiresAt   time.Time `json:"expiresAt"`
}
```

### 5.2 农场检查结果

```go
type FriendInspection struct {
    SessionID       string        `json:"sessionId"`
    TargetGID       int64         `json:"targetGid"`
    ProtocolOK      bool          `json:"protocolOk"`
    IsSceneEntered  bool          `json:"isSceneEntered"` // 直接检查固定为 false
    LandCount       int           `json:"landCount"`
    ServerUnixMilli int64         `json:"serverUnixMilli"`
    Dog             DogSummary    `json:"dog"`
    FailureCode     string        `json:"failureCode,omitempty"`
    FailureMessage  string        `json:"failureMessage,omitempty"`
}

type DogSummary struct {
    Present bool   `json:"present"`
    ID      int    `json:"id,omitempty"`
    Name    string `json:"name,omitempty"`
}
```

不返回 `basic.open_id`、`binded_communities`、完整 `basic`、原始土地对象、原始回包或请求字节。若后续确有资料展示需求，应逐字段白名单新增，不能直接透传运行时对象。

### 5.3 加好友准备与结果

```go
type AddFriendPreparation struct {
    ConfirmationID string    `json:"confirmationId"`
    TargetGID      int64     `json:"targetGid"`
    ExpiresAt      time.Time `json:"expiresAt"`
    RequiresNativeConfirmation bool `json:"requiresNativeConfirmation"`
}

type NativeAddFriendResult struct {
    TargetGID       int64  `json:"targetGid"`
    InvocationState string `json:"invocationState"` // Invoked, NotInvoked
    Reason          string `json:"reason,omitempty"`
    CallbackState   string `json:"callbackState"` // Unknown, Success, Failure
}
```

`InvocationState=Invoked` 只代表已调用 QQ 原生 API。除非在可配置观察窗口内真的收到原生回调，否则 `CallbackState` 必须是 `Unknown`。

## 6. 前端界面与交互

采用一个紧凑的工作台，不增加营销页或通用协议控制台。

### 6.1 工作台结构

1. **运行时状态栏**：QQ WS 连接、当前监听状态、会话剩余时间。
2. **链接观察区**：开始/停止监听；捕获后显示目标 GID、场景、原因和时间。用户在 QQ 内手动打开分享链接。
3. **好友检查区**：仅在 `TargetReady` 可用；显示协议检查状态、土地数量、护主犬状态、服务器时间和明确的“未进入前端农场”标记。
4. **添加好友区**：仅在检查成功且会话未过期时可用。先显示确认模态框，再调用一次原生 API。
5. **本次审计区**：显示观察、检查、准备、调用的时间和脱敏状态，不显示敏感字段或原始包。

### 6.2 必须实现的交互限制

- 运行时未连接时，观察、检查和加好友操作全部禁用。
- 新的分享链接会使旧确认令牌立即失效。
- 确认模态框明确说明：将打开 QQ 原生弹窗，工具无法代替用户确认，也无法保证最终加好友成功。
- 点击确认后按钮进入不可重复提交状态，直到返回“已调用”或“未调用”。
- 原生调用失败、缺少 API、GID 不匹配、OpenID 校验失败都显示可读错误，但不显示原始数据。

## 7. 安全、审计与隐私

1. **查询先于动作。** 每次原生加好友调用都必须重新执行 `Enter(reason=5)`，并在本次回复内验证 GID 与 `basic.open_id`。
2. **能力最小化。** 前端只能调用领域方法，不能调用 `RunDiagnostic` 或传入运行时方法名。
3. **单目标、单次。** 一次分享会话只允许一个目标；一次确认令牌只能触发一次原生调用。
4. **无自动化确认。** 不检测或驱动 QQ 外部弹窗，不伪造成功状态。
5. **敏感字段不落盘。** OpenID、完整协议回包、请求字节、QQ 头像 URL 和验证文案不写入业务日志、导出文件或前端状态。运行时原始捕获仅可在受控调试环境短期保留。
6. **审计可追溯。** 审计仅保存时间、会话匿名 ID、目标 GID 的脱敏摘要、动作、结果枚举和失败原因；默认保留期建议 7 天。
7. **会话失效。** 运行时重启、断线、新分享事件、超过有效期时，清空目标与确认令牌。

## 8. 错误处理

| 条件 | 后端行为 | 前端文案方向 |
| --- | --- | --- |
| QQ WS 未连接 | 不发送请求 | 请连接 QQ 小程序后重试 |
| 未捕获有效分享会话 | 不检查、不加好友 | 请先在 QQ 中打开分享链接并等待捕获 |
| `Enter(reason=2)` 返回 `1002002` | 只作为协议差异证据，不走该路径 | 不向用户暴露为普通流程 |
| `Enter(reason=5)` 失败 | 不进入确认状态 | 无法读取当前分享目标的信息 |
| 回复 GID 不匹配 | 不调用原生 API | 当前会话校验失败，已阻止操作 |
| `basic.open_id` 缺失或格式无效 | 不调用原生 API | QQ 目标信息不完整，已阻止操作 |
| 原生 API 不存在 | 不调用 | 当前 QQ 客户端不支持该操作 |
| 未收到原生回调 | 记录 `Unknown` | 已请求 QQ 打开原生加好友弹窗，请在 QQ 中确认 |

## 9. 实施阶段

### Phase 1：受限 QQ 链接观察

- 从现有 runtime spy 中提取分享链路所需事件。
- 实现会话归并、有效期、断线失效和 Go 强类型状态接口。
- 添加仅通过人工分享入口产生目标的回归测试。

### Phase 2：检查与护主犬摘要

- 使用 `inspectFriendFarmByProtocol` 的 `reason=5` 路径。
- 只返回白名单摘要；复用或抽取护主犬判定纯函数。
- 覆盖成功、协议失败、无狗、有狗、会话过期和 UI 未进入的测试。

### Phase 3：原生加好友确认

- 将已有 `addFriendByGidDiagnostic` 收敛为工具专用的内部调用。
- 实现“准备 -> 一次性令牌 -> 明确确认 -> 单次调用”流程。
- 验证 GID 匹配、OpenID 来源、无重试、令牌单用、无敏感字段泄露。

### Phase 4：Wails 工作台与审计

- 实现状态栏、观察区、检查区、确认模态框和审计区。
- 完成断线、过期和原生回调未知状态的用户体验。
- 进行手动 QQ 回归：由操作者手动打开真实分享链接并在 QQ 原生弹窗内处理结果。

## 10. 测试与验收

### 自动化测试

- Go：会话状态机、输入约束、超时、令牌单用、脱敏序列化、审计保留策略。
- JavaScript VM：`Enter` 请求字段、`reason=5`、回复 GID 精确比对、只读取 `basic.open_id`、仅调用一次 `addFriendByOpenId`、保留 QQ API receiver。
- 前端：连接断开禁用、旧会话失效、二次确认、重复提交防护、`Unknown` 回调状态展示。
- 回归：既有运行时 spy、护主犬扫描及全量 Go 测试不得退化。

### 人工验收

1. 连接 QQ 小程序，开始监听。
2. 操作者在 QQ 内手动打开一次真实分享链接。
3. 工具显示一个受限的当前会话，而非任意目标输入。
4. 执行检查后显示土地摘要与护主犬结果，并清楚标记这不是前端场景进入。
5. 点击添加好友，完成二次确认。
6. 工具只发起一次 QQ 原生调用；QQ 外部弹窗由操作者自行处理。
7. 审计中不存在 OpenID、原始协议包或验证文案。

## 11. 已知限制

- 当前没有证据表明“添加 QQ 好友”使用分享 URL、`mqqapi`、scheme 或游戏业务 WebSocket；不要为它构造链接。
- 原生成功/失败回调在短观察窗口内未出现，不能根据“已调用”推断 QQ 最终结果。
- 手动分享链接实验包含未命名的不透明字段；它们的作用尚未证明，不能在生产逻辑中生成或依赖。
- `reason=5` 的行为是基于当前受控实验确认的运行时事实。QQ 客户端、小游戏版本或服务端策略变化后，应重新执行单目标、受控回归后再发布。

## 12. 实现完成定义

当且仅当以下条件都满足时，本工具可进入试用：

1. 只包含 QQ 链接链路，且不存在微信或应用宝入口。
2. 正式 UI 无任意 GID、OpenID、协议方法或原始参数输入能力。
3. 所有原生加好友调用都满足当前会话、实时回复、GID 匹配、OpenID 格式校验、二次确认和单次执行。
4. UI 明确区分“协议检查成功”“已发起 QQ 原生弹窗”和“QQ 最终加好友结果”。
5. 自动化测试、运行时脚本语法检查、Go 全量测试与一次人工 QQ 链接回归均通过。
