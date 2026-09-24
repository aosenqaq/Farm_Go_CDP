# 自动施肥主开关一致性设计

## 目标

让施肥功能卡片的“自动施肥”总开关、`autoFarmFertilizerEnabled` 配置键和调度中心的 `own_fertilizer` 开关始终一致。种植施肥与催熟施肥作为两个子策略，只有总开关开启时才允许执行。

## 根因

当前系统把三个相关状态分别保存：

- 施肥功能卡片使用 `autoFarmFeatureGroupEnabled.fertilizer`。
- 自动施肥兼容配置使用 `autoFarmFertilizerEnabled`。
- 调度中心使用 `own_fertilizer.enabled`。

三者没有统一，数据库因此能保存“总开关开启、催熟模式已选择、调度任务关闭”。种植施肥又直接挂在 `own_plant` 后面，只读取 `autoFarmPlantFertilizerMode`，所以它可以在调度中心自动施肥关闭时继续运行，造成用户看到两个施肥功能行为相反。

## 状态模型

### 主开关

施肥功能卡片总开关是自动施肥功能的唯一业务主开关。以下状态必须同步：

- `autoFarmFeatureGroupEnabled.fertilizer`
- `autoFarmFertilizerEnabled`
- 持久化的 `own_fertilizer.enabled`
- 调度中心显示的 `own_fertilizer.enabled`

任意入口修改主开关后，其余状态必须在当前界面立即同步，并在保存、加载和调度器热更新后保持一致。

### 子策略

- `autoFarmPlantFertilizerMode` 仅决定成功种植后是否使用普通或有机化肥。
- `autoFarmRushFertilizerMode` 仅决定是否催熟以及催熟使用的肥料。
- 两个模式都允许为 `none`，且不反向关闭主开关。
- 主开关开启、催熟策略为 `none` 时，调度中心“自动施肥”仍显示开启；任务运行后按现有逻辑快速返回“催熟策略已关闭”。
- 主开关关闭时保留两个子策略的选项值，重新开启后恢复原选择。

自动填充肥料仍保留自己的子功能开关；施肥功能组关闭时它同样受上级功能组门控，但本次不改变其持久化偏好。

## 兼容与冲突处理

加载旧配置时按以下顺序确定主开关：

1. 如果存在显式的 `autoFarmFeatureGroupEnabled.fertilizer`，以用户可见的施肥总开关为准。
2. 如果缺少功能组键，使用 `autoFarmFertilizerEnabled`。
3. 如果两个配置键都缺失，使用持久化的 `own_fertilizer.enabled`。
4. 将确定后的值回写到三个持久化状态。

因此客户数据库中的 `featureGroup=true + autoFarmFertilizerEnabled=false + own_fertilizer=false` 会自动修复为全部开启。

## 前端行为

- 点击施肥功能卡片总开关时，同步功能组键、自动施肥配置键和调度任务开关。
- 点击调度中心“自动施肥”开关时，反向同步施肥功能卡片总开关和自动施肥配置键。
- 修改种植施肥策略或催熟策略时，只更新各自模式，不改变总开关。
- 通用调度任务开关在修改 `enabled` 时同步对应的配置键，避免新值被旧配置覆盖。
- 不新增说明卡片、弹窗或页面布局；现有开关和选择器直接显示统一后的状态。

## 后端行为

- `StateFromSettings` 加载时执行主开关归一化，并按兼容优先级修复旧配置。
- `SettingsFromState` 保存前再次执行归一化，确保绑定调用或未来客户端不能持久化矛盾状态。
- 调度器配置与热更新使用归一化后的 `own_fertilizer.enabled`。
- `runOwnPlant` 的种植后施肥增加主开关门控；主开关关闭时即使种植施肥模式为普通或有机，也不调用 `fertilizeLandsBatch`。
- `runOwnFertilizer` 保留现有催熟模式、候选地块、阈值、协议调用和联动收获逻辑。

## 测试

- Go 测试覆盖旧配置三个状态的冲突组合，并验证显式功能组总开关优先。
- Go 测试验证保存后功能组键、自动施肥配置键和持久化任务开关完全一致。
- Go 测试验证主开关关闭时种植成功但不会执行种植施肥。
- Go 测试验证主开关开启时现有种植施肥行为保持不变。
- Go 测试验证主开关开启且催熟策略为 `none` 时，任务保持启用但运行结果为跳过。
- 前端测试验证功能卡片总开关与调度任务开关双向同步。
- 前端测试验证修改两种施肥策略不会改变总开关。
- 运行相关前端测试、Go 测试、前端生产构建，并确认 `public/app/index.html` 引用新产物。

## 非目标

- 不修改催熟候选地块的 5 秒下限或用户配置阈值。
- 不修改肥料协议、库存选择、自动填充肥料或联动收获算法。
- 不修改其他功能组与子任务保持偏好分离的现有规则。
- 不修改页面布局和视觉风格。
