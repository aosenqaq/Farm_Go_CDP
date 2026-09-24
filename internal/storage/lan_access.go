package storage

import (
	"context"
	"strconv"
)

const (
	lanAccessEnabledKey      = "lanAccess.enabled"
	lanAccessModeKey         = "lanAccess.mode"
	lanAccessPortKey         = "lanAccess.port"
	lanAccessPasswordHashKey = "lanAccess.passwordHash"
)

type LANAccessMode string

const (
	LANAccessModeLAN    LANAccessMode = "lan"
	LANAccessModeTunnel LANAccessMode = "tunnel"
)

type LANAccessSettings struct {
	Enabled      bool
	Mode         LANAccessMode
	Port         int
	PasswordHash string `json:"-"`
}

type LANAccessPublicSettings struct {
	Enabled            bool          `json:"enabled"`
	Mode               LANAccessMode `json:"mode"`
	Port               int           `json:"port"`
	PasswordConfigured bool          `json:"passwordConfigured"`
}

func DefaultLANAccessSettings() LANAccessSettings {
	return LANAccessSettings{
		Mode: LANAccessModeLAN,
		Port: 8788,
	}
}

func (s LANAccessSettings) Public() LANAccessPublicSettings {
	return LANAccessPublicSettings{
		Enabled:            s.Enabled,
		Mode:               s.Mode,
		Port:               s.Port,
		PasswordConfigured: s.PasswordHash != "",
	}
}

func (s *Store) LoadLANAccessSettings(ctx context.Context) (LANAccessSettings, error) {
	settings := DefaultLANAccessSettings()
	values, err := s.loadSettingsForAccount(ctx, GlobalSettingsAccountKey, []string{
		lanAccessEnabledKey,
		lanAccessModeKey,
		lanAccessPortKey,
		lanAccessPasswordHashKey,
	})
	if err != nil {
		return LANAccessSettings{}, err
	}
	if value := values[lanAccessEnabledKey]; value != "" {
		settings.Enabled = value == "true"
	}
	if value := values[lanAccessModeKey]; value != "" {
		settings.Mode = LANAccessMode(value)
	}
	if value := values[lanAccessPortKey]; value != "" {
		settings.Port, err = strconv.Atoi(value)
		if err != nil {
			return LANAccessSettings{}, err
		}
	}
	settings.PasswordHash = values[lanAccessPasswordHashKey]
	return settings, nil
}

func (s *Store) SaveLANAccessSettings(ctx context.Context, settings LANAccessSettings) error {
	return s.saveSettingsForAccount(ctx, GlobalSettingsAccountKey, map[string]string{
		lanAccessEnabledKey:      strconv.FormatBool(settings.Enabled),
		lanAccessModeKey:         string(settings.Mode),
		lanAccessPortKey:         strconv.Itoa(settings.Port),
		lanAccessPasswordHashKey: settings.PasswordHash,
	})
}
