# KAuth Card Renewal Design

## 1. Goal

Add a renewal entry to the workbench. An authorized user can enter a new card, use it to renew the currently authorized card, and immediately see the renewal result and refreshed expiration time.

The implementation uses KAuth Go SDK `v1.0.0` method `RechargeKa`. After a successful recharge, the active card is logged in once more and `GetUserInfo` is queried to obtain the new expiration time.

## 2. Confirmed Decisions

- Place a `续费` command in the workbench header beside `查看公告`.
- Open a modal that asks only for the new recharge card.
- Use the currently authorized card as `CardPwd` and the entered card as `RechargeCardPwd`.
- After recharge succeeds, log in once with the current card and query user information.
- Update the shared license status so the modal and workbench expiration display use the same server result.
- Keep the current card only in backend memory for the active authorization session.
- Never persist the new recharge card or expose either card in status DTOs, events, errors, or logs.

## 3. SDK Capability

KAuth exposes:

```go
func (api *KauthApi) RechargeKa(req KaRechargeKaReq) (*Response, error)

type KaRechargeKaReq struct {
    CardPwd         string `json:"cardPwd"`
    RechargeCardPwd string `json:"rechargeCardPwd"`
}
```

The SDK treats response code `200` as success and converts other response codes into `ApiError`. The project adapter must classify these errors into safe internal codes without returning raw request or response content.

## 4. Architecture

The renewal operation belongs to `internal/license.Manager`, because it needs the active SDK client, current card, device ID, session generation, and shared license status.

```text
Workbench renewal button
        |
        v
Renewal dialog -- new card --> Wails LicenseRenew API
                                      |
                                      v
                          internal/license.Manager
                           - validate active session
                           - RechargeKa(old, new)
                           - Login(old, device ID) once
                           - UserInfo()
                           - update and publish Status
                                      |
                         +------------+------------+
                         |                         |
                         v                         v
               renewal result DTO       license:status event
                         |                         |
                         v                         v
                dialog result view       workbench expiration
```

The existing KAuth client adapter gains a narrowly scoped card recharge operation. The manager remains the only component that coordinates authorization state changes.

## 5. Backend Design

### 5.1 Client Adapter

Extend the internal `Client` interface with a recharge operation accepting the target card and recharge card. The production adapter calls `KauthApi.RechargeKa`.

The adapter returns no raw KAuth response. It maps SDK failures to stable internal service errors. Error values and messages must not contain either card, the Token, request bodies, response bodies, signatures, or cryptographic configuration.

### 5.2 Active Card Lifetime

After login and runtime activation succeed, the manager retains the trimmed current card in memory as part of the active session. It is never included in `Status`.

The in-memory card is cleared when:

- login or activation fails;
- the session is explicitly revoked;
- heartbeat revocation starts;
- runtime teardown completes;
- the application shuts down.

The existing remembered-card setting remains unchanged. If enabled, DPAPI persistence continues to follow the current credential-store behavior; renewal adds no new persisted credential.

### 5.3 Renewal Operation

Expose a manager method that accepts only the new card and returns a safe result DTO containing:

- whether recharge succeeded;
- whether expiration refresh succeeded;
- the refreshed expiration time when available;
- a stable error code and user-facing message when needed.

The manager rejects empty input, unauthorized sessions, concurrent renewals, and results belonging to a replaced session generation.

For one accepted request:

1. Snapshot the active generation, client, current card, and device ID.
2. Serialize the SDK operation with heartbeat calls for that client.
3. Call `RechargeKa(currentCard, newCard)`.
4. If recharge fails, return a failed result and leave the current session unchanged.
5. If recharge succeeds, call `Login(currentCard, deviceID)` exactly once on the active client.
6. If login succeeds, call `UserInfo()` once.
7. If both refresh calls succeed and the generation is still current, replace `Status.User`, preserve authorization state, and publish `license:status`.
8. Return the same new expiration time in the renewal result.

The renewed login updates the SDK client's Token in place. Heartbeat and renewal SDK calls must use a shared operation lock so a Pong request cannot overlap the Token refresh.

The existing heartbeat interval remains unchanged. It is a program-level value and renewal does not create a second heartbeat worker.

### 5.4 Result Semantics

There are three terminal outcomes:

