package social

import "time"

type Status string

const (
	StatusOK              Status = "ok"
	StatusRuntimeNotReady Status = "runtime_not_ready"
	StatusUnsupported     Status = "unsupported_target"
	StatusBusy            Status = "busy"
	StatusSkipped         Status = "skipped"
	StatusFailed          Status = "failed"
)

type ActionResult struct {
	OK      bool   `json:"ok"`
	Status  Status `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type FriendRules struct {
	WhitelistEnabled bool     `json:"whitelistEnabled"`
	WhitelistScopes  []string `json:"whitelistScopes"`
	Whitelist        []string `json:"whitelist"`
	BlacklistEnabled bool     `json:"blacklistEnabled"`
	BlacklistScopes  []string `json:"blacklistScopes"`
	Blacklist        []string `json:"blacklist"`
	MaskedBlacklist  bool     `json:"maskedBlacklist"`
	MaskedMaxLevel   int      `json:"maskedMaxLevel"`
}

type FriendRow struct {
	GID             int            `json:"gid"`
	Name            string         `json:"name,omitempty"`
	DisplayName     string         `json:"displayName"`
	Remark          string         `json:"remark,omitempty"`
	AvatarURL       string         `json:"avatarUrl,omitempty"`
	Level           int            `json:"level,omitempty"`
	WorkCounts      map[string]int `json:"workCounts"`
	Stealable       bool           `json:"stealable"`
	Helpable        bool           `json:"helpable"`
	Mischiefable    bool           `json:"mischiefable"`
	Blacklisted     bool           `json:"blacklisted"`
	Whitelisted     bool           `json:"whitelisted"`
	MaskedBlocked   bool           `json:"maskedBlocked"`
	Protected       bool           `json:"protected"`
	ProtocolBlocked bool           `json:"protocolBlocked"`
	HasGuardDog     bool           `json:"hasGuardDog"`
	Raw             map[string]any `json:"raw,omitempty"`
}

type FriendActionRequest struct {
	Action  string       `json:"action"`
	Target  string       `json:"target,omitempty"`
	Targets []string     `json:"targets,omitempty"`
	Rules   *FriendRules `json:"rules,omitempty"`
	DryRun  bool         `json:"dryRun,omitempty"`
}

type ProtocolBlockList struct {
	OK      bool        `json:"ok"`
	Status  Status      `json:"status"`
	Message string      `json:"message"`
	Friends []FriendRow `json:"friends"`
	Raw     any         `json:"raw,omitempty"`
}

type RankingRequest struct {
	Tab       string    `json:"tab,omitempty"`
	ViewMode  string    `json:"viewMode,omitempty"`
	DateRange string    `json:"dateRange,omitempty"`
	Cursor    string    `json:"cursor,omitempty"`
	Limit     int       `json:"limit,omitempty"`
	Now       time.Time `json:"-"`
}

type RankingPageRow struct {
	Kind         string      `json:"kind"`
	Key          string      `json:"key"`
	IdentityKey  string      `json:"-"`
	TimeMS       int64       `json:"timeMS"`
	DisplayName  string      `json:"displayName"`
	Rank         int         `json:"rank,omitempty"`
	EventCount   int         `json:"eventCount,omitempty"`
	StealCount   int         `json:"stealCount,omitempty"`
	Items        []StealItem `json:"items"`
	ActionType   int         `json:"actionType,omitempty"`
	ActionLabel  string      `json:"actionLabel,omitempty"`
	ActionTarget string      `json:"actionTarget,omitempty"`
}

type RankingPage struct {
	OK         bool             `json:"ok"`
	Status     Status           `json:"status"`
	Message    string           `json:"message"`
	Tab        string           `json:"tab"`
	ViewMode   string           `json:"viewMode"`
	DateRange  string           `json:"dateRange"`
	Summary    RankingSummary   `json:"summary"`
	Rows       []RankingPageRow `json:"rows"`
	NextCursor string           `json:"nextCursor,omitempty"`
	HasMore    bool             `json:"hasMore"`
}

type RankingSummary struct {
	VisitorCount          int `json:"visitorCount"`
	StolenFromMeCount     int `json:"stolenFromMeCount"`
	StolenByMeCount       int `json:"stolenByMeCount"`
	StolenByMeRecordCount int `json:"stolenByMeRecordCount"`
}

type RankingPreferences struct {
	StolenByMeViewMode   string `json:"stolenByMeViewMode"`
	StolenFromMeViewMode string `json:"stolenFromMeViewMode"`
}

type DogGuardScanRequest struct {
	Refresh         bool `json:"refresh,omitempty"`
	SkipScanned     bool `json:"skipScanned,omitempty"`
	ExcludeGuardDog bool `json:"excludeGuardDog,omitempty"`
	ScanIntervalMS  int  `json:"scanIntervalMs,omitempty"`
}

type DogGuardActionRequest struct {
	Action string `json:"action"`
	DogGuardScanRequest
}

type DogGuardRow struct {
	GID         int            `json:"gid"`
	Name        string         `json:"name"`
	DisplayName string         `json:"displayName,omitempty"`
	Remark      string         `json:"remark,omitempty"`
	Level       int            `json:"level,omitempty"`
	Scanned     bool           `json:"scanned"`
	HasGuardDog bool           `json:"hasGuardDog"`
	DogID       int            `json:"dogId,omitempty"`
	DogName     string         `json:"dogName,omitempty"`
	DogConfig   map[string]any `json:"dogConfig,omitempty"`
	Error       string         `json:"error,omitempty"`
	ScannedAt   string         `json:"scannedAt,omitempty"`
}

type DogGuardState struct {
	Running          bool          `json:"running"`
	StopRequested    bool          `json:"stopRequested"`
	StartedAt        string        `json:"startedAt,omitempty"`
	FinishedAt       string        `json:"finishedAt,omitempty"`
	Total            int           `json:"total"`
	Scanned          int           `json:"scanned"`
	HasGuardDogCount int           `json:"hasGuardDogCount"`
	Current          *DogGuardRow  `json:"current,omitempty"`
	Results          []DogGuardRow `json:"results"`
	Error            string        `json:"error,omitempty"`
}

type ExportRequest struct {
	Groups                []string `json:"groups"`
	RefreshFriendSnapshot bool     `json:"refreshFriendSnapshot,omitempty"`
}

type ImportExportPayload struct {
	OK             bool            `json:"ok"`
	Status         Status          `json:"status"`
	Message        string          `json:"message"`
	Version        int             `json:"version"`
	ExportedAt     string          `json:"exportedAt,omitempty"`
	AccountKey     string          `json:"accountKey"`
	Groups         []string        `json:"groups"`
	Rules          *FriendRules    `json:"rules,omitempty"`
	FriendSnapshot []FriendRow     `json:"friendSnapshot,omitempty"`
	StealRecords   []StealRecord   `json:"stealRecords,omitempty"`
	VisitorRecords []VisitorRecord `json:"visitorRecords,omitempty"`
	DogGuard       *DogGuardState  `json:"dogGuard,omitempty"`
}
