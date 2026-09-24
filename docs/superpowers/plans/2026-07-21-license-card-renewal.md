# KAuth Card Renewal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an authorized workbench renewal flow that consumes a new KAuth card, refreshes the active old-card session once, and displays the new expiration time.

**Architecture:** Extend the KAuth adapter with `RechargeCard`, then let `internal/license.Manager` retain the active card only for the session, serialize renewal with heartbeat traffic, and publish refreshed safe status. Expose one protected Wails method and add a focused React dialog launched from the workbench header.

**Tech Stack:** Go 1.24, KAuth Go SDK v1.0.0, Wails v2 bindings, React 18, TypeScript, Vitest, react-test-renderer, Lucide React.

---

## File Map

- Modify `internal/license/client.go`: add SDK recharge mapping and safe error classification.
- Modify `internal/license/client_test.go`: verify request mapping and credential-safe failures.
- Modify `internal/license/types.go`: define renewal request/result DTOs.
- Modify `internal/license/manager.go`: retain and clear session secrets and serialize SDK operations.
- Create `internal/license/renewal.go`: coordinate recharge, one old-card login, user-info refresh, and generation checks.
- Modify `internal/license/manager_test.go`: cover renewal outcomes, concurrency, generation isolation, and cleanup.
- Modify `license_api.go` and `license_api_test.go`: expose and test the Wails method.
- Modify `authorization_gate.go` and `authorization_gate_test.go`: classify renewal as authorization-required.
- Modify `frontend/wailsjs/go/main/App.js`, `App.d.ts`, and `models.ts`: add generated-style bindings.
- Create `frontend/src/components/LicenseRenewalDialog.tsx` and its test: own sensitive form state and result rendering.
- Modify `frontend/src/views/OverviewView.tsx` and its test: add the workbench command.
- Modify `frontend/src/style.css`: add restrained responsive dialog styles.

### Task 1: KAuth Recharge Adapter

**Files:**
- Modify: `internal/license/client.go`
- Test: `internal/license/client_test.go`

- [ ] **Step 1: Write failing adapter tests**

Add these tests:

~~~go
func TestKAuthClientRechargesCurrentCardWithoutExposingCredentials(t *testing.T) {
	api := &fakeKAuthAPI{rechargeResponse: &kauth.Response{Code: 200, Msg: "success"}}
	client := newKAuthClient(api)

	err := client.RechargeCard("old-card-must-not-leak", "new-card-must-not-leak")

	if err != nil {
		t.Fatalf("RechargeCard() error = %v", err)
	}
	want := kauth.KaRechargeKaReq{
		CardPwd:         "old-card-must-not-leak",
		RechargeCardPwd: "new-card-must-not-leak",
	}
	if api.rechargeRequest != want {
		t.Fatalf("RechargeKa() request = %#v, want %#v", api.rechargeRequest, want)
	}
}

func TestKAuthClientClassifiesRechargeFailureWithoutLeakingCredentials(t *testing.T) {
	api := &fakeKAuthAPI{
		rechargeResponse: &kauth.Response{Code: 400, Msg: "充值卡密不可用"},
		rechargeErr:      &kauth.ApiError{Code: 400, Msg: "充值卡密不可用"},
	}
	client := newKAuthClient(api)

	err := client.RechargeCard("old-card-must-not-leak", "new-card-must-not-leak")

	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) ||
		serviceErr.Code != "license_recharge_failed" ||
		serviceErr.Message != "充值卡密不可用" {
		t.Fatalf("RechargeCard() error = %#v", err)
	}
	if strings.Contains(err.Error(), "old-card-must-not-leak") ||
		strings.Contains(err.Error(), "new-card-must-not-leak") {
		t.Fatalf("RechargeCard() leaked a card: %v", err)
	}
}
~~~

Extend `fakeKAuthAPI`:

~~~go
rechargeRequest  kauth.KaRechargeKaReq
rechargeResponse *kauth.Response
rechargeErr      error

func (f *fakeKAuthAPI) RechargeKa(request kauth.KaRechargeKaReq) (*kauth.Response, error) {
	f.rechargeRequest = request
	return f.rechargeResponse, f.rechargeErr
}
~~~

- [ ] **Step 2: Run the adapter tests and verify RED**

Run:

~~~powershell
go test ./internal/license -run 'TestKAuthClient(Recharges|ClassifiesRecharge)' -count=1
~~~

Expected: compilation fails because `RechargeCard` and `RechargeKa` are not part of the internal interfaces.

- [ ] **Step 3: Implement the minimal adapter**

Extend `Client` and `kauthAPI`:

~~~go
type Client interface {
	Login(LoginInput) (LoginResult, error)
	RechargeCard(currentCard, rechargeCard string) error
	UserInfo() (UserInfo, error)
	Pong() error
	Logout() error
	ProgramDetail() (ProgramVersion, error)
}

type kauthAPI interface {
	KaLogin(kauth.LoginRequest) (*kauth.LoginResponse, *kauth.Response, error)
	RechargeKa(kauth.KaRechargeKaReq) (*kauth.Response, error)
	GetUserInfo() (*kauth.UserInfoResponse, *kauth.Response, error)
	Pong() (*kauth.Response, error)
	LoginOut() (*kauth.Response, error)
	GetProgramDetail() (*kauth.ProgramDetailResponse, *kauth.Response, error)
}
~~~

