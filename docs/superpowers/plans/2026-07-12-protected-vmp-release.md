# Protected VMP Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a repeatable protected Windows release workflow that obfuscates only generated frontend assets, hands the executable to VMProtect GUI, and verifies the returned release artifact.

**Architecture:** A Node utility owns the narrow frontend asset allow-list and produces deterministic hashes. PowerShell scripts wrap the existing Wails build into a timestamped pre-VMP handoff directory, then validate and finalize the GUI-produced executable. The workflow preserves every runtime resource under `resources/` without JavaScript obfuscation.

**Tech Stack:** PowerShell 7, Node.js CommonJS, `javascript-obfuscator`, npm, Wails/Go, VMProtect Professional GUI.

---

### Task 1: Implement the frontend asset obfuscator

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/package-lock.json`
- Create: `scripts/obfuscate-frontend.cjs`
- Create: `scripts/test-obfuscate-frontend.cjs`

- [ ] **Step 1: Add a failing Node test for the allow-list and manifest contract**

Create `scripts/test-obfuscate-frontend.cjs` using `node:assert/strict` and a
temporary `dist` fixture. Require `./obfuscate-frontend.cjs` and assert that
`collectJavaScriptAssets()` returns only `assets/app.js`, rejects a resolved
path outside `dist/assets`, removes `assets/app.js.map`, and returns a SHA-256
entry after `obfuscateFrontendAssets()` runs.

```js
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { collectJavaScriptAssets, obfuscateFrontendAssets } = require('./obfuscate-frontend.cjs');

const root = fs.mkdtempSync(path.join(os.tmpdir(), 'farm-go-obfuscator-'));
const dist = path.join(root, 'dist');
fs.mkdirSync(path.join(dist, 'assets'), { recursive: true });
fs.writeFileSync(path.join(dist, 'assets', 'app.js'), 'const secret = "value"; console.log(secret);');
fs.writeFileSync(path.join(dist, 'assets', 'app.js.map'), '{}');
fs.writeFileSync(path.join(dist, 'outside.js'), 'console.log("outside");');

try {
  assert.deepEqual(collectJavaScriptAssets(dist), [path.join(dist, 'assets', 'app.js')]);
  const manifest = obfuscateFrontendAssets(dist);
  assert.equal(manifest.files.length, 1);
  assert.equal(manifest.files[0].path, 'assets/app.js');
  assert.match(manifest.files[0].sha256, /^[a-f0-9]{64}$/);
  assert.equal(fs.existsSync(path.join(dist, 'assets', 'app.js.map')), false);
  assert.throws(() => collectJavaScriptAssets(path.join(root, 'missing')), /assets directory/i);
} finally {
  fs.rmSync(root, { recursive: true, force: true });
}
```

- [ ] **Step 2: Run the test and verify it fails because the module is absent**

Run: `node scripts/test-obfuscate-frontend.cjs`

Expected: `MODULE_NOT_FOUND` for `obfuscate-frontend.cjs`.

- [ ] **Step 3: Install the production-build dependency and add protected build commands**

Run from `frontend/`:

```powershell
npm install --save-dev javascript-obfuscator
```

Update `frontend/package.json` scripts to keep the existing normal build and
add the protected variant:

```json
"build": "tsc && vite build",
"build:protected": "tsc && vite build && node ../scripts/obfuscate-frontend.cjs"
```

- [ ] **Step 4: Implement the narrow obfuscator module**

Create `scripts/obfuscate-frontend.cjs` with exported functions
`collectJavaScriptAssets`, `sha256File`, and `obfuscateFrontendAssets`. Resolve
the asset root from `frontend/dist/assets`, recurse only below it, sort paths,
and transform only `.js` files. Use these initial options:

```js
const options = {
  compact: true,
  controlFlowFlattening: false,
  deadCodeInjection: false,
  identifierNamesGenerator: 'hexadecimal',
  renameGlobals: false,
  selfDefending: false,
  sourceMap: false,
  stringArray: true,
  stringArrayEncoding: ['base64'],
  stringArrayThreshold: 0.75,
  transformObjectKeys: false,
  unicodeEscapeSequence: false,
  reservedNames: ['^App$', '^Events(On|Off|Once|Emit)$'],
  reservedStrings: ['wailsjs', 'runtime', 'license:status', 'license:revoked']
};
```

Delete every `.map` file beneath `frontend/dist` before returning the relative
path, SHA-256, and byte length for each transformed asset. Throw when the
assets directory is absent or a candidate resolves outside it. Run the module
as a CLI when `require.main === module` and print JSON only after success.

- [ ] **Step 5: Run the focused test and protected frontend build**

Run:

```powershell
node scripts/test-obfuscate-frontend.cjs
Set-Location frontend
npm run build:protected
Get-ChildItem -Recurse -File dist -Filter *.map
```

Expected: the Node test exits `0`, the build exits `0`, and the final command
prints no source maps.

- [ ] **Step 6: Commit the frontend obfuscator**

```powershell
git add frontend/package.json frontend/package-lock.json scripts/obfuscate-frontend.cjs scripts/test-obfuscate-frontend.cjs
git commit -m "build: obfuscate protected frontend assets"
```

### Task 2: Add protected Wails build selection

**Files:**
- Modify: `scripts/build.ps1`
- Modify: `scripts/build-output-redaction.test.ps1`

- [ ] **Step 1: Add a failing PowerShell assertion for protected frontend selection**

Extend the fake `wails.cmd` in `scripts/build-output-redaction.test.ps1` to copy
the received command line to `$env:FAKE_WAILS_ARGS_FILE`. Invoke the build once
with `-ProtectedFrontend` and assert that the captured arguments contain
`frontend:build` configuration whose command is `npm run build:protected`.

```powershell
$protectedBuild = Invoke-FakeBuild 0 -ProtectedFrontend
if ($protectedBuild.ExitCode -ne 0) { throw "Expected protected build to succeed." }
if (-not $protectedBuild.WailsConfigText.Contains('npm run build:protected')) {
  throw "Protected build did not select the protected frontend command."
}
```

- [ ] **Step 2: Run the test and verify the new assertion fails**

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build-output-redaction.test.ps1`

