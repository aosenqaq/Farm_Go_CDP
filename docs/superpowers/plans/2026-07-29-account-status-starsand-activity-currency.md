# 星砂活动货币 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在账户状态页将本期活动货币从荷露替换为星砂，并使用给定的星砂图标。

**Architecture:** 保留 `AccountStatusView` 的既有活动货币状态模型和卡片布局。仅将仓库精确名称匹配、可用状态下的展示文本和静态图片 URL 切换为星砂；现有的不可用分支继续负责缺失、无效和非运行时仓库数据。

**Tech Stack:** React 18、TypeScript、Vite、Vitest、PowerShell

---

### Task 1: 覆盖星砂仓库匹配与卡片呈现

**Files:**
- Modify: `frontend/src/views/AccountStatusView.test.tsx:252-296`

- [ ] **Step 1: 先写失败测试**

在 `renders runtime account profile, level progress, fertilizer container, and activity currency` 的 `initialWarehouse` 夹具中，把活动物品改为星砂，并把断言更新为：

```tsx
initialWarehouse={{
  status: 'runtime',
  items: [{ name: '星砂', count: 5225, categoryLabel: '道具' }],
}}

expect(html).toContain('星砂');
expect(html).toContain('src="/items/starsand.png"');
expect(html).toContain('alt="星砂"');
expect(html).not.toContain('荷露');
```

将不可用测试重命名为 `does not treat the previous activity currency as current`，并让仓库仅返回旧物品，以确保它不能被当作星砂：

```tsx
initialWarehouse={{
  status: 'runtime',
  items: [{ name: '荷露', count: 1, categoryLabel: '道具' }],
}}
```

保留对 `暂未获取` 和不存在 `仓库实时数量` 的断言。

- [ ] **Step 2: 运行测试并确认预期失败**

Run: `npm --prefix frontend test -- src/views/AccountStatusView.test.tsx`

Expected: 两项活动货币断言失败，因为当前实现只匹配并渲染荷露。

### Task 2: 最小替换星砂实现与静态资源

**Files:**
- Create: `frontend/public/items/starsand.png`
- Modify: `frontend/src/views/AccountStatusView.tsx:138,171,356-357,482,486-492`

- [ ] **Step 1: 复制用户提供的图片资源**

Run:

```powershell
Copy-Item -LiteralPath 'C:\Users\奥森\AppData\Roaming\QQEX\miniapp\temps\miniapp_src\1112386029_3_299cdabac04ffa90c15a88aa6676ed53\fetched-resources\新增资源整理\道具\道具图标\1023_星砂\图标\1023_星砂_33e67f42-a66a-4793-b426-7a434617db75.png' -Destination 'frontend\public\items\starsand.png'
```

- [ ] **Step 2: 写入最小实现**

将三个调用点与函数名从 `resolveLotusDew` 统一改为 `resolveStarSand`。在可用卡片中使用以下标记：

```tsx
<img className="activity-currency-mark" src="/items/starsand.png" alt="星砂" />
<div className="activity-currency-copy"><strong>星砂</strong><span>{state.categoryLabel} · 仓库实时数量</span></div>
```

将解析器中的精确名称匹配改为：

```tsx
function resolveStarSand(payload?: WarehousePayloadLike): ActivityCurrencyState {
  if (payload?.status !== 'runtime') return { state: 'unavailable' };
  const item = payload.items?.find((candidate) => candidate.name === '星砂');
  const count = Number(item?.count);
  if (!item || !Number.isFinite(count) || count < 0) return { state: 'unavailable' };
  return { state: 'available', count, categoryLabel: item.categoryLabel || '道具' };
}
```

- [ ] **Step 3: 运行活动货币测试并确认通过**

Run: `npm --prefix frontend test -- src/views/AccountStatusView.test.tsx`

Expected: `AccountStatusView` 测试文件全部通过；星砂夹具显示名称、图片路径和数量，旧荷露夹具显示不可用状态。

### Task 3: 构建并提交最小变更

**Files:**
- Create: `frontend/public/items/starsand.png`
- Modify: `frontend/src/views/AccountStatusView.tsx`
- Modify: `frontend/src/views/AccountStatusView.test.tsx`

- [ ] **Step 1: 验证复制的图标与来源一致**

Run:

```powershell
Get-FileHash -Algorithm SHA256 -LiteralPath 'frontend\public\items\starsand.png'
```

Expected: SHA256 为 `D8CDBB1C40E24592AEC95AB4DF023706824CBAFE9A06A65A0D42BD7F82DB7584`。

- [ ] **Step 2: 运行前端生产构建**

Run: `npm --prefix frontend run build`

Expected: TypeScript 检查和 Vite 构建以退出码 0 完成。

- [ ] **Step 3: 检查变更范围**

Run: `git diff --check -- frontend/src/views/AccountStatusView.tsx frontend/src/views/AccountStatusView.test.tsx && git status --short -- frontend/public/items/starsand.png frontend/src/views/AccountStatusView.tsx frontend/src/views/AccountStatusView.test.tsx`

Expected: 无空白错误；仅有星砂 PNG、账户状态视图与其测试的未提交产品变更。

- [ ] **Step 4: 提交实现**

Run:

```powershell
git add -- 'frontend/public/items/starsand.png' 'frontend/src/views/AccountStatusView.tsx' 'frontend/src/views/AccountStatusView.test.tsx'
git commit -m 'feat: show starsand activity currency'
```
