package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	farmapp "Farm_Go/internal/app"
	"Farm_Go/internal/config"
	"Farm_Go/internal/diagnostics"
	"Farm_Go/internal/eventbus"
	"Farm_Go/internal/farm"
	"Farm_Go/internal/farm/automation"
	"Farm_Go/internal/farm/social"
	"Farm_Go/internal/maintenance"
	"Farm_Go/internal/messagepush"
	farmruntime "Farm_Go/internal/runtime"
	"Farm_Go/internal/runtime/guard"
	"Farm_Go/internal/runtime/qqlink"
	"Farm_Go/internal/runtime/qqpatch"
	"Farm_Go/internal/runtime/wmpf"
	"Farm_Go/internal/storage"
)

var qqHostScript string

var runtimeButtonScript string

const embeddedFridaRoot = "resources/wmpf/frida"

const tsdkRuntimeAccountKey = "global:tsdk"

var embeddedFridaResources fs.FS

type pendingHostMinimize struct {
	runtimeTarget string
}

type App struct {
	ctx                 context.Context
	desktopRuntime      desktopRuntime
	closeMu             sync.Mutex
	exitApproved        bool
	cfg                 config.Config
	manager             *farmruntime.Manager
	supervisor          *farmruntime.Supervisor
	guard               *guard.Manager
	guardian            *guard.Supervisor
	guardianCoordinator *guard.RecoveryCoordinator
	hosts               *guard.HostBindingRegistry
	service             *farmapp.Service
	maintenance         *maintenance.Service
	store               *storage.Store
	lanAccess           *lanAccessManager
	cancel              context.CancelFunc
	processCtx          context.Context
	dataDir             string
	fridaPython         string
	lastErr             error

	lifecycleMu             sync.Mutex
	authorizedSessionCancel context.CancelFunc
	authorizedGeneration    uint64
	authorized              bool
	authorizationForTests   bool

	listHostSnapshots                   func() ([]guard.HostProcessSnapshot, error)
	stopHostPID                         func(pid int) error
	closeHostWindows                    func([]guard.HostWindowSnapshot) error
	launchHost                          func(guard.LaunchRequest) error
	minimizeHostWindows                 func([]guard.HostWindowSnapshot) error
	waitForRuntimeStatus                func(time.Duration, func(farmruntime.Status) bool) bool
	waitForQQMiniappClosed              func(int, time.Duration) (guard.QQCloseObservation, error)
	pendingHostMinimizeMu               sync.Mutex
	pendingHostMinimize                 *pendingHostMinimize
	restartLaunchDelay                  time.Duration
	sleep                               func(time.Duration)
	beforeWarehouseAutoSellSettingsSave func()

	patchMu     sync.RWMutex
	patchStatus QQDebugPatchStatus

	socialMu           sync.Mutex
	dogGuardScanner    *social.DogGuardScanner
	dogGuardStore      *storage.Store
	dogGuardAccountKey string

	automationSchedulerActionMu     sync.Mutex
	automationSettingsWriteMu       sync.Mutex
	automationSchedulerMu           sync.Mutex
	automationScheduler             *automation.Scheduler
	automationSchedulerAccountKey   string
	automationExecutionGate         *automationExecutionGate
	beforeAutomationSchedulerAction func()
	beforeAutomationSettingsWrite   func()
	runtimeCache                    *automation.RuntimeCallCache
	landDetailsOperationMu          sync.Mutex
	landDetailsMu                   sync.Mutex
	landDetailsRevision             uint64
	landDetailsSnapshot             farm.LandDetailsPayload
	landDetailsSigns                map[int]string
	landDetailsTopState             string
	guardianPauseMu                 sync.RWMutex
	guardianDispatchPaused          bool

	messagePushMu            sync.Mutex
	messagePush              *messagepush.Service
	messagePushAccountKey    string
	messagePushCancel        context.CancelFunc
	messagePushDailyMu       sync.Mutex
	messagePushDailyWG       sync.WaitGroup
	messagePushDailyInterval time.Duration
	messagePushHTTPClient    *http.Client
	messagePushQueue         chan messagePushWork
	messagePushAuditQueue    chan messagePushAudit
	messagePushWorkerMu      sync.Mutex
	messagePushWorkerCancel  context.CancelFunc
	messagePushWorkerWG      sync.WaitGroup

	eventsMu     sync.Mutex
	memoryEvents []accountRuntimeEvent
	nextEventID  int64

	runStatisticsStartedAt time.Time

	accountMu         sync.RWMutex
	currentAccountKey string
	currentAccount    RuntimeAccount
}

type RuntimeAccount struct {
	AccountKey   string `json:"accountKey"`
	GID          int    `json:"gid"`
	Nickname     string `json:"nickname,omitempty"`
	AvatarURL    string `json:"avatarUrl,omitempty"`
	Confirmed    bool   `json:"confirmed"`
	IdentifiedAt string `json:"identifiedAt,omitempty"`
	Error        string `json:"error,omitempty"`
}

type GuardianStatusDTO struct {
	Enabled         bool                 `json:"enabled"`
	Running         bool                 `json:"running"`
	Phase           guard.Phase          `json:"phase"`
	RuntimeTarget   string               `json:"runtimeTarget"`
	Settings        guard.Settings       `json:"settings"`
	Process         guard.Status         `json:"process"`
	Network         guard.WorkerStatus   `json:"network"`
	OtherPlaceLogin guard.WorkerStatus   `json:"otherPlaceLogin"`
	RecentEvents    []guard.RuntimeEvent `json:"recentEvents"`
}

type accountRuntimeEvent struct {
	accountKey string
	event      eventbus.Event
}

type messagePushWork struct {
	accountKey string
	event      messagepush.MessageEvent
}

type messagePushAudit struct {
	accountKey string
	event      eventbus.Event
}

const messagePushQueueCapacity = 64

type QQDebugPatchStatus struct {
	Phase           string   `json:"phase"`
	Injected        bool     `json:"injected"`
	InFlight        bool     `json:"inFlight"`
	LastTrigger     string   `json:"lastTrigger,omitempty"`
	Action          string   `json:"action,omitempty"`
	TargetPath      string   `json:"targetPath,omitempty"`
	BackupPath      string   `json:"backupPath,omitempty"`
	ScriptHash      string   `json:"scriptHash,omitempty"`
	HostVersion     string   `json:"hostVersion,omitempty"`
	CandidatePaths  []string `json:"candidatePaths,omitempty"`
	RestartRequired bool     `json:"restartRequired"`
	Error           string   `json:"error,omitempty"`
	StartedAt       string   `json:"startedAt,omitempty"`
	FinishedAt      string   `json:"finishedAt,omitempty"`
}

func newCDPLinkConfig(cfg config.Config, fridaPython string) wmpf.CDPLinkConfig {
	return wmpf.CDPLinkConfig{
		DebugPort:       cfg.WMPF.DebugPort,
		LegacyDebugPort: cfg.WMPF.LegacyDebugPort,
		CDPPort:         cfg.CDP.Port,
		FridaRoot:       embeddedFridaRoot,
		FridaResources:  embeddedFridaResources,
		FridaPython:     fridaPython,
		FridaEnabled:    cfg.WMPF.FridaEnabled,
		ButtonScript:    runtimeButtonScript,
	}
}

func NewApp(lanAssets ...fs.FS) *App {
	cfg := config.Default()
	manager := farmruntime.NewManager()
	hosts := guard.NewHostBindingRegistry()
	var app *App
	var guardianCoordinator *guard.RecoveryCoordinator
	guardManager := guard.NewManager(guard.ManagerOptions{
		Settings: guardSettingsFromRuntimeSettings(storage.RuntimeSettings{}),
		OnLifecycleEvent: func(event guard.LifecycleEvent) {
			if app != nil {
				app.enqueueMessagePushLifecycleEventForAccount(app.accountKey(), event)
			}
		},
		Restart: func(reason guard.RestartReason) (guard.RestartResult, error) {
			if app != nil && guardianCoordinator != nil {
				guardianCoordinator.AdvanceGeneration("process_restart")
				app.setGuardianDispatchPaused(true)
				defer app.setGuardianDispatchPaused(false)
			}
			return app.restartHostForRuntimeTarget(reason.RuntimeTarget)
		},
	})
	guardianCaller := &guardianRuntimeCaller{}
	guardianCoordinator = guard.NewRecoveryCoordinator(guard.CoordinatorOptions{
		Run: func(context.Context, guard.RecoveryRequest) guard.RecoveryResult {
			// Runtime-local watchers perform the concrete reconnect action. The
			// coordinator serializes the corresponding lifecycle work and pause state.
			return guard.RecoveryResult{OK: true}
		},
		OnPause: func(paused bool) {
			if app != nil {
				app.setGuardianDispatchPaused(paused)
			}
		},
	})
	guardian := guard.NewSupervisor(guard.SupervisorOptions{
		Settings:    guardSettingsFromRuntimeSettings(storage.RuntimeSettings{}),
		Process:     guardManager,
		Coordinator: guardianCoordinator,
		Caller:      guardianCaller,
		Snapshot: func() guard.RuntimeSnapshot {
			if app == nil || app.manager == nil {
				return guard.RuntimeSnapshot{}
			}
			return guardianSnapshotFromRuntimeStatus(app.manager.Status())
		},
		OnEvent: func(event guard.RuntimeEvent) {
			if app != nil {
				app.handleGuardianRuntimeEvent(event)
			}
		},
	})
	qqLink := qqlink.New(cfg.QQWS, manager)
	wechatLink := wmpf.NewCDPLink(
		wmpf.ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP),
		newCDPLinkConfig(cfg, ""),
		manager,
	)
	yybLink := wmpf.NewCDPLink(
		wmpf.ProfileForTarget(farmruntime.RuntimeTargetYYBCDP),
		newCDPLinkConfig(cfg, ""),
		manager,
	)
	supervisor := farmruntime.NewSupervisor(manager, map[farmruntime.RuntimeTarget]farmruntime.RuntimeLink{
		farmruntime.RuntimeTargetQQWS:      qqLink,
		farmruntime.RuntimeTargetWeChatCDP: wechatLink,
		farmruntime.RuntimeTargetYYBCDP:    yybLink,
	})
	app = &App{
		desktopRuntime:           wailsDesktopRuntime{},
		cfg:                      cfg,
		manager:                  manager,
		supervisor:               supervisor,
		guard:                    guardManager,
		guardian:                 guardian,
		guardianCoordinator:      guardianCoordinator,
		hosts:                    hosts,
		service:                  farmapp.NewService(manager, diagnostics.NewService(supervisor), cfg),
		maintenance:              maintenance.NewService(),
		dataDir:                  defaultDataDir(),
		patchStatus:              QQDebugPatchStatus{Phase: "idle"},
		currentAccountKey:        storage.DefaultRuntimeAccountKey,
		runStatisticsStartedAt:   time.Now(),
		messagePushQueue:         make(chan messagePushWork, messagePushQueueCapacity),
		messagePushAuditQueue:    make(chan messagePushAudit, messagePushQueueCapacity),
		messagePushDailyInterval: time.Minute,
		automationExecutionGate:  newAutomationExecutionGate(),
		listHostSnapshots:        guard.ListHostProcessSnapshots,
		stopHostPID:              guard.StopHostPID,
		closeHostWindows:         guard.CloseHostWindows,
		launchHost:               guard.LaunchHost,
		minimizeHostWindows:      guard.MinimizeHostWindows,
		restartLaunchDelay:       2 * time.Second,
		sleep:                    time.Sleep,
	}
	app.waitForRuntimeStatus = func(timeout time.Duration, accept func(farmruntime.Status) bool) bool {
		return waitForRuntimeStatus(app.manager, timeout, accept)
	}
	app.waitForQQMiniappClosed = func(rootPID int, timeout time.Duration) (guard.QQCloseObservation, error) {
		return waitForQQMiniappClosed(app.manager, app.listHostSnapshots, rootPID, timeout)
	}
	guardianCaller.base = appSupervisorRuntimeCaller{app: app}
	guardianCaller.guardian = guardian
	guardianCaller.generation = guardian.Generation
	qqLink.OnRuntimeEvent(func(event map[string]any) {
		app.handleGuardianRuntimeEventPayload(string(farmruntime.RuntimeTargetQQWS), event)
	})
	qqLink.OnLog(func(level, message string, data map[string]any) {
		app.recordRuntimeLogEvent(eventbus.Event{
			Level:   eventbus.Level(level),
			Source:  "qq_ws",
			Type:    "qqhost.log",
			Message: message,
			Data:    data,
		})
	})
	wechatLink.OnRuntimeEvent(func(event map[string]any) {
		app.handleGuardianRuntimeEventPayload(string(farmruntime.RuntimeTargetWeChatCDP), event)
	})
	wechatLink.OnRuntimeEvent(func(event map[string]any) {
		app.handleCDPHostLogEvent(string(farmruntime.RuntimeTargetWeChatCDP), event)
	})
	yybLink.OnRuntimeEvent(func(event map[string]any) {
		app.handleGuardianRuntimeEventPayload(string(farmruntime.RuntimeTargetYYBCDP), event)
	})
	yybLink.OnRuntimeEvent(func(event map[string]any) {
		app.handleCDPHostLogEvent(string(farmruntime.RuntimeTargetYYBCDP), event)
	})
	app.runtimeCache = automation.NewRuntimeCallCache(guardianCaller, automation.RuntimeCallCacheOptions{
		Scope: app.runtimeCallCacheScope,
	})
	manager.OnStatusChange(func(previous farmruntime.Status, next farmruntime.Status) {
		if runtimeStatusInvalidatesCache(previous, next) {
			app.runtimeCache.Clear()
			app.clearLandDetailsSnapshot()
		}
		app.recordRuntimeStatus(previous, next)
	})
	var assetFS fs.FS
	if len(lanAssets) > 0 {
		assetFS = lanAssets[0]
	}
	app.lanAccess = newLANAccessManager(lanAccessManagerOptions{
		Assets: assetFS,
		GameAssets: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !farm.ServeLocalGameConfigImage(w, r) {
				http.NotFound(w, r)
			}
		}),
		Poll: newRuntimePollHandler(newAppRuntimePollService(app)),
		RPC:  newLANRPCDispatcher(app),
		Save: func(ctx context.Context, settings storage.LANAccessSettings) error {
			if app.store == nil {
				return errors.New("LAN access storage is not available")
			}
			return app.store.SaveLANAccessSettings(ctx, settings)
		},
	})
	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if root, err := installBundledGameConfig(a.dataDir); err != nil {
		a.lastErr = fmt.Errorf("install bundled game config: %w", err)
		slog.Error("install bundled game config", "error", err)
	} else {
		farm.SetGameConfigRoot(root)
	}
	if python, err := installBundledFridaPython(a.dataDir); err != nil {
		a.lastErr = fmt.Errorf("install bundled Frida Python: %w", err)
		slog.Error("install bundled Frida Python", "error", err)
	} else {
		a.fridaPython = python
		a.rebuildCDPLinks()
	}
	runCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.processCtx = runCtx

	store, err := storage.Open(ctx, filepath.Join(a.dataDir, "data"))
	if err != nil {
		a.lastErr = err
		a.recordEvent(eventbus.Event{
			Level:   eventbus.LevelError,
			Source:  "storage",
			Type:    "storage.open",
			Message: "open storage failed",
			Data:    map[string]any{"error": err.Error()},
		})
		a.manager.SetStatus(farmruntime.Status{
			Target:    string(farmruntime.RuntimeTargetQQWS),
			Phase:     farmruntime.PhaseError,
			LastError: err.Error(),
		})
		return
	}
	a.store = store
	settings, err := store.LoadLANAccessSettings(ctx)
	if err != nil {
		a.lastErr = fmt.Errorf("load LAN access settings: %w", err)
		slog.Error("load LAN access settings", "error", err)
	} else if err := a.lanAccess.Restore(settings); err != nil {
		a.lastErr = fmt.Errorf("restore LAN access: %w", err)
		slog.Error("restore LAN access", "error", err)
	}
	a.recordEvent(eventbus.Event{
		Level:   eventbus.LevelInfo,
		Source:  "app",
		Type:    "app.startup",
		Message: "Farm_Go started",
		Data:    map[string]any{"dataDir": a.dataDir},
	})
	if err := a.activateAuthorizedRuntime(ctx, 1); err != nil {
		a.lastErr = err
		slog.Error("start authorized runtime", "error", err)
	}
}

func (a *App) shutdown(ctx context.Context) {
	if a.lanAccess != nil {
		if err := a.lanAccess.Close(); err != nil {
			slog.Error("close LAN access server", "error", err)
		}
	}
	_ = a.deactivateAuthorizedRuntime(context.Background(), 0)
	if a.cancel != nil {
		a.cancel()
	}
	if a.store != nil {
		if err := a.store.Close(); err != nil {
			slog.Error("close storage", "error", err)
		}
	}
	farm.SetGameConfigRoot("")
}

func (a *App) RuntimeStatus() farmruntime.Status {
	return a.service.RuntimeStatus(a.contextOrBackground())
}

func (a *App) ConnectionInfo() farmapp.ConnectionInfo {
	return a.service.ConnectionInfo(a.contextOrBackground())
}

func (a *App) RunDiagnostic(method string, params map[string]any) diagnostics.Result {
	result := a.service.RunDiagnostic(a.contextOrBackground(), method, params)
	level := eventbus.LevelInfo
	message := "diagnostic completed"
	if !result.OK {
		level = eventbus.LevelWarn
		message = "diagnostic failed"
	}
	a.recordEvent(eventbus.Event{
		Level:   level,
		Source:  "diagnostics",
		Type:    "diagnostic.run",
		Message: message,
		Data: map[string]any{
			"method":     method,
			"ok":         result.OK,
			"durationMs": result.DurationMS,
			"error":      result.Error,
		},
	})
	return result
}

func (a *App) PreviewWeChatCacheCleanup() (maintenance.Summary, error) {
	return a.maintenance.PreviewWeChat()
}

func (a *App) CleanWeChatCache() (maintenance.Summary, error) {
	return a.maintenance.CleanWeChat()
}

func (a *App) CleanQQMiniappCache() (maintenance.Summary, error) {
	return a.maintenance.CleanQQ()
}

func (a *App) CleanYYBMiniappCache() (maintenance.Summary, error) {
	return a.maintenance.CleanYYB()
}

func (a *App) RuntimeSettings() storage.RuntimeSettings {
	if a.store != nil {
		settings, err := a.store.LoadRuntimeSettings(a.contextOrBackground())
		if err != nil {
			a.lastErr = err
			return a.runtimeSettingsFromConfig()
		}
		accountKey := a.accountKey()
		warehouse, err := a.store.LoadWarehouseAutoSellSettingsForAccount(a.contextOrBackground(), accountKey)
		if err != nil {
			a.lastErr = err
			a.recordEventForAccount(accountKey, eventbus.Event{
				Level:   eventbus.LevelError,
				Source:  "settings",
				Type:    "warehouse.auto_sell.settings.load",
				Message: "load warehouse auto sell settings failed",
				Data:    map[string]any{"error": err.Error()},
			})
			warehouse = storage.DefaultWarehouseAutoSellSettings()
		}
		return a.normalizeRuntimeSettings(applyWarehouseSettings(settings, warehouse))
	}
	return a.runtimeSettingsFromConfig()
}

func (a *App) MessagePushState() messagepush.ViewState {
	service, accountKey := a.ensureMessagePushService()
	state, err := service.State(a.contextOrBackground())
	if err != nil {
		a.lastErr = err
		a.recordMessagePushEventForAccount(accountKey, eventbus.LevelError, "message_push.state", "load message push state failed", map[string]any{"error": err.Error()})
		return messagepush.ViewState{Config: messagepush.NormalizeConfig(messagepush.Config{})}
	}
	return state
}

func (a *App) SaveMessagePushConfig(config messagepush.Config) (messagepush.ViewState, error) {
	service, accountKey := a.ensureMessagePushService()
	state, err := service.SaveConfig(a.contextOrBackground(), config)
	if err != nil {
		a.recordMessagePushEventForAccount(accountKey, eventbus.LevelError, "message_push.config.save", "save message push config failed", map[string]any{"error": err.Error()})
		a.lastErr = err
		return messagepush.ViewState{}, err
	}
	a.recordMessagePushEventForAccount(accountKey, eventbus.LevelInfo, "message_push.config.save", "message push config saved", map[string]any{
		"enabled":          state.Config.Enabled,
		"selectedChannels": state.Config.SelectedChannels,
	})
	return state, nil
}

func (a *App) SendMessagePushTest(config messagepush.Config) messagepush.SendResult {
	service, accountKey := a.ensureMessagePushService()
	result, err := service.SendTest(a.contextOrBackground(), config)
	if err != nil {
		result.OK = false
		result.Summary = err.Error()
	}
	a.recordMessagePushSendForAccount(accountKey, "message_push.test", "message push test sent", result, err)
	return result
}

func (a *App) SendMessagePushDailyTest(config messagepush.Config) messagepush.SendResult {
	service, accountKey := a.ensureMessagePushService()
	result, err := service.SendDailyNow(a.contextOrBackground(), config)
	if err != nil {
		result.OK = false
		result.Summary = err.Error()
	}
	a.recordMessagePushSendForAccount(accountKey, "message_push.daily_test", "message push daily test sent", result, err)
	return result
}

func (a *App) RunDueMessagePushDaily() messagepush.SendResult {
	service, accountKey := a.ensureMessagePushService()
	result, err := service.RunDueDaily(a.contextOrBackground())
	a.recordMessagePushSendForAccount(accountKey, "message_push.daily_due", "message push daily check completed", result, err)
	return result
}

func (a *App) PreviewMessagePushTemplate(config messagepush.Config, messageType string, channel string) messagepush.TemplatePreview {
	service, _ := a.ensureMessagePushService()
	preview, err := service.PreviewTemplate(config, messagepush.MessageType(messageType), channel)
	if err != nil {
		preview.OK = false
		preview.Error = err.Error()
	}
	return preview
}

