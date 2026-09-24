package storage

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

const (
	GlobalSettingsAccountKey = "global"
	DefaultRuntimeAccountKey = "default"
)

type RuntimeSettings struct {
	DefaultTarget                           string   `json:"defaultTarget"`
	CurrentTarget                           string   `json:"currentTarget"`
	AutoStart                               bool     `json:"autoStart"`
	CDPPort                                 int      `json:"cdpPort"`
	WMPFDebugPort                           int      `json:"wmpfDebugPort"`
	ProcessGuardEnabled                     bool     `json:"processGuardEnabled"`
	ProcessGuardFailureRecoveryEnabled      bool     `json:"processGuardFailureRecoveryEnabled"`
	ProcessGuardTimeoutThreshold            int      `json:"processGuardTimeoutThreshold"`
	ProcessGuardMonitorIntervalMS           int      `json:"processGuardMonitorIntervalMs"`
	ProcessGuardRestartReconnectGraceSec    int      `json:"processGuardRestartReconnectGraceSec"`
	ProcessGuardMaxRestartsPer10Min         int      `json:"processGuardMaxRestartsPer10Min"`
	ProcessGuardScheduledRestartEnabled     bool     `json:"processGuardScheduledRestartEnabled"`
	ProcessGuardScheduledRestartIntervalMin int      `json:"processGuardScheduledRestartIntervalMin"`
	ProcessGuardAutoMinimizeAfterRestart    bool     `json:"processGuardAutoMinimizeAfterRestart"`
	NetworkReconnectEnabled                 bool     `json:"networkReconnectEnabled"`
	NetworkReconnectIntervalMS              int      `json:"networkReconnectIntervalMs"`
	NetworkReconnectRecoveryTimeoutMS       int      `json:"networkReconnectRecoveryTimeoutMs"`
	OtherPlaceLoginReconnectEnabled         bool     `json:"otherPlaceLoginReconnectEnabled"`
	OtherPlaceLoginCheckIntervalMS          int      `json:"otherPlaceLoginCheckIntervalMs"`
	OtherPlaceLoginReconnectDelayMin        int      `json:"otherPlaceLoginReconnectDelayMin"`
	AutoWarehouseSellEnabled                bool     `json:"autoWarehouseSellEnabled"`
	AutoWarehouseSellIntervalMinute         int      `json:"autoWarehouseSellIntervalMinute"`
	AutoWarehouseSellCategories             []string `json:"autoWarehouseSellCategories"`
	WarehouseRefreshOnlyOnAutoSell          bool     `json:"warehouseRefreshOnlyOnAutoSell"`
}

