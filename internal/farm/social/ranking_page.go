package social

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	rankingCursorVersion = 1
	defaultRankingLimit  = 50
	maximumRankingLimit  = 100
)

type RankingCursor struct {
	TimeMS      int64
	Key         string
	IdentityKey string
	StealCount  int
	EventCount  int
	RankOffset  int
	StartMS     *int64
	EndMS       int64
}

type RankingQuery struct {
	Tab       string
	ViewMode  string
	Window    DateRangeWindow
	Cursor    *RankingCursor
	Limit     int
	ScopeHash string
}

type RankingPageData struct {
	Summary RankingSummary
	Rows    []RankingPageRow
	HasMore bool
}

func (s *Service) Rankings(ctx context.Context, request RankingRequest) RankingPage {
	query, err := NormalizeRankingRequest(s.accountKey, request)
	if err != nil {
		return failedRankingPage(request, "排行榜请求无效："+err.Error())
	}

	data := RankingPageData{Rows: []RankingPageRow{}}
	if store, ok := s.store.(RankingPageStore); ok {
		data, err = store.QueryRankingPage(ctx, s.accountKey, query)
		if err != nil {
			return failedRankingPage(request, "读取排行榜失败："+err.Error())
		}
	}
	if data.Rows == nil {
		data.Rows = []RankingPageRow{}
	}
	for index := range data.Rows {
		if data.Rows[index].Items == nil {
			data.Rows[index].Items = []StealItem{}
		}
	}

	page := RankingPage{
		OK: true, Status: StatusOK, Message: "排行榜数据已更新。",
		Tab: query.Tab, ViewMode: query.ViewMode, DateRange: query.Window.Range,
		Summary: data.Summary, Rows: data.Rows, HasMore: data.HasMore,
	}
	if data.HasMore {
		if len(data.Rows) == 0 {
			return failedRankingPage(request, "读取排行榜失败：分页结果缺少游标行")
		}
		page.NextCursor, err = EncodeRankingCursor(query, data.Rows[len(data.Rows)-1])
		if err != nil {
			return failedRankingPage(request, "生成排行榜游标失败："+err.Error())
		}
	}
	return page
}

func failedRankingPage(request RankingRequest, message string) RankingPage {
	return RankingPage{
		Status: StatusFailed, Message: message,
		Tab: request.Tab, ViewMode: request.ViewMode, DateRange: request.DateRange,
		Rows: []RankingPageRow{},
	}
}

type rankingCursorPayload struct {
	Version     int    `json:"version"`
	ScopeHash   string `json:"scopeHash"`
	TimeMS      *int64 `json:"timeMS"`
	Key         string `json:"key"`
	IdentityKey string `json:"identityKey,omitempty"`
	StealCount  *int   `json:"stealCount,omitempty"`
	EventCount  *int   `json:"eventCount,omitempty"`
	RankOffset  *int   `json:"rankOffset,omitempty"`
	StartMS     *int64 `json:"startMS,omitempty"`
	EndMS       *int64 `json:"endMS"`
}

type rankingCursorScope struct {
	AccountKey string `json:"accountKey"`
	Tab        string `json:"tab"`
	ViewMode   string `json:"viewMode"`
	DateRange  string `json:"dateRange"`
	StartMS    *int64 `json:"startMS"`
	EndMS      int64  `json:"endMS"`
}

func NormalizeRankingRequest(accountKey string, request RankingRequest) (RankingQuery, error) {
	if strings.TrimSpace(accountKey) == "" {
		return RankingQuery{}, errors.New("ranking account key is required")
	}
	tab, err := normalizeRankingTab(request.Tab)
	if err != nil {
		return RankingQuery{}, err
	}
	viewMode, err := normalizeRankingViewMode(request.ViewMode)
	if err != nil {
		return RankingQuery{}, err
	}
	if tab == "visitors" {
		viewMode = "timeline"
	}
	dateRange, err := normalizeRankingRequestDateRange(request.DateRange)
	if err != nil {
		return RankingQuery{}, err
	}
	limit := request.Limit
	if limit == 0 {
		limit = defaultRankingLimit
	}
	if limit < 1 || limit > maximumRankingLimit {
		return RankingQuery{}, fmt.Errorf("ranking limit must be between 1 and %d", maximumRankingLimit)
	}

	if request.Cursor != "" {
		payload, err := decodeRankingCursor(request.Cursor)
		if err != nil {
			return RankingQuery{}, err
		}
		window := DateRangeWindow{Range: dateRange, StartMS: payload.StartMS, EndMS: payload.EndMS}
		if err := validateRankingWindow(window); err != nil {
			return RankingQuery{}, err
		}
		scopeHash, err := rankingScopeHash(accountKey, tab, viewMode, window)
		if err != nil {
			return RankingQuery{}, err
		}
		if payload.ScopeHash != scopeHash {
			return RankingQuery{}, errors.New("ranking cursor does not match the request scope")
		}
		cursor := RankingCursor{
			TimeMS: *payload.TimeMS, Key: payload.Key, IdentityKey: payload.IdentityKey,
			StartMS: payload.StartMS, EndMS: *payload.EndMS,
		}
		if viewMode == "ranking" {
			if payload.StealCount == nil || payload.EventCount == nil || payload.RankOffset == nil {
				return RankingQuery{}, errors.New("ranking cursor sort fields are missing")
			}
			cursor.StealCount = *payload.StealCount
			cursor.EventCount = *payload.EventCount
			cursor.RankOffset = *payload.RankOffset
		}
		if err := validateRankingCursor(viewMode, cursor); err != nil {
			return RankingQuery{}, err
		}
		return RankingQuery{
			Tab: tab, ViewMode: viewMode, Window: window, Cursor: &cursor,
			Limit: limit, ScopeHash: scopeHash,
		}, nil
	}

	now := request.Now
	if now.IsZero() {
		now = time.Now()
	}
	window := ResolveDateRangeWindow(dateRange, now)
	window.EndMS = int64Pointer(now.UnixMilli())
	if err := validateRankingWindow(window); err != nil {
		return RankingQuery{}, err
	}
	scopeHash, err := rankingScopeHash(accountKey, tab, viewMode, window)
	if err != nil {
		return RankingQuery{}, err
	}
	return RankingQuery{
		Tab: tab, ViewMode: viewMode, Window: window,
		Limit: limit, ScopeHash: scopeHash,
	}, nil
}

