# KAuth License Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a mandatory KAuth card login before Farm_Go starts its operational runtime, keep the session alive with KAuth Pong heartbeats, remember cards with Windows DPAPI when requested, and add non-blocking update checks to System Settings.

**Architecture:** Wails startup becomes a bootstrap phase that opens storage and exposes only license APIs. A session-scoped license manager performs RSA KAuth login, activates the existing operational services, owns heartbeat state, and deactivates everything after three consecutive Pong failures. React mounts the existing application only while the backend reports an authorized session.

**Tech Stack:** Go 1.25, Wails v2, `github.com/kauth-coder/kauth-go` v1.0.0, `golang.org/x/sys/windows`, `github.com/google/uuid`, SQLite, React 18, TypeScript, Vitest, lucide-react, PowerShell.

**Design Reference:** `docs/superpowers/specs/2026-07-11-kauth-license-gate-design.md`

---

## File Structure

### New Go files

- `internal/license/config.go`: build/runtime KAuth and local version configuration.
- `internal/license/config_test.go`: configuration parsing and missing-secret tests.
- `internal/license/types.go`: license phases, safe DTOs, login/update request and response types.
- `internal/license/client.go`: KAuth client and factory interfaces plus SDK adapter.
- `internal/license/client_test.go`: SDK response normalization and error-redaction tests.
- `internal/license/device.go`: platform-neutral device ID provider contract and deterministic UUID helper.
- `internal/license/device_windows.go`: Windows `MachineGuid` reader.
- `internal/license/device_other.go`: unsupported-platform fallback used by cross-platform builds.
- `internal/license/device_test.go`: deterministic UUID and fallback tests.
- `internal/license/credential.go`: remembered-card store composed from DPAPI and SQLite blob storage.
- `internal/license/dpapi_windows.go`: Windows DPAPI protector.
- `internal/license/dpapi_other.go`: unsupported-platform implementation.
- `internal/license/credential_test.go`: remember, replace, delete, and decrypt-failure tests.
- `internal/license/manager.go`: login state machine, generations, session ownership, and heartbeat worker.
- `internal/license/manager_test.go`: login, activation, heartbeat, revoke, and stale-generation tests.
- `internal/license/update.go`: version comparison and download URL validation.
- `internal/license/update_test.go`: update-state tests.
- `license_api.go`: Wails bootstrap/license/update methods and event emission.
- `license_api_test.go`: root App integration tests for login, activation, revoke, and update calls.
- `authorized_lifecycle.go`: idempotent activation/deactivation of existing Farm_Go services.
- `authorized_lifecycle_test.go`: start, rollback, stop-order, and reactivation tests.
- `authorization_gate.go`: backend authorization helpers and explicit bootstrap allowlist.
- `authorization_gate_test.go`: unauthorized-call and exported-method policy tests.
- `internal/license/integration_test.go`: opt-in live KAuth login/Pong/program-detail/logout test.

### Modified Go files

- `go.mod`, `go.sum`: add direct KAuth and UUID dependencies.
- `main.go`: pass build configuration into `NewApp` and keep only root App binding.
- `app.go`: split bootstrap from authorized startup; add license and session fields; reuse deactivation on shutdown.
- `app_test.go`: use an explicitly authorized test constructor for legacy operational tests.
- `internal/storage/settings.go`: persist only DPAPI ciphertext and fallback installation UUID.
- `internal/storage/storage_test.go`: verify license blob settings round trips.
- `scripts/build.ps1`: validate secrets/version, inject Go build variables, and set Wails product version for the build.
- `wails.json`: add checked-in non-secret product metadata/default development version.
- `frontend/wailsjs/go/main/App.js`, `frontend/wailsjs/go/main/App.d.ts`, `frontend/wailsjs/go/models.ts`: regenerated Wails bindings.

### New frontend files

- `frontend/src/AuthorizedApp.tsx`: current operational React application moved out of the bootstrap root.
- `frontend/src/AppBootstrap.test.tsx`: verifies formal UI and polling remain unmounted before authorization.
- `frontend/src/components/LicenseGate.tsx`: focused card entry page.
- `frontend/src/components/LicenseGate.test.tsx`: remembered, loading, keyboard, visibility, and error states.
- `frontend/src/components/LicenseHeartbeatNotice.tsx`: recoverable 1/3 and 2/3 warnings.
- `frontend/src/components/LicenseHeartbeatNotice.test.tsx`: warning rendering tests.
- `frontend/src/components/UpdateAvailableToast.tsx`: non-blocking update reminder.
- `frontend/src/components/UpdateAvailableToast.test.tsx`: reminder action tests.
- `frontend/src/lib/license.ts`: frontend DTO types and state normalization.
- `frontend/src/lib/license.test.ts`: normalization and update-state tests.

### Modified frontend files

- `frontend/src/App.tsx`: become `AppBootstrap`, subscribe to Wails license events, and mount `LicenseGate` or `AuthorizedApp`.
- `frontend/src/App.test.tsx`: move operational helper tests to `AuthorizedApp` and retain root export compatibility only where required.
- `frontend/src/views/SettingsView.tsx`: add application update state and actions.
- `frontend/src/views/SettingsView.test.tsx`: add manual update checks and latest/new/error state coverage.
- `frontend/src/components/AppShell.tsx`: host heartbeat and update notifications without adding navigation.
- `frontend/src/components/AppShell.test.tsx`: notification placement and settings navigation tests.
- `frontend/src/style.css`: focused gate, heartbeat notice, update panel, and responsive states.

## Task 1: Add KAuth and Build Configuration

**Files:**
- Create: `internal/license/config.go`
- Create: `internal/license/config_test.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `wails.json`

- [ ] **Step 1: Write failing configuration tests**

```go
package license

import "testing"

func TestLoadConfigRequiresKAuthValues(t *testing.T) {
	t.Setenv("KAUTH_PROGRAM_ID", "")
	t.Setenv("KAUTH_PROGRAM_SECRET", "")
	t.Setenv("KAUTH_MERCHANT_PUBLIC_KEY", "")
	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected missing KAuth configuration error")
	}
}

