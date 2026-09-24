package automation

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (r RuntimeFacade) runMysteryShopRead(ctx context.Context) ActionResult {
	const taskID = "mystery_shop_read"
	requestValue, err := r.requestMysteryShopCards(ctx, taskID)
	if err != nil {
		return errResultFromMysteryShopReadError(taskID, err)
	}
	if failed, reason := runtimeResultFailed(requestValue); failed {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "读取神秘商店卡片返回失败：" + reason,
		}
	}
	return ActionResult{
		OK:      true,
		Status:  StatusOK,
		TaskID:  taskID,
		Message: mysteryShopReadMessage(requestValue),
	}
}

func (r RuntimeFacade) requestMysteryShopCards(ctx context.Context, taskID string) (any, error) {
	if r.caller == nil {
		return nil, errMysteryShopRuntimeNotReady{taskID: taskID}
	}
	return r.caller.Call(ctx, "gameCtl.requestMysteryMerchantCardsByProtocol", []any{map[string]any{
		"silent":                    true,
		"includeCallbackDiagnostic": true,
		"source":                    "farm_go_" + taskID,
	}}, 45*time.Second)
}

func (r RuntimeFacade) runMysteryShopAutoBuy(ctx context.Context) ActionResult {
	const taskID = "mystery_shop_auto_buy"
	requestValue, err := r.requestMysteryShopCards(ctx, taskID)
	if err != nil {
		return errResultFromMysteryShopReadError(taskID, err)
	}
	if failed, reason := runtimeResultFailed(requestValue); failed {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "读取神秘商店卡片返回失败：" + reason,
		}
	}
	card := selectMysteryShopBuyCard(requestValue, r.config)
	if len(card) == 0 {
		message := "没有检测到可购买的神秘商店卡片，本轮自动购买跳过。"
		if len(r.config) > 0 && mysteryShopCardCount(requestValue) > 0 {
			message = "没有符合货币和折扣设置的神秘商店卡片，本轮自动购买跳过。"
		}
		return ActionResult{
			OK:      true,
			Status:  StatusOK,
			TaskID:  taskID,
			Message: message,
		}
	}

	buyResult, err := r.caller.Call(ctx, "gameCtl.buyMysteryMerchantCardByProtocol", []any{map[string]any{
		"goodsId": mysteryShopGoodsID(card),
		"cardId":  intFromAny(firstExistingAny(card["cardId"], card["card_id"])),
		"silent":  true,
		"waitMs":  800,
		"source":  "farm_go_auto_mystery_shop_auto_buy",
	}}, 45*time.Second)
	if err != nil {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "神秘商店自动购买调用失败：" + err.Error(),
		}
	}
	if failed, reason := runtimeResultFailed(buyResult); failed {
		return ActionResult{
			OK:      false,
			Status:  StatusFailed,
			TaskID:  taskID,
			Message: "神秘商店自动购买返回失败：" + reason,
		}
	}
	r.saveMysteryShopPurchaseRecord(ctx, card)
	return ActionResult{
		OK:      true,
		Status:  StatusOK,
		TaskID:  taskID,
		Message: fmt.Sprintf("已提交神秘商店商品 %d 自动购买请求。", mysteryShopGoodsID(card)),
	}
}

func (r RuntimeFacade) saveMysteryShopPurchaseRecord(ctx context.Context, card map[string]any) {
	if r.mysteryShopPurchaseRecordWriter == nil {
		return
	}
	now := time.Now()
	payload := make(map[string]any, len(card))
	for key, value := range card {
		payload[key] = value
	}
	_ = r.mysteryShopPurchaseRecordWriter.SaveMysteryShopPurchaseRecord(ctx, r.accountKey, MysteryShopPurchaseRecord{
		ID:           "mystery_shop:" + strconv.FormatInt(now.UnixNano(), 10),
		OccurredAt:   now.Format(time.RFC3339Nano),
		GoodsID:      mysteryShopGoodsID(card),
		ItemID:       intFromAny(firstExistingAny(card["item_id"], card["itemId"])),
		ItemName:     mysteryShopCardName(card),
		Count:        intFromAny(firstExistingAny(card["count"], card["num"], card["amount"])),
		UnitPrice:    intFromAny(firstExistingAny(card["unit_price"], card["unitPrice"], card["price"])),
		CurrencyID:   intFromAny(firstExistingAny(card["currency_id"], card["currencyId"])),
		CurrencyName: mysteryShopCurrencyName(card),
		Discount:     intFromAny(firstExistingAny(card["discount"], card["discountRatio"], card["discount_ratio"])),
		Payload:      payload,
	})
}

