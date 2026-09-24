package guard

import (
	"errors"
	"fmt"
	"time"
)

const RestartStatusReconnected = "reconnected"

type WMPFRestartRequest struct {
	Registry            *HostBindingRegistry
	Owner               string
	Snapshots           []HostProcessSnapshot
	CloseWindows        func([]HostWindowSnapshot) error
	WaitForDisconnected func(time.Duration) bool
	Launch              func(LaunchRequest) error
	WaitForReady        func(time.Duration) bool
	RefreshSnapshots    func() ([]HostProcessSnapshot, error)
	CloseTimeout        time.Duration
	ReconnectTimeout    time.Duration
}

type wmpfRestartProfile struct {
	platform string
	label    string
}

func restartWMPFMiniapp(request WMPFRestartRequest, profile wmpfRestartProfile) (RestartResult, error) {
	if request.Registry == nil {
		return RestartResult{}, errors.New("host binding registry is required")
	}
	owner := normalizeOwner(request.Owner)
	binding, ok := request.Registry.Binding(owner)
	if !ok {
		return RestartResult{}, errors.New("owner has no bound host PID")
	}
	result := RestartResult{Owner: owner, OldPID: binding.PID, Binding: &binding}
	fail := func(err error) (RestartResult, error) {
		result.Status = "restart_failed"
		result.Reason = err.Error()
		return result, err
	}

	preview := PreviewRestart(request.Registry, owner, request.Snapshots)
	if !preview.Allowed {
		return fail(fmt.Errorf("%s restart unavailable: %s", profile.label, preview.Reason))
	}
	currentSnapshot, ok := findSnapshotByPID(request.Snapshots, binding.PID)
	if !ok {
		return fail(fmt.Errorf("bound %s host is no longer running", profile.label))
	}
	if request.CloseWindows == nil {
		return fail(errors.New("close windows callback is required"))
	}
	if err := request.CloseWindows(currentSnapshot.Windows); err != nil {
		return fail(fmt.Errorf("close %s miniapp window: %w", profile.label, err))
	}
	if request.WaitForDisconnected == nil {
		return fail(errors.New("disconnect waiter is required"))
	}
	if !request.WaitForDisconnected(request.CloseTimeout) {
		return fail(fmt.Errorf("timed out waiting for %s CDP disconnect", profile.label))
	}
	if request.Launch == nil {
		return fail(errors.New("launch callback is required"))
	}
	launchRequest, err := launchRequestForPlatform(profile.platform, &binding)
	if err != nil {
		return fail(err)
	}
	if err := request.Launch(launchRequest); err != nil {
		return fail(fmt.Errorf("launch %s miniapp: %w", profile.label, err))
	}
	result.LaunchDispatched = true
	if request.WaitForReady == nil {
		return fail(errors.New("ready waiter is required"))
	}
	if !request.WaitForReady(request.ReconnectTimeout) {
		return fail(fmt.Errorf("timed out waiting for %s CDP readiness", profile.label))
	}
	if request.RefreshSnapshots == nil {
		return fail(errors.New("snapshot refresh callback is required"))
	}
	refreshedSnapshots, err := request.RefreshSnapshots()
	if err != nil {
		return fail(fmt.Errorf("refresh %s host snapshot: %w", profile.label, err))
	}
	refreshedSnapshot, ok := findSnapshotByPID(refreshedSnapshots, binding.PID)
	if !ok {
		return fail(fmt.Errorf("preserved %s host PID disappeared after reconnect", profile.label))
	}
	refreshedCandidate := candidateFromSnapshot(refreshedSnapshot)
	if !snapshotMatchesPlatform(profile.platform, refreshedSnapshot) || !hostIdentityMatches(binding, refreshedCandidate) {
		return fail(fmt.Errorf("preserved %s host identity changed after reconnect", profile.label))
	}
	if !hasMiniappWindowTitle(refreshedCandidate.WindowTitles) || len(refreshedCandidate.HWNDs) == 0 {
		return fail(fmt.Errorf("reconnected %s miniapp window was not found", profile.label))
	}
	refreshedBinding, err := request.Registry.Bind(owner, refreshedCandidate)
	if err != nil {
		return fail(fmt.Errorf("refresh %s host binding: %w", profile.label, err))
	}
	result.Status = RestartStatusReconnected
	result.Reason = profile.label + " CDP reconnected and became ready"
	result.Binding = &refreshedBinding
	result.Candidates = CandidatesForPlatform(profile.platform, refreshedSnapshots)
	return result, nil
}
