package automation

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type mysteryShopPurchaseRecorder struct {
	accountKey string
	records    []MysteryShopPurchaseRecord
}

func (r *mysteryShopPurchaseRecorder) SaveMysteryShopPurchaseRecord(_ context.Context, accountKey string, record MysteryShopPurchaseRecord) error {
	r.accountKey = accountKey
	r.records = append(r.records, record)
	return nil
}

func TestRuntimeFacadeMysteryShopAutoBuyRecordsSuccessfulPurchase(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.requestMysteryMerchantCardsByProtocol": map[string]any{
			"ok": true,
			"cards": []any{map[string]any{
				"goodsId": float64(8801), "cardId": float64(7), "itemId": float64(31001), "itemName": "高级化肥",
				"count": float64(2), "unitPrice": float64(80), "currencyId": float64(1005), "currencyName": "金豆豆", "discount": float64(50),
			}},
		},
		"gameCtl.buyMysteryMerchantCardByProtocol": map[string]any{"ok": true, "success": true},
	}}
	recorder := &mysteryShopPurchaseRecorder{}
	facade := NewRuntimeFacadeWithConfig(caller, nil).WithMysteryShopPurchaseRecordWriter(recorder, "gid:10001")

	result := facade.RunTask(context.Background(), "mystery_shop_auto_buy")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("mystery_shop_auto_buy should report OK, got %#v", result)
	}
	if recorder.accountKey != "gid:10001" || len(recorder.records) != 1 {
		t.Fatalf("purchase record was not saved for current account: account=%q records=%#v", recorder.accountKey, recorder.records)
	}
	record := recorder.records[0]
	if record.GoodsID != 8801 || record.ItemID != 31001 || record.ItemName != "高级化肥" || record.Count != 2 || record.UnitPrice != 80 || record.CurrencyID != 1005 || record.CurrencyName != "金豆豆" || record.Discount != 50 {
		t.Fatalf("unexpected purchase record: %#v", record)
	}
}

func TestRuntimeFacadeMysteryShopAutoBuyBuysFirstAvailableCard(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.requestMysteryMerchantCardsByProtocol": map[string]any{
			"ok": true,
			"cards": []any{
				map[string]any{"goodsId": float64(0), "name": "invalid"},
				map[string]any{"goodsId": float64(8801), "cardId": float64(7), "name": "稀有种子"},
			},
		},
		"gameCtl.buyMysteryMerchantCardByProtocol": map[string]any{"ok": true, "success": true},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "mystery_shop_auto_buy")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("mystery_shop_auto_buy should report OK, got %#v", result)
	}
	if len(caller.calls) != 2 {
		t.Fatalf("runtime call count = %d, want 2: %#v", len(caller.calls), caller.calls)
	}
	if caller.calls[0].method != "gameCtl.requestMysteryMerchantCardsByProtocol" {
		t.Fatalf("first method = %q, want requestMysteryMerchantCardsByProtocol", caller.calls[0].method)
	}
	if caller.calls[1].method != "gameCtl.buyMysteryMerchantCardByProtocol" {
		t.Fatalf("second method = %q, want buyMysteryMerchantCardByProtocol", caller.calls[1].method)
	}
	wantBuyArgs := []any{map[string]any{
		"goodsId": 8801,
		"cardId":  7,
		"silent":  true,
		"waitMs":  800,
		"source":  "farm_go_auto_mystery_shop_auto_buy",
	}}
	if !reflect.DeepEqual(caller.calls[1].args, wantBuyArgs) {
		t.Fatalf("buy args = %#v, want %#v", caller.calls[1].args, wantBuyArgs)
	}
}

func TestRuntimeFacadeMysteryShopReadOnlyFetchesCurrentCards(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.requestMysteryMerchantCardsByProtocol": map[string]any{
			"ok": true,
			"cards": []any{
				map[string]any{"goodsId": float64(8801), "cardId": float64(7), "item_name": "稀有种子", "count": float64(2), "currency_name": "金币", "unit_price": float64(100)},
				map[string]any{"goodsId": float64(8802), "cardId": float64(8), "itemName": "折扣种子", "count": float64(1), "currencyId": float64(1002), "unitPrice": float64(30), "discount": float64(50)},
			},
		},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "mystery_shop_read")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("mystery_shop_read should report OK, got %#v", result)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("read should not call buy runtime, got %#v", caller.calls)
	}
	if caller.calls[0].method != "gameCtl.requestMysteryMerchantCardsByProtocol" {
		t.Fatalf("read method = %q, want requestMysteryMerchantCardsByProtocol", caller.calls[0].method)
	}
	if result.Message != "已读取当前神秘商店商品 2 个：稀有种子 x2（金币 100）；折扣种子 x1（货币1002 30，5折）。" {
		t.Fatalf("message = %q", result.Message)
	}
}