func TestLoadConfigParsesVersionAndNeverReturnsSecretInString(t *testing.T) {
	t.Setenv("KAUTH_PROGRAM_ID", "123")
	t.Setenv("KAUTH_PROGRAM_SECRET", "sixteen-byte-key")
	t.Setenv("KAUTH_MERCHANT_PUBLIC_KEY", "public-key-data")
	t.Setenv("FARM_GO_VERSION_NO", "130")
	t.Setenv("FARM_GO_VERSION_NAME", "1.3.0")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProgramID != 123 || cfg.Version.Number != 130 || cfg.Version.Name != "1.3.0" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.String() != "KAuth(program=123, version=1.3.0+130)" {
		t.Fatalf("unsafe or unexpected String output: %q", cfg.String())
	}
}

func TestLoadConfigRejectsInvalidAESSecretLength(t *testing.T) {
	t.Setenv("KAUTH_PROGRAM_ID", "123")
	t.Setenv("KAUTH_PROGRAM_SECRET", "too-short")
	t.Setenv("KAUTH_MERCHANT_PUBLIC_KEY", "public-key-data")
	t.Setenv("FARM_GO_VERSION_NO", "130")
	t.Setenv("FARM_GO_VERSION_NAME", "1.3.0")
	if _, err := LoadConfig(); err == nil { t.Fatal("accepted invalid AES key length") }
}
```

- [ ] **Step 2: Run the tests and verify they fail**

Run: `go test ./internal/license -run TestLoadConfig -v`

Expected: FAIL because `LoadConfig` and `Config` do not exist.

- [ ] **Step 3: Add the dependencies**

Run:

```powershell
go get github.com/kauth-coder/kauth-go@v1.0.0
go get github.com/google/uuid@v1.6.0
```

Expected: `go.mod` lists both modules as direct requirements and `go.sum` contains their checksums.

- [ ] **Step 4: Implement strict configuration loading**

```go
package license

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

var (
	buildProgramID        string
	buildProgramSecret    string
	buildMerchantPublicKey string
	buildVersionNo        string
	buildVersionName      string
)

type Version struct {
	Number int    `json:"number"`
	Name   string `json:"name"`
}

type Config struct {
	APIDomain        string
	ProgramID        int64
	ProgramSecret    string
	MerchantPublicKey string
	Version          Version
}

func LoadConfig() (Config, error) {
	programIDRaw := value("KAUTH_PROGRAM_ID", buildProgramID)
	programSecret := value("KAUTH_PROGRAM_SECRET", buildProgramSecret)
	publicKey := value("KAUTH_MERCHANT_PUBLIC_KEY", buildMerchantPublicKey)
	versionNoRaw := value("FARM_GO_VERSION_NO", buildVersionNo)
	versionName := value("FARM_GO_VERSION_NAME", buildVersionName)
	if programIDRaw == "" || programSecret == "" || publicKey == "" {
		return Config{}, fmt.Errorf("KAuth configuration is incomplete")
	}
	if size := len([]byte(programSecret)); size != 16 && size != 24 && size != 32 {
		return Config{}, fmt.Errorf("KAuth program secret must be a 16, 24, or 32 byte AES key")
	}
	programID, err := strconv.ParseInt(programIDRaw, 10, 64)
	if err != nil || programID <= 0 {
		return Config{}, fmt.Errorf("KAuth program ID is invalid")
	}
	versionNo, err := strconv.Atoi(versionNoRaw)
	if err != nil || versionNo < 0 || versionName == "" {
		return Config{}, fmt.Errorf("Farm_Go version configuration is invalid")
	}
	return Config{
		APIDomain: "https://api.kauth.cn", ProgramID: programID,
		ProgramSecret: programSecret, MerchantPublicKey: publicKey,
		Version: Version{Number: versionNo, Name: versionName},
	}, nil
}

func (c Config) String() string {
	return fmt.Sprintf("KAuth(program=%d, version=%s+%d)", c.ProgramID, c.Version.Name, c.Version.Number)
}

func value(name, fallback string) string {
	if current := strings.TrimSpace(os.Getenv(name)); current != "" { return current }
	return strings.TrimSpace(fallback)
}
```

- [ ] **Step 5: Add non-secret product metadata to `wails.json`**

Add an `info` object with a development version such as `0.0.0`; the release script will temporarily replace it from `FARM_GO_VERSION_NAME` during the build and restore the original file in `finally`.

```json
"info": {
  "companyName": "",
  "productName": "Farm_Go",
  "productVersion": "0.0.0",
  "copyright": "",
  "comments": ""
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/license -run TestLoadConfig -v`

Expected: PASS, with no secret value printed in test output.

- [ ] **Step 7: Commit**

```powershell
git add go.mod go.sum wails.json internal/license/config.go internal/license/config_test.go
git commit -m "feat: add KAuth build configuration"
```

## Task 2: Add Stable Device ID and DPAPI Remembered Cards

**Files:**
- Create: `internal/license/device.go`
- Create: `internal/license/device_windows.go`
- Create: `internal/license/device_other.go`
- Create: `internal/license/device_test.go`
- Create: `internal/license/credential.go`
- Create: `internal/license/dpapi_windows.go`
- Create: `internal/license/dpapi_other.go`
- Create: `internal/license/credential_test.go`
- Modify: `internal/storage/settings.go`
- Modify: `internal/storage/storage_test.go`

- [ ] **Step 1: Write failing storage and credential tests**

```go
func TestLicenseCredentialBlobRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.SaveLicenseCredentialBlob(ctx, "ciphertext"); err != nil { t.Fatal(err) }
	got, err := store.LoadLicenseCredentialBlob(ctx)
	if err != nil || got != "ciphertext" { t.Fatalf("got %q, err %v", got, err) }
	if err := store.DeleteLicenseCredentialBlob(ctx); err != nil { t.Fatal(err) }
	got, err = store.LoadLicenseCredentialBlob(ctx)
	if err != nil || got != "" { t.Fatalf("delete got %q, err %v", got, err) }
}
```

```go
func TestCredentialStoreOnlyPersistsProtectedValue(t *testing.T) {
	blobs := &memoryBlobStore{}
	store := CredentialStore{Blobs: blobs, Protector: prefixProtector{}}
	if err := store.Remember(context.Background(), "card-value"); err != nil { t.Fatal(err) }
	if blobs.value != "protected:card-value" { t.Fatalf("unexpected blob %q", blobs.value) }
	got, remembered, err := store.Load(context.Background())
	if err != nil || !remembered || got != "card-value" { t.Fatalf("got %q %v %v", got, remembered, err) }
}
```

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./internal/storage ./internal/license -run 'LicenseCredential|CredentialStore' -v`