func EncodeRankingCursor(query RankingQuery, row RankingPageRow) (string, error) {
	if err := validateRankingWindow(query.Window); err != nil {
		return "", err
	}
	if query.ScopeHash == "" {
		return "", errors.New("ranking cursor scope is required")
	}
	if strings.TrimSpace(row.Key) == "" {
		return "", errors.New("ranking cursor key is required")
	}
	if query.ViewMode == "ranking" {
		if strings.TrimSpace(row.IdentityKey) == "" {
			return "", errors.New("ranking cursor identity key is required")
		}
		if row.Rank < 1 {
			return "", errors.New("ranking cursor rank offset is required")
		}
		if row.StealCount < 0 || row.EventCount < 0 {
			return "", errors.New("ranking cursor counts cannot be negative")
		}
	}

	payload := rankingCursorPayload{
		Version: rankingCursorVersion, ScopeHash: query.ScopeHash,
		TimeMS: int64Pointer(row.TimeMS), Key: row.Key, IdentityKey: row.IdentityKey,
		StartMS: query.Window.StartMS, EndMS: query.Window.EndMS,
	}
	if query.ViewMode == "ranking" {
		payload.StealCount = intPointer(row.StealCount)
		payload.EventCount = intPointer(row.EventCount)
		payload.RankOffset = intPointer(row.Rank)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode ranking cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeRankingCursor(encoded string) (rankingCursorPayload, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return rankingCursorPayload{}, fmt.Errorf("decode ranking cursor: %w", err)
	}
	var payload rankingCursorPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return rankingCursorPayload{}, fmt.Errorf("decode ranking cursor payload: %w", err)
	}
	if payload.Version != rankingCursorVersion {
		return rankingCursorPayload{}, fmt.Errorf("unsupported ranking cursor version %d", payload.Version)
	}
	if payload.ScopeHash == "" {
		return rankingCursorPayload{}, errors.New("ranking cursor scope is missing")
	}
	if payload.TimeMS == nil {
		return rankingCursorPayload{}, errors.New("ranking cursor timestamp is missing")
	}
	if payload.EndMS == nil {
		return rankingCursorPayload{}, errors.New("ranking cursor window end is missing")
	}
	return payload, nil
}

func rankingScopeHash(accountKey, tab, viewMode string, window DateRangeWindow) (string, error) {
	if window.EndMS == nil {
		return "", errors.New("ranking window end is required")
	}
	scope := rankingCursorScope{
		AccountKey: accountKey, Tab: tab, ViewMode: viewMode, DateRange: window.Range,
		StartMS: window.StartMS, EndMS: *window.EndMS,
	}
	raw, err := json.Marshal(scope)
	if err != nil {
		return "", fmt.Errorf("encode ranking cursor scope: %w", err)
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func normalizeRankingTab(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "stolenByMe", "stolenFromMe", "visitors":
		return strings.TrimSpace(value), nil
	default:
		return "", fmt.Errorf("invalid ranking tab %q", value)
	}
}

func normalizeRankingViewMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "timeline":
		return "timeline", nil
	case "ranking":
		return "ranking", nil
	default:
		return "", fmt.Errorf("invalid ranking view mode %q", value)
	}
}

func normalizeRankingRequestDateRange(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "current", nil
	}
	switch normalized {
	case "current", "3d", "7d", "30d", "all":
		return normalized, nil
	default:
		return "", fmt.Errorf("invalid ranking date range %q", value)
	}
}

func validateRankingWindow(window DateRangeWindow) error {
	if window.EndMS == nil {
		return errors.New("ranking window end is required")
	}
	if window.StartMS != nil && *window.EndMS < *window.StartMS {
		return errors.New("ranking window end cannot be before its start")
	}
	return nil
}

func validateRankingCursor(viewMode string, cursor RankingCursor) error {
	if strings.TrimSpace(cursor.Key) == "" {
		return errors.New("ranking cursor key is required")
	}
	if viewMode == "ranking" {
		if strings.TrimSpace(cursor.IdentityKey) == "" {
			return errors.New("ranking cursor identity key is required")
		}
		if cursor.RankOffset < 1 {
			return errors.New("ranking cursor rank offset is required")
		}
		if cursor.StealCount < 0 || cursor.EventCount < 0 {
			return errors.New("ranking cursor counts cannot be negative")
		}
	}
	return nil
}

func int64Pointer(value int64) *int64 {
	return &value
}

func intPointer(value int) *int {
	return &value
}
