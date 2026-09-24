package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

type MysteryShopPurchaseRecord struct {
	ID           string         `json:"id"`
	OccurredAt   string         `json:"occurredAt"`
	GoodsID      int            `json:"goodsId"`
	ItemID       int            `json:"itemId"`
	ItemName     string         `json:"itemName"`
	Count        int            `json:"count"`
	UnitPrice    int            `json:"unitPrice"`
	CurrencyID   int            `json:"currencyId"`
	CurrencyName string         `json:"currencyName"`
	Discount     int            `json:"discount"`
	Payload      map[string]any `json:"payload,omitempty"`
}

func (s *Store) AppendMysteryShopPurchaseRecord(ctx context.Context, accountKey string, record MysteryShopPurchaseRecord) error {
	if strings.TrimSpace(record.ID) == "" || strings.TrimSpace(record.OccurredAt) == "" {
		return nil
	}
	var payloadJSON sql.NullString
	if len(record.Payload) > 0 {
		raw, err := json.Marshal(record.Payload)
		if err != nil {
			return err
		}
		payloadJSON = sql.NullString{String: string(raw), Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mystery_shop_purchase_records (
			id, account_key, occurred_at, goods_id, item_id, item_name, count, unit_price, currency_id, currency_name, discount, payload_json
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_key, id) DO UPDATE SET
			occurred_at = excluded.occurred_at,
			goods_id = excluded.goods_id,
			item_id = excluded.item_id,
			item_name = excluded.item_name,
			count = excluded.count,
			unit_price = excluded.unit_price,
			currency_id = excluded.currency_id,
			currency_name = excluded.currency_name,
			discount = excluded.discount,
			payload_json = excluded.payload_json
	`, record.ID, NormalizeAccountKey(accountKey), record.OccurredAt, record.GoodsID, record.ItemID, record.ItemName, record.Count, record.UnitPrice, record.CurrencyID, record.CurrencyName, record.Discount, payloadJSON)
	return err
}

func (s *Store) ListMysteryShopPurchaseRecords(ctx context.Context, accountKey string, limit int) ([]MysteryShopPurchaseRecord, error) {
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, occurred_at, goods_id, item_id, item_name, count, unit_price, currency_id, currency_name, discount, payload_json
		FROM mystery_shop_purchase_records
		WHERE account_key = ?
		ORDER BY occurred_at DESC, id DESC
		LIMIT ?
	`, NormalizeAccountKey(accountKey), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []MysteryShopPurchaseRecord{}
	for rows.Next() {
		var record MysteryShopPurchaseRecord
		var payloadJSON sql.NullString
		if err := rows.Scan(&record.ID, &record.OccurredAt, &record.GoodsID, &record.ItemID, &record.ItemName, &record.Count, &record.UnitPrice, &record.CurrencyID, &record.CurrencyName, &record.Discount, &payloadJSON); err != nil {
			return nil, err
		}
		if payloadJSON.Valid && payloadJSON.String != "" {
			if err := json.Unmarshal([]byte(payloadJSON.String), &record.Payload); err != nil {
				return nil, err
			}
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
