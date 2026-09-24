package storage

import (
	"context"
	"strconv"
	"strings"
)

type WarehouseAutoSellSettings struct {
	Enabled               bool     `json:"enabled"`
	IntervalMinute        int      `json:"intervalMinute"`
	Categories            []string `json:"categories"`
	RefreshOnlyOnAutoSell bool     `json:"refreshOnlyOnAutoSell"`
}

func DefaultWarehouseAutoSellSettings() WarehouseAutoSellSettings {
	return WarehouseAutoSellSettings{
		IntervalMinute:        60,
		Categories:            []string{"fruit"},
		RefreshOnlyOnAutoSell: true,
	}
}

func (s *Store) LoadWarehouseAutoSellSettingsForAccount(ctx context.Context, accountKey string) (WarehouseAutoSellSettings, error) {
	settings := DefaultWarehouseAutoSellSettings()
	values, err := s.loadSettingsForAccount(ctx, NormalizeAccountKey(accountKey), []string{
		"warehouse.autoSellEnabled",
		"warehouse.autoSellIntervalMinute",
		"warehouse.autoSellIntervalHour",
		"warehouse.autoSellCategories",
		"warehouse.refreshOnlyOnAutoSell",
	})
	if err != nil {
		return WarehouseAutoSellSettings{}, err
	}
	if value := values["warehouse.autoSellEnabled"]; value != "" {
		settings.Enabled = value == "true"
	}
	if value := values["warehouse.autoSellIntervalMinute"]; value != "" {
		settings.IntervalMinute, err = strconv.Atoi(value)
		if err != nil {
			return WarehouseAutoSellSettings{}, err
		}
	} else if value := values["warehouse.autoSellIntervalHour"]; value != "" {
		legacyHours, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			return WarehouseAutoSellSettings{}, parseErr
		}
		settings.IntervalMinute = legacyHours * 60
		settings = normalizeWarehouseAutoSellSettings(settings)
		if err := s.saveSettingsForAccount(ctx, NormalizeAccountKey(accountKey), map[string]string{
			"warehouse.autoSellIntervalMinute": strconv.Itoa(settings.IntervalMinute),
		}); err != nil {
			return WarehouseAutoSellSettings{}, err
		}
	}
	if value := values["warehouse.autoSellCategories"]; value != "" {
		settings.Categories = strings.Split(value, ",")
	}
	return normalizeWarehouseAutoSellSettings(settings), nil
}

func (s *Store) SaveWarehouseAutoSellSettingsForAccount(ctx context.Context, accountKey string, settings WarehouseAutoSellSettings) error {
	settings = normalizeWarehouseAutoSellSettings(settings)
	return s.saveSettingsForAccount(ctx, NormalizeAccountKey(accountKey), warehouseAutoSellSettingsValues(settings))
}

func warehouseAutoSellSettingsValues(settings WarehouseAutoSellSettings) map[string]string {
	return map[string]string{
		"warehouse.autoSellEnabled":        strconv.FormatBool(settings.Enabled),
		"warehouse.autoSellIntervalMinute": strconv.Itoa(settings.IntervalMinute),
		"warehouse.autoSellCategories":     strings.Join(settings.Categories, ","),
		"warehouse.refreshOnlyOnAutoSell":  strconv.FormatBool(settings.RefreshOnlyOnAutoSell),
	}
}

func normalizeWarehouseAutoSellSettings(settings WarehouseAutoSellSettings) WarehouseAutoSellSettings {
	if settings.IntervalMinute <= 0 {
		settings.IntervalMinute = 60
	}
	settings.Categories = normalizeWarehouseSellCategories(settings.Categories)
	if len(settings.Categories) == 0 {
		settings.Categories = []string{"fruit"}
	}
	settings.RefreshOnlyOnAutoSell = true
	return settings
}