Expected: FAIL because the storage methods and credential types do not exist.

- [ ] **Step 3: Add global SQLite blob methods**

Use the existing `settings` table; do not add a migration or a plaintext card column.

```go
const licenseCredentialBlobKey = "license.card.dpapi.v1"
const licenseInstallIDKey = "license.device.install_id.v1"

func (s *Store) LoadLicenseCredentialBlob(ctx context.Context) (string, error) {
	values, err := s.loadSettingsForAccount(ctx, GlobalSettingsAccountKey, []string{licenseCredentialBlobKey})
	return values[licenseCredentialBlobKey], err
}

func (s *Store) SaveLicenseCredentialBlob(ctx context.Context, value string) error {
	return s.saveSettingForAccount(ctx, GlobalSettingsAccountKey, licenseCredentialBlobKey, value)
}

func (s *Store) DeleteLicenseCredentialBlob(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM settings WHERE account_key = ? AND key = ?`, GlobalSettingsAccountKey, licenseCredentialBlobKey)
	return err
}
```

Add equivalent `LoadLicenseInstallID` and `SaveLicenseInstallID` methods for the fallback UUID.

- [ ] **Step 4: Write deterministic device ID tests**

```go
func TestDeriveDeviceIDIsStableAndNamespaced(t *testing.T) {
	one := DeriveDeviceID("machine-guid")
	two := DeriveDeviceID("machine-guid")
	if one != two { t.Fatalf("device ID changed: %q != %q", one, two) }
	if _, err := uuid.Parse(one); err != nil { t.Fatalf("not UUID: %v", err) }
	if one == "machine-guid" { t.Fatal("raw machine identifier leaked") }
}
```

- [ ] **Step 5: Implement Windows MachineGuid with persisted fallback**

`device_windows.go` uses `golang.org/x/sys/windows/registry` to read `HKLM\SOFTWARE\Microsoft\Cryptography\MachineGuid` with `WOW64_64KEY`. `device.go` derives a UUID with a fixed Farm_Go namespace:

```go
var farmGoDeviceNamespace = uuid.MustParse("4bcad7dc-61d0-5b3d-a8ca-48933bde64b1")

func DeriveDeviceID(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	return uuid.NewSHA1(farmGoDeviceNamespace, []byte(normalized)).String()
}
```

If registry reading fails, load `license.device.install_id.v1`; if absent, generate `uuid.NewString()` and save it. Never log the raw MachineGuid.

- [ ] **Step 6: Implement DPAPI protection**

Use `windows.CryptProtectData` and `windows.CryptUnprotectData` with `CRYPTPROTECT_UI_FORBIDDEN`, copy the returned `DataBlob` into Go memory, then call `windows.LocalFree` for OS-allocated output. Encode ciphertext as Base64 before SQLite storage.

```go
type Protector interface {
	Protect([]byte) ([]byte, error)
	Unprotect([]byte) ([]byte, error)
}

type CredentialStore struct {
	Blobs interface {
		LoadLicenseCredentialBlob(context.Context) (string, error)
		SaveLicenseCredentialBlob(context.Context, string) error
		DeleteLicenseCredentialBlob(context.Context) error
	}
	Protector Protector
}
```

The non-Windows implementation returns `ErrCredentialProtectionUnsupported`; it must never fall back to plaintext.

- [ ] **Step 7: Run focused tests**

Run: `go test ./internal/storage ./internal/license -run 'LicenseCredential|CredentialStore|DeviceID' -v`

Expected: PASS.

- [ ] **Step 8: Run the Windows DPAPI round-trip test**

Add a Windows-only test that protects a random sentinel, verifies ciphertext differs, unprotects it, and calls `Forget`.

Run: `go test ./internal/license -run TestDPAPIRoundTrip -v`

Expected: PASS on Windows.

- [ ] **Step 9: Commit**

```powershell
git add internal/license internal/storage/settings.go internal/storage/storage_test.go
git commit -m "feat: protect remembered license cards with DPAPI"
```

## Task 3: Wrap the KAuth RSA Client and Version Comparison

**Files:**
- Create: `internal/license/types.go`
- Create: `internal/license/client.go`
- Create: `internal/license/client_test.go`
- Create: `internal/license/update.go`
- Create: `internal/license/update_test.go`

- [ ] **Step 1: Write failing adapter and update tests**

```go
func TestParsePongIntervalRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"", "0", "-1", "abc"} {
		if _, err := ParsePongInterval(value); err == nil { t.Fatalf("accepted %q", value) }
	}
}

func TestCompareVersionFindsOptionalUpdate(t *testing.T) {
	state := CompareVersion(Version{Number: 130, Name: "1.3.0"}, ProgramVersion{
		Number: 140, Name: "1.4.0", Description: "Fixes", DownloadURL: "https://example.invalid/download",
	})
	if !state.Available || state.Latest.Number != 140 { t.Fatalf("unexpected state %#v", state) }
}

func TestValidateDownloadURLRejectsExecutableSchemes(t *testing.T) {
	if _, err := ValidateDownloadURL("file:///tmp/app.exe"); err == nil { t.Fatal("accepted file URL") }
}
```

- [ ] **Step 2: Verify tests fail**

Run: `go test ./internal/license -run 'PongInterval|CompareVersion|DownloadURL' -v`

Expected: FAIL with undefined functions/types.

- [ ] **Step 3: Define safe public types**

`types.go` defines `Phase`, `Status`, `LoginRequest`, `LoginResult`, `UserInfo`, `ProgramVersion`, and `UpdateState`. JSON fields contain only safe values; no token or secret fields exist.

```go
type LoginRequest struct { Card string `json:"card"`; Remember bool `json:"remember"` }
type Status struct {
	Phase Phase `json:"phase"`; Authorized bool `json:"authorized"`
	Generation uint64 `json:"generation"`
	Nickname string `json:"nickname,omitempty"`; ExpireTime string `json:"expireTime,omitempty"`
	Remaining int64 `json:"remaining"`; HeartbeatFailures int `json:"heartbeatFailures"`
	ErrorCode string `json:"errorCode,omitempty"`; Message string `json:"message,omitempty"`
}

