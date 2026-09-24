package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type WarehouseSellRecordItem struct {
	ItemID    int    `json:"itemId"`
	Name      string `json:"name,omitempty"`
	Count     int    `json:"count"`
	UnitPrice int    `json:"unitPrice,omitempty"`
	Amount    int    `json:"amount"`
}

type WarehouseSellRecord struct {
	ID          string                    `json:"id"`
	DateKey     string                    `json:"dateKey"`
	OccurredAt  string                    `json:"occurredAt"`
	Mode        string                    `json:"mode"`
	ItemKinds   int                       `json:"itemKinds"`
	TotalCount  int                       `json:"totalCount"`
	TotalAmount int                       `json:"totalAmount"`
	Items       []WarehouseSellRecordItem `json:"items"`
	Payload     map[string]any            `json:"payload,omitempty"`
}

func (s *Store) AppendWarehouseSellRecord(ctx context.Context, accountKey string, record WarehouseSellRecord) error {
	if strings.TrimSpace(record.ID) == "" || strings.TrimSpace(record.OccurredAt) == "" || strings.TrimSpace(record.DateKey) == "" {
		return nil
	}
	if record.Mode == "" {
		record.Mode = "manual"
	}
	itemsJSON, err := json.Marshal(record.Items)
	if err != nil {
		return err
	}
	var payloadJSON sql.NullString
	if len(record.Payload) > 0 {
		raw, err := json.Marshal(record.Payload)
		if err != nil {
			return err
		}
		payloadJSON = sql.NullString{String: string(raw), Valid: true}
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO warehouse_sell_records (
			id, account_key, date_key, occurred_at, mode, item_kinds, total_count, total_amount, items_json, payload_json
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_key, id) DO UPDATE SET
			date_key = excluded.date_key,
			occurred_at = excluded.occurred_at,
			mode = excluded.mode,
			item_kinds = excluded.item_kinds,
			total_count = excluded.total_count,
			total_amount = excluded.total_amount,
			items_json = excluded.items_json,
			payload_json = excluded.payload_json
	`, record.ID, NormalizeAccountKey(accountKey), record.DateKey, record.OccurredAt, record.Mode, record.ItemKinds, record.TotalCount, record.TotalAmount, string(itemsJSON), payloadJSON)
	return err
}

func (s *Store) ListWarehouseSellRecords(ctx context.Context, accountKey string, dateKey string, limit int) ([]WarehouseSellRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	query := `
		SELECT id, date_key, occurred_at, mode, item_kinds, total_count, total_amount, items_json, payload_json
		FROM warehouse_sell_records
		WHERE account_key = ?`
	args := []any{NormalizeAccountKey(accountKey)}
	if strings.TrimSpace(dateKey) != "" {
		query += ` AND date_key = ?`
		args = append(args, strings.TrimSpace(dateKey))
	}
	query += ` ORDER BY occurred_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []WarehouseSellRecord{}
	for rows.Next() {
		var record WarehouseSellRecord
		var itemsJSON string
		var payloadJSON sql.NullString
		if err := rows.Scan(&record.ID, &record.DateKey, &record.OccurredAt, &record.Mode, &record.ItemKinds, &record.TotalCount, &record.TotalAmount, &itemsJSON, &payloadJSON); err != nil {
			return nil, err
		}
		if itemsJSON != "" {
			if err := json.Unmarshal([]byte(itemsJSON), &record.Items); err != nil {
				return nil, err
			}
		}
		if payloadJSON.Valid && payloadJSON.String != "" {
			_ = json.Unmarshal([]byte(payloadJSON.String), &record.Payload)
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) SumWarehouseSellAmountsByDate(ctx context.Context, accountKey string, startDateKey string, endDateKey string) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT date_key, COALESCE(SUM(total_amount), 0)
		FROM warehouse_sell_records
		WHERE account_key = ? AND date_key >= ? AND date_key <= ?
		GROUP BY date_key
		ORDER BY date_key ASC
	`, NormalizeAccountKey(accountKey), strings.TrimSpace(startDateKey), strings.TrimSpace(endDateKey))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	totals := map[string]int64{}
	for rows.Next() {
		var dateKey string
		var amount int64
		if err := rows.Scan(&dateKey, &amount); err != nil {
			return nil, err
		}
		totals[dateKey] = amount
	}
	return totals, rows.Err()
}

func (s *Store) SumWarehouseSellAmountSince(ctx context.Context, accountKey string, startedAt time.Time) (int64, error) {
	var total int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(total_amount), 0)
		FROM warehouse_sell_records
		WHERE account_key = ? AND occurred_at >= ?
	`, NormalizeAccountKey(accountKey), startedAt.Format(time.RFC3339Nano)).Scan(&total)
	return total, err
}