Add the adapter implementation:

~~~go
func (c *kauthClient) RechargeCard(currentCard, rechargeCard string) error {
	response, err := c.api.RechargeKa(kauth.KaRechargeKaReq{
		CardPwd:         currentCard,
		RechargeCardPwd: rechargeCard,
	})
	if err == nil && response != nil && response.Code == 200 {
		return nil
	}
	return classifyRechargeError(response, err)
}

func classifyRechargeError(response *kauth.Response, err error) *ServiceError {
	var networkError net.Error
	if errors.As(err, &networkError) {
		return serviceError("network_unavailable", "license service is unavailable")
	}
	if err != nil && strings.HasPrefix(err.Error(), "请求失败: ") {
		return serviceError("network_unavailable", "license service is unavailable")
	}
	if response != nil && response.Code > 0 && response.Code != 200 {
		return serviceError("license_recharge_failed", serviceMessage(response.Msg, "license recharge failed"))
	}
	var apiError *kauth.ApiError
	if errors.As(err, &apiError) && apiError.Code > 0 {
		return serviceError("license_recharge_failed", serviceMessage(apiError.Msg, "license recharge failed"))
	}
	return serviceError("license_recharge_failed", "license recharge failed")
}
~~~

- [ ] **Step 4: Run adapter tests**

~~~powershell
go test ./internal/license -run 'TestKAuthClient(Recharges|ClassifiesRecharge)' -count=1
go test ./internal/license -count=1
~~~

Expected: both commands pass without printing credentials.

- [ ] **Step 5: Commit the adapter**

~~~powershell
git add internal/license/client.go internal/license/client_test.go
git commit -m "feat: add KAuth card recharge adapter"
~~~

### Task 2: Renewal DTOs and Manager Lifecycle

**Files:**
- Modify: `internal/license/types.go`
- Modify: `internal/license/manager.go`
- Create: `internal/license/renewal.go`
- Test: `internal/license/manager_test.go`

- [ ] **Step 1: Write failing validation and complete-success tests**

Add:

~~~go
func TestManagerRenewRejectsEmptyAndUnauthorizedRequests(t *testing.T) {
	manager := newTestManager(managerSuccessfulClient(), &managerFakeRuntime{}, &managerFakeCredentials{})

	if got := manager.Renew(context.Background(), RenewalRequest{RechargeCard: "new-card"}); got.ErrorCode != "license_required" {
		t.Fatalf("unauthorized Renew() = %#v", got)
	}
	if status := manager.Login(context.Background(), LoginRequest{Card: "old-card"}); !status.Authorized {
		t.Fatalf("Login() = %#v", status)
	}
	if got := manager.Renew(context.Background(), RenewalRequest{RechargeCard: " \t "}); got.ErrorCode != "recharge_card_required" {
		t.Fatalf("empty Renew() = %#v", got)
	}
}

func TestManagerRenewRechargesRelogsOnceAndPublishesExpiration(t *testing.T) {
	client := managerSuccessfulClient()
	client.renewedUser = UserInfo{ID: "user-1", ServerExpireTime: "2027-12-31 23:59:59"}
	events := &managerFakeEvents{}
	manager := newTestManagerWithEvents(client, &managerFakeRuntime{}, &managerFakeCredentials{}, events)
	authorized := manager.Login(context.Background(), LoginRequest{Card: " old-card "})

	got := manager.Renew(context.Background(), RenewalRequest{RechargeCard: " new-card "})

	if !got.Renewed || !got.Refreshed || got.ExpireTime != "2027-12-31 23:59:59" {
		t.Fatalf("Renew() = %#v", got)
	}
	if client.rechargeCurrentCard != "old-card" || client.rechargeCard != "new-card" {
		t.Fatalf("recharge request = %q, %q", client.rechargeCurrentCard, client.rechargeCard)
	}
	if client.loginCalls != 2 || client.userInfoCalls != 2 || client.rechargeCalls != 1 {
		t.Fatalf("calls: login=%d user=%d recharge=%d", client.loginCalls, client.userInfoCalls, client.rechargeCalls)
	}
	if status := manager.Status(); status.Generation != authorized.Generation || status.User.ServerExpireTime != got.ExpireTime {
		t.Fatalf("Status() = %#v", status)
	}
	if events.last().User.ServerExpireTime != got.ExpireTime {
		t.Fatalf("last event = %#v", events.last())
	}
}
~~~

Record every `LoginInput` in the fake. Make its first `UserInfo` call return `user` and later calls return `renewedUser`.

- [ ] **Step 2: Run focused tests and verify RED**

~~~powershell
go test ./internal/license -run 'TestManagerRenew' -count=1
~~~

Expected: compilation fails because the renewal DTOs and manager method do not exist.

- [ ] **Step 3: Define safe DTOs**

Add to `internal/license/types.go`:

~~~go
type RenewalRequest struct {
	RechargeCard string `json:"rechargeCard"`
}

type RenewalResult struct {
	Renewed    bool   `json:"renewed"`
	Refreshed  bool   `json:"refreshed"`
	ExpireTime string `json:"expireTime,omitempty"`
	ErrorCode  string `json:"errorCode,omitempty"`
	Message    string `json:"message,omitempty"`
}
~~~

