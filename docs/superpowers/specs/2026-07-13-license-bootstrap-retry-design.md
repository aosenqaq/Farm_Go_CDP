# License Bootstrap Retry Design

Date: 2026-07-13

## Problem

The release executable can validate a card successfully, so its KAuth build
configuration is valid. During cold startup, however, the frontend can call
the authorization status and remembered-card APIs before the Go authorization
manager has completed initialization. The frontend currently consumes those
one-shot results, showing an unavailable authorization error and losing the
remembered-card prefill for the remainder of the session.

## Decision

Keep the authorization backend unchanged. The card gate always opens in the
quiet locked state and does not request authorization status or perform online
verification during startup. The frontend retries only the local
`RememberedLicenseCard` read after a short, bounded delay until it receives a
saved card. Online verification remains exclusively behind the user's
"verify and continue" action.

## Behavior

- The card gate immediately displays without an authorization-service error.
- Startup does not call the authorization status API.
- A saved card is populated when the local credential store becomes ready.
- An empty or unavailable local credential response is retried only in the
  background and never changes the gate into an authorization error.
- Unmounting prevents stale remembered-card updates.

## Verification

Add an App bootstrap test that confirms no authorization status request is
made. Return an empty remembered-card response first, then a saved card.
Advance the retry timer and assert the gate is quiet, the saved card is
prefilled, and the checkbox is selected. Run the frontend test suite and
production build.
