# HQ Farm Agent Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce an isolated HQ Farm pre-VMProtect EXE and GUI handoff directory from the HQ agent configuration without changing Farm Go source artifacts.

**Architecture:** A PowerShell agent script owns config validation, branding, disposable-copy construction, Wails build invocation, and output verification. A small PowerShell test drives the script against fixtures before a real build. The user runs VMProtect GUI against the emitted pre-VMP EXE; a separate agent finalizer verifies the returned executable and refreshes delivery metadata.

**Tech Stack:** PowerShell 7, .NET System.Drawing, Wails, Go, Vite/Node.js, VMProtect GUI.

---

### Task 1: Define HQ Farm metadata and icon generation

**Files:**
- Create: `代理版打包配置/HQ Farm/agent.release.json`
- Create: `scripts/build-hq-farm-icon.ps1`
- Create: `scripts/build-hq-farm-icon.test.ps1`
- Create: `代理版打包配置/HQ Farm/icons/appicon.png`
- Create: `代理版打包配置/HQ Farm/icons/icon.ico`

- [ ] **Step 1: Write the failing icon-contract test**

Create `scripts/build-hq-farm-icon.test.ps1`. Invoke the missing icon script against a temporary 3840x2160 PNG fixture and assert that `appicon.png` is exactly 1024x1024 and that `icon.ico` starts with the ICO header `00 00 01 00`.

```powershell
& $iconScript -SourceLogo $source -OutputDirectory $output
if ($LASTEXITCODE -ne 0) { throw 'Icon script failed.' }
$png = [System.Drawing.Image]::FromFile((Join-Path $output 'appicon.png'))
if ($png.Width -ne 1024 -or $png.Height -ne 1024) { throw 'Expected a square PNG.' }
$ico = [System.IO.File]::ReadAllBytes((Join-Path $output 'icon.ico'))
if ([BitConverter]::ToString($ico[0..3]) -ne '00-00-01-00') { throw 'Expected ICO header.' }
```

- [ ] **Step 2: Run the focused test and verify it fails**

Run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-hq-farm-icon.test.ps1
```

Expected: failure because `build-hq-farm-icon.ps1` is absent.

- [ ] **Step 3: Add declarative non-secret agent metadata**

Create `代理版打包配置/HQ Farm/agent.release.json`:

```json
{
  "id": "hq-farm",
  "productName": "HQ Farm",
  "exeName": "HQ Farm.exe",
  "releasePrefix": "hq-farm",
  "sourceLogo": "HQ Farm_logo.png",
  "kauthEnv": "Kauth验证配置.txt"
}
```

- [ ] **Step 4: Implement the deterministic icon converter**

Create `scripts/build-hq-farm-icon.ps1` with mandatory `SourceLogo` and `OutputDirectory` parameters. Use a 1024px `Bitmap`, fill black, use `Graphics.DrawImage` with a 10% inset and preserved source aspect ratio, write `appicon.png`, and write an ICO containing PNG frames at 16, 24, 32, 48, 64, 128, and 256 pixels. Reject a missing/non-decodable source image and validate both written outputs before returning.

- [ ] **Step 5: Verify icon assets and commit**

Run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-hq-farm-icon.test.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-hq-farm-icon.ps1 -SourceLogo '代理版打包配置\HQ Farm\HQ Farm_logo.png' -OutputDirectory '代理版打包配置\HQ Farm\icons'
```

Expected: test exits `0`; `icons/appicon.png` is square; `icons/icon.ico` is a multi-frame ICO. Commit the metadata, converter, test, and generated icon files.

### Task 2: Test the isolated agent-release contract

**Files:**
- Create: `scripts/build-hq-farm-agent-release.test.ps1`
- Create: `scripts/build-hq-farm-agent-release.ps1`

- [ ] **Step 1: Write the failing agent-release test**

Create a fixture repository in `%TEMP%` containing a minimal `wails.json`, `go.mod`, `main.go`, `app.go`, `frontend`, `build`, and a fake `scripts/build.ps1` that writes `build/bin/HQ Farm.exe`. Copy the HQ-style agent fixture into `agent/`. Invoke the missing agent script with `-ProjectRoot`, `-AgentDirectory`, `-ReleaseRoot`, `-SkipTests`, and `-SkipBuild` disabled.

Assert that it creates exactly one `hq-farm-v103-*` directory containing:

```powershell
@(
  'pre-vmp\HQ Farm.exe',
  'PRE-VMP-MANIFEST.json',
  'SHA256SUMS.txt',
  'VMP-GUI-CHECKLIST.md',
  'vmp-output\.gitkeep'
) | ForEach-Object {
  if (-not (Test-Path (Join-Path $releaseDirectory $_))) { throw "Missing $_" }
}
```

Read the copied temporary-source substitutions through the script's `-KeepWorkspace` option and assert all four `Farm Go` variants are absent while `HQ Farm` is present. Assert the release manifest names `HQ Farm.exe` and does not contain the fixture KAuth secret.

- [ ] **Step 2: Run the test and verify it fails**

