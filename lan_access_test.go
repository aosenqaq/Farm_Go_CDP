package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"Farm_Go/internal/farm"
	"Farm_Go/internal/lanaccess"
	"Farm_Go/internal/storage"
)

const lanAccessTestPassword = "correct horse battery staple"

func TestLANAccessDisplayAddressUsesPrivateIPv4ForLAN(t *testing.T) {
	got := lanAccessDisplayAddress(lanaccess.ModeLAN, 8788, []netip.Addr{
		netip.MustParseAddr("127.0.0.1"),
		netip.MustParseAddr("192.168.12.34"),
	})
	if got != "192.168.12.34:8788" {
		t.Fatalf("LAN address = %q", got)
	}
}

func TestLANAccessDisplayAddressUsesLoopbackForTunnel(t *testing.T) {
	got := lanAccessDisplayAddress(lanaccess.ModeTunnel, 8788, []netip.Addr{
		netip.MustParseAddr("192.168.12.34"),
	})
	if got != "127.0.0.1:8788" {
		t.Fatalf("tunnel address = %q", got)
	}
}

func TestLANAccessHTTPServiceSmoke(t *testing.T) {
	app := newLANAccessTestApp(t)
	status := enableLANAccess(t, app, lanaccess.ModeLAN)
	baseURL := "http://" + net.JoinHostPort("127.0.0.1", itoa(status.Port))
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second}

	postRPC := func(csrf string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, baseURL+"/api/rpc/FarmAutomationState", strings.NewReader(`{"args":[]}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", baseURL)
		if csrf != "" {
			req.Header.Set("X-Farm-Go-CSRF", csrf)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	unauthenticated := postRPC("")
	if unauthenticated.StatusCode != http.StatusUnauthorized {
		unauthenticated.Body.Close()
		t.Fatalf("unauthenticated RPC status = %d, want %d", unauthenticated.StatusCode, http.StatusUnauthorized)
	}
	unauthenticated.Body.Close()

	loginRequest, err := http.NewRequest(http.MethodPost, baseURL+"/api/auth/login", strings.NewReader(`{"password":"`+lanAccessTestPassword+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	loginRequest.Header.Set("Content-Type", "application/json")
	loginResponse, err := client.Do(loginRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer loginResponse.Body.Close()
	if loginResponse.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginResponse.StatusCode, http.StatusOK)
	}
	var session struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.NewDecoder(loginResponse.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if session.CSRFToken == "" {
		t.Fatal("login response did not contain a CSRF token")
	}

	missingCSRF := postRPC("")
	if missingCSRF.StatusCode != http.StatusForbidden {
		missingCSRF.Body.Close()
		t.Fatalf("RPC without CSRF status = %d, want %d", missingCSRF.StatusCode, http.StatusForbidden)
	}
	missingCSRF.Body.Close()

	authenticated := postRPC(session.CSRFToken)
	defer authenticated.Body.Close()
	if authenticated.StatusCode != http.StatusOK {
		t.Fatalf("authenticated RPC status = %d, want %d", authenticated.StatusCode, http.StatusOK)
	}
}

func TestLANAccessHTTPServiceSavesAutomationState(t *testing.T) {
	app := newLANAccessTestApp(t)
	status := enableLANAccess(t, app, lanaccess.ModeLAN)
	baseURL := "http://" + net.JoinHostPort("127.0.0.1", itoa(status.Port))
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second}

	loginRequest, err := http.NewRequest(http.MethodPost, baseURL+"/api/auth/login", strings.NewReader(`{"password":"`+lanAccessTestPassword+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	loginRequest.Header.Set("Content-Type", "application/json")
	loginResponse, err := client.Do(loginRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer loginResponse.Body.Close()
	if loginResponse.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginResponse.StatusCode, http.StatusOK)
	}
	var session struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.NewDecoder(loginResponse.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}

	state := app.FarmAutomationState()
	state.Config["autoFarmOneClickEnabled"] = !state.Config["autoFarmOneClickEnabled"].(bool)
	payload, err := json.Marshal(map[string]any{"args": []any{state}})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, baseURL+"/api/rpc/SaveFarmAutomationState", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", baseURL)
	request.Header.Set("X-Farm-Go-CSRF", session.CSRFToken)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("save status = %d, want %d: %s", response.StatusCode, http.StatusOK, body)
	}
	var saved struct {
		Config map[string]any `json:"config"`
	}
	if err := json.NewDecoder(response.Body).Decode(&saved); err != nil {
		t.Fatal(err)
	}
	if got, want := saved.Config["autoFarmOneClickEnabled"], state.Config["autoFarmOneClickEnabled"]; got != want {
		t.Fatalf("saved autoFarmOneClickEnabled = %#v, want %#v", got, want)
	}
}

func TestLANAccessHTTPServiceServesRegisteredGameConfigImage(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Plant.json"), []byte(`[{"id":1020002,"name":"白萝卜","fruit":{"id":40002},"seed_id":20002}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	imagePath := filepath.Join(root, "plant_images", "stages", "作物", "白萝卜", "白萝卜_02_发芽.png")
	if err := os.MkdirAll(filepath.Dir(imagePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	previousRoot := farm.DefaultGameConfigRoot()
	farm.SetGameConfigRoot(root)
	t.Cleanup(func() { farm.SetGameConfigRoot(previousRoot) })
	land := farm.BuildRuntimeLandDetails(map[string]any{
		"grids": []any{map[string]any{
			"landId": 1, "seedId": 20002, "plantName": "白萝卜", "currentStage": 2, "phaseName": "发芽",
		}},
	})
	if len(land.Lands) != 1 {
		t.Fatalf("expected registered land image, got %#v", land.Lands)
	}
	imageURL := land.Lands[0].ImageURL
	if !strings.HasPrefix(imageURL, "/farm-assets/") {
		t.Fatalf("expected registered image URL, got %q", imageURL)
	}

	app := newLANAccessTestApp(t)

	status := enableLANAccess(t, app, lanaccess.ModeLAN)
	baseURL := "http://" + net.JoinHostPort("127.0.0.1", itoa(status.Port))
	response, err := (&http.Client{Timeout: 5 * time.Second}).Get(baseURL + imageURL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "image/png" || len(body) == 0 {
		t.Fatalf("unexpected LAN image response: status=%d type=%q bytes=%d", response.StatusCode, response.Header.Get("Content-Type"), len(body))
	}

	missing, err := (&http.Client{Timeout: 5 * time.Second}).Get(baseURL + "/farm-assets/missing")
	if err != nil {
		t.Fatal(err)
	}
	defer missing.Body.Close()
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing image status=%d", missing.StatusCode)
	}
}

func newLANAccessTestApp(t *testing.T) *App {
	t.Helper()
	app := newAuthorizedTestApp(t)
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app.store = store
	t.Cleanup(func() { app.shutdown(context.Background()) })
	return app
}

func enableLANAccess(t *testing.T, app *App, mode lanaccess.Mode) LANAccessStatus {
	t.Helper()
	status, err := app.SaveLANAccessSettings(LANAccessSettingsInput{
		Enabled:         true,
		Mode:            string(mode),
		Port:            freeTCPPort(t),
		Password:        lanAccessTestPassword,
		ConfirmPassword: lanAccessTestPassword,
	})
	if err != nil {
		t.Fatalf("enable LAN access: %v", err)
	}
	if !status.Running {
		t.Fatalf("LAN server did not start: %#v", status)
	}
	return status
}

func TestSaveLANAccessSettingsKeepsRunningServerWhenCandidatePortIsBusy(t *testing.T) {
	app := newLANAccessTestApp(t)
	old := enableLANAccess(t, app, lanaccess.ModeLAN)
	blocker, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()

	_, err = app.SaveLANAccessSettings(LANAccessSettingsInput{
		Enabled: true,
		Mode:    string(lanaccess.ModeLAN),
		Port:    blocker.Addr().(*net.TCPAddr).Port,
	})
	if err == nil {
		t.Fatal("busy port accepted")
	}
	if got := app.LANAccessSettings(); got.Port != old.Port || !got.Running {
		t.Fatalf("old server lost: %#v", got)
	}
}

func TestSaveLANAccessSettingsKeepsRunningServerWhenPersistenceFails(t *testing.T) {
	app := newLANAccessTestApp(t)
	old := enableLANAccess(t, app, lanaccess.ModeLAN)
	if err := app.store.Close(); err != nil {
		t.Fatal(err)
	}
	candidatePort := freeTCPPort(t)

	_, err := app.SaveLANAccessSettings(LANAccessSettingsInput{
		Enabled: true,
		Mode:    string(lanaccess.ModeLAN),
		Port:    candidatePort,
	})
	if err == nil {
		t.Fatal("closed storage accepted settings")
	}
	if got := app.LANAccessSettings(); got.Port != old.Port || !got.Running {
		t.Fatalf("old server lost: %#v", got)
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", itoa(candidatePort)))
	if err != nil {
		t.Fatalf("candidate listener remained open: %v", err)
	}
	listener.Close()
}

func TestStartupRestoresLANAccessAndShutdownStopsIt(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	store, err := storage.Open(ctx, filepath.Join(dataDir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	port := freeTCPPort(t)
	if err := store.SaveLANAccessSettings(ctx, storage.LANAccessSettings{
		Enabled:      true,
		Mode:         storage.LANAccessModeLAN,
		Port:         port,
		PasswordHash: "configured-password-hash",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.authorizationForTests = true
	app.dataDir = dataDir
	app.startup(ctx)
	status := app.LANAccessSettings()
	if !status.Enabled || !status.PasswordConfigured || !status.Running || status.Port != port {
		t.Fatalf("startup did not restore LAN access: %#v", status)
	}

	app.shutdown(ctx)
	if got := app.LANAccessSettings(); got.Running {
		t.Fatalf("shutdown left LAN server running: %#v", got)
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", itoa(port)))
	if err != nil {
		t.Fatalf("shutdown did not release LAN listener: %v", err)
	}
	listener.Close()
}

func TestSaveLANAccessSettingsValidatesPasswordAndDoesNotExposeHash(t *testing.T) {
	app := newLANAccessTestApp(t)

	for _, input := range []LANAccessSettingsInput{
		{Enabled: true, Mode: string(lanaccess.ModeLAN), Port: freeTCPPort(t)},
		{Enabled: true, Mode: string(lanaccess.ModeLAN), Port: freeTCPPort(t), Password: "too short", ConfirmPassword: "too short"},
		{Enabled: true, Mode: string(lanaccess.ModeLAN), Port: freeTCPPort(t), Password: lanAccessTestPassword, ConfirmPassword: "different confirmation"},
	} {
		if _, err := app.SaveLANAccessSettings(input); err == nil {
			t.Fatalf("invalid settings accepted: %#v", input)
		}
	}

	status := enableLANAccess(t, app, lanaccess.ModeTunnel)
	if !status.PasswordConfigured {
		t.Fatalf("password was not configured: %#v", status)
	}
	if _, exposed := reflect.TypeOf(status).FieldByName("PasswordHash"); exposed {
		t.Fatal("LAN status exposes PasswordHash")
	}

	updated, err := app.SaveLANAccessSettings(LANAccessSettingsInput{
		Enabled: true,
		Mode:    string(lanaccess.ModeTunnel),
		Port:    freeTCPPort(t),
	})
	if err != nil {
		t.Fatalf("empty password did not preserve configured hash: %v", err)
	}
	if !updated.PasswordConfigured || !updated.Running {
		t.Fatalf("preserved password state = %#v", updated)
	}
}