func (a *App) DefaultMessagePushTemplate(messageType string, channel string) messagepush.Template {
	return messagepush.DefaultTemplate(messagepush.MessageType(messageType), channel)
}

func (a *App) SendMessagePushTemplateTest(config messagepush.Config, messageType string, channel string) messagepush.SendResult {
	service, _ := a.ensureMessagePushService()
	result, err := service.SendTemplateTest(a.contextOrBackground(), config, messagepush.MessageType(messageType), channel)
	if err != nil {
		result.OK = false
		result.Summary = err.Error()
	}
	return result
}

func (a *App) SaveRuntimeSettings(settings storage.RuntimeSettings) (storage.RuntimeSettings, error) {
	settings = a.normalizeRuntimeSettings(settings)
	baseline := a.currentRuntimeSettingsForChangeDetection()
	if a.store != nil {
		if err := a.store.SaveRuntimeSettings(a.contextOrBackground(), settings); err != nil {
			a.recordEvent(eventbus.Event{
				Level:   eventbus.LevelError,
				Source:  "settings",
				Type:    "settings.save",
				Message: "save runtime settings failed",
				Data:    map[string]any{"error": err.Error()},
			})
			a.lastErr = err
			return storage.RuntimeSettings{}, err
		}
		a.recordEvent(eventbus.Event{
			Level:   eventbus.LevelInfo,
			Source:  "settings",
			Type:    "settings.save",
			Message: "runtime settings saved",
			Data: map[string]any{
				"defaultTarget": settings.DefaultTarget,
				"currentTarget": settings.CurrentTarget,
				"autoStart":     settings.AutoStart,
				"cdpPort":       settings.CDPPort,
				"wmpfDebugPort": settings.WMPFDebugPort,
			},
		})
	}
	if runtimeSettingsAffectRuntime(baseline, settings) {
		a.applyRuntimeSettings(settings)
	}
	if cdpLinkSettingsChanged(baseline, settings) {
		a.rebuildCDPLinks()
	}
	return settings, nil
}

func (a *App) WarehouseAutoSellSettings() storage.WarehouseAutoSellSettings {
	accountKey := a.accountKey()
	if a.store == nil {
		return storage.DefaultWarehouseAutoSellSettings()
	}
	settings, err := a.store.LoadWarehouseAutoSellSettingsForAccount(a.contextOrBackground(), accountKey)
	if err != nil {
		a.lastErr = err
		a.recordEventForAccount(accountKey, eventbus.Event{
			Level:   eventbus.LevelError,
			Source:  "settings",
			Type:    "warehouse.auto_sell.settings.load",
			Message: "load warehouse auto sell settings failed",
			Data:    map[string]any{"error": err.Error()},
		})
		return storage.DefaultWarehouseAutoSellSettings()
	}
	return settings
}

func (a *App) SaveWarehouseAutoSellSettings(settings storage.WarehouseAutoSellSettings) (storage.WarehouseAutoSellSettings, error) {
	accountKey := a.accountKey()
	settings = normalizeWarehouseAutoSellSettings(settings)
	if a.store == nil {
		return settings, nil
	}
	a.automationSettingsWriteMu.Lock()
	defer a.automationSettingsWriteMu.Unlock()
	if hook := a.beforeWarehouseAutoSellSettingsSave; hook != nil {
		hook()
	}
	if err := a.store.SaveWarehouseAutoSellSettingsForAccount(a.contextOrBackground(), accountKey, settings); err != nil {
		a.recordEventForAccount(accountKey, eventbus.Event{
			Level:   eventbus.LevelError,
			Source:  "settings",
			Type:    "warehouse.auto_sell.settings.save",
			Message: "save warehouse auto sell settings failed",
			Data:    map[string]any{"error": err.Error()},
		})
		a.lastErr = err
		return storage.WarehouseAutoSellSettings{}, err
	}
	a.recordEventForAccount(accountKey, eventbus.Event{
		Level:   eventbus.LevelInfo,
		Source:  "settings",
		Type:    "warehouse.auto_sell.settings.save",
		Message: "warehouse auto sell settings saved",
		Data: map[string]any{
			"enabled":        settings.Enabled,
			"intervalMinute": settings.IntervalMinute,
			"categories":     append([]string(nil), settings.Categories...),
		},
	})
	a.reconfigureExistingAutomationSchedulerForAccount(accountKey, settings)
	return settings, nil
}

func (a *App) reconfigureExistingAutomationSchedulerForAccount(accountKey string, warehouse storage.WarehouseAutoSellSettings) {
	if a.store == nil {
		return
	}
	stored, err := a.store.LoadAutoFarmSettingsForAccount(a.contextOrBackground(), accountKey)
	if err != nil {
		a.lastErr = err
		return
	}
	settings := applyWarehouseAutoSellToAutomationSettings(automationSettingsFromStorage(stored), warehouse)
	a.automationSchedulerMu.Lock()
	defer a.automationSchedulerMu.Unlock()
	if a.automationScheduler != nil && a.automationSchedulerAccountKey == accountKey && a.accountKey() == accountKey {
		a.automationScheduler.Configure(settings)
	}
}

func warehouseSettingsFromRuntime(settings storage.RuntimeSettings) storage.WarehouseAutoSellSettings {
	return storage.WarehouseAutoSellSettings{
		Enabled:               settings.AutoWarehouseSellEnabled,
		IntervalMinute:        settings.AutoWarehouseSellIntervalMinute,
		Categories:            append([]string(nil), settings.AutoWarehouseSellCategories...),
		RefreshOnlyOnAutoSell: true,
	}
}

func applyWarehouseSettings(settings storage.RuntimeSettings, warehouse storage.WarehouseAutoSellSettings) storage.RuntimeSettings {
	settings.AutoWarehouseSellEnabled = warehouse.Enabled
	settings.AutoWarehouseSellIntervalMinute = warehouse.IntervalMinute
	settings.AutoWarehouseSellCategories = append([]string(nil), warehouse.Categories...)
	settings.WarehouseRefreshOnlyOnAutoSell = true
	return settings
}

func normalizeWarehouseAutoSellSettings(settings storage.WarehouseAutoSellSettings) storage.WarehouseAutoSellSettings {
	runtimeSettings := storage.NormalizeWarehouseRuntimeSettings(applyWarehouseSettings(storage.RuntimeSettings{}, settings))
	return warehouseSettingsFromRuntime(runtimeSettings)
}

func (a *App) currentRuntimeSettingsForChangeDetection() storage.RuntimeSettings {
	if a.store != nil {
		if settings, err := a.store.LoadRuntimeSettings(a.contextOrBackground()); err == nil {
			return a.normalizeRuntimeSettings(settings)
		}
	}
	return a.normalizeRuntimeSettings(a.runtimeSettingsFromConfig())
}

func runtimeSettingsAffectRuntime(before storage.RuntimeSettings, after storage.RuntimeSettings) bool {
	if before.DefaultTarget != after.DefaultTarget ||
		before.CurrentTarget != after.CurrentTarget ||
		before.AutoStart != after.AutoStart ||
		before.CDPPort != after.CDPPort ||
		before.WMPFDebugPort != after.WMPFDebugPort {
		return true
	}
	return before.ProcessGuardEnabled != after.ProcessGuardEnabled ||
		before.ProcessGuardFailureRecoveryEnabled != after.ProcessGuardFailureRecoveryEnabled ||
		before.ProcessGuardTimeoutThreshold != after.ProcessGuardTimeoutThreshold ||
		before.ProcessGuardMonitorIntervalMS != after.ProcessGuardMonitorIntervalMS ||
		before.ProcessGuardRestartReconnectGraceSec != after.ProcessGuardRestartReconnectGraceSec ||
		before.ProcessGuardMaxRestartsPer10Min != after.ProcessGuardMaxRestartsPer10Min ||
		before.ProcessGuardScheduledRestartEnabled != after.ProcessGuardScheduledRestartEnabled ||
		before.ProcessGuardScheduledRestartIntervalMin != after.ProcessGuardScheduledRestartIntervalMin ||
		before.ProcessGuardAutoMinimizeAfterRestart != after.ProcessGuardAutoMinimizeAfterRestart ||
		before.NetworkReconnectEnabled != after.NetworkReconnectEnabled ||
		before.NetworkReconnectIntervalMS != after.NetworkReconnectIntervalMS ||
		before.NetworkReconnectRecoveryTimeoutMS != after.NetworkReconnectRecoveryTimeoutMS ||
		before.OtherPlaceLoginReconnectEnabled != after.OtherPlaceLoginReconnectEnabled ||
		before.OtherPlaceLoginCheckIntervalMS != after.OtherPlaceLoginCheckIntervalMS ||
		before.OtherPlaceLoginReconnectDelayMin != after.OtherPlaceLoginReconnectDelayMin
}

func cdpLinkSettingsChanged(before storage.RuntimeSettings, after storage.RuntimeSettings) bool {
	return before.CDPPort != after.CDPPort || before.WMPFDebugPort != after.WMPFDebugPort
}

func (a *App) SwitchRuntimeTarget(target string) farmruntime.Status {
	selected := farmruntime.RuntimeTarget(target)
	if selected == "" {
		selected = farmruntime.RuntimeTargetQQWS
	}
	// Switching runtime chains always drops the previous session account binding.
	// Business settings must rebind to the newly identified GID after reconnect.
	a.clearRuntimeAccountBinding("runtime.switch")
	a.recordEvent(eventbus.Event{
		Level:   eventbus.LevelInfo,
		Source:  string(selected),
		Type:    "runtime.switch",
		Message: "switch runtime target requested",
		Data:    map[string]any{"target": string(selected)},
	})
	err := a.supervisor.Switch(a.contextOrBackground(), selected)
	status := a.supervisor.Status()
	if err != nil {
		a.lastErr = err
		status = a.manager.Status()
		a.recordEvent(eventbus.Event{
			Level:   eventbus.LevelError,
			Source:  string(selected),
			Type:    "runtime.switch",
			Message: "switch runtime target failed",
			Data:    map[string]any{"target": string(selected), "error": err.Error()},
		})
	} else {
		a.recordEvent(eventbus.Event{
			Level:   eventbus.LevelInfo,
			Source:  string(selected),
			Type:    "runtime.switch",
			Message: "runtime target switched",
			Data:    map[string]any{"target": string(selected), "phase": string(status.Phase)},
		})
	}
	a.cfg.Runtime.CurrentTarget = string(selected)
	if a.store != nil {
		settings := a.runtimeSettingsFromConfig()
		if saveErr := a.store.SaveRuntimeSettings(a.contextOrBackground(), settings); saveErr != nil {
			a.lastErr = saveErr
		}
	}
	return status
}

func (a *App) RuntimeLinkStatus() farmruntime.Status {
	return a.supervisor.Status()
}

func (a *App) RuntimeEvents(limit int) []eventbus.Event {
	return a.runtimeEventsForAccount(a.accountKey(), limit)
}

func (a *App) TSDKRuntimeEvents(limit int) []eventbus.Event {
	return a.runtimeEventsForAccount(tsdkRuntimeAccountKey, limit)
}

func (a *App) runtimeEventsForAccount(accountKey string, limit int) []eventbus.Event {
	accountKey = storage.NormalizeAccountKey(accountKey)
	if limit <= 0 {
		limit = 100
	}
	if a.store != nil {
		events, err := a.store.ListRuntimeEventsForAccount(a.contextOrBackground(), accountKey, limit)
		if err == nil {
			return events
		}
		a.lastErr = err
	}

	a.eventsMu.Lock()
	defer a.eventsMu.Unlock()
	capacity := limit
	if capacity > len(a.memoryEvents) {
		capacity = len(a.memoryEvents)
	}
	events := make([]eventbus.Event, 0, capacity)
	for i := len(a.memoryEvents) - 1; i >= 0 && len(events) < limit; i-- {
		entry := a.memoryEvents[i]
		if entry.accountKey == accountKey {
			events = append(events, entry.event)
		}
	}
	return events
}

func (a *App) InstallQQDebugPatch(targetPath string) qqpatch.Result {
	return a.runQQDebugPatch("manual", targetPath)
}

func (a *App) QQDebugPatchStatus() QQDebugPatchStatus {
	a.patchMu.RLock()
	defer a.patchMu.RUnlock()
	return a.patchStatus
}

func (a *App) FarmFeatureCatalog() farm.FeatureCatalog {
	return farm.Catalog()
}

func (a *App) FarmAutomationState() automation.State {
	return a.farmAutomationStateForAccount(a.accountKey())
}

func (a *App) farmAutomationStateForAccount(accountKey string) automation.State {
	accountKey = storage.NormalizeAccountKey(accountKey)
	a.automationSchedulerMu.Lock()
	scheduler := a.automationScheduler
	schedulerAccountKey := a.automationSchedulerAccountKey
	if scheduler != nil && schedulerAccountKey == accountKey {
		state := scheduler.State()
		a.automationSchedulerMu.Unlock()
		return state
	}
	a.automationSchedulerMu.Unlock()
	if a.store != nil {
		settings, err := a.store.LoadAutoFarmSettingsForAccount(a.contextOrBackground(), accountKey)
		if err == nil {
			automationSettings := automationSettingsFromStorage(settings)
			warehouseSettings, warehouseErr := a.store.LoadWarehouseAutoSellSettingsForAccount(a.contextOrBackground(), accountKey)
			if warehouseErr == nil {
				automationSettings = applyWarehouseAutoSellToAutomationSettings(automationSettings, warehouseSettings)
			} else {
				a.lastErr = warehouseErr
			}
			return automation.StateFromSettings(automationSettings)
		}
		a.lastErr = err
		a.recordEventForAccount(accountKey, eventbus.Event{
			Level:   eventbus.LevelError,
			Source:  "auto_farm",
			Type:    "auto_farm.settings.load",
			Message: "load auto farm settings failed",
			Data:    map[string]any{"accountKey": accountKey, "error": err.Error()},
		})
	}
	return automation.DefaultState()
}

func applyWarehouseAutoSellToAutomationSettings(settings automation.Settings, warehouse storage.WarehouseAutoSellSettings) automation.Settings {
	if settings.Config == nil {
		settings.Config = map[string]any{}
	}
	settings.Config["autoWarehouseSellEnabled"] = warehouse.Enabled
	settings.Config["autoWarehouseSellIntervalMinute"] = warehouse.IntervalMinute
	settings.Config["autoWarehouseSellIntervalSec"] = warehouse.IntervalMinute * 60
	settings.Config["autoWarehouseSellCategories"] = append([]string(nil), warehouse.Categories...)
	settings.Config["warehouseRefreshOnlyOnAutoSell"] = true
	for index := range settings.Tasks {
		if settings.Tasks[index].ID != "auto_warehouse_sell" {
			continue
		}
		settings.Tasks[index].Enabled = warehouse.Enabled
		settings.Tasks[index].IntervalSec = warehouse.IntervalMinute * 60
	}
	return settings
}

func warehouseAutoSellFromAutomationSettings(settings automation.Settings, fallback storage.WarehouseAutoSellSettings) storage.WarehouseAutoSellSettings {
	warehouse := fallback
	intervalSec := warehouse.IntervalMinute * 60
	if settings.Config != nil {
		warehouse.Enabled = boolFromAny(settings.Config["autoWarehouseSellEnabled"], warehouse.Enabled)
		intervalSec = intFromAny(settings.Config["autoWarehouseSellIntervalSec"], intervalSec)
	}
	for _, task := range settings.Tasks {
		if task.ID != "auto_warehouse_sell" {
			continue
		}
		warehouse.Enabled = task.Enabled
		intervalSec = task.IntervalSec
		break
	}
	if intervalSec < 60 {
		intervalSec = 60
	}
	warehouse.IntervalMinute = (intervalSec + 59) / 60
	return normalizeWarehouseAutoSellSettings(warehouse)
}

func (a *App) FarmSocialState(refresh bool) social.State {
	return a.socialService().State(a.contextOrBackground(), social.StateRequest{Refresh: refresh})
}

func (a *App) FarmSocialAction(input social.FriendActionRequest) social.ActionResult {
	if strings.TrimSpace(input.Action) == "view_qq" && a.manager.Status().Target != string(farmruntime.RuntimeTargetQQWS) {
		return social.ActionResult{
			OK: false, Status: social.StatusUnsupported,
			Message: "查看好友QQ仅支持 QQ 运行链路。",
		}
	}
	return a.socialService().Action(a.contextOrBackground(), input)
}

func (a *App) FarmSocialProtocolBlockList(refresh bool) social.ProtocolBlockList {
	return a.socialService().ProtocolBlockList(a.contextOrBackground(), false)
}

func (a *App) FarmSocialRankings(input social.RankingRequest) social.RankingPage {
	return a.socialService().Rankings(a.contextOrBackground(), input)
}

func (a *App) FarmSocialRefreshVisitors() social.ActionResult {
	return a.socialService().RefreshVisitors(a.contextOrBackground())
}

func (a *App) FarmSocialRankingPreferences() social.RankingPreferences {
	return a.socialService().RankingPreferences(a.contextOrBackground())
}

func (a *App) SaveFarmSocialRankingPreferences(accountKey string, input social.RankingPreferences) (social.RankingPreferences, error) {
	accountKey = storage.NormalizeAccountKey(accountKey)
	if accountKey != a.accountKey() {
		return social.RankingPreferences{}, errors.New("stale social ranking preference scope: " + accountKey)
	}
	return a.socialServiceForAccount(accountKey).SaveRankingPreferences(a.contextOrBackground(), input)
}

func (a *App) FarmSocialDogGuardState() social.DogGuardState {
	return a.socialDogGuardScanner().State()
}

func (a *App) FarmSocialDogGuardAction(input social.DogGuardActionRequest) social.DogGuardState {
	scanner := a.socialDogGuardScanner()
	action := strings.ToLower(strings.TrimSpace(input.Action))
	switch action {
	case "", "state":
		return scanner.State()
	case "start", "scan":
		return scanner.Start(a.contextOrBackground(), input.DogGuardScanRequest)
	case "stop":
		return scanner.Stop()
	case "clear":
		return scanner.Clear(a.contextOrBackground())
	case "wait":
		return scanner.Wait(a.contextOrBackground())
	default:
		state := scanner.State()
		state.Error = "不支持的护主犬操作：" + input.Action
		return state
	}
}

func (a *App) FarmSocialExport(input social.ExportRequest) social.ImportExportPayload {
	return a.socialService().Export(a.contextOrBackground(), input)
}

func (a *App) FarmSocialImport(input social.ImportExportPayload) social.ActionResult {
	return a.socialService().Import(a.contextOrBackground(), input)
}

func (a *App) SaveFarmAutomationState(state automation.State) (automation.State, error) {
	accountKey := a.accountKey()
	settings := automation.SettingsFromState(state)
	a.automationSettingsWriteMu.Lock()
	defer a.automationSettingsWriteMu.Unlock()
	current := automation.SettingsFromState(a.farmAutomationStateForAccount(accountKey))
	settings.RunMode = current.RunMode
	settings.Config = automation.MergeRuntimeDailyState(settings.Config, current.Config)
	warehouse := storage.DefaultWarehouseAutoSellSettings()
	if a.store != nil {
		loaded, err := a.store.LoadWarehouseAutoSellSettingsForAccount(a.contextOrBackground(), accountKey)
		if err != nil {
			a.recordEventForAccount(accountKey, eventbus.Event{
				Level:   eventbus.LevelError,
				Source:  "auto_farm",
				Type:    "auto_farm.settings.save",
				Message: "load warehouse auto sell settings before automation save failed",
				Data:    map[string]any{"accountKey": accountKey, "error": err.Error()},
			})
			a.lastErr = err
			return automation.State{}, err
		}
		warehouse = loaded
	}
	settings = applyWarehouseAutoSellToAutomationSettings(settings, warehouse)
	if hook := a.beforeAutomationSettingsWrite; hook != nil {
		hook()
	}
	if a.store != nil {
		if err := a.store.SaveAutoFarmSettingsForAccount(a.contextOrBackground(), accountKey, storageAutoFarmSettings(settings)); err != nil {
			a.recordEventForAccount(accountKey, eventbus.Event{
				Level:   eventbus.LevelError,
				Source:  "auto_farm",
				Type:    "auto_farm.settings.save",
				Message: "save auto farm settings failed",
				Data:    map[string]any{"accountKey": accountKey, "error": err.Error()},
			})
			a.lastErr = err
			return automation.State{}, err
		}
		a.recordEventForAccount(accountKey, eventbus.Event{
			Level:   eventbus.LevelInfo,
			Source:  "auto_farm",
			Type:    "auto_farm.settings.save",
			Message: "auto farm settings saved",
			Data: map[string]any{
				"accountKey":                   accountKey,
				"schedulerEnabled":             settings.SchedulerEnabled,
				"schedulerMinGapMs":            settings.SchedulerMinGapMs,
				"taskCount":                    len(settings.Tasks),
				"ownCollectEnabled":            boolFromAny(settings.Config["autoFarmOwnCollectEnabled"], false),
				"ownCollectIntervalSec":        intFromAny(settings.Config["autoFarmOwnCollectIntervalSec"], 0),
				"fertilizerEnabled":            boolFromAny(settings.Config["autoFarmFertilizerEnabled"], false),
				"rushFertilizerMode":           strings.TrimSpace(stringFromAny(settings.Config["autoFarmRushFertilizerMode"])),
				"fertilizerRushThresholdSec":   intFromAny(settings.Config["autoFarmFertilizerRushThresholdSec"], 0),
				"fertilizerHarvestLinkEnabled": boolFromAny(settings.Config["autoFarmFertilizerHarvestLinkEnabled"], false),
			},
		})
	}
	a.configureCurrentAutomationScheduler(accountKey, settings)
	return automation.StateFromSettings(settings), nil
}

