package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"Farm_Go/internal/eventbus"
)

func (s *Store) AppendRuntimeEvent(ctx context.Context, event eventbus.Event) error {
	return s.AppendRuntimeEventForAccount(ctx, DefaultRuntimeAccountKey, event)
}

func (s *Store) AppendRuntimeEventForAccount(ctx context.Context, accountKey string, event eventbus.Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.Level == "" {
		event.Level = eventbus.LevelInfo
	}

	var dataJSON sql.NullString
	if len(event.Data) > 0 {
		raw, err := json.Marshal(event.Data)
		if err != nil {
			return err
		}
		dataJSON = sql.NullString{String: string(raw), Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO runtime_events (account_key, ts, level, source, event_type, message, data_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, NormalizeAccountKey(accountKey), event.Timestamp.Format(time.RFC3339Nano), string(event.Level), event.Source, event.Type, event.Message, dataJSON)
	return err
}

func (s *Store) ListRuntimeEvents(ctx context.Context, limit int) ([]eventbus.Event, error) {
	return s.ListRuntimeEventsForAccount(ctx, DefaultRuntimeAccountKey, limit)
}

func (s *Store) ListRuntimeEventsForAccount(ctx context.Context, accountKey string, limit int) ([]eventbus.Event, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, ts, level, source, event_type, message, data_json
		FROM runtime_events
		WHERE account_key = ?
		ORDER BY id DESC
		LIMIT ?
	`, NormalizeAccountKey(accountKey), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []eventbus.Event{}
	for rows.Next() {
		var event eventbus.Event
		var ts string
		var level string
		var dataJSON sql.NullString
		if err := rows.Scan(&event.ID, &ts, &level, &event.Source, &event.Type, &event.Message, &dataJSON); err != nil {
			return nil, err
		}
		parsed, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return nil, err
		}
		event.Timestamp = parsed
		event.Level = eventbus.Level(level)
		if dataJSON.Valid && dataJSON.String != "" {
			if err := json.Unmarshal([]byte(dataJSON.String), &event.Data); err != nil {
				return nil, err
			}
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *Store) ListRuntimeEventsForAccountSince(ctx context.Context, accountKey string, since time.Time) ([]eventbus.Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, ts, level, source, event_type, message, data_json
		FROM runtime_events
		WHERE account_key = ? AND ts >= ?
		ORDER BY id ASC
	`, NormalizeAccountKey(accountKey), since.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []eventbus.Event{}
	for rows.Next() {
		var event eventbus.Event
		var ts string
		var level string
		var dataJSON sql.NullString
		if err := rows.Scan(&event.ID, &ts, &level, &event.Source, &event.Type, &event.Message, &dataJSON); err != nil {
			return nil, err
		}
		parsed, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return nil, err
		}
		event.Timestamp = parsed
		event.Level = eventbus.Level(level)
		if dataJSON.Valid && dataJSON.String != "" {
			if err := json.Unmarshal([]byte(dataJSON.String), &event.Data); err != nil {
				return nil, err
			}
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}
