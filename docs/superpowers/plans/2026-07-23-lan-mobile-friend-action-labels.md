# LAN Mobile Friend Action Labels Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a Farm Go executable whose embedded LAN WebUI displays readable `操作` and `配置` controls in each mobile friend row.

**Architecture:** The existing React source already renders the approved text labels and the remote-phone CSS gives them 55px visual controls inside 42px touch targets. The corrective action is a packaging refresh: regenerate `frontend/dist`, build the Wails executable that embeds those files through `main.go`, then replace the process serving the LAN page and reload its browser client.

**Tech Stack:** React 18, TypeScript, Vitest, Vite, Go 1.25, Wails v2, PowerShell.

---

## File Structure

- `frontend/src/views/SocialView.tsx`: Existing semantic `操作` / `配置` labels; verification target only.
- `frontend/src/views/SocialView.test.tsx`: Existing regression contract for the friend command triggers and remote phone layout; verification target only.
- `frontend/src/style.css`: Existing mobile dimensions and button visibility rules; verification target only.
- `frontend/dist/*`: Generated Vite assets embedded by the next Wails build.
- `build/bin/Farm_Go.exe`: Generated executable that the LAN service serves from its embedded asset filesystem.

### Task 1: Verify The Source-Level Regression Contract

**Files:**
- Verify: `frontend/src/views/SocialView.tsx`
- Verify: `frontend/src/views/SocialView.test.tsx`
- Verify: `frontend/src/style.css`

- [x] **Step 1: Confirm the text buttons exist before packaging**

Run from the repository root:

```powershell
rg -n -C 4 '操作|配置' frontend/src/views/SocialView.tsx
rg -n -C 3 'social-command-trigger|width: 114px|min-height: 42px' frontend/src/style.css
```

Expected: `SocialView.tsx` renders `操作` and `配置` in `.social-command-trigger` buttons; the remote-phone CSS retains a 114px action region and a 42px touch target.

- [x] **Step 2: Run the focused friend-row regression test**

Run from `frontend`:

```powershell
npx vitest run src/views/SocialView.test.tsx
```

Expected: PASS. The test confirms the two command triggers, their actions, and the mobile CSS contract. No new source test is required because the desired source behavior is already covered and passes; the fault is in the running embedded artifact.

- [x] **Step 3: Confirm no source edit is needed**

Run from the repository root:

```powershell
git diff -- frontend/src/views/SocialView.tsx frontend/src/views/SocialView.test.tsx frontend/src/style.css
```

Expected: no modification is necessary in the three source files. Do not introduce a duplicate CSS override or change the existing friend actions.

### Task 2: Regenerate And Inspect The Embedded WebUI

**Files:**
- Generate: `frontend/dist/index.html`
- Generate: `frontend/dist/assets/*`

- [x] **Step 1: Record the current generated asset names**

Run from `frontend`:

```powershell
Get-Content dist/index.html
Get-ChildItem dist/assets | Sort-Object LastWriteTime -Descending | Select-Object -First 5 Name, LastWriteTime, Length
```

Expected: retain the existing HTML asset references and timestamps as pre-build evidence.

- [x] **Step 2: Build the production frontend**

Run from `frontend`:

```powershell
npm run build
```

Expected: `tsc && vite build` exits with code `0` and regenerates `dist/index.html` plus its hashed JavaScript and CSS assets.

- [x] **Step 3: Inspect the generated labels and mobile styling**

Run from the repository root:

```powershell
$index = Get-Content -Raw frontend/dist/index.html
$script = [regex]::Match($index, 'src="(/assets/[^"]+\.js)"').Groups[1].Value.TrimStart('/')
$style = [regex]::Match($index, 'href="(/assets/[^"]+\.css)"').Groups[1].Value.TrimStart('/')
rg -n --fixed-strings '操作' (Join-Path frontend/dist $script)
rg -n --fixed-strings 'social-command-trigger' (Join-Path frontend/dist $style)
```

Expected: the emitted JavaScript contains `操作` and the emitted CSS contains the compact `social-command-trigger` styling. The referenced files must exist under `frontend/dist/assets`.

- [x] **Step 4: Check generated output without staging ignored artifacts**

Run from the repository root:

```powershell
git status --short
git check-ignore -v frontend/dist/index.html
```

Expected: `frontend/dist` is generated build output and remains unstaged; source files are unchanged.

### Task 3: Package The Executable And Validate The LAN Runtime

**Files:**
- Generate: `build/bin/Farm_Go.exe`
- Verify: `main.go`

- [x] **Step 1: Prove the generated bundle is the embedded source**

Run from the repository root:

```powershell
rg -n 'go:embed all:frontend/dist' main.go
```

Expected: the application embeds `frontend/dist`, establishing that the next Wails build packages the regenerated labels for LAN delivery.

- [x] **Step 2: Build the local executable**

Run from the repository root:

```powershell
wails build -clean
```

Expected: Wails exits with code `0`, re-runs the configured frontend build, and writes `build/bin/Farm_Go.exe` with a current timestamp.

- [x] **Step 3: Verify the packaged binary timestamp**

Run from the repository root:

```powershell
Get-Item build/bin/Farm_Go.exe | Select-Object FullName, Length, LastWriteTime
```

Expected: the timestamp is later than the regenerated `frontend/dist` assets.

- [ ] **Step 4: Start the freshly built executable and reload the LAN phone page**

Start `build/bin/Farm_Go.exe`, enable LAN access from its settings if it is not already enabled, then open the same LAN address on the phone. Perform a hard refresh or clear the page cache before rechecking the social page.

Expected: each friend row shows `操作` and `配置`; both buttons open the existing command sheet; desktop friend rows remain unaffected. The hard refresh is required because an already-open phone browser can retain the prior hashed asset bundle.

- [x] **Step 5: Record the final repository state**

Run from the repository root:

```powershell
git diff --check
git status --short
```

Expected: no whitespace errors and no unintended source changes. The implementation is a verified generated-artifact and executable refresh, with its design documentation already committed.
