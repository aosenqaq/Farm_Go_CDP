package desktop

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Farm_Go/internal/eventbus"
	"Farm_Go/internal/farm"
	"Farm_Go/internal/farm/social"
)

type fakeRuntimePollService struct {
	authorized    bool
	dashboardTabs []string
	landRevisions []string
	dogGuardReads int
}

func (s *fakeRuntimePollService) Authorize() error {
	if !s.authorized {
		return errors.New("poll unauthorized")
	}
	return nil
}

func (s *fakeRuntimePollService) Dashboard(activeTab string) dashboardPollResponse {
	s.dashboardTabs = append(s.dashboardTabs, activeTab)
	return dashboardPollResponse{
		Events:     []eventbus.Event{},
		TSDKEvents: []eventbus.Event{},
	}
}

func (s *fakeRuntimePollService) Land(revision string) farm.LandDetailsDeltaPayload {
	s.landRevisions = append(s.landRevisions, revision)
	return farm.LandDetailsDeltaPayload{Full: revision == "", Revision: "r1", Lands: []farm.LandDetailsItem{}, RemovedLandIDs: []int{}}
}

func (s *fakeRuntimePollService) DogGuard() social.DogGuardState {
	s.dogGuardReads++
	return social.DogGuardState{Results: []social.DogGuardRow{}}
}

func TestRuntimePollHandlerRejectsUnsupportedRequests(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		want   int
	}{
		{name: "method", method: http.MethodPost, target: "/farm-api/poll/dashboard?activeTab=workspace", want: http.StatusMethodNotAllowed},
		{name: "route", method: http.MethodGet, target: "/farm-api/poll/unknown", want: http.StatusNotFound},
		{name: "tab", method: http.MethodGet, target: "/farm-api/poll/dashboard?activeTab=invalid", want: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.target, nil)
			newRuntimePollHandler(&fakeRuntimePollService{authorized: true}).ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, test.want, recorder.Body.String())
			}
			if recorder.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control = %q", recorder.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestRuntimePollHandlerRejectsUnauthorizedRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/farm-api/poll/dashboard?activeTab=workspace", nil)
	newRuntimePollHandler(&fakeRuntimePollService{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.Len() > 256 {
		t.Fatalf("unauthorized response is unbounded: %d bytes", recorder.Body.Len())
	}
}

func TestWritePollJSONReturnsBoundedEncodingError(t *testing.T) {
	recorder := httptest.NewRecorder()
	writePollJSON(recorder, http.StatusOK, map[string]any{"unsupported": make(chan int)})
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.Len() > 256 {
		t.Fatalf("encoding error response is unbounded: %d bytes", recorder.Body.Len())
	}
}

func TestAppRuntimePollDashboardScopesRunStatisticsToWorkspace(t *testing.T) {
	app, _ := newAppWithFakeRuntime(t, map[string]any{})
	service := newAppRuntimePollService(app)
	workspace := service.Dashboard("workspace")
	if workspace.RunStatistics == nil {
		t.Fatal("workspace response omitted run statistics")
	}
	settings := service.Dashboard("settings")
	if settings.RunStatistics != nil {
		t.Fatalf("settings response included run statistics: %#v", settings.RunStatistics)
	}
	if workspace.Status.Target != "qq_ws" || workspace.Events == nil || workspace.TSDKEvents == nil || workspace.AutomationState.FeatureGroups == nil {
		t.Fatalf("incomplete dashboard response: %#v", workspace)
	}
}

func TestAppRuntimePollDashboardReturnsIndependentEventFeeds(t *testing.T) {
	app := newAuthorizedTestApp(t)
	for index := 0; index < 105; index++ {
		app.recordRuntimeLogEvent(eventbus.Event{Type: "qqhost.log", Message: "[TSDK-BLOCK] ready"})
	}
	for index := 0; index < 125; index++ {
		app.recordEvent(eventbus.Event{Type: "ordinary", Message: "ordinary"})
	}

	response := newAppRuntimePollService(app).Dashboard("settings")
	if len(response.Events) != 120 {
		t.Fatalf("ordinary event count = %d, want 120", len(response.Events))
	}
	if len(response.TSDKEvents) != 100 {
		t.Fatalf("TSDK event count = %d, want 100", len(response.TSDKEvents))
	}
	for _, event := range response.Events {
		if isTSDKBlockRuntimeEvent(event) {
			t.Fatalf("ordinary dashboard feed contains TSDK event: %#v", event)
		}
	}
	for _, event := range response.TSDKEvents {
		if !isTSDKBlockRuntimeEvent(event) {
			t.Fatalf("TSDK dashboard feed contains ordinary event: %#v", event)
		}
	}
}

func TestAppRuntimePollLandDelegatesRevision(t *testing.T) {
	app, _ := newAppWithFakeRuntime(t, map[string]any{
		"gameCtl.getFarmStatus": map[string]any{
			"farmType":   "own",
			"totalGrids": 0,
			"grids":      []any{},
		},
	})
	service := newAppRuntimePollService(app)
	first := service.Land("")
	if !first.Full || first.Revision == "" {
		t.Fatalf("first land response = %#v", first)
	}
	second := service.Land(first.Revision)
	if second.Full || second.Revision != first.Revision {
		t.Fatalf("unchanged land response = %#v", second)
	}
}

func TestAppRuntimePollDogGuardReturnsResults(t *testing.T) {
	service := newAppRuntimePollService(newAuthorizedTestApp(t))
	if state := service.DogGuard(); state.Results == nil {
		t.Fatalf("dog guard response omitted results: %#v", state)
	}
}

func TestAppRuntimePollAuthorizeAllowsUnlicensedApp(t *testing.T) {
	service := newAppRuntimePollService(NewApp())
	if err := service.Authorize(); err != nil {
		t.Fatalf("local app should not require a license: %v", err)
	}
}

func TestAssetMiddlewareRoutesOnlyFarmPollPrefix(t *testing.T) {
	app := newAuthorizedTestApp(t)
	fallthroughCount := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallthroughCount++
		w.WriteHeader(http.StatusNoContent)
	})
	handler := newAssetMiddleware(app)(next)

	pollRecorder := httptest.NewRecorder()
	handler.ServeHTTP(pollRecorder, httptest.NewRequest(http.MethodGet, "/farm-api/poll/dog-guard", nil))
	if pollRecorder.Code != http.StatusOK || fallthroughCount != 0 {
		t.Fatalf("poll status=%d fallthrough=%d", pollRecorder.Code, fallthroughCount)
	}

	assetRecorder := httptest.NewRecorder()
	handler.ServeHTTP(assetRecorder, httptest.NewRequest(http.MethodGet, "/index.html", nil))
	if assetRecorder.Code != http.StatusNoContent || fallthroughCount != 1 {
		t.Fatalf("asset status=%d fallthrough=%d", assetRecorder.Code, fallthroughCount)
	}
}
