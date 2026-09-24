package storage

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

type AutoFarmSettings struct {
	SchedulerEnabled  bool                   `json:"schedulerEnabled"`
	SchedulerMinGapMS int                    `json:"schedulerMinGapMs"`
	RunMode           string                 `json:"runMode"`
	Tasks             []AutoFarmTaskSettings `json:"tasks"`
	Config            map[string]any         `json:"config"`
}

type AutoFarmTaskSettings struct {
	ID          string `json:"id"`
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"`
	IntervalSec int    `json:"intervalSec"`
}

func (s *Store) LoadAutoFarmSettings(ctx context.Context) (AutoFarmSettings, error) {
	return s.LoadAutoFarmSettingsForAccount(ctx, DefaultRuntimeAccountKey)
}

func (s *Store) LoadAutoFarmSettingsForAccount(ctx context.Context, accountKey string) (AutoFarmSettings, error) {
	settings := AutoFarmSettings{
		SchedulerEnabled:  true,
		SchedulerMinGapMS: 350,
	}
	values, err := s.loadSettingsForAccount(ctx, accountKey, []string{
		"autoFarm.scheduler.enabled",
		"autoFarm.scheduler.minGapMs",
		"autoFarm.config",
		"autoFarm.runMode",
	})
	if err != nil {
		return AutoFarmSettings{}, err
	}
	if value := values["autoFarm.scheduler.enabled"]; value != "" {
		settings.SchedulerEnabled = value == "true"
	}
	if value := values["autoFarm.scheduler.minGapMs"]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return AutoFarmSettings{}, err
		}
		settings.SchedulerMinGapMS = parsed
	}
	if value := values["autoFarm.config"]; value != "" {
		if err := json.Unmarshal([]byte(value), &settings.Config); err != nil {
			return AutoFarmSettings{}, err
		}
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT key, value
		FROM settings
		WHERE account_key = ? AND key LIKE 'autoFarm.task.%'
	`, NormalizeAccountKey(accountKey))
	if err != nil {
		return AutoFarmSettings{}, err
	}
	defer rows.Close()

	byID := map[string]*AutoFarmTaskSettings{}
	hasTaskRecord := false
	for rows.Next() {
		var key string
		var value string
		if err := rows.Scan(&key, &value); err != nil {
			return AutoFarmSettings{}, err
		}
		hasTaskRecord = true
		id, field, ok := parseAutoFarmTaskSettingKey(key)
		if !ok {
			continue
		}
		task := byID[id]
		if task == nil {
			task = &AutoFarmTaskSettings{ID: id}
			byID[id] = task
		}
		switch field {
		case "enabled":
			task.Enabled = value == "true"
		case "priority":
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return AutoFarmSettings{}, err
			}
			task.Priority = parsed
		case "intervalSec":
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return AutoFarmSettings{}, err
			}
			task.IntervalSec = parsed
		}
	}
	if err := rows.Err(); err != nil {
		return AutoFarmSettings{}, err
	}

	runMode, err := s.loadOrInitializeAutoFarmRunMode(ctx, accountKey, values, hasTaskRecord)
	if err != nil {
		return AutoFarmSettings{}, err
	}
	settings.RunMode = runMode

	settings.Tasks = make([]AutoFarmTaskSettings, 0, len(byID))
	for _, task := range byID {
		settings.Tasks = append(settings.Tasks, *task)
	}
	sortAutoFarmTaskSettings(settings.Tasks)
	return settings, nil
}

func (s *Store) SaveAutoFarmSettings(ctx context.Context, settings AutoFarmSettings) error {
	return s.SaveAutoFarmSettingsForAccount(ctx, DefaultRuntimeAccountKey, settings)
}

func (s *Store) SaveAutoFarmSettingsForAccount(ctx context.Context, accountKey string, settings AutoFarmSettings) error {
	values, err := autoFarmSettingsValues(settings)
	if err != nil {
		return err
	}
	return s.saveSettingsForAccount(ctx, accountKey, values)
}

func autoFarmSettingsValues(settings AutoFarmSettings) (map[string]string, error) {
	configJSON, err := json.Marshal(settings.Config)
	if err != nil {
		return nil, err
	}
	values := map[string]string{
		"autoFarm.scheduler.enabled":  strconv.FormatBool(settings.SchedulerEnabled),
		"autoFarm.scheduler.minGapMs": strconv.Itoa(settings.SchedulerMinGapMS),
		"autoFarm.runMode":            normalizeAutoFarmRunMode(settings.RunMode),
		"autoFarm.config":             string(configJSON),
	}
	for _, task := range settings.Tasks {
		id := strings.TrimSpace(task.ID)
		if id == "" {
			continue
		}
		prefix := "autoFarm.task." + id + "."
		values[prefix+"enabled"] = strconv.FormatBool(task.Enabled)
		values[prefix+"priority"] = strconv.Itoa(task.Priority)
		values[prefix+"intervalSec"] = strconv.Itoa(task.IntervalSec)
	}
	return values, nil
}

func (s *Store) loadOrInitializeAutoFarmRunMode(ctx context.Context, accountKey string, values map[string]string, hasTaskRecord bool) (string, error) {
	const runModeKey = "autoFarm.runMode"
	if value, ok := values[runModeKey]; ok {
		normalized := normalizeAutoFarmRunMode(value)
		if value != normalized {
			if err := s.saveSettingForAccount(ctx, accountKey, runModeKey, normalized); err != nil {
				return "", err
			}
		}
		return normalized, nil
	}

	runMode := "safe"
	for _, key := range []string{"autoFarm.scheduler.enabled", "autoFarm.scheduler.minGapMs", "autoFarm.config"} {
		if _, ok := values[key]; ok {
			runMode = "god"
			break
		}
	}
	if hasTaskRecord {
		runMode = "god"
	}
	if err := s.saveSettingForAccount(ctx, accountKey, runModeKey, runMode); err != nil {
		return "", err
	}
	return runMode, nil
}

func normalizeAutoFarmRunMode(value string) string {
	switch value {
	case "safe", "god":
		return value
	default:
		return "god"
	}
}

func parseAutoFarmTaskSettingKey(key string) (string, string, bool) {
	const prefix = "autoFarm.task."
	rest := strings.TrimPrefix(key, prefix)
	if rest == key {
		return "", "", false
	}
	index := strings.LastIndex(rest, ".")
	if index <= 0 || index == len(rest)-1 {
		return "", "", false
	}
	field := rest[index+1:]
	switch field {
	case "enabled", "priority", "intervalSec":
		return rest[:index], field, true
	default:
		return "", "", false
	}
}

func sortAutoFarmTaskSettings(tasks []AutoFarmTaskSettings) {
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})
}
