package guard

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type HostWindowSnapshot struct {
	HWND    uint64 `json:"hwnd"`
	PID     int    `json:"pid"`
	Title   string `json:"title"`
	Visible bool   `json:"visible"`
}

type HostProcessSnapshot struct {
	PID            int                  `json:"pid"`
	ParentPID      int                  `json:"parentPid,omitempty"`
	ProcessName    string               `json:"processName"`
	ExecutablePath string               `json:"executablePath,omitempty"`
	Windows        []HostWindowSnapshot `json:"windows"`
}

type HostCandidateEvidence struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type HostProcessCandidate struct {
	PID            int                     `json:"pid"`
	ParentPID      int                     `json:"parentPid,omitempty"`
	ProcessName    string                  `json:"processName"`
	ExecutablePath string                  `json:"executablePath,omitempty"`
	WindowTitles   []string                `json:"windowTitles"`
	HWNDs          []uint64                `json:"hwnds"`
	Confidence     string                  `json:"confidence"`
	BoundOwner     string                  `json:"boundOwner,omitempty"`
	Available      bool                    `json:"available"`
	Evidence       []HostCandidateEvidence `json:"evidence"`
}

type HostBinding struct {
	Owner          string   `json:"owner"`
	PID            int      `json:"pid"`
	ParentPID      int      `json:"parentPid,omitempty"`
	ProcessName    string   `json:"processName"`
	ExecutablePath string   `json:"executablePath,omitempty"`
	WindowTitles   []string `json:"windowTitles"`
	HWNDs          []uint64 `json:"hwnds"`
	Confidence     string   `json:"confidence"`
	BoundAt        string   `json:"boundAt"`
}

type AutoBindOwnerResult struct {
	Owner      string                 `json:"owner"`
	Status     string                 `json:"status"`
	Binding    *HostBinding           `json:"binding,omitempty"`
	Candidates []HostProcessCandidate `json:"candidates"`
}

type HostRestartPreview struct {
	Owner   string                `json:"owner"`
	Allowed bool                  `json:"allowed"`
	Status  string                `json:"status"`
	Reason  string                `json:"reason"`
	Binding *HostBinding          `json:"binding,omitempty"`
	Current *HostProcessCandidate `json:"current,omitempty"`
}

type RestartRequest struct {
	Registry    *HostBindingRegistry
	Owner       string
	Platform    string
	Snapshots   []HostProcessSnapshot
	StopPID     func(pid int) error
	Launch      func(LaunchRequest) error
	LaunchDelay time.Duration
	Sleep       func(time.Duration)
}

type RestartResult struct {
	Owner            string                 `json:"owner"`
	Status           string                 `json:"status"`
	Reason           string                 `json:"reason"`
	OldPID           int                    `json:"oldPid,omitempty"`
	Stopped          bool                   `json:"stopped"`
	LaunchDispatched bool                   `json:"launchDispatched"`
	Binding          *HostBinding           `json:"binding,omitempty"`
	Candidates       []HostProcessCandidate `json:"candidates"`
}

type HostLaunchResult struct {
	Owner            string `json:"owner"`
	Platform         string `json:"platform"`
	Status           string `json:"status"`
	Reason           string `json:"reason"`
	LaunchDispatched bool   `json:"launchDispatched"`
}

type HostBindingRegistry struct {
	mu       sync.Mutex
	bindings map[string]HostBinding
}

func NewHostBindingRegistry() *HostBindingRegistry {
	return &HostBindingRegistry{bindings: map[string]HostBinding{}}
}

