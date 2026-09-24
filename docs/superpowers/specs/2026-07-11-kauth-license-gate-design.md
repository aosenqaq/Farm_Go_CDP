# Farm_Go KAuth 卡密门禁、心跳与更新检查设计

日期：2026-07-11

状态：已确认

## 1. 目标

在 Farm_Go 正式工作台前增加 KAuth 卡密门禁。每次启动都必须由用户点击“验证并进入”并完成在线验证；验证成功前，不启动正式运行链路、守护服务、消息推送、自动化相关服务或 QQ 调试补丁。

登录后按照 KAuth 返回的 `PongInterval` 发送心跳。连续三次心跳失败时撤销本次授权，完整停止正式服务并退回卡密页。

卡密验证成功后异步检查更新，不阻塞进入工作台。在“系统设置”中提供应用版本状态和手动“检查更新”入口。更新功能只提醒并打开下载页，不强制更新、不自动下载、不自动安装。

## 2. 已确认决策

- 使用 Go 后端硬门禁和延迟启动，不采用仅 React 遮挡的软门禁。
- 直接接入 KAuth Go SDK，不增加自有验证代理服务。
- KAuth 后台签名方式使用 RSA；程序密钥负责 SDK 协议规定的 AES 正文加解密，商户公钥负责签名和验签。
- 每次启动都必须在线验证，且必须由用户点击提交，不自动登录。
- 设备与当前 Windows 电脑绑定；从系统机器标识派生应用专属 UUID。
- 卡密页提供“记住卡密”，默认关闭。勾选后使用 Windows DPAPI 加密保存；下次启动自动填入，但仍需点击验证。
- 心跳周期严格来自登录响应的 `PongInterval`。连续失败三次才撤销授权；任一次成功都会将失败计数清零。
- 更新检查在卡密验证成功后后台执行。发现新版只提醒；检查失败不影响授权和正式功能。
- 卡密页使用已确认的“纯专注验证”布局，不展示正式导航。

## 3. 非目标

- 不实现账号密码登录、试用登录、注册、充值或解绑设备界面。
- 不实现离线授权或离线宽限期。
- 不实现自动更新器、增量更新、静默安装或强制更新。
- 不新增独立启动器 EXE。
- 不承诺客户端内嵌程序密钥不可被逆向提取。构建注入用于避免源码和日志泄露，不等于服务端级密钥保护。
- 不在本次工作中迁移或重构与授权门无关的农场业务模块。

## 4. 当前系统约束

当前 `App.startup()` 会在 Wails UI 启动时立即执行以下正式动作：

1. 打开 SQLite。
2. 创建消息推送服务并启动每日调度器。
3. 读取和应用运行设置。
4. 启动守护服务。
5. 根据配置自动启动运行链路。
6. 对 QQ WS 目标执行启动时调试补丁。

当前 React `App` 挂载后也会立即启动运行状态、守护状态、账户状态、QQ 补丁状态和社交状态等轮询。

因此仅在 React 外层显示卡密页面不能满足门禁要求。Go 生命周期和 React 挂载边界都必须拆分。

## 5. 总体架构

```text
Wails cold start
    |
    v
Bootstrap runtime
  - open SQLite
  - load protected remembered credential
  - initialise license manager
    |
    v
React LicenseGate ---- License API ----> internal/license.Manager
                                             |
                                  KAuth RSA login + user info
                                             |
                                             v
                                      Authorized session
                                             |
                    +------------------------+------------------------+
                    |                                                 |
                    v                                                 v
       activateAuthorizedRuntime()                         heartbeat worker
       - apply runtime settings                            - client.Pong()
       - message push scheduler                            - PongInterval
       - guardian                                          - failure counter
       - runtime auto-start
       - QQ startup patch
                    |
                    v
             React AuthorizedApp
```

授权会话拥有独立的 `context.Context` 和 generation。Wails 应用进程上下文只管理整个进程；授权 context 管理本次正式服务和心跳。重新登录会创建新的 generation，旧心跳或旧更新请求的迟到结果不得修改新会话。

## 6. 后端组件

### 6.1 License client adapter

在 `internal/license` 中定义最小客户端接口，业务代码不直接依赖 KAuth 具体类型：

- `Login(card, deviceID)`
- `GetUserInfo()`
- `Pong()`
- `GetProgramDetail()`
- `Logout()`

生产适配器使用 `github.com/kauth-coder/kauth-go/kauth` 和 `kauth.SignTypeRSA`。单元测试使用假客户端，不访问网络。

