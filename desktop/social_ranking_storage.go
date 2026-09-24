package desktop

import (
	"context"
	"encoding/json"
	"fmt"

	"Farm_Go/internal/farm/social"
	"Farm_Go/internal/storage"
)

func (s socialStorageAdapter) QueryRankingPage(ctx context.Context, accountKey string, query social.RankingQuery) (social.RankingPageData, error) {
	storageQuery := storage.SocialRankingQuery{
		AccountKey: accountKey,
		Tab:        query.Tab,
		ViewMode:   query.ViewMode,
		StartMS:    query.Window.StartMS,
		EndMS:      query.Window.EndMS,
		Limit:      query.Limit,
	}
	if query.Cursor != nil {
		storageQuery.Cursor = &storage.SocialRankingCursor{
			TimeMS: query.Cursor.TimeMS, Key: query.Cursor.Key, IdentityKey: query.Cursor.IdentityKey,
			StealCount: query.Cursor.StealCount, EventCount: query.Cursor.EventCount, RankOffset: query.Cursor.RankOffset,
		}
	}

	page, err := s.store.QuerySocialRankingPage(ctx, storageQuery)
	if err != nil {
		return social.RankingPageData{}, err
	}
	result := social.RankingPageData{
		Summary: social.RankingSummary{
			VisitorCount: page.Summary.VisitorCount, StolenFromMeCount: page.Summary.StolenFromMeCount,
			StolenByMeCount: page.Summary.StolenByMeCount, StolenByMeRecordCount: page.Summary.StolenByMeRecordCount,
		},
		Rows:    make([]social.RankingPageRow, 0, len(page.Rows)),
		HasMore: page.HasMore,
	}
	for _, row := range page.Rows {
		mapped, err := mapSocialRankingStorageRow(row)
		if err != nil {
			return social.RankingPageData{}, err
		}
		result.Rows = append(result.Rows, mapped)
	}
	return result, nil
}

func mapSocialRankingStorageRow(row storage.SocialRankingPageRow) (social.RankingPageRow, error) {
	mapped := social.RankingPageRow{
		Kind: row.Kind, Key: row.Key, IdentityKey: row.IdentityKey,
		TimeMS: row.TimeMS, DisplayName: row.DisplayName, Rank: row.Rank,
		EventCount: row.EventCount, StealCount: row.StealCount,
		Items: []social.StealItem{}, ActionType: row.ActionType,
		ActionLabel: row.ActionLabel, ActionTarget: row.ActionTarget,
	}
	payloads := row.PayloadJSONs
	if row.PayloadJSON != "" {
		payloads = append([]string{row.PayloadJSON}, payloads...)
	}
	if len(payloads) == 0 {
		return mapped, nil
	}

	if row.Kind == "stealRecord" {
		var record social.StealRecord
		if err := json.Unmarshal([]byte(payloads[0]), &record); err != nil {
			return social.RankingPageRow{}, fmt.Errorf("decode social ranking row %q: %w", row.Key, err)
		}
		mapped.StealCount = record.StealCount
		if mapped.StealCount <= 0 {
			mapped.StealCount = 1
		}
		mapped.Items = make([]social.StealItem, len(record.Items))
		for index, item := range record.Items {
			item.LandIDs = append([]int(nil), item.LandIDs...)
			mapped.Items[index] = item
		}
		return mapped, nil
	}
	if row.Kind == "stealRanking" {
		items, err := stealItemsFromRankingPayloads(payloads)
		if err != nil {
			return social.RankingPageRow{}, fmt.Errorf("decode social ranking row %q: %w", row.Key, err)
		}
		mapped.Items = items
		return mapped, nil
	}

	var record social.VisitorRecord
	if err := json.Unmarshal([]byte(payloads[0]), &record); err != nil {
		return social.RankingPageRow{}, fmt.Errorf("decode social ranking row %q: %w", row.Key, err)
	}
	if mapped.ActionType == 0 {
		mapped.ActionType = record.ActionType
	}
	if mapped.ActionLabel == "" {
		mapped.ActionLabel = record.ActionLabel
		if mapped.ActionLabel == "" {
			mapped.ActionLabel = social.FormatVisitorAction(record)
		}
	}
	return mapped, nil
}

func stealItemsFromRankingPayloads(payloads []string) ([]social.StealItem, error) {
	type itemKey struct {
		id   int
		name string
	}
	order := make([]itemKey, 0)
	items := make(map[itemKey]*social.StealItem)
	for _, payload := range payloads {
		var record social.StealRecord
		if err := json.Unmarshal([]byte(payload), &record); err != nil {
			return nil, err
		}
		for _, item := range record.Items {
			key := itemKey{id: item.ItemID, name: item.Name}
			existing := items[key]
			if existing == nil {
				item.LandIDs = append([]int(nil), item.LandIDs...)
				items[key] = &item
				order = append(order, key)
				continue
			}
			existing.Count += item.Count
			existing.LandIDs = appendUniqueSocialLandIDs(existing.LandIDs, item.LandIDs)
		}
	}
	result := make([]social.StealItem, 0, len(order))
	for _, key := range order {
		result = append(result, *items[key])
	}
	return result, nil
}

func appendUniqueSocialLandIDs(current, added []int) []int {
	seen := make(map[int]struct{}, len(current)+len(added))
	result := make([]int, 0, len(current)+len(added))
	for _, values := range [][]int{current, added} {
		for _, value := range values {
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}