func (r *HostBindingRegistry) Bind(owner string, candidate HostProcessCandidate) (HostBinding, error) {
	if r == nil {
		return HostBinding{}, errors.New("host binding registry is required")
	}
	owner = normalizeOwner(owner)
	if candidate.PID <= 0 {
		return HostBinding{}, errors.New("host candidate pid is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for existingOwner, binding := range r.bindings {
		if existingOwner != owner && binding.PID == candidate.PID {
			return HostBinding{}, fmt.Errorf("host_pid_already_bound: %s", existingOwner)
		}
	}
	binding := HostBinding{
		Owner:          owner,
		PID:            candidate.PID,
		ParentPID:      candidate.ParentPID,
		ProcessName:    candidate.ProcessName,
		ExecutablePath: candidate.ExecutablePath,
		WindowTitles:   append([]string{}, candidate.WindowTitles...),
		HWNDs:          append([]uint64{}, candidate.HWNDs...),
		Confidence:     "verified",
		BoundAt:        time.Now().UTC().Format(time.RFC3339Nano),
	}
	r.bindings[owner] = binding
	return binding, nil
}

func (r *HostBindingRegistry) Clear(owner string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	owner = normalizeOwner(owner)
	_, ok := r.bindings[owner]
	delete(r.bindings, owner)
	return ok
}

func (r *HostBindingRegistry) Binding(owner string) (HostBinding, bool) {
	if r == nil {
		return HostBinding{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	binding, ok := r.bindings[normalizeOwner(owner)]
	return binding, ok
}

func (r *HostBindingRegistry) Bindings() []HostBinding {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]HostBinding, 0, len(r.bindings))
	for _, binding := range r.bindings {
		result = append(result, binding)
	}
	return result
}

func (r *HostBindingRegistry) AutoBind(owner string, candidates []HostProcessCandidate) (AutoBindOwnerResult, error) {
	owner = normalizeOwner(owner)
	annotated := r.annotateBoundOwners(owner, candidates)
	bindable := autoBindableCandidates(annotated)
	for i := range bindable {
		if bindable[i].BoundOwner == owner {
			binding, _ := r.Binding(owner)
			return AutoBindOwnerResult{Owner: owner, Status: "already_bound", Binding: &binding, Candidates: annotated}, nil
		}
	}
	if len(bindable) == 0 {
		return AutoBindOwnerResult{Owner: owner, Status: "not_found", Candidates: annotated}, nil
	}
	if len(bindable) > 1 {
		return AutoBindOwnerResult{Owner: owner, Status: "ambiguous", Candidates: annotated}, nil
	}
	binding, err := r.Bind(owner, bindable[0])
	if err != nil {
		return AutoBindOwnerResult{}, err
	}
	annotated = r.annotateBoundOwners(owner, annotated)
	return AutoBindOwnerResult{Owner: owner, Status: "bound", Binding: &binding, Candidates: annotated}, nil
}

func CandidatesForPlatform(platform string, snapshots []HostProcessSnapshot) []HostProcessCandidate {
	result := make([]HostProcessCandidate, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if !snapshotMatchesPlatform(platform, snapshot) {
			continue
		}
		result = append(result, candidateFromSnapshot(snapshot))
	}
	return result
}

func PreviewRestart(registry *HostBindingRegistry, owner string, snapshots []HostProcessSnapshot) HostRestartPreview {
	owner = normalizeOwner(owner)
	binding, ok := registry.Binding(owner)
	if !ok {
		return HostRestartPreview{Owner: owner, Status: "not_bound", Reason: "owner has no bound host PID"}
	}
	currentSnapshot, ok := findSnapshotByPID(snapshots, binding.PID)
	if !ok {
		return HostRestartPreview{Owner: owner, Status: "stale", Reason: "bound PID is no longer running", Binding: &binding}
	}
	current := candidateFromSnapshot(currentSnapshot)
	if !hostIdentityMatches(binding, current) {
		return HostRestartPreview{Owner: owner, Status: "mismatch", Reason: "current PID identity does not match the bound host", Binding: &binding, Current: &current}
	}
	if !boundWindowStillVisible(binding, current) {
		return HostRestartPreview{Owner: owner, Status: "window_closed", Reason: "bound window is no longer visible under the host PID", Binding: &binding, Current: &current}
	}
	return HostRestartPreview{
		Owner:   owner,
		Allowed: true,
		Status:  "ready",
		Reason:  "bound PID is still running and matches the recorded host identity",
		Binding: &binding,
		Current: &current,
	}
}

func RestartBoundHost(request RestartRequest) (RestartResult, error) {
	if request.Registry == nil {
		return RestartResult{}, errors.New("host binding registry is required")
	}
	owner := normalizeOwner(request.Owner)
	binding, ok := request.Registry.Binding(owner)
	if !ok {
		return RestartResult{}, errors.New("owner has no bound host PID")
	}
	preview := PreviewRestart(request.Registry, owner, request.Snapshots)
	if !preview.Allowed {
		return RestartResult{
			Owner:   owner,
			Status:  preview.Status,
			Reason:  preview.Reason,
			OldPID:  binding.PID,
			Binding: &binding,
		}, nil
	}
	if request.StopPID == nil {
		return RestartResult{}, errors.New("stop pid callback is required")
	}
	if err := request.StopPID(binding.PID); err != nil {
		return RestartResult{}, err
	}
	request.Registry.Clear(owner)
	launchRequest, err := launchRequestForPlatform(request.Platform, &binding)
	if err != nil {
		return RestartResult{}, err
	}
	if request.Launch != nil {
		if request.LaunchDelay > 0 {
			sleep := request.Sleep
			if sleep == nil {
				sleep = time.Sleep
			}
			sleep(request.LaunchDelay)
		}
		if err := request.Launch(launchRequest); err != nil {
			return RestartResult{}, err
		}
	}
	return RestartResult{
		Owner:            owner,
		Status:           "launch_dispatched",
		Reason:           "bound host PID was stopped and launch was dispatched",
		OldPID:           binding.PID,
		Stopped:          true,
		LaunchDispatched: true,
		Binding:          &binding,
	}, nil
}

func (r *HostBindingRegistry) annotateBoundOwners(owner string, candidates []HostProcessCandidate) []HostProcessCandidate {
	owner = normalizeOwner(owner)
	if r == nil {
		return candidates
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]HostProcessCandidate, len(candidates))
	for i, candidate := range candidates {
		result[i] = candidate
		if !result[i].Available && result[i].BoundOwner == "" {
			continue
		}
		result[i].Available = true
		for boundOwner, binding := range r.bindings {
			if binding.PID != candidate.PID {
				continue
			}
			result[i].BoundOwner = boundOwner
			result[i].Available = boundOwner == owner
			break
		}
	}
	return result
}

