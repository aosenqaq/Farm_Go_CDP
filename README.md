# Farm_Go

> **警告：该方式已经被证实会造成账户封禁。**
>
> 本项目仅供个人学习与技术研究，禁止二次打包并出售。二次打包出售者替作者挡灾，自行承担全部后果。
>
> 交流群号：540464203

Farm_Go 是运行在 Windows 上的桌面程序，使用 Wails、Go 和 React 构建。它连接本机的 QQ、微信或应用宝农场小程序，用于查看账户、土地、仓库和好友，并提供自动化、守护服务、运行日志和消息推送。

启动不再要求卡密。局域网 Web 控制台可在“系统设置”中单独开启：密码至少 12 位，默认端口 `8788`。局域网模式只接受本机和私有网段；安全隧道模式只监听 `127.0.0.1`。

## 环境

- Windows，并安装 WebView2 Runtime
- Go 1.25.5 或与 `go.mod` 兼容的版本
- Node.js 与 npm
- Wails CLI v2

Wails 一般位于 `%USERPROFILE%\go\bin`。调试和打包前，先把它加入当前终端的 PATH。

## 调试运行

在仓库根目录执行：

```powershell
$env:PATH = "$env:USERPROFILE\go\bin;$env:PATH"
wails dev
```

`wails.json` 会先执行 `npm install`，再执行 `npm run dev`。首次运行需要联网安装前端依赖。

## 打包 exe

在仓库根目录执行：

```powershell
$env:PATH = "$env:USERPROFILE\go\bin;$env:PATH"
powershell -ExecutionPolicy Bypass -File scripts\build.ps1
```

产物是 `build\bin\Farm_Go.exe`。

需要写入产品版本时，先设置版本号再打包：

```powershell
$env:FARM_GO_VERSION_NAME = "1.0.0"
powershell -ExecutionPolicy Bypass -File scripts\build.ps1
```

脚本会先生成资源包，然后执行 `wails build -clean -trimpath`。

## 测试

```powershell
go test ./...
cd frontend
npm test
```

## 目录

- `desktop`：桌面程序主体。
- `internal`：农场逻辑、运行时和本地存储。
- `frontend`：界面。
- `resources`：小程序脚本和游戏配置。
- `scripts`：打包与检查脚本。