func (a *App) SetFarmAutomationRunMode(value string) (automation.State, error) {
	mode, err := automation.ParseRunMode(value)
	if err != nil {
		return automation.State{}, err
	}
	accountKey := a.accountKey()
	a.automationSettingsWriteMu.Lock()
	defer a.automationSettingsWriteMu.Unlock()

	settings := automation.SettingsFromState(a.farmAutomationStateForAccount(accountKey))
	settings.RunMode = mode
	if a.store != nil {
		if err := a.store.SaveAutoFarmSettingsForAccount(a.contextOrBackground(), accountKey, storageAutoFarmSettings(settings)); err != nil {
			a.recordEventForAccount(accountKey, eventbus.Event{Level: eventbus.LevelError, Source: "auto_farm", Type: "auto_farm.run_mode.save", Message: "save automation run mode failed", Data: map[string]any{"accountKey": accountKey, "error": err.Error()}})
			a.lastErr = err
			return automation.State{}, err
		}
	}
	a.configureCurrentAutomationScheduler(accountKey, settings)
	return automation.StateFromSettings(settings), nil
}

func (a *App) configureCurrentAutomationScheduler(accountKey string, settings automation.Settings) {
	a.automationSchedulerMu.Lock()
	if a.automationScheduler != nil && a.automationSchedulerAccountKey == accountKey {
		a.automationScheduler.Configure(settings)
	}
	a.automationSchedulerMu.Unlock()
}

func (a *App) RunFarmAutomationTask(taskID string) automation.ActionResult {
	return a.runFarmAutomationTask(taskID, "manual")
}

func (a *App) RunScheduledFarmAutomationTask(taskID string) automation.ActionResult {
	return a.runFarmAutomationTask(taskID, "auto")
}

func (a *App) FarmAutomationSchedulerState() automation.SchedulerState {
	state, _ := a.runAutomationSchedulerAction(func(scheduler *automation.Scheduler) automation.SchedulerState {
		return scheduler.State().Scheduler
	})
	return state
}

func (a *App) StartFarmAutomationScheduler() automation.SchedulerState {
	state, accountKey := a.runAutomationSchedulerAction(func(scheduler *automation.Scheduler) automation.SchedulerState {
		scheduler.Start(a.contextOrBackground())
		return scheduler.State().Scheduler
	})
	a.recordEventForAccount(accountKey, eventbus.Event{
		Level:   eventbus.LevelInfo,
		Source:  "auto_farm",
		Type:    "scheduler.started",
		Message: "automation scheduler started",
		Data: map[string]any{
			"trigger": "manual",
		},
	})
	return state
}

func (a *App) runBeforeAutomationSchedulerActionHook() {
	a.automationSchedulerMu.Lock()
	hook := a.beforeAutomationSchedulerAction
	a.automationSchedulerMu.Unlock()
	if hook != nil {
		hook()
	}
}

func (a *App) StopFarmAutomationScheduler() automation.SchedulerState {
	for {
		a.automationSchedulerActionMu.Lock()
		accountKey := a.accountKey()
		a.automationSchedulerMu.Lock()
		scheduler := a.automationScheduler
		schedulerAccountKey := a.automationSchedulerAccountKey
		a.automationSchedulerMu.Unlock()
		a.automationSchedulerActionMu.Unlock()
		if scheduler == nil || schedulerAccountKey != accountKey {
			return automation.DefaultState().Scheduler
		}

		a.runBeforeAutomationSchedulerActionHook()
		a.automationSchedulerActionMu.Lock()
		a.automationSchedulerMu.Lock()
		if a.automationScheduler != scheduler || a.automationSchedulerAccountKey != accountKey || a.accountKey() != accountKey {
			a.automationSchedulerMu.Unlock()
			a.automationSchedulerActionMu.Unlock()
			continue
		}
		a.automationSchedulerMu.Unlock()
		scheduler.Stop()
		state := scheduler.State().Scheduler
		a.automationSchedulerActionMu.Unlock()
		a.recordEventForAccount(accountKey, eventbus.Event{
			Level:   eventbus.LevelInfo,
			Source:  "auto_farm",
			Type:    "scheduler.stopped",
			Message: "automation scheduler stopped",
			Data: map[string]any{
				"trigger": "manual",
			},
		})
		return state
	}
}

func (a *App) RunDueFarmAutomationTasks() automation.SchedulerState {
	state, _ := a.runAutomationSchedulerAction(func(scheduler *automation.Scheduler) automation.SchedulerState {
		scheduler.RunDue(a.contextOrBackground())
		return scheduler.State().Scheduler
	})
	return state
}

func (a *App) runAutomationSchedulerAction(action func(*automation.Scheduler) automation.SchedulerState) (automation.SchedulerState, string) {
	for {
		scheduler, accountKey := a.farmAutomationSchedulerForCurrentSettings()
		a.runBeforeAutomationSchedulerActionHook()

		a.automationSchedulerActionMu.Lock()
		a.automationSchedulerMu.Lock()
		if a.automationScheduler != scheduler || a.automationSchedulerAccountKey != accountKey || a.accountKey() != accountKey {
			a.automationSchedulerMu.Unlock()
			a.automationSchedulerActionMu.Unlock()
			continue
		}
		a.automationSchedulerMu.Unlock()
		state := action(scheduler)
		a.automationSchedulerActionMu.Unlock()
		return state, accountKey
	}
}

func (a *App) farmAutomationSchedulerForCurrentSettings() (*automation.Scheduler, string) {
	for {
		accountKey := a.accountKey()
		settings := automation.SettingsFromState(a.farmAutomationStateForAccount(accountKey))
		a.automationSchedulerActionMu.Lock()
		a.automationSchedulerMu.Lock()
		if a.accountKey() != accountKey {
			a.automationSchedulerMu.Unlock()
			a.automationSchedulerActionMu.Unlock()
			continue
		}
		scheduler := a.ensureAutomationSchedulerLocked(accountKey, settings)
		a.automationSchedulerMu.Unlock()
		a.automationSchedulerActionMu.Unlock()
		return scheduler, accountKey
	}
}

func (a *App) ensureAutomationSchedulerLocked(accountKey string, settings automation.Settings) *automation.Scheduler {
	if a.automationScheduler == nil || a.automationSchedulerAccountKey != accountKey {
		if a.automationScheduler != nil {
			a.automationScheduler.Stop()
		}
		a.automationScheduler = automation.NewScheduler(automation.SchedulerOptions{
			Runner: func(ctx context.Context, taskID string) automation.ActionResult {
				return a.executeFarmAutomationTaskForAccount(ctx, taskID, "auto", accountKey)
			},
			OnLog: func(log automation.SchedulerLog) {
				a.recordAutomationSchedulerLogForAccount(accountKey, log)
			},
		})
		a.automationSchedulerAccountKey = accountKey
	}
	a.automationScheduler.Configure(settings)
	return a.automationScheduler
}

func (a *App) recordAutomationSchedulerLogForAccount(accountKey string, log automation.SchedulerLog) {
	level := eventbus.LevelInfo
	if log.Type == "task.failed" {
		level = eventbus.LevelWarn
	}
	message := log.Message
	if message == "" {
		switch log.Type {
		case "task.start":
			message = "automation task started"
		case "task.done":
			message = "automation task completed"
		case "task.failed":
			message = "automation task failed"
		default:
			message = "automation scheduler event"
		}
	}
	a.recordEventForAccount(accountKey, eventbus.Event{
		Level:   level,
		Source:  "auto_farm",
		Type:    log.Type,
		Message: message,
		Data: map[string]any{
			"taskId":      log.TaskID,
			"trigger":     "auto",
			"ok":          log.OK,
			"status":      string(log.Status),
			"startedAt":   log.StartedAt,
			"finishedAt":  log.FinishedAt,
			"durationMs":  log.DurationMs,
			"actionCount": log.ActionCount,
		},
	})
}

func (a *App) runFarmAutomationTask(taskID string, trigger string) automation.ActionResult {
	accountKey := a.accountKey()
	trigger = normalizeAutomationTaskTrigger(trigger)
	startedAt := time.Now()
	a.recordEventForAccount(accountKey, eventbus.Event{
		Level:   eventbus.LevelInfo,
		Source:  "auto_farm",
		Type:    "task.start",
		Message: "automation task started",
		Data: map[string]any{
			"taskId":  taskID,
			"trigger": trigger,
		},
	})

	result := a.executeFarmAutomationTaskForAccount(a.contextOrBackground(), taskID, trigger, accountKey)
	level := eventbus.LevelInfo
	eventType := "task.done"
	message := result.Message
	if message == "" {
		message = "automation task completed"
	}
	if !result.OK {
		level = eventbus.LevelWarn
		eventType = "task.failed"
	}
	a.recordEventForAccount(accountKey, eventbus.Event{
		Level:   level,
		Source:  "auto_farm",
		Type:    eventType,
		Message: message,
		Data: map[string]any{
			"taskId":      result.TaskID,
			"requested":   taskID,
			"trigger":     trigger,
			"ok":          result.OK,
			"status":      string(result.Status),
			"durationMs":  time.Since(startedAt).Milliseconds(),
			"actionCount": result.ActionCount,
		},
	})
	return result
}

func (a *App) executeFarmAutomationTaskForAccount(ctx context.Context, taskID string, trigger string, accountKey string) automation.ActionResult {
	accountKey = storage.NormalizeAccountKey(accountKey)
	if a.guardianDispatchIsPaused() {
		return automation.ActionResult{
			OK:      false,
			Status:  automation.StatusRuntimeBusy,
			TaskID:  taskID,
			Message: "守护服务正在恢复运行环境，请稍后重试。",
		}
	}
	mode := a.farmAutomationStateForAccount(accountKey).RunMode
	var release func()
	if normalizeAutomationTaskTrigger(trigger) == "auto" {
		var err error
		release, err = a.automationExecutionGate.Acquire(ctx, mode)
		if err != nil {
			return automation.ActionResult{
				OK:      false,
				Status:  automation.StatusRuntimeBusy,
				TaskID:  taskID,
				Message: "当前有任务正在执行，请稍后再试。",
			}
		}
	} else {
		var acquired bool
		release, acquired = a.automationExecutionGate.TryAcquire(mode)
		if !acquired {
			return automation.ActionResult{
				OK:      false,
				Status:  automation.StatusRuntimeBusy,
				TaskID:  taskID,
				Message: "当前有任务正在执行，请稍后再试。",
			}
		}
	}
	defer release()
	if taskID == "auto_warehouse_sell" {
		return a.executeWarehouseAutoSellForAccount(ctx, accountKey)
	}
	now := time.Now()
	config := cloneAutomationConfig(a.farmAutomationStateForAccount(accountKey).Config)
	config["automationTrigger"] = trigger
	if trigger != "auto" {
		delete(config, automation.FriendMischiefDailyDoneDateConfigKey)
	}
	if trigger == "auto" && automation.DailyOnceTaskDone(config, taskID, now) {
		return automation.DailyOnceTaskSkipResult(taskID)
	}
	facade := automation.NewRuntimeFacadeWithConfig(a.farmRuntimeCaller(), config)
	if a.store != nil {
		storeAdapter := socialStorageAdapter{store: a.store}
		facade = automation.NewRuntimeFacadeWithConfigAndSocialStore(a.farmRuntimeCaller(), config, storeAdapter, accountKey)
		facade = facade.WithMysteryShopPurchaseRecordWriter(storeAdapter, accountKey)
	}
	facade = facade.WithRunMode(mode)
	result := facade.RunTask(ctx, taskID)
	if trigger == "auto" && result.TaskID == "friend_help" && result.SuccessfulFriends > 0 {
		a.markFriendHelpDailySuccessesForAccount(accountKey, now, result.SuccessfulFriends)
	}
	if trigger == "auto" && automation.IsFriendMischiefDailyLimitResult(result) {
		a.markFriendMischiefDailyDoneForAccount(accountKey, now)
	}
	if automation.IsDailyOnceTaskDoneResult(result) {
		a.markDailyOnceTaskDoneForAccount(accountKey, result.TaskID, now)
	}
	return result
}

func (a *App) executeWarehouseAutoSellForAccount(ctx context.Context, accountKey string) automation.ActionResult {
	settings := storage.DefaultWarehouseAutoSellSettings()
	if a.store != nil {
		loaded, err := a.store.LoadWarehouseAutoSellSettingsForAccount(ctx, accountKey)
		if err != nil {
			return automation.ActionResult{OK: false, Status: automation.StatusFailed, TaskID: "auto_warehouse_sell", Message: "读取仓库自动出售设置失败：" + err.Error()}
		}
		settings = loaded
	}
	if !settings.Enabled || len(settings.Categories) == 0 {
		return automation.ActionResult{OK: true, Status: automation.StatusOK, TaskID: "auto_warehouse_sell", Message: "仓库自动出售已关闭。"}
	}

	value, err := a.farmRuntimeCaller().Call(ctx, "gameCtl.refreshWarehouseSnapshot", []any{map[string]any{
		"silent":         true,
		"preferProtocol": true,
		"protocolWaitMs": 1200,
		"closeAfter":     true,
		"openTimeoutMs":  2600,
		"readTimeoutMs":  3200,
		"allowEmpty":     true,
	}}, 15*time.Second)
	if err != nil {
		return automation.ActionResult{OK: false, Status: automation.StatusFailed, TaskID: "auto_warehouse_sell", Message: "刷新仓库失败：" + err.Error()}
	}
	refreshResult := mapFromAny(value)
	if !boolFromAny(refreshResult["ok"], true) {
		return automation.ActionResult{OK: false, Status: automation.StatusFailed, TaskID: "auto_warehouse_sell", Message: "刷新仓库失败：" + stringFromAny(firstExistingAny(refreshResult["error"], refreshResult["reason"]))}
	}

	itemMap, _ := farm.LoadItemInfoMap(farm.DefaultGameConfigRoot())
	warehouse := farm.BuildRuntimeWarehouse(refreshResult, itemMap)
	categorySet := make(map[string]bool, len(settings.Categories))
	for _, category := range settings.Categories {
		categorySet[category] = true
	}
	itemKeys := make([]string, 0, len(warehouse.Items))
	for _, item := range warehouse.Items {
		if categorySet[item.Category] && item.CanSell && !item.Locked {
			itemKeys = append(itemKeys, item.ID)
		}
	}
	if len(itemKeys) == 0 {
		return automation.ActionResult{OK: true, Status: automation.StatusOK, TaskID: "auto_warehouse_sell", Message: "仓库中没有可出售的指定分类物品。"}
	}

	input := map[string]any{"itemKeys": itemKeys, "mode": "auto"}
	args, err := buildWarehouseSellRuntimeArgs(input)
	if err != nil {
		return automation.ActionResult{OK: false, Status: automation.StatusFailed, TaskID: "auto_warehouse_sell", Message: err.Error()}
	}
	value, err = a.farmRuntimeCaller().Call(ctx, "gameCtl.sellWarehouseItems", []any{args}, 20*time.Second)
	if err != nil {
		return automation.ActionResult{OK: false, Status: automation.StatusFailed, TaskID: "auto_warehouse_sell", Message: "自动出售失败：" + err.Error()}
	}
	result := mapFromAny(value)
	if !boolFromAny(result["ok"], true) {
		return automation.ActionResult{OK: false, Status: automation.StatusFailed, TaskID: "auto_warehouse_sell", Message: "自动出售失败：" + stringFromAny(firstExistingAny(result["error"], result["reason"]))}
	}
	if record, ok := a.saveWarehouseSellRecord(accountKey, input, result); ok {
		result["record"] = record
	}
	return automation.ActionResult{OK: true, Status: automation.StatusOK, TaskID: "auto_warehouse_sell", Message: fmt.Sprintf("仓库自动出售完成，共提交 %d 种物品。", len(itemKeys))}
}

func (a *App) markFriendMischiefDailyDoneForAccount(accountKey string, now time.Time) {
	if a.store == nil {
		return
	}
	a.automationSettingsWriteMu.Lock()
	defer a.automationSettingsWriteMu.Unlock()
	settings := automation.SettingsFromState(a.farmAutomationStateForAccount(accountKey))
	settings.Config = automation.MarkFriendMischiefDailyDone(settings.Config, now)
	a.saveAutomationDailyStateLocked(accountKey, settings, "auto_farm.friend_mischief_daily_done.save", "保存好友捣乱今日完成状态失败：")
}

func (a *App) markFriendHelpDailySuccessesForAccount(accountKey string, now time.Time, successCount int) {
	if a.store == nil || successCount <= 0 {
		return
	}
	a.automationSettingsWriteMu.Lock()
	defer a.automationSettingsWriteMu.Unlock()
	settings := automation.SettingsFromState(a.farmAutomationStateForAccount(accountKey))
	settings.Config = automation.MarkFriendHelpDailySuccesses(settings.Config, now, successCount)
	a.saveAutomationDailyStateLocked(accountKey, settings, "auto_farm.friend_help_daily_count.save", "保存好友帮助每日计数失败：")
}

func (a *App) markDailyOnceTaskDoneForAccount(accountKey string, taskID string, now time.Time) {
	if a.store == nil {
		return
	}
	a.automationSettingsWriteMu.Lock()
	defer a.automationSettingsWriteMu.Unlock()
	settings := automation.SettingsFromState(a.farmAutomationStateForAccount(accountKey))
	settings.Config = automation.MarkDailyOnceTaskDone(settings.Config, taskID, now)
	a.saveAutomationDailyStateLocked(accountKey, settings, "auto_farm.daily_once_done.save", "保存每日一次任务完成状态失败：")
}

func (a *App) saveAutomationDailyStateLocked(accountKey string, settings automation.Settings, eventType string, errorPrefix string) {
	accountKey = storage.NormalizeAccountKey(accountKey)
	if hook := a.beforeAutomationSettingsWrite; hook != nil {
		hook()
	}
	if err := a.store.SaveAutoFarmSettingsForAccount(a.contextOrBackground(), accountKey, storageAutoFarmSettings(settings)); err != nil {
		a.recordEventForAccount(accountKey, eventbus.Event{
			Level:   eventbus.LevelWarn,
			Source:  "auto_farm",
			Type:    eventType,
			Message: errorPrefix + err.Error(),
			Data:    map[string]any{"accountKey": accountKey},
		})
		return
	}
	a.automationSchedulerMu.Lock()
	scheduler := a.automationScheduler
	schedulerAccountKey := a.automationSchedulerAccountKey
	if scheduler != nil && schedulerAccountKey == accountKey {
		scheduler.Configure(settings)
	}
	a.automationSchedulerMu.Unlock()
}

func cloneAutomationConfig(config map[string]any) map[string]any {
	next := make(map[string]any, len(config))
	for key, value := range config {
		next[key] = value
	}
	return next
}

func (a *App) FarmBackpackSeedOptions(input map[string]any) farm.BackpackSeedOptionsPayload {
	if input == nil {
		input = map[string]any{}
	}
	value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.getSeedList", []any{map[string]any{
		"sortMode": 3,
		"silent":   true,
		"noCache":  boolFromAny(input["refresh"], false),
	}}, 8*time.Second)
	if err != nil {
		err = sanitizeAccountRuntimeError(err)
		return farm.BackpackSeedOptionsPayload{
			OK:    false,
			List:  []farm.BackpackSeedOption{},
			Error: err.Error(),
		}
	}
	return farm.BuildBackpackSeedOptions(sliceFromAny(value), farm.BackpackSeedOptionsRequest{
		SelectedSeedIDs:      normalizePositiveUniqueInts(firstExistingAny(input["selectedSeedIds"], input["selected"])),
		DisabledSeedIDs:      normalizePositiveUniqueInts(firstExistingAny(input["disabledSeedIds"], input["disabled"])),
		ForcePriority:        boolFromAny(input["forcePriority"], false),
		FourGridPlantEnabled: boolFromAny(input["fourGridPlantEnabled"], false),
	})
}

func (a *App) FarmStealCropOptions() farm.StealCropOptionsPayload {
	payload, err := farm.BuildStealCropOptions(farm.DefaultGameConfigRoot())
	if err != nil {
		return farm.StealCropOptionsPayload{
			OK:    false,
			List:  []farm.StealCropOption{},
			Error: err.Error(),
		}
	}
	return payload
}

func (a *App) FarmAccountStatus() farm.AccountStatusPayload {
	profileValue, profileErr := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.getPlayerProfile", []any{map[string]any{
		"silent":  true,
		"noCache": true,
	}}, 8*time.Second)
	fertilizerValue, fertilizerErr := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.getFertilizerContainerStatus", []any{map[string]any{
		"silent": true,
	}}, 8*time.Second)
	profileErr = sanitizeAccountRuntimeError(profileErr)
	fertilizerErr = sanitizeAccountRuntimeError(fertilizerErr)
	payload := farm.BuildRuntimeAccountStatus(mapFromAny(profileValue), mapFromAny(fertilizerValue), profileErr, fertilizerErr)
	if history := a.automationStatsHistory(time.Now(), 30); len(history.Days) > 0 {
		payload.Profile.StatsHistory = history
		for _, day := range history.Days {
			if day.DateKey == history.TodayKey {
				payload.Profile.TodayStats = day
				break
			}
		}
	}
	return payload
}

func (a *App) automationStatsHistory(now time.Time, windowDays int) farm.StatsHistoryWindow {
	return a.automationStatsHistoryForAccount(a.contextOrBackground(), a.accountKey(), now, windowDays)
}

func (a *App) FarmWorkspaceRunStatistics() farm.RunStatistics {
	now := time.Now()
	accountKey := a.accountKey()
	stats := farm.BuildRunStatistics(a.runStatisticsStartedAt, now, a.runStatisticsEvents(accountKey))
	stats.SaleEstimate, stats.EstimateReady = a.runWarehouseSellAmount(accountKey, a.runStatisticsStartedAt)
	return stats
}

func (a *App) runWarehouseSellAmount(accountKey string, startedAt time.Time) (int64, bool) {
	if a.store == nil {
		return 0, true
	}
	total, err := a.store.SumWarehouseSellAmountSince(a.contextOrBackground(), accountKey, startedAt)
	if err != nil {
		a.lastErr = err
		return 0, false
	}
	return total, true
}

