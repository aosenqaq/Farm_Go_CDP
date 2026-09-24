# Read-Only Runtime Spy Isolation

## Goal

Account identification and account-status reads must not install runtime Spy hooks in the game process. Existing explicit diagnostics remain available.

## Scope

- Remove implicit `installRuntimeSpies()` calls from the read-only profile path in `resources/wmpf/button.js`.
- Preserve `gameCtl.startRuntimeSpies()` and all existing diagnostic APIs that explicitly request Spy data.
- Add a focused regression test against the embedded runtime script.

## Non-Goals

- Do not alter QQ host auto-connect, CDP bridge setup, Frida configuration, guardian worker behavior, or existing explicit packet-capture diagnostics.
- Do not add a full Spy restore API in this change. That requires recording each wrapped object and function, and is a separate higher-risk change.

## Design

`getPlayerProfile()` and `getProtocolAccountProfile()` are data readers. They will no longer call `installRuntimeSpies()`.

The normal account paths remain unchanged:

`IdentifyRuntimeAccount` -> `gameCtl.getRuntimeAccountIdentity` -> `getPlayerProfile`

`FarmAccountStatus` -> `gameCtl.getPlayerProfile`

After the change, these calls only inspect existing game state. Spy installation remains reachable only through the explicit `gameCtl.startRuntimeSpies()` and diagnostic APIs that already call `installRuntimeSpies()` by name.

## Error Handling

Removing the implicit setup does not change the readers' existing fallback and error behavior. Reads can still return an unavailable-runtime error or partial profile data as they do now.

## Verification

- Add a regression test that extracts the bodies of the two read-only functions from `runtimeButtonScript` and asserts that neither contains `installRuntimeSpies()`.
- Assert that the explicit `startRuntimeSpies()` API still contains the installer call.
- Run the focused root-package test plus the existing WMPF runtime tests.

## Acceptance Criteria

1. Account identification and account-status reads no longer install Spy hooks.
2. Explicit Spy diagnostics remain available.
3. No Go API signature, UI workflow, runtime target configuration, or transport startup behavior changes.