func autoBindableCandidates(candidates []HostProcessCandidate) []HostProcessCandidate {
	var miniapp []HostProcessCandidate
	for _, candidate := range candidates {
		if !candidate.Available {
			continue
		}
		if hasMiniappWindowTitle(candidate.WindowTitles) {
			miniapp = append(miniapp, candidate)
		}
	}
	if len(miniapp) > 0 {
		return miniapp
	}
	var visible []HostProcessCandidate
	for _, candidate := range candidates {
		if candidate.Available && hasEvidence(candidate, "windowTitle") && !isWeChatAppEx(candidate.ProcessName) {
			visible = append(visible, candidate)
		}
	}
	if len(visible) > 0 {
		return visible
	}
	var processOnly []HostProcessCandidate
	for _, candidate := range candidates {
		if candidate.Available && !isWeChatAppEx(candidate.ProcessName) {
			processOnly = append(processOnly, candidate)
		}
	}
	return processOnly
}

func candidateFromSnapshot(snapshot HostProcessSnapshot) HostProcessCandidate {
	candidate := HostProcessCandidate{
		PID:            snapshot.PID,
		ParentPID:      snapshot.ParentPID,
		ProcessName:    snapshot.ProcessName,
		ExecutablePath: snapshot.ExecutablePath,
		Available:      true,
		Confidence:     "candidate",
	}
	for _, window := range snapshot.Windows {
		if !window.Visible || strings.TrimSpace(window.Title) == "" {
			continue
		}
		candidate.WindowTitles = append(candidate.WindowTitles, window.Title)
		candidate.HWNDs = append(candidate.HWNDs, window.HWND)
		candidate.Evidence = append(candidate.Evidence, HostCandidateEvidence{Kind: "windowTitle", Value: window.Title})
	}
	if candidate.ExecutablePath != "" {
		candidate.Evidence = append(candidate.Evidence, HostCandidateEvidence{Kind: "path", Value: candidate.ExecutablePath})
	}
	return candidate
}

func snapshotMatchesPlatform(platform string, snapshot HostProcessSnapshot) bool {
	name := strings.ToLower(snapshot.ProcessName)
	switch normalizePlatform(platform) {
	case "qq":
		return strings.Contains(name, "qq")
	case "wx":
		return isWeChatAppEx(snapshot.ProcessName)
	case "yyb":
		return strings.Contains(name, "androws") || strings.Contains(name, "wechatappex")
	default:
		return false
	}
}

func findSnapshotByPID(snapshots []HostProcessSnapshot, pid int) (HostProcessSnapshot, bool) {
	for _, snapshot := range snapshots {
		if snapshot.PID == pid {
			return snapshot, true
		}
	}
	return HostProcessSnapshot{}, false
}

func hostIdentityMatches(binding HostBinding, current HostProcessCandidate) bool {
	return strings.EqualFold(strings.TrimSpace(binding.ProcessName), strings.TrimSpace(current.ProcessName))
}

func boundWindowStillVisible(binding HostBinding, current HostProcessCandidate) bool {
	if len(binding.HWNDs) > 0 {
		for _, want := range binding.HWNDs {
			for _, got := range current.HWNDs {
				if want == got && got != 0 {
					return true
				}
			}
		}
		return false
	}
	for _, want := range binding.WindowTitles {
		for _, got := range current.WindowTitles {
			if strings.EqualFold(strings.TrimSpace(want), strings.TrimSpace(got)) {
				return true
			}
		}
	}
	return len(binding.WindowTitles) == 0 && len(current.WindowTitles) == 0
}

func hasMiniappWindowTitle(titles []string) bool {
	for _, title := range titles {
		text := strings.ToLower(strings.TrimSpace(title))
		if strings.Contains(text, "qq经典农场") || strings.Contains(text, "小程序") || strings.Contains(text, "农场") {
			return true
		}
	}
	return false
}

func hasEvidence(candidate HostProcessCandidate, kind string) bool {
	for _, evidence := range candidate.Evidence {
		if evidence.Kind == kind {
			return true
		}
	}
	return false
}

func isWeChatAppEx(processName string) bool {
	return strings.EqualFold(processName, "WeChatAppEx.exe") || strings.Contains(strings.ToLower(processName), "wechatappex")
}

func normalizeOwner(owner string) string {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return "main"
	}
	return owner
}

func minimizableWindowHandles(snapshots []HostWindowSnapshot) []uint64 {
	handles := make([]uint64, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.Visible && snapshot.HWND != 0 {
			handles = append(handles, snapshot.HWND)
		}
	}
	return handles
}