- [ ] **Step 4: Add active-session secret ownership**

Add to `Manager`:

~~~go
clientOps          sync.Mutex
currentCard        string
deviceID           string
renewingGeneration uint64
~~~

Change the authorized completion call to:

~~~go
m.completeAuthorizedAttempt(attemptID, client, card, deviceID, result)
~~~

Change the method signature and assignments:

~~~go
func (m *Manager) completeAuthorizedAttempt(
	attemptID uint64,
	client Client,
	card string,
	deviceID string,
	status Status,
) (Status, bool) {
	m.transitionMu.Lock()
	m.mu.Lock()
	if m.attemptID != attemptID || m.status.Phase != PhaseAuthorizing {
		current := m.status
		m.mu.Unlock()
		m.transitionMu.Unlock()
		return current, false
	}
	m.client = client
	m.currentCard = card
	m.deviceID = deviceID
	m.cancel = m.attemptCancel
	m.attemptID = 0
	m.attemptCancel = nil
	m.status = status
	m.mu.Unlock()
	m.publish(status)
	m.transitionMu.Unlock()
	return status, true
}
~~~

Add:

~~~go
func (m *Manager) clearSessionSecretsLocked() {
	m.currentCard = ""
	m.deviceID = ""
	m.renewingGeneration = 0
}

func (m *Manager) pong(client Client) error {
	m.clientOps.Lock()
	defer m.clientOps.Unlock()
	return client.Pong()
}

func (m *Manager) logout(client Client) {
	m.clientOps.Lock()
	defer m.clientOps.Unlock()
	_ = client.Logout()
}
~~~

Call `clearSessionSecretsLocked` wherever an active client is detached during explicit or heartbeat revocation. Replace manager-owned active-session `client.Pong()` and `client.Logout()` calls with `m.pong(client)` and `m.logout(client)`.

- [ ] **Step 5: Implement the renewal coordinator**

Create `internal/license/renewal.go`:

~~~go
package license

import (
	"context"
	"errors"
	"strings"
)

func (m *Manager) Renew(_ context.Context, request RenewalRequest) RenewalResult {
	rechargeCard := strings.TrimSpace(request.RechargeCard)
	if rechargeCard == "" {
		return RenewalResult{
			ErrorCode: "recharge_card_required",
			Message:   "recharge card is required",
		}
	}

	generation, client, currentCard, deviceID, failure := m.beginRenewal()
	if failure.ErrorCode != "" {
		return failure
	}
	defer m.finishRenewal(generation)

	m.clientOps.Lock()
	rechargeErr := client.RechargeCard(currentCard, rechargeCard)
	if rechargeErr != nil {
		m.clientOps.Unlock()
		return renewalFailure(rechargeErr)
	}
	_, loginErr := client.Login(LoginInput{Card: currentCard, DeviceID: deviceID})
	if loginErr != nil {
		m.clientOps.Unlock()
		return renewalRefreshFailure()
	}
	user, userErr := client.UserInfo()
	m.clientOps.Unlock()
	if userErr != nil {
		return renewalRefreshFailure()
	}

	m.transitionMu.Lock()
	m.mu.Lock()
	if !m.status.Authorized || m.status.Generation != generation || m.client != client {
		m.mu.Unlock()
		m.transitionMu.Unlock()
		return RenewalResult{
			Renewed:   true,
			ErrorCode: "license_session_changed",
			Message:   "license renewed but session changed",
		}
	}
	m.status.User = user
	status := m.status
	m.mu.Unlock()
	m.publish(status)
	m.transitionMu.Unlock()

	return RenewalResult{
		Renewed:    true,
		Refreshed:  true,
		ExpireTime: user.ServerExpireTime,
		Message:    "license renewed",
	}
}

func (m *Manager) beginRenewal() (uint64, Client, string, string, RenewalResult) {
	m.transitionMu.Lock()
	defer m.transitionMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.status.Authorized || m.status.Phase != PhaseAuthorized ||
		m.client == nil || m.currentCard == "" || m.deviceID == "" {
		return 0, nil, "", "", RenewalResult{
			ErrorCode: "license_required",
			Message:   "license is required",
		}
	}
	if m.renewingGeneration != 0 {
		return 0, nil, "", "", RenewalResult{
			ErrorCode: "license_renewal_in_progress",
			Message:   "license renewal is already in progress",
		}
	}
	m.renewingGeneration = m.status.Generation
	return m.status.Generation, m.client, m.currentCard, m.deviceID, RenewalResult{}
}

func (m *Manager) finishRenewal(generation uint64) {
	m.mu.Lock()
	if m.renewingGeneration == generation {
		m.renewingGeneration = 0
	}
	m.mu.Unlock()
}

func renewalFailure(err error) RenewalResult {
	var serviceErr *ServiceError
	if errors.As(err, &serviceErr) {
		return RenewalResult{ErrorCode: serviceErr.Code, Message: serviceErr.Message}
	}
	return RenewalResult{
		ErrorCode: "license_recharge_failed",
		Message:   "license recharge failed",
	}
}