type RememberedCard struct { Card string `json:"card"`; Remembered bool `json:"remembered"` }
type LoginResult struct { Nickname string; TokenPresent bool; PongInterval time.Duration }
type ProgramVersion struct { Number int `json:"number"`; Name string `json:"name"`; Description string `json:"description"`; DownloadURL string `json:"downloadUrl"` }
type UpdateState struct { Current Version `json:"current"`; Latest ProgramVersion `json:"latest"`; Available bool `json:"available"`; CheckedAt string `json:"checkedAt"`; ErrorCode string `json:"errorCode,omitempty"`; Message string `json:"message,omitempty"` }
```

- [ ] **Step 4: Implement the SDK adapter**

```go
type Client interface {
	Login(LoginInput) (LoginResult, error)
	UserInfo() (UserInfo, error)
	Pong() error
	ProgramDetail() (ProgramVersion, error)
	Logout() error
}

type Factory interface { New() Client }
```

The production factory creates `kauth.NewKauthApi(kauth.Config{...}, kauth.SignTypeRSA)`. `Login` calls `KaLogin` with `PlatformType: "golang"`, requires a non-empty Token, parses `PongInterval`, and relies on the SDK's in-memory token. `ProgramDetail` calls the SDK's `GetProgramDetail()` and maps `CurrentVersion`; nil current version returns a valid “no remote version” state.

Never include `err.Error()` from cryptographic failures in UI DTOs. Map errors to stable codes such as `network_unavailable`, `card_rejected`, and `protocol_error`; retain only sanitized diagnostic text internally.

- [ ] **Step 5: Implement version comparison and URL validation**

```go
func ValidateDownloadURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", ErrInvalidDownloadURL
	}
	return u.String(), nil
}

func CompareVersion(local Version, remote ProgramVersion) UpdateState {
	return UpdateState{Current: local, Latest: remote, Available: remote.Number > local.Number}
}
```

- [ ] **Step 6: Run focused tests**

Run: `go test ./internal/license -run 'Client|PongInterval|CompareVersion|DownloadURL' -v`

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add internal/license
git commit -m "feat: add KAuth RSA client adapter"
```

## Task 4: Implement Login State Machine and Session Generations

**Files:**
- Create: `internal/license/manager.go`
- Create: `internal/license/manager_test.go`

- [ ] **Step 1: Write failing login/activation tests**

Create fakes for `Factory`, `DeviceIDProvider`, `CredentialStore`, `RuntimeLifecycle`, and event sink.

```go
type DeviceIDProvider interface { DeviceID(context.Context) (string, error) }
type RememberedCredentialStore interface {
	Load(context.Context) (card string, remembered bool, err error)
	Remember(context.Context, string) error
	Forget(context.Context) error
}
type RuntimeLifecycle interface {
	Activate(context.Context, uint64) error
	Deactivate(context.Context, uint64) error
}
type EventSink interface { Publish(Status) }
```

```go
func TestManagerLoginActivatesRuntimeAndStoresCardAfterSuccess(t *testing.T) {
	runtime := &fakeRuntime{}
	credentials := &fakeCredentials{}
	m := newTestManager(successfulClient(), runtime, credentials)
	status := m.Login(context.Background(), LoginRequest{Card: " card ", Remember: true})
	if !status.Authorized || runtime.activations != 1 { t.Fatalf("status %#v runtime %#v", status, runtime) }
	if credentials.remembered != "card" { t.Fatalf("remembered %q", credentials.remembered) }
}

func TestManagerActivationFailureRollsBackAndDoesNotRemember(t *testing.T) {
	runtime := &fakeRuntime{activateErr: errors.New("boom")}
	credentials := &fakeCredentials{}
	m := newTestManager(successfulClient(), runtime, credentials)
	status := m.Login(context.Background(), LoginRequest{Card: "card", Remember: true})
	if status.Authorized || runtime.deactivations != 1 || credentials.remembered != "" {
		t.Fatalf("status %#v runtime %#v credentials %#v", status, runtime, credentials)
	}
}
```

- [ ] **Step 2: Run and verify failure**

Run: `go test ./internal/license -run 'ManagerLogin|ActivationFailure' -v`

Expected: FAIL because `Manager` is undefined.

- [ ] **Step 3: Implement atomic login**

`Manager.Login` must:

1. Trim card and reject empty input.
2. Transition `locked/error -> authenticating` under mutex.
3. Reject a second concurrent login with `authentication_in_progress`.
4. Create a fresh SDK client and device ID.
5. Login and fetch user info.
6. Create a new generation and session context.
7. Activate runtime.
8. Only after activation succeeds, update or delete the remembered credential.
9. Store the client, status, and cancellation function; transition to `authorized`.

Use a deferred failure path that logs out best-effort, cancels the provisional context, deactivates any partial runtime, and returns `locked/error` without persisting the submitted card.

- [ ] **Step 4: Add stale-generation tests**

```go
func TestOldGenerationCannotRevokeNewLogin(t *testing.T) {
	m := newTestManager(successfulClient(), &fakeRuntime{}, &fakeCredentials{})
	first := m.Login(context.Background(), LoginRequest{Card: "one"})
	oldGeneration := first.Generation
	m.Revoke(context.Background(), "test")
	second := m.Login(context.Background(), LoginRequest{Card: "two"})
	m.handleHeartbeatResult(oldGeneration, errors.New("late failure"))
	if !m.Status().Authorized || m.Status().Generation != second.Generation { t.Fatal("stale result revoked new session") }
}
```

- [ ] **Step 5: Run manager tests**

Run: `go test ./internal/license -run 'Manager|Generation' -v`

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add internal/license/manager.go internal/license/manager_test.go
git commit -m "feat: add license login state machine"
```

## Task 5: Add Pong Heartbeats and Three-Failure Revocation

**Files:**
- Modify: `internal/license/manager.go`
- Modify: `internal/license/manager_test.go`

- [ ] **Step 1: Write failing heartbeat tests with a fake timer**

```go
func TestHeartbeatSuccessResetsFailures(t *testing.T) {
	m := authorizedTestManager(t)
	m.handleHeartbeatResult(m.Status().Generation, errors.New("network"))
	m.handleHeartbeatResult(m.Status().Generation, nil)
	if got := m.Status().HeartbeatFailures; got != 0 { t.Fatalf("failures=%d", got) }
}

