package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

const (
	socialStealIdentityExpression   = `CASE WHEN gid > 0 THEN 'gid:' || CAST(gid AS TEXT) ELSE 'name:' || trim(display_name) END`
	socialVisitorIdentityExpression = `CASE WHEN player_id > 0 THEN 'player:' || CAST(player_id AS TEXT) ELSE 'name:' || trim(display_name) END`
)

type SocialRankingCursor struct {
	TimeMS      int64
	Key         string
	IdentityKey string
	StealCount  int
	EventCount  int
	RankOffset  int
}

type SocialRankingQuery struct {
	AccountKey string
	Tab        string
	ViewMode   string
	StartMS    *int64
	EndMS      *int64
	Cursor     *SocialRankingCursor
	Limit      int
}

type SocialRankingPageRow struct {
	Kind         string
	Key          string
	IdentityKey  string
	TimeMS       int64
	DisplayName  string
	Rank         int
	EventCount   int
	StealCount   int
	ActionType   int
	ActionLabel  string
	ActionTarget string
	PayloadJSON  string
	PayloadJSONs []string
}

type SocialRankingPage struct {
	Summary SocialRankingSummary
	Rows    []SocialRankingPageRow
	HasMore bool
}

type SocialRankingSummary struct {
	VisitorCount          int
	StolenFromMeCount     int
	StolenByMeCount       int
	StolenByMeRecordCount int
}

func (s *Store) QuerySocialRankingPage(ctx context.Context, query SocialRankingQuery) (SocialRankingPage, error) {
	query.AccountKey = normalizeSocialAccountKey(query.AccountKey)
	if query.ViewMode == "" {
		query.ViewMode = "timeline"
	}
	if query.ViewMode != "timeline" && query.ViewMode != "ranking" {
		return SocialRankingPage{}, fmt.Errorf("unsupported social ranking view mode %q", query.ViewMode)
	}
	if query.ViewMode == "ranking" && query.Tab == "visitors" {
		return SocialRankingPage{}, fmt.Errorf("unsupported social ranking tab %q for ranking mode", query.Tab)
	}
	if query.Limit < 1 {
		return SocialRankingPage{}, fmt.Errorf("social ranking limit must be positive")
	}
	if query.StartMS != nil && query.EndMS != nil && *query.EndMS < *query.StartMS {
		return SocialRankingPage{}, fmt.Errorf("social ranking end cannot be before start")
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return SocialRankingPage{}, err
	}
	defer tx.Rollback()

	var summary SocialRankingSummary
	var rows []SocialRankingPageRow
	var hasMore bool
	switch query.ViewMode {
	case "timeline":
		summary, err = querySocialRankingSummary(ctx, tx, query)
		if err != nil {
			return SocialRankingPage{}, err
		}
		rows, hasMore, err = querySocialRankingTimeline(ctx, tx, query)
	case "ranking":
		if query.Cursor == nil {
			summary, err = querySocialRankingComplementarySummary(ctx, tx, query)
		} else {
			summary, err = querySocialRankingSummary(ctx, tx, query)
		}
		if err != nil {
			return SocialRankingPage{}, err
		}
		var groupedSummary SocialRankingSummary
		rows, hasMore, groupedSummary, err = querySocialRankingGrouped(ctx, tx, query)
		if query.Cursor == nil {
			if query.Tab == "stolenByMe" {
				summary.StolenByMeCount = groupedSummary.StolenByMeCount
				summary.StolenByMeRecordCount = groupedSummary.StolenByMeRecordCount
			} else {
				summary.StolenFromMeCount = groupedSummary.StolenFromMeCount
			}
		}
	}
	if err != nil {
		return SocialRankingPage{}, err
	}
	if err := tx.Commit(); err != nil {
		return SocialRankingPage{}, err
	}
	return SocialRankingPage{Summary: summary, Rows: rows, HasMore: hasMore}, nil
}