func renewalRefreshFailure() RenewalResult {
	return RenewalResult{
		Renewed:   true,
		ErrorCode: "license_refresh_failed",
		Message:   "license renewed but refresh failed",
	}
}
~~~

- [ ] **Step 6: Run complete-success tests and verify GREEN**

~~~powershell
go test ./internal/license -run 'TestManagerRenew(Rejects|Recharges)' -count=1
~~~

Expected: both tests pass.

- [ ] **Step 7: Write failing outcome and concurrency tests**

Add concrete tests with channel synchronization:

~~~go
func TestManagerRenewReportsRechargeFailureWithoutRefreshing(t *testing.T) {
	client := managerSuccessfulClient()
	client.rechargeErr = serviceError("license_recharge_failed", "充值卡密不可用")
	manager := newTestManager(client, &managerFakeRuntime{}, &managerFakeCredentials{})
	manager.Login(context.Background(), LoginRequest{Card: "old-card"})

	got := manager.Renew(context.Background(), RenewalRequest{RechargeCard: "new-card"})

	if got.Renewed || got.Refreshed || got.ErrorCode != "license_recharge_failed" {
		t.Fatalf("Renew() = %#v", got)
	}
	if client.loginCalls != 1 || client.userInfoCalls != 1 || client.rechargeCalls != 1 {
		t.Fatalf("unexpected refresh calls: %#v", client)
	}
}

func TestManagerRenewReportsPartialSuccessWithoutRetryingRecharge(t *testing.T) {
	client := managerSuccessfulClient()
	client.renewLoginErr = serviceError("card_rejected", "refresh failed")
	manager := newTestManager(client, &managerFakeRuntime{}, &managerFakeCredentials{})
	manager.Login(context.Background(), LoginRequest{Card: "old-card"})

	got := manager.Renew(context.Background(), RenewalRequest{RechargeCard: "new-card"})

	if !got.Renewed || got.Refreshed || got.ErrorCode != "license_refresh_failed" {
		t.Fatalf("Renew() = %#v", got)
	}
	if client.rechargeCalls != 1 {
		t.Fatalf("RechargeCard() calls = %d, want 1", client.rechargeCalls)
	}
}

func TestManagerRenewRejectsConcurrentRenewals(t *testing.T) {
	client := managerSuccessfulClient()
	client.rechargeStarted = make(chan struct{})
	client.allowRecharge = make(chan struct{})
	manager := newTestManager(client, &managerFakeRuntime{}, &managerFakeCredentials{})
	manager.Login(context.Background(), LoginRequest{Card: "old-card"})

	firstDone := make(chan RenewalResult, 1)
	go func() {
		firstDone <- manager.Renew(context.Background(), RenewalRequest{RechargeCard: "first"})
	}()
	<-client.rechargeStarted

	second := manager.Renew(context.Background(), RenewalRequest{RechargeCard: "second"})
	if second.ErrorCode != "license_renewal_in_progress" {
		t.Fatalf("second Renew() = %#v", second)
	}
	close(client.allowRecharge)
	<-firstDone
}

func TestManagerRenewSerializesWithHeartbeatPong(t *testing.T) {
	client := managerSuccessfulClient()
	client.pongStarted = make(chan struct{})
	client.allowPong = make(chan struct{})
	client.rechargeStarted = make(chan struct{})
	manager := newTestManager(client, &managerFakeRuntime{}, &managerFakeCredentials{})
	status := manager.Login(context.Background(), LoginRequest{Card: "old-card"})

	pongDone := make(chan struct{})
	go func() {
		m.handleHeartbeatResult(status.Generation, manager.pong(client))
		close(pongDone)
	}()
	<-client.pongStarted

	renewDone := make(chan RenewalResult, 1)
	go func() {
		renewDone <- manager.Renew(context.Background(), RenewalRequest{RechargeCard: "new-card"})
	}()
	select {
	case <-client.rechargeStarted:
		t.Fatal("RechargeCard overlapped Pong")
	case <-time.After(20 * time.Millisecond):
	}
	close(client.allowPong)
	<-pongDone
	if got := <-renewDone; !got.Renewed {
		t.Fatalf("Renew() = %#v", got)
	}
}

func TestManagerRevokeClearsActiveCard(t *testing.T) {
	client := managerSuccessfulClient()
	manager := newTestManager(client, &managerFakeRuntime{}, &managerFakeCredentials{})
	manager.Login(context.Background(), LoginRequest{Card: "old-card"})
	manager.Revoke(context.Background(), "test")

	got := manager.Renew(context.Background(), RenewalRequest{RechargeCard: "new-card"})
	if got.ErrorCode != "license_required" || client.rechargeCalls != 0 {
		t.Fatalf("Renew() after revoke = %#v, calls=%d", got, client.rechargeCalls)
	}
}
~~~

Add the generation-change test with revoke running concurrently so it cannot deadlock behind the serialized SDK operation:

