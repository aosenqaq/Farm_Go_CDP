# QQ Runtime Reconnect Dialog Design

## Goal

Prevent a stale in-memory QQ patch from repeatedly reopening the startup
injection dialog while the QQ WebSocket reconnects. Preserve a clear, single
instruction to restart the miniapp after Farm_Go replaces a patch on disk.

## Scope

The change is limited to the desktop `AuthorizedApp` automatic QQ patch check
and its test coverage. QQ host-version validation, patch contents, and the
startup dialog's existing manual retry and restart actions remain unchanged.

## Design

The automatic check distinguishes a patch-generation change from a WebSocket
connection change:

- A ready QQ runtime is checked once per current disk patch generation.
- The generation is the expected Farm_Go patch hash, rather than the adapter's
  per-WebSocket `instanceId`.
- When a running script hash differs from that generation, Farm_Go keeps the
  existing restart instruction visible once. Further reconnects with the same
  hash pair neither rewrite the disk patch nor clear a user-dismissed dialog.
- A manual reinjection, or a newly written patch with a different script hash,
  starts a new generation and may show the instruction again.

This preserves automatic discovery when the patch changes while avoiding a
reconnect loop caused by an old in-memory HQ patch. The stale script still
requires the user to close and reopen the QQ farm miniapp; it cannot be
replaced in process by changing `game.js` on disk.

## Validation

Frontend unit tests will first prove that reconnecting with a different QQ WS
instance ID does not schedule another automatic check for the same patch
generation, while a changed patch generation does. Existing startup-injection
tests continue to verify the one-time restart instruction shown for a hash
mismatch.
