package storage

import (
	"context"
	"errors"
	"strconv"
)

const (
	DefaultUpdateCheckIntervalMinutes = 120
	MinUpdateCheckIntervalMinutes     = 1
	MaxUpdateCheckIntervalMinutes     = 10080

	updateCheckEnabledKey         = "update.checkEnabled"
	updateCheckIntervalMinutesKey = "update.checkIntervalMinutes"
)

var ErrInvalidUpdateCheckInterval = errors.New("update check interval must be between 1 and 10080 minutes")

type UpdateCheckPreferences struct {
	Enabled         bool `json:"enabled"`
	IntervalMinutes int  `json:"intervalMinutes"`
}

func DefaultUpdateCheckPreferences() UpdateCheckPreferences {
	return UpdateCheckPreferences{
		Enabled:         true,
		IntervalMinutes: DefaultUpdateCheckIntervalMinutes,
	}
}

func (s *Store) LoadUpdateCheckPreferences(ctx context.Context) (UpdateCheckPreferences, error) {
	values, err := s.loadSettingsForAccount(ctx, GlobalSettingsAccountKey, []string{
		updateCheckEnabledKey,
		updateCheckIntervalMinutesKey,
	})
	if err != nil {
		return UpdateCheckPreferences{}, err
	}

	preferences := DefaultUpdateCheckPreferences()
	if enabled, parseErr := strconv.ParseBool(values[updateCheckEnabledKey]); parseErr == nil {
		preferences.Enabled = enabled
	}
	if interval, parseErr := strconv.Atoi(values[updateCheckIntervalMinutesKey]); parseErr == nil && validUpdateCheckInterval(interval) {
		preferences.IntervalMinutes = interval
	}
	return preferences, nil
}

func (s *Store) SaveUpdateCheckPreferences(ctx context.Context, preferences UpdateCheckPreferences) error {
	if !validUpdateCheckInterval(preferences.IntervalMinutes) {
		return ErrInvalidUpdateCheckInterval
	}
	return s.saveSettingsForAccount(ctx, GlobalSettingsAccountKey, map[string]string{
		updateCheckEnabledKey:         strconv.FormatBool(preferences.Enabled),
		updateCheckIntervalMinutesKey: strconv.Itoa(preferences.IntervalMinutes),
	})
}

func validUpdateCheckInterval(interval int) bool {
	return interval >= MinUpdateCheckIntervalMinutes && interval <= MaxUpdateCheckIntervalMinutes
}
