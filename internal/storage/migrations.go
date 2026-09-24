package storage

import (
	"context"
	"database/sql"
)

func (s *Store) Migrate(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS runtime_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_key TEXT NOT NULL DEFAULT 'default',
			ts TEXT NOT NULL,
			level TEXT NOT NULL,
			source TEXT NOT NULL,
			event_type TEXT NOT NULL,
			message TEXT NOT NULL,
			data_json TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS diagnostic_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts TEXT NOT NULL,
			method TEXT NOT NULL,
			params_json TEXT,
			ok INTEGER NOT NULL,
			duration_ms INTEGER NOT NULL,
			result_json TEXT,
			error TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS social_friend_rules (
			account_key TEXT PRIMARY KEY,
			rules_json TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS social_steal_records (
			id TEXT NOT NULL,
			account_key TEXT NOT NULL,
			date_key TEXT NOT NULL,
			occurred_at TEXT NOT NULL,
			gid INTEGER,
			display_name TEXT,
			payload_json TEXT NOT NULL,
			PRIMARY KEY (account_key, id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_social_steal_records_account_date
			ON social_steal_records(account_key, date_key, occurred_at);`,
		`CREATE TABLE IF NOT EXISTS social_visitor_records (
			id TEXT NOT NULL,
			account_key TEXT NOT NULL,
			occurred_at_ms INTEGER NOT NULL,
			player_id INTEGER,
			display_name TEXT,
			action_type INTEGER,
			payload_json TEXT NOT NULL,
			PRIMARY KEY (account_key, id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_social_visitor_records_account_time
			ON social_visitor_records(account_key, occurred_at_ms);`,
		`CREATE TABLE IF NOT EXISTS social_dog_guard_cache (
			account_key TEXT PRIMARY KEY,
			state_json TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS runtime_accounts (
			account_key TEXT PRIMARY KEY,
			gid INTEGER NOT NULL,
			nickname TEXT,
			avatar_url TEXT,
			identified_at TEXT NOT NULL,
			confirmed_at TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS warehouse_sell_records (
			id TEXT NOT NULL,
			account_key TEXT NOT NULL,
			date_key TEXT NOT NULL,
			occurred_at TEXT NOT NULL,
			mode TEXT NOT NULL,
			item_kinds INTEGER NOT NULL,
			total_count INTEGER NOT NULL,
			total_amount INTEGER NOT NULL,
			items_json TEXT NOT NULL,
			payload_json TEXT,
			PRIMARY KEY (account_key, id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_warehouse_sell_records_account_date
			ON warehouse_sell_records(account_key, date_key, occurred_at);`,
		`CREATE TABLE IF NOT EXISTS mystery_shop_purchase_records (
			id TEXT NOT NULL,
			account_key TEXT NOT NULL,
			occurred_at TEXT NOT NULL,
			goods_id INTEGER NOT NULL,
			item_id INTEGER NOT NULL,
			item_name TEXT NOT NULL,
			count INTEGER NOT NULL,
			unit_price INTEGER NOT NULL,
			currency_id INTEGER NOT NULL,
			currency_name TEXT NOT NULL,
			discount INTEGER NOT NULL,
			payload_json TEXT,
			PRIMARY KEY (account_key, id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_mystery_shop_purchase_records_account_time
			ON mystery_shop_purchase_records(account_key, occurred_at DESC, id DESC);`,
	}

	if err := s.ensureSettingsTable(ctx); err != nil {
		return err
	}
	for _, query := range queries {
		if _, err := s.db.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	if err := s.ensureColumn(ctx, "runtime_events", "account_key", "TEXT NOT NULL DEFAULT 'default'"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "social_steal_records", "occurred_at_ms", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "social_visitor_records", "steal_item_num", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	for _, query := range []string{
		`UPDATE social_steal_records
			SET occurred_at_ms = COALESCE(CAST(strftime('%s', occurred_at) AS INTEGER) * 1000, 0)
			WHERE occurred_at_ms = 0;`,
		`UPDATE social_visitor_records
			SET steal_item_num = CASE
				WHEN json_valid(payload_json) THEN COALESCE(CAST(json_extract(payload_json, '$.stealItemNum') AS INTEGER), 0)
				ELSE 0
			END
			WHERE steal_item_num = 0;`,
		`CREATE INDEX IF NOT EXISTS idx_social_steal_records_page
			ON social_steal_records(account_key, occurred_at_ms DESC, id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_social_steal_records_date_page
			ON social_steal_records(account_key, date_key, occurred_at_ms DESC, id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_social_visitor_records_page
			ON social_visitor_records(account_key, occurred_at_ms DESC, id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_social_visitor_records_action_page
			ON social_visitor_records(account_key, action_type, occurred_at_ms DESC, id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_social_steal_records_ranking
			ON social_steal_records(
				account_key,
				CASE WHEN gid > 0 THEN 'gid:' || CAST(gid AS TEXT) ELSE 'name:' || trim(display_name) END,
				occurred_at_ms DESC,
				display_name
			);`,
		`CREATE INDEX IF NOT EXISTS idx_social_visitor_records_ranking
			ON social_visitor_records(
				account_key,
				action_type,
				CASE WHEN player_id > 0 THEN 'player:' || CAST(player_id AS TEXT) ELSE 'name:' || trim(display_name) END,
				occurred_at_ms DESC,
				display_name,
				steal_item_num
			);`,
	} {
		if _, err := s.db.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA optimize=0x10002;`); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureSettingsTable(ctx context.Context) error {
	exists, err := s.tableExists(ctx, "settings")
	if err != nil {
		return err
	}
	if !exists {
		_, err := s.db.ExecContext(ctx, `
			CREATE TABLE settings (
				account_key TEXT NOT NULL DEFAULT 'global',
				key TEXT NOT NULL,
				value TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				PRIMARY KEY (account_key, key)
			);
		`)
		return err
	}

	columns, err := s.tableColumns(ctx, "settings")
	if err != nil {
		return err
	}
	accountPK := columns["account_key"].PrimaryKey
	keyPK := columns["key"].PrimaryKey
	if accountPK > 0 && keyPK > 0 {
		return nil
	}

	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE settings_v2 (
			account_key TEXT NOT NULL DEFAULT 'global',
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (account_key, key)
		);
	`); err != nil {
		return err
	}

	if _, ok := columns["account_key"]; ok {
		if _, err := s.db.ExecContext(ctx, `
			INSERT OR REPLACE INTO settings_v2 (account_key, key, value, updated_at)
			SELECT COALESCE(NULLIF(account_key, ''), 'global'), key, value, updated_at FROM settings;
		`); err != nil {
			return err
		}
	} else {
		if _, err := s.db.ExecContext(ctx, `
			INSERT OR REPLACE INTO settings_v2 (account_key, key, value, updated_at)
			SELECT 'global', key, value, updated_at FROM settings;
		`); err != nil {
			return err
		}
	}

	if _, err := s.db.ExecContext(ctx, `DROP TABLE settings;`); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `ALTER TABLE settings_v2 RENAME TO settings;`)
	return err
}

func (s *Store) ensureColumn(ctx context.Context, table string, column string, definition string) error {
	columns, err := s.tableColumns(ctx, table)
	if err != nil {
		return err
	}
	if _, ok := columns[column]; ok {
		return nil
	}
	_, err = s.db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition)
	return err
}

func (s *Store) tableExists(ctx context.Context, name string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count)
	return count > 0, err
}

type tableColumn struct {
	PrimaryKey int
}

func (s *Store) tableColumns(ctx context.Context, table string) (map[string]tableColumn, error) {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := map[string]tableColumn{}
	for rows.Next() {
		var cid int
		var name string
		var typ string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		columns[name] = tableColumn{PrimaryKey: primaryKey}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return columns, nil
}
