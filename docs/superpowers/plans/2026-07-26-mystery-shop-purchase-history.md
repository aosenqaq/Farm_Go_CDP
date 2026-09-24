# 神秘商店购买记录 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为神秘商店自动购买补齐金豆豆，并保存、展示当前运行账号的成功购买记录。

**Architecture:** 新增由 `account_key` 隔离的 SQLite 表。运行时仅在购买协议确认成功后写入，应用层通过只读 Wails RPC 查询当前账号最近 50 条，前端在账号变化和购买结束时刷新记录。

**Tech Stack:** Go, SQLite, Wails, React, TypeScript, Vitest.

---

## 文件结构

- Create: `internal/storage/mystery_shop.go` 和 `internal/storage/mystery_shop_test.go`。
- Modify: `internal/storage/migrations.go`、`internal/farm/automation/runtime.go`、`internal/farm/automation/runtime_mystery_shop.go`、对应测试、`app.go`、`app_test.go`。
- Modify: `frontend/src/AuthorizedApp.tsx`、`frontend/src/views/FarmWorkspaceView.tsx`、`frontend/src/views/AutomationView.tsx`、对应测试和 `frontend/src/style.css`。
- Regenerate: `frontend/wailsjs/go/main/App.{js,d.ts}` 和 `frontend/wailsjs/go/models.ts`。

### Task 1: 持久化记录

**Files:**
- Create: `internal/storage/mystery_shop.go`
- Create: `internal/storage/mystery_shop_test.go`
- Modify: `internal/storage/migrations.go`

- [ ] **Step 1: 写入失败测试**

```go
func TestMysteryShopPurchaseRecordsAreScopedSortedAndLimited(t *testing.T) {
    store, _ := Open(context.Background(), t.TempDir())
    defer store.Close()
    for i := 0; i < 51; i++ {
        err := store.AppendMysteryShopPurchaseRecord(context.Background(), "gid:10001", MysteryShopPurchaseRecord{
            ID: fmt.Sprintf("a-%02d", i), OccurredAt: time.Date(2026, 7, 26, 10, i, 0, 0, time.UTC).Format(time.RFC3339Nano),
            ItemName: "高级化肥", Count: 2, UnitPrice: 80, CurrencyID: 1005, CurrencyName: "金豆豆", Discount: 50,
        })
        if err != nil { t.Fatal(err) }
    }
    records, err := store.ListMysteryShopPurchaseRecords(context.Background(), "gid:10001", 50)
    if err != nil { t.Fatal(err) }
    if len(records) != 50 || records[0].ID != "a-50" || records[49].ID != "a-01" { t.Fatalf("records=%#v", records) }
}
```

- [ ] **Step 2: 验证 RED**

Run: `go test ./internal/storage -run TestMysteryShopPurchaseRecordsAreScopedSortedAndLimited -count=1`

Expected: FAIL because the record type and Store methods do not exist.

- [ ] **Step 3: 实现最小存储 API**

在 `Migrate` 增加 `mystery_shop_purchase_records` 表：主键为 `(account_key, id)`，列包含 `occurred_at, goods_id, item_id, item_name, count, unit_price, currency_id, currency_name, discount, payload_json`；增加 `(account_key, occurred_at DESC, id DESC)` 索引。在新文件定义：

```go
type MysteryShopPurchaseRecord struct {
    ID string `json:"id"`; OccurredAt string `json:"occurredAt"`
    GoodsID int `json:"goodsId"`; ItemID int `json:"itemId"`; ItemName string `json:"itemName"`
    Count int `json:"count"`; UnitPrice int `json:"unitPrice"`
    CurrencyID int `json:"currencyId"`; CurrencyName string `json:"currencyName"`; Discount int `json:"discount"`
    Payload map[string]any `json:"payload,omitempty"`
}
func (s *Store) AppendMysteryShopPurchaseRecord(ctx context.Context, accountKey string, record MysteryShopPurchaseRecord) error
func (s *Store) ListMysteryShopPurchaseRecords(ctx context.Context, accountKey string, limit int) ([]MysteryShopPurchaseRecord, error)
```

追加时使用 `NormalizeAccountKey` 和 upsert，空 ID/时间跳过；读取将上限限制到 50，按最新时间倒序，并反序列化可选 payload。

- [ ] **Step 4: 验证 GREEN 并提交**

Run: `go test ./internal/storage -run TestMysteryShopPurchaseRecordsAreScopedSortedAndLimited -count=1`

Expected: PASS.