func (a *App) runStatisticsEvents(accountKey string) []eventbus.Event {
	startedAt := a.runStatisticsStartedAt
	if a.store != nil {
		events, err := a.store.ListRuntimeEventsForAccountSince(a.contextOrBackground(), accountKey, startedAt)
		if err == nil {
			return events
		}
		a.lastErr = err
	}

	a.eventsMu.Lock()
	defer a.eventsMu.Unlock()
	events := make([]eventbus.Event, 0)
	for _, entry := range a.memoryEvents {
		if entry.accountKey == accountKey && !entry.event.Timestamp.Before(startedAt) {
			events = append(events, entry.event)
		}
	}
	return events
}

func (a *App) automationStatsHistoryForAccount(ctx context.Context, accountKey string, now time.Time, windowDays int) farm.StatsHistoryWindow {
	if a.store == nil {
		return farm.StatsHistoryWindow{}
	}
	if windowDays <= 0 {
		windowDays = 30
	}
	todayKey := now.Format("2006-01-02")
	start := localDateStart(now).AddDate(0, 0, 1-windowDays)
	events, err := a.store.ListRuntimeEventsForAccountSince(ctx, storage.NormalizeAccountKey(accountKey), start)
	if err != nil {
		a.lastErr = err
		return farm.StatsHistoryWindow{}
	}

	byDate := map[string]farm.DayStats{}
	for _, event := range events {
		if !isSuccessfulAutomationTaskDone(event) {
			continue
		}
		dateKey := event.Timestamp.Local().Format("2006-01-02")
		if dateKey < start.Format("2006-01-02") || dateKey > todayKey {
			continue
		}
		stats := byDate[dateKey]
		stats.DateKey = dateKey
		stats.UpdatedAt = event.Timestamp.Format(time.RFC3339Nano)
		stats.Runs++
		actionCount := intFromAny(event.Data["actionCount"], 0)
		switch automationTaskIDFromEvent(event) {
		case "own_collect", "own_fertilizer":
			if actionCount > 0 {
				stats.Collect += actionCount
			}
		case "own_base":
			if actionCount > 0 {
				stats.Water += actionCount
			}
		case "friend_steal":
			if actionCount > 0 {
				stats.Steal += actionCount
			}
		case "friend_help":
			if actionCount > 0 {
				stats.Help += actionCount
			}
		case "friend_mischief":
			if actionCount > 0 {
				stats.MischiefGrass += actionCount
			}
		case "warehouse_sell", "auto_warehouse_sell":
			stats.Sell++
		}
		byDate[dateKey] = stats
	}
	saleAmounts, saleErr := a.store.SumWarehouseSellAmountsByDate(ctx, accountKey, start.Format("2006-01-02"), todayKey)
	if saleErr != nil {
		a.lastErr = saleErr
	}

	days := make([]farm.DayStats, 0, windowDays)
	for day := start; !day.After(localDateStart(now)); day = day.AddDate(0, 0, 1) {
		dateKey := day.Format("2006-01-02")
		stats := byDate[dateKey]
		stats.DateKey = dateKey
		stats.SaleEstimate = saleAmounts[dateKey]
		stats.EstimateReady = saleErr == nil
		days = append(days, stats)
	}
	return farm.StatsHistoryWindow{TodayKey: todayKey, Days: days}
}

func localDateStart(value time.Time) time.Time {
	local := value.Local()
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
}

func isSuccessfulAutomationTaskDone(event eventbus.Event) bool {
	if event.Source != "auto_farm" || event.Type != "task.done" {
		return false
	}
	if !boolFromAny(event.Data["ok"], false) {
		return false
	}
	return stringFromAny(event.Data["status"]) == string(automation.StatusOK)
}

func automationTaskIDFromEvent(event eventbus.Event) string {
	return stringFromAny(firstExistingAny(event.Data["taskId"], event.Data["requested"]))
}

func (a *App) CurrentRuntimeAccount() RuntimeAccount {
	a.accountMu.RLock()
	defer a.accountMu.RUnlock()
	if a.currentAccount.AccountKey != "" {
		return a.currentAccount
	}
	return RuntimeAccount{AccountKey: a.accountKeyLocked()}
}

func (a *App) IdentifyRuntimeAccount() RuntimeAccount {
	const attempts = 5
	const retryDelay = 300 * time.Millisecond

	var last RuntimeAccount
	for attempt := 0; attempt < attempts; attempt++ {
		account, retry := a.identifyRuntimeAccountOnce()
		last = account
		if account.GID > 0 || !retry {
			return account
		}
		if attempt < attempts-1 && a.sleep != nil {
			a.sleep(retryDelay)
		}
	}
	return last
}

func (a *App) identifyRuntimeAccountOnce() (RuntimeAccount, bool) {
	if account, done, retry := a.identifyRuntimeAccountByIdentityMethod(); done {
		return account, retry
	}
	if account, done, retry := a.identifyRuntimeAccountBySelfGID(); done {
		return account, retry
	}
	return a.identifyRuntimeAccountByProfile()
}

func (a *App) identifyRuntimeAccountByIdentityMethod() (RuntimeAccount, bool, bool) {
	const timeout = 1200 * time.Millisecond

	value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.getRuntimeAccountIdentity", []any{map[string]any{
		"silent": true,
	}}, timeout)
	if err != nil {
		err = sanitizeAccountRuntimeError(err)
		if isRuntimeMethodUnavailable(err) {
			return RuntimeAccount{}, false, false
		}
		account := RuntimeAccount{AccountKey: a.accountKey(), Error: err.Error()}
		return account, true, err.Error() == "游戏运行时尚未就绪"
	}
	account := runtimeAccountFromProfile(mapFromAny(value))
	if account.GID <= 0 {
		account.Error = "未识别到有效 GId"
		return account, true, true
	}
	return account, true, false
}

func (a *App) identifyRuntimeAccountBySelfGID() (RuntimeAccount, bool, bool) {
	const timeout = 1200 * time.Millisecond

	value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.getSelfGid", []any{}, timeout)
	if err != nil {
		err = sanitizeAccountRuntimeError(err)
		if isRuntimeMethodUnavailable(err) {
			return RuntimeAccount{}, false, false
		}
		account := RuntimeAccount{AccountKey: a.accountKey(), Error: err.Error()}
		return account, true, err.Error() == "游戏运行时尚未就绪"
	}
	gid := intFromAny(value, 0)
	if gid <= 0 {
		return RuntimeAccount{}, false, false
	}
	return RuntimeAccount{
		GID:          gid,
		AccountKey:   storage.AccountKeyForGID(gid),
		IdentifiedAt: time.Now().Format(time.RFC3339Nano),
	}, true, false
}

func (a *App) identifyRuntimeAccountByProfile() (RuntimeAccount, bool) {
	const timeout = 1200 * time.Millisecond

	value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.getPlayerProfile", []any{map[string]any{
		"silent":       true,
		"noCache":      true,
		"identityOnly": true,
	}}, timeout)
	if err != nil {
		err = sanitizeAccountRuntimeError(err)
		account := RuntimeAccount{AccountKey: a.accountKey(), Error: err.Error()}
		return account, err.Error() == "游戏运行时尚未就绪"
	}
	account := runtimeAccountFromProfile(mapFromAny(value))
	if account.GID <= 0 {
		account.Error = "未识别到有效 GId"
		return account, true
	}
	return account, false
}

func isRuntimeMethodUnavailable(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	return strings.Contains(text, "call_path_not_ready") || strings.Contains(text, "not a function")
}

func (a *App) ConfirmRuntimeAccount(input RuntimeAccount) RuntimeAccount {
	account := normalizeRuntimeAccount(input)
	account.Confirmed = false
	if account.GID <= 0 {
		account.Error = "未识别到有效 GId"
		return account
	}
	if account.IdentifiedAt == "" {
		account.IdentifiedAt = time.Now().Format(time.RFC3339Nano)
	}
	if a.store != nil {
		if err := a.store.SaveRuntimeAccount(a.contextOrBackground(), storage.RuntimeAccount{
			AccountKey:   account.AccountKey,
			GID:          account.GID,
			Nickname:     account.Nickname,
			AvatarURL:    account.AvatarURL,
			IdentifiedAt: account.IdentifiedAt,
			ConfirmedAt:  time.Now().Format(time.RFC3339Nano),
		}); err != nil {
			account.Error = err.Error()
			return account
		}
		if err := a.store.SeedLegacyAccountConfiguration(a.contextOrBackground(), account.AccountKey); err != nil {
			account.Error = err.Error()
			return account
		}
	}
	account.Confirmed = true

	a.accountMu.Lock()
	previous := a.currentAccountKey
	a.currentAccountKey = account.AccountKey
	a.currentAccount = account
	a.accountMu.Unlock()

	if previous != account.AccountKey {
		a.resetAccountScopedServices()
	}
	a.recordEvent(eventbus.Event{
		Level:   eventbus.LevelInfo,
		Source:  "account",
		Type:    "account.confirmed",
		Message: "runtime account confirmed",
		Data: map[string]any{
			"gid":         account.GID,
			"accountKey":  account.AccountKey,
			"displayName": account.Nickname,
		},
	})
	return account
}

func sanitizeAccountRuntimeError(err error) error {
	if err == nil {
		return nil
	}
	text := err.Error()
	if strings.Contains(text, "gameCtl_not_ready") || strings.Contains(text, "runtime evaluator is not connected") || strings.Contains(text, "execution context is not ready") {
		return errors.New("游戏运行时尚未就绪")
	}
	return err
}

func runtimeAccountFromProfile(profile map[string]any) RuntimeAccount {
	gid := intFromAny(firstExistingAny(profile["gid"], profile["uid"], profile["player_id"], profile["playerId"], profile["roleId"]), 0)
	account := RuntimeAccount{
		GID:          gid,
		AccountKey:   storage.AccountKeyForGID(gid),
		Nickname:     firstNonEmptyString(stringFromAny(profile["name"]), stringFromAny(profile["displayName"]), stringFromAny(profile["nick"]), stringFromAny(profile["nickname"]), stringFromAny(profile["role_name"]), stringFromAny(profile["limitName"])),
		AvatarURL:    firstNonEmptyString(stringFromAny(profile["avatarUrl"]), stringFromAny(profile["avatar_url"]), stringFromAny(profile["avatar"])),
		IdentifiedAt: time.Now().Format(time.RFC3339Nano),
	}
	return account
}

func normalizeRuntimeAccount(input RuntimeAccount) RuntimeAccount {
	if input.GID <= 0 {
		input.GID = intFromAny(strings.TrimPrefix(input.AccountKey, "gid:"), 0)
	}
	input.AccountKey = storage.AccountKeyForGID(input.GID)
	return input
}

func (a *App) FarmCropAnalytics() farm.CropAnalyticsPayload {
	root := farm.DefaultGameConfigRoot()
	var profile map[string]any
	var seedList []any
	var shopList []any
	var runtimeError string
	if value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.getPlayerProfile", []any{map[string]any{"silent": true}}, 8*time.Second); err == nil {
		profile = mapFromAny(value)
	} else {
		runtimeError = err.Error()
	}
	if value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.getSeedList", []any{map[string]any{"sortMode": 3, "silent": true}}, 8*time.Second); err == nil {
		seedList = sliceFromAny(value)
	} else if runtimeError == "" {
		runtimeError = err.Error()
	}
	_, _ = a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.requestShopData", []any{2}, 8*time.Second)
	levelInfo := farm.NormalizeAnalyticsLevelRequest(0, profile)
	if value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.getShopSeedList", []any{map[string]any{
		"sortByLevel": true,
		"silent":      true,
		"playerLevel": levelInfo.EffectiveMaxLevel,
	}}, 8*time.Second); err == nil {
		shopList = sliceFromAny(value)
	} else if runtimeError == "" {
		runtimeError = err.Error()
	}
	payload, err := farm.LoadCropAnalyticsForLevel(root, farm.CropAnalyticsOptions{
		Profile:      profile,
		SeedList:     seedList,
		ShopList:     shopList,
		RuntimeError: runtimeError,
	})
	if err != nil {
		return farm.CropAnalyticsPayload{
			Source: "resources/gameConfig",
			Items:  []farm.CropAnalyticsItem{},
			Error:  err.Error(),
		}
	}
	return payload
}

func (a *App) FarmAtlasPreview() farm.AtlasPreviewPayload {
	if value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.requestAtlasUnlockRowsByProtocol", []any{map[string]any{
		"silent": true,
		"waitMs": 2200,
	}}, 30*time.Second); err == nil {
		return farm.BuildRuntimeAtlasPreview(mapFromAny(value))
	}
	payload, err := farm.LoadAtlasPreview(farm.DefaultGameConfigRoot())
	if err != nil {
		return farm.AtlasPreviewPayload{
			Source:         "resources/gameConfig",
			Status:         "static_preview",
			Message:        "图鉴运行时刷新和购买待迁移，当前无法读取本地配置。",
			RefreshEnabled: false,
			BuyEnabled:     false,
			Sections:       []farm.AtlasSection{},
			Error:          err.Error(),
		}
	}
	return payload
}

func (a *App) FarmAtlasBuyLockedPreview(input map[string]any) farm.AtlasPurchasePreviewPayload {
	if input == nil {
		input = map[string]any{}
	}
	items := farm.AtlasItemsFromAny(input["items"])
	if len(items) == 0 {
		return farm.AtlasPurchasePreviewPayload{OK: false, Error: "请先刷新作物图鉴数据"}
	}
	profile, levelInfo, shopList, err := a.atlasPurchaseContext()
	if err != nil {
		return farm.AtlasPurchasePreviewPayload{OK: false, Error: err.Error(), Profile: profile, Level: levelInfo}
	}
	plan := farm.BuildAtlasLockedCropPurchasePlan(farm.AtlasLockedCropPurchaseInput{
		AtlasItems:     items,
		ShopList:       shopList,
		EffectiveLevel: levelInfo.EffectiveMaxLevel,
		CountPerSeed:   intFromAny(input["countPerSeed"], 1),
	})
	return farm.AtlasPurchasePreviewPayload{OK: true, Profile: profile, Level: levelInfo, Plan: plan}
}

func (a *App) FarmAtlasBuyLockedCrops(input map[string]any) farm.AtlasPurchasePayload {
	if input == nil {
		input = map[string]any{}
	}
	preview := a.FarmAtlasBuyLockedPreview(input)
	if !preview.OK {
		return farm.AtlasPurchasePayload{OK: false, Error: preview.Error, Profile: preview.Profile, Level: preview.Level, Plan: preview.Plan}
	}
	if len(preview.Plan.Purchases) == 0 {
		return farm.AtlasPurchasePayload{
			OK:       true,
			Profile:  preview.Profile,
			Level:    preview.Level,
			Plan:     preview.Plan,
			Purchase: map[string]any{"ok": true, "bought": []any{}, "failed": []any{}, "attempts": []any{}},
		}
	}
	seeds := make([]any, 0, len(preview.Plan.Purchases))
	for _, purchase := range preview.Plan.Purchases {
		seeds = append(seeds, map[string]any{
			"seedId":        purchase.SeedID,
			"seedName":      purchase.SeedName,
			"goodsId":       purchase.GoodsID,
			"price":         purchase.Price,
			"count":         purchase.Count,
			"requiredLevel": purchase.RequiredLevel,
			"sort":          purchase.Sort,
		})
	}
	timeout := 15*time.Second + time.Duration(len(seeds))*8*time.Second
	if timeout < 30*time.Second {
		timeout = 30 * time.Second
	}
	if timeout > 300*time.Second {
		timeout = 300 * time.Second
	}
	value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.buyShopSeedsBatch", []any{map[string]any{
		"seeds":       seeds,
		"silent":      true,
		"stopOnError": boolFromAny(input["stopOnError"], false),
	}}, timeout)
	if err != nil {
		return farm.AtlasPurchasePayload{OK: false, Error: err.Error(), Profile: preview.Profile, Level: preview.Level, Plan: preview.Plan}
	}
	purchase := mapFromAny(value)
	ok := true
	if len(purchase) > 0 {
		ok = boolFromAny(purchase["ok"], true)
	}
	return farm.AtlasPurchasePayload{OK: ok, Profile: preview.Profile, Level: preview.Level, Plan: preview.Plan, Purchase: purchase}
}

func (a *App) atlasPurchaseContext() (map[string]any, farm.AnalyticsLevelInfo, []any, error) {
	var profile map[string]any
	if value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.getPlayerProfile", []any{map[string]any{"silent": true}}, 8*time.Second); err == nil {
		profile = mapFromAny(value)
	} else {
		return nil, farm.AnalyticsLevelInfo{}, nil, err
	}
	levelInfo := farm.NormalizeAnalyticsLevelRequest(0, profile)
	_, _ = a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.requestShopData", []any{2}, 8*time.Second)
	value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.getShopSeedList", []any{map[string]any{
		"sortByLevel": true,
		"silent":      true,
		"playerLevel": levelInfo.EffectiveMaxLevel,
	}}, 8*time.Second)
	if err != nil {
		return profile, levelInfo, nil, err
	}
	return profile, levelInfo, sliceFromAny(value), nil
}

func (a *App) FarmLandDetails() farm.LandDetailsPayload {
	a.landDetailsOperationMu.Lock()
	defer a.landDetailsOperationMu.Unlock()
	payload := a.readLandDetails()
	a.landDetailsMu.Lock()
	defer a.landDetailsMu.Unlock()
	a.publishLandDetailsLocked(payload)
	return a.landDetailsSnapshot
}

func (a *App) FarmLandDetailsSince(revision string) farm.LandDetailsDeltaPayload {
	a.landDetailsOperationMu.Lock()
	defer a.landDetailsOperationMu.Unlock()
	payload := a.readLandDetails()
	a.landDetailsMu.Lock()
	defer a.landDetailsMu.Unlock()
	if revision == "" || revision != a.landDetailsSnapshot.Revision {
		a.publishLandDetailsLocked(payload)
		return landDetailsFullDelta(a.landDetailsSnapshot)
	}
	if payload.RuntimeError != "" {
		return farm.LandDetailsDeltaPayload{
			Revision:       a.landDetailsSnapshot.Revision,
			Lands:          []farm.LandDetailsItem{},
			RemovedLandIDs: []int{},
		}
	}

	signs := a.stabilizeLandDetailsLocked(&payload)
	changed := make([]farm.LandDetailsItem, 0)
	for _, land := range payload.Lands {
		if a.landDetailsSigns[land.LandID] != signs[land.LandID] {
			changed = append(changed, land)
		}
	}
	removed := make([]int, 0)
	for landID := range a.landDetailsSigns {
		if _, found := signs[landID]; !found {
			removed = append(removed, landID)
		}
	}
	topState := landDetailsTopState(payload)
	if len(changed) == 0 && len(removed) == 0 && topState == a.landDetailsTopState {
		return farm.LandDetailsDeltaPayload{
			Revision:       a.landDetailsSnapshot.Revision,
			Lands:          []farm.LandDetailsItem{},
			RemovedLandIDs: []int{},
		}
	}
	a.landDetailsRevision++
	payload.Revision = strconv.FormatUint(a.landDetailsRevision, 10)
	a.landDetailsSnapshot = payload
	a.landDetailsSigns = signs
	a.landDetailsTopState = topState
	return farm.LandDetailsDeltaPayload{
		Revision:       payload.Revision,
		Status:         payload.Status,
		Message:        payload.Message,
		FarmType:       payload.FarmType,
		TotalGrids:     payload.TotalGrids,
		Lands:          changed,
		RemovedLandIDs: removed,
		Actions:        payload.Actions,
		RuntimeError:   payload.RuntimeError,
	}
}

func (a *App) readLandDetails() farm.LandDetailsPayload {
	value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.getFarmStatus", []any{map[string]any{
		"includeGrids":          true,
		"includeLandIds":        false,
		"includeRawGrid":        true,
		"includeRawLandRuntime": true,
		"noCache":               true,
		"silent":                true,
	}}, 15*time.Second)
	if err != nil {
		payload := farm.LandDetailsGate()
		payload.RuntimeError = err.Error()
		return payload
	}
	return farm.BuildRuntimeLandDetails(mapFromAny(value))
}

func (a *App) publishLandDetailsLocked(payload farm.LandDetailsPayload) {
	signs := a.stabilizeLandDetailsLocked(&payload)
	a.landDetailsRevision++
	payload.Revision = strconv.FormatUint(a.landDetailsRevision, 10)
	a.landDetailsSnapshot = payload
	a.landDetailsSigns = signs
	a.landDetailsTopState = landDetailsTopState(payload)
}

func (a *App) stabilizeLandDetailsLocked(payload *farm.LandDetailsPayload) map[int]string {
	signs := make(map[int]string, len(payload.Lands))
	previousByID := make(map[int]farm.LandDetailsItem, len(a.landDetailsSnapshot.Lands))
	for _, land := range a.landDetailsSnapshot.Lands {
		previousByID[land.LandID] = land
	}
	for index := range payload.Lands {
		land := &payload.Lands[index]
		signature := farm.LandStateSignature(*land)
		if previous, found := previousByID[land.LandID]; found && a.landDetailsSigns[land.LandID] == signature && previous.MatureAtMs > 0 {
			land.MatureAtMs = previous.MatureAtMs
		}
		signs[land.LandID] = signature
	}
	return signs
}

func (a *App) clearLandDetailsSnapshot() {
	a.landDetailsMu.Lock()
	defer a.landDetailsMu.Unlock()
	a.landDetailsRevision = 0
	a.landDetailsSnapshot = farm.LandDetailsPayload{}
	a.landDetailsSigns = nil
	a.landDetailsTopState = ""
}

