package farm

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type roleLevelExperience struct {
	Level int   `json:"level"`
	Exp   int64 `json:"exp"`
}

var (
	roleLevelExperienceOnce sync.Once
	roleLevelExperienceByID map[int]int64
)

type AccountStatusPayload struct {
	Status          string                    `json:"status"`
	Message         string                    `json:"message"`
	Profile         AccountProfile            `json:"profile"`
	Fertilizer      FertilizerContainerStatus `json:"fertilizer"`
	ProfileError    string                    `json:"profileError,omitempty"`
	FertilizerError string                    `json:"fertilizerError,omitempty"`
}

type AccountProfile struct {
	GID           int64              `json:"gid,omitempty"`
	Name          string             `json:"name,omitempty"`
	Nick          string             `json:"nick,omitempty"`
	Level         int                `json:"level,omitempty"`
	PlantLevel    int                `json:"plantLevel,omitempty"`
	MaxLandLevel  int                `json:"farmMaxLandLevel,omitempty"`
	Exp           int64              `json:"exp,omitempty"`
	NextLevelExp  int64              `json:"nextLevelExp,omitempty"`
	Gold          int64              `json:"gold,omitempty"`
	Money         int64              `json:"money,omitempty"`
	Bean          int64              `json:"bean,omitempty"`
	Coupon        int64              `json:"coupon,omitempty"`
	Diamond       int64              `json:"diamond,omitempty"`
	AvatarURL     string             `json:"avatarUrl,omitempty"`
	LevelProgress LevelProgress      `json:"levelProgress"`
	TodayStats    DayStats           `json:"todayStats"`
	StatsHistory  StatsHistoryWindow `json:"statsHistory"`
}

type LevelProgress struct {
	Level     int   `json:"level,omitempty"`
	Current   int64 `json:"current"`
	Needed    int64 `json:"needed"`
	Remaining int64 `json:"remaining"`
	Percent   int   `json:"percent"`
	NextLevel int   `json:"nextLevel,omitempty"`
}

type DayStats struct {
	DateKey       string `json:"dateKey,omitempty"`
	UpdatedAt     string `json:"updatedAt,omitempty"`
	Runs          int    `json:"runs"`
	Collect       int    `json:"collect"`
	Water         int    `json:"water"`
	Steal         int    `json:"steal"`
	Help          int    `json:"help"`
	MischiefGrass int    `json:"mischiefGrass"`
	MischiefBug   int    `json:"mischiefBug"`
	Sell          int    `json:"sell"`
	SaleEstimate  int64  `json:"saleEstimate"`
	EstimateReady bool   `json:"estimateReady"`
}

type StatsHistoryWindow struct {
	TodayKey string     `json:"todayKey,omitempty"`
	Days     []DayStats `json:"days"`
}

type FertilizerContainerStatus struct {
	Normal  FertilizerSlotStatus `json:"normal"`
	Organic FertilizerSlotStatus `json:"organic"`
}

type FertilizerSlotStatus struct {
	Available      bool    `json:"available"`
	RemainingSec   int64   `json:"remainingSec"`
	RemainingHours float64 `json:"remainingHours"`
	RemainingText  string  `json:"remainingText,omitempty"`
}

func BuildRuntimeAccountStatus(profileRaw map[string]any, fertilizerRaw map[string]any, profileErr error, fertilizerErr error) AccountStatusPayload {
	payload := AccountStatusPayload{
		Status:     "runtime",
		Message:    "账户状态已从游戏运行时读取。",
		Profile:    BuildRuntimeAccountProfile(profileRaw),
		Fertilizer: BuildRuntimeFertilizerContainerStatus(fertilizerRaw),
	}
	if profileErr != nil {
		payload.ProfileError = profileErr.Error()
	}
	if fertilizerErr != nil {
		payload.FertilizerError = fertilizerErr.Error()
	}
	if profileErr != nil && fertilizerErr != nil {
		payload.Status = "runtime_error"
		payload.Message = "账户状态读取失败。"
	} else if profileErr != nil || fertilizerErr != nil {
		payload.Status = "partial"
		payload.Message = "账户状态部分读取失败。"
	}
	return payload
}

func BuildRuntimeAccountProfile(values map[string]any) AccountProfile {
	profile := AccountProfile{
		GID:          int64FromMap(values, "gid"),
		Name:         firstNonEmptyString(stringFromMap(values, "name"), stringFromMap(values, "nickname")),
		Nick:         stringFromMap(values, "nick"),
		Level:        firstPositiveInt(intFromMap(values, "level"), intFromMap(values, "plantLevel")),
		PlantLevel:   intFromMap(values, "plantLevel"),
		MaxLandLevel: intFromMap(values, "farmMaxLandLevel"),
		Exp:          firstNonZeroInt64(int64FromMap(values, "exp"), int64FromMap(values, "_exp"), int64FromMap(values, "curExp"), int64FromMap(values, "currentExp"), int64FromMap(values, "role_exp"), int64FromMap(values, "experience")),
		NextLevelExp: firstNonZeroInt64(int64FromMap(values, "nextLevelExp"), int64FromMap(values, "maxExp"), int64FromMap(values, "next_exp"), int64FromMap(values, "needExp"), int64FromMap(values, "targetExp")),
		Gold:         firstNonZeroInt64(int64FromMap(values, "gold"), int64FromMap(values, "money")),
		Money:        int64FromMap(values, "money"),
		Bean:         int64FromMap(values, "bean"),
		Coupon:       int64FromMap(values, "coupon"),
		Diamond:      int64FromMap(values, "diamond"),
		AvatarURL:    stringFromMap(values, "avatarUrl"),
		TodayStats:   buildDayStats(anyMap(values["todayStats"])),
		StatsHistory: buildStatsHistory(anyMap(values["statsHistory"])),
	}
	if profile.PlantLevel <= 0 {
		profile.PlantLevel = profile.Level
	}
	if profile.MaxLandLevel <= 0 {
		profile.MaxLandLevel = profile.Level
	}
	profile.LevelProgress = buildLevelProgress(values, profile)
	return profile
}

