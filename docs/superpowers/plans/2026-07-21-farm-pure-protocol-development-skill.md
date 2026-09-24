# Farm Pure Protocol Development Skill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a reusable personal skill for zero-UI QQ Farm protocol discovery and retain the reusable capture scripts while deleting raw capture data.

**Architecture:** A dedicated personal skill documents the evidence-gated workflow and points to Farm_Go runtime conventions. Project scripts move from the ephemeral capture directory into `scripts/protocol-capture/`, while `.gitignore` reserves `data/debug-captures/` for disposable runtime evidence.

**Tech Stack:** Codex skills, Markdown, YAML, Node.js WebSocket diagnostics, Go QQ patch helper, Git.

---

### Task 1: Initialize the personal skill

**Files:**
- Create: `C:/Users/奥森/.codex/skills/farm-pure-protocol-development/SKILL.md`
- Create: `C:/Users/奥森/.codex/skills/farm-pure-protocol-development/agents/openai.yaml`
- Create: `C:/Users/奥森/.codex/skills/farm-pure-protocol-development/references/farm-go-runtime.md`

- [x] **Step 1: Initialize the skill scaffold**

Run:

```powershell
python C:/Users/奥森/.codex/skills/.system/skill-creator/scripts/init_skill.py farm-pure-protocol-development --path C:/Users/奥森/.codex/skills --resources references --interface 'display_name=Farm Pure Protocol Development' --interface 'short_description=Develop QQ Farm features through verified zero-UI protocols' --interface 'default_prompt=Use $farm-pure-protocol-development to discover and implement this QQ Farm feature through pure protocol calls without UI automation.'
```

Expected: the skill folder, `SKILL.md`, `agents/openai.yaml`, and `references/` are created.

- [x] **Step 2: Replace the generated SKILL.md**

Use this frontmatter and workflow structure:

```markdown
---
name: farm-pure-protocol-development
description: Use when a Farm_Go QQ Farm feature must be discovered, implemented, repaired, or verified through direct QQ WS protocols without opening or clicking game UI, especially for status queries, dynamic IDs, protobuf callback decoding, strict field extraction, request encoding, and live zero-UI acceptance checks.
---

# Farm Pure Protocol Development

1. Prove the query/action protocol with runtime spies.
2. Add a bounded query probe before implementing a parser.
3. Capture the callback through Wails RPC.
4. Decode with the exact runtime protobuf codec.
5. Freeze field paths; never guess IDs.
6. Implement through VM-based TDD.
7. Patch/restart QQ and verify no UI events.
```

The complete body must include evidence gates, query failure behavior, no-fallback rules, artifact hygiene, and the handoff to `farm-protocol-capture` when a manual click is explicitly required.

- [x] **Step 3: Write the runtime reference**

Document these exact project facts:

```text
Wails RPC: ws://127.0.0.1:34115/wails/ipc
QQ runtime: ws://127.0.0.1:8787/runtime/qqws
RPC call prefix: C
RPC response prefix: c
Diagnostic method: main.App.RunDiagnostic
Production script: resources/wmpf/button.js
Capture output: data/debug-captures/
```

Include the callback envelope, `*pb.ts` System codec lookup, patch/restart sequence, and reusable script paths.

- [x] **Step 4: Validate the skill**

Run:

```powershell
python C:/Users/奥森/.codex/skills/.system/skill-creator/scripts/quick_validate.py C:/Users/奥森/.codex/skills/farm-pure-protocol-development
```

Expected: `Skill is valid!`

### Task 2: Preserve scripts outside the capture directory

**Files:**
- Create: `scripts/protocol-capture/install-current-qq-patch.go`
- Create: `scripts/protocol-capture/svip-live-capture.cjs`
- Create: `scripts/protocol-capture/svip-status-capture.cjs`
- Create: `scripts/protocol-capture/svip-dynamic-live.cjs`
- Delete: matching script files under `data/debug-captures/`

- [x] **Step 1: Move the four scripts with Git-visible paths**

Move each file without changing its contents. Verify hashes before and after:

```powershell
Get-FileHash data/debug-captures/install-current-qq-patch.go
Get-FileHash scripts/protocol-capture/install-current-qq-patch.go
```

Repeat the hash comparison for all four scripts. Expected: each source/destination hash pair matches before the old copy is removed.

- [x] **Step 2: Remove the user-specific ws dependency path**

In the three `.cjs` files, replace the absolute `C:/Users/奥森/.../ws` import with a resolver that first uses project `ws`, then the sibling `farm-protocol-capture` skill:

```js
function requireWebSocket() {
  try { return require("ws"); } catch (_) {}
  return require(path.join(process.env.USERPROFILE, ".codex", "skills", "farm-protocol-capture", "scripts", "node_modules", "ws"));
}
const WebSocket = requireWebSocket();
```

- [x] **Step 3: Syntax-check retained scripts**

Run:

```powershell
node --check scripts/protocol-capture/svip-live-capture.cjs
node --check scripts/protocol-capture/svip-status-capture.cjs
node --check scripts/protocol-capture/svip-dynamic-live.cjs
go run ./scripts/protocol-capture/install-current-qq-patch.go
```

Expected: Node syntax checks exit 0. The Go helper exits 2 with its usage message because no target path was supplied, proving it compiles without modifying QQ.

### Task 3: Delete captures and prevent future Git noise

**Files:**
- Modify: `.gitignore`
- Delete: `data/debug-captures/*.json`
- Delete: `data/debug-captures/*.log`

- [x] **Step 1: Add the capture directory to .gitignore**

Append exactly:

```gitignore
/data/debug-captures/
```

- [x] **Step 2: Verify deletion targets**

Resolve every `.json` and `.log` path and confirm it is under `E:/desktop/Farm_Go/data/debug-captures/`. Do not delete any script in this step.

- [x] **Step 3: Delete capture artifacts**

Delete only `.json` and `.log` files from `data/debug-captures/`, then remove the old script copies after their preserved versions are verified.

- [x] **Step 4: Verify repository state**

Run: `git status --short`

Expected: only `.gitignore`, the implementation plan, and `scripts/protocol-capture/` are changed; `data/debug-captures/` is absent.

### Task 4: Final validation and commit

**Files:**
- Verify: all files above

- [x] **Step 1: Run focused validation**

Run:

```powershell
python C:/Users/奥森/.codex/skills/.system/skill-creator/scripts/quick_validate.py C:/Users/奥森/.codex/skills/farm-pure-protocol-development
node --check scripts/protocol-capture/svip-live-capture.cjs
node --check scripts/protocol-capture/svip-status-capture.cjs
node --check scripts/protocol-capture/svip-dynamic-live.cjs
go test ./... -count=1
git diff --check
```

Expected: all commands exit 0.

- [x] **Step 2: Commit project-owned files**

Run:

```powershell
git add .gitignore scripts/protocol-capture docs/superpowers/plans/2026-07-21-farm-pure-protocol-development-skill.md
git commit -m "chore: preserve pure protocol capture workflow"
```

Expected: the commit contains no `data/debug-captures/` artifacts and no personal skill files.