func landDetailsTopState(payload farm.LandDetailsPayload) string {
	payload.Revision = ""
	payload.Lands = nil
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func landDetailsFullDelta(payload farm.LandDetailsPayload) farm.LandDetailsDeltaPayload {
	return farm.LandDetailsDeltaPayload{
		Full:           true,
		Revision:       payload.Revision,
		Status:         payload.Status,
		Message:        payload.Message,
		FarmType:       payload.FarmType,
		TotalGrids:     payload.TotalGrids,
		Lands:          payload.Lands,
		RemovedLandIDs: []int{},
		Actions:        payload.Actions,
		RuntimeError:   payload.RuntimeError,
	}
}

func (a *App) FarmLandRush(input map[string]any) map[string]any {
	args, err := buildLandRushRuntimeArgs(input)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	if scope, _ := input["fertilizerSubmissionScope"].(string); scope == "manual" {
		args = automation.ApplyFertilizerSubmissionRuntimeArgs(
			args,
			a.farmAutomationStateForAccount(a.accountKey()).Config,
			"manual",
		)
	}
	value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.fertilizeLandsBatch", []any{args}, 45*time.Second)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	result := mapFromAny(value)
	if len(result) == 0 {
		result = map[string]any{"ok": true, "result": value}
	}
	if _, ok := result["ok"]; !ok {
		result["ok"] = true
	}
	if result["ok"] == false {
		return result
	}
	if !boolFromAny(args["linkedHarvestAfterFertilize"], false) {
		return result
	}
	return a.attachDeadCleanupAfterHarvest(
		result,
		"farm_go_land_rush_dead_cleanup",
		a.farmAutomationStateForAccount(a.accountKey()).Config,
	)
}

func (a *App) FarmFertilizeLand(input map[string]any) map[string]any {
	args, err := buildFertilizeLandRuntimeArgs(input)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.fertilizeLand", []any{args}, 45*time.Second)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	result := mapFromAny(value)
	if len(result) == 0 {
		return map[string]any{"ok": true, "result": value}
	}
	if _, ok := result["ok"]; !ok {
		result["ok"] = true
	}
	return result
}

func (a *App) FarmShovelLands(input map[string]any) map[string]any {
	args, err := buildShovelLandsRuntimeArgs(input)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.shovelLandsBatch", []any{args}, 90*time.Second)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	result := mapFromAny(value)
	if len(result) == 0 {
		return map[string]any{"ok": true, "result": value}
	}
	if _, ok := result["ok"]; !ok {
		result["ok"] = true
	}
	return result
}

func (a *App) attachDeadCleanupAfterHarvest(result map[string]any, source string, config map[string]any) map[string]any {
	statusValue, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.getFarmStatus", []any{map[string]any{
		"includeGrids":          true,
		"includeLandIds":        true,
		"includeRawGrid":        true,
		"includeRawLandRuntime": true,
		"silent":                true,
	}}, 15*time.Second)
	if err != nil {
		result["ok"] = false
		result["deadCleanup"] = map[string]any{
			"ok":        false,
			"deadCount": 0,
			"landIds":   []int{},
			"error":     err.Error(),
		}
		return result
	}

	status := mapFromAny(statusValue)
	deadLandIDs := collectRuntimeDeadLandIDs(status)
	cleanup := map[string]any{
		"ok":        true,
		"deadCount": len(deadLandIDs),
		"landIds":   deadLandIDs,
	}
	if len(deadLandIDs) > 0 {
		shovelValue, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.shovelLandsBatch", []any{map[string]any{
			"landIds":         deadLandIDs,
			"onlyDead":        true,
			"silent":          true,
			"dryRun":          false,
			"waitAfterAction": 0,
			"betweenLandWait": 0,
			"source":          source,
		}}, 45*time.Second)
		if err != nil {
			result["ok"] = false
			cleanup["ok"] = false
			cleanup["error"] = err.Error()
			result["deadCleanup"] = cleanup
			return result
		}
		shovelResult := mapFromAny(shovelValue)
		if len(shovelResult) == 0 {
			cleanup["result"] = shovelValue
		} else {
			cleanup["result"] = shovelResult
			if _, ok := shovelResult["ok"]; ok && shovelResult["ok"] == false {
				result["ok"] = false
				cleanup["ok"] = false
				cleanup["error"] = firstExistingAny(shovelResult["reason"], shovelResult["message"], shovelResult["error"], "unknown")
				result["deadCleanup"] = cleanup
				return result
			}
		}
	}
	result["deadCleanup"] = cleanup

	mode := automation.MultiSeasonFertilizerMode(config)
	landIDs := automation.MultiSeasonContinuationLandIDs(
		status,
		automation.SuccessfulRuntimeLandIDs(mapFromAny(result["linkedHarvest"])),
	)
	if mode == "" || len(landIDs) == 0 {
		return result
	}

	fertilizerValue, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.fertilizeLandsBatch", []any{map[string]any{
		"landIds":                     landIDs,
		"type":                        mode,
		"mode":                        mode,
		"dryRun":                      false,
		"cleanupUi":                   true,
		"linkedHarvestAfterFertilize": false,
		"silent":                      true,
		"source":                      "farm_go_land_rush_multi_season",
	}}, 45*time.Second)
	if err != nil {
		result["ok"] = false
		result["multiSeasonFertilizer"] = map[string]any{
			"ok":      false,
			"landIds": landIDs,
			"mode":    mode,
			"error":   err.Error(),
		}
		return result
	}

	fertilizerResult := mapFromAny(fertilizerValue)
	if len(fertilizerResult) > 0 {
		result["multiSeasonFertilizer"] = fertilizerResult
		if fertilizerResult["ok"] == false {
			result["ok"] = false
		}
	} else {
		result["multiSeasonFertilizer"] = map[string]any{
			"ok":      true,
			"landIds": landIDs,
			"mode":    mode,
			"result":  fertilizerValue,
		}
	}
	return result
}

func collectRuntimeDeadLandIDs(status map[string]any) []int {
	seen := map[int]bool{}
	for _, item := range sliceFromAny(status["grids"]) {
		grid := mapFromAny(item)
		if !runtimeGridIsDead(grid) {
			continue
		}
		id := intFromAny(firstExistingAny(grid["landId"], grid["id"]), 0)
		if id > 0 {
			seen[id] = true
		}
	}

	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func runtimeGridIsDead(grid map[string]any) bool {
	if len(grid) == 0 {
		return false
	}
	stageKind, _ := grid["stageKind"].(string)
	if strings.EqualFold(strings.TrimSpace(stageKind), "dead") {
		return true
	}
	for _, key := range []string{"isDead", "canEraseDead", "needsEraseDead", "needEraseDead"} {
		if boolFromAny(grid[key], false) {
			return true
		}
	}
	return false
}

func (a *App) FarmWarehouse() farm.WarehousePayload {
	return a.FarmWarehouseRefresh()
}

func (a *App) FarmWarehouseRefresh() farm.WarehousePayload {
	value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.refreshWarehouseSnapshot", []any{map[string]any{
		"silent":         true,
		"preferProtocol": true,
		"protocolWaitMs": 1200,
		"closeAfter":     true,
		"openTimeoutMs":  2600,
		"readTimeoutMs":  3200,
		"allowEmpty":     true,
	}}, 15*time.Second)
	if err != nil {
		payload := farm.WarehouseGate()
		payload.RuntimeError = err.Error()
		return payload
	}
	itemMap, _ := farm.LoadItemInfoMap(farm.DefaultGameConfigRoot())
	return farm.BuildRuntimeWarehouse(mapFromAny(value), itemMap)
}

func (a *App) FarmWarehouseSell(input map[string]any) farm.WarehouseSellPayload {
	accountKey := a.accountKey()
	args, err := buildWarehouseSellRuntimeArgs(input)
	if err != nil {
		return farm.WarehouseSellPayload{OK: false, Error: err.Error(), Warehouse: farm.WarehouseGate()}
	}
	value, err := a.farmRuntimeCaller().Call(a.contextOrBackground(), "gameCtl.sellWarehouseItems", []any{args}, 20*time.Second)
	if err != nil {
		payload := farm.WarehouseGate()
		payload.RuntimeError = err.Error()
		return farm.WarehouseSellPayload{OK: false, Error: err.Error(), Warehouse: payload}
	}
	result := mapFromAny(value)
	itemMap, _ := farm.LoadItemInfoMap(farm.DefaultGameConfigRoot())
	warehouseItems := firstExistingAny(result["afterItems"], result["items"], result["beforeItems"])
	warehouse := farm.BuildRuntimeWarehouse(map[string]any{"items": warehouseItems}, itemMap)
	if record, ok := a.saveWarehouseSellRecord(accountKey, input, result); ok {
		result["record"] = record
	}
	return farm.WarehouseSellPayload{
		OK:        boolFromAny(result["ok"], true),
		Warehouse: warehouse,
		Sell:      result,
	}
}

func (a *App) FarmWarehouseSellRecords(input map[string]any) []storage.WarehouseSellRecord {
	if a.store == nil {
		return []storage.WarehouseSellRecord{}
	}
	dateKey := ""
	limit := 100
	if input != nil {
		dateKey = stringFromAny(firstExistingAny(input["date"], input["dateKey"]))
		limit = intFromAny(input["limit"], limit)
	}
	records, err := a.store.ListWarehouseSellRecords(a.contextOrBackground(), a.accountKey(), dateKey, limit)
	if err != nil {
		a.lastErr = err
		return []storage.WarehouseSellRecord{}
	}
	return records
}

func (a *App) FarmMysteryShopPurchaseRecords() []storage.MysteryShopPurchaseRecord {
	if a.store == nil {
		return []storage.MysteryShopPurchaseRecord{}
	}
	records, err := a.store.ListMysteryShopPurchaseRecords(a.contextOrBackground(), a.accountKey(), 50)
	if err != nil {
		a.lastErr = err
		return []storage.MysteryShopPurchaseRecord{}
	}
	return records
}

func (a *App) saveWarehouseSellRecord(accountKey string, input map[string]any, result map[string]any) (storage.WarehouseSellRecord, bool) {
	if a.store == nil || !boolFromAny(result["ok"], true) {
		return storage.WarehouseSellRecord{}, false
	}
	items := warehouseSellRecordItemsFromResult(result)
	if len(items) == 0 {
		return storage.WarehouseSellRecord{}, false
	}
	now := time.Now()
	mode := normalizeWarehouseSellRecordMode(stringFromAny(firstExistingAny(input["mode"], input["source"])))
	record := storage.WarehouseSellRecord{
		ID:         mode + ":" + strconv.FormatInt(now.UnixNano(), 10),
		DateKey:    now.Local().Format("2006-01-02"),
		OccurredAt: now.Format(time.RFC3339Nano),
		Mode:       mode,
		ItemKinds:  len(items),
		Items:      items,
		Payload:    warehouseSellRecordPayload(result),
	}
	for _, item := range items {
		record.TotalCount += item.Count
		record.TotalAmount += item.Amount
	}
	if record.TotalAmount <= 0 {
		record.TotalAmount = warehouseSellRewardAmount(result)
	}
	if record.TotalCount <= 0 {
		return storage.WarehouseSellRecord{}, false
	}
	if err := a.store.AppendWarehouseSellRecord(a.contextOrBackground(), accountKey, record); err != nil {
		a.lastErr = err
		return storage.WarehouseSellRecord{}, false
	}
	return record, true
}

func normalizeWarehouseSellRecordMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "auto", "automatic", "scheduler":
		return "auto"
	default:
		return "manual"
	}
}

func warehouseSellRecordItemsFromResult(result map[string]any) []storage.WarehouseSellRecordItem {
	items := []storage.WarehouseSellRecordItem{}
	for _, raw := range sliceFromAny(result["soldDiff"]) {
		item := mapFromAny(raw)
		count := intFromAny(firstExistingAny(item["soldCount"], item["count"]), 0)
		itemID := intFromAny(firstExistingAny(item["itemId"], item["id"]), 0)
		if itemID <= 0 || count <= 0 {
			continue
		}
		unitPrice := intFromAny(firstExistingAny(item["saleUnitPrice"], item["unitPrice"]), 0)
		amount := intFromAny(item["amount"], unitPrice*count)
		items = append(items, storage.WarehouseSellRecordItem{
			ItemID:    itemID,
			Name:      stringFromAny(item["name"]),
			Count:     count,
			UnitPrice: unitPrice,
			Amount:    amount,
		})
	}
	if len(items) > 0 {
		return items
	}
	for _, raw := range sliceFromAny(result["requestPayload"]) {
		item := mapFromAny(raw)
		count := intFromAny(item["count"], 0)
		itemID := intFromAny(firstExistingAny(item["id"], item["itemId"]), 0)
		if itemID <= 0 || count <= 0 {
			continue
		}
		items = append(items, storage.WarehouseSellRecordItem{ItemID: itemID, Count: count})
	}
	return items
}

func warehouseSellRewardAmount(result map[string]any) int {
	total := 0
	for _, raw := range sliceFromAny(result["protocolRewards"]) {
		reward := mapFromAny(raw)
		total += intFromAny(firstExistingAny(reward["amount"], reward["count"]), 0)
	}
	return total
}

func warehouseSellRecordPayload(result map[string]any) map[string]any {
	payload := map[string]any{}
	for _, key := range []string{"source", "requestPayload", "soldDiff", "protocolRewards", "observedChange"} {
		if value, ok := result[key]; ok {
			payload[key] = value
		}
	}
	return payload
}

func (a *App) ProcessGuardStatus() guard.Status {
	if a.guard == nil {
		return guard.Status{Phase: guard.PhaseDisabled}
	}
	return a.guard.Status()
}

func (a *App) GuardianStatus() GuardianStatusDTO {
	settings := guardSettingsFromRuntimeSettings(a.currentRuntimeSettingsForChangeDetection())
	if a.guardian == nil {
		return GuardianStatusDTO{Phase: guard.PhaseDisabled, Settings: settings, RecentEvents: []guard.RuntimeEvent{}}
	}
	status := a.guardian.Status()
	return GuardianStatusDTO{
		Enabled:         status.Enabled,
		Running:         status.Running,
		Phase:           status.Phase,
		RuntimeTarget:   status.RuntimeTarget,
		Settings:        settings,
		Process:         status.Process,
		Network:         status.Network,
		OtherPlaceLogin: status.OtherPlaceLogin,
		RecentEvents:    status.RecentEvents,
	}
}

func (a *App) SaveGuardianSettings(input map[string]any) (guard.Settings, error) {
	previous := a.currentRuntimeSettingsForChangeDetection()
	settings := previous
	if value, ok := input["enabled"]; ok {
		settings.ProcessGuardEnabled = boolFromAny(value, settings.ProcessGuardEnabled)
	}
	if value, ok := input["failureRecoveryEnabled"]; ok {
		settings.ProcessGuardFailureRecoveryEnabled = boolFromAny(value, settings.ProcessGuardFailureRecoveryEnabled)
	}
	if value, ok := input["timeoutThreshold"]; ok {
		settings.ProcessGuardTimeoutThreshold = intFromAny(value, settings.ProcessGuardTimeoutThreshold)
	}
	if value, ok := input["monitorIntervalMs"]; ok {
		settings.ProcessGuardMonitorIntervalMS = intFromAny(value, settings.ProcessGuardMonitorIntervalMS)
	}
	if value, ok := input["restartReconnectGraceSec"]; ok {
		settings.ProcessGuardRestartReconnectGraceSec = intFromAny(value, settings.ProcessGuardRestartReconnectGraceSec)
	}
	if value, ok := input["maxRestartsPer10Min"]; ok {
		settings.ProcessGuardMaxRestartsPer10Min = intFromAny(value, settings.ProcessGuardMaxRestartsPer10Min)
	}
	if value, ok := input["scheduledRestartEnabled"]; ok {
		settings.ProcessGuardScheduledRestartEnabled = boolFromAny(value, settings.ProcessGuardScheduledRestartEnabled)
	}
	if value, ok := input["scheduledRestartIntervalMin"]; ok {
		settings.ProcessGuardScheduledRestartIntervalMin = intFromAny(value, settings.ProcessGuardScheduledRestartIntervalMin)
	}
	if value, ok := input["autoMinimizeAfterRestart"]; ok {
		settings.ProcessGuardAutoMinimizeAfterRestart = boolFromAny(value, settings.ProcessGuardAutoMinimizeAfterRestart)
	}
	if value, ok := input["networkReconnectEnabled"]; ok {
		settings.NetworkReconnectEnabled = boolFromAny(value, settings.NetworkReconnectEnabled)
	}
	if value, ok := input["networkReconnectIntervalMs"]; ok {
		settings.NetworkReconnectIntervalMS = intFromAny(value, settings.NetworkReconnectIntervalMS)
	}
	if value, ok := input["networkRecoveryTimeoutMs"]; ok {
		settings.NetworkReconnectRecoveryTimeoutMS = intFromAny(value, settings.NetworkReconnectRecoveryTimeoutMS)
	}
	if value, ok := input["otherPlaceLoginEnabled"]; ok {
		settings.OtherPlaceLoginReconnectEnabled = boolFromAny(value, settings.OtherPlaceLoginReconnectEnabled)
	}
	if value, ok := input["otherPlaceLoginIntervalMs"]; ok {
		settings.OtherPlaceLoginCheckIntervalMS = intFromAny(value, settings.OtherPlaceLoginCheckIntervalMS)
	}
	if value, ok := input["otherPlaceLoginDelayMin"]; ok {
		settings.OtherPlaceLoginReconnectDelayMin = intFromAny(value, settings.OtherPlaceLoginReconnectDelayMin)
	}
	saved, err := a.SaveRuntimeSettings(settings)
	if err != nil {
		return guardSettingsFromRuntimeSettings(previous), err
	}
	if !saved.ProcessGuardAutoMinimizeAfterRestart {
		a.clearPendingHostMinimize()
	}
	return guardSettingsFromRuntimeSettings(saved), nil
}

func (a *App) RunGuardianAction(input guard.ActionRequest) guard.ActionResult {
	switch strings.ToLower(strings.TrimSpace(input.Action)) {
	case "restart":
		result := a.RestartHostProcess()
		if result.Status == "launch_dispatched" || result.Status == guard.RestartStatusReconnected {
			return guard.ActionResult{OK: true, Status: result.Status}
		}
		return guard.ActionResult{Status: result.Status, Error: result.Reason}
	case "launch":
		result := a.LaunchHostProcess()
		if result.Status == "launch_dispatched" {
			return guard.ActionResult{OK: true, Status: result.Status}
		}
		return guard.ActionResult{Status: result.Status, Error: result.Reason}
	default:
		return guard.ActionResult{Status: "unsupported_action", Error: "unsupported guardian action"}
	}
}

func (a *App) SaveProcessGuardSettings(input map[string]any) (map[string]any, error) {
	previous := a.currentRuntimeSettingsForChangeDetection()
	settings := previous
	if value, ok := input["enabled"]; ok {
		settings.ProcessGuardEnabled = boolFromAny(value, settings.ProcessGuardEnabled)
	}
	if value, ok := input["timeoutThreshold"]; ok {
		settings.ProcessGuardTimeoutThreshold = intFromAny(value, settings.ProcessGuardTimeoutThreshold)
	}
	if value, ok := input["monitorIntervalMs"]; ok {
		settings.ProcessGuardMonitorIntervalMS = intFromAny(value, settings.ProcessGuardMonitorIntervalMS)
	}
	if value, ok := input["restartReconnectGraceSec"]; ok {
		settings.ProcessGuardRestartReconnectGraceSec = intFromAny(value, settings.ProcessGuardRestartReconnectGraceSec)
	}
	if value, ok := input["maxRestartsPer10Min"]; ok {
		settings.ProcessGuardMaxRestartsPer10Min = intFromAny(value, settings.ProcessGuardMaxRestartsPer10Min)
	}
	if value, ok := input["scheduledRestartEnabled"]; ok {
		settings.ProcessGuardScheduledRestartEnabled = boolFromAny(value, settings.ProcessGuardScheduledRestartEnabled)
	}
	if value, ok := input["scheduledRestartIntervalMin"]; ok {
		settings.ProcessGuardScheduledRestartIntervalMin = intFromAny(value, settings.ProcessGuardScheduledRestartIntervalMin)
	}
	if value, ok := input["autoMinimizeAfterRestart"]; ok {
		settings.ProcessGuardAutoMinimizeAfterRestart = boolFromAny(value, settings.ProcessGuardAutoMinimizeAfterRestart)
	}
	saved, err := a.SaveRuntimeSettings(settings)
	if err != nil {
		return processGuardSettingsMap(previous), err
	}
	if !saved.ProcessGuardAutoMinimizeAfterRestart {
		a.clearPendingHostMinimize()
	}
	return processGuardSettingsMap(saved), nil
}

func (a *App) ListHostCandidates() []guard.HostProcessCandidate {
	snapshots, err := a.listHostSnapshots()
	if err != nil {
		a.lastErr = err
		return nil
	}
	return guard.CandidatesForPlatform(a.guardPlatform(), snapshots)
}

func (a *App) AutoBindHostProcess() guard.AutoBindOwnerResult {
	candidates := a.ListHostCandidates()
	result, err := a.hosts.AutoBind("main", candidates)
	if err != nil {
		a.lastErr = err
		return guard.AutoBindOwnerResult{Owner: "main", Status: "error", Candidates: candidates}
	}
	return result
}

func (a *App) BindHostProcess(pid int) guard.HostBinding {
	for _, candidate := range a.ListHostCandidates() {
		if candidate.PID != pid {
			continue
		}
		binding, err := a.hosts.Bind("main", candidate)
		if err != nil {
			a.lastErr = err
			return guard.HostBinding{}
		}
		return binding
	}
	a.lastErr = os.ErrNotExist
	return guard.HostBinding{}
}

func (a *App) ClearHostBinding() bool {
	return a.hosts.Clear("main")
}

func (a *App) PreviewHostRestart() guard.HostRestartPreview {
	a.AutoBindHostProcess()
	snapshots, err := a.listHostSnapshots()
	if err != nil {
		a.lastErr = err
		return guard.HostRestartPreview{Owner: "main", Status: "snapshot_failed", Reason: err.Error()}
	}
	return guard.PreviewRestart(a.hosts, "main", snapshots)
}

func (a *App) RestartHostProcess() guard.RestartResult {
	snapshot := guard.RuntimeSnapshot{RuntimeTarget: a.cfg.Runtime.CurrentTarget}
	if current := a.manager.Status(); current.Target != "" {
		if snapshot.RuntimeTarget == "" {
			snapshot.RuntimeTarget = current.Target
		}
		snapshot.Ready = current.Ready
		snapshot.Connected = current.Connected
	}
	result, err := a.guard.ManualRestart(snapshot, "manual restart")
	if err != nil {
		a.lastErr = err
		return guard.RestartResult{Owner: "main", Status: "restart_failed", Reason: err.Error()}
	}
	return result
}

