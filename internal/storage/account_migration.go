package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const legacyAccountConfigSeedMarker = "migration.accountConfigSeed.v1"

const selectAccountSettingSQL = `
	SELECT value
	FROM settings
	WHERE account_key = ? AND key = ?
`

const copyMissingAccountSettingsSQL = `
	INSERT INTO settings (account_key, key, value, updated_at)
	SELECT ?, key, value, ?
	FROM settings
	WHERE account_key = ? AND key LIKE ?
	ON CONFLICT(account_key, key) DO NOTHING
`

const copyMissingExactAccountSettingSQL = `
	INSERT INTO settings (account_key, key, value, updated_at)
	SELECT ?, key, value, ?
	FROM settings
	WHERE account_key = ? AND key = ? COLLATE BINARY
	ON CONFLICT(account_key, key) DO NOTHING
`

func (s *Store) SeedLegacyAccountConfiguration(ctx context.Context, accountKey string) error {
	accountKey = NormalizeAccountKey(accountKey)
	if accountKey == DefaultRuntimeAccountKey || accountKey == GlobalSettingsAccountKey {
		return errors.New("legacy account configuration requires a gid account key")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var marker string
	err = tx.QueryRowContext(ctx, selectAccountSettingSQL, accountKey, legacyAccountConfigSeedMarker).Scan(&marker)
	if err == nil && marker == "done" {
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	now := time.Now().Format(time.RFC3339Nano)
	patterns := []string{"autoFarm.%", "messagePush.%", "warehouse.%"}
	for _, source := range []string{DefaultRuntimeAccountKey, GlobalSettingsAccountKey} {
		for _, pattern := range patterns {
			if _, err := tx.ExecContext(ctx, copyMissingAccountSettingsSQL, accountKey, now, source, pattern); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, copyMissingExactAccountSettingSQL, accountKey, now, source, "social.rankingPreferences"); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, settingUpsertSQL, accountKey, legacyAccountConfigSeedMarker, "done", now); err != nil {
		return err
	}
	return tx.Commit()
}
