# 工作台本次运行出售预估设计

## 目标

让工作台“本次运行统计”中的“出售预估收益”仅汇总当前运行开始后发生的出售记录；账户状态页的“运行统计”继续按原有日期范围汇总，不受影响。

## 现状

`App.FarmWorkspaceRunStatistics` 已用 `runStatisticsStartedAt` 过滤自动化事件，但出售金额通过 `todayWarehouseSellAmount` 按当天 `date_key` 汇总。因此同一账号当天、当前运行启动前的出售记录会被计入工作台。

账户状态页通过 `automationStatsHistoryForAccount` 调用按日期范围聚合的存储查询。这是历史统计行为，保留不变。

## 方案

在存储层新增按账号和起始时间汇总出售金额的查询，比较 `warehouse_sell_records.occurred_at >= runStatisticsStartedAt`。工作台改为调用该查询；存储不可用时沿用当前返回 `0, true` 的语义，查询失败时返回 `0, false`。

不修改 `RunStatistics` 的接口、前端展示组件或账户状态页的日期统计。保留“出售预估收益”现有标签和格式化方式。

## 验证

在工作台统计应用测试中创建同账号的三类出售记录：运行开始前但同一天、运行开始后、其他账号。断言只汇总运行开始后的记录。同时运行工作台前端测试，确认现有展示契约未变。