func (a *App) LaunchHostProcess() guard.HostLaunchResult {
	platform := a.guardPlatform()
	var snapshots []guard.HostProcessSnapshot
	if platform == "yyb" {
		var err error
		snapshots, err = a.listHostSnapshots()
		if err != nil {
			a.lastErr = err
			return guard.HostLaunchResult{Owner: "main", Platform: platform, Status: "launch_failed", Reason: err.Error()}
		}
	}
	request, err := guard.ResolveLaunchRequestForPlatform(platform, snapshots)
	if err != nil {
		a.lastErr = err
		return guard.HostLaunchResult{Owner: "main", Platform: platform, Status: "launch_failed", Reason: err.Error()}
	}
	if err := a.launchHost(request); err != nil {
		a.lastErr = err
		return guard.HostLaunchResult{Owner: "main", Platform: platform, Status: "launch_failed", Reason: err.Error()}
	}
	return guard.HostLaunchResult{Owner: "main", Platform: platform, Status: "launch_dispatched", Reason: "launch was dispatched", LaunchDispatched: true}
}

func (a *App) runtimeSettingsFromConfig() storage.RuntimeSettings {
	settings := storage.RuntimeSettings{
		DefaultTarget: a.cfg.Runtime.DefaultTarget,
		CurrentTarget: a.cfg.Runtime.CurrentTarget,
		AutoStart:     a.cfg.Runtime.AutoStart,
		CDPPort:       a.cfg.CDP.Port,
		WMPFDebugPort: a.cfg.WMPF.DebugPort,
	}
	if a.guardian != nil {
		guardSettings := a.guardian.Settings()
		settings.ProcessGuardEnabled = guardSettings.Enabled
		settings.ProcessGuardFailureRecoveryEnabled = guardSettings.FailureRecoveryEnabled
		settings.ProcessGuardTimeoutThreshold = guardSettings.TimeoutThreshold
		settings.ProcessGuardMonitorIntervalMS = guardSettings.MonitorIntervalMS
		settings.ProcessGuardRestartReconnectGraceSec = guardSettings.RestartReconnectGraceSec
		settings.ProcessGuardMaxRestartsPer10Min = guardSettings.MaxRestartsPer10Min
		settings.ProcessGuardScheduledRestartEnabled = guardSettings.ScheduledRestartEnabled
		settings.ProcessGuardScheduledRestartIntervalMin = guardSettings.ScheduledRestartIntervalMin
		settings.ProcessGuardAutoMinimizeAfterRestart = guardSettings.AutoMinimizeAfterRestart
		settings.NetworkReconnectEnabled = guardSettings.NetworkReconnectEnabled
		settings.NetworkReconnectIntervalMS = guardSettings.NetworkReconnectIntervalMS
		settings.NetworkReconnectRecoveryTimeoutMS = guardSettings.NetworkRecoveryTimeoutMS
		settings.OtherPlaceLoginReconnectEnabled = guardSettings.OtherPlaceLoginEnabled
		settings.OtherPlaceLoginCheckIntervalMS = guardSettings.OtherPlaceLoginIntervalMS
		settings.OtherPlaceLoginReconnectDelayMin = guardSettings.OtherPlaceLoginDelayMin
	}
	return settings
}

func (a *App) normalizeRuntimeSettings(settings storage.RuntimeSettings) storage.RuntimeSettings {
	defaults := a.runtimeSettingsFromConfig()
	if settings.DefaultTarget == "" {
		settings.DefaultTarget = defaults.DefaultTarget
	}
	if settings.CurrentTarget == "" {
		settings.CurrentTarget = defaults.CurrentTarget
	}
	if settings.CDPPort <= 0 {
		settings.CDPPort = defaults.CDPPort
	}
	if settings.WMPFDebugPort <= 0 {
		settings.WMPFDebugPort = defaults.WMPFDebugPort
	}
	settings = normalizeGuardRuntimeSettings(settings)
	settings = storage.NormalizeWarehouseRuntimeSettings(settings)
	return settings
}

func (a *App) applyRuntimeSettings(settings storage.RuntimeSettings) {
	settings = a.normalizeRuntimeSettings(settings)
	a.cfg.Runtime.DefaultTarget = settings.DefaultTarget
	a.cfg.Runtime.CurrentTarget = settings.CurrentTarget
	a.cfg.Runtime.AutoStart = settings.AutoStart
	a.cfg.CDP.Port = settings.CDPPort
	a.cfg.WMPF.DebugPort = settings.WMPFDebugPort
	if a.guardian != nil {
		a.guardian.UpdateSettings(guardSettingsFromRuntimeSettings(settings))
	}
}

func (a *App) rebuildCDPLinks() {
	if a.supervisor == nil {
		return
	}
	wechatLink := wmpf.NewCDPLink(
		wmpf.ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP),
		newCDPLinkConfig(a.cfg, a.fridaPython),
		a.manager,
	)
	wechatLink.OnRuntimeEvent(func(event map[string]any) {
		a.handleGuardianRuntimeEventPayload(string(farmruntime.RuntimeTargetWeChatCDP), event)
	})
	wechatLink.OnRuntimeEvent(func(event map[string]any) {
		a.handleCDPHostLogEvent(string(farmruntime.RuntimeTargetWeChatCDP), event)
	})
	a.supervisor.Register(farmruntime.RuntimeTargetWeChatCDP, wechatLink)
	yybLink := wmpf.NewCDPLink(
		wmpf.ProfileForTarget(farmruntime.RuntimeTargetYYBCDP),
		newCDPLinkConfig(a.cfg, a.fridaPython),
		a.manager,
	)
	yybLink.OnRuntimeEvent(func(event map[string]any) {
		a.handleGuardianRuntimeEventPayload(string(farmruntime.RuntimeTargetYYBCDP), event)
	})
	yybLink.OnRuntimeEvent(func(event map[string]any) {
		a.handleCDPHostLogEvent(string(farmruntime.RuntimeTargetYYBCDP), event)
	})
	a.supervisor.Register(farmruntime.RuntimeTargetYYBCDP, yybLink)
}

func (a *App) runQQDebugPatch(trigger string, targetPath string) qqpatch.Result {
	startedAt := time.Now().Format(time.RFC3339)
	a.patchMu.Lock()
	a.patchStatus = QQDebugPatchStatus{
		Phase:       "scanning",
		InFlight:    true,
		LastTrigger: trigger,
		StartedAt:   startedAt,
		HostVersion: a.cfg.QQWS.ExpectedHostVersion,
	}
	a.patchMu.Unlock()
	a.recordEvent(eventbus.Event{
		Level:   eventbus.LevelInfo,
		Source:  "qq_ws",
		Type:    "qq.patch",
		Message: "QQ debug patch started",
		Data:    map[string]any{"trigger": trigger, "targetPath": targetPath},
	})

	result := qqpatch.Install(qqpatch.Options{
		TargetPath:   targetPath,
		ButtonScript: runtimeButtonScript,
		HostScript:   qqHostScript,
		HostVersion:  a.cfg.QQWS.ExpectedHostVersion,
	})

	finishedAt := time.Now().Format(time.RFC3339)
	phase := "error"
	if result.OK {
		phase = "ready"
	}
	a.patchMu.Lock()
	a.patchStatus = QQDebugPatchStatus{
		Phase:           phase,
		Injected:        result.OK,
		InFlight:        false,
		LastTrigger:     trigger,
		Action:          result.Action,
		TargetPath:      result.TargetPath,
		BackupPath:      result.BackupPath,
		ScriptHash:      result.ScriptHash,
		HostVersion:     result.HostVersion,
		CandidatePaths:  result.CandidatePaths,
		RestartRequired: result.RestartRequired,
		Error:           result.Error,
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
	}
	a.patchMu.Unlock()
	level := eventbus.LevelInfo
	message := "QQ debug patch ready"
	if !result.OK {
		level = eventbus.LevelError
		message = "QQ debug patch failed"
	}
	a.recordEvent(eventbus.Event{
		Level:   level,
		Source:  "qq_ws",
		Type:    "qq.patch",
		Message: message,
		Data: map[string]any{
			"trigger":         trigger,
			"action":          result.Action,
			"targetPath":      result.TargetPath,
			"backupPath":      result.BackupPath,
			"scriptHash":      result.ScriptHash,
			"hostVersion":     result.HostVersion,
			"candidatePath":   result.CandidatePaths,
			"restartRequired": result.RestartRequired,
			"error":           result.Error,
		},
	})
	return result
}

func (a *App) recordRuntimeStatus(previous farmruntime.Status, next farmruntime.Status) {
	if previous.Target == next.Target &&
		previous.Phase == next.Phase &&
		previous.Connected == next.Connected &&
		previous.Ready == next.Ready &&
		previous.ProgressDetail == next.ProgressDetail &&
		previous.LastError == next.LastError {
		return
	}

	level := eventbus.LevelInfo
	if next.Phase == farmruntime.PhaseError {
		level = eventbus.LevelError
	} else if next.Phase == farmruntime.PhaseDisconnected {
		level = eventbus.LevelWarn
	}

	data := map[string]any{
		"target":         next.Target,
		"phase":          string(next.Phase),
		"connected":      next.Connected,
		"ready":          next.Ready,
		"instanceId":     next.InstanceID,
		"hostVersion":    next.HostVersion,
		"lastSeenAt":     next.LastSeenAt,
		"progressDetail": next.ProgressDetail,
		"lastError":      next.LastError,
	}
	if (next.Target == string(farmruntime.RuntimeTargetWeChatCDP) || next.Target == string(farmruntime.RuntimeTargetYYBCDP)) &&
		next.Phase == farmruntime.PhaseListening &&
		!next.Connected {
		data["hint"] = "Bridge is listening, but no miniapp debug client has connected yet."
	}

	a.recordEvent(eventbus.Event{
		Level:   level,
		Source:  next.Target,
		Type:    "runtime.status",
		Message: next.Target + " " + string(next.Phase),
		Data:    data,
	})
	a.recordProcessGuardRuntimeStatus(next)
}

func (a *App) recordProcessGuardRuntimeStatus(status farmruntime.Status) {
	if status.Ready && status.Connected {
		a.consumePendingHostMinimize(status.Target)
	}
	if a.guardian == nil {
		return
	}
	a.guardian.OnRuntimeStatus(guardianSnapshotFromRuntimeStatus(status))
}

func guardianSnapshotFromRuntimeStatus(status farmruntime.Status) guard.RuntimeSnapshot {
	return guard.RuntimeSnapshot{
		RuntimeTarget: status.Target,
		Ready:         status.Ready,
		Connected:     status.Connected,
	}
}

func (a *App) handleGuardianRuntimeEventPayload(runtimeTarget string, payload map[string]any) {
	if a.guardian == nil {
		return
	}
	name := stringFromAny(payload["name"])
	if name != "network_reconnect" && name != "other_place_login_reconnect" {
		return
	}
	event := guard.RuntimeEvent{
		Name:            name,
		Phase:           stringFromAny(payload["phase"]),
		RuntimeTarget:   firstNonEmptyString(stringFromAny(payload["runtimeTarget"]), runtimeTarget),
		AccountKey:      stringFromAny(payload["accountKey"]),
		GID:             stringFromAny(payload["gid"]),
		Handled:         boolFromAny(payload["handled"], false),
		Via:             stringFromAny(payload["via"]),
		FirstDetectedAt: int64(intFromAny(payload["firstDetectedAt"], 0)),
		HandledAt:       int64(intFromAny(payload["handledAt"], 0)),
		RemainingMS:     int64(intFromAny(payload["remainingMs"], 0)),
		Error:           stringFromAny(payload["error"]),
	}
	if event.Phase == "" {
		event.Phase = "detected"
	}
	a.guardian.HandleRuntimeEvent(event)
}

// handleCDPHostLogEvent forwards tsdkBlock events emitted by TSDK-BLOCK (inside button.js)
// back to the log center via recordEvent. Events arrive through the CDP Runtime.bindingCalled
// → __qqFarmRuntimeEventBridge bridge and are dispatched here via a separate OnRuntimeEvent
// subscription alongside the guardian handler.
func (a *App) handleCDPHostLogEvent(runtimeTarget string, payload map[string]any) {
	if stringFromAny(payload["name"]) != "tsdkBlock" {
		return
	}
	level := stringFromAny(payload["level"])
	if level == "" {
		level = "info"
	}
	message := stringFromAny(payload["message"])
	if message == "" {
		return
	}
	var data map[string]any
	if extra, ok := payload["extra"].(map[string]any); ok {
		data = extra
	}
	a.recordRuntimeLogEvent(eventbus.Event{
		Level:   eventbus.Level(level),
		Source:  runtimeTarget,
		Type:    "qqhost.log",
		Message: message,
		Data:    data,
	})
}

func (a *App) handleGuardianRuntimeEvent(event guard.RuntimeEvent) {
	worker := "runtime"
	switch event.Name {
	case "network_reconnect":
		worker = "network"
	case "other_place_login_reconnect":
		worker = "other_place_login"
	}
	phase := event.Phase
	if phase == "" {
		phase = "detected"
	}
	level := eventbus.LevelInfo
	if event.Error != "" {
		level = eventbus.LevelWarn
	}
	a.recordEventForAccount(firstNonEmptyString(event.AccountKey, a.accountKey()), eventbus.Event{
		Level:   level,
		Source:  "guardian",
		Type:    "guardian." + worker + "." + phase,
		Message: event.Name + " " + phase,
		Data: map[string]any{
			"runtimeTarget":   event.RuntimeTarget,
			"gid":             event.GID,
			"handled":         event.Handled,
			"via":             event.Via,
			"firstDetectedAt": event.FirstDetectedAt,
			"handledAt":       event.HandledAt,
			"remainingMs":     event.RemainingMS,
			"error":           event.Error,
		},
	})
}

func (a *App) setGuardianDispatchPaused(paused bool) {
	a.guardianPauseMu.Lock()
	a.guardianDispatchPaused = paused
	a.guardianPauseMu.Unlock()
}

func (a *App) guardianDispatchIsPaused() bool {
	a.guardianPauseMu.RLock()
	defer a.guardianPauseMu.RUnlock()
	return a.guardianDispatchPaused
}

func (a *App) restartHostForRuntimeTarget(runtimeTarget string) (guard.RestartResult, error) {
	target := farmruntime.RuntimeTarget(runtimeTarget)
	if target == farmruntime.RuntimeTargetQQWS {
		snapshots, err := a.listHostSnapshots()
		if err != nil {
			return guard.RestartResult{}, err
		}
		settings := a.guard.Settings()
		current := a.manager.Status()
		result, restartErr := guard.RestartQQMiniapp(guard.QQRestartRequest{
			Registry:          a.hosts,
			Owner:             "main",
			Snapshots:         snapshots,
			RuntimeConnected:  current.Target == string(farmruntime.RuntimeTargetQQWS) && current.Connected,
			RuntimeInstanceID: current.InstanceID,
			CloseWindows:      a.closeHostWindows,
			WaitForClosed:     a.waitForQQMiniappClosed,
			StopPID:           a.stopHostPID,
			Launch:            a.launchHost,
			WaitForReady: func(previous string, timeout time.Duration) guard.QQReadyObservation {
				accepted := a.waitForRuntimeStatus(timeout, func(status farmruntime.Status) bool {
					return status.Target == string(farmruntime.RuntimeTargetQQWS) &&
						status.Connected && status.Ready && status.InstanceID != "" && status.InstanceID != previous
				})
				status := a.manager.Status()
				return guard.QQReadyObservation{
					Accepted:   accepted,
					Target:     status.Target,
					Connected:  status.Connected,
					Ready:      status.Ready,
					InstanceID: status.InstanceID,
				}
			},
			RefreshSnapshots: a.listHostSnapshots,
			CloseTimeout:     guard.DefaultQQMiniappCloseTimeout,
			ReconnectTimeout: time.Duration(settings.RestartReconnectGraceSec) * time.Second,
		})
		if restartErr == nil && result.Status == guard.RestartStatusReconnected && result.Binding != nil &&
			settings.AutoMinimizeAfterRestart && a.minimizeHostWindows != nil {
			windows := make([]guard.HostWindowSnapshot, 0, len(result.Binding.HWNDs))
			for index, hwnd := range result.Binding.HWNDs {
				if hwnd == 0 {
					continue
				}
				title := ""
				if index < len(result.Binding.WindowTitles) {
					title = result.Binding.WindowTitles[index]
				}
				windows = append(windows, guard.HostWindowSnapshot{
					HWND: hwnd, PID: result.Binding.PID, Title: title, Visible: true,
				})
			}
			if err := a.minimizeHostWindows(windows); err != nil {
				a.recordEvent(eventbus.Event{
					Level:   eventbus.LevelError,
					Source:  "guardian",
					Type:    "guardian.restart_window_minimize",
					Message: "restart window minimize failed",
					Data: map[string]any{
						"runtimeTarget": runtimeTarget,
						"pid":           result.Binding.PID,
						"error":         err.Error(),
					},
				})
			} else {
				a.recordEvent(eventbus.Event{
					Level:   eventbus.LevelInfo,
					Source:  "guardian",
					Type:    "guardian.restart_window_minimize",
					Message: "restarted host windows minimized",
					Data: map[string]any{
						"runtimeTarget": runtimeTarget,
						"pid":           result.Binding.PID,
						"windowCount":   len(windows),
					},
				})
			}
		}
		return result, restartErr
	}

	a.AutoBindHostProcess()
	snapshots, err := a.listHostSnapshots()
	if err != nil {
		return guard.RestartResult{}, err
	}
	if target == farmruntime.RuntimeTargetWeChatCDP || target == farmruntime.RuntimeTargetYYBCDP {
		settings := a.guard.Settings()
		targetName := string(target)
		request := guard.WMPFRestartRequest{
			Registry:     a.hosts,
			Owner:        "main",
			Snapshots:    snapshots,
			CloseWindows: a.closeHostWindows,
			WaitForDisconnected: func(timeout time.Duration) bool {
				return a.waitForRuntimeStatus(timeout, func(status farmruntime.Status) bool {
					return status.Target == targetName && !status.Connected
				})
			},
			Launch: a.launchHost,
			WaitForReady: func(timeout time.Duration) bool {
				return a.waitForRuntimeStatus(timeout, func(status farmruntime.Status) bool {
					return status.Target == targetName && status.Connected && status.Ready
				})
			},
			RefreshSnapshots: a.listHostSnapshots,
			CloseTimeout:     a.restartLaunchDelay,
			ReconnectTimeout: time.Duration(settings.RestartReconnectGraceSec) * time.Second,
		}
		restart := guard.RestartWeChatMiniapp
		if target == farmruntime.RuntimeTargetYYBCDP {
			restart = guard.RestartYYBMiniapp
		}
		result, restartErr := restart(request)
		if restartErr == nil && result.Status == guard.RestartStatusReconnected && settings.AutoMinimizeAfterRestart {
			a.queuePendingHostMinimize(runtimeTarget)
			a.consumePendingHostMinimize(runtimeTarget)
		}
		return result, restartErr
	}
	result, err := guard.RestartBoundHost(guard.RestartRequest{
		Registry:    a.hosts,
		Owner:       "main",
		Platform:    platformFromRuntimeTarget(runtimeTarget),
		Snapshots:   snapshots,
		StopPID:     a.stopHostPID,
		Launch:      a.launchHost,
		LaunchDelay: a.restartLaunchDelay,
		Sleep:       a.sleep,
	})
	if err == nil && result.Status == "launch_dispatched" && a.currentRuntimeSettingsForChangeDetection().ProcessGuardAutoMinimizeAfterRestart {
		a.queuePendingHostMinimize(runtimeTarget)
	}
	return result, err
}

func waitForRuntimeStatus(manager *farmruntime.Manager, timeout time.Duration, accept func(farmruntime.Status) bool) bool {
	if manager == nil || accept == nil {
		return false
	}
	if accept(manager.Status()) {
		return true
	}
	if timeout <= 0 {
		return false
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if accept(manager.Status()) {
				return true
			}
		case <-timer.C:
			return false
		}
	}
}

func waitForQQMiniappClosed(
	manager *farmruntime.Manager,
	listSnapshots func() ([]guard.HostProcessSnapshot, error),
	rootPID int,
	timeout time.Duration,
) (guard.QQCloseObservation, error) {
	observe := func() (guard.QQCloseObservation, error) {
		if manager == nil || listSnapshots == nil {
			return guard.QQCloseObservation{}, errors.New("QQ close observer dependencies are required")
		}
		snapshots, err := listSnapshots()
		if err != nil {
			return guard.QQCloseObservation{}, err
		}
		status := manager.Status()
		return guard.QQCloseObservation{
			Disconnected: status.Target == string(farmruntime.RuntimeTargetQQWS) && !status.Connected,
			TreeExited:   guard.QQMiniappTreeExited(rootPID, snapshots),
			Snapshots:    snapshots,
		}, nil
	}
	observation, err := observe()
	if err != nil || (observation.Disconnected && observation.TreeExited) || timeout <= 0 {
		return observation, err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			observation, err = observe()
			if err != nil || (observation.Disconnected && observation.TreeExited) {
				return observation, err
			}
		case <-timer.C:
			return observe()
		}
	}
}

func (a *App) queuePendingHostMinimize(runtimeTarget string) {
	runtimeTarget = strings.TrimSpace(runtimeTarget)
	if runtimeTarget == "" {
		return
	}
	a.pendingHostMinimizeMu.Lock()
	defer a.pendingHostMinimizeMu.Unlock()
	a.pendingHostMinimize = &pendingHostMinimize{runtimeTarget: runtimeTarget}
}

func (a *App) clearPendingHostMinimize() {
	a.pendingHostMinimizeMu.Lock()
	defer a.pendingHostMinimizeMu.Unlock()
	a.pendingHostMinimize = nil
}

