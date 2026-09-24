package desktop

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"Farm_Go/internal/eventbus"
	"Farm_Go/internal/farm"
	"Farm_Go/internal/farm/automation"
	"Farm_Go/internal/farm/social"
	farmruntime "Farm_Go/internal/runtime"
	"Farm_Go/internal/runtime/guard"
)

const runtimePollPrefix = "/farm-api/poll/"

var runtimePollTabs = map[string]struct{}{
	"workspace": {}, "automation": {}, "assets": {}, "social": {},
	"account": {}, "guard": {}, "logs": {}, "message_push": {}, "settings": {},
}

type runtimePollService interface {
	Authorize() error
	Dashboard(activeTab string) dashboardPollResponse
	Land(revision string) farm.LandDetailsDeltaPayload
	DogGuard() social.DogGuardState
}

type dashboardPollResponse struct {
	Status          farmruntime.Status        `json:"status"`
	GuardStatus     GuardianStatusDTO         `json:"guardStatus"`
	BindingStatus   guard.AutoBindOwnerResult `json:"bindingStatus"`
	Events          []eventbus.Event          `json:"events"`
	TSDKEvents      []eventbus.Event          `json:"tsdkEvents"`
	AutomationState automation.State          `json:"automationState"`
	RunStatistics   *farm.RunStatistics       `json:"runStatistics,omitempty"`
	PatchStatus     QQDebugPatchStatus        `json:"patchStatus"`
}

type appRuntimePollService struct{ app *App }

func newAppRuntimePollService(app *App) runtimePollService {
	return &appRuntimePollService{app: app}
}

func (s *appRuntimePollService) Authorize() error {
	return nil
}

func (s *appRuntimePollService) Dashboard(activeTab string) dashboardPollResponse {
	response := dashboardPollResponse{
		Status:          s.app.RuntimeStatus(),
		GuardStatus:     s.app.GuardianStatus(),
		BindingStatus:   s.app.AutoBindHostProcess(),
		Events:          s.app.RuntimeEvents(120),
		TSDKEvents:      s.app.TSDKRuntimeEvents(100),
		AutomationState: s.app.FarmAutomationState(),
		PatchStatus:     s.app.QQDebugPatchStatus(),
	}
	if activeTab == "workspace" {
		statistics := s.app.FarmWorkspaceRunStatistics()
		response.RunStatistics = &statistics
	}
	return response
}

func (s *appRuntimePollService) Land(revision string) farm.LandDetailsDeltaPayload {
	return s.app.FarmLandDetailsSince(revision)
}

func (s *appRuntimePollService) DogGuard() social.DogGuardState {
	return s.app.FarmSocialDogGuardState()
}

func writePollJSON(w http.ResponseWriter, status int, value any) {
	var payload bytes.Buffer
	if err := json.NewEncoder(&payload).Encode(value); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("{\"error\":\"poll response encoding failed\"}\n"))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(payload.Bytes())
}

func newRuntimePollHandler(service runtimePollService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writePollJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		resource := strings.TrimPrefix(r.URL.Path, runtimePollPrefix)
		if resource != "dashboard" && resource != "land" && resource != "dog-guard" {
			writePollJSON(w, http.StatusNotFound, map[string]string{"error": "poll resource not found"})
			return
		}
		if err := service.Authorize(); err != nil {
			writePollJSON(w, http.StatusUnauthorized, map[string]string{"error": "authorization required"})
			return
		}
		switch resource {
		case "dashboard":
			activeTab := r.URL.Query().Get("activeTab")
			if _, ok := runtimePollTabs[activeTab]; !ok {
				writePollJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid activeTab"})
				return
			}
			writePollJSON(w, http.StatusOK, service.Dashboard(activeTab))
		case "land":
			writePollJSON(w, http.StatusOK, service.Land(r.URL.Query().Get("revision")))
		case "dog-guard":
			writePollJSON(w, http.StatusOK, service.DogGuard())
		}
	})
}