func (s *Store) LoadRuntimeSettings(ctx context.Context) (RuntimeSettings, error) {
	values, err := s.loadSettingsForAccount(ctx, GlobalSettingsAccountKey, []string{
		"runtime.defaultTarget",
		"runtime.currentTarget",
		"runtime.autoStart",
		"cdp.port",
		"wmpf.debugPort",
		"processGuard.enabled",
		"processGuard.failureRecoveryEnabled",
		"processGuard.timeoutThreshold",
		"processGuard.monitorIntervalMs",
		"processGuard.restartReconnectGraceSec",
		"processGuard.maxRestartsPer10Min",
		"processGuard.scheduledRestartEnabled",
		"processGuard.scheduledRestartIntervalMin",
		"processGuard.autoMinimizeAfterRestart",
		"networkReconnect.enabled",
		"networkReconnect.intervalMs",
		"networkReconnect.recoveryTimeoutMs",
		"otherPlaceLoginReconnect.enabled",
		"otherPlaceLoginReconnect.intervalMs",
		"otherPlaceLoginReconnect.delayMin",
	})
	if err != nil {
		return RuntimeSettings{}, err
	}

	settings := RuntimeSettings{
		DefaultTarget:                           "qq_ws",
		CurrentTarget:                           "qq_ws",
		AutoStart:                               true,
		CDPPort:                                 62000,
		WMPFDebugPort:                           9420,
		ProcessGuardFailureRecoveryEnabled:      true,
		ProcessGuardTimeoutThreshold:            3,
		ProcessGuardMonitorIntervalMS:           3000,
		ProcessGuardRestartReconnectGraceSec:    45,
		ProcessGuardMaxRestartsPer10Min:         4,
		ProcessGuardScheduledRestartIntervalMin: 60,
		NetworkReconnectEnabled:                 true,
		NetworkReconnectIntervalMS:              1000,
		NetworkReconnectRecoveryTimeoutMS:       20000,
		OtherPlaceLoginCheckIntervalMS:          5000,
		OtherPlaceLoginReconnectDelayMin:        5,
		AutoWarehouseSellIntervalMinute:         60,
		AutoWarehouseSellCategories:             []string{"fruit"},
		WarehouseRefreshOnlyOnAutoSell:          true,
	}
	if value := values["runtime.defaultTarget"]; value != "" {
		settings.DefaultTarget = value
	}
	if value := values["runtime.currentTarget"]; value != "" {
		settings.CurrentTarget = value
	}
	if value := values["runtime.autoStart"]; value != "" {
		settings.AutoStart = value == "true"
	}
	if value := values["cdp.port"]; value != "" {
		port, err := strconv.Atoi(value)
		if err != nil {
			return RuntimeSettings{}, err
		}
		settings.CDPPort = port
	}
	if value := values["wmpf.debugPort"]; value != "" {
		port, err := strconv.Atoi(value)
		if err != nil {
			return RuntimeSettings{}, err
		}
		settings.WMPFDebugPort = port
	}
	if value := values["processGuard.enabled"]; value != "" {
		settings.ProcessGuardEnabled = value == "true"
	}
	if value := values["processGuard.failureRecoveryEnabled"]; value != "" {
		settings.ProcessGuardFailureRecoveryEnabled = value == "true"
	}
	if value := values["processGuard.timeoutThreshold"]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return RuntimeSettings{}, err
		}
		settings.ProcessGuardTimeoutThreshold = parsed
	}
	if value := values["processGuard.monitorIntervalMs"]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return RuntimeSettings{}, err
		}
		settings.ProcessGuardMonitorIntervalMS = parsed
	}
	if value := values["processGuard.restartReconnectGraceSec"]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return RuntimeSettings{}, err
		}
		settings.ProcessGuardRestartReconnectGraceSec = parsed
	}
	if value := values["processGuard.maxRestartsPer10Min"]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return RuntimeSettings{}, err
		}
		settings.ProcessGuardMaxRestartsPer10Min = parsed
	}
	if value := values["processGuard.scheduledRestartEnabled"]; value != "" {
		settings.ProcessGuardScheduledRestartEnabled = value == "true"
	}
	if value := values["processGuard.autoMinimizeAfterRestart"]; value != "" {
		settings.ProcessGuardAutoMinimizeAfterRestart = value == "true"
	}
	if value := values["processGuard.scheduledRestartIntervalMin"]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return RuntimeSettings{}, err
		}
		settings.ProcessGuardScheduledRestartIntervalMin = parsed
	}
	if value := values["networkReconnect.enabled"]; value != "" {
		settings.NetworkReconnectEnabled = value == "true"
	}
	if value := values["networkReconnect.intervalMs"]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return RuntimeSettings{}, err
		}
		settings.NetworkReconnectIntervalMS = parsed
	}
	if value := values["networkReconnect.recoveryTimeoutMs"]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return RuntimeSettings{}, err
		}
		settings.NetworkReconnectRecoveryTimeoutMS = parsed
	}
	if value := values["otherPlaceLoginReconnect.enabled"]; value != "" {
		settings.OtherPlaceLoginReconnectEnabled = value == "true"
	}
	if value := values["otherPlaceLoginReconnect.intervalMs"]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return RuntimeSettings{}, err
		}
		settings.OtherPlaceLoginCheckIntervalMS = parsed
	}
	if value := values["otherPlaceLoginReconnect.delayMin"]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return RuntimeSettings{}, err
		}
		settings.OtherPlaceLoginReconnectDelayMin = parsed
	}
	settings = normalizeGuardianSettings(settings)
	settings = normalizeWarehouseRuntimeSettings(settings)
	return settings, nil
}

