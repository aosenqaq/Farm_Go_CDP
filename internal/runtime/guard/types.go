package guard

type Phase string

const (
	PhaseDisabled         Phase = "disabled"
	PhaseStandby          Phase = "standby"
	PhaseWatching         Phase = "watching"
	PhaseDegraded         Phase = "degraded"
	PhaseRestarting       Phase = "restarting"
	PhaseWaitingReconnect Phase = "waiting_reconnect"
	PhaseCircuitOpen      Phase = "circuit_open"
)

type LifecycleKind string

const (
	LifecycleSuspected        LifecycleKind = "suspected"
	LifecycleAbnormal         LifecycleKind = "abnormal"
	LifecycleRecovery         LifecycleKind = "recovery"
	LifecycleRestartCompleted LifecycleKind = "restart_completed"
)

type LifecycleEvent struct {
	Kind          LifecycleKind `json:"kind"`
	RuntimeTarget string        `json:"runtimeTarget"`
	Error         string        `json:"error,omitempty"`
	Trigger       string        `json:"trigger,omitempty"`
	Result        RestartResult `json:"result"`
	Streak        int           `json:"streak,omitempty"`
	Threshold     int           `json:"threshold,omitempty"`
	DurationMS    int64         `json:"durationMs,omitempty"`
	At            string        `json:"at"`
}

type Settings struct {
	Enabled                     bool `json:"enabled"`
	FailureRecoveryEnabled      bool `json:"failureRecoveryEnabled"`
	TimeoutThreshold            int  `json:"timeoutThreshold"`
	MonitorIntervalMS           int  `json:"monitorIntervalMs"`
	RestartReconnectGraceSec    int  `json:"restartReconnectGraceSec"`
	MaxRestartsPer10Min         int  `json:"maxRestartsPer10Min"`
	ScheduledRestartEnabled     bool `json:"scheduledRestartEnabled"`
	ScheduledRestartIntervalMin int  `json:"scheduledRestartIntervalMin"`
	AutoMinimizeAfterRestart    bool `json:"autoMinimizeAfterRestart"`
	NetworkReconnectEnabled     bool `json:"networkReconnectEnabled"`
	NetworkReconnectIntervalMS  int  `json:"networkReconnectIntervalMs"`
	NetworkRecoveryTimeoutMS    int  `json:"networkRecoveryTimeoutMs"`
	OtherPlaceLoginEnabled      bool `json:"otherPlaceLoginEnabled"`
	OtherPlaceLoginIntervalMS   int  `json:"otherPlaceLoginIntervalMs"`
	OtherPlaceLoginDelayMin     int  `json:"otherPlaceLoginDelayMin"`
}

func DefaultSettings() Settings {
	return Settings{
		FailureRecoveryEnabled:      true,
		TimeoutThreshold:            3,
		MonitorIntervalMS:           3000,
		RestartReconnectGraceSec:    45,
		MaxRestartsPer10Min:         4,
		ScheduledRestartIntervalMin: 60,
		NetworkReconnectEnabled:     true,
		NetworkReconnectIntervalMS:  1000,
		NetworkRecoveryTimeoutMS:    20000,
		OtherPlaceLoginIntervalMS:   5000,
		OtherPlaceLoginDelayMin:     5,
	}
}

type WorkerStatus struct {
	Enabled       bool   `json:"enabled"`
	Running       bool   `json:"running"`
	Busy          bool   `json:"busy"`
	LastCheckAt   string `json:"lastCheckAt,omitempty"`
	LastHandledAt string `json:"lastHandledAt,omitempty"`
	LastResult    string `json:"lastResult,omitempty"`
	LastError     string `json:"lastError,omitempty"`
}

type RuntimeEvent struct {
	Name            string `json:"name"`
	Phase           string `json:"phase"`
	RuntimeTarget   string `json:"runtimeTarget,omitempty"`
	AccountKey      string `json:"accountKey,omitempty"`
	GID             string `json:"gid,omitempty"`
	Handled         bool   `json:"handled,omitempty"`
	Via             string `json:"via,omitempty"`
	FirstDetectedAt int64  `json:"firstDetectedAt,omitempty"`
	HandledAt       int64  `json:"handledAt,omitempty"`
	RemainingMS     int64  `json:"remainingMs,omitempty"`
	Error           string `json:"error,omitempty"`
}

type SupervisorStatus struct {
	Enabled         bool           `json:"enabled"`
	Running         bool           `json:"running"`
	Phase           Phase          `json:"phase"`
	RuntimeTarget   string         `json:"runtimeTarget"`
	Process         Status         `json:"process"`
	Network         WorkerStatus   `json:"network"`
	OtherPlaceLogin WorkerStatus   `json:"otherPlaceLogin"`
	RecentEvents    []RuntimeEvent `json:"recentEvents"`
}

type ActionRequest struct {
	Action string `json:"action"`
	Reason string `json:"reason,omitempty"`
}

type ActionResult struct {
	OK     bool   `json:"ok"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}
