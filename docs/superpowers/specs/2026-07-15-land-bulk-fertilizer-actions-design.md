# Land Bulk Fertilizer Actions Design

## Goal

在土地详情顶部动作区新增“一键无机肥”和“一键有机肥”，点击后对所有生长中地块施加对应肥料。

## User-Facing Behavior

- 土地详情顶部动作区显示：
  - 一键催熟
  - 一键无机肥
  - 一键有机肥
- “一键催熟”行为不变，仍打开催熟对话框。
- 点击“一键无机肥 / 一键有机肥”后：
  - 筛选当前土地列表中所有生长中地块
  - 直接提交批量施肥
  - 不打开对话框
  - 不联动收获
- 没有生长中地块、好友农场、运行时未迁移、或正在处理其他土地动作时，按钮禁用。
- 执行中显示“无机施肥中 / 有机施肥中”，并阻止重复点击。

## Growing Land Rule

地块计入一键施肥目标，当且仅当：

- 有有效 `landId`
- 不是 `mature` / `dead` / `empty` / `locked`
- `canHarvest` 为假
- 状态为 `growing` 或 `unknown`
- 存在植物信息（`displayPlantName` / `plantName` / `seedId`），或状态明确为 `growing`

这比“一键催熟”更宽：不受成熟阈值限制，覆盖所有仍在生长的地块。

## Runtime Flow

前端复用已有 `FarmLandRush`：

```json
{
  "landIds": [1, 2, 4],
  "fertilizerMode": "normal",
  "harvestLinkEnabled": false,
  "continuousRushEnabled": false,
  "rushThresholdSec": 2592000
}
```

说明：

- `fertilizerMode` 为 `normal` 或 `organic`
- `harvestLinkEnabled` 固定为 `false`，避免误触发收获/清理
- `rushThresholdSec` 使用足够大的阈值，避免后端/运行时按催熟阈值二次过滤掉远成熟地块
- 成功后刷新土地详情

后端仅扩展土地详情 `actions` 列表：

- gate：`rush` / `fertilize_normal` / `fertilize_organic`，全部 `enabled: false`
- runtime：同上，全部 `enabled: true`

不新增 Wails API。

## Error Handling

- 无生长中地块：提示“没有生长中的地块可施肥”
- 好友农场：提示“好友农场不支持一键施肥”
- 运行时返回失败：在土地详情错误区展示错误信息
- 后台刷新失败时，已有土地卡片继续保留

## Testing

- Go：土地 gate / runtime actions 包含三个动作
- Frontend：渲染“一键无机肥 / 一键有机肥”
- Frontend：源码接线检查 `submitBulkFertilizer`、`isGrowingLandForFertilizer`、`FarmLandRush` 且关闭联动收获
