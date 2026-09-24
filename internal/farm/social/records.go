package social

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type StealRecord struct {
	ID          string         `json:"id"`
	GID         int            `json:"gid"`
	Name        string         `json:"name,omitempty"`
	DisplayName string         `json:"displayName"`
	AvatarURL   string         `json:"avatarUrl,omitempty"`
	Level       int            `json:"level,omitempty"`
	StealCount  int            `json:"stealCount"`
	Action      string         `json:"action"`
	OccurredAt  string         `json:"occurredAt"`
	LandIDs     []int          `json:"landIds,omitempty"`
	Items       []StealItem    `json:"items,omitempty"`
	Raw         map[string]any `json:"raw,omitempty"`
}

type StealItem struct {
	ItemID  int    `json:"itemId"`
	Name    string `json:"name"`
	Count   int    `json:"count"`
	LandIDs []int  `json:"landIds,omitempty"`
}

type StealRankingRow struct {
	GID            int    `json:"gid,omitempty"`
	Name           string `json:"name,omitempty"`
	DisplayName    string `json:"displayName"`
	AvatarURL      string `json:"avatarUrl,omitempty"`
	Level          int    `json:"level,omitempty"`
	StealCount     int    `json:"stealCount"`
	EventCount     int    `json:"eventCount"`
	LastOccurredAt string `json:"lastOccurredAt,omitempty"`
}

type StealRecordOptions struct {
	Action     string
	OccurredAt time.Time
}

type VisitorRecord struct {
	ID               string `json:"id"`
	PlayerID         int    `json:"playerId"`
	Name             string `json:"name,omitempty"`
	DisplayName      string `json:"displayName"`
	AvatarURL        string `json:"avatarUrl,omitempty"`
	ActionType       int    `json:"actionType"`
	Time             int64  `json:"time"`
	StealItemID      int    `json:"stealItemId,omitempty"`
	StealItemName    string `json:"stealItemName,omitempty"`
	StealItemNum     int    `json:"stealItemNum,omitempty"`
	Count            int    `json:"count,omitempty"`
	Level            int    `json:"level,omitempty"`
	AuthorizedStatus int    `json:"authorizedStatus,omitempty"`
	HostType         int    `json:"hostType,omitempty"`
	ActionLabel      string `json:"actionLabel"`
}