func TestThirdConsecutiveHeartbeatFailureRevokesExactlyOnce(t *testing.T) {
	runtime := &fakeRuntime{}
	m := authorizedTestManagerWithRuntime(t, runtime)
	generation := m.Status().Generation
	for i := 0; i < 3; i++ { m.handleHeartbeatResult(generation, errors.New("network")) }
	if m.Status().Authorized || runtime.deactivations != 1 { t.Fatalf("status %#v runtime %#v", m.Status(), runtime) }
}
```

- [ ] **Step 2: Verify tests fail**

Run: `go test ./internal/license -run Heartbeat -v`

Expected: FAIL because heartbeat handling is incomplete.

- [ ] **Step 3: Implement one worker per authorized session**

Add an injected `After func(time.Duration) <-chan time.Time`, defaulting to `time.After`. The worker waits one `PongInterval`, calls `Pong`, and forwards the result with its captured generation. A success resets failures and emits healthy status. Failures 1 and 2 emit warning status. Failure 3 atomically transitions to `revoking` and calls `Revoke` once.

Do not use a fixed ticker shared across sessions. Cancellation must interrupt the wait immediately.

- [ ] **Step 4: Test cancellation and re-login**

Add tests proving a canceled worker exits, a new login creates one new worker, and old worker results are ignored.

Run: `go test ./internal/license -run 'Heartbeat|Revoke|Generation' -count=20`

Expected: PASS 20 consecutive runs without races or flakes.

- [ ] **Step 5: Run the race detector for the package**

Run: `go test -race ./internal/license`

Expected: PASS with no race reports.

- [ ] **Step 6: Commit**

```powershell
git add internal/license/manager.go internal/license/manager_test.go
git commit -m "feat: revoke license after heartbeat failures"
```

## Task 6: Split Wails Bootstrap from Authorized Runtime

**Files:**
- Create: `authorized_lifecycle.go`
- Create: `authorized_lifecycle_test.go`
- Create: `license_api.go`
- Create: `license_api_test.go`
- Modify: `app.go`
- Modify: `main.go`
- Modify: `app_test.go`

- [ ] **Step 1: Add a legacy authorized test constructor**

In `app_test.go`, add:

```go
func newAuthorizedTestApp(t *testing.T) *App {
	t.Helper()
	app := NewApp()
	app.authorizationForTests = true
	return app
}
```

Mechanically replace the existing operational `NewApp()` calls in tests with `newAuthorizedTestApp(t)`. New license/bootstrap tests must continue to use production `NewApp()`.

- [ ] **Step 2: Write failing bootstrap and lifecycle tests**

```go
func TestStartupDoesNotStartAuthorizedServices(t *testing.T) {
	app := NewApp()
	app.dataDir = t.TempDir()
	app.startup(context.Background())
	t.Cleanup(func() { app.shutdown(context.Background()) })
	if app.messagePushCancel != nil { t.Fatal("message push scheduler started before authorization") }
	if app.guardian.Status().Running { t.Fatal("guardian started before authorization") }
	if app.supervisor.Status().Connected { t.Fatal("runtime connected before authorization") }
}