适配器负责：

- 将 SDK 的网络错误、协议错误和 API 错误转换为稳定的内部错误分类。
- 不向日志输出请求正文、响应正文、卡密、Token 或密钥。
- 验证登录响应包含 Token 和合法的正整数 `PongInterval`。
- 登录成功后调用 `GetUserInfo()`，获得到期时间、计费类型和剩余量。

### 6.2 License manager

授权状态：

- `locked`
- `authenticating`
- `authorized`
- `revoking`
- `error`

管理器保存当前 generation、内存 Token 所属的 KAuth 客户端、授权用户摘要、心跳周期、连续失败次数和会话取消函数。卡密不进入长期 manager 状态。

管理器对外只返回安全 DTO：

- phase
- authorized
- nickname
- expire time
- remaining amount
- billing type
- heartbeat health
- consecutive heartbeat failures
- user-facing error code and message

DTO 不包含卡密、Token、程序密钥、公钥原文或原始机器标识。

### 6.3 Device ID provider

Windows 实现读取 `MachineGuid`，规范化后与 Farm_Go 固定命名空间组合，派生确定性 UUID。原始 `MachineGuid` 不落盘、不记录日志、不返回前端。

如果系统机器标识无法读取，则生成安装 UUID 并保存到 `%APPDATA%\Farm_Go`，后续继续使用该值。降级值在 Farm_Go 重装并清空配置后可能变化；这是显式降级行为。

设备标识逻辑通过接口注入，单元测试不读取真实注册表。

### 6.4 Remembered credential store

“记住卡密”默认关闭。

- 验证成功且复选框勾选：使用 Windows DPAPI、当前用户作用域加密卡密，再保存密文。
- 验证成功且复选框未勾选：删除已有密文。
- 验证失败：不修改已有密文。
- 心跳撤销：不删除密文。
- 下次启动：后端解密后提供给卡密输入框自动填入，但不自动提交。

禁止保存卡密明文。存储层只接收 DPAPI 密文。DPAPI 实现与授权 manager 之间通过 `CredentialStore` 接口隔离。

### 6.5 Authorized runtime lifecycle

将当前 `startup()` 拆成两个阶段：

`bootstrap()` 只做：

- 保存 Wails context。
- 创建进程 context。
- 打开 SQLite 和执行迁移。
- 初始化授权 manager 和记忆卡密存储。

`activateAuthorizedRuntime(sessionCtx)` 才做：

- 创建消息推送服务并启动每日调度器。
- 读取、应用和必要时保存运行设置。
- 重建 CDP links。
- 启动 guardian。
- 根据 `Runtime.AutoStart` 启动默认运行链路。
- 对 QQ WS 目标执行启动调试补丁。

激活必须幂等。若任一步失败，按相反顺序回滚已启动组件，清除 Token，并回到 `locked`，不得留下半启动状态。

`deactivateAuthorizedRuntime()` 的顺序：

1. 将授权标记切到不可用，拒绝新的正式调用。
2. 取消 session context。
3. 停止农场自动化调度器。
4. 停止消息推送每日调度器。
5. 关闭 guardian。
6. 停止 runtime supervisor。
7. 清理账户级服务、缓存和 KAuth Token。
8. 发出最终撤销事件。

Wails 进程退出时复用相同停机逻辑，并对 `Logout()` 做尽力而为调用。网络退出失败不阻塞本地关闭。

### 6.6 Wails authorization boundary

未授权可调用方法采用显式白名单，至少包括：

- 查询授权状态。
- 读取已记住的卡密。
- 提交卡密登录。
- 清除已记住的卡密。

检查更新只在授权成功后开放。

其余正式 Wails 方法均属于受保护接口。未授权调用必须返回安全的未授权结果或错误，不能启动运行链路、修改设置、运行诊断、执行农场动作、启动守护或访问账户数据。

由于 root `App` 当前暴露大量方法，实施时必须维护可审计的 bootstrap 白名单，并增加测试防止新导出方法默认绕过授权。默认策略是“未列入白名单即受保护”。

## 7. 登录与心跳数据流

### 7.1 登录

1. React 读取 bootstrap 状态和已记住卡密。
2. 用户编辑卡密、选择是否记住，并点击“验证并进入”。
3. Go manager 原子地从 `locked` 切换到 `authenticating`；重复提交被拒绝或合并。
4. 生成当前电脑的派生 Device ID。
5. 调用 KAuth 卡密登录，平台类型使用明确的 PC/Go 标识。
6. 验证 Token 和 `PongInterval`。
7. 调用 `GetUserInfo()`。
8. 按复选框状态保存或删除 DPAPI 密文。
9. 创建新的授权 generation 和 session context。
10. 激活正式运行时。
11. 状态切换为 `authorized`，前端挂载 `AuthorizedApp`。
12. 并行启动心跳 worker 和一次更新检查。

