package desktop

import (
	"errors"

	"Farm_Go/internal/storage"
)

func (a *App) UpdateCheckPreferences() storage.UpdateCheckPreferences {
	if a.store == nil {
		return storage.DefaultUpdateCheckPreferences()
	}
	preferences, err := a.store.LoadUpdateCheckPreferences(a.contextOrBackground())
	if err != nil {
		a.lastErr = err
		return storage.DefaultUpdateCheckPreferences()
	}
	return preferences
}

func (a *App) SaveUpdateCheckPreferences(input storage.UpdateCheckPreferences) (storage.UpdateCheckPreferences, error) {
	if a.store == nil {
		err := errors.New("update check preference storage is unavailable")
		a.lastErr = err
		return input, err
	}
	if err := a.store.SaveUpdateCheckPreferences(a.contextOrBackground(), input); err != nil {
		a.lastErr = err
		return input, err
	}
	return input, nil
}
