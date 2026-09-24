# 应用宝注入弹窗启动小程序修复设计

## 目标

修复“应用宝 Frida 注入”弹窗中的“启动小程序”按钮，使其在应用宝已运行或未运行时都直接打开应用宝内的“QQ 经典农场”，而不是仅唤起应用宝主界面或静默结束。

## 根因

弹窗统一调用 Wails 方法 `LaunchHostProcess()`。该方法为应用宝构造的 QQ 农场快捷启动参数正确，但启动请求始终使用固定目录 `D:\Program Files\Tencent\Androws\Application`，且只把 `AndrowsLauncher.exe` 作为相对目标传给 Windows。

当应用宝安装在其他目录时，手动启动入口既不利用当前应用宝进程的可执行路径，也不检查启动器是否存在，因此无法稳定执行真正的小程序快捷启动动作。

## 范围

本次修改仅覆盖应用宝 `yyb_cdp` 的手动启动请求：

- 保持注入弹窗布局、按钮文案和加载状态不变。
- 保持 QQ 农场的应用宝包名、伪协议和启动参数不变。
- 保持 QQ `qq_ws` 与微信 `wechat_cdp` 的协议启动行为不变。
- 不修改重启链路、Frida 注入流程或应用宝授权弹窗处理。

## 方案

### 启动目标选择

`LaunchHostProcess()` 继续根据当前运行时链路选择平台。应用宝分支在派发启动前读取主机进程快照，并交给 guard 包解析实际启动请求。

解析器按以下顺序查找应用宝启动器：

1. 从正在运行的 Androws/WMPF 相关进程可执行路径反推同一安装根下的 `Application` 目录。
2. 检查 Windows 常见安装根，包括 `ProgramFiles`、`ProgramFiles(x86)`、`LOCALAPPDATA` 和 `ProgramData` 下的 `Tencent\Androws\Application`。
3. 检查兼容的固定候选目录，包括 C/D 盘的 Program Files、Program Files (x86) 和根级 `Androws\Application`。

候选目录只有在其中存在 `AndrowsLauncher.exe` 时才有效。成功后，启动请求同时携带启动器绝对路径和工作目录，避免依赖 Windows 对相对可执行文件名的搜索行为。

### 启动动作

解析成功后继续使用现有参数启动 `AndrowsLauncher.exe`：

- `--launch-pkg-name` 指向 QQ 经典农场应用包。
- `--launch-proc-name` 指向 `AndrowsStore.exe`。
- `--pseudo-protocol` 使用 `androws://client/launchWithShortcut` 打开对应小程序。

同一动作适用于应用宝已运行和未运行状态；不根据主界面进程状态降级为仅打开应用宝。

## 错误处理

如果进程路径和所有常见目录中都找不到 `AndrowsLauncher.exe`，后端返回 `launch_failed`，原因明确说明未找到应用宝安装目录。此时不调用 Windows 启动 API，也不返回 `launch_dispatched`。

进程快照读取失败时同样返回 `launch_failed`，保留底层错误，避免用固定错误目录继续尝试并产生误导结果。

## 测试

先添加失败测试，再实现最小修复：

- 从 `Tencent\Androws\WmpfRuntime` 进程路径解析 `Application\AndrowsLauncher.exe`。
- 从根级 `Androws\WmpfRuntime` 进程路径解析启动器。
- 没有运行进程时从常见安装目录找到启动器。
- 所有候选均不存在时返回明确错误。
- `LaunchHostProcess()` 在 `yyb_cdp` 下派发绝对启动器路径、正确工作目录和完整 QQ 农场参数。
- QQ 和微信仍分别使用原有协议请求。

完成后运行 guard 包测试、应用层相关 Go 测试、完整 Go 测试，以及前端现有测试和生产构建。

## 验收

- 应用宝未运行时点击按钮，启动应用宝并进入 QQ 经典农场。
- 应用宝已运行但小程序未打开时点击按钮，直接进入 QQ 经典农场。
- 非默认安装目录可通过运行进程路径或常见目录解析。
- 找不到安装目录时返回失败，不误报已派发。
- QQ 与微信链路启动行为无回归。