Run: `git add internal/storage/migrations.go internal/storage/mystery_shop.go internal/storage/mystery_shop_test.go && git commit -m "feat: persist mystery shop purchase records"`

### Task 2: 成功购买才追加记录

**Files:**
- Modify: `internal/farm/automation/runtime.go`
- Modify: `internal/farm/automation/runtime_mystery_shop.go`
- Modify: `internal/farm/automation/runtime_mystery_shop_test.go`

- [ ] **Step 1: 写入运行时失败测试**

```go
// Extend the existing runtimeSocialStoreStub with accountKey, records, and this method.
func (s *runtimeSocialStoreStub) SaveMysteryShopPurchaseRecord(_ context.Context, key string, record MysteryShopPurchaseRecord) error {
    s.accountKeys = append(s.accountKeys, key)
    s.mysteryShopPurchaseRecords = append(s.mysteryShopPurchaseRecords, record)
    return s.mysteryShopPurchaseRecordErr
}

func TestRuntimeFacadeMysteryShopAutoBuyRecordsSuccessfulPurchase(t *testing.T) {
    store := &runtimeSocialStoreStub{}
    result := NewRuntimeFacadeWithConfigAndSocialStore(caller, nil, store, "gid:10001").RunTask(context.Background(), "mystery_shop_auto_buy")
    if !result.OK || len(store.mysteryShopPurchaseRecords) != 1 { t.Fatalf("result=%#v records=%#v", result, store) }
    got := store.mysteryShopPurchaseRecords[0]
    if got.ItemName != "高级化肥" || got.Count != 2 || got.UnitPrice != 80 || got.CurrencyID != 1005 || got.CurrencyName != "金豆豆" || got.Discount != 50 { t.Fatalf("record=%#v", got) }
}
```

在“无匹配”和“购买返回 `ok:false`”测试加入 `len(recorder.records) == 0` 断言。

- [ ] **Step 2: 验证 RED**

Run: `go test ./internal/farm/automation -run TestRuntimeFacadeMysteryShopAutoBuy -count=1`

Expected: FAIL because runtime has no purchase-record writer.

- [ ] **Step 3: 实现 writer 与成功路径**

在 `runtime.go` 增加运行时模型和接口：

```go
type MysteryShopPurchaseRecord struct {
    ID string; OccurredAt time.Time
    GoodsID, ItemID, Count, UnitPrice, CurrencyID, Discount int
    ItemName, CurrencyName string; Payload map[string]any
}
type MysteryShopPurchaseRecordWriter interface {
    SaveMysteryShopPurchaseRecord(context.Context, string, MysteryShopPurchaseRecord) error
}
```

扩展 `RuntimeSocialStore` 并由现有构造函数注入 writer。`runMysteryShopAutoBuy` 在购买协议成功后，从选中卡片读取 `itemId`、数量、单价、货币、折扣，用 `mysteryShopCardName` 和 `mysteryShopCurrencyName` 填补名称，使用 `mystery_shop:<UnixNano>` 和 `time.Now()` 生成记录。writer 为 nil 时跳过。writer 失败保持购买成功结果，并在 `ActionResult.Warnings` 追加 `购买记录写入失败：<error>`。

- [ ] **Step 4: 验证 GREEN 并提交**

Run: `go test ./internal/farm/automation -run TestRuntimeFacadeMysteryShopAutoBuy -count=1`

Expected: PASS.

Run: `git add internal/farm/automation/runtime.go internal/farm/automation/runtime_mystery_shop.go internal/farm/automation/runtime_mystery_shop_test.go && git commit -m "feat: record successful mystery shop purchases"`

### Task 3: 当前账号只读 RPC

**Files:**
- Modify: `app.go`, `app_test.go`
- Regenerate: `frontend/wailsjs/go/main/App.{js,d.ts}`, `frontend/wailsjs/go/models.ts`

- [ ] **Step 1: 写入失败测试**

```go
func TestFarmMysteryShopPurchaseRecordsUseCurrentAccount(t *testing.T) {
    app, _ := newAppWithFakeRuntime(t, nil)
    mustAppendMysteryRecord(t, app.store, "gid:10001", "first")
    mustAppendMysteryRecord(t, app.store, "gid:10002", "other")
    app.setRuntimeAccountForTest("gid:10001")
    records := app.FarmMysteryShopPurchaseRecords()
    if len(records) != 1 || records[0].ID != "first" { t.Fatalf("records=%#v", records) }
}
```

- [ ] **Step 2: 验证 RED**