func querySocialRankingComplementarySummary(ctx context.Context, tx *sql.Tx, query SocialRankingQuery) (SocialRankingSummary, error) {
	visitorWhere, visitorArgs := socialRankingWindowPredicates(query.AccountKey, query.StartMS, query.EndMS)
	visitorPredicate := strings.Join(visitorWhere, " AND ")
	if query.Tab == "stolenByMe" {
		statement := `SELECT COUNT(*),
			COUNT(DISTINCT CASE WHEN action_type = 1 THEN ` + socialVisitorIdentityExpression + ` END)
			FROM social_visitor_records WHERE ` + visitorPredicate
		var summary SocialRankingSummary
		err := tx.QueryRowContext(ctx, statement, visitorArgs...).Scan(&summary.VisitorCount, &summary.StolenFromMeCount)
		return summary, err
	}

	stealWhere, stealArgs := socialRankingWindowPredicates(query.AccountKey, query.StartMS, query.EndMS)
	stealPredicate := strings.Join(stealWhere, " AND ")
	statement := `WITH visitor_summary AS (
		SELECT COUNT(*) AS visitor_count
		FROM social_visitor_records WHERE ` + visitorPredicate + `
	), steal_summary AS (
		SELECT COUNT(DISTINCT ` + socialStealIdentityExpression + `) AS stolen_by_me_count,
			COUNT(*) AS stolen_by_me_record_count
		FROM social_steal_records WHERE ` + stealPredicate + `
	)
	SELECT visitor_count, stolen_by_me_count, stolen_by_me_record_count
	FROM visitor_summary CROSS JOIN steal_summary`
	args := make([]any, 0, len(visitorArgs)+len(stealArgs))
	args = append(args, visitorArgs...)
	args = append(args, stealArgs...)
	var summary SocialRankingSummary
	err := tx.QueryRowContext(ctx, statement, args...).Scan(
		&summary.VisitorCount, &summary.StolenByMeCount, &summary.StolenByMeRecordCount,
	)
	return summary, err
}

func querySocialRankingGrouped(ctx context.Context, tx *sql.Tx, query SocialRankingQuery) ([]SocialRankingPageRow, bool, SocialRankingSummary, error) {
	statement, args, kind, err := buildSocialGroupedRankingQuery(query)
	if err != nil {
		return nil, false, SocialRankingSummary{}, err
	}
	rows, err := tx.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, false, SocialRankingSummary{}, err
	}
	defer rows.Close()

	var summary SocialRankingSummary
	result := make([]SocialRankingPageRow, 0, query.Limit+1)
	for rows.Next() {
		var row SocialRankingPageRow
		var identityCount, eventCount int
		if err := rows.Scan(
			&row.IdentityKey, &row.TimeMS, &row.StealCount, &row.EventCount,
			&identityCount, &eventCount,
		); err != nil {
			return nil, false, SocialRankingSummary{}, err
		}
		if query.Tab == "stolenByMe" {
			summary.StolenByMeCount = identityCount
			summary.StolenByMeRecordCount = eventCount
		} else {
			summary.StolenFromMeCount = identityCount
		}
		row.Kind = kind
		row.Key = row.IdentityKey
		if query.Tab == "stolenFromMe" {
			row.ActionType = 1
			if strings.HasPrefix(row.IdentityKey, "player:") {
				row.ActionTarget = strings.TrimPrefix(row.IdentityKey, "player:")
			}
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false, SocialRankingSummary{}, err
	}

	hasMore := len(result) > query.Limit
	if hasMore {
		result = result[:query.Limit]
	}
	rankOffset := 0
	if query.Cursor != nil {
		rankOffset = query.Cursor.RankOffset
	}
	for index := range result {
		result[index].Rank = rankOffset + index + 1
	}
	if query.Tab == "stolenByMe" {
		if err := querySocialStealRankingPayloads(ctx, tx, query, result); err != nil {
			return nil, false, SocialRankingSummary{}, err
		}
	} else if err := querySocialVisitorRankingDisplayNames(ctx, tx, query, result); err != nil {
		return nil, false, SocialRankingSummary{}, err
	}
	return result, hasMore, summary, nil
}