Expected: failure stating that the protected frontend command was not selected.

- [ ] **Step 3: Add the build switch without changing the default build path**

Add a `[switch]$ProtectedFrontend` parameter to `scripts/build.ps1`. In the
existing temporary `wails.json` update block, set the JSON property only when
the switch is set:

```powershell
if ($ProtectedFrontend) {
  $wailsConfig.'frontend:build' = 'npm run build:protected'
}
```

Keep byte-for-byte restoration of `wails.json` in the existing `finally` block.
Do not expose any KAuth value in output.

- [ ] **Step 4: Re-run the build-script test**

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build-output-redaction.test.ps1`

Expected: exit `0`, normal builds retain their existing command, protected
builds select `npm run build:protected`, and the fake secret never appears.

- [ ] **Step 5: Commit protected Wails build selection**

```powershell
git add scripts/build.ps1 scripts/build-output-redaction.test.ps1
git commit -m "build: add protected frontend build mode"
```

### Task 3: Create the pre-VMP handoff builder

**Files:**
- Create: `scripts/build-protected-release.ps1`
- Create: `scripts/test-protected-release.ps1`
- Modify: `.gitignore`
- Modify: `README.md`

- [ ] **Step 1: Write failing tests for release layout and the GUI checklist**

Create `scripts/test-protected-release.ps1` with a temporary fixture containing
a fake `build.ps1` that writes a valid `build/bin/Farm_Go.exe` fixture. Invoke
the new script with a fixed `-ReleaseId test-release` and assert all of these
paths exist:

```powershell
$releaseRoot = Join-Path $fixtureRoot 'release\test-release'
@(
  'pre-vmp\Farm_Go.exe',
  'PRE-VMP-MANIFEST.json',
  'VMP-GUI-CHECKLIST.md',
  'vmp-output\.gitkeep'
) | ForEach-Object {
  if (-not (Test-Path -LiteralPath (Join-Path $releaseRoot $_))) {
    throw "Missing release artifact: $_"
  }
}
```

Also assert `PRE-VMP-MANIFEST.json` contains the SHA-256 of the copied fixture
and the checklist names `internal/license/*` before `internal/runtime/cdp/*`.

- [ ] **Step 2: Run the test and verify it fails because the builder is absent**

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-protected-release.ps1`

Expected: failure because `build-protected-release.ps1` does not exist.

- [ ] **Step 3: Implement the pre-VMP release builder**

Create `scripts/build-protected-release.ps1` with parameters:

```powershell
param(
  [Parameter(Mandatory = $true)][string]$Version,
  [string]$ReleaseId = "",
  [string]$ReleaseRoot = ""
)
```

Validate the five existing KAuth build variables. Default `ReleaseId` to
`farm-go-v<Version>-<yyyyMMdd-HHmmss>` and `ReleaseRoot` to `<repo>/release`.
Refuse an existing target directory. Invoke:

```powershell
& (Join-Path $PSScriptRoot 'build.ps1') -ProtectedFrontend
```

Copy the generated executable to `pre-vmp/Farm_Go.exe`, hash it using
`Get-FileHash -Algorithm SHA256`, collect protected frontend asset hashes from
`frontend/dist/assets`, and write the JSON manifest with `ConvertTo-Json -Depth
8`. Generate a Markdown GUI checklist with the agreed protection priorities and
the conservative VMP settings from the design. Create an empty
`vmp-output/.gitkeep` file.

- [ ] **Step 4: Add ignored release outputs and operator documentation**

Append `release/` to `.gitignore`. Extend `README.md` with exact commands for
creating a pre-VMP release and the instruction to write the GUI output to the
generated `vmp-output/Farm_Go.exe` path. State that no `resources/**/*.js` file
is transformed by this workflow.

- [ ] **Step 5: Run the pre-VMP builder test**

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-protected-release.ps1`