func (s *Store) SaveRuntimeSettings(ctx context.Context, settings RuntimeSettings) error {
	settings = normalizeGuardianSettings(settings)
	values := map[string]string{
		"runtime.defaultTarget":                    settings.DefaultTarget,
		"runtime.currentTarget":                    settings.CurrentTarget,
		"runtime.autoStart":                        strconv.FormatBool(settings.AutoStart),
		"cdp.port":                                 strconv.Itoa(settings.CDPPort),
		"wmpf.debugPort":                           strconv.Itoa(settings.WMPFDebugPort),
		"processGuard.enabled":                     strconv.FormatBool(settings.ProcessGuardEnabled),
		"processGuard.failureRecoveryEnabled":      strconv.FormatBool(settings.ProcessGuardFailureRecoveryEnabled),
		"processGuard.timeoutThreshold":            strconv.Itoa(settings.ProcessGuardTimeoutThreshold),
		"processGuard.monitorIntervalMs":           strconv.Itoa(settings.ProcessGuardMonitorIntervalMS),
		"processGuard.restartReconnectGraceSec":    strconv.Itoa(settings.ProcessGuardRestartReconnectGraceSec),
		"processGuard.maxRestartsPer10Min":         strconv.Itoa(settings.ProcessGuardMaxRestartsPer10Min),
		"processGuard.scheduledRestartEnabled":     strconv.FormatBool(settings.ProcessGuardScheduledRestartEnabled),
		"processGuard.scheduledRestartIntervalMin": strconv.Itoa(settings.ProcessGuardScheduledRestartIntervalMin),
		"processGuard.autoMinimizeAfterRestart":    strconv.FormatBool(settings.ProcessGuardAutoMinimizeAfterRestart),
		"networkReconnect.enabled":                 strconv.FormatBool(settings.NetworkReconnectEnabled),
		"networkReconnect.intervalMs":              strconv.Itoa(settings.NetworkReconnectIntervalMS),
		"networkReconnect.recoveryTimeoutMs":       strconv.Itoa(settings.NetworkReconnectRecoveryTimeoutMS),
		"otherPlaceLoginReconnect.enabled":         strconv.FormatBool(settings.OtherPlaceLoginReconnectEnabled),
		"otherPlaceLoginReconnect.intervalMs":      strconv.Itoa(settings.OtherPlaceLoginCheckIntervalMS),
		"otherPlaceLoginReconnect.delayMin":        strconv.Itoa(settings.OtherPlaceLoginReconnectDelayMin),
	}
	return s.saveSettingsForAccount(ctx, GlobalSettingsAccountKey, values)
}

func (s *Store) LoadMessagePushConfigJSON(ctx context.Context) (string, error) {
	return s.LoadMessagePushConfigJSONForAccount(ctx, DefaultRuntimeAccountKey)
}

func (s *Store) LoadMessagePushConfigJSONForAccount(ctx context.Context, accountKey string) (string, error) {
	values, err := s.loadSettingsForAccount(ctx, accountKey, []string{"messagePush.config"})
	if err != nil {
		return "", err
	}
	return values["messagePush.config"], nil
}

func (s *Store) SaveMessagePushConfigJSON(ctx context.Context, raw string) error {
	return s.SaveMessagePushConfigJSONForAccount(ctx, DefaultRuntimeAccountKey, raw)
}

func (s *Store) SaveMessagePushConfigJSONForAccount(ctx context.Context, accountKey string, raw string) error {
	return s.saveSettingForAccount(ctx, accountKey, "messagePush.config", raw)
}