type errMysteryShopRuntimeNotReady struct {
	taskID string
}

func (e errMysteryShopRuntimeNotReady) Error() string {
	return "runtime_not_ready"
}

func errResultFromMysteryShopReadError(taskID string, err error) ActionResult {
	if runtimeErr, ok := err.(errMysteryShopRuntimeNotReady); ok {
		action := "执行神秘商店自动购买"
		if runtimeErr.taskID == "mystery_shop_read" {
			action = "读取神秘商店商品"
		}
		return ActionResult{
			OK:      false,
			Status:  StatusRuntimeNotReady,
			TaskID:  taskID,
			Message: "游戏运行时尚未连接，无法" + action + "。",
		}
	}
	return ActionResult{
		OK:      false,
		Status:  StatusFailed,
		TaskID:  taskID,
		Message: "读取神秘商店卡片失败：" + err.Error(),
	}
}

func mysteryShopReadMessage(value any) string {
	count := mysteryShopCardCount(value)
	details := mysteryShopCardSummaries(value)
	if len(details) == 0 {
		return fmt.Sprintf("已读取当前神秘商店商品 %d 个。", count)
	}
	return fmt.Sprintf("已读取当前神秘商店商品 %d 个：%s。", count, strings.Join(details, "；"))
}

func mysteryShopCardSummaries(value any) []string {
	payload := mapFromAny(value)
	items := sliceFromAny(payload["cards"])
	if len(items) == 0 {
		items = sliceFromAny(payload["items"])
	}
	summaries := make([]string, 0, len(items))
	for _, item := range items {
		card := mapFromAny(item)
		if len(card) == 0 {
			continue
		}
		summary := mysteryShopCardName(card)
		if cardCount := intFromAny(firstExistingAny(card["count"], card["num"], card["amount"])); cardCount > 0 {
			summary += fmt.Sprintf(" x%d", cardCount)
		}
		extras := mysteryShopCardPriceParts(card)
		if len(extras) > 0 {
			summary += "（" + strings.Join(extras, "，") + "）"
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func mysteryShopCardName(card map[string]any) string {
	value := firstExistingAny(
		card["item_name"],
		card["itemName"],
		card["name"],
		card["goods_name"],
		card["goodsName"],
		card["title"],
	)
	if value != nil {
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" {
			return text
		}
	}
	if itemID := intFromAny(firstExistingAny(card["item_id"], card["itemId"])); itemID > 0 {
		return fmt.Sprintf("物品%d", itemID)
	}
	if goodsID := mysteryShopGoodsID(card); goodsID > 0 {
		return fmt.Sprintf("商品%d", goodsID)
	}
	return "未知商品"
}

func mysteryShopCardPriceParts(card map[string]any) []string {
	parts := []string{}
	priceValue := firstExistingAny(card["unit_price"], card["unitPrice"], card["price"])
	if priceValue != nil {
		currency := mysteryShopCurrencyName(card)
		price := intFromAny(priceValue)
		if currency != "" {
			parts = append(parts, fmt.Sprintf("%s %d", currency, price))
		} else {
			parts = append(parts, fmt.Sprintf("价格 %d", price))
		}
	}
	if discount := intFromAny(firstExistingAny(card["discount"], card["discountRatio"], card["discount_ratio"])); discount > 0 && discount < 100 {
		parts = append(parts, mysteryShopDiscountText(discount))
	}
	return parts
}

func mysteryShopCurrencyName(card map[string]any) string {
	value := firstExistingAny(card["currency_name"], card["currencyName"])
	if value != nil {
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" {
			return text
		}
	}
	if currencyID := intFromAny(firstExistingAny(card["currency_id"], card["currencyId"])); currencyID > 0 {
		return fmt.Sprintf("货币%d", currencyID)
	}
	return ""
}

func mysteryShopDiscountText(discount int) string {
	if discount%10 == 0 {
		return fmt.Sprintf("%d折", discount/10)
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", float64(discount)/10), "0"), ".") + "折"
}