Run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-hq-farm-agent-release.test.ps1
```

Expected: failure because `build-hq-farm-agent-release.ps1` is absent.

- [ ] **Step 3: Implement configuration parsing and safety guards**

Create `scripts/build-hq-farm-agent-release.ps1` parameters:

```powershell
param(
  [string]$ProjectRoot = (Split-Path -Parent $PSScriptRoot),
  [string]$AgentDirectory = (Join-Path (Split-Path -Parent $PSScriptRoot) '代理版打包配置\HQ Farm'),
  [string]$ReleaseRoot = '',
  [switch]$KeepWorkspace,
  [switch]$SkipTests
)
```

Read `agent.release.json`; require `id`, `productName`, `exeName`, `releasePrefix`, `sourceLogo`, and `kauthEnv`. Parse only assignment lines from the KAuth file, strip an optional `$env:` prefix from keys, and require the five existing KAuth variables without printing any values. Refuse an agent directory outside the project root, a release root outside the agent directory, or a pre-existing timestamped release directory.

- [ ] **Step 4: Implement disposable-copy branding**

Copy the repository into a `%TEMP%\hq-farm-agent-*` directory while excluding `.git`, `.worktrees`, `node_modules`, `frontend/node_modules`, `release`, `build/bin`, `frontend/dist`, existing `代理版打包配置/**/dist`, and `graphify-out`. Copy the generated HQ icons over `build/appicon.png` and `build/windows/icon.ico`.

Replace `Farm Go`, `Farm_Go`, `farm-go`, and `farm_go` in text source, configuration, scripts, manifests, resource names, tests, and frontend files with `HQ Farm`, `HQ_Farm`, `hq-farm`, and `hq_farm` respectively. Rename paths that contain those same identifiers, including the Go module directory-independent source names and release script output names. Do not transform binary files. Re-parse the copied `go.mod` and ensure Go linker package paths match its new module value.

- [ ] **Step 5: Add release layout, no-leak logging, and cleanup**

Dot-source the copied KAuth environment file inside a nested PowerShell process, then run the copied build script with `-ProtectedFrontend`. Filter build output so neither `ldflags` nor `KAUTH_PROGRAM_SECRET` appears. Copy `build/bin/HQ Farm.exe` to `pre-vmp/HQ Farm.exe`; calculate SHA-256; generate a manifest with agent id, version, file paths, byte lengths, hashes, and public frontend asset hashes only. Write `SHA256SUMS.txt`, `vmp-output/.gitkeep`, and a VMP checklist naming `HQ Farm.exe`.

Use a `try/finally` block to delete the temporary copy unless `-KeepWorkspace` is supplied. Never remove files outside the temporary workspace or selected agent `dist` directory.

- [ ] **Step 6: Run the focused agent test and commit**

Run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-hq-farm-agent-release.test.ps1
```

Expected: exit `0`; fixture release layout, branding substitutions, no-secret manifest, and cleanup checks pass. Commit the agent builder and its test.

### Task 3: Add final VMP handoff verification

**Files:**
- Create: `scripts/finalize-hq-farm-agent-release.ps1`
- Create: `scripts/finalize-hq-farm-agent-release.test.ps1`

- [ ] **Step 1: Write the failing finalizer test**

Create a minimal HQ release fixture with `PRE-VMP-MANIFEST.json` and a pre-VMP `HQ Farm.exe`. Verify the missing finalizer rejects no GUI output, rejects output with the pre-VMP hash, rejects non-PE output, and accepts different `MZ`-prefixed GUI output by generating root `HQ Farm.exe`, `RELEASE-MANIFEST.json`, and `SHA256SUMS.txt`.

- [ ] **Step 2: Run the test and verify it fails**

Run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\finalize-hq-farm-agent-release.test.ps1
```

Expected: failure because `finalize-hq-farm-agent-release.ps1` is absent.

- [ ] **Step 3: Implement the HQ finalizer**

Create a finalizer with a mandatory `ReleaseDirectory`. Require `pre-vmp/HQ Farm.exe`, `vmp-output/HQ Farm.exe`, and `PRE-VMP-MANIFEST.json`; ensure the GUI output is a distinct PE file; copy it to root `HQ Farm.exe`; write final release metadata and its checksum. Scan the final EXE, both manifests, checksum, and VMP checklist for `Farm Go`, `Farm_Go`, `farm-go`, and `farm_go`; fail before any delivery claim if any match remains.

- [ ] **Step 4: Run focused finalizer test and commit**

Run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\finalize-hq-farm-agent-release.test.ps1
```

Expected: exit `0`; every failure case is rejected and the valid fixture produces only HQ-named delivery metadata. Commit both files.

### Task 4: Build and verify the actual HQ Farm pre-VMP artifact

**Files:**
- Verify: `代理版打包配置/HQ Farm/agent.release.json`
- Verify: `代理版打包配置/HQ Farm/icons/appicon.png`
- Verify: `代理版打包配置/HQ Farm/icons/icon.ico`
- Verify: `代理版打包配置/HQ Farm/dist/hq-farm-v<version>-<timestamp>/`

- [ ] **Step 1: Run all focused agent-release tests**

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-hq-farm-icon.test.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-hq-farm-agent-release.test.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\finalize-hq-farm-agent-release.test.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-output-redaction.test.ps1
```

Expected: every command exits `0`.

- [ ] **Step 2: Run application regression checks**

```powershell
go test ./...
Set-Location frontend
npm test
```

Expected: Go and frontend suites exit `0` before the real release build.

- [ ] **Step 3: Generate the actual HQ Farm pre-VMP EXE**

Run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-hq-farm-agent-release.ps1
```

Expected: exactly one new agent-only timestamped directory under `代理版打包配置/HQ Farm/dist/`, with `pre-vmp/HQ Farm.exe`, matching hashes, HQ square icon assets, no KAuth values in metadata, and zero matches for every forbidden Farm Go variant.

- [ ] **Step 4: Hand off VMProtect GUI input**

Give the user only `pre-vmp/HQ Farm.exe` and `VMP-GUI-CHECKLIST.md`. The user writes VMProtect's result to `vmp-output/HQ Farm.exe`, then finalizes with:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\finalize-hq-farm-agent-release.ps1 -ReleaseDirectory '<agent-dist-directory>'
```

Expected: a changed, zero-residue, final root `HQ Farm.exe` with an updated manifest and SHA-256 checksum.
