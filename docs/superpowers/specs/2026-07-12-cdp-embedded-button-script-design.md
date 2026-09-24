# CDP Embedded Button Script Design

**Date:** 2026-07-12

## Problem

The WeChat CDP and YYB CDP links share `wmpf.CDPLink`. Although
`resources/wmpf/button.js` is already compiled into the executable with
`go:embed`, the link still derives `resources/wmpf/button.js` from
`FridaRoot` and reads it from the process working directory.

The protected single-file release contains no adjacent `resources` tree. Its
CDP bridge can therefore detect the miniapp, but script injection fails before
`gameCtl` is created. Both channels remain connected and handshaking without
becoming ready. A development build can appear healthy when launched with the
repository root as its working directory, which hides the packaging defect.

## Design

Add an in-memory button script field to `wmpf.CDPLinkConfig`. Script loading
uses the embedded in-memory value when it is non-empty. The existing explicit
or `FridaRoot`-derived disk path remains a fallback only when no in-memory
script was supplied, preserving focused tests and development compatibility.

Pass the existing `runtimeButtonScript` value to both WeChat and YYB link
configurations in `NewApp`, and again when `rebuildCDPLinks` replaces those
links after runtime settings change. No script is extracted to disk and the
single-file release layout remains unchanged.

## Error Handling

An empty in-memory value keeps the current disk behavior and its read errors.
When an in-memory value is present, an absent disk resource is irrelevant.
Injection, hashing, readiness probing, and `gameCtl_not_ready` behavior remain
unchanged after the source has been selected.

## Testing

Add table-driven `wmpf` regression coverage for both WeChat and YYB profiles.
Each case supplies an in-memory script together with a deliberately missing
disk path, then verifies that readiness injection uses the in-memory source
and succeeds. Retain coverage for the disk fallback so existing callers do not
silently lose compatibility.

Run the focused WMPF tests, the main package tests that compile application
wiring, and the complete Go test suite. A release smoke test should launch the
new protected executable without an adjacent `resources` directory and verify
that both CDP channels can progress from miniapp connected to ready.

## Out Of Scope

- Changing Frida hook resource loading.
- Extracting `button.js` beside the executable or into user data.
- Changing CDP context selection, retry timing, or frontend status wording.
- Changing VMProtect settings.
