# 仓库 UI 出售单价解析修复设计

## 目标

修复仓库 UI 模型中存在出售单价但被错误识别为不可出售的问题，已知复现场景为青梅 `itemId=41221`，运行时配置售价为 240。

## 根因

仓库数据有协议和 UI 两条来源。协议背包路径通过运行时物品配置读取 `price`，能够得到青梅售价。UI 模型路径通过 `readWarehouseModelSaleUnitPrice` 读取售价，但当前实现只解析 `sells`、`saleRewards` 和 `sellRewards`。

仓库 UI 模型的售价还可能位于模型本身或 `show`、`_tempData`、`tempData`、`detail` 等嵌套对象的 `saleUnitPrice`、`sellPrice`、`price` 字段。现有 `readWarehouseModelNumber` 已能遍历这些嵌套对象，但售价读取函数没有使用它，导致售价归零，`readWarehouseModelCanSell` 随后返回 `false`。

## 修改

- 保留 `saleRewards/sells` 的现有解析，并优先使用其首个奖励数量作为出售单价。
- 当奖励中没有有效单价时，通过 `readWarehouseModelNumber` 依次读取 `saleUnitPrice`、`sellPrice`、`price`。
- 出售货币 ID 同样在奖励缺失时读取 `saleCurrencyId`、`priceId`、`price_id`，保持售价和货币来源一致。
- 不使用 Go 本地 `ItemInfo.json` 作为仓库可出售判定兜底，避免掩盖运行时数据解析问题。
- 不放宽锁定判定；锁定物品继续不可出售。

## 验证

新增 Node VM 回归测试，加载真实 `resources/wmpf/button.js`，仅在测试内导出内部售价和可出售判定函数，并使用以下模型验证：

- `_tempData.price = 240` 的青梅模型返回单价 240。
- 同一模型被判定为可出售。
- 锁定的同一模型仍被判定为不可出售。
- 已有 `saleRewards` 仍优先于普通 `price` 字段。

最后运行该回归脚本、Go 全量测试和现有前端测试。
