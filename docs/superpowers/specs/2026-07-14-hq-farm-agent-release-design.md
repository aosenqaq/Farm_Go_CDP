# HQ Farm Agent Release Design

## Goal

Create a repeatable HQ Farm Windows release preparation flow. It must build a branded, VMP-preparation EXE from the HQ Farm configuration without changing the normal Farm Go workspace, output directory, credentials, or artifacts.

## Scope

- Read the agent KAuth values only from `代理版打包配置/HQ Farm/Kauth验证配置.txt`.
- Convert the supplied wide HQ Farm logo into a square Windows icon and use it for the agent build.
- Build a protected-frontend, pre-VMProtect `HQ Farm.exe` below the agent directory.
- Create an explicit GUI handoff contract for VMProtect and the later finalization step.
- Verify the pre-VMP EXE and its delivery metadata contain no `Farm Go`, `Farm_Go`, `farm-go`, or `farm_go` branding.

The normal Farm Go build path and its files remain unmodified. VMProtect GUI operation is performed by the user and is outside this automation.

## Architecture

The agent configuration directory becomes the source of agent-only input and output:

```text
代理版打包配置/HQ Farm/
  HQ Farm_logo.png
  Kauth验证配置.txt
  agent.release.json
  icons/
    appicon.png
    icon.ico
  dist/
    hq-farm-v<version>-<timestamp>/
      pre-vmp/HQ Farm.exe
      PRE-VMP-MANIFEST.json
      SHA256SUMS.txt
      VMP-GUI-CHECKLIST.md
      vmp-output/
```

`agent.release.json` declares non-secret brand metadata: agent id `hq-farm`, display name `HQ Farm`, executable name `HQ Farm.exe`, release prefix `hq-farm`, and the relative paths to the logo and KAuth environment file.

The build script creates a disposable copy of the repository outside the source tree. In that copy it changes all runtime, package, resource, and release identifiers required for a zero-brand-residue pre-VMP EXE, installs the generated HQ icon, imports the agent's KAuth environment values, and runs the existing Wails protected build. It copies only final agent artifacts back to the agent `dist` directory and deletes the temporary workspace even on build failure.

## Branding Rules

The HQ build uses `HQ Farm` in the window title, application metadata, EXE filename, web title, visible React labels, tray text, notifications, diagnostics, cache backups, and per-user data directory.

The agent build also replaces Farm Go identifiers that are embedded in the unprotected binary: the Go module import prefix, build linker paths, resource manifest name, Frida helper environment names, temporary filenames, and release manifest paths. Test fixtures are updated in the temporary workspace as necessary; the normal source checkout is not changed.

## Icon Conversion

The supplied 16:9 source logo is not a Windows icon. The conversion script renders a square PNG with the complete HQ mark centered on its black background, then creates a multi-size ICO with 16, 24, 32, 48, 64, 128, and 256 pixel frames. It fails if either generated artifact is absent or cannot be decoded as an image.

## KAuth Handling

The release script loads exactly the five variables expected by the current build: `KAUTH_PROGRAM_ID`, `KAUTH_PROGRAM_SECRET`, `KAUTH_MERCHANT_PUBLIC_KEY`, `FARM_GO_VERSION_NO`, and `FARM_GO_VERSION_NAME`. It validates their presence, never prints values, and supplies them only to the disposable build process. Version variable names may remain input compatibility names, but no `FARM_GO` identifier may remain in the delivered EXE or metadata.

## VMProtect Handoff

The release output contains `pre-vmp/HQ Farm.exe` and a checklist naming that file as VMProtect input. The user saves the protected result to `vmp-output/HQ Farm.exe`; a dedicated finalizer then verifies it is a changed PE file, copies it to the release root, refreshes the manifest and SHA-256 file, and repeats the zero-brand-residue scan.

The release is not ready for external delivery until the finalizer has succeeded after the GUI step. For the requested first handoff, the pre-VMP EXE is the artifact given to the user for VMProtect.

## Verification

- Focused PowerShell tests cover KAuth-file parsing, no-secret log behavior, generated-agent directory shape, branding substitutions, release manifest filenames, and zero-residue scanning.
- The focused tests must fail before the release implementation is added and pass after it is implemented.
- The script runs the existing build-output redaction test and the Go test suite before the release build.
- A real HQ pre-VMP build verifies `HQ Farm.exe`, manifest hashes, icon assets, and absence of all four Farm Go string variants in the EXE and delivery metadata.