func (a *App) takePendingHostMinimize(runtimeTarget string) *pendingHostMinimize {
	runtimeTarget = strings.TrimSpace(runtimeTarget)
	a.pendingHostMinimizeMu.Lock()
	defer a.pendingHostMinimizeMu.Unlock()
	if a.pendingHostMinimize == nil {
		return nil
	}
	if runtimeTarget != "" && a.pendingHostMinimize.runtimeTarget != runtimeTarget {
		return nil
	}
	pending := a.pendingHostMinimize
	a.pendingHostMinimize = nil
	return pending
}

func (a *App) consumePendingHostMinimize(runtimeTarget string) {
	pending := a.takePendingHostMinimize(runtimeTarget)
	if pending == nil {
		return
	}
	a.AutoBindHostProcess()
	if a.listHostSnapshots == nil || a.minimizeHostWindows == nil || a.hosts == nil {
		a.recordEvent(eventbus.Event{
			Level:   eventbus.LevelError,
			Source:  "guardian",
			Type:    "guardian.restart_window_minimize",
			Message: "restart window minimize skipped: dependencies unavailable",
			Data: map[string]any{
				"runtimeTarget": pending.runtimeTarget,
			},
		})
		return
	}
	snapshots, err := a.listHostSnapshots()
	if err != nil {
		a.recordEvent(eventbus.Event{
			Level:   eventbus.LevelError,
			Source:  "guardian",
			Type:    "guardian.restart_window_minimize",
			Message: "restart window minimize failed to list host windows",
			Data: map[string]any{
				"runtimeTarget": pending.runtimeTarget,
				"error":         err.Error(),
			},
		})
		return
	}
	binding, ok := a.hosts.Binding("main")
	if !ok || binding.PID <= 0 {
		a.recordEvent(eventbus.Event{
			Level:   eventbus.LevelError,
			Source:  "guardian",
			Type:    "guardian.restart_window_minimize",
			Message: "restart window minimize skipped: host not bound",
			Data: map[string]any{
				"runtimeTarget": pending.runtimeTarget,
			},
		})
		return
	}
	var windows []guard.HostWindowSnapshot
	for _, snapshot := range snapshots {
		if snapshot.PID != binding.PID {
			continue
		}
		for _, window := range snapshot.Windows {
			if window.Visible && window.HWND != 0 {
				windows = append(windows, window)
			}
		}
	}
	if err := a.minimizeHostWindows(windows); err != nil {
		a.recordEvent(eventbus.Event{
			Level:   eventbus.LevelError,
			Source:  "guardian",
			Type:    "guardian.restart_window_minimize",
			Message: "restart window minimize failed",
			Data: map[string]any{
				"runtimeTarget": pending.runtimeTarget,
				"pid":           binding.PID,
				"error":         err.Error(),
			},
		})
		return
	}
	a.recordEvent(eventbus.Event{
		Level:   eventbus.LevelInfo,
		Source:  "guardian",
		Type:    "guardian.restart_window_minimize",
		Message: "restarted host windows minimized",
		Data: map[string]any{
			"runtimeTarget": pending.runtimeTarget,
			"pid":           binding.PID,
			"windowCount":   len(windows),
		},
	})
}

func (a *App) setIdleRuntimeTarget(target string) {
	if target == "" {
		target = string(farmruntime.RuntimeTargetQQWS)
	}
	current := a.manager.Status()
	if current.Phase != farmruntime.PhaseIdle || current.Connected || current.Ready {
		return
	}
	a.manager.SetStatus(farmruntime.Status{
		Target: target,
		Phase:  farmruntime.PhaseIdle,
	})
}

func (a *App) guardPlatform() string {
	return platformFromRuntimeTarget(a.cfg.Runtime.CurrentTarget)
}

func platformFromRuntimeTarget(runtimeTarget string) string {
	switch farmruntime.RuntimeTarget(runtimeTarget) {
	case farmruntime.RuntimeTargetQQWS:
		return "qq"
	case farmruntime.RuntimeTargetYYBCDP:
		return "yyb"
	default:
		return "wechat_cdp"
	}
}

type runtimeStatusError string

func (e runtimeStatusError) Error() string {
	return string(e)
}

func normalizeGuardRuntimeSettings(settings storage.RuntimeSettings) storage.RuntimeSettings {
	defaults := guard.DefaultSettings()
	if settings.ProcessGuardTimeoutThreshold <= 0 {
		settings.ProcessGuardTimeoutThreshold = defaults.TimeoutThreshold
	}
	if settings.ProcessGuardMonitorIntervalMS <= 0 {
		settings.ProcessGuardMonitorIntervalMS = defaults.MonitorIntervalMS
	}
	if settings.ProcessGuardRestartReconnectGraceSec <= 0 {
		settings.ProcessGuardRestartReconnectGraceSec = defaults.RestartReconnectGraceSec
	}
	if settings.ProcessGuardMaxRestartsPer10Min <= 0 {
		settings.ProcessGuardMaxRestartsPer10Min = defaults.MaxRestartsPer10Min
	}
	if settings.ProcessGuardScheduledRestartIntervalMin <= 0 {
		settings.ProcessGuardScheduledRestartIntervalMin = defaults.ScheduledRestartIntervalMin
	}
	if settings.NetworkReconnectIntervalMS <= 0 {
		settings.NetworkReconnectIntervalMS = defaults.NetworkReconnectIntervalMS
	}
	if settings.NetworkReconnectRecoveryTimeoutMS <= 0 {
		settings.NetworkReconnectRecoveryTimeoutMS = defaults.NetworkRecoveryTimeoutMS
	}
	if settings.OtherPlaceLoginCheckIntervalMS <= 0 {
		settings.OtherPlaceLoginCheckIntervalMS = defaults.OtherPlaceLoginIntervalMS
	}
	if settings.OtherPlaceLoginReconnectDelayMin <= 0 {
		settings.OtherPlaceLoginReconnectDelayMin = defaults.OtherPlaceLoginDelayMin
	}
	return settings
}

func guardSettingsFromRuntimeSettings(settings storage.RuntimeSettings) guard.Settings {
	settings = normalizeGuardRuntimeSettings(settings)
	return guard.Settings{
		Enabled:                     settings.ProcessGuardEnabled,
		FailureRecoveryEnabled:      settings.ProcessGuardFailureRecoveryEnabled,
		TimeoutThreshold:            settings.ProcessGuardTimeoutThreshold,
		MonitorIntervalMS:           settings.ProcessGuardMonitorIntervalMS,
		RestartReconnectGraceSec:    settings.ProcessGuardRestartReconnectGraceSec,
		MaxRestartsPer10Min:         settings.ProcessGuardMaxRestartsPer10Min,
		ScheduledRestartEnabled:     settings.ProcessGuardScheduledRestartEnabled,
		ScheduledRestartIntervalMin: settings.ProcessGuardScheduledRestartIntervalMin,
		AutoMinimizeAfterRestart:    settings.ProcessGuardAutoMinimizeAfterRestart,
		NetworkReconnectEnabled:     settings.NetworkReconnectEnabled,
		NetworkReconnectIntervalMS:  settings.NetworkReconnectIntervalMS,
		NetworkRecoveryTimeoutMS:    settings.NetworkReconnectRecoveryTimeoutMS,
		OtherPlaceLoginEnabled:      settings.OtherPlaceLoginReconnectEnabled,
		OtherPlaceLoginIntervalMS:   settings.OtherPlaceLoginCheckIntervalMS,
		OtherPlaceLoginDelayMin:     settings.OtherPlaceLoginReconnectDelayMin,
	}
}

func processGuardSettingsMap(settings storage.RuntimeSettings) map[string]any {
	settings = normalizeGuardRuntimeSettings(settings)
	return map[string]any{
		"enabled":                     settings.ProcessGuardEnabled,
		"timeoutThreshold":            settings.ProcessGuardTimeoutThreshold,
		"monitorIntervalMs":           settings.ProcessGuardMonitorIntervalMS,
		"restartReconnectGraceSec":    settings.ProcessGuardRestartReconnectGraceSec,
		"maxRestartsPer10Min":         settings.ProcessGuardMaxRestartsPer10Min,
		"scheduledRestartEnabled":     settings.ProcessGuardScheduledRestartEnabled,
		"scheduledRestartIntervalMin": settings.ProcessGuardScheduledRestartIntervalMin,
		"autoMinimizeAfterRestart":    settings.ProcessGuardAutoMinimizeAfterRestart,
	}
}

func automationSettingsFromStorage(settings storage.AutoFarmSettings) automation.Settings {
	tasks := make([]automation.TaskSettings, 0, len(settings.Tasks))
	for _, task := range settings.Tasks {
		tasks = append(tasks, automation.TaskSettings{
			ID:          task.ID,
			Enabled:     task.Enabled,
			Priority:    task.Priority,
			IntervalSec: task.IntervalSec,
		})
	}
	return automation.Settings{
		SchedulerEnabled:  settings.SchedulerEnabled,
		SchedulerMinGapMs: settings.SchedulerMinGapMS,
		RunMode:           automation.RunMode(settings.RunMode),
		Tasks:             tasks,
		Config:            settings.Config,
	}
}

func storageAutoFarmSettings(settings automation.Settings) storage.AutoFarmSettings {
	tasks := make([]storage.AutoFarmTaskSettings, 0, len(settings.Tasks))
	for _, task := range settings.Tasks {
		tasks = append(tasks, storage.AutoFarmTaskSettings{
			ID:          task.ID,
			Enabled:     task.Enabled,
			Priority:    task.Priority,
			IntervalSec: task.IntervalSec,
		})
	}
	return storage.AutoFarmSettings{
		SchedulerEnabled:  settings.SchedulerEnabled,
		SchedulerMinGapMS: settings.SchedulerMinGapMs,
		RunMode:           string(settings.RunMode),
		Tasks:             tasks,
		Config:            settings.Config,
	}
}

func normalizeAutomationTaskTrigger(trigger string) string {
	trigger = strings.ToLower(strings.TrimSpace(trigger))
	switch trigger {
	case "auto", "scheduler":
		return "auto"
	default:
		return "manual"
	}
}

func (a *App) ensureMessagePushService() (*messagepush.Service, string) {
	a.accountMu.RLock()
	accountKey := a.accountKeyLocked()
	accountGID := ""
	if a.currentAccount.AccountKey == accountKey && a.currentAccount.GID > 0 {
		accountGID = strconv.Itoa(a.currentAccount.GID)
	}
	a.accountMu.RUnlock()
	return a.messagePushServiceForAccount(accountKey, accountGID), accountKey
}

func (a *App) messagePushServiceForAccount(accountKey string, accountGID string) *messagepush.Service {
	accountKey = storage.NormalizeAccountKey(accountKey)
	a.messagePushMu.Lock()
	defer a.messagePushMu.Unlock()
	if a.messagePush != nil && a.messagePushAccountKey == accountKey {
		return a.messagePush
	}
	var store messagepush.Store
	if a.store != nil {
		store = accountMessagePushStore{store: a.store, accountKey: accountKey}
	}
	a.messagePush = messagepush.NewService(messagepush.ServiceOptions{
		Store:      store,
		HTTPClient: a.messagePushHTTPClient,
		AccountKey: accountKey,
		AccountGID: accountGID,
		DailyReportProvider: func(ctx context.Context, now time.Time) (messagepush.DailyReport, error) {
			return a.dailyReportForAccount(ctx, accountKey, accountGID, now)
		},
		FallbackDiagnostic: func(_ string, messageType messagepush.MessageType, channelType string, err error) {
			a.recordMessagePushEventForAccount(accountKey, eventbus.LevelWarn, "message_push.template_fallback", "message push template fallback used", map[string]any{
				"type":    messageType,
				"channel": channelType,
				"error":   err.Error(),
			})
		},
	})
	a.messagePushAccountKey = accountKey
	return a.messagePush
}

func (a *App) dailyReportForAccount(ctx context.Context, accountKey string, expectedGID string, now time.Time) (messagepush.DailyReport, error) {
	profileValue, err := a.farmRuntimeCaller().Call(ctx, "gameCtl.getPlayerProfile", []any{map[string]any{"silent": true, "noCache": true}}, 8*time.Second)
	if err != nil {
		return messagepush.DailyReport{}, fmt.Errorf("读取运行账户资料失败: %w", sanitizeAccountRuntimeError(err))
	}
	profile := farm.BuildRuntimeAccountProfile(mapFromAny(profileValue))
	if profile.GID <= 0 {
		return messagepush.DailyReport{}, errors.New("运行账户资料未返回有效 GID")
	}
	actualGID := strconv.FormatInt(profile.GID, 10)
	if expectedGID != "" && expectedGID != actualGID {
		return messagepush.DailyReport{}, fmt.Errorf("运行账户 GID 不匹配: 当前 %s，日报账户 %s", actualGID, expectedGID)
	}

	warehouseValue, err := a.farmRuntimeCaller().Call(ctx, "gameCtl.refreshWarehouseSnapshot", []any{map[string]any{
		"silent": true, "preferProtocol": true, "protocolWaitMs": 1200, "closeAfter": true,
		"openTimeoutMs": 2600, "readTimeoutMs": 3200, "allowEmpty": true,
	}}, 15*time.Second)
	if err != nil {
		return messagepush.DailyReport{}, fmt.Errorf("读取仓库快照失败: %w", err)
	}
	warehouseRaw := mapFromAny(warehouseValue)
	if !boolFromAny(warehouseRaw["ok"], true) {
		return messagepush.DailyReport{}, fmt.Errorf("读取仓库快照失败: %s", stringFromAny(firstExistingAny(warehouseRaw["error"], warehouseRaw["reason"])))
	}
	itemMap, _ := farm.LoadItemInfoMap(farm.DefaultGameConfigRoot())
	warehouse := farm.BuildRuntimeWarehouse(warehouseRaw, itemMap)

	reportDateKey := messagepush.ShiftDateKey(now, -1)
	stats := farm.DayStats{DateKey: reportDateKey}
	if history := a.automationStatsHistoryForAccount(ctx, accountKey, now, 2); len(history.Days) > 0 {
		for _, day := range history.Days {
			if day.DateKey == reportDateKey {
				stats = day
				break
			}
		}
	}
	sellCount, sellAmount := 0, 0
	if a.store != nil {
		records, err := a.store.ListWarehouseSellRecords(ctx, accountKey, reportDateKey, 500)
		if err != nil {
			return messagepush.DailyReport{}, fmt.Errorf("读取日报出售记录失败: %w", err)
		}
		for _, record := range records {
			sellCount += record.TotalCount
			sellAmount += record.TotalAmount
		}
	}
	name := profile.Name
	if name == "" {
		name = profile.Nick
	}
	values := map[string]any{
		"name": name, "level": profile.Level, "gold": profile.Gold, "bean": profile.Bean,
		"warehouseEstimate": warehouse.Summary.EstimatedAllSellPrice, "warehouseSellableCount": warehouse.Summary.SellableCount,
		"sellCount": sellCount, "sellAmount": sellAmount, "runs": stats.Runs, "collect": stats.Collect,
		"water": stats.Water, "steal": stats.Steal, "help": stats.Help,
		"mischiefGrass": stats.MischiefGrass, "mischiefBug": stats.MischiefBug,
	}
	summary := fmt.Sprintf("金币 %d，金豆 %d；%s 出售 %d 件，获得 %d 金币；仓库可售估值 %d 金币。", profile.Gold, profile.Bean, reportDateKey, sellCount, sellAmount, warehouse.Summary.EstimatedAllSellPrice)
	return messagepush.DailyReport{AccountGID: actualGID, DateKey: reportDateKey, Summary: summary, Values: values}, nil
}

func (a *App) startMessagePushDailyScheduler(parent context.Context) {
	a.messagePushDailyMu.Lock()
	defer a.messagePushDailyMu.Unlock()
	if a.messagePushCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	a.messagePushCancel = cancel
	interval := a.messagePushDailyInterval
	if interval <= 0 {
		interval = time.Minute
	}
	a.messagePushDailyWG.Add(1)

	go func() {
		defer a.messagePushDailyWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.enqueueMessagePushEventForAccount(a.accountKey(), messagepush.MessageEvent{Type: messagepush.MessageTypeDaily})
			}
		}
	}()
}

func (a *App) stopMessagePushDailyScheduler() {
	a.messagePushDailyMu.Lock()
	defer a.messagePushDailyMu.Unlock()
	cancel := a.messagePushCancel
	a.messagePushCancel = nil
	if cancel != nil {
		cancel()
		a.messagePushDailyWG.Wait()
	}
}

func (a *App) startMessagePushDeliveryWorker(parent context.Context) {
	a.messagePushWorkerMu.Lock()
	defer a.messagePushWorkerMu.Unlock()
	if a.messagePushWorkerCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	a.messagePushWorkerCancel = cancel
	a.messagePushWorkerWG.Add(1)
	go func() {
		defer a.messagePushWorkerWG.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case audit := <-a.messagePushAuditQueue:
				a.recordEventForAccount(audit.accountKey, audit.event)
			case work := <-a.messagePushQueue:
				if ctx.Err() != nil {
					return
				}
				a.dispatchMessagePushEventForAccount(ctx, work.accountKey, work.event)
			}
		}
	}()
}

func (a *App) stopMessagePushDeliveryWorker() {
	a.messagePushWorkerMu.Lock()
	cancel := a.messagePushWorkerCancel
	a.messagePushWorkerCancel = nil
	a.messagePushWorkerMu.Unlock()
	if cancel != nil {
		cancel()
		a.messagePushWorkerWG.Wait()
	}
}

func (a *App) enqueueMessagePushEventForAccount(accountKey string, event messagepush.MessageEvent) {
	accountKey = storage.NormalizeAccountKey(accountKey)
	event.AccountKey = accountKey
	select {
	case a.messagePushQueue <- messagePushWork{accountKey: accountKey, event: event}:
	default:
		a.enqueueMessagePushAudit(messagePushAudit{accountKey: accountKey, event: eventbus.Event{
			Level:   eventbus.LevelWarn,
			Source:  "message_push",
			Type:    "message_push.queue_dropped",
			Message: "message push delivery queue is full",
			Data:    map[string]any{"type": event.Type},
		}})
	}
}

func (a *App) enqueueMessagePushAudit(audit messagePushAudit) {
	select {
	case a.messagePushAuditQueue <- audit:
	default:
		slog.Warn("message push audit queue is full", "accountKey", audit.accountKey, "eventType", audit.event.Type)
	}
}

func (a *App) enqueueMessagePushLifecycleEventForAccount(accountKey string, lifecycle guard.LifecycleEvent) {
	event, ok := messagePushEventFromLifecycle(lifecycle)
	if !ok {
		return
	}
	a.enqueueMessagePushEventForAccount(accountKey, event)
}

func messagePushEventFromLifecycle(lifecycle guard.LifecycleEvent) (messagepush.MessageEvent, bool) {
	occurredAt := time.Now()
	if parsed, err := time.Parse(time.RFC3339, lifecycle.At); err == nil {
		occurredAt = parsed
	}
	event := messagepush.MessageEvent{
		AccountKey:    "",
		RuntimeTarget: lifecycle.RuntimeTarget,
		Error:         lifecycle.Error,
		Trigger:       lifecycle.Trigger,
		OccurredAt:    occurredAt,
	}
	switch lifecycle.Kind {
	case guard.LifecycleSuspected:
		event.Type = messagepush.MessageTypeSuspected
		event.Values = map[string]any{"monitor": map[string]any{"streak": lifecycle.Streak, "threshold": lifecycle.Threshold}}
	case guard.LifecycleAbnormal:
		event.Type = messagepush.MessageTypeAbnormal
	case guard.LifecycleRecovery:
		event.Type = messagepush.MessageTypeRecovery
		event.RecoveryVia = "guardian"
		event.Values = map[string]any{"recovery": map[string]any{
			"duration": (time.Duration(lifecycle.DurationMS) * time.Millisecond).String(),
			"via":      "guardian",
		}}
	case guard.LifecycleRestartCompleted:
		event.Type = messagepush.MessageTypeRestart
		event.Values = map[string]any{"restart": map[string]any{
			"trigger": lifecycle.Trigger,
			"reason":  lifecycle.Result.Reason,
			"ok":      lifecycle.Error == "",
			"error":   lifecycle.Error,
		}}
	default:
		return messagepush.MessageEvent{}, false
	}
	return event, true
}

func (a *App) dispatchMessagePushEventForAccount(ctx context.Context, accountKey string, event messagepush.MessageEvent) {
	accountKey = storage.NormalizeAccountKey(accountKey)
	event.AccountKey = accountKey
	service := a.messagePushServiceForAccount(accountKey, strings.TrimPrefix(accountKey, "gid:"))
	if event.Type == messagepush.MessageTypeDaily {
		result, err := service.RunDueDaily(ctx)
		if err != nil || result.OK {
			a.recordMessagePushSendForAccount(accountKey, "message_push.daily_due", "message push daily check completed", result, err)
		}
		return
	}
	result, err := service.Dispatch(ctx, event)
	if err != nil {
		a.recordMessagePushEventForAccount(accountKey, eventbus.LevelWarn, "message_push.failed", "message push dispatch failed", map[string]any{"type": event.Type, "error": err.Error()})
		return
	}
	if result.Sent {
		a.recordMessagePushEventForAccount(accountKey, eventbus.LevelInfo, "message_push.sent", "message push sent", map[string]any{"type": event.Type, "summary": result.Send.Summary})
		return
	}
	a.recordMessagePushEventForAccount(accountKey, eventbus.LevelInfo, "message_push.suppressed", "message push dispatch suppressed", map[string]any{"type": event.Type, "count": result.SuppressedCount})
}

func (a *App) recordMessagePushSendForAccount(accountKey string, eventType string, message string, result messagepush.SendResult, err error) {
	level := eventbus.LevelInfo
	data := map[string]any{
		"ok":      result.OK,
		"summary": result.Summary,
	}
	if len(result.Results) > 0 {
		data["results"] = result.Results
	}
	if err != nil {
		level = eventbus.LevelWarn
		data["error"] = err.Error()
		a.lastErr = err
	}
	a.recordMessagePushEventForAccount(accountKey, level, eventType, message, data)
}