Expected: exit `0`; the fixture contains the four required artifacts and a
manifest hash matching its pre-VMP executable.

- [ ] **Step 6: Commit the pre-VMP handoff**

```powershell
git add scripts/build-protected-release.ps1 scripts/test-protected-release.ps1 .gitignore README.md
git commit -m "build: add VMProtect GUI release handoff"
```

### Task 4: Create final VMP-release verification

**Files:**
- Create: `scripts/finalize-vmp-release.ps1`
- Create: `scripts/test-finalize-vmp-release.ps1`
- Modify: `README.md`

- [ ] **Step 1: Write failing finalization tests**

Create a fixture release directory with a pre-VMP manifest and a minimal valid
PE header (`MZ`). Test these cases against the new finalizer:

```powershell
Assert-Fails { Invoke-Finalizer $releaseRoot } 'missing GUI output'
Copy-Item $preVmpExe (Join-Path $releaseRoot 'vmp-output\Farm_Go.exe')
Assert-Fails { Invoke-Finalizer $releaseRoot } 'unchanged hash'
Set-Content -LiteralPath (Join-Path $releaseRoot 'vmp-output\Farm_Go.exe') -Value 'not-a-pe'
Assert-Fails { Invoke-Finalizer $releaseRoot } 'PE header'
```

Then write different `MZ`-prefixed bytes and assert that finalization creates
`Farm_Go.exe`, `RELEASE-MANIFEST.json`, and `SHA256SUMS.txt`.

- [ ] **Step 2: Run the test and verify it fails because the finalizer is absent**

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-finalize-vmp-release.ps1`

Expected: failure because `finalize-vmp-release.ps1` does not exist.

- [ ] **Step 3: Implement the finalizer**

Create `scripts/finalize-vmp-release.ps1` with one mandatory parameter:

```powershell
param([Parameter(Mandatory = $true)][string]$ReleaseDirectory)
```

Resolve the directory, require `PRE-VMP-MANIFEST.json`, require exactly
`vmp-output/Farm_Go.exe`, verify its first two bytes are `0x4D, 0x5A`, and
compare its SHA-256 against the `preVmp.sha256` manifest value. On success, copy
it to `Farm_Go.exe`, create a release manifest recording the pre-VMP and final
hashes, and write `SHA256SUMS.txt` as `<hash>  Farm_Go.exe`. Do not delete the
pre-VMP executable or GUI output. Write a final message requiring the operator
to run the protected executable and complete an authorization smoke test.

- [ ] **Step 4: Run the finalizer test**

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-finalize-vmp-release.ps1`

Expected: all three rejection cases fail for their stated reason and the valid
case exits `0` with the three final artifacts.

- [ ] **Step 5: Document finalization and commit**

Add the finalization command to `README.md`:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\finalize-vmp-release.ps1 -ReleaseDirectory .\release\<release-id>
```

Commit:

```powershell
git add scripts/finalize-vmp-release.ps1 scripts/test-finalize-vmp-release.ps1 README.md
git commit -m "build: verify finalized VMProtect releases"
```

### Task 5: Verify the complete release workflow

**Files:**
- Verify: `scripts/obfuscate-frontend.cjs`
- Verify: `scripts/build.ps1`
- Verify: `scripts/build-protected-release.ps1`
- Verify: `scripts/finalize-vmp-release.ps1`

- [ ] **Step 1: Run all focused release tests**

```powershell
node scripts/test-obfuscate-frontend.cjs
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build-output-redaction.test.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-protected-release.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-finalize-vmp-release.ps1
```

Expected: every command exits `0`.

- [ ] **Step 2: Run existing application regression suites**

```powershell
go test ./...
Set-Location frontend
npm test
npm run build:protected
```

Expected: Go tests, frontend tests, and protected frontend build all exit `0`.

- [ ] **Step 3: Run a real pre-VMP release with release environment present**

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-protected-release.ps1 -Version $env:FARM_GO_VERSION_NAME
```

Expected: a new timestamped directory below `release/` containing the pre-VMP
executable, manifest, GUI checklist, and `vmp-output` directory.

- [ ] **Step 4: Complete the VMP GUI handoff and finalization**

Open the generated `pre-vmp/Farm_Go.exe` in VMProtect, apply the generated
checklist, and save exactly `vmp-output/Farm_Go.exe`. Then run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\finalize-vmp-release.ps1 -ReleaseDirectory .\release\<release-id>
```

Expected: the final executable hash differs from pre-VMP, the manifest and
checksum are generated, and a manual launch plus KAuth authorization check pass.
