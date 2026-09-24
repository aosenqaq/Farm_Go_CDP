# Protected VMP Release Design

## Goal

Create a repeatable Windows release workflow for Farm_Go that obfuscates the
embedded frontend business bundle, produces a versioned pre-VMP executable,
hands that executable to VMProtect Professional GUI, and verifies the returned
protected executable before distribution.

## Scope and Constraints

- Keep the existing KAuth client-side configuration model. This workflow does
  not move `KAUTH_PROGRAM_SECRET` to a service.
- Do not encrypt image resources that the application extracts to the local
  cache at runtime.
- Do not obfuscate `resources/**/*.js`. In particular, `qq-host.js` and
  `button.js` must remain byte-for-byte source files before Go embeds them.
- VMProtect Professional 3.9.6 Build 2385 is GUI-operated in this environment.
  The workflow must not rely on a VMProtect command-line executable.
- Preserve the existing `scripts/build.ps1` build contract and its required
  KAuth environment variables.

## Protection Boundary

The final executable has two distinct protection layers.

1. The frontend layer applies JavaScript obfuscation only to generated files
   below `frontend/dist/assets`. This protects React business code while keeping
   static runtime injection files untouched. It must preserve Wails bridge names
   and may not emit source maps.
2. The native layer passes the resulting Wails executable through VMProtect.
   VMProtect protects the complete executable shell and the selected GUI
   functions. The GUI checklist prioritizes authorization and business logic;
   it does not claim that client-side secrets become unrecoverable.

The release checklist identifies the following code as VMP protection targets:

- Highest priority: `internal/license/*`, `authorization_gate.go`.
- High priority: `internal/runtime/cdp/*`, `internal/runtime/wmpf/*`, and
  `internal/runtime/qqpatch/*`.
- Medium priority: `internal/farm/automation/*`, `internal/farm/social/*`, and
  `internal/farm/stealrules/*`.

These source paths are guidance for function selection in the VMP GUI. Go
symbol visibility varies after `-trimpath`, so the initial profile must at
minimum protect the entry point and use the global protection options in the
generated GUI checklist. VMP SDK markers for exact Go functions are explicitly
outside this release workflow.

## Release Data Flow

```text
required build environment
  -> protected build script
  -> frontend protected build
  -> Wails EXE in release/<id>/pre-vmp/Farm_Go.exe
  -> pre-VMP manifest + GUI checklist
  -> manual VMProtect GUI output in release/<id>/vmp-output/Farm_Go.exe
  -> finalization script
  -> release/<id>/Farm_Go.exe + release manifest + SHA256SUMS.txt
```

The pre-VMP executable is never replaced in place. The GUI writes a separate
output file under the release directory. This keeps the original available for
comparison and makes accidental distribution of the unprotected file detectable.

## Components

### Frontend Obfuscator

`scripts/obfuscate-frontend.cjs` will expose small testable functions that:

- discover only `frontend/dist/assets/**/*.js` files;
- reject paths outside the asset directory;
- invoke `javascript-obfuscator` with browser-safe, moderate settings;
- retain Wails runtime identifiers through reserved names and strings;
- remove `.map` files from the protected frontend output; and
- return a manifest-ready list of transformed files and SHA-256 hashes.

The frontend package adds a protected build command. Normal development builds
remain unchanged. `scripts/build.ps1` selects the protected command only when
called by the protected release script.

### Pre-VMP Release Script

`scripts/build-protected-release.ps1` will:

- require the existing KAuth build environment variables;
- require a release version argument and derive a timestamped release ID;
- invoke the existing build script in protected frontend mode;
- copy `build/bin/Farm_Go.exe` into `release/<id>/pre-vmp/Farm_Go.exe`;
- write `PRE-VMP-MANIFEST.json` with version, file size, SHA-256 and protected
  frontend asset hashes;
- write `VMP-GUI-CHECKLIST.md` containing the approved VMP settings and source
  priority list; and
- create the empty `vmp-output` handoff directory.

### VMP GUI Handoff

The operator opens the pre-VMP executable in VMProtect and writes exactly one
protected output file to `vmp-output/Farm_Go.exe`. The checklist sets a
conservative initial profile: protect the entry point, enable import/resource/
memory protection, compressed output and debug-info removal, use user-mode
debugger detection, and do not enable kernel-mode detection or serial-number
locking. The operator performs a manual launch smoke test after VMP.

### Finalization Script

`scripts/finalize-vmp-release.ps1` will:

- require the pre-VMP manifest and exactly one expected GUI output file;
- validate the output has a PE `MZ` header;
- reject an output whose SHA-256 equals the pre-VMP executable;
- copy the verified result to `release/<id>/Farm_Go.exe` without overwriting the
  pre-VMP input;
- generate `RELEASE-MANIFEST.json` and `SHA256SUMS.txt`; and
- report that Authenticode signing, when configured, must be applied after this
  step and followed by a fresh finalization/manifest generation.

## Failure Handling

- Missing KAuth environment variables: fail before the frontend or Wails build.
- Missing obfuscator dependency, source maps in protected output, or an asset
  outside the allow-list: fail the pre-VMP build.
- Missing GUI output, multiple executable outputs, non-PE output, or unchanged
  output hash: fail finalization.
- A failed Wails build retains its original `wails.json` configuration because
  the existing build script restores it in `finally`.

## Verification

Automated tests will cover the frontend allow-list, source-map removal, release
layout creation, manifest hashing, and finalization rejection conditions. The
release workflow will additionally run the existing Go and frontend test suites,
then require a manual launch and KAuth authorization check against the final
VMP-protected executable.
