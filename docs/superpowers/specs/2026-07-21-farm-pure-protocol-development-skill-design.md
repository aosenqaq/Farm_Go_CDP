# 农场纯协议开发 Skill 设计

## 目标

创建个人 skill `farm-pure-protocol-development`，复用本次 SVIP 功能的开发流程，在不打开、点击或依赖游戏 UI 的前提下，为 QQ 农场新功能发现查询协议、读取真实回包字段、动态编码请求并完成实时验证。

## Skill 边界

新 skill 负责：

- 通过 Farm_Go Wails RPC 和 QQ WS 调用已有或临时 `gameCtl` 纯协议探针。
- 使用 runtime spies 记录 service、method、请求字节、回调和消息事件。
- 从回调 envelope 的 `body` 保存 protobuf 字节或有界对象摘要。
- 优先使用运行时 `*pb.ts` codec 解码回包，再冻结经过证据确认的字段路径。
- 使用 VM 加载真实 `resources/wmpf/button.js`，按 TDD 实现解析、编码与编排。
- 实时验收查询、执行、成功跳过、失败停止以及零 UI 行为。

新 skill 不负责人工点击抓包。需要用户点击未知 UI 时继续使用现有 `farm-protocol-capture`。

## 强制规则

- 默认使用 `ws://127.0.0.1:34115/wails/ipc` 调用 `main.App.RunDiagnostic`。
- QQ runtime 业务链路为 `ws://127.0.0.1:8787/runtime/qqws`，不得使用旧 `/ws` 默认值。
- 查询回包不明确时停止，不扫描任意数字猜测业务 ID。
- 不使用历史 ID、抓包 ID、固定 ID 或 UI 模型作为协议失败回退。
- 原始回包只写入 `data/debug-captures/`，不得加入提交。
- 生产日志只保留受限摘要、字段路径、请求字节和失败原因。

## 可复用文件

项目内保留本次四个诊断脚本，移动到 `scripts/protocol-capture/`：

- `install-current-qq-patch.go`：用当前工作树脚本更新 QQ `game.js` 补丁。
- `svip-live-capture.cjs`：人工按钮请求的历史捕获示例。
- `svip-status-capture.cjs`：纯协议状态回包捕获示例。
- `svip-dynamic-live.cjs`：动态协议端到端验收示例。

Skill 本身包含：

- `SKILL.md`：选择流程、证据门槛、TDD 和验收步骤。
- `references/farm-go-runtime.md`：端点、Wails wire 格式、codec 模式和项目脚本说明。
- `agents/openai.yaml`：技能列表元数据。

## 项目清理

- 删除 `data/debug-captures/` 下现有 `.json`、`.log` 抓包产物。
- 脚本移动后删除该目录中的旧脚本副本。
- 将 `/data/debug-captures/` 加入 `.gitignore`，让后续抓包保持临时状态。
- 提交保留的脚本、忽略规则、设计与实施计划；个人 skill 目录不属于项目 Git 提交。

## 验收

- `quick_validate.py` 验证新 skill 成功。
- `agents/openai.yaml` 与 `SKILL.md` 名称和用途一致。
- 项目保留四个脚本且原始内容不丢失。
- `data/debug-captures/` 不再出现在 `git status`。
- 项目测试与 `git diff --check` 通过。
