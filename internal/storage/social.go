package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type SocialFriendRules struct {
	WhitelistEnabled bool     `json:"whitelistEnabled"`
	WhitelistScopes  []string `json:"whitelistScopes"`
	Whitelist        []string `json:"whitelist"`
	BlacklistEnabled bool     `json:"blacklistEnabled"`
	BlacklistScopes  []string `json:"blacklistScopes"`
	Blacklist        []string `json:"blacklist"`
	MaskedBlacklist  bool     `json:"maskedBlacklist"`
	MaskedMaxLevel   int      `json:"maskedMaxLevel"`
}

type SocialRecordRow struct {
	ID           string
	DateKey      string
	OccurredAt   string
	OccurredAtMS int64
	GID          int
	DisplayName  string
	PayloadJSON  string
}

type SocialVisitorRow struct {
	ID           string
	OccurredAtMS int64
	PlayerID     int
	DisplayName  string
	ActionType   int
	StealItemNum int
	PayloadJSON  string
}

type SocialRankingPreferences struct {
	StolenByMeViewMode   string `json:"stolenByMeViewMode"`
	StolenFromMeViewMode string `json:"stolenFromMeViewMode"`
}

func (s *Store) LoadSocialFriendRules(ctx context.Context, accountKey string) (SocialFriendRules, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT rules_json
		FROM social_friend_rules
		WHERE account_key = ?
	`, normalizeSocialAccountKey(accountKey)).Scan(&raw)
	if err == sql.ErrNoRows {
		return defaultSocialFriendRules(), nil
	}
	if err != nil {
		return SocialFriendRules{}, err
	}
	var rules SocialFriendRules
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return SocialFriendRules{}, err
	}
	if rules.MaskedMaxLevel <= 0 {
		rules.MaskedMaxLevel = 1
	}
	return rules, nil
}

func (s *Store) SaveSocialFriendRules(ctx context.Context, accountKey string, rules SocialFriendRules) error {
	if rules.MaskedMaxLevel <= 0 {
		rules.MaskedMaxLevel = 1
	}
	raw, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO social_friend_rules (account_key, rules_json, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(account_key) DO UPDATE SET
			rules_json = excluded.rules_json,
			updated_at = excluded.updated_at
	`, normalizeSocialAccountKey(accountKey), string(raw), time.Now().Format(time.RFC3339Nano))
	return err
}