type VisitorRankingRow struct {
	PlayerID    int    `json:"playerId,omitempty"`
	Name        string `json:"name,omitempty"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
	Level       int    `json:"level,omitempty"`
	EventCount  int    `json:"eventCount"`
	StealCount  int    `json:"stealCount"`
	LastTime    int64  `json:"lastTime"`
}

type VisitorSummary struct {
	VisitorRecords []VisitorRecord     `json:"visitorRecords"`
	StolenFromMe   []VisitorRankingRow `json:"stolenFromMe"`
}

type DateRangeWindow struct {
	Range   string
	StartMS *int64
	EndMS   *int64
}

func NormalizeRankingDateRange(value string) string {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "today" {
		return "current"
	}
	switch text {
	case "current", "3d", "7d", "30d", "all":
		return text
	default:
		return "current"
	}
}

func ResolveDateRangeWindow(value string, now time.Time) DateRangeWindow {
	normalized := NormalizeRankingDateRange(value)
	days := map[string]int{"current": 1, "3d": 3, "7d": 7, "30d": 30}[normalized]
	if days <= 0 {
		return DateRangeWindow{Range: normalized}
	}
	if now.IsZero() {
		now = time.Now()
	}
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if days > 1 {
		start = start.AddDate(0, 0, -(days - 1))
	}
	startMS := start.UnixMilli()
	endMS := now.UnixMilli()
	return DateRangeWindow{Range: normalized, StartMS: &startMS, EndMS: &endMS}
}

func FilterStealRecords(records []StealRecord, window DateRangeWindow) []StealRecord {
	if window.StartMS == nil || window.EndMS == nil {
		return normalizeStealRecords(records)
	}
	filtered := []StealRecord{}
	for _, record := range normalizeStealRecords(records) {
		parsed, err := time.Parse(time.RFC3339, record.OccurredAt)
		if err != nil {
			continue
		}
		ms := parsed.UnixMilli()
		if ms >= *window.StartMS && ms <= *window.EndMS {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func FilterVisitorRecords(records []VisitorRecord, window DateRangeWindow) []VisitorRecord {
	if window.StartMS == nil || window.EndMS == nil {
		return normalizeVisitorRecords(records)
	}
	filtered := []VisitorRecord{}
	for _, record := range normalizeVisitorRecords(records) {
		ms := visitorRecordTimeMS(record)
		if ms >= *window.StartMS && ms <= *window.EndMS {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func SummarizeStealRanking(records []StealRecord) []StealRankingRow {
	byKey := map[string]*StealRankingRow{}
	for _, record := range normalizeStealRecords(records) {
		key := record.DisplayName
		if record.GID > 0 {
			key = stringKey(record.GID)
		}
		row := byKey[key]
		if row == nil {
			row = &StealRankingRow{
				GID:         record.GID,
				Name:        record.Name,
				DisplayName: record.DisplayName,
				AvatarURL:   record.AvatarURL,
				Level:       record.Level,
			}
			byKey[key] = row
		}
		row.StealCount++
		row.EventCount++
		if record.OccurredAt > row.LastOccurredAt {
			row.LastOccurredAt = record.OccurredAt
		}
	}
	result := make([]StealRankingRow, 0, len(byKey))
	for _, row := range byKey {
		result = append(result, *row)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].StealCount != result[j].StealCount {
			return result[i].StealCount > result[j].StealCount
		}
		if result[i].EventCount != result[j].EventCount {
			return result[i].EventCount > result[j].EventCount
		}
		return result[i].LastOccurredAt > result[j].LastOccurredAt
	})
	return result
}

func BuildStealRecord(gid int, status map[string]any, landIDs []int, harvestValue any, opts StealRecordOptions) StealRecord {
	friend := mapFromAny(firstExistingAny(status["friend"], status["friendInfo"], status["owner"], status["user"], status["basic"]))
	displayName := firstText(friend["displayName"], friend["remark"], friend["name"], status["displayName"], status["name"], fmt.Sprint(gid))
	occurredAt := opts.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	action := strings.TrimSpace(opts.Action)
	if action == "" {
		action = "steal"
	}
	record := StealRecord{
		GID:         gid,
		Name:        firstText(friend["name"], status["name"]),
		DisplayName: displayName,
		AvatarURL:   firstText(friend["avatarUrl"], friend["avatar"], status["avatarUrl"], status["avatar"]),
		Level:       PositiveInt(firstExistingAny(friend["level"], status["level"])),
		StealCount:  1,
		Action:      action,
		OccurredAt:  occurredAt.Format(time.RFC3339Nano),
		LandIDs:     normalizeIntIDs(landIDs),
		Items:       buildStealItems(status, landIDs, harvestValue),
	}
	record.ID = fmt.Sprintf("%d:%s:%s", record.GID, record.OccurredAt, record.Action)
	if raw := mapFromAny(harvestValue); len(raw) > 0 {
		record.Raw = raw
	}
	return record
}

func SummarizeVisitorRankings(records []VisitorRecord) VisitorSummary {
	visitorRecords := normalizeVisitorRecords(records)
	byKey := map[string]*VisitorRankingRow{}
	for _, record := range visitorRecords {
		if record.ActionType != 1 {
			continue
		}
		key := record.DisplayName
		if record.PlayerID > 0 {
			key = stringKey(record.PlayerID)
		}
		row := byKey[key]
		if row == nil {
			row = &VisitorRankingRow{
				PlayerID:    record.PlayerID,
				Name:        record.Name,
				DisplayName: record.DisplayName,
				AvatarURL:   record.AvatarURL,
				Level:       record.Level,
			}
			byKey[key] = row
		}
		row.EventCount++
		row.StealCount += record.StealItemNum
		if record.Time > row.LastTime {
			row.LastTime = record.Time
		}
	}
	stolenFromMe := make([]VisitorRankingRow, 0, len(byKey))
	for _, row := range byKey {
		stolenFromMe = append(stolenFromMe, *row)
	}
	sort.SliceStable(stolenFromMe, func(i, j int) bool {
		if stolenFromMe[i].StealCount != stolenFromMe[j].StealCount {
			return stolenFromMe[i].StealCount > stolenFromMe[j].StealCount
		}
		if stolenFromMe[i].EventCount != stolenFromMe[j].EventCount {
			return stolenFromMe[i].EventCount > stolenFromMe[j].EventCount
		}
		return stolenFromMe[i].LastTime > stolenFromMe[j].LastTime
	})
	return VisitorSummary{VisitorRecords: visitorRecords, StolenFromMe: stolenFromMe}
}

func normalizeStealRecords(records []StealRecord) []StealRecord {
	out := make([]StealRecord, 0, len(records))
	seen := map[string]bool{}
	for _, record := range records {
		if record.StealCount <= 0 {
			record.StealCount = 1
		}
		if record.DisplayName == "" {
			if record.GID > 0 {
				record.DisplayName = stringKey(record.GID)
			} else {
				record.DisplayName = "未知好友"
			}
		}
		if record.ID == "" {
			record.ID = record.DisplayName + ":" + record.OccurredAt
		}
		if seen[record.ID] {
			continue
		}
		seen[record.ID] = true
		out = append(out, record)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].OccurredAt > out[j].OccurredAt
	})
	if len(out) > 1000 {
		return out[:1000]
	}
	return out
}

func buildStealItems(status map[string]any, landIDs []int, harvestValue any) []StealItem {
	selected := intSet(normalizeIntIDs(landIDs))
	items := stealItemsFromHarvest(harvestValue)
	if len(items) == 0 {
		items = stealItemsFromInspectLands(status, selected)
	}
	return normalizeStealItems(items)
}

func stealItemsFromHarvest(value any) []StealItem {
	payload := mapFromAny(value)
	if len(payload) == 0 {
		return nil
	}
	candidates := [][]any{
		sliceFromAny(payload["items"]),
		sliceFromAny(payload["rewards"]),
		sliceFromAny(payload["harvestItems"]),
		sliceFromAny(payload["harvestedItems"]),
		sliceFromAny(payload["itemUpdates"]),
		sliceFromAny(payload["results"]),
		sliceFromAny(payload["list"]),
	}
	if data := mapFromAny(payload["data"]); len(data) > 0 {
		candidates = append(candidates,
			sliceFromAny(data["items"]),
			sliceFromAny(data["rewards"]),
			sliceFromAny(data["harvestItems"]),
			sliceFromAny(data["harvestedItems"]),
			sliceFromAny(data["itemUpdates"]),
		)
	}
	out := []StealItem{}
	for _, candidate := range candidates {
		for _, rawItem := range candidate {
			item := stealItemFromMap(mapFromAny(rawItem))
			if item.ItemID <= 0 && item.Name == "" {
				continue
			}
			out = append(out, item)
		}
	}
	return out
}

func stealItemFromMap(raw map[string]any) StealItem {
	if len(raw) == 0 {
		return StealItem{}
	}
	landIDs := normalizeAnyIntIDs(firstExistingAny(raw["landIds"], raw["landIDs"], raw["lands"], raw["land_ids"]))
	count := PositiveInt(firstExistingAny(raw["count"], raw["num"], raw["amount"], raw["quantity"], raw["itemNum"], raw["item_num"], raw["stealCount"]))
	if count <= 0 {
		count = len(landIDs)
	}
	if count <= 0 {
		count = 1
	}
	return StealItem{
		ItemID:  PositiveInt(firstExistingAny(raw["itemId"], raw["itemID"], raw["id"], raw["goodsId"], raw["plantId"], raw["cropId"])),
		Name:    firstText(raw["name"], raw["itemName"], raw["plantName"], raw["cropName"], raw["displayName"]),
		Count:   count,
		LandIDs: landIDs,
	}
}

func stealItemsFromInspectLands(status map[string]any, selected map[int]bool) []StealItem {
	if len(status) == 0 || len(selected) == 0 {
		return nil
	}
	items := []StealItem{}
	for _, rawLand := range sliceFromAny(status["lands"]) {
		land := mapFromAny(rawLand)
		if len(land) == 0 {
			continue
		}
		landID := PositiveInt(firstExistingAny(land["landId"], land["landID"], land["id"], land["land_id"]))
		if landID <= 0 || !selected[landID] {
			continue
		}
		plant := mapFromAny(firstExistingAny(land["plantInfo"], land["plant"], land["plant_info"], land["crop"], land["cropInfo"]))
		if len(plant) == 0 {
			plant = land
		}
		itemID := PositiveInt(firstExistingAny(plant["itemId"], plant["itemID"], plant["id"], plant["plantId"], plant["cropId"], land["plantId"], land["cropId"]))
		name := firstText(plant["name"], plant["itemName"], plant["plantName"], plant["cropName"], land["plantName"], land["cropName"])
		if itemID <= 0 && name == "" {
			continue
		}
		items = append(items, StealItem{ItemID: itemID, Name: name, Count: 1, LandIDs: []int{landID}})
	}
	return items
}

func normalizeStealItems(items []StealItem) []StealItem {
	type key struct {
		id   int
		name string
	}
	ordered := []key{}
	byKey := map[key]*StealItem{}
	for _, item := range items {
		if item.ItemID <= 0 && item.Name == "" {
			continue
		}
		if item.Count <= 0 {
			item.Count = 1
		}
		item.LandIDs = normalizeIntIDs(item.LandIDs)
		k := key{id: item.ItemID, name: item.Name}
		existing := byKey[k]
		if existing == nil {
			copyItem := item
			byKey[k] = &copyItem
			ordered = append(ordered, k)
			continue
		}
		existing.Count += item.Count
		existing.LandIDs = normalizeIntIDs(append(existing.LandIDs, item.LandIDs...))
	}
	out := make([]StealItem, 0, len(ordered))
	for _, k := range ordered {
		out = append(out, *byKey[k])
	}
	return out
}

func normalizeIntIDs(values []int) []int {
	seen := map[int]bool{}
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func normalizeAnyIntIDs(value any) []int {
	out := []int{}
	for _, item := range sliceFromAny(value) {
		if id := PositiveInt(item); id > 0 {
			out = append(out, id)
		}
	}
	return normalizeIntIDs(out)
}

func intSet(values []int) map[int]bool {
	out := map[int]bool{}
	for _, value := range values {
		out[value] = true
	}
	return out
}

func normalizeVisitorRecords(records []VisitorRecord) []VisitorRecord {
	out := make([]VisitorRecord, 0, len(records))
	seen := map[string]bool{}
	for _, record := range records {
		if record.DisplayName == "" {
			if record.PlayerID > 0 {
				record.DisplayName = stringKey(record.PlayerID)
			} else {
				record.DisplayName = "未知访客"
			}
		}
		if record.ActionLabel == "" {
			record.ActionLabel = FormatVisitorAction(record)
		}
		if record.ID == "" {
			record.ID = record.DisplayName + ":" + stringKey(int(record.Time)) + ":" + stringKey(record.ActionType)
		}
		if seen[record.ID] {
			continue
		}
		seen[record.ID] = true
		out = append(out, record)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Time > out[j].Time
	})
	if len(out) > 1000 {
		return out[:1000]
	}
	return out
}

func FormatVisitorAction(record VisitorRecord) string {
	switch record.ActionType {
	case 1:
		itemName := record.StealItemName
		if itemName == "" {
			itemName = "物品"
		}
		if record.StealItemNum > 0 {
			return "摘取" + itemName + stringKey(record.StealItemNum) + "个"
		}
		return "摘取" + itemName
	case 2:
		if record.Count > 0 {
			return "帮你照顾了农场" + stringKey(record.Count) + "次"
		}
		return "帮你照顾了农场"
	default:
		return "访问了农场"
	}
}

func stringKey(value int) string {
	return strconv.Itoa(value)
}

func visitorRecordTimeMS(record VisitorRecord) int64 {
	if record.Time > 1e10 {
		return record.Time
	}
	return record.Time * 1000
}