func (a *App) recordMessagePushEventForAccount(accountKey string, level eventbus.Level, eventType string, message string, data map[string]any) {
	a.recordEventForAccount(accountKey, eventbus.Event{
		Level:   level,
		Source:  "message_push",
		Type:    eventType,
		Message: message,
		Data:    data,
	})
}

type accountMessagePushStore struct {
	store      *storage.Store
	accountKey string
}

func (s accountMessagePushStore) LoadMessagePushConfigJSON(ctx context.Context) (string, error) {
	return s.store.LoadMessagePushConfigJSONForAccount(ctx, s.accountKey)
}

func (s accountMessagePushStore) SaveMessagePushConfigJSON(ctx context.Context, raw string) error {
	return s.store.SaveMessagePushConfigJSONForAccount(ctx, s.accountKey, raw)
}

func (s accountMessagePushStore) LoadMessagePushStateJSON(ctx context.Context) (string, error) {
	return s.store.LoadMessagePushStateJSONForAccount(ctx, s.accountKey)
}

func (s accountMessagePushStore) SaveMessagePushStateJSON(ctx context.Context, raw string) error {
	return s.store.SaveMessagePushStateJSONForAccount(ctx, s.accountKey, raw)
}

func boolFromAny(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(typed)
		if err == nil {
			return parsed
		}
	case float64:
		return typed != 0
	case int:
		return typed != 0
	}
	return fallback
}

func intFromAny(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(typed)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func buildLandRushRuntimeArgs(input map[string]any) (map[string]any, error) {
	if input == nil {
		input = map[string]any{}
	}
	landIDs := normalizePositiveUniqueInts(firstExistingAny(
		input["landIds"],
		input["landIdList"],
		input["landId"],
	))
	if len(landIDs) == 0 {
		return nil, errors.New("请选择至少 1 块需要催熟的地块")
	}

	mode, err := normalizeFertilizerMode(firstNonEmptyString(
		stringFromAny(input["fertilizerMode"]),
		stringFromAny(input["mode"]),
		stringFromAny(input["type"]),
	), "organic")
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"landIds":                     landIDs,
		"type":                        mode,
		"mode":                        mode,
		"dryRun":                      false,
		"cleanupUi":                   true,
		"linkedHarvestAfterFertilize": boolFromAny(input["harvestLinkEnabled"], true),
		"rushThresholdSec":            intFromAny(input["rushThresholdSec"], 300),
	}, nil
}

func buildFertilizeLandRuntimeArgs(input map[string]any) (map[string]any, error) {
	if input == nil {
		input = map[string]any{}
	}
	landID := intFromAny(firstExistingAny(input["landId"], input["id"]), 0)
	if landID <= 0 {
		return nil, errors.New("请选择需要催熟的地块")
	}
	mode, err := normalizeFertilizerMode(firstNonEmptyString(
		stringFromAny(input["fertilizerMode"]),
		stringFromAny(input["mode"]),
		stringFromAny(input["type"]),
	), "auto")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"silent":           true,
		"dryRun":           false,
		"type":             mode,
		"mode":             mode,
		"internalFallback": true,
		"landId":           landID,
	}, nil
}

func buildShovelLandsRuntimeArgs(input map[string]any) (map[string]any, error) {
	if input == nil {
		input = map[string]any{}
	}
	landIDs := normalizePositiveUniqueInts(firstExistingAny(
		input["landIds"],
		input["landIdList"],
		input["landId"],
		input["id"],
	))
	if len(landIDs) == 0 {
		return nil, errors.New("请选择至少 1 块需要铲除的地块")
	}
	return map[string]any{
		"silent":          true,
		"dryRun":          false,
		"landIds":         landIDs,
		"waitAfterAction": intFromAny(input["waitAfterAction"], 250),
		"betweenLandWait": intFromAny(input["betweenLandWait"], 160),
	}, nil
}

func buildWarehouseSellRuntimeArgs(input map[string]any) (map[string]any, error) {
	if input == nil {
		input = map[string]any{}
	}
	itemIDs := normalizePositiveUniqueInts(firstExistingAny(
		input["itemIds"],
		input["itemIdList"],
		input["itemId"],
	))
	itemKeys := normalizeNonEmptyStrings(firstExistingAny(
		input["itemKeys"],
		input["itemKeyList"],
		input["itemKey"],
	))
	if len(itemIDs) == 0 && len(itemKeys) == 0 {
		return nil, errors.New("请选择至少 1 个仓库物品")
	}
	return map[string]any{
		"silent":          true,
		"preferProtocol":  true,
		"protocolWaitMs":  1200,
		"mode":            normalizeWarehouseSellRecordMode(stringFromAny(input["mode"])),
		"itemIds":         itemIDs,
		"itemKeys":        itemKeys,
		"closeAfter":      true,
		"forceCloseAfter": false,
		"openTimeoutMs":   2600,
		"readTimeoutMs":   2600,
		"sellTimeoutMs":   9000,
		"pollMs":          140,
	}, nil
}

func normalizeFertilizerMode(value string, fallback string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(value))
	if mode == "" {
		mode = fallback
	}
	if mode == "inorganic" {
		mode = "normal"
	}
	switch mode {
	case "auto", "normal", "organic":
		return mode, nil
	default:
		return "", errors.New("unsupported fertilizer mode: " + mode)
	}
}

func normalizePositiveUniqueInts(value any) []int {
	values := []any{}
	switch typed := value.(type) {
	case []any:
		values = typed
	case []int:
		for _, item := range typed {
			values = append(values, item)
		}
	case []float64:
		for _, item := range typed {
			values = append(values, item)
		}
	default:
		if value != nil {
			values = []any{value}
		}
	}

	result := []int{}
	seen := map[int]bool{}
	for _, item := range values {
		id := intFromAny(item, 0)
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

func normalizeNonEmptyStrings(value any) []string {
	values := []any{}
	switch typed := value.(type) {
	case []any:
		values = typed
	case []string:
		for _, item := range typed {
			values = append(values, item)
		}
	default:
		if value != nil {
			values = []any{value}
		}
	}

	result := []string{}
	seen := map[string]bool{}
	for _, item := range values {
		text := stringFromAny(item)
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		result = append(result, text)
	}
	return result
}

func firstExistingAny(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func stringFromAny(value any) string {
	if typed, ok := value.(string); ok {
		return strings.TrimSpace(typed)
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func sliceFromAny(value any) []any {
	if value == nil {
		return nil
	}
	if typed, ok := value.([]any); ok {
		return typed
	}
	return nil
}

func (a *App) recordEvent(event eventbus.Event) {
	a.recordEventForAccount(a.accountKey(), event)
}

func isTSDKBlockRuntimeEvent(event eventbus.Event) bool {
	return event.Type == "qqhost.log" && strings.Contains(event.Message, "[TSDK-BLOCK]")
}

func (a *App) recordRuntimeLogEvent(event eventbus.Event) {
	if isTSDKBlockRuntimeEvent(event) {
		a.recordEventForAccount(tsdkRuntimeAccountKey, event)
		return
	}
	a.recordEvent(event)
}

func (a *App) recordEventForAccount(accountKey string, event eventbus.Event) {
	accountKey = storage.NormalizeAccountKey(accountKey)
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.Level == "" {
		event.Level = eventbus.LevelInfo
	}
	if event.Source == "" {
		event.Source = "app"
	}
	if event.Type == "" {
		event.Type = "system"
	}

	a.eventsMu.Lock()
	a.nextEventID++
	if event.ID == 0 {
		event.ID = a.nextEventID
	}
	a.memoryEvents = append(a.memoryEvents, accountRuntimeEvent{accountKey: accountKey, event: event})
	if len(a.memoryEvents) > 500 {
		a.memoryEvents = a.memoryEvents[len(a.memoryEvents)-500:]
	}
	a.eventsMu.Unlock()

	if a.store != nil {
		if err := a.store.AppendRuntimeEventForAccount(a.contextOrBackground(), accountKey, event); err != nil {
			a.lastErr = err
		}
	}
	if strings.EqualFold(event.Source, "message_push") {
		return
	}
	if event.Level != eventbus.LevelWarn && event.Level != eventbus.LevelError {
		return
	}
	a.enqueueMessagePushEventForAccount(accountKey, messagepush.MessageEvent{
		Type:       messagepush.MessageTypeLogMonitor,
		OccurredAt: event.Timestamp,
		RuntimeLog: &messagepush.RuntimeLog{
			Level:   string(event.Level),
			Source:  event.Source,
			Type:    event.Type,
			Message: event.Message,
			Data:    event.Data,
		},
	})
}

func (a *App) accountKey() string {
	a.accountMu.RLock()
	defer a.accountMu.RUnlock()
	return a.accountKeyLocked()
}

func (a *App) accountKeyLocked() string {
	return storage.NormalizeAccountKey(a.currentAccountKey)
}

func (a *App) clearRuntimeAccountBinding(reason string) {
	a.accountMu.Lock()
	previousKey := storage.NormalizeAccountKey(a.currentAccountKey)
	previous := a.currentAccount
	a.currentAccountKey = storage.DefaultRuntimeAccountKey
	a.currentAccount = RuntimeAccount{}
	a.accountMu.Unlock()

	hadBinding := previous.Confirmed || previous.GID > 0 || (previousKey != "" && previousKey != storage.DefaultRuntimeAccountKey)
	if !hadBinding {
		return
	}
	a.resetAccountScopedServices()
	a.recordEventForAccount(previousKey, eventbus.Event{
		Level:   eventbus.LevelInfo,
		Source:  "account",
		Type:    "account.cleared",
		Message: "runtime account binding cleared",
		Data: map[string]any{
			"reason":           reason,
			"previousAccount":  previousKey,
			"previousGID":      previous.GID,
			"previousNickname": previous.Nickname,
		},
	})
}

func (a *App) resetAccountScopedServices() {
	if a.runtimeCache != nil {
		a.runtimeCache.Clear()
	}
	a.messagePushMu.Lock()
	a.messagePush = nil
	a.messagePushAccountKey = ""
	a.messagePushMu.Unlock()

	a.socialMu.Lock()
	a.dogGuardScanner = nil
	a.dogGuardStore = nil
	a.dogGuardAccountKey = ""
	a.socialMu.Unlock()

	a.automationSchedulerActionMu.Lock()
	a.automationSchedulerMu.Lock()
	if a.automationScheduler != nil {
		a.automationScheduler.Stop()
	}
	a.automationScheduler = nil
	a.automationSchedulerAccountKey = ""
	a.automationSchedulerMu.Unlock()
	a.automationSchedulerActionMu.Unlock()
}

type appSupervisorRuntimeCaller struct {
	app *App
}

func (c appSupervisorRuntimeCaller) Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error) {
	if c.app == nil || c.app.supervisor == nil {
		return nil, errors.New("runtime supervisor is not configured")
	}
	return c.app.supervisor.Call(ctx, method, args, timeout)
}

type guardianRuntimeCaller struct {
	base       automation.RuntimeCaller
	guardian   *guard.Supervisor
	generation func() uint64
}

func (c guardianRuntimeCaller) Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error) {
	if c.base == nil {
		return nil, errors.New("guardian runtime caller is not configured")
	}
	if c.guardian == nil {
		return c.base.Call(ctx, method, args, timeout)
	}
	generation := c.guardian.Generation()
	if c.generation != nil {
		generation = c.generation()
	}
	value, err := c.base.Call(ctx, method, args, timeout)
	if err != nil {
		c.guardian.NoteRuntimeError(err)
		return nil, err
	}
	c.guardian.NoteHealthy()
	if !c.guardian.IsCurrentGeneration(generation) {
		return nil, guard.ErrStaleRuntimeGeneration
	}
	return value, nil
}

func (a *App) farmRuntimeCaller() automation.RuntimeCaller {
	if a != nil && a.runtimeCache != nil {
		return a.runtimeCache
	}
	if a != nil {
		return a.supervisor
	}
	return nil
}

func (a *App) runtimeCallCacheScope() string {
	if a == nil {
		return "default"
	}
	accountKey := a.accountKey()
	if a.supervisor == nil {
		return accountKey
	}
	status := a.supervisor.Status()
	return strings.Join([]string{
		accountKey,
		status.Target,
		status.InstanceID,
		string(status.Phase),
	}, "|")
}

func runtimeStatusInvalidatesCache(previous farmruntime.Status, next farmruntime.Status) bool {
	return previous.Target != next.Target ||
		previous.Phase != next.Phase ||
		previous.Connected != next.Connected ||
		previous.Ready != next.Ready ||
		previous.InstanceID != next.InstanceID
}

func (a *App) contextOrBackground() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) socialService() *social.Service {
	return a.socialServiceForAccount(a.accountKey())
}

func (a *App) socialServiceForAccount(accountKey string) *social.Service {
	accountKey = storage.NormalizeAccountKey(accountKey)
	var socialStore social.Store
	if a.store != nil {
		socialStore = socialStorageAdapter{store: a.store}
	}
	return social.NewService(socialStore, a.farmRuntimeCaller(), social.Options{
		AccountKey:       accountKey,
		AutomationConfig: cloneAutomationConfig(a.farmAutomationStateForAccount(accountKey).Config),
	})
}

func (a *App) socialDogGuardScanner() *social.DogGuardScanner {
	a.socialMu.Lock()
	defer a.socialMu.Unlock()
	accountKey := a.accountKey()
	if a.dogGuardScanner != nil && a.dogGuardStore == a.store && a.dogGuardAccountKey == accountKey {
		return a.dogGuardScanner
	}

	var dogStore social.DogGuardStore
	if a.store != nil {
		dogStore = socialStorageAdapter{store: a.store}
	}
	a.dogGuardScanner = social.NewDogGuardScanner(dogStore, a.farmRuntimeCaller(), social.DogGuardOptions{AccountKey: accountKey})
	a.dogGuardStore = a.store
	a.dogGuardAccountKey = accountKey
	return a.dogGuardScanner
}

type socialStorageAdapter struct {
	store *storage.Store
}

func (s socialStorageAdapter) SaveMysteryShopPurchaseRecord(ctx context.Context, accountKey string, record automation.MysteryShopPurchaseRecord) error {
	return s.store.AppendMysteryShopPurchaseRecord(ctx, accountKey, storage.MysteryShopPurchaseRecord{
		ID:           record.ID,
		OccurredAt:   record.OccurredAt,
		GoodsID:      record.GoodsID,
		ItemID:       record.ItemID,
		ItemName:     record.ItemName,
		Count:        record.Count,
		UnitPrice:    record.UnitPrice,
		CurrencyID:   record.CurrencyID,
		CurrencyName: record.CurrencyName,
		Discount:     record.Discount,
		Payload:      record.Payload,
	})
}

func (s socialStorageAdapter) LoadFriendRules(ctx context.Context, accountKey string) (social.FriendRules, error) {
	rules, err := s.store.LoadSocialFriendRules(ctx, accountKey)
	if err != nil {
		return social.FriendRules{}, err
	}
	return social.FriendRules{
		WhitelistEnabled: rules.WhitelistEnabled,
		WhitelistScopes:  rules.WhitelistScopes,
		Whitelist:        rules.Whitelist,
		BlacklistEnabled: rules.BlacklistEnabled,
		BlacklistScopes:  rules.BlacklistScopes,
		Blacklist:        rules.Blacklist,
		MaskedBlacklist:  rules.MaskedBlacklist,
		MaskedMaxLevel:   rules.MaskedMaxLevel,
	}, nil
}

func (s socialStorageAdapter) SaveFriendRules(ctx context.Context, accountKey string, rules social.FriendRules) error {
	return s.store.SaveSocialFriendRules(ctx, accountKey, storage.SocialFriendRules{
		WhitelistEnabled: rules.WhitelistEnabled,
		WhitelistScopes:  rules.WhitelistScopes,
		Whitelist:        rules.Whitelist,
		BlacklistEnabled: rules.BlacklistEnabled,
		BlacklistScopes:  rules.BlacklistScopes,
		Blacklist:        rules.Blacklist,
		MaskedBlacklist:  rules.MaskedBlacklist,
		MaskedMaxLevel:   rules.MaskedMaxLevel,
	})
}

func (s socialStorageAdapter) LoadRankingPreferences(ctx context.Context, accountKey string) (social.RankingPreferences, error) {
	preferences, err := s.store.LoadSocialRankingPreferences(ctx, accountKey)
	if err != nil {
		return social.RankingPreferences{}, err
	}
	return social.RankingPreferences{
		StolenByMeViewMode:   preferences.StolenByMeViewMode,
		StolenFromMeViewMode: preferences.StolenFromMeViewMode,
	}, nil
}

func (s socialStorageAdapter) SaveRankingPreferences(ctx context.Context, accountKey string, preferences social.RankingPreferences) error {
	return s.store.SaveSocialRankingPreferences(ctx, accountKey, storage.SocialRankingPreferences{
		StolenByMeViewMode:   preferences.StolenByMeViewMode,
		StolenFromMeViewMode: preferences.StolenFromMeViewMode,
	})
}

func (s socialStorageAdapter) ListStealRecords(ctx context.Context, accountKey string, dateKeys []string) ([]social.StealRecord, error) {
	rows, err := s.store.ListSocialStealRecords(ctx, accountKey, dateKeys)
	if err != nil {
		return nil, err
	}
	records := make([]social.StealRecord, 0, len(rows))
	for _, row := range rows {
		var record social.StealRecord
		if row.PayloadJSON != "" {
			_ = json.Unmarshal([]byte(row.PayloadJSON), &record)
		}
		if record.ID == "" {
			record.ID = row.ID
		}
		if record.GID == 0 {
			record.GID = row.GID
		}
		if record.DisplayName == "" {
			record.DisplayName = row.DisplayName
		}
		if record.OccurredAt == "" {
			record.OccurredAt = row.OccurredAt
		}
		records = append(records, record)
	}
	return records, nil
}

func (s socialStorageAdapter) ListVisitorRecords(ctx context.Context, accountKey string) ([]social.VisitorRecord, error) {
	rows, err := s.store.ListSocialVisitorRecords(ctx, accountKey)
	if err != nil {
		return nil, err
	}
	records := make([]social.VisitorRecord, 0, len(rows))
	for _, row := range rows {
		var record social.VisitorRecord
		if row.PayloadJSON != "" {
			_ = json.Unmarshal([]byte(row.PayloadJSON), &record)
		}
		if record.ID == "" {
			record.ID = row.ID
		}
		if record.PlayerID == 0 {
			record.PlayerID = row.PlayerID
		}
		if record.DisplayName == "" {
			record.DisplayName = row.DisplayName
		}
		if record.ActionType == 0 {
			record.ActionType = row.ActionType
		}
		if record.Time == 0 {
			record.Time = row.OccurredAtMS
		}
		records = append(records, record)
	}
	return records, nil
}

func (s socialStorageAdapter) SaveStealRecords(ctx context.Context, accountKey string, records []social.StealRecord) error {
	rows := make([]storage.SocialRecordRow, 0, len(records))
	for _, record := range records {
		if record.StealCount <= 0 {
			record.StealCount = 1
		}
		raw, err := json.Marshal(record)
		if err != nil {
			return err
		}
		rows = append(rows, storage.SocialRecordRow{
			ID:          nonEmpty(record.ID, record.DisplayName+":"+record.OccurredAt),
			DateKey:     socialDateKey(record.OccurredAt),
			OccurredAt:  record.OccurredAt,
			GID:         record.GID,
			DisplayName: record.DisplayName,
			PayloadJSON: string(raw),
		})
	}
	return s.store.AppendSocialStealRecords(ctx, accountKey, rows)
}

func (s socialStorageAdapter) SaveVisitorRecords(ctx context.Context, accountKey string, records []social.VisitorRecord) error {
	rows := make([]storage.SocialVisitorRow, 0, len(records))
	for _, record := range records {
		raw, err := json.Marshal(record)
		if err != nil {
			return err
		}
		rows = append(rows, storage.SocialVisitorRow{
			ID:           nonEmpty(record.ID, record.DisplayName+":"+strconv.FormatInt(record.Time, 10)+":"+strconv.Itoa(record.ActionType)),
			OccurredAtMS: socialVisitorTimeMS(record.Time),
			PlayerID:     record.PlayerID,
			DisplayName:  record.DisplayName,
			ActionType:   record.ActionType,
			PayloadJSON:  string(raw),
		})
	}
	return s.store.AppendSocialVisitorRecords(ctx, accountKey, rows)
}

func (s socialStorageAdapter) LoadDogGuardState(ctx context.Context, accountKey string) (social.DogGuardState, error) {
	raw, err := s.store.LoadSocialDogGuardCache(ctx, accountKey)
	if err != nil {
		return social.DogGuardState{}, err
	}
	if strings.TrimSpace(raw) == "" {
		return social.DogGuardState{Results: []social.DogGuardRow{}}, nil
	}
	var state social.DogGuardState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return social.DogGuardState{}, err
	}
	if state.Results == nil {
		state.Results = []social.DogGuardRow{}
	}
	return state, nil
}

func (s socialStorageAdapter) SaveDogGuardState(ctx context.Context, accountKey string, state social.DogGuardState) error {
	if state.Results == nil {
		state.Results = []social.DogGuardRow{}
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.store.SaveSocialDogGuardCache(ctx, accountKey, string(raw))
}

func socialDateKey(occurredAt string) string {
	if parsed, err := time.Parse(time.RFC3339, occurredAt); err == nil {
		return parsed.Format("2006-01-02")
	}
	if len(occurredAt) >= 10 {
		return occurredAt[:10]
	}
	return time.Now().Format("2006-01-02")
}

func socialVisitorTimeMS(value int64) int64 {
	if value > 1e10 {
		return value
	}
	return value * 1000
}

func nonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func defaultDataDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "Farm_Go")
	}
	return filepath.Join(os.TempDir(), "Farm_Go")
}