func (s *Store) AppendSocialStealRecords(ctx context.Context, accountKey string, rows []SocialRecordRow) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	key := normalizeSocialAccountKey(accountKey)
	for _, row := range rows {
		if strings.TrimSpace(row.ID) == "" || strings.TrimSpace(row.PayloadJSON) == "" {
			continue
		}
		if strings.TrimSpace(row.OccurredAt) != "" {
			occurredAt, err := time.Parse(time.RFC3339Nano, row.OccurredAt)
			if err != nil {
				return err
			}
			if row.OccurredAtMS == 0 {
				row.OccurredAtMS = occurredAt.UnixMilli()
			}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO social_steal_records (id, account_key, date_key, occurred_at, occurred_at_ms, gid, display_name, payload_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(account_key, id) DO UPDATE SET
				date_key = excluded.date_key,
				occurred_at = excluded.occurred_at,
				occurred_at_ms = excluded.occurred_at_ms,
				gid = excluded.gid,
				display_name = excluded.display_name,
				payload_json = excluded.payload_json
		`, row.ID, key, row.DateKey, row.OccurredAt, row.OccurredAtMS, row.GID, row.DisplayName, row.PayloadJSON); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListSocialStealRecords(ctx context.Context, accountKey string, dateKeys []string) ([]SocialRecordRow, error) {
	key := normalizeSocialAccountKey(accountKey)
	query := `
		SELECT id, date_key, occurred_at, occurred_at_ms, gid, display_name, payload_json
		FROM social_steal_records
		WHERE account_key = ?`
	args := []any{key}
	if len(dateKeys) > 0 {
		query += ` AND date_key IN (` + sqlPlaceholders(len(dateKeys)) + `)`
		for _, dateKey := range dateKeys {
			args = append(args, dateKey)
		}
	}
	query += ` ORDER BY occurred_at DESC, id DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []SocialRecordRow{}
	for rows.Next() {
		var row SocialRecordRow
		if err := rows.Scan(&row.ID, &row.DateKey, &row.OccurredAt, &row.OccurredAtMS, &row.GID, &row.DisplayName, &row.PayloadJSON); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *Store) AppendSocialVisitorRecords(ctx context.Context, accountKey string, rows []SocialVisitorRow) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	key := normalizeSocialAccountKey(accountKey)
	for _, row := range rows {
		if strings.TrimSpace(row.ID) == "" || strings.TrimSpace(row.PayloadJSON) == "" {
			continue
		}
		if row.StealItemNum == 0 && json.Valid([]byte(row.PayloadJSON)) {
			var payload struct {
				StealItemNum int `json:"stealItemNum"`
			}
			if err := json.Unmarshal([]byte(row.PayloadJSON), &payload); err == nil {
				row.StealItemNum = payload.StealItemNum
			}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO social_visitor_records (id, account_key, occurred_at_ms, player_id, display_name, action_type, steal_item_num, payload_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(account_key, id) DO UPDATE SET
				occurred_at_ms = excluded.occurred_at_ms,
				player_id = excluded.player_id,
				display_name = excluded.display_name,
				action_type = excluded.action_type,
				steal_item_num = excluded.steal_item_num,
				payload_json = excluded.payload_json
		`, row.ID, key, row.OccurredAtMS, row.PlayerID, row.DisplayName, row.ActionType, row.StealItemNum, row.PayloadJSON); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListSocialVisitorRecords(ctx context.Context, accountKey string) ([]SocialVisitorRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, occurred_at_ms, player_id, display_name, action_type, steal_item_num, payload_json
		FROM social_visitor_records
		WHERE account_key = ?
		ORDER BY occurred_at_ms DESC, id DESC
	`, normalizeSocialAccountKey(accountKey))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []SocialVisitorRow{}
	for rows.Next() {
		var row SocialVisitorRow
		if err := rows.Scan(&row.ID, &row.OccurredAtMS, &row.PlayerID, &row.DisplayName, &row.ActionType, &row.StealItemNum, &row.PayloadJSON); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *Store) LoadSocialDogGuardCache(ctx context.Context, accountKey string) (string, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT state_json
		FROM social_dog_guard_cache
		WHERE account_key = ?
	`, normalizeSocialAccountKey(accountKey)).Scan(&raw)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return raw, nil
}

func (s *Store) SaveSocialDogGuardCache(ctx context.Context, accountKey string, stateJSON string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO social_dog_guard_cache (account_key, state_json, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(account_key) DO UPDATE SET
			state_json = excluded.state_json,
			updated_at = excluded.updated_at
	`, normalizeSocialAccountKey(accountKey), stateJSON, time.Now().Format(time.RFC3339Nano))
	return err
}

func (s *Store) LoadSocialRankingPreferences(ctx context.Context, accountKey string) (SocialRankingPreferences, error) {
	values, err := s.loadSettingsForAccount(ctx, normalizeSocialAccountKey(accountKey), []string{"social.rankingPreferences"})
	if err != nil {
		return SocialRankingPreferences{}, err
	}
	raw := strings.TrimSpace(values["social.rankingPreferences"])
	if raw == "" {
		return defaultSocialRankingPreferences(), nil
	}
	var preferences SocialRankingPreferences
	if err := json.Unmarshal([]byte(raw), &preferences); err != nil {
		return SocialRankingPreferences{}, err
	}
	return normalizeSocialRankingPreferences(preferences), nil
}

func (s *Store) SaveSocialRankingPreferences(ctx context.Context, accountKey string, preferences SocialRankingPreferences) error {
	preferences = normalizeSocialRankingPreferences(preferences)
	raw, err := json.Marshal(preferences)
	if err != nil {
		return err
	}
	return s.saveSettingForAccount(ctx, normalizeSocialAccountKey(accountKey), "social.rankingPreferences", string(raw))
}

func defaultSocialFriendRules() SocialFriendRules {
	return SocialFriendRules{
		WhitelistScopes: []string{},
		Whitelist:       []string{},
		BlacklistScopes: []string{},
		Blacklist:       []string{},
		MaskedBlacklist: true,
		MaskedMaxLevel:  1,
	}
}

func defaultSocialRankingPreferences() SocialRankingPreferences {
	return SocialRankingPreferences{StolenByMeViewMode: "timeline", StolenFromMeViewMode: "timeline"}
}

func normalizeSocialRankingPreferences(preferences SocialRankingPreferences) SocialRankingPreferences {
	if preferences.StolenByMeViewMode != "ranking" {
		preferences.StolenByMeViewMode = "timeline"
	}
	if preferences.StolenFromMeViewMode != "ranking" {
		preferences.StolenFromMeViewMode = "timeline"
	}
	return preferences
}

func normalizeSocialAccountKey(value string) string {
	key := strings.TrimSpace(value)
	if key == "" {
		return "default"
	}
	return key
}

func sqlPlaceholders(count int) string {
	if count <= 0 {
		return ""
	}
	parts := make([]string, count)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}
