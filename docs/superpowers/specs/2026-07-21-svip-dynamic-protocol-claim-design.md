# SVIP 礼包动态协议领取设计

## 目标

将 `gameCtl.claimSvipDailyGift` 从固定礼包 ID 请求改为纯协议动态流程：先调用 `QQVipService.GetQQVipRewardsStatus` 获取当前可领取礼包 ID，再编码并调用 `QQVipService.ClaimQQVipRewards`。

整个流程必须在后台完成，不打开、点击、关闭或依赖 `QQVIPGiftUI`。礼包 ID 变化后不需要再次修改代码。

## 现状与证据

当前实现固定发送 `0a 02 01 02`，等价于 protobuf 字段 1 中的 packed ID 列表 `[1, 2]`。

2026-07-21 的 QQ WS 运行时捕获证明：

- 打开礼包界面时，游戏先调用 `gamepb.qqvippb.QQVipService.RefreshVipInfo`，随后调用 `GetQQVipRewardsStatus`。
- 当前版本实际领取请求为 `0a 03 01 04 03`，字段 1 的 ID 列表为 `[1, 4, 3]`。
- 同一时间窗出现 `GET_QQ_REWARDS`、`ITEM_UPDATE` 和皮肤入库事件，确认该请求领取成功。

捕获到的 `[1, 4, 3]` 只作为回归证据，不进入生产代码。

## 范围

本次修改只调整 `resources/wmpf/button.js` 内的 SVIP 每日礼包协议实现及对应测试：

- 保持 Go 自动化任务 ID、调度配置、调用方法名和前端行为不变。
- 保持 `gameCtl.claimSvipDailyGift(opts)` 对外入口不变。
- 保持月卡、商城礼包、任务奖励及其他协议领取逻辑不变。
- 不增加 UI 自动化回退，不读取混淆后的 QQVIP 私有状态模型。
- 不使用固定 ID、历史 ID 或猜测 ID 作为查询失败后的回退。

## 协议流程

### 1. 查询状态

调用：

- Service: `gamepb.qqvippb.QQVipService`
- Method: `GetQQVipRewardsStatus`
- Request: 空字节数组

查询封装负责捕获回调参数、协议失败信息和超时。实现前先通过只读诊断调用保存当前版本的真实回调结构，随后解析器只接受该结构中经证据确认的字段路径，不进行全对象数字扫描。

### 2. 提取礼包 ID

状态解析器输出规范化结果：

- `claimIds`: 去重后的正整数数组，保持服务端返回顺序。
- `alreadyClaimed`: 服务端明确表示今日已领取。
- `reason`: 查询失败、响应结构不合法或没有可领取奖励时的原因。

解析规则：

- 只读取已确认的礼包 ID 字段，不把道具 ID、皮肤 ID、状态码或时间戳当作礼包 ID。
- 拒绝非整数、零、负数和超出 protobuf uint32 范围的值。
- 响应结构缺失或出现相互冲突的候选字段时返回失败，不发送领取请求。
- 服务端明确表示已领取，或合法响应中的可领取列表为空时，返回成功跳过。

### 3. 编码领取请求

`ClaimQQVipRewards` 的请求使用 protobuf 字段 1 的 packed repeated uint32 编码：

1. 编码每个礼包 ID 的 uint32 varint。
2. 拼接所有 ID varint 得到 packed payload。
3. 写入字段标签 `0x0a`。
4. 使用 varint 写入 packed payload 长度。
5. 追加 packed payload。

编码器不能假设 ID 或 payload 长度小于 128，确保后续较大礼包 ID 仍可工作。

### 4. 领取与结果判定

只有查询成功且 `claimIds` 非空时才调用：

- Service: `gamepb.qqvippb.QQVipService`
- Method: `ClaimQQVipRewards`
- Request: 本次查询结果动态编码的字节数组

成功判定继续使用现有证据，包括领取回调、匹配的发送事件和 `ITEM_UPDATE`。协议返回“今日已领取”时按成功跳过处理。

返回结果增加本次查询和编码证据：

- `claimIds`
- `statusMethodName`
- `statusCallbackCalled`
- `requestBytes`
- `requestHex`

不在普通日志中输出完整状态响应，避免把无关账户数据带入日志。

## 错误处理

- 状态查询超时、回调错误或响应结构不合法：返回 `ok: false`，不调用领取协议，也不记录当天完成。
- 服务端明确已领取：返回 `ok: true`、`skipped: true`、`reason: already_claimed`，记录当天完成。
- 合法状态响应没有可领取 ID：返回 `ok: true`、`skipped: true`、`reason: no_claimable_rewards`，记录当天完成。
- ID 编码失败：返回 `ok: false`，不发送领取请求。
- 领取协议失败：保持现有失败原因提取和错误返回行为。

## 测试

先添加失败测试，再实现最小修改。测试通过 VM 加载真实 `button.js`，使用可控的 `netWebSocket.sendMsg` 回调，不复制生产实现。

覆盖以下行为：

- 先发送空的 `GetQQVipRewardsStatus`，再使用响应中的 ID 发送 `ClaimQQVipRewards`。
- 当前捕获 ID `[1, 4, 3]` 编码为 `0a 03 01 04 03`。
- 大于 127 的 ID 和 packed payload 长度使用正确 varint 编码。
- 重复 ID 被去重且保持服务端顺序。
- 已领取和空列表返回成功跳过，不发送领取请求。
- 缺失字段、冲突字段、非法 ID、查询错误和查询超时均失败且不领取。
- 领取回调、`ITEM_UPDATE` 和协议失败原因继续按现有规则判定。
- Go 自动化 facade 仍以相同方法和参数调用 `gameCtl.claimSvipDailyGift`。

## 验收

- 运行 SVIP 自动领取时不出现或操作任何游戏 UI。
- 运行时先产生 `GetQQVipRewardsStatus`，后产生使用本次返回 ID 的 `ClaimQQVipRewards`。
- 修改服务端礼包 ID 后无需更新客户端常量。
- 查询失败或响应不明确时不会发送旧 ID、当前捕获 ID 或任何猜测 ID。
- 已领取或无可领取奖励按成功跳过处理，并保持现有每日完成状态语义。
- 其他自动化领取任务无回归。
