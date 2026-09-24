package guard

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const DefaultQQMiniappCloseTimeout = 5 * time.Second

func QQMiniappCandidates(snapshots []HostProcessSnapshot) []HostProcessCandidate {
	byPID := make(map[int]HostProcessSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		byPID[snapshot.PID] = snapshot
	}
	result := make([]HostProcessCandidate, 0, 1)
	for _, snapshot := range snapshots {
		if !strings.EqualFold(strings.TrimSpace(snapshot.ProcessName), "QQ.exe") {
			continue
		}
		parent, ok := byPID[snapshot.ParentPID]
		if !ok || !strings.EqualFold(strings.TrimSpace(parent.ProcessName), "QQ.exe") {
			continue
		}
		if !hasQQMiniappWindow(snapshot.Windows) {
			continue
		}
		candidate := candidateFromSnapshot(snapshot)
		result = append(result, candidate)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].PID < result[j].PID })
	return result
}

func hasQQMiniappWindow(windows []HostWindowSnapshot) bool {
	for _, window := range windows {
		if window.Visible && window.HWND != 0 && hasMiniappWindowTitle([]string{window.Title}) {
			return true
		}
	}
	return false
}

func QQMiniappTreeExited(rootPID int, snapshots []HostProcessSnapshot) bool {
	if rootPID <= 0 {
		return true
	}
	parents := make(map[int]int, len(snapshots))
	for _, snapshot := range snapshots {
		parents[snapshot.PID] = snapshot.ParentPID
	}
	for _, snapshot := range snapshots {
		if snapshot.PID == rootPID || processDescendsFrom(snapshot.PID, rootPID, parents) {
			return false
		}
	}
	return true
}

func processDescendsFrom(pid int, rootPID int, parents map[int]int) bool {
	seen := map[int]bool{}
	for current := pid; current > 0 && !seen[current]; current = parents[current] {
		seen[current] = true
		parent, ok := parents[current]
		if !ok {
			return false
		}
		if parent == rootPID {
			return true
		}
	}
	return false
}

type QQCloseObservation struct {
	Disconnected bool
	TreeExited   bool
	Snapshots    []HostProcessSnapshot
}

type QQReadyObservation struct {
	Accepted   bool
	Target     string
	Connected  bool
	Ready      bool
	InstanceID string
}

type QQRestartRequest struct {
	Registry          *HostBindingRegistry
	Owner             string
	Snapshots         []HostProcessSnapshot
	RuntimeConnected  bool
	RuntimeInstanceID string
	CloseWindows      func([]HostWindowSnapshot) error
	WaitForClosed     func(int, time.Duration) (QQCloseObservation, error)
	StopPID           func(int) error
	Launch            func(LaunchRequest) error
	WaitForReady      func(string, time.Duration) QQReadyObservation
	RefreshSnapshots  func() ([]HostProcessSnapshot, error)
	CloseTimeout      time.Duration
	ReconnectTimeout  time.Duration
}

type qqProcessFingerprint struct {
	PID            int
	ParentPID      int
	ProcessName    string
	ExecutablePath string
	Depth          int
}