func (s *Store) LoadMessagePushStateJSON(ctx context.Context) (string, error) {
	return s.LoadMessagePushStateJSONForAccount(ctx, DefaultRuntimeAccountKey)
}

func (s *Store) LoadMessagePushStateJSONForAccount(ctx context.Context, accountKey string) (string, error) {
	values, err := s.loadSettingsForAccount(ctx, accountKey, []string{"messagePush.state"})
	if err != nil {
		return "", err
	}
	return values["messagePush.state"], nil
}

func (s *Store) SaveMessagePushStateJSON(ctx context.Context, raw string) error {
	return s.SaveMessagePushStateJSONForAccount(ctx, DefaultRuntimeAccountKey, raw)
}

func (s *Store) SaveMessagePushStateJSONForAccount(ctx context.Context, accountKey string, raw string) error {
	return s.saveSettingForAccount(ctx, accountKey, "messagePush.state", raw)
}

type RuntimeAccount struct {
	AccountKey   string `json:"accountKey"`
	GID          int    `json:"gid"`
	Nickname     string `json:"nickname,omitempty"`
	AvatarURL    string `json:"avatarUrl,omitempty"`
	IdentifiedAt string `json:"identifiedAt"`
	ConfirmedAt  string `json:"confirmedAt,omitempty"`
}

func NormalizeAccountKey(accountKey string) string {
	key := strings.TrimSpace(accountKey)
	if key == "" {
		return DefaultRuntimeAccountKey
	}
	return key
}

func AccountKeyForGID(gid int) string {
	if gid <= 0 {
		return DefaultRuntimeAccountKey
	}
	return "gid:" + strconv.Itoa(gid)
}