func buildSocialGroupedRankingQuery(query SocialRankingQuery) (string, []any, string, error) {
	if query.Limit < 1 {
		return "", nil, "", fmt.Errorf("social ranking limit must be positive")
	}
	if query.StartMS != nil && query.EndMS != nil && *query.EndMS < *query.StartMS {
		return "", nil, "", fmt.Errorf("social ranking end cannot be before start")
	}

	var table, identityExpression, stealExpression, eventExpression, kind string
	switch query.Tab {
	case "stolenByMe":
		table = "social_steal_records"
		identityExpression = socialStealIdentityExpression
		stealExpression = "COUNT(*)"
		eventExpression = "COUNT(*)"
		kind = "stealRanking"
	case "stolenFromMe":
		table = "social_visitor_records"
		identityExpression = socialVisitorIdentityExpression
		stealExpression = "COALESCE(SUM(steal_item_num), 0)"
		eventExpression = "COUNT(*)"
		kind = "stolenRanking"
	default:
		return "", nil, "", fmt.Errorf("unsupported social ranking tab %q for ranking mode", query.Tab)
	}

	where, args := socialRankingWindowPredicates(normalizeSocialAccountKey(query.AccountKey), query.StartMS, query.EndMS)
	if query.Tab == "stolenFromMe" {
		where = append(where, "action_type = 1")
	}
	statement := `SELECT ` + identityExpression + ` AS identity_key,
		MAX(occurred_at_ms) AS last_time_ms,
		` + stealExpression + ` AS steal_count,
		` + eventExpression + ` AS event_count,
		COUNT(*) OVER() AS ranked_identity_count,
		SUM(COUNT(*)) OVER() AS ranked_event_count
		FROM ` + table + `
		WHERE ` + strings.Join(where, " AND ") + `
		GROUP BY ` + identityExpression
	if query.Cursor != nil {
		if strings.TrimSpace(query.Cursor.IdentityKey) == "" {
			return "", nil, "", fmt.Errorf("social ranking cursor identity key is required")
		}
		if query.Cursor.StealCount < 0 || query.Cursor.EventCount < 0 {
			return "", nil, "", fmt.Errorf("social ranking cursor counts cannot be negative")
		}
		if query.Cursor.RankOffset < 1 {
			return "", nil, "", fmt.Errorf("social ranking cursor rank offset must be positive")
		}
		statement += `
		HAVING (` + stealExpression + ` < ?
			OR (` + stealExpression + ` = ? AND ` + eventExpression + ` < ?)
			OR (` + stealExpression + ` = ? AND ` + eventExpression + ` = ? AND MAX(occurred_at_ms) < ?)
			OR (` + stealExpression + ` = ? AND ` + eventExpression + ` = ? AND MAX(occurred_at_ms) = ? AND ` + identityExpression + ` > ?))`
		args = append(args,
			query.Cursor.StealCount,
			query.Cursor.StealCount, query.Cursor.EventCount,
			query.Cursor.StealCount, query.Cursor.EventCount, query.Cursor.TimeMS,
			query.Cursor.StealCount, query.Cursor.EventCount, query.Cursor.TimeMS, query.Cursor.IdentityKey,
		)
	}
	statement += `
		ORDER BY steal_count DESC, event_count DESC, last_time_ms DESC, identity_key ASC
		LIMIT ?`
	args = append(args, query.Limit+1)
	return statement, args, kind, nil
}