- Recharge failed: `renewed=false`, `refreshed=false`; show a safe failure message and allow retry.
- Recharge succeeded and refresh succeeded: `renewed=true`, `refreshed=true`; show success and the new expiration time.
- Recharge succeeded but login or user-info refresh failed: `renewed=true`, `refreshed=false`; state clearly that renewal succeeded but the expiration time could not be refreshed. Do not report the recharge as failed and do not automatically submit the recharge card again.

If the session generation changes while the request is in flight, the result must not update the replacement session. A recharge already accepted by KAuth is still reported truthfully, but stale user information is discarded.

### 5.5 Wails Boundary

Add an authorized Wails method that forwards the new card to the manager. It must return `license_required` when no active authorized session exists.

The method is protected by default and must not be added to the pre-authorization bootstrap whitelist. Generated Wails bindings expose only the safe request and result DTOs.

## 6. Frontend Design

### 6.1 Workbench Entry

Add a secondary `续费` button with the appropriate Lucide card icon beside `查看公告` in the workbench header. The command is available only in the authorized workbench.

### 6.2 Renewal Dialog

Create a dedicated renewal dialog consistent with the existing program notice and update dialogs.

The initial state contains:

- title `卡密续费`;
- one masked `新卡密` input;
- an eye icon to show or hide the value;
- `取消` and `确定续费` commands;
- a fixed-height inline result area to avoid layout shifts.

An empty or whitespace-only card cannot be submitted. During submission, the input, visibility toggle, close command, and confirm command are disabled so the request cannot be duplicated.

On complete success, the dialog replaces the form result area with `续费成功` and the formatted new expiration time. The card input is cleared. Closing and reopening the dialog always starts with an empty input.

On recharge failure, the dialog keeps the input available for correction and displays the safe failure message. On partial success, the dialog clears the recharge card and displays `续费已成功，但到期时间刷新失败`; it must not offer a one-click recharge retry that could consume the same card twice.

The dialog renders only the safe backend result and never shows the current card.

### 6.3 Status Synchronization

The existing `license:status` event remains the single source of truth for authorized status. After a successful refresh, `AppBootstrap` receives the updated status, passes it through `AuthorizedApp`, and the workbench expiration signal rerenders automatically.

The dialog uses the expiration value returned by the renewal request for its immediate result. It does not maintain a second long-lived license-status store.

## 7. Error Handling

Stable renewal error categories include:

- `recharge_card_required`;
- `license_required`;
- `license_renewal_in_progress`;
- `license_recharge_failed`;
- `license_refresh_failed`;
- `license_session_changed`.

Safe KAuth business messages may supplement the generic Chinese message only after confirming they contain no credential or protocol details. Network and protocol failures use local Chinese fallbacks.

No renewal path logs the current card, recharge card, Token, SDK request, SDK response, or cryptographic material.

## 8. Testing

### 8.1 Go Tests

- Client adapter sends the correct old and recharge cards to the SDK fake.
- Client adapter classifies recharge success and failure without leaking credentials.
- Manager rejects empty input, unauthorized calls, and concurrent renewals.
- A successful recharge performs exactly one old-card login and one user-info query.
- Complete success updates and publishes the new expiration time.
- Recharge failure does not perform refresh calls or change license status.
- Refresh failure reports partial success and does not retry recharge.
- Heartbeat Pong cannot overlap the Token-refresh sequence.
- A changed generation cannot receive stale refreshed user information.
- Revocation and teardown clear the in-memory current card.
- The Wails API is unavailable before authorization and is absent from the bootstrap whitelist.

### 8.2 Frontend Tests

- The workbench renders the renewal entry and opens the dialog.
- The confirm command is disabled for empty input.
- Password visibility and submitting states behave correctly.
- Duplicate submission is prevented.
- Complete success displays the refreshed expiration time and clears the new card.
- Recharge failure permits correction and retry.
- Partial success warns that recharge succeeded and does not offer recharge retry.
- Closing and reopening resets sensitive input and transient result state.
- The workbench expiration display updates when license status changes.

### 8.3 Verification

- Run focused Go license and Wails API tests.
- Run the full Go test suite.
- Run focused frontend component tests.
- Run the full frontend test suite and production build.
- Verify the dialog at desktop and narrow viewport widths.
- Perform an opt-in live KAuth renewal only with dedicated disposable test cards; never print either card or Token.

## 9. Non-Goals

- No account-based recharge flow.
- No purchase, card generation, or payment UI.
- No renewal history.
- No persistence of recharge cards.
- No automatic recharge retry.
- No changes to existing update, announcement, or remembered-card behavior.
