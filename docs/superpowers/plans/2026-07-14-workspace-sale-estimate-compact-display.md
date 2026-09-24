# 工作台出售预估收益紧凑显示 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让工作台出售预估收益以中文紧凑金额显示，并在悬停时提供完整数值。

**Architecture:** 在 `OverviewView` 内保留计数指标既有的 `formatMetricNumber`，新增专用于收益的纯格式化函数。出售收益卡片接收格式化显示值和完整数值的原生 `title`，因此不引入新的状态、接口或样式。

**Tech Stack:** React 18、TypeScript、Vitest、React server rendering。

---

### Task 1: 收益格式化和悬停提示

**Files:**
- Modify: `frontend/src/views/OverviewView.tsx:201-224`
- Modify: `frontend/src/views/FarmWorkspaceView.test.tsx:18-94`

- [ ] **Step 1: 写入失败测试**

```tsx
const html = renderToStaticMarkup(
  <FarmWorkspaceView {...workspaceProps} runStatistics={{ ...runStatistics, saleEstimate: 1_234_567 }} />,
);

expect(html).toContain('123.46万');
expect(html).toContain('title="1,234,567"');
```

另写金额为 `100_000_000` 的渲染断言，期望包含 `1亿`；金额为 `12_000` 时期望包含 `1.2万`，以覆盖尾随零裁剪。

- [ ] **Step 2: 验证测试失败**

Run: `npm test -- FarmWorkspaceView.test.tsx`

Expected: FAIL，因为现有代码仍将 `1_234_567` 渲染成 `1,234,567`，且没有 `title` 属性。

- [ ] **Step 3: 最小实现**

```tsx
const saleEstimate = formatSaleEstimate(runStatistics?.saleEstimate);
<WorkbenchMetric label="出售预估收益" value={saleEstimate.display} title={saleEstimate.title} />
```

`formatSaleEstimate` 对小于 `10_000` 的有限数值使用 `formatMetricNumber`；对小于 `100_000_000` 的数值以 `10_000` 转换为 `万`；其他数值以 `100_000_000` 转换为 `亿`。统一使用最多两位小数并移除尾随零，`title` 始终是完整千位分隔整数。`WorkbenchMetric` 仅在提供 `title` 时把它加到数值元素。

- [ ] **Step 4: 验证测试通过**

Run: `npm test -- FarmWorkspaceView.test.tsx`

Expected: PASS。

- [ ] **Step 5: 完整验证**

Run: `npm test`

Expected: PASS。

Run: `npm run build`

Expected: exit 0。

- [ ] **Step 6: 提交实现**

```bash
git add frontend/src/views/OverviewView.tsx frontend/src/views/FarmWorkspaceView.test.tsx docs/superpowers/plans/2026-07-14-workspace-sale-estimate-compact-display.md
git commit -m "feat: compact workspace sale estimate"
```