### 7.2 心跳

- 每个授权 session 只有一个心跳 worker。
- 周期来自登录响应的 `PongInterval`，按毫秒解析。
- 周期缺失、非数字或不大于零时，本次登录按协议错误处理，不激活正式运行时。
- `Pong()` 成功时将连续失败数清零。
- 第一次或第二次连续失败时保持正式功能运行，并发出轻量健康警告。
- 第三次连续失败时执行一次撤销；并发失败不得触发重复停机。
- 撤销完成后前端收到 `license:revoked`，卸载 `AuthorizedApp` 并显示验证页。
- 旧 generation 的心跳、登录或更新结果一律丢弃。

## 8. 更新检查

KAuth `GetProgramDetail()` 返回服务端 `CurrentVersion`：

- `VersionNo`
- `VersionName`
- `VersionDesc`
- `VersionDownUrl`

本地构建提供唯一版本来源：

- 数字构建号，用于与 `VersionNo` 比较。
- 展示版本名，用于 UI 和 Windows 版本信息。

判断规则仅为 `remote.VersionNo > local.VersionNo`。服务端强制更新字段不参与门禁。

登录后自动检查：

- 在正式页面挂载后后台执行。
- 失败保持静默，不影响授权、心跳和正式服务。
- 有新版时展示一次非阻塞通知。
- 已是最新版时不展示自动通知。

系统设置手动检查：

- 展示当前版本、最新版本、上次检查时间和状态。
- 有新版时展示版本说明和“前往下载”。
- 已是最新版时明确显示结果。
- 失败时在更新区域内显示可重试错误。

下载地址只允许 HTTP 或 HTTPS URL，并交给系统默认浏览器。应用不下载或执行远程文件。

## 9. 前端结构与 UI

将当前 React `App` 拆成：

- `AppBootstrap`：查询授权状态、订阅授权事件并选择挂载页面。
- `LicenseGate`：卡密验证页。
- `AuthorizedApp`：现有正式应用内容和轮询 effects。

未授权时不得挂载 `AuthorizedApp`，从而避免当前轮询在卡密验证前调用正式 Wails API。

### 9.1 LicenseGate

采用已确认的“纯专注验证”方向：

- 1024×720 基准窗口下使用暖白背景和顶部轻量品牌栏。
- 顶部显示 Farm_Go 和在线授权状态，不显示正式导航。
- 中央单列、稳定宽度的卡密表单，不使用嵌套卡片。
- 卡密输入默认遮挡，提供 lucide 眼睛图标切换显示。
- 提供“记住卡密”复选框和设备已识别状态。
- 主按钮为“验证并进入”。空值时不可提交。
- 支持粘贴和 Enter 提交，但不在页面展示操作教程或快捷键说明。
- 加载时锁定输入、复选框和按钮，按钮尺寸保持不变。
- 错误在固定高度区域内联显示，不弹模态框、不清空输入。
- 验证成功后直接挂载工作台，不增加成功倒计时页。
- 底部仅显示当前 Farm_Go 版本。

记忆卡密在前端表现为正常填入的密码值，默认遮挡。只有用户主动切换可见性时显示明文。

### 9.2 Heartbeat warning

第一次和第二次连续心跳失败在正式页面显示轻量、可恢复的授权连接警告，包含 `1/3` 或 `2/3`。成功心跳后自动消失。

第三次失败不继续停留在工作台；完成后端停机后退回验证页，并显示“授权连接已失效，请重新验证”。

### 9.3 Settings update section

在现有“系统设置”页面增加“应用更新”区域：

- 当前版本。
- 最新版本。
- 上次检查时间。
- 检查状态。
- “检查更新”按钮，使用刷新图标。
- 有新版时显示版本说明和“前往下载”按钮。

自动发现新版时显示非阻塞通知，提供“查看更新”和“稍后”。“查看更新”切换到系统设置并定位应用更新区域。

## 10. 错误处理

稳定错误分类至少包括：

- local initialization failed
- invalid or expired card
- device rejected or bound elsewhere
- network unavailable or timeout
- malformed KAuth response
- heartbeat temporarily unhealthy
- session revoked
- authorized runtime activation failed
- update check failed
- invalid download URL