func qqTreeFingerprints(rootPID int, protectedParentPID int, snapshots []HostProcessSnapshot) ([]qqProcessFingerprint, error) {
	seenPIDs := make(map[int]bool, len(snapshots))
	for _, snapshot := range snapshots {
		if seenPIDs[snapshot.PID] {
			return nil, fmt.Errorf("duplicate process PID %d", snapshot.PID)
		}
		seenPIDs[snapshot.PID] = true
	}
	result := []qqProcessFingerprint{}
	for _, snapshot := range snapshots {
		depth, ok := qqTreeDepth(snapshot.PID, rootPID, protectedParentPID, snapshots)
		if !ok {
			continue
		}
		result = append(result, qqProcessFingerprint{
			PID:            snapshot.PID,
			ParentPID:      snapshot.ParentPID,
			ProcessName:    snapshot.ProcessName,
			ExecutablePath: snapshot.ExecutablePath,
			Depth:          depth,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Depth == result[j].Depth {
			return result[i].PID < result[j].PID
		}
		return result[i].Depth > result[j].Depth
	})
	return result, nil
}

func qqTreeDepth(pid int, rootPID int, protectedParentPID int, snapshots []HostProcessSnapshot) (int, bool) {
	if pid == rootPID {
		return 0, true
	}
	if pid == protectedParentPID {
		return 0, false
	}
	parents := make(map[int]int, len(snapshots))
	for _, snapshot := range snapshots {
		parents[snapshot.PID] = snapshot.ParentPID
	}
	seen := map[int]bool{}
	depth := 0
	for current := pid; current > 0 && !seen[current]; current = parents[current] {
		if current == protectedParentPID {
			return 0, false
		}
		seen[current] = true
		parent, ok := parents[current]
		if !ok {
			return 0, false
		}
		depth++
		if parent == rootPID {
			return depth, true
		}
		if parent == protectedParentPID {
			return 0, false
		}
	}
	return 0, false
}

func qqFingerprintMatches(fingerprint qqProcessFingerprint, snapshot HostProcessSnapshot) bool {
	return fingerprint.PID == snapshot.PID &&
		fingerprint.ParentPID == snapshot.ParentPID &&
		strings.EqualFold(strings.TrimSpace(fingerprint.ProcessName), strings.TrimSpace(snapshot.ProcessName)) &&
		strings.EqualFold(strings.TrimSpace(fingerprint.ExecutablePath), strings.TrimSpace(snapshot.ExecutablePath))
}

func qqMarkMissingProcessed(authorized []qqProcessFingerprint, processed map[int]bool, snapshots []HostProcessSnapshot) {
	currentPIDs := make(map[int]bool, len(snapshots))
	for _, snapshot := range snapshots {
		currentPIDs[snapshot.PID] = true
	}
	for _, fingerprint := range authorized {
		if !currentPIDs[fingerprint.PID] {
			processed[fingerprint.PID] = true
		}
	}
}

func qqRejectUnexpectedDescendants(rootPID int, protectedParentPID int, authorized []qqProcessFingerprint, processed map[int]bool, snapshots []HostProcessSnapshot) error {
	authorizedPIDs := make(map[int]bool, len(authorized))
	initialParents := make(map[int]int, len(authorized))
	for _, fingerprint := range authorized {
		authorizedPIDs[fingerprint.PID] = true
		initialParents[fingerprint.PID] = fingerprint.ParentPID
	}
	currentParents := make(map[int]int, len(snapshots))
	for _, snapshot := range snapshots {
		if _, exists := currentParents[snapshot.PID]; exists {
			return fmt.Errorf("duplicate process PID %d during cleanup", snapshot.PID)
		}
		currentParents[snapshot.PID] = snapshot.ParentPID
	}
	for _, snapshot := range snapshots {
		if authorizedPIDs[snapshot.PID] {
			if processed[snapshot.PID] {
				return fmt.Errorf("processed PID %d reappeared during cleanup", snapshot.PID)
			}
			continue
		}
		if snapshot.PID == protectedParentPID {
			continue
		}
		if qqDescendsFromOldRoot(snapshot.PID, rootPID, protectedParentPID, currentParents, initialParents) {
			return fmt.Errorf("unauthorized PID %d joined the old QQ miniapp tree", snapshot.PID)
		}
	}
	return nil
}

func qqDescendsFromOldRoot(pid int, rootPID int, protectedParentPID int, currentParents map[int]int, initialParents map[int]int) bool {
	seen := map[int]bool{}
	for current := pid; current > 0 && !seen[current]; {
		if current == protectedParentPID {
			return false
		}
		seen[current] = true
		parent, ok := currentParents[current]
		if !ok {
			parent, ok = initialParents[current]
		}
		if !ok {
			return false
		}
		if parent == rootPID {
			return true
		}
		if parent == protectedParentPID {
			return false
		}
		current = parent
	}
	return false
}

type qqCleanupResult struct {
	Stopped   bool
	Processed map[int]bool
}

func validateQQClosedObservation(rootPID int, authorized []qqProcessFingerprint, snapshots []HostProcessSnapshot) error {
	processed := make(map[int]bool, len(authorized))
	for _, fingerprint := range authorized {
		processed[fingerprint.PID] = true
	}
	protectedParentPID := authorized[len(authorized)-1].ParentPID
	return qqRejectUnexpectedDescendants(rootPID, protectedParentPID, authorized, processed, snapshots)
}

func stopVerifiedQQTree(request QQRestartRequest, rootPID int, authorized []qqProcessFingerprint, snapshots []HostProcessSnapshot) (qqCleanupResult, error) {
	result := qqCleanupResult{Processed: make(map[int]bool, len(authorized))}
	if request.StopPID == nil || request.RefreshSnapshots == nil {
		return result, errors.New("verified process cleanup callbacks are required")
	}
	if len(authorized) == 0 || authorized[len(authorized)-1].PID != rootPID {
		return result, errors.New("authorized QQ miniapp root fingerprint was not found")
	}
	timedOutRoot, ok := findSnapshotByPID(snapshots, rootPID)
	if !ok || !qqFingerprintMatches(authorized[len(authorized)-1], timedOutRoot) {
		return result, errors.New("verified QQ miniapp root identity changed before cleanup")
	}
	preflightSnapshots, err := request.RefreshSnapshots()
	if err != nil {
		return result, err
	}
	protectedParentPID := authorized[len(authorized)-1].ParentPID
	qqMarkMissingProcessed(authorized, result.Processed, preflightSnapshots)
	if err := qqRejectUnexpectedDescendants(rootPID, protectedParentPID, authorized, result.Processed, preflightSnapshots); err != nil {
		return result, err
	}
	for _, fingerprint := range authorized {
		current, ok := findSnapshotByPID(preflightSnapshots, fingerprint.PID)
		if !ok {
			continue
		}
		if strings.TrimSpace(fingerprint.ExecutablePath) == "" || strings.TrimSpace(current.ExecutablePath) == "" {
			return result, fmt.Errorf("PID %d executable path is unavailable before cleanup", fingerprint.PID)
		}
	}
	for _, fingerprint := range authorized {
		currentSnapshots, err := request.RefreshSnapshots()
		if err != nil {
			return result, err
		}
		qqMarkMissingProcessed(authorized, result.Processed, currentSnapshots)
		if err := qqRejectUnexpectedDescendants(rootPID, protectedParentPID, authorized, result.Processed, currentSnapshots); err != nil {
			return result, err
		}
		current, ok := findSnapshotByPID(currentSnapshots, fingerprint.PID)
		if !ok {
			result.Processed[fingerprint.PID] = true
			continue
		}
		if fingerprint.PID != rootPID {
			currentRoot, ok := findSnapshotByPID(currentSnapshots, rootPID)
			if !ok || !qqFingerprintMatches(authorized[len(authorized)-1], currentRoot) {
				return result, errors.New("verified QQ miniapp root identity changed during cleanup")
			}
		}
		if !qqFingerprintMatches(fingerprint, current) {
			return result, fmt.Errorf("PID %d identity changed before cleanup", fingerprint.PID)
		}
		if strings.TrimSpace(fingerprint.ExecutablePath) == "" || strings.TrimSpace(current.ExecutablePath) == "" {
			return result, fmt.Errorf("PID %d executable path is unavailable before cleanup", fingerprint.PID)
		}
		if fingerprint.PID != rootPID {
			parents := make(map[int]int, len(currentSnapshots))
			for _, snapshot := range currentSnapshots {
				parents[snapshot.PID] = snapshot.ParentPID
			}
			if !processDescendsFrom(fingerprint.PID, rootPID, parents) {
				return result, fmt.Errorf("PID %d left the verified QQ miniapp tree", fingerprint.PID)
			}
		}
		if err := request.StopPID(fingerprint.PID); err != nil {
			return result, fmt.Errorf("stop PID %d: %w", fingerprint.PID, err)
		}
		result.Stopped = true
		result.Processed[fingerprint.PID] = true
	}
	finalSnapshots, err := request.RefreshSnapshots()
	if err != nil {
		return result, err
	}
	qqMarkMissingProcessed(authorized, result.Processed, finalSnapshots)
	if err := qqRejectUnexpectedDescendants(rootPID, protectedParentPID, authorized, result.Processed, finalSnapshots); err != nil {
		return result, err
	}
	return result, nil
}

func RestartQQMiniapp(request QQRestartRequest) (RestartResult, error) {
	if request.Registry == nil {
		return RestartResult{}, errors.New("host binding registry is required")
	}
	owner := normalizeOwner(request.Owner)
	result := RestartResult{Owner: owner, Candidates: QQMiniappCandidates(request.Snapshots)}
	fail := func(stage string, err error) (RestartResult, error) {
		wrapped := fmt.Errorf("%s: %w", stage, err)
		result.Status = "restart_failed"
		result.Reason = wrapped.Error()
		return result, wrapped
	}

	if len(result.Candidates) > 1 {
		return fail("qq_miniapp_unverified", errors.New("multiple strict QQ miniapp roots found"))
	}
	if len(result.Candidates) == 0 {
		if request.RuntimeConnected {
			return fail("qq_miniapp_unverified", errors.New("runtime is connected without a strict QQ miniapp root"))
		}
		request.Registry.Clear(owner)
		return launchAndBindQQMiniapp(request, result, 0, 0)
	}

	current := result.Candidates[0]
	binding, err := request.Registry.Bind(owner, current)
	if err != nil {
		return fail("binding_refresh_failed", err)
	}
	result.OldPID = binding.PID
	result.Binding = &binding
	currentSnapshot, ok := findSnapshotByPID(request.Snapshots, binding.PID)
	if !ok {
		return fail("qq_miniapp_unverified", errors.New("selected QQ miniapp root disappeared"))
	}
	authorizedTree, err := qqTreeFingerprints(binding.PID, binding.ParentPID, request.Snapshots)
	if err != nil {
		return fail("qq_miniapp_unverified", err)
	}
	if len(authorizedTree) == 0 || authorizedTree[len(authorizedTree)-1].PID != binding.PID {
		return fail("qq_miniapp_unverified", errors.New("authorized QQ miniapp tree fingerprint was not found"))
	}
	if request.CloseWindows == nil {
		return fail("close_failed", errors.New("close windows callback is required"))
	}
	if err := request.CloseWindows(currentSnapshot.Windows); err != nil {
		return fail("close_failed", err)
	}
	if request.WaitForClosed == nil {
		return fail("disconnect_timeout", errors.New("close observer is required"))
	}
	observation, err := request.WaitForClosed(binding.PID, request.CloseTimeout)
	if err != nil {
		return fail("disconnect_timeout", err)
	}
	if !observation.TreeExited {
		cleanup, cleanupErr := stopVerifiedQQTree(request, binding.PID, authorizedTree, observation.Snapshots)
		result.Stopped = cleanup.Stopped
		if cleanupErr != nil {
			return fail("old_tree_cleanup_failed", cleanupErr)
		}
		observation, err = request.WaitForClosed(binding.PID, request.CloseTimeout)
		if err != nil {
			return fail("old_tree_exit_timeout", err)
		}
	}
	if !observation.TreeExited {
		return fail("old_tree_exit_timeout", errors.New("old QQ miniapp subtree is still running"))
	}
	if err := validateQQClosedObservation(binding.PID, authorizedTree, observation.Snapshots); err != nil {
		return fail("old_tree_cleanup_failed", err)
	}
	request.Registry.Clear(owner)
	if !observation.Disconnected {
		return fail("disconnect_timeout", errors.New("old QQ WS session is still connected"))
	}
	return launchAndBindQQMiniapp(request, result, binding.PID, binding.ParentPID)
}

func launchAndBindQQMiniapp(request QQRestartRequest, result RestartResult, oldPID int, expectedParentPID int) (RestartResult, error) {
	fail := func(stage string, err error) (RestartResult, error) {
		wrapped := fmt.Errorf("%s: %w", stage, err)
		result.Status = "restart_failed"
		result.Reason = wrapped.Error()
		return result, wrapped
	}
	if request.Launch == nil {
		return fail("launch_failed", errors.New("launch callback is required"))
	}
	launchRequest, err := launchRequestForPlatform("qq", result.Binding)
	if err != nil {
		return fail("launch_failed", err)
	}
	if err := request.Launch(launchRequest); err != nil {
		return fail("launch_failed", err)
	}
	result.LaunchDispatched = true
	if request.WaitForReady == nil {
		return fail("reconnect_timeout", errors.New("ready observer is required"))
	}
	ready := request.WaitForReady(request.RuntimeInstanceID, request.ReconnectTimeout)
	if !ready.Accepted || strings.TrimSpace(ready.Target) != "qq_ws" {
		return fail("reconnect_timeout", errors.New("replacement QQ WS readiness was not accepted"))
	}
	if !ready.Connected || !ready.Ready {
		return fail("reconnect_timeout", errors.New("replacement QQ WS session did not become ready"))
	}
	if strings.TrimSpace(ready.InstanceID) == "" || (request.RuntimeInstanceID != "" && ready.InstanceID == request.RuntimeInstanceID) {
		return fail("new_instance_not_observed", errors.New("replacement QQ WS instance ID was not observed"))
	}
	if request.RefreshSnapshots == nil {
		return fail("new_process_not_found", errors.New("snapshot refresh callback is required"))
	}
	refreshed, err := request.RefreshSnapshots()
	if err != nil {
		return fail("new_process_not_found", err)
	}
	candidates := QQMiniappCandidates(refreshed)
	result.Candidates = candidates
	if oldPID > 0 {
		for _, candidate := range candidates {
			if candidate.PID == oldPID {
				return fail("new_process_not_found", errors.New("old QQ miniapp root is still present"))
			}
		}
	}
	filtered := make([]HostProcessCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if expectedParentPID > 0 && candidate.ParentPID != expectedParentPID {
			continue
		}
		filtered = append(filtered, candidate)
	}
	result.Candidates = filtered
	if len(filtered) != 1 {
		return fail("new_process_not_found", fmt.Errorf("strict replacement candidates: %d", len(filtered)))
	}
	binding, err := request.Registry.Bind(result.Owner, filtered[0])
	if err != nil {
		return fail("binding_refresh_failed", err)
	}
	result.Status = RestartStatusReconnected
	result.Reason = "QQ WS reconnected and became ready"
	result.Binding = &binding
	return result, nil
}