func (s *Store) SaveRuntimeAccount(ctx context.Context, account RuntimeAccount) error {
	if account.AccountKey == "" {
		account.AccountKey = AccountKeyForGID(account.GID)
	}
	account.AccountKey = NormalizeAccountKey(account.AccountKey)
	now := time.Now().Format(time.RFC3339Nano)
	if account.IdentifiedAt == "" {
		account.IdentifiedAt = now
	}
	if account.ConfirmedAt == "" {
		account.ConfirmedAt = now
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO runtime_accounts (account_key, gid, nickname, avatar_url, identified_at, confirmed_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_key) DO UPDATE SET
			gid = excluded.gid,
			nickname = excluded.nickname,
			avatar_url = excluded.avatar_url,
			identified_at = excluded.identified_at,
			confirmed_at = excluded.confirmed_at
	`, account.AccountKey, account.GID, account.Nickname, account.AvatarURL, account.IdentifiedAt, account.ConfirmedAt)
	return err
}

func (s *Store) LoadRuntimeAccount(ctx context.Context, accountKey string) (RuntimeAccount, error) {
	accountKey = NormalizeAccountKey(accountKey)
	var account RuntimeAccount
	err := s.db.QueryRowContext(ctx, `
		SELECT account_key, gid, nickname, avatar_url, identified_at, confirmed_at
		FROM runtime_accounts
		WHERE account_key = ?
	`, accountKey).Scan(&account.AccountKey, &account.GID, &account.Nickname, &account.AvatarURL, &account.IdentifiedAt, &account.ConfirmedAt)
	if err == sql.ErrNoRows {
		return RuntimeAccount{}, nil
	}
	if err != nil {
		return RuntimeAccount{}, err
	}
	return account, nil
}

func (s *Store) saveSetting(ctx context.Context, key string, value string) error {
	return s.saveSettingForAccount(ctx, GlobalSettingsAccountKey, key, value)
}

func (s *Store) saveSettingForAccount(ctx context.Context, accountKey string, key string, value string) error {
	return s.saveSettingsForAccount(ctx, accountKey, map[string]string{key: value})
}

const settingUpsertSQL = `
	INSERT INTO settings (account_key, key, value, updated_at)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(account_key, key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
`

func (s *Store) saveSettingsForAccount(ctx context.Context, accountKey string, values map[string]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	accountKey = NormalizeAccountKey(accountKey)
	now := time.Now().Format(time.RFC3339Nano)
	for key, value := range values {
		if _, err := tx.ExecContext(ctx, settingUpsertSQL, accountKey, key, value, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func normalizeWarehouseRuntimeSettings(settings RuntimeSettings) RuntimeSettings {
	if settings.AutoWarehouseSellIntervalMinute <= 0 {
		settings.AutoWarehouseSellIntervalMinute = 60
	}
	settings.AutoWarehouseSellCategories = normalizeWarehouseSellCategories(settings.AutoWarehouseSellCategories)
	if len(settings.AutoWarehouseSellCategories) == 0 {
		settings.AutoWarehouseSellCategories = []string{"fruit"}
	}
	settings.WarehouseRefreshOnlyOnAutoSell = true
	return settings
}

func normalizeWarehouseSellCategories(categories []string) []string {
	allowed := map[string]bool{"fruit": true, "mutation": true, "seed": true, "tool": true}
	seen := map[string]bool{}
	normalized := make([]string, 0, len(categories))
	for _, category := range categories {
		key := strings.TrimSpace(strings.ToLower(category))
		if !allowed[key] || seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, key)
	}
	return normalized
}

func NormalizeWarehouseRuntimeSettings(settings RuntimeSettings) RuntimeSettings {
	return normalizeWarehouseRuntimeSettings(settings)
}

func normalizeGuardianSettings(settings RuntimeSettings) RuntimeSettings {
	if settings.ProcessGuardTimeoutThreshold <= 0 {
		settings.ProcessGuardTimeoutThreshold = 3
	}
	if settings.ProcessGuardMonitorIntervalMS <= 0 {
		settings.ProcessGuardMonitorIntervalMS = 3000
	}
	settings.ProcessGuardMonitorIntervalMS = clamp(settings.ProcessGuardMonitorIntervalMS, 500, 60000)
	if settings.ProcessGuardRestartReconnectGraceSec <= 0 {
		settings.ProcessGuardRestartReconnectGraceSec = 45
	}
	if settings.ProcessGuardMaxRestartsPer10Min <= 0 {
		settings.ProcessGuardMaxRestartsPer10Min = 4
	}
	if settings.ProcessGuardScheduledRestartIntervalMin <= 0 {
		settings.ProcessGuardScheduledRestartIntervalMin = 60
	}
	if settings.NetworkReconnectIntervalMS <= 0 {
		settings.NetworkReconnectIntervalMS = 1000
	}
	settings.NetworkReconnectIntervalMS = clamp(settings.NetworkReconnectIntervalMS, 300, 60000)
	if settings.NetworkReconnectRecoveryTimeoutMS <= 0 {
		settings.NetworkReconnectRecoveryTimeoutMS = 20000
	}
	settings.NetworkReconnectRecoveryTimeoutMS = clamp(settings.NetworkReconnectRecoveryTimeoutMS, 1000, 120000)
	if settings.OtherPlaceLoginCheckIntervalMS <= 0 {
		settings.OtherPlaceLoginCheckIntervalMS = 5000
	}
	settings.OtherPlaceLoginCheckIntervalMS = clamp(settings.OtherPlaceLoginCheckIntervalMS, 1000, 60000)
	settings.OtherPlaceLoginReconnectDelayMin = clamp(settings.OtherPlaceLoginReconnectDelayMin, 0, 1440)
	return settings
}

func clamp(value int, minimum int, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func (s *Store) loadSettings(ctx context.Context, keys []string) (map[string]string, error) {
	return s.loadSettingsForAccount(ctx, GlobalSettingsAccountKey, keys)
}

func (s *Store) loadSettingsForAccount(ctx context.Context, accountKey string, keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	accountKey = NormalizeAccountKey(accountKey)
	for _, key := range keys {
		var value string
		err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE account_key = ? AND key = ?`, accountKey, key).Scan(&value)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		values[key] = value
	}
	return values, nil
}