func TestAuthorizedLifecycleCanActivateDeactivateAndReactivate(t *testing.T) {
	app := newAuthorizedTestApp(t)
	// inject fake start/stop hooks, then assert start -> stop -> start order and idempotency
}
```

- [ ] **Step 3: Verify tests fail**

Run: `go test . -run 'StartupDoesNotStartAuthorizedServices|AuthorizedLifecycle' -v`

Expected: FAIL because current `startup` starts formal services.

- [ ] **Step 4: Split startup into bootstrap and session lifecycle**

Add App fields for `license.Manager`, config, authorized session cancel, generation, and lifecycle mutex. Keep storage opening in `startup`, but move these existing calls into `activateAuthorizedRuntime`:

- `ensureMessagePushService`
- `startMessagePushDailyScheduler`
- `LoadRuntimeSettings`, `applyRuntimeSettings`, `rebuildCDPLinks`, and target initialization
- `guardian.Start`
- runtime autostart `supervisor.Switch`
- startup `runQQDebugPatch`

`deactivateAuthorizedRuntime` must mark authorization unavailable first, cancel session context, stop automation, push scheduler, guardian, and runtime supervisor, reset account-scoped services, and remain safe when called repeatedly.

- [ ] **Step 5: Add Wails license API methods**

```go
func (a *App) LicenseStatus() license.Status
func (a *App) RememberedLicenseCard() license.RememberedCard
func (a *App) LicenseLogin(input license.LoginRequest) license.Status
func (a *App) ForgetLicenseCard() error
func (a *App) CheckForUpdates() license.UpdateState
```

Use `runtime.EventsEmit(a.ctx, "license:status", status)` after status changes and `runtime.EventsEmit(a.ctx, "license:revoked", status)` only after deactivation completes. Do not emit cards or tokens.

- [ ] **Step 6: Make shutdown reuse deactivation**

`shutdown` first calls manager shutdown/best-effort logout, then `deactivateAuthorizedRuntime`, then closes storage. It must not duplicate stop logic.

- [ ] **Step 7: Run backend lifecycle tests**

Run: `go test . -run 'License|Authorized|Startup|Shutdown' -v`

Expected: PASS.

- [ ] **Step 8: Regenerate Wails bindings without starting a dev server**

Run: `wails build -s -nopackage -o Farm_Go-bindings.exe`

Expected: Go build succeeds and `frontend/wailsjs/go/main/App.js`, `App.d.ts`, and `models.ts` include the five new methods/types.

- [ ] **Step 9: Commit**

```powershell
git add app.go app_test.go main.go license_api.go license_api_test.go authorized_lifecycle.go authorized_lifecycle_test.go frontend/wailsjs
git commit -m "feat: defer Farm Go runtime until license activation"
```

## Task 7: Enforce the Backend Authorization Boundary

**Files:**
- Create: `authorization_gate.go`
- Create: `authorization_gate_test.go`
- Modify: `app.go`
- Modify: `app_test.go`

- [ ] **Step 1: Write failing authorization policy tests**

Use Go reflection to enumerate exported `*App` methods. The bootstrap allowlist is exact:

```go
var bootstrapMethodAllowlist = map[string]bool{
	"LicenseStatus": true,
	"RememberedLicenseCard": true,
	"LicenseLogin": true,
	"ForgetLicenseCard": true,
}
```

The protected registry must contain the current operational surface plus the authorized update call:

```go
var protectedMethodRegistry = map[string]bool{
	"RuntimeStatus": true,
	"ConnectionInfo": true,
	"RunDiagnostic": true,
	"RuntimeSettings": true,
	"MessagePushState": true,
	"SaveMessagePushConfig": true,
	"SendMessagePushTest": true,
	"SendMessagePushDailyTest": true,
	"RunDueMessagePushDaily": true,
	"SaveRuntimeSettings": true,
	"WarehouseAutoSellSettings": true,
	"SaveWarehouseAutoSellSettings": true,
	"SwitchRuntimeTarget": true,
	"RuntimeLinkStatus": true,
	"RuntimeEvents": true,
	"InstallQQDebugPatch": true,
	"QQDebugPatchStatus": true,
	"FarmFeatureCatalog": true,
	"FarmAutomationState": true,
	"FarmSocialState": true,
	"FarmSocialAction": true,
	"FarmSocialProtocolBlockList": true,
	"FarmSocialRankings": true,
	"FarmSocialRankingPreferences": true,
	"SaveFarmSocialRankingPreferences": true,
	"FarmSocialDogGuardState": true,
	"FarmSocialDogGuardAction": true,
	"FarmSocialExport": true,
	"FarmSocialImport": true,
	"SaveFarmAutomationState": true,
	"RunFarmAutomationTask": true,
	"RunScheduledFarmAutomationTask": true,
	"FarmAutomationSchedulerState": true,
	"StartFarmAutomationScheduler": true,
	"StopFarmAutomationScheduler": true,
	"RunDueFarmAutomationTasks": true,
	"FarmBackpackSeedOptions": true,
	"FarmStealCropOptions": true,
	"FarmAccountStatus": true,
	"CurrentRuntimeAccount": true,
	"IdentifyRuntimeAccount": true,
	"ConfirmRuntimeAccount": true,
	"FarmCropAnalytics": true,
	"FarmAtlasPreview": true,
	"FarmAtlasBuyLockedPreview": true,
	"FarmAtlasBuyLockedCrops": true,
	"FarmLandDetails": true,
	"FarmLandRush": true,
	"FarmFertilizeLand": true,
	"FarmShovelLands": true,
	"FarmWarehouse": true,
	"FarmWarehouseRefresh": true,
	"FarmWarehouseSell": true,
	"FarmWarehouseSellRecords": true,
	"ProcessGuardStatus": true,
	"GuardianStatus": true,
	"SaveGuardianSettings": true,
	"RunGuardianAction": true,
	"SaveProcessGuardSettings": true,
	"ListHostCandidates": true,
	"AutoBindHostProcess": true,
	"BindHostProcess": true,
	"ClearHostBinding": true,
	"PreviewHostRestart": true,
	"RestartHostProcess": true,
	"LaunchHostProcess": true,
	"SaveTextFile": true,
	"CheckForUpdates": true,
}
```

Add behavioral tests proving unauthorized calls cannot switch runtime, save settings, launch/restart a host, start automation, run diagnostics, send push tests, or execute farm/social/warehouse actions.

- [ ] **Step 2: Verify unauthorized behavior currently fails**

Run: `go test . -run Unauthorized -v`

Expected: FAIL because operational methods still execute or return ordinary state.

- [ ] **Step 3: Add central guard helpers**

```go
var ErrLicenseRequired = errors.New("license verification required")

func (a *App) isAuthorized() bool {
	if a.authorizationForTests { return true }
	return a.licenseManager != nil && a.licenseManager.Status().Authorized
}