~~~go
func TestManagerRenewDiscardsRefreshAfterGenerationChanges(t *testing.T) {
	client := managerSuccessfulClient()
	client.renewedUser = UserInfo{ID: "user-1", ServerExpireTime: "2027-12-31 23:59:59"}
	client.renewUserInfoStarted = make(chan struct{})
	client.allowRenewUserInfo = make(chan struct{})
	manager := newTestManager(client, &managerFakeRuntime{}, &managerFakeCredentials{})
	manager.Login(context.Background(), LoginRequest{Card: "old-card"})

	renewDone := make(chan RenewalResult, 1)
	go func() {
		renewDone <- manager.Renew(context.Background(), RenewalRequest{RechargeCard: "new-card"})
	}()
	<-client.renewUserInfoStarted

	revokeDone := make(chan Status, 1)
	go func() {
		revokeDone <- manager.Revoke(context.Background(), "test")
	}()
	for manager.Status().Phase != PhaseRevoking {
		time.Sleep(time.Millisecond)
	}
	close(client.allowRenewUserInfo)

	result := <-renewDone
	if !result.Renewed || result.Refreshed || result.ErrorCode != "license_session_changed" {
		t.Fatalf("Renew() = %#v", result)
	}
	if revoked := <-revokeDone; revoked.Phase != PhaseLocked || revoked.Authorized {
		t.Fatalf("Revoke() = %#v", revoked)
	}
	if got := manager.Status(); got.User.ServerExpireTime == "2027-12-31 23:59:59" {
		t.Fatalf("stale renewal updated status: %#v", got)
	}
}
~~~

The fake exposes `renewUserInfoStarted` and `allowRenewUserInfo`; only its second `UserInfo` call signals and waits on those channels.

- [ ] **Step 8: Run edge-case tests and complete fake synchronization**

~~~powershell
go test ./internal/license -run 'TestManagerRenew|TestManagerRevokeClearsActiveCard' -count=1
~~~

Expected before completing fake controls: the new concurrency tests fail. Add locked fake counters, second-login error selection, recharge channels, and second-user-info channels. Rerun until every focused test passes.

- [ ] **Step 9: Run race-aware package tests**

~~~powershell
go test -race ./internal/license -count=1
~~~

Expected: PASS with no race report.

- [ ] **Step 10: Commit manager renewal**

~~~powershell
git add internal/license/types.go internal/license/manager.go internal/license/renewal.go internal/license/manager_test.go
git commit -m "feat: coordinate active license renewal"
~~~

### Task 3: Protected Wails Renewal API

**Files:**
- Modify: `license_api.go`
- Modify: `license_api_test.go`
- Modify: `authorization_gate.go`
- Modify: `authorization_gate_test.go`
- Modify: `frontend/wailsjs/go/main/App.js`
- Modify: `frontend/wailsjs/go/main/App.d.ts`
- Modify: `frontend/wailsjs/go/models.ts`

- [ ] **Step 1: Write failing API and authorization tests**

Add:

~~~go
func TestLicenseRenewIsSafeBeforeBootstrap(t *testing.T) {
	result := NewApp().LicenseRenew(license.RenewalRequest{
		RechargeCard: "new-card-must-not-leak",
	})
	if result.Renewed ||
		result.ErrorCode != "license_required" ||
		strings.Contains(result.Message, "new-card-must-not-leak") {
		t.Fatalf("LicenseRenew() = %#v", result)
	}
}
~~~

Do not add `LicenseRenew` to the bootstrap allowlist. The existing reflection-based registry test requires the exported method to have a policy and fails if it is misclassified.

- [ ] **Step 2: Run API tests and verify RED**

~~~powershell
go test . -run 'TestLicenseRenew|TestAppAuthorizationRegistry' -count=1
~~~

Expected: compilation or registry failure because `App.LicenseRenew` and its policy do not exist.

- [ ] **Step 3: Add the protected method**

Add to `license_api.go`:

~~~go
func (a *App) LicenseRenew(input license.RenewalRequest) license.RenewalResult {
	if a.requireAuthorized("LicenseRenew") != nil || a.licenseManager == nil {
		return license.RenewalResult{
			ErrorCode: "license_required",
			Message:   ErrLicenseRequired.Error(),
		}
	}
	return a.licenseManager.Renew(a.contextOrBackground(), input)
}
~~~

Add to `appAuthorizationPolicies`:

~~~go
"LicenseRenew": authorizationRequired,
~~~

- [ ] **Step 4: Update generated-style bindings**

Add to `App.js`:

~~~js
export function LicenseRenew(arg1) {
  return window['go']['main']['App']['LicenseRenew'](arg1);
}
~~~

Add to `App.d.ts`:

~~~ts
export function LicenseRenew(arg1:license.RenewalRequest):Promise<license.RenewalResult>;
~~~

Add under `export namespace license` in `models.ts`:

~~~ts
export class RenewalRequest {
    rechargeCard: string;

    static createFrom(source: any = {}) {
        return new RenewalRequest(source);
    }

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.rechargeCard = source["rechargeCard"];
    }
}

export class RenewalResult {
    renewed: boolean;
    refreshed: boolean;
    expireTime?: string;
    errorCode?: string;
    message?: string;

    static createFrom(source: any = {}) {
        return new RenewalResult(source);
    }

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.renewed = source["renewed"];
        this.refreshed = source["refreshed"];
        this.expireTime = source["expireTime"];
        this.errorCode = source["errorCode"];
        this.message = source["message"];
    }
}
~~~

- [ ] **Step 5: Run API tests and frontend type-check**

~~~powershell
go test . -run 'TestLicenseRenew|TestAppAuthorizationRegistry' -count=1
Set-Location frontend
npm run build
~~~