func BuildRuntimeFertilizerContainerStatus(values map[string]any) FertilizerContainerStatus {
	return FertilizerContainerStatus{
		Normal:  buildFertilizerSlot(anyMap(values["normal"])),
		Organic: buildFertilizerSlot(anyMap(values["organic"])),
	}
}

func buildLevelProgress(values map[string]any, profile AccountProfile) LevelProgress {
	raw := anyMap(values["levelProgress"])
	current := firstNonZeroInt64(int64FromMap(raw, "current"), int64FromMap(raw, "expInLevel"))
	needed := firstNonZeroInt64(int64FromMap(raw, "needed"), int64FromMap(raw, "nextLevelExp"))
	if needed == 0 {
		if start, next, ok := roleLevelExperienceWindow(profile.Level); ok {
			needed = next - start
			if profile.Exp >= start {
				current = profile.Exp - start
			} else {
				current = firstNonZeroInt64(current, profile.Exp)
			}
		} else {
			current = firstNonZeroInt64(current, profile.Exp)
			needed = profile.NextLevelExp
		}
	}
	remaining := needed - current
	if remaining < 0 {
		remaining = 0
	}
	percent := 0
	if needed > 0 {
		percent = int(math.Round((float64(current) / float64(needed)) * 100))
		if percent < 0 {
			percent = 0
		}
		if percent > 100 {
			percent = 100
		}
	}
	return LevelProgress{
		Level:     firstPositiveInt(intFromMap(raw, "level"), profile.Level),
		Current:   current,
		Needed:    needed,
		Remaining: remaining,
		Percent:   percent,
		NextLevel: firstPositiveInt(intFromMap(raw, "nextLevel"), profile.Level+1),
	}
}

func roleLevelExperienceWindow(level int) (int64, int64, bool) {
	if level <= 0 {
		return 0, 0, false
	}
	roleLevelExperienceOnce.Do(func() {
		data, err := os.ReadFile(filepath.Join(DefaultGameConfigRoot(), "RoleLevel.json"))
		if err != nil {
			return
		}
		var entries []roleLevelExperience
		if err := json.Unmarshal(data, &entries); err != nil {
			return
		}
		roleLevelExperienceByID = make(map[int]int64, len(entries))
		for _, entry := range entries {
			if entry.Level > 0 && entry.Exp >= 0 {
				roleLevelExperienceByID[entry.Level] = entry.Exp
			}
		}
	})
	start, hasStart := roleLevelExperienceByID[level]
	next, hasNext := roleLevelExperienceByID[level+1]
	return start, next, hasStart && hasNext && next > start
}

func buildStatsHistory(values map[string]any) StatsHistoryWindow {
	rawDays := anySlice(values["days"])
	days := make([]DayStats, 0, len(rawDays))
	for _, raw := range rawDays {
		day := buildDayStats(anyMap(raw))
		if strings.TrimSpace(day.DateKey) != "" {
			days = append(days, day)
		}
	}
	return StatsHistoryWindow{
		TodayKey: stringFromMap(values, "todayKey"),
		Days:     days,
	}
}

func buildDayStats(values map[string]any) DayStats {
	return DayStats{
		DateKey:       stringFromMap(values, "dateKey"),
		UpdatedAt:     stringFromMap(values, "updatedAt"),
		Runs:          intFromMap(values, "runs"),
		Collect:       intFromMap(values, "collect"),
		Water:         intFromMap(values, "water"),
		Steal:         intFromMap(values, "steal"),
		Help:          intFromMap(values, "help"),
		MischiefGrass: intFromMap(values, "mischiefGrass"),
		MischiefBug:   intFromMap(values, "mischiefBug"),
		Sell:          intFromMap(values, "sell"),
		SaleEstimate:  int64FromMap(values, "saleEstimate"),
		EstimateReady: boolFromMap(values, "estimateReady"),
	}
}

func buildFertilizerSlot(values map[string]any) FertilizerSlotStatus {
	remainingSec := int64FromMap(values, "remainingSec")
	remainingHours := floatFromMap(values, "remainingHours")
	if remainingHours == 0 && remainingSec > 0 {
		remainingHours = math.Round((float64(remainingSec)/3600)*10) / 10
	}
	if remainingSec == 0 && remainingHours > 0 {
		remainingSec = int64(math.Round(remainingHours * 3600))
	}
	return FertilizerSlotStatus{
		Available:      boolFromMap(values, "available"),
		RemainingSec:   remainingSec,
		RemainingHours: remainingHours,
		RemainingText:  stringFromMap(values, "remainingText"),
	}
}

func int64FromMap(values map[string]any, key string) int64 {
	value, ok := values[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case string:
		var out int64
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &out); err == nil {
			return out
		}
	}
	return 0
}

func floatFromMap(values map[string]any, key string) float64 {
	value, ok := values[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case string:
		var out float64
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%f", &out); err == nil {
			return out
		}
	}
	return 0
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