func (a *App) requireAuthorized() error {
	if !a.isAuthorized() { return ErrLicenseRequired }
	return nil
}
```

Production `NewApp()` leaves `authorizationForTests` false. Bootstrap initialization failure must remain unauthorized; never treat a nil manager as authorized.

- [ ] **Step 4: Guard every exported operational method by return-shape group**

Apply `requireAuthorized` before work in all exported methods except the four bootstrap methods. Use consistent safe results:

- Methods returning `(T, error)`: return zero `T`, `ErrLicenseRequired`.
- Methods returning `error`: return `ErrLicenseRequired`.
- Status/payload-only methods: return the existing typed failed/error payload with message `license verification required` and no account data.
- Read-only runtime/guard/settings methods: return zero or locked-safe status; they must not lazily start services.

Cover these groups explicitly: runtime/diagnostics/settings, message push, QQ patch, automation/scheduler, farm/social/assets/warehouse, account identity, guardian/process control, host binding/launch/restart, and file export.

- [ ] **Step 5: Add an exported-method audit assertion**

The test must fail when a future exported method is neither bootstrap-allowed nor listed in a protected-method registry. Keep the registry beside `bootstrapMethodAllowlist`; adding a new Wails method requires an explicit policy decision.

- [ ] **Step 6: Run the entire backend suite**

Run: `go test ./...`

Expected: PASS. Existing operational tests use `newAuthorizedTestApp(t)` and preserve prior behavior.

- [ ] **Step 7: Commit**

```powershell
git add authorization_gate.go authorization_gate_test.go app.go app_test.go
git commit -m "feat: enforce license checks on Wails operations"
```

## Task 8: Add React Bootstrap and the Focused License Gate

**Files:**
- Create: `frontend/src/AuthorizedApp.tsx`
- Create: `frontend/src/AppBootstrap.test.tsx`
- Create: `frontend/src/components/LicenseGate.tsx`
- Create: `frontend/src/components/LicenseGate.test.tsx`
- Create: `frontend/src/lib/license.ts`
- Create: `frontend/src/lib/license.test.ts`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/App.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Move current operational App without changing behavior**

Move the current `App.tsx` body to `AuthorizedApp.tsx`, rename the component `AuthorizedApp`, and keep existing helper exports there. Update the existing operational tests to import from `AuthorizedApp`.

Run: `cd frontend; npm test -- AuthorizedApp App.test.tsx`

Expected: existing operational assertions PASS before adding the gate.

- [ ] **Step 2: Write failing bootstrap tests**

Mock Wails methods and runtime `EventsOn`:

```tsx
it('does not mount AuthorizedApp while locked', async () => {
  vi.mocked(LicenseStatus).mockResolvedValue({ phase: 'locked', authorized: false } as any)
  const renderer = await renderAppBootstrap()
  expect(renderer.root.findAllByType(AuthorizedApp)).toHaveLength(0)
  expect(textOf(renderer)).toContain('卡密验证')
  expect(RuntimeStatus).not.toHaveBeenCalled()
})
```

- [ ] **Step 3: Write failing LicenseGate interaction tests**

Cover remembered-card prefill, password visibility, disabled empty submit, Enter submit, loading lock, inline failure, and `remember` request value. Assert the actual card value never appears in rendered HTML while the input type is password.

Run: `cd frontend; npm test -- AppBootstrap LicenseGate`

Expected: FAIL because the components do not exist.

- [ ] **Step 4: Implement frontend license types and normalization**

`lib/license.ts` defines discriminated phases and defaults. Unknown/malformed backend data normalizes to locked/error, never authorized.

- [ ] **Step 5: Implement `LicenseGate`**

Use `KeyRound`, `Eye`, `EyeOff`, `Loader2`, and `ShieldCheck` from lucide-react. Use a real `<form>` for Enter behavior, a password input by default, a checkbox for remember, fixed error/status height, and no formal navigation. Save the remembered card only through the backend login request; do not use `localStorage` or session storage.

- [ ] **Step 6: Implement `AppBootstrap`**

On mount, call `LicenseStatus` and `RememberedLicenseCard`, subscribe to `license:status` and `license:revoked`, and clean up both event listeners. Render:

```tsx
if (booting) return <LicenseGate loading statusMessage="正在初始化授权" />
if (!license.authorized) return <LicenseGate initialCard={remembered.card} remembered={remembered.remembered} onLogin={login} />
return <AuthorizedApp />
```

Do not mount `AuthorizedApp` during bootstrap or authentication.

- [ ] **Step 7: Add responsive gate styling**

Match the approved A layout: warm white full surface, 70px lightweight brand bar, centered 340–400px form, 6px-or-less control radius, stable 44px controls, fixed error area, and bottom-right version. Add mobile constraints without viewport-scaled font sizes.

- [ ] **Step 8: Run frontend tests and build**

Run:

```powershell
cd frontend
npm test
npm run build
```

Expected: all Vitest tests PASS and TypeScript/Vite build succeeds.

- [ ] **Step 9: Commit**

```powershell
git add frontend/src/App.tsx frontend/src/App.test.tsx frontend/src/AppBootstrap.test.tsx frontend/src/AuthorizedApp.tsx frontend/src/components/LicenseGate.tsx frontend/src/components/LicenseGate.test.tsx frontend/src/lib/license.ts frontend/src/lib/license.test.ts frontend/src/style.css
git commit -m "feat: add card verification gate UI"
```

## Task 9: Show Recoverable Heartbeat Health and Revoke Cleanly

**Files:**
- Create: `frontend/src/components/LicenseHeartbeatNotice.tsx`
- Create: `frontend/src/components/LicenseHeartbeatNotice.test.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/AuthorizedApp.tsx`
- Modify: `frontend/src/components/AppShell.tsx`
- Modify: `frontend/src/components/AppShell.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write failing notice and revoke tests**

```tsx
it('shows only recoverable heartbeat failures', () => {
  expect(renderToStaticMarkup(<LicenseHeartbeatNotice failures={1} />)).toContain('1/3')
  expect(renderToStaticMarkup(<LicenseHeartbeatNotice failures={0} />)).toBe('')
  expect(renderToStaticMarkup(<LicenseHeartbeatNotice failures={3} />)).toBe('')
})
```

Add an `AppBootstrap` test that emits `license:revoked`, verifies `AuthorizedApp` unmounts, and verifies the gate shows `授权连接已失效，请重新验证` only after the event.

- [ ] **Step 2: Verify tests fail**

Run: `cd frontend; npm test -- LicenseHeartbeatNotice AppBootstrap AppShell`

Expected: FAIL because the notice and event behavior are absent.

- [ ] **Step 3: Implement notice placement**

Pass safe license status from `AppBootstrap` into `AuthorizedApp` and `AppShell`. Render one unframed status strip at the top of the main surface for failures 1 or 2. Do not use a modal and do not change content dimensions when the strip appears; reserve its track or use a fixed notification region.

- [ ] **Step 4: Implement revoke transition**

On `license:revoked`, clear authorized state and set the gate message. Do not preserve any operational React state across reauthorization; remount `AuthorizedApp` with the license generation as its key.

- [ ] **Step 5: Run frontend tests**

Run: `cd frontend; npm test -- LicenseHeartbeatNotice AppBootstrap AppShell`

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add frontend/src
git commit -m "feat: surface license heartbeat health"
```

## Task 10: Add Non-Blocking Update Checks to System Settings

**Files:**
- Create: `frontend/src/components/UpdateAvailableToast.tsx`
- Create: `frontend/src/components/UpdateAvailableToast.test.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/AuthorizedApp.tsx`
- Modify: `frontend/src/views/SettingsView.tsx`
- Modify: `frontend/src/views/SettingsView.test.tsx`
- Modify: `frontend/src/components/AppShell.tsx`
- Modify: `frontend/src/components/AppShell.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write failing settings update tests**

Mock `CheckForUpdates` and `BrowserOpenURL`. Cover:

- automatic check starts after authorization and does not delay `AuthorizedApp` rendering;
- latest version shows `当前已是最新版本` only after manual check;
- newer version shows current/latest version, description, and download button;
- failed automatic check is silent;
- failed manual check renders inside the update section;
- invalid/absent URL disables the download action.

- [ ] **Step 2: Verify tests fail**

Run: `cd frontend; npm test -- SettingsView UpdateAvailableToast AppBootstrap`

Expected: FAIL because update UI and orchestration are absent.

- [ ] **Step 3: Trigger automatic check without blocking entry**

