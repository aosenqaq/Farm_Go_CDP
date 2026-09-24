# PushPlus Protocol Adaptation Design

## Goal

Make the existing PushPlus message-push channel conform to the current PushPlus
send API without changing the saved configuration schema or the frontend workflow.

The supplied test token is an ephemeral verification credential. It must not be
written to source code, tests, documentation, configuration, logs, or commits.

## Current Gap

Farm_Go already exposes PushPlus in the channel selector, persists
`pushPlusToken`, renders text and Markdown templates, and sends JSON requests to
`https://www.pushplus.plus/send`.

Two protocol details are incomplete:

- Farm_Go's internal text template mode is serialized as `text`, while PushPlus
  requires the provider value `txt`.
- A successful HTTP status is currently treated as success without checking the
  PushPlus JSON response. PushPlus reports request validation failures through a
  non-200 `code` inside an HTTP 2xx response.

## Design

Keep `text` and `markdown` as Farm_Go's channel-independent template modes.
Translate them only while building a PushPlus request:

| Farm_Go mode | PushPlus `template` |
| --- | --- |
| `text` | `txt` |
| `markdown` | `markdown` |

No persisted values, generated Wails models, or frontend fields change.

Extend channel-response validation with a PushPlus branch. Decode the response
object's `code` and `msg` fields. Treat `code == 200` as accepted. For any other
code, return a channel-specific error containing the code and the provider
message when present. Malformed or unreadable PushPlus JSON is an error because
the client cannot prove the request was accepted.

The API remains asynchronous: `code == 200` means PushPlus accepted the request,
not that every downstream delivery channel completed successfully. Final
delivery polling or callbacks are outside this change.

## Data Flow

1. Existing service and template rendering select the PushPlus channel.
2. The sender builds the existing JSON body with token, title, and content.
3. The sender maps the internal template mode to the PushPlus provider value.
4. The HTTP request is sent through the existing timeout and retry path.
5. HTTP non-2xx responses fail as before.
6. HTTP 2xx responses are decoded and accepted only when PushPlus returns code
   200; otherwise the existing retry and result-recording behavior receives the
   validation error.

## Error Handling

- Unsupported internal modes remain rejected by the existing template
  capability validation.
- PushPlus business errors include the returned code and message in the channel
  error.
- Empty, malformed, or unreadable PushPlus response bodies fail validation.
- Other providers retain their current response-validation behavior.
- Credentials are never included in returned errors.

## Testing

Add focused Go tests that prove:

- a text PushPlus payload sends `template: "txt"`;
- a Markdown PushPlus payload sends `template: "markdown"`;
- a PushPlus response with code 200 succeeds;
- an HTTP 2xx response with a non-200 PushPlus code fails with useful context;
- malformed PushPlus JSON fails instead of producing a false success.

Run the focused message-push tests, the complete Go test suite, and the existing
frontend tests/build in proportion to the affected backend contract. After all
local verification passes, send exactly one real PushPlus test message using the
ephemeral token in process memory only and report the provider response without
printing the token.

## Out Of Scope

- Persisting the supplied token.
- Adding PushPlus topics, alternate delivery channels, callbacks, or status
  polling.
- Replacing the existing HTTP integration with a PushPlus SDK.
- Changing the single-channel selection model or any frontend layout.