Expected: API tests and TypeScript build pass.

- [ ] **Step 6: Commit the API boundary**

~~~powershell
git add license_api.go license_api_test.go authorization_gate.go authorization_gate_test.go frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/models.ts
git commit -m "feat: expose protected license renewal API"
~~~

### Task 4: Renewal Dialog Behavior

**Files:**
- Create: `frontend/src/components/LicenseRenewalDialog.tsx`
- Create: `frontend/src/components/LicenseRenewalDialog.test.tsx`

- [ ] **Step 1: Write failing component tests**

Use `react-test-renderer`. Define the component contract as:

~~~tsx
<LicenseRenewalDialog
  open
  onClose={onClose}
  onRenew={async (rechargeCard) => result}
/>
~~~

Add these concrete assertions:

~~~tsx
it('disables confirmation until a new card is entered', () => {
  const renderer = create(
    <LicenseRenewalDialog open onClose={() => undefined} onRenew={vi.fn()} />,
  );
  expect(renderer.root.findByProps({ name: 'confirm-license-renewal' }).props.disabled).toBe(true);
});

it('shows complete success and clears the sensitive input', async () => {
  const renew = vi.fn(async () => ({
    renewed: true,
    refreshed: true,
    expireTime: '2027-12-31 23:59:59',
  }));
  const renderer = create(
    <LicenseRenewalDialog open onClose={() => undefined} onRenew={renew} />,
  );
  act(() => renderer.root.findByProps({ name: 'recharge-card' }).props.onChange({
    target: { value: ' new-card ' },
  }));
  await act(async () => {
    await renderer.root.findByProps({ name: 'confirm-license-renewal' }).props.onClick();
  });

  expect(renew).toHaveBeenCalledWith('new-card');
  expect(renderer.root.findByProps({ name: 'recharge-card' }).props.value).toBe('');
  const output = JSON.stringify(renderer.toJSON());
  expect(output).toContain('续费成功');
  expect(output).toContain('2027-12-31 23:59');
});
~~~

Add explicit failure and partial-success tests:

~~~tsx
it('retains the card after recharge failure so it can be corrected', async () => {
  const renew = vi.fn(async () => ({
    renewed: false,
    refreshed: false,
    errorCode: 'license_recharge_failed',
  }));
  const renderer = create(
    <LicenseRenewalDialog open onClose={() => undefined} onRenew={renew} />,
  );
  act(() => renderer.root.findByProps({ name: 'recharge-card' }).props.onChange({
    target: { value: 'bad-card' },
  }));
  await act(async () => {
    await renderer.root.findByProps({ name: 'confirm-license-renewal' }).props.onClick();
  });

  expect(renderer.root.findByProps({ name: 'recharge-card' }).props.value).toBe('bad-card');
  expect(JSON.stringify(renderer.toJSON())).toContain('续费失败');
});

it('does not offer recharge again after partial success', async () => {
  const renderer = create(
    <LicenseRenewalDialog
      open
      onClose={() => undefined}
      onRenew={async () => ({ renewed: true, refreshed: false, errorCode: 'license_refresh_failed' })}
    />,
  );
  act(() => renderer.root.findByProps({ name: 'recharge-card' }).props.onChange({
    target: { value: 'consumed-card' },
  }));
  await act(async () => {
    await renderer.root.findByProps({ name: 'confirm-license-renewal' }).props.onClick();
  });

  expect(JSON.stringify(renderer.toJSON())).toContain('续费已成功，但到期时间刷新失败');
  expect(renderer.root.findAllByProps({ name: 'confirm-license-renewal' })).toHaveLength(0);
  expect(renderer.root.findAllByProps({ name: 'recharge-card' })).toHaveLength(0);
});
~~~

Add visibility, pending-state, and reset tests:

~~~tsx
it('toggles new-card visibility', () => {
  const renderer = create(
    <LicenseRenewalDialog open onClose={() => undefined} onRenew={vi.fn()} />,
  );
  expect(renderer.root.findByProps({ name: 'recharge-card' }).props.type).toBe('password');
  act(() => renderer.root.findByProps({ name: 'toggle-recharge-card-visibility' }).props.onClick());
  expect(renderer.root.findByProps({ name: 'recharge-card' }).props.type).toBe('text');
});

it('locks every dialog command while renewal is pending', async () => {
  let resolveRenew!: (value: RenewalResultDto) => void;
  const renew = vi.fn(() => new Promise<RenewalResultDto>((resolve) => {
    resolveRenew = resolve;
  }));
  const renderer = create(
    <LicenseRenewalDialog open onClose={() => undefined} onRenew={renew} />,
  );
  act(() => renderer.root.findByProps({ name: 'recharge-card' }).props.onChange({
    target: { value: 'new-card' },
  }));
  let pending!: Promise<void>;
  act(() => {
    pending = renderer.root.findByProps({ name: 'confirm-license-renewal' }).props.onClick();
  });

  expect(renderer.root.findByProps({ name: 'recharge-card' }).props.disabled).toBe(true);
  expect(renderer.root.findByProps({ name: 'close-license-renewal' }).props.disabled).toBe(true);
  expect(renderer.root.findByProps({ name: 'toggle-recharge-card-visibility' }).props.disabled).toBe(true);
  expect(renderer.root.findByProps({ name: 'confirm-license-renewal' }).props.disabled).toBe(true);

  await act(async () => {
    resolveRenew({ renewed: true, refreshed: true, expireTime: '2027-12-31 23:59:59' });
    await pending;
  });
});