func TestRuntimeFacadeMysteryShopAutoBuyUsesCurrencyAndDiscountConfig(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.requestMysteryMerchantCardsByProtocol": map[string]any{
			"ok": true,
			"cards": []any{
				map[string]any{"goodsId": float64(8801), "cardId": float64(7), "currencyId": float64(1004), "discount": float64(90)},
				map[string]any{"goodsId": float64(8802), "cardId": float64(8), "currencyId": float64(1002), "discount": float64(30)},
				map[string]any{"goodsId": float64(8803), "cardId": float64(9), "currencyId": float64(1001), "discount": float64(80)},
			},
		},
		"gameCtl.buyMysteryMerchantCardByProtocol": map[string]any{"ok": true, "success": true},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmMysteryShopCurrencyIds":       []int{1001, 1002},
		"autoFarmMysteryShopDiscountThreshold": 50,
	})

	result := facade.RunTask(context.Background(), "mystery_shop_auto_buy")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("mystery_shop_auto_buy should report OK, got %#v", result)
	}
	wantBuyArgs := []any{map[string]any{
		"goodsId": 8802,
		"cardId":  8,
		"silent":  true,
		"waitMs":  800,
		"source":  "farm_go_auto_mystery_shop_auto_buy",
	}}
	if !reflect.DeepEqual(caller.calls[1].args, wantBuyArgs) {
		t.Fatalf("buy args = %#v, want %#v", caller.calls[1].args, wantBuyArgs)
	}
}

func TestRuntimeFacadeMysteryShopAutoBuySkipsWhenNoCardMatchesConfig(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.requestMysteryMerchantCardsByProtocol": map[string]any{
			"ok": true,
			"cards": []any{
				map[string]any{"goodsId": float64(8801), "cardId": float64(7), "currencyId": float64(1004), "discount": float64(30)},
				map[string]any{"goodsId": float64(8802), "cardId": float64(8), "currencyId": float64(1002), "discount": float64(80)},
			},
		},
	}}
	facade := NewRuntimeFacadeWithConfig(caller, map[string]any{
		"autoFarmMysteryShopCurrencyIds":       []int{1001, 1002},
		"autoFarmMysteryShopDiscountThreshold": 50,
	})

	result := facade.RunTask(context.Background(), "mystery_shop_auto_buy")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("no matching mystery shop cards should be OK skip, got %#v", result)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("skip should not call buy runtime, got %#v", caller.calls)
	}
	if result.Message != "没有符合货币和折扣设置的神秘商店卡片，本轮自动购买跳过。" {
		t.Fatalf("message = %q", result.Message)
	}
}

func TestRuntimeFacadeMysteryShopAutoBuySkipsWhenNoCards(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.requestMysteryMerchantCardsByProtocol": map[string]any{"ok": true, "cards": []any{}},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "mystery_shop_auto_buy")

	if !result.OK || result.Status != StatusOK {
		t.Fatalf("no mystery shop cards should be OK skip, got %#v", result)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("skip should not call buy runtime, got %#v", caller.calls)
	}
}

func TestRuntimeFacadeMysteryShopAutoBuyHandlesNilRuntimeCaller(t *testing.T) {
	facade := NewRuntimeFacade(nil)

	result := facade.RunTask(context.Background(), "mystery_shop_auto_buy")

	if result.OK || result.Status != StatusRuntimeNotReady {
		t.Fatalf("nil runtime should be runtime_not_ready, got %#v", result)
	}
}

func TestRuntimeFacadeMysteryShopAutoBuyReportsRequestError(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.requestMysteryMerchantCardsByProtocol": errors.New("mystery service unavailable"),
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "mystery_shop_auto_buy")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("request error should fail, got %#v", result)
	}
}

func TestRuntimeFacadeMysteryShopAutoBuyReportsRequestNotOKResult(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.requestMysteryMerchantCardsByProtocol": map[string]any{"ok": false, "reason": "reply_not_observed"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "mystery_shop_auto_buy")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("request ok=false should fail, got %#v", result)
	}
}

func TestRuntimeFacadeMysteryShopAutoBuyReportsBuyNotOKResult(t *testing.T) {
	caller := &fakeRuntimeCaller{responses: map[string]any{
		"gameCtl.requestMysteryMerchantCardsByProtocol": map[string]any{
			"ok":    true,
			"cards": []any{map[string]any{"goodsId": float64(8801), "cardId": float64(7)}},
		},
		"gameCtl.buyMysteryMerchantCardByProtocol": map[string]any{"ok": false, "reason": "currency_insufficient"},
	}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "mystery_shop_auto_buy")

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("buy ok=false should fail, got %#v", result)
	}
}
