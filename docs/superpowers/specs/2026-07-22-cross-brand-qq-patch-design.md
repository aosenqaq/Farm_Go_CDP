# Cross-Brand QQ Patch Isolation Design

Date: 2026-07-22

## Goal

Switching between the normal Farm_Go application and a branded agent such as HQ Farm must leave exactly one active QQ miniapp debug patch. A later installation must replace all prior managed patches, so the QQ WebSocket host version always matches the application that installed the current patch.

## Problem

The normal patcher recognizes only the `FARM_GO_QQ_DEBUG` marker. The HQ build rewrites that marker in its disposable source workspace to `HQ_FARM_QQ_DEBUG`. When a normal Farm_Go installation follows an HQ installation, it replaces the Farm_Go block but leaves the later HQ block in `game.js`. JavaScript executes the remaining HQ block last, sending `hq-farm-host-1`; Farm_Go correctly rejects that WebSocket client because it expects `farm-go-host-1`.

## Design

### Managed Patch Contract

A managed QQ debug patch is any complete block with marker lines in this form:

```text
// >>> <BRAND>_QQ_DEBUG START >>>
...
// <<< <BRAND>_QQ_DEBUG END <<<
```

`<BRAND>` is an uppercase alphanumeric-and-underscore token. This includes the existing `FARM_GO` and transformed `HQ_FARM` markers. Legacy `QQ_FARM_AUTOMATION` blocks remain removable as before. Application JavaScript outside managed or legacy blocks must remain byte-for-byte unchanged apart from the existing newline normalization around a replacement.

### Installation Behavior

`qqpatch.Install` will inspect all managed blocks in a target `game.js`.

- It returns `already_latest` only when exactly one managed block exists and it contains the requested script hash.
- Otherwise it removes every managed and legacy patch block, appends one newly built current-brand block, preserves the existing backup behavior, and reports `RestartRequired: true`.
- Replacing a foreign-brand block is reported as a replacement, not as a no-op.

This applies symmetrically after source branding replacement: the HQ binary will remove a previous Farm_Go block, and the normal binary will remove a previous HQ block.

### Agent Build Guard

The HQ release builder already creates a disposable rewritten source tree and runs Go regressions there. It will also run the focused cross-brand patch test before the full suite. A failure stops the build before any pre-VMP executable is written. The release script continues to avoid normal-workspace and QQ-cache writes; cache cleanup remains a runtime responsibility of the executable patcher.

## Tests

Backend tests in `internal/runtime/qqpatch/patcher_test.go` will cover:

- normal installation over a `game.js` containing both Farm_Go and HQ managed blocks, leaving only one normal block;
- a foreign managed block forcing replacement even when a matching normal block is also present;
- a single matching block remaining idempotent;
- legacy block removal continuing to work.

The agent build script test will assert the focused cross-brand test is a required regression step before the copied workspace runs the full Go suite. The agent build itself verifies the rewritten source by running that test in the disposable workspace.

## Acceptance

- A normal Farm_Go installation removes `HQ_FARM_QQ_DEBUG` from a previously agent-patched `game.js`.
- A branded agent installation removes `FARM_GO_QQ_DEBUG` after source branding substitution.
- The resulting file contains exactly one managed block and exposes the installing binary's host version.
- `go test ./internal/runtime/qqpatch`, the HQ build script test, and the full existing verification suites pass.
- No release build or test step reads or prints KAuth secrets.