it('clears sensitive and transient state after close and reopen', () => {
  const renew = vi.fn(async () => ({ renewed: false, refreshed: false, errorCode: 'license_recharge_failed' }));
  const onClose = vi.fn();
  const renderer = create(<LicenseRenewalDialog open onClose={onClose} onRenew={renew} />);
  act(() => renderer.root.findByProps({ name: 'recharge-card' }).props.onChange({
    target: { value: 'new-card' },
  }));
  act(() => renderer.update(<LicenseRenewalDialog open={false} onClose={onClose} onRenew={renew} />));
  act(() => renderer.update(<LicenseRenewalDialog open onClose={onClose} onRenew={renew} />));

  expect(renderer.root.findByProps({ name: 'recharge-card' }).props.value).toBe('');
  expect(JSON.stringify(renderer.toJSON())).not.toContain('续费失败');
});
~~~

- [ ] **Step 2: Run dialog tests and verify RED**

~~~powershell
Set-Location frontend
npm test -- src/components/LicenseRenewalDialog.test.tsx
~~~

Expected: FAIL because the component does not exist.

- [ ] **Step 3: Implement the dialog contract**

Create `LicenseRenewalDialog.tsx` with:

~~~tsx
import { useEffect, useState } from 'react';
import { CreditCard, Eye, EyeOff, X } from 'lucide-react';

export type RenewalResultDto = {
  renewed: boolean;
  refreshed: boolean;
  expireTime?: string;
  errorCode?: string;
  message?: string;
};

type LicenseRenewalDialogProps = {
  open: boolean;
  onClose: () => void;
  onRenew: (rechargeCard: string) => Promise<RenewalResultDto>;
};
~~~

Keep `rechargeCard`, `visible`, `submitting`, and `result` state. Reset them when `open` becomes false. The submit function trims the card, awaits `onRenew`, clears the card only when `result.renewed` is true, converts thrown errors into `license_recharge_failed`, and always releases `submitting`.

Render these stable controls:

~~~tsx
<input
  name="recharge-card"
  type={visible ? 'text' : 'password'}
  value={rechargeCard}
  disabled={submitting}
  onChange={(event) => setRechargeCard(event.target.value)}
/>
<button
  name="toggle-recharge-card-visibility"
  type="button"
  aria-label={visible ? '隐藏新卡密' : '显示新卡密'}
  disabled={submitting}
  onClick={() => setVisible((value) => !value)}
>
  {visible ? <EyeOff size={17} /> : <Eye size={17} />}
</button>
<button
  name="confirm-license-renewal"
  type="button"
  disabled={submitting || !rechargeCard.trim()}
  onClick={submit}
>
  {submitting ? '正在续费' : '确定续费'}
</button>
~~~

Complete success displays `续费成功` and `formatRenewalExpiry(result.expireTime)`. Partial success displays `续费已成功，但到期时间刷新失败` and only a `知道了` command. Recharge failure maps known error codes to Chinese, retains the input, and leaves `确定续费` available.

- [ ] **Step 4: Run dialog tests and verify GREEN**

~~~powershell
npm test -- src/components/LicenseRenewalDialog.test.tsx
~~~

Expected: all dialog tests pass.

- [ ] **Step 5: Commit dialog behavior**

~~~powershell
git add frontend/src/components/LicenseRenewalDialog.tsx frontend/src/components/LicenseRenewalDialog.test.tsx
git commit -m "feat: add license renewal dialog"
~~~

### Task 5: Workbench Entry and Styling

