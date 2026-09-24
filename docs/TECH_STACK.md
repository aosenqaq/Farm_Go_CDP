# Farm_Go 技术决策文档

## 1. 项目定位

Farm_Go 是一个重新起步的 Windows 本机桌面工具。第一阶段目标不是迁移真实农场业务，而是做出稳定、可观察、可诊断的 QQ WS 运行时连接底座。

第一阶段当前决策：

- 使用 Wails v2 构建 Windows 桌面应用。
- 后端核心使用 Go。
- 前端使用 React、TypeScript、Vite、Tailwind CSS 风格和 `lucide-react` 图标。
- 本阶段不提供浏览器 WebUI，也不提供远程访问入口。
- 默认只监听 `127.0.0.1`。
- 数据存储第一版直接使用 SQLite。
- 第一版不迁移真实农场任务，只跑通 QQ WS 调试链路。

当前目录已经包含 Wails 工程骨架、Go 后端包、React 前端和第一阶段 UI 设计说明。

## 2. 第一阶段验收标准

必须完成：

- `Farm_Go.exe` 启动后打开 Wails 桌面窗口。
- 首次启动生成本地 SQLite 数据库。
- QQ WS adapter 监听 `127.0.0.1:8787/runtime/qqws`。
- UI 能显示运行时状态、连接信息、最近事件和诊断结果。
- UI 提供 `总览`、`连接`、`诊断`、`设置` 四个页面。
- 调试面板能执行 `host.describe` 或 `gameCtl.probe`。
- 未连接 QQ host 时，诊断返回结构化错误。
- release build 生成 Windows exe。

第一阶段明确不做：

- 不迁移真实农场任务。
- 不实现 WMPF、Frida、CDP。
- 不做浏览器 WebUI。
- 不做 Tauri 桌面壳。
- 不使用 `agmmnn/tauri-controls`，因为它依赖 Tauri，不适用于 Wails。
- 不直接公网裸露服务端口。
- 不做复杂插件系统或调度系统。

## 3. 总体架构

```text
Wails Desktop Window
        |
        v
React UI <---- Wails Bindings ----> Go App Service
                                      |
                                      v
QQ WS Adapter -> Runtime Manager -> Diagnostics
                                      |
                                      v
                                  SQLite Storage
```

分层原则：

- `main.go` 创建 Wails 桌面窗口并绑定 root `App`。
- root `app.go` 负责启动 SQLite、QQ WS adapter，并暴露 Wails 方法。
- `internal/app` 是 UI 可调用的应用服务层。
- `internal/runtime` 管理运行时状态。
- `internal/runtime/qqws` 管理 QQ host WebSocket 监听和基础握手。
- `internal/diagnostics` 承载 `host.describe`、`gameCtl.probe` 调用入口。
- `internal/storage` 管理 SQLite 连接和迁移。
- `frontend/src` 实现桌面 UI。

## 4. 技术选型结论

| 模块 | 选择 | 说明 |
| --- | --- | --- |
| 桌面壳 | Wails v2 | 单 exe 友好，Go 与 Web 前端桥接直接 |
| 后端语言 | Go | Windows 分发友好，并发和网络服务简单 |
| 前端 | React + TypeScript + Vite | 适合快速构建现代桌面 UI |
| 样式 | Tailwind CSS 风格 + 普通 CSS | 第一版使用集中 CSS，保留 Tailwind 构建能力 |
| 图标 | `lucide-react` | 轻量、适合工具型界面 |
| WebSocket | `github.com/gorilla/websocket` | QQ host 接入 |
| 数据库 | SQLite | 第一版结构化存储 |
| SQLite 驱动 | `modernc.org/sqlite` | 纯 Go，符合单 exe 目标 |
| 日志 | `log/slog` | 标准库结构化日志 |
| Tauri | 不使用 | 第一阶段已明确选 Wails |

## 5. UI 方向

第一阶段 UI 已确认：

- 精致桌面诊断站。
- 状态优先首页。
- 全中文界面。
- `QQ WS`、`host.describe`、`gameCtl.probe` 等协议标识保留原文。

详细说明见：

```text
docs/superpowers/specs/2026-07-06-wails-phase1-ui-design.md
```

## 6. 数据与运行时

SQLite 数据库位置：

```text
%APPDATA%\Farm_Go\data\farm_go.db
```

第一版表：

- `schema_migrations`
- `settings`
- `runtime_events`
- `diagnostic_runs`

运行时状态：

- `idle`
- `listening`
- `handshaking`
- `ready`
- `disconnected`
- `error`

## 7. 构建与验证

常用命令：

```powershell
go test ./...
cd frontend
npm test
npm run build
cd ..
powershell -ExecutionPolicy Bypass -File scripts\build.ps1
```

release exe 产物：

```text
build\bin\Farm_Go.exe
```