After `AppBootstrap` receives authorized status, mount `AuthorizedApp` immediately and start `CheckForUpdates()` in an effect keyed by license generation. Ignore results from stale generations. Store successful update state in the authorized root; swallow automatic errors after recording a console-safe message.

- [ ] **Step 4: Implement the update toast**

Use `RefreshCw`, `ExternalLink`, and `X` icons. The toast provides `查看更新` and an icon-only dismiss button with tooltip. `查看更新` changes the active tab to `settings` and focuses the update section; it does not open the browser immediately.

- [ ] **Step 5: Add the System Settings update section**

Below the existing runtime-link section, add an un-nested `settings-panel` titled `应用更新`. Keep separate busy/error state from runtime settings save. The manual button calls `CheckForUpdates`; `前往下载` calls `BrowserOpenURL` only with the backend-validated URL from `UpdateState`.

- [ ] **Step 6: Run tests and build**

Run:

```powershell
cd frontend
npm test
npm run build
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add frontend/src
git commit -m "feat: add KAuth update reminders"
```

## Task 11: Harden Build, Add Opt-In Live Test, and Run Release Acceptance

**Files:**
- Create: `internal/license/integration_test.go`
- Modify: `scripts/build.ps1`
- Modify: `README.md`

- [ ] **Step 1: Add an opt-in live KAuth test**

```go
func TestKAuthIntegration(t *testing.T) {
	card := strings.TrimSpace(os.Getenv("KAUTH_TEST_CARD"))
	if card == "" { t.Skip("set KAUTH_TEST_CARD to run live KAuth integration") }
	cfg, err := LoadConfig()
	if err != nil { t.Fatal(err) }
	client := NewKAuthFactory(cfg).New()
	login, err := client.Login(LoginInput{Card: card, DeviceID: os.Getenv("KAUTH_TEST_DEVICE_ID"), PlatformType: "golang"})
	if err != nil { t.Fatal("login failed") }
	if login.PongInterval <= 0 { t.Fatal("invalid PongInterval") }
	if _, err := client.UserInfo(); err != nil { t.Fatal("user info failed") }
	if err := client.Pong(); err != nil { t.Fatal("pong failed") }
	if _, err := client.ProgramDetail(); err != nil { t.Fatal("program detail failed") }
	if err := client.Logout(); err != nil { t.Fatal("logout failed") }
}
```

The test messages deliberately omit error bodies that might contain sensitive protocol data. It is skipped in ordinary test runs.

- [ ] **Step 2: Verify ordinary tests skip live access**

Run: `go test ./internal/license -run TestKAuthIntegration -v`

Expected: SKIP because `KAUTH_TEST_CARD` is unset.

- [ ] **Step 3: Harden `scripts/build.ps1`**

Validate all five required environment values before invoking Wails. Build `-ldflags` with `-X Farm_Go/internal/license.build...` assignments. Temporarily set `wails.json.info.productVersion` from `FARM_GO_VERSION_NAME`, run `wails build -clean -trimpath -ldflags $ldflags`, and restore the original `wails.json` content in `finally` even when the build fails.

Never print the generated ldflags or secret values. Print only program ID, version name, and version number.

- [ ] **Step 4: Document developer and release commands**

Update `README.md` with environment variable names only, ordinary tests, the opt-in integration command, and release build command. Do not include example real-looking secrets or cards.

- [ ] **Step 5: Run the live integration test with environment-provided values**

Set the KAuth values and test card only in the current shell environment, then run:

```powershell
go test ./internal/license -run TestKAuthIntegration -v -count=1
```

Expected: PASS through login, user info, Pong, program detail, and logout. No card, token, key, signature, or raw response is printed.

- [ ] **Step 6: Run full automated verification**

```powershell
go test ./...
go test -race ./internal/license
Set-Location frontend
npm test
npm run build
Set-Location ..
```

Expected: all commands exit 0.

- [ ] **Step 7: Build the release executable**

Run: `powershell -ExecutionPolicy Bypass -File scripts\build.ps1`

Expected: exit 0 and `build\bin\Farm_Go.exe` exists with the configured product version.

- [ ] **Step 8: Perform manual acceptance**

Verify each condition and record the result in the implementation handoff:

1. With network disabled at launch, the card page remains open and no runtime listener, guardian, push scheduler, automation scheduler, or QQ patch starts.
2. Invalid card stays on the gate with an inline error and no card is persisted.
3. Valid card activates the workbench and configured runtime exactly once.
4. Remembered card prefills after restart but does not auto-submit.
5. One or two simulated Pong failures show 1/3 and 2/3; a success clears the warning.
6. Three consecutive failures stop services and return to the gate.
7. Revalidation after revoke starts a fresh working session.
8. Automatic update check does not delay the workbench.
9. Manual update checks show latest/new/error states and open only HTTP(S) URLs.

- [ ] **Step 9: Scan tracked content for secret leakage**

Run:

```powershell
git grep -n -E "KAUTH_TEST_CARD=|KAUTH_PROGRAM_SECRET=|accesstoken.*[A-Za-z0-9]" -- ':!docs/superpowers/plans/*'
git status --short
```

Expected: no embedded values; only intentional environment-variable documentation may match. Worktree contains only intended changes.

- [ ] **Step 10: Commit**

```powershell
git add internal/license/integration_test.go scripts/build.ps1 README.md
git commit -m "test: verify KAuth release workflow"
```

## Final Verification

- [ ] Run `go test ./...` and confirm zero failures.
- [ ] Run `go test -race ./internal/license` and confirm zero race reports.
- [ ] Run `cd frontend; npm test; npm run build` and confirm zero failures.
- [ ] Run the opt-in KAuth integration test and confirm login, Pong, program detail, and logout pass without sensitive output.
- [ ] Run `scripts/build.ps1` and confirm `build/bin/Farm_Go.exe` is produced.
- [ ] Confirm an unauthorized cold start has no operational background services.
- [ ] Confirm three consecutive Pong failures deactivate services and return to the card gate.
- [ ] Confirm the remembered card is DPAPI ciphertext at rest and only prefills, never auto-submits.
- [ ] Confirm update checks never block workbench entry and never force installation.
- [ ] Confirm tracked files, frontend bundle, test snapshots, and build logs contain no actual program secret, test card, or token.