func querySocialStealRankingPayloads(ctx context.Context, tx *sql.Tx, query SocialRankingQuery, pageRows []SocialRankingPageRow) error {
	if len(pageRows) == 0 {
		return nil
	}
	indices := make(map[string]int, len(pageRows))
	identities := make([]string, 0, len(pageRows))
	for index := range pageRows {
		indices[pageRows[index].IdentityKey] = index
		identities = append(identities, pageRows[index].IdentityKey)
	}

	where, args := socialRankingWindowPredicates(normalizeSocialAccountKey(query.AccountKey), query.StartMS, query.EndMS)
	where = append(where, socialStealIdentityExpression+" IN ("+sqlPlaceholders(len(identities))+")")
	for _, identity := range identities {
		args = append(args, identity)
	}
	statement := `SELECT ` + socialStealIdentityExpression + ` AS identity_key, trim(display_name), payload_json
		FROM social_steal_records
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY occurred_at_ms DESC, id DESC`
	rows, err := tx.QueryContext(ctx, statement, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	seenNames := make(map[string]bool, len(pageRows))
	for rows.Next() {
		var identity, displayName, payloadJSON string
		if err := rows.Scan(&identity, &displayName, &payloadJSON); err != nil {
			return err
		}
		index, ok := indices[identity]
		if !ok {
			continue
		}
		if !seenNames[identity] {
			pageRows[index].DisplayName = displayName
			seenNames[identity] = true
		}
		pageRows[index].PayloadJSONs = append(pageRows[index].PayloadJSONs, payloadJSON)
	}
	return rows.Err()
}

func querySocialVisitorRankingDisplayNames(ctx context.Context, tx *sql.Tx, query SocialRankingQuery, pageRows []SocialRankingPageRow) error {
	if len(pageRows) == 0 {
		return nil
	}
	indices := make(map[string]int, len(pageRows))
	identities := make([]string, 0, len(pageRows))
	for index := range pageRows {
		indices[pageRows[index].IdentityKey] = index
		identities = append(identities, pageRows[index].IdentityKey)
	}

	where, args := socialRankingWindowPredicates(normalizeSocialAccountKey(query.AccountKey), query.StartMS, query.EndMS)
	where = append(where, "action_type = 1")
	where = append(where, socialVisitorIdentityExpression+" IN ("+sqlPlaceholders(len(identities))+")")
	for _, identity := range identities {
		args = append(args, identity)
	}
	statement := `SELECT ` + socialVisitorIdentityExpression + ` AS identity_key, trim(display_name)
		FROM social_visitor_records
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY occurred_at_ms DESC, id DESC`
	rows, err := tx.QueryContext(ctx, statement, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	seenNames := make(map[string]bool, len(pageRows))
	for rows.Next() {
		var identity, displayName string
		if err := rows.Scan(&identity, &displayName); err != nil {
			return err
		}
		index, ok := indices[identity]
		if !ok || seenNames[identity] {
			continue
		}
		pageRows[index].DisplayName = displayName
		seenNames[identity] = true
	}
	return rows.Err()
}

func querySocialRankingSummary(ctx context.Context, tx *sql.Tx, query SocialRankingQuery) (SocialRankingSummary, error) {
	visitorWhere, visitorArgs := socialRankingWindowPredicates(query.AccountKey, query.StartMS, query.EndMS)
	stealWhere, stealArgs := socialRankingWindowPredicates(query.AccountKey, query.StartMS, query.EndMS)
	visitorPredicate := strings.Join(visitorWhere, " AND ")
	stealPredicate := strings.Join(stealWhere, " AND ")

	statement := `
		WITH visitor_summary AS (
			SELECT
				COUNT(*) AS visitor_count,
				COUNT(DISTINCT CASE WHEN action_type = 1 THEN ` + socialVisitorIdentityExpression + ` END) AS stolen_from_me_count
			FROM social_visitor_records
			WHERE ` + visitorPredicate + `
		), steal_summary AS (
			SELECT
				COUNT(DISTINCT ` + socialStealIdentityExpression + `) AS stolen_by_me_count,
				COUNT(*) AS stolen_by_me_record_count
			FROM social_steal_records
			WHERE ` + stealPredicate + `
		)
		SELECT visitor_count, stolen_from_me_count, stolen_by_me_count, stolen_by_me_record_count
		FROM visitor_summary CROSS JOIN steal_summary`
	args := make([]any, 0, len(visitorArgs)+len(stealArgs))
	args = append(args, visitorArgs...)
	args = append(args, stealArgs...)

	var summary SocialRankingSummary
	err := tx.QueryRowContext(ctx, statement, args...).Scan(
		&summary.VisitorCount,
		&summary.StolenFromMeCount,
		&summary.StolenByMeCount,
		&summary.StolenByMeRecordCount,
	)
	return summary, err
}

func querySocialRankingTimeline(ctx context.Context, tx *sql.Tx, query SocialRankingQuery) ([]SocialRankingPageRow, bool, error) {
	statement, args, err := buildSocialTimelineQuery(query)
	if err != nil {
		return nil, false, err
	}
	rows, err := tx.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	result := make([]SocialRankingPageRow, 0, query.Limit+1)
	for rows.Next() {
		var row SocialRankingPageRow
		switch query.Tab {
		case "stolenByMe":
			var gid int
			if err := rows.Scan(&row.Key, &row.TimeMS, &gid, &row.DisplayName, &row.PayloadJSON); err != nil {
				return nil, false, err
			}
			row.Kind = "stealRecord"
			row.IdentityKey = socialStealIdentity(gid, row.DisplayName)
		case "stolenFromMe", "visitors":
			var playerID int
			if err := rows.Scan(
				&row.Key, &row.TimeMS, &playerID, &row.DisplayName,
				&row.ActionType, &row.StealCount, &row.PayloadJSON,
			); err != nil {
				return nil, false, err
			}
			row.Kind = "visitorRecord"
			row.IdentityKey = socialVisitorIdentity(playerID, row.DisplayName)
			if query.Tab == "stolenFromMe" && playerID > 0 {
				row.ActionTarget = strconv.Itoa(playerID)
			}
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(result) > query.Limit
	if hasMore {
		result = result[:query.Limit]
	}
	return result, hasMore, nil
}

func buildSocialTimelineQuery(query SocialRankingQuery) (string, []any, error) {
	if query.Limit < 1 {
		return "", nil, fmt.Errorf("social ranking limit must be positive")
	}
	if query.StartMS != nil && query.EndMS != nil && *query.EndMS < *query.StartMS {
		return "", nil, fmt.Errorf("social ranking end cannot be before start")
	}

	var selectClause string
	switch query.Tab {
	case "stolenByMe":
		selectClause = `SELECT id, occurred_at_ms, gid, display_name, payload_json FROM social_steal_records`
	case "stolenFromMe":
		selectClause = `SELECT id, occurred_at_ms, player_id, display_name, action_type, steal_item_num, payload_json FROM social_visitor_records`
	case "visitors":
		selectClause = `SELECT id, occurred_at_ms, player_id, display_name, action_type, steal_item_num, payload_json FROM social_visitor_records`
	default:
		return "", nil, fmt.Errorf("unsupported social ranking tab %q", query.Tab)
	}

	where, args := socialRankingWindowPredicates(normalizeSocialAccountKey(query.AccountKey), query.StartMS, query.EndMS)
	if query.Tab == "stolenFromMe" {
		where = append(where, "action_type = 1")
	}
	if query.Cursor != nil {
		if strings.TrimSpace(query.Cursor.Key) == "" {
			return "", nil, fmt.Errorf("social ranking cursor key is required")
		}
		where = append(where, "(occurred_at_ms < ? OR (occurred_at_ms = ? AND id < ?))")
		args = append(args, query.Cursor.TimeMS, query.Cursor.TimeMS, query.Cursor.Key)
	}
	args = append(args, query.Limit+1)

	statement := selectClause + " WHERE " + strings.Join(where, " AND ") +
		" ORDER BY occurred_at_ms DESC, id DESC LIMIT ?"
	return statement, args, nil
}

func socialRankingWindowPredicates(accountKey string, startMS, endMS *int64) ([]string, []any) {
	where := []string{"account_key = ?"}
	args := []any{accountKey}
	if startMS != nil {
		where = append(where, "occurred_at_ms >= ?")
		args = append(args, *startMS)
	}
	if endMS != nil {
		where = append(where, "occurred_at_ms <= ?")
		args = append(args, *endMS)
	}
	return where, args
}

func socialStealIdentity(gid int, displayName string) string {
	if gid > 0 {
		return "gid:" + strconv.Itoa(gid)
	}
	return "name:" + strings.TrimSpace(displayName)
}

func socialVisitorIdentity(playerID int, displayName string) string {
	if playerID > 0 {
		return "player:" + strconv.Itoa(playerID)
	}
	return "name:" + strings.TrimSpace(displayName)
}