**Files:**
- Modify: `frontend/src/views/OverviewView.tsx`
- Modify: `frontend/src/views/OverviewView.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write failing workbench tests**

Add:

~~~tsx
it('renders the renewal entry beside the announcement command', () => {
  const html = renderToStaticMarkup(
    <OverviewView
      status={readyStatus}
      licenseStatus={authorizedLicense}
      events={[]}
      onRefresh={() => undefined}
    />,
  );
  expect(html).toContain('续费');
  expect(html).toContain('lucide-credit-card');
  expect(html).toContain('查看公告');
});

it('keeps the renewal dialog usable on narrow screens', () => {
  const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
  const mobileStyles = mediaDeclarations(css, 560);
  expect(ruleDeclarations(css, '.license-renewal-dialog')).toMatch(
    /width:\s*min\(460px,\s*calc\(100vw\s*-\s*40px\)\)/,
  );
  expect(ruleDeclarations(mobileStyles, '.license-renewal-actions')).toMatch(
    /flex-direction:\s*column-reverse/,
  );
});
~~~

Extend the safe-boundary source test with exact assertions:

~~~tsx
expect(overviewSource).toContain("import { GetProgramNotice, LicenseRenew } from '../../wailsjs/go/main/App';");
expect(overviewSource).toContain('LicenseRenew({ rechargeCard })');
expect(overviewSource).not.toMatch(/LicenseRenew\(\{[^}]*currentCard/);
~~~

- [ ] **Step 2: Run workbench tests and verify RED**

~~~powershell
Set-Location frontend
npm test -- src/views/OverviewView.test.tsx
~~~

Expected: FAIL because the entry and CSS do not exist.

- [ ] **Step 3: Wire the command and API callback**

Update imports:

~~~tsx
import { CreditCard, Megaphone, RefreshCw } from 'lucide-react';
import {
  LicenseRenewalDialog,
  type RenewalResultDto,
} from '../components/LicenseRenewalDialog';
import { GetProgramNotice, LicenseRenew } from '../../wailsjs/go/main/App';
~~~

Add state and callback:

~~~tsx
const [renewalOpen, setRenewalOpen] = useState(false);

const renewLicense = useCallback(async (rechargeCard: string) => {
  return await LicenseRenew({ rechargeCard }) as RenewalResultDto;
}, []);
~~~

Place immediately before `查看公告`:

~~~tsx
<button
  className="secondary-button"
  type="button"
  onClick={() => setRenewalOpen(true)}
  title="卡密续费"
>
  <CreditCard size={16} />
  <span>续费</span>
</button>
~~~

Render next to `ProgramNoticeDialog`:

~~~tsx
<LicenseRenewalDialog
  open={renewalOpen}
  onClose={() => setRenewalOpen(false)}
  onRenew={renewLicense}
/>
~~~

- [ ] **Step 4: Add responsive styles**

Add beside the program-notice rules:

~~~css
.license-renewal-backdrop {
  z-index: 31;
}

.license-renewal-dialog {
  width: min(460px, calc(100vw - 40px));
  max-height: calc(100vh - 40px);
  overflow: auto;
  padding: 20px;
  border: 1px solid #cbd8d0;
  border-radius: 8px;
  color: #20271f;
  background: #ffffff;
  box-shadow: 0 24px 64px rgba(28, 42, 32, 0.2);
}

.license-renewal-header,
.license-renewal-heading,
.license-renewal-actions,
.license-renewal-input-row {
  display: flex;
  align-items: center;
}

.license-renewal-header {
  justify-content: space-between;
  gap: 16px;
}

.license-renewal-body {
  margin-top: 18px;
}

.license-renewal-input-row {
  gap: 8px;
}

.license-renewal-input-row input {
  min-width: 0;
  min-height: 40px;
  flex: 1;
}

.license-renewal-result {
  min-height: 48px;
  margin-top: 12px;
  overflow-wrap: anywhere;
}

.license-renewal-result-success {
  color: #287a34;
}

.license-renewal-result-error {
  color: #9b2f1f;
}

.license-renewal-actions {
  justify-content: flex-end;
  gap: 9px;
  margin-top: 18px;
}

@media (max-width: 560px) {
  .license-renewal-actions {
    align-items: stretch;
    flex-direction: column-reverse;
  }
}
~~~

Reuse `.dialog-backdrop`, `.primary-button`, `.secondary-button`, and `.icon-button`.

- [ ] **Step 5: Run focused tests and build**

~~~powershell
npm test -- src/components/LicenseRenewalDialog.test.tsx src/views/OverviewView.test.tsx
npm run build
~~~

Expected: tests and production build pass.

- [ ] **Step 6: Commit workbench integration**

~~~powershell
git add frontend/src/views/OverviewView.tsx frontend/src/views/OverviewView.test.tsx frontend/src/style.css
git commit -m "feat: add workbench license renewal entry"
~~~

### Task 6: Full Verification and Acceptance

**Files:**
- Verify only; modify only files already listed when a failing check identifies a defect.

- [ ] **Step 1: Run all Go tests**

~~~powershell
go test ./... -count=1
~~~

Expected: every Go package passes.

- [ ] **Step 2: Run race-sensitive tests**

~~~powershell
go test -race ./internal/license . -count=1
~~~

Expected: PASS with no race report.

- [ ] **Step 3: Run frontend tests and production build**

From `frontend`:

~~~powershell
npm test
npm run build
~~~

Expected: all Vitest suites pass and Vite produces the bundle.

- [ ] **Step 4: Audit credential leakage**

~~~powershell
rg -n "old-card-must-not-leak|new-card-must-not-leak" . -g '!**/*_test.go' -g '!**/*.test.tsx' -g '!docs/**' -g '!graphify-out/**'
~~~

Expected: no matches.

- [ ] **Step 5: Start the development app and inspect the interaction**

~~~powershell
wails dev
~~~

With an authorized disposable test card, verify the workbench command opens the dialog, desktop and narrow widths do not overlap, empty submission is disabled, controls remain stable while submitting, complete success shows the new expiration time, and close/reopen clears the new card.

- [ ] **Step 6: Perform opt-in live KAuth acceptance only with disposable cards**

Use one active old card and one dedicated recharge card. Confirm KAuth consumes the recharge card once, the old card logs in once after recharge, the dialog and workbench show the same new expiration time, and no log prints either card or Token. If disposable cards are unavailable, record this live check as not run and do not substitute production credentials.

- [ ] **Step 7: Review final diff and history**

~~~powershell
git diff --check
git status --short
git log -6 --oneline
~~~

Expected: no whitespace errors, only intentional changes remain, and the task commits are present.