KAuth 可安全展示的业务消息可以作为补充，但不得把原始响应、签名、Token 或请求正文透传到 UI 或日志。

启动时本地存储失败属于 fatal bootstrap error。应用显示本地初始化失败页，不尝试绕过存储直接启动正式服务。

## 11. 构建配置与敏感信息

构建输入使用明确的环境变量或等价的受控 CI secret：

- `KAUTH_PROGRAM_ID`
- `KAUTH_PROGRAM_SECRET`
- `KAUTH_MERCHANT_PUBLIC_KEY`
- `FARM_GO_VERSION_NO`
- `FARM_GO_VERSION_NAME`

实际值不得写入设计文档、源码、Git、前端资源、默认配置、测试代码或构建日志。

`scripts/build.ps1` 在发布构建前验证必要参数，缺失时终止。开发环境允许通过环境变量提供测试配置。程序 ID 和公钥本身不是秘密，但仍集中管理，避免多处副本。

直接桌面 SDK 模式下，发布 EXE 最终包含程序密钥。后续如需要提高提取成本，可与现有保护发布流程结合，但不能把混淆描述为真正的密钥保密。

## 12. 测试策略

### 12.1 Go 单元测试

- 授权状态合法转换和重复提交。
- Windows 机器标识派生及降级安装 UUID。
- `CredentialStore` 保存、加载和删除语义。
- 登录错误分类和敏感信息脱敏。
- 合法与非法 `PongInterval`。
- 心跳成功清零；一、二、三次失败行为。
- generation 隔离和迟到结果丢弃。
- 正式运行时未授权不启动、授权后只启动一次。
- 激活中途失败的反向回滚。
- 完整停机顺序和幂等性。
- 未授权 Wails 方法白名单策略。
- 版本号比较、无版本数据和下载 URL 校验。

### 12.2 Windows credential tests

- DPAPI 密文可由同一用户解密。
- 保存内容不是卡密明文。
- 删除后无法再读取。
- 解密失败返回可恢复错误且不泄露密文内容。

非 Windows 普通测试使用接口替身，不要求真实 DPAPI。

### 12.3 Frontend tests

- 默认、已记住、加载和错误状态。
- 空值不可提交，Enter 提交，显示/隐藏卡密。
- 记住复选框请求语义。
- 未授权不挂载 `AuthorizedApp`，因此不启动现有轮询。
- 授权后只挂载一次。
- 心跳警告出现、恢复和撤销退回验证页。
- 更新区的检查中、最新版、有新版、失败状态。
- 新版通知跳转到设置区。

### 12.4 Opt-in KAuth integration test

默认 `go test ./...` 不访问 KAuth。真实集成测试必须显式提供环境变量，并串行验证：

1. 卡密登录。
2. 获取用户信息。
3. 至少一次 Pong。
4. 获取程序详情。
5. 退出登录。

集成测试不得打印卡密、Token、程序密钥、签名或原始响应正文。

### 12.5 Release acceptance

- 未验证时无运行链路监听、守护、消息推送调度、自动化调度或 QQ 启动补丁。
- 验证成功后正式服务按配置启动。
- 重启应用后记忆卡密自动填入但不自动验证。
- 连续三次心跳失败后完整停机并退回验证页。
- 更新检查不阻塞进入工作台。
- 手动检查更新可展示最新版、新版和错误状态。
- 仓库、前端 bundle、测试快照和构建日志不包含实际敏感参数或测试卡密。

## 13. 风险与缓解

### 客户端密钥可提取

风险：直连 SDK 需要在 EXE 中使用程序密钥。

缓解：构建 secret 注入、发布保护、禁止源码和日志泄露。若未来安全目标提高，应迁移到自有服务端代理。

### 现有 Wails API 面积较大

风险：新增导出方法可能忘记加授权检查。

缓解：未授权白名单、默认保护策略、自动化测试和代码审查清单。

### 服务停机竞态

风险：心跳撤销、用户操作和应用退出同时发生。

缓解：session generation、单次撤销、幂等 activate/deactivate 和明确停机顺序。

### KAuth 网络抖动

风险：单次请求失败误踢用户。

缓解：连续三次失败阈值，任一次成功清零；启动登录仍保持每次在线验证要求。

### 记忆卡密增加本地敏感数据

风险：卡密需要落盘才能自动填入。

缓解：默认关闭、DPAPI 当前用户加密、只存密文、成功登录后才更新、用户取消时删除。