Run: `go test . -run TestFarmMysteryShopPurchaseRecordsUseCurrentAccount -count=1`

Expected: FAIL with missing RPC.

- [ ] **Step 3: 实现 RPC 与存储适配器**

在 `app.go` 添加：

```go
func (a *App) FarmMysteryShopPurchaseRecords() []storage.MysteryShopPurchaseRecord {
    if a.requireAuthorized("FarmMysteryShopPurchaseRecords") != nil || a.store == nil { return []storage.MysteryShopPurchaseRecord{} }
    records, err := a.store.ListMysteryShopPurchaseRecords(a.contextOrBackground(), a.accountKey(), 50)
    if err != nil { a.lastErr = err; return []storage.MysteryShopPurchaseRecord{} }
    return records
}
```

让 `socialStorageAdapter` 将运行时记录转为 `storage.MysteryShopPurchaseRecord`。`ActionResult` 增加 `Warnings []string` JSON 字段；App 在神秘商店购买成功且 warnings 非空时写入 `auto_farm.mystery_shop_purchase_record.write` 运行事件，不降低购买结果。执行 `wails generate module` 更新前端 bindings。

- [ ] **Step 4: 验证 GREEN 并提交**

Run: `wails generate module; go test . -run TestFarmMysteryShopPurchaseRecordsUseCurrentAccount -count=1`

Expected: bindings include `FarmMysteryShopPurchaseRecords`; test PASS.

Run: `git add app.go app_test.go frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/models.ts && git commit -m "feat: expose mystery shop history by account"`

### Task 4: UI、金豆豆和历史列表

**Files:**
- Modify: `frontend/src/AuthorizedApp.tsx`, `frontend/src/views/FarmWorkspaceView.tsx`
- Modify: `frontend/src/views/AutomationView.tsx`, `frontend/src/views/AutomationView.test.tsx`, `frontend/src/style.css`

- [ ] **Step 1: 写入失败测试**

```tsx
const records = [{ id: 'shop-1', occurredAt: '2026-07-26T19:42:00Z', itemName: '高级化肥', count: 2, unitPrice: 80, currencyId: 1005, currencyName: '金豆豆', discount: 50 }];
const html = renderToStaticMarkup(<AutomationView state={state} mysteryShopPurchaseRecords={records} onRunTask={() => undefined} initialSettingsGroupId="mystery_shop" />);
expect(html).toContain('金豆豆');
expect(html).toContain('购买记录');
expect(html).toContain('高级化肥 x2');
expect(html).toContain('80 金豆豆');
expect(html).toContain('5折');
```

以空数组渲染并断言“暂无成功购买记录”。

- [ ] **Step 2: 验证 RED**

Run: `pnpm --dir frontend test -- AutomationView.test.tsx`

Expected: FAIL because the records prop and history markup do not exist.

- [ ] **Step 3: 实现刷新和界面**

`AuthorizedApp.tsx` 导入 RPC 并使用现有账号作用域 token 刷新记录；在账号变化、自动化页面加载和 `mystery_shop_auto_buy` 完成后调用。通过 `FarmWorkspaceView` 传给 `AutomationView`。

在 `AutomationView.tsx` 将货币固定为：

```ts
const mysteryCurrencies = [[1001, '金币'], [1002, '点券'], [1004, '钻石'], [1005, '金豆豆']] as const;
```

紧随四个货币复选项渲染购买记录区：桌面显示时间、商品数量、支付信息；价格为 `${unitPrice} ${currencyName || `货币${currencyId}`}`；仅在 `0 < discount < 100` 显示折扣。样式用 `.automation-mystery-history` 系列类添加分隔线，不引入嵌套卡片；小屏改为单列并允许商品换行。

- [ ] **Step 4: 验证 GREEN、构建并提交**

Run: `pnpm --dir frontend test -- AutomationView.test.tsx`

Expected: PASS.

Run: `pnpm run frontend:build`

Expected: PASS and `public/app/index.html` references a fresh generated asset.

Run: `git add frontend/src/AuthorizedApp.tsx frontend/src/views/FarmWorkspaceView.tsx frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx frontend/src/style.css public/app && git commit -m "feat: show mystery shop purchase history"`

### Task 5: 完整验证

**Files:** none

- [ ] **Step 1: Go 回归**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 2: 前端回归和生成产物验证**

Run: `pnpm --dir frontend test; pnpm run frontend:build; git status --short`

Expected: tests and build PASS; work区只包含计划内的生成资源变化。
