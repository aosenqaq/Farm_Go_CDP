# Guardian Relogin Popup Handling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the existing guardian reconnect watcher immediately and safely click the unique "重新登录" button on the generic two-button platform-login-failure popup.

**Architecture:** Keep `ServerKickOutUICom` as the existing fast path, then add a narrowly constrained fallback that starts from active `cc.Button` nodes labeled exactly "重新登录" and finds the nearest active ancestor containing the login-failure copy and an "退出游戏" alternative. Feed that fallback through the existing reconnect prompt state, click, recovery, episode-suppression, and event-reporting pipeline.

**Tech Stack:** Injected JavaScript (`resources/wmpf/button.js`), Node.js `vm` test harness, `node:assert`, Go guardian/runtime tests

---

## File Map

- Modify `scripts/test-guardian-runtime-watchers.js`: add label/rich-text support to the fake Cocos runtime, model the generic two-button login-failure popup, and assert that only its relogin handler runs.
- Modify `resources/wmpf/button.js`: discover a unique generic relogin popup from active text/button structure, summarize it for the existing reconnect state, and click its positive button through `smartClick`.

### Task 1: Reproduce the missing generic popup behavior

**Files:**
- Modify: `scripts/test-guardian-runtime-watchers.js:48-222,751-769`
- Test: `scripts/test-guardian-runtime-watchers.js`

- [ ] **Step 1: Add fake text components and separate action counters**

Add fake Cocos text classes next to `FakeButton` and initialize an exit counter with the existing confirmation counter:

```js
class FakeLabel {
  constructor(string) {
    this.string = string;
  }
}

class FakeRichText extends FakeLabel {}
```

```js
const runtime = {
  confirmCount: 0,
  exitCount: 0,
  hideOnConfirm: options.hideOnConfirm !== false,
};
```

Expose both classes in the fake `cc` object so production text traversal uses the same API as Cocos:

```js
Label: FakeLabel,
RichText: FakeRichText,
```

- [ ] **Step 2: Model the screenshot's generic two-button popup**

Add this component next to `ServerKickOutUICom`:

```js
class GenericReloginPromptComp {
  constructor(node, runtime) {
    this.node = node;
    this.runtime = runtime;
  }

  relogin() {
    this.runtime.confirmCount += 1;
    if (this.runtime.hideOnConfirm) this.runtime.setPromptVisible(false);
  }

  exitGame() {
    this.runtime.exitCount += 1;
  }
}
```

When `options.promptKind === 'generic_relogin'`, use this helper to build the active hierarchy instead of attaching `ServerKickOutUICom` to the prompt root:

```text
Scene/CommonPrompt
  title                 cc.Label("提示信息")
  content               cc.RichText("平台登录失败，是否重新登录？")
  btn_exit               cc.Button -> GenericReloginPromptComp.exitGame
    label                cc.Label("退出游戏")
  btn_relogin            cc.Button -> GenericReloginPromptComp.relogin
    label                cc.Label("重新登录")
```

```js
function addTextNode(parent, name, text, TextComponent) {
  const node = new FakeNode(name);
  node.components.push(new TextComponent(text));
  parent.addChild(node);
  return node;
}

function addPromptButton(parent, name, label, handler) {
  const node = new FakeNode(name);
  node.components.push(new FakeButton([{
    target: parent,
    component: 'GenericReloginPromptComp',
    handler,
    customEventData: '',
  }]));
  addTextNode(node, 'label', label, FakeLabel);
  parent.addChild(node);
  return node;
}

const component = new GenericReloginPromptComp(promptNode, runtime);
promptNode.components.push(component);
addTextNode(promptNode, 'title', '提示信息', FakeLabel);
addTextNode(
  promptNode,
  'content',
  options.promptContent || '平台登录失败，是否重新登录？',
  FakeRichText,
);
addPromptButton(promptNode, 'btn_exit', '退出游戏', 'exitGame');
addPromptButton(
  promptNode,
  'btn_relogin',
  options.reloginLabel || '重新登录',
  'relogin',
);
if (options.duplicateRelogin) {
  addPromptButton(promptNode, 'btn_relogin_duplicate', '重新登录', 'relogin');
}
```

Both button events target `CommonPrompt`, set `component: 'GenericReloginPromptComp'`, and name their exact handlers. Update `runtime.setPromptVisible` to toggle the selected prompt root and all descendants with this local helper:

```js
function setNodeTreeActive(node, visible) {
  if (!node) return;
  node.active = !!visible;
  node.activeInHierarchy = !!visible;
  for (const child of node.children || []) setNodeTreeActive(child, visible);
}
```

- [ ] **Step 3: Add the positive-path regression test**

Add the test before the other-place watcher tests:

```js
async function testGenericReloginPopupClicksOnlyRelogin() {
  const runtime = createRuntime({ promptKind: 'generic_relogin' });
  const { gameCtl, clock } = runtime;
  gameCtl.startReconnectWatcher({
    silent: true,
    intervalMs: 1000,
    delayMs: 1000,
    waitAfter: 0,
    recoverTimeoutMs: 20000,
  });

  await clock.advance(1000);

  const events = eventsByName(runtime, 'network_reconnect');
  assert.equal(runtime.confirmCount, 1, 'generic popup triggers relogin once');
  assert.equal(runtime.exitCount, 0, 'generic popup never triggers exit game');
  assert.equal(events.length, 1);
  assert.equal(events[0].phase, 'reconnected');
  assert.equal(events[0].handled, true);
  assert.equal(events[0].via, 'relogin_ok_node');
  gameCtl.stopReconnectWatcher({ silent: true });
}
```

Call it from `run()` after the existing network watcher cases and before other-place login cases.

- [ ] **Step 4: Add strict-match safety cases**

Allow `createRuntime` to override the generic body text and positive button text with `options.promptContent` and `options.reloginLabel`. Add one table-driven test:

```js
async function testGenericReloginPopupRejectsIncompleteMatches() {
  const cases = [
    { promptContent: '普通提示，是否继续？', reloginLabel: '重新登录' },
    { promptContent: '平台登录失败，是否重新登录？', reloginLabel: '确定' },
    { promptContent: '平台登录失败，是否重新登录？', reloginLabel: '重新登录', duplicateRelogin: true },
  ];

  for (const testCase of cases) {
    const runtime = createRuntime({ promptKind: 'generic_relogin', ...testCase });
    const { gameCtl, clock } = runtime;
    gameCtl.startReconnectWatcher({ silent: true, intervalMs: 1000, delayMs: 1000, waitAfter: 0 });
    await clock.advance(1000);
    assert.equal(runtime.confirmCount, 0);
    assert.equal(runtime.exitCount, 0);
    assert.equal(eventsByName(runtime, 'network_reconnect').length, 0);
    gameCtl.stopReconnectWatcher({ silent: true });
  }
}
```

Call this test immediately after the positive generic relogin case.

- [ ] **Step 5: Run the focused test and verify RED**

Run from the repository root:

```powershell
node scripts/test-guardian-runtime-watchers.js
```

Expected: FAIL in `testGenericReloginPopupClicksOnlyRelogin` because `findServerKickOutUiComp` cannot see `GenericReloginPromptComp`, leaving `confirmCount` at `0`. The pre-existing watcher tests must have passed before that assertion.

### Task 2: Add the constrained generic relogin fallback

**Files:**
- Modify: `resources/wmpf/button.js:6002-6033,6091-6139,6238-6291`
- Test: `scripts/test-guardian-runtime-watchers.js`

- [ ] **Step 1: Read labels and rich text, then add exact button-text and ancestor helpers**

Extend the existing `getNodeTextList` visitor so generic popup content is visible whether Cocos renders it as `cc.Label` or `cc.RichText`:

```js
const label = cc.Label && cur.getComponent ? cur.getComponent(cc.Label) : null;
const richText = cc.RichText && cur.getComponent ? cur.getComponent(cc.RichText) : null;
const values = [
  label && typeof label.string === 'string' ? label.string.trim() : '',
  richText && typeof richText.string === 'string' ? richText.string.trim() : '',
];
for (let i = 0; i < values.length; i += 1) {
  const text = values[i];
  if (text && !seen.has(text)) {
    seen.add(text);
    texts.push(text);
  }
}
```

Then add the following helpers after `summarizeServerKickOutUiComp`. They deliberately start from real active `cc.Button` nodes and reject ambiguous matches:

```js
function nodeHasExactText(node, expected, maxDepth) {
  const normalizedExpected = normalizeMatchText(expected);
  return getNodeTextList(node, { maxDepth: maxDepth == null ? 2 : maxDepth }).some(function (text) {
    return normalizeMatchText(text) === normalizedExpected;
  });
}

function findGenericReloginPrompt(opts) {
  opts = opts || {};
  const root = scene();
  const nodes = walk(root);
  const candidates = [];

  for (let i = 0; i < nodes.length; i += 1) {
    const buttonNode = nodes[i];
    if (opts.activeOnly !== false && !buttonNode.activeInHierarchy) continue;
    const button = cc.Button && buttonNode.getComponent ? buttonNode.getComponent(cc.Button) : null;
    if (!button || !nodeHasExactText(buttonNode, '重新登录', 2)) continue;

    for (let ancestor = buttonNode.parent; ancestor && ancestor !== root; ancestor = ancestor.parent) {
      if (opts.activeOnly !== false && !ancestor.activeInHierarchy) continue;
      const texts = getNodeTextList(ancestor, { maxDepth: 8 });
      const joined = texts.join(' ');
      if (!nodeHasExactText(ancestor, '退出游戏', 8)) continue;
      if (!isReloginPromptText({ content: joined, okWord: '重新登录' })) continue;
      candidates.push({ node: ancestor, okNode: buttonNode, texts: texts });
      break;
    }
  }

  return candidates.length === 1 ? candidates[0] : null;
}

function summarizeGenericReloginPrompt(prompt) {
  if (!prompt || !prompt.node || !prompt.okNode) return null;
  const texts = Array.isArray(prompt.texts) ? prompt.texts : [];
  return {
    source: 'generic_relogin',
    path: fullPath(prompt.node),
    active: !!prompt.node.active,
    activeInHierarchy: !!prompt.node.activeInHierarchy,
    title: texts.length > 0 ? texts[0] : '',
    content: texts.join(' '),
    operateContent: '',
    okWord: '重新登录',
    cancelWord: '退出游戏',
    okNodePath: fullPath(prompt.okNode),
    cancelNodePath: null,
  };
}
```

- [ ] **Step 2: Feed the fallback into reconnect prompt state**

In `getReconnectPromptState`, keep both lookup errors isolated and prefer the existing component:

```js
let genericReloginPrompt = null;
let genericReloginError = null;
```

```js
try {
  genericReloginPrompt = findGenericReloginPrompt({ activeOnly: opts.activeOnly !== false });
} catch (error) {
  genericReloginError = error instanceof Error ? error.message : String(error);
}
```

Replace the UI summary assignment with:

```js
const ui = summarizeServerKickOutUiComp(serverKickOutComp)
  || summarizeGenericReloginPrompt(genericReloginPrompt);
```

Include `genericReloginError` under `payload.errors.genericRelogin` alongside the existing net/UI errors. Do not change `promptByUi`, `promptByNet`, game-state, or event logic.

- [ ] **Step 3: Click only the discovered generic positive button**

In `clickReconnectPrompt`, resolve the generic candidate only when the state summary identifies that source:

```js
const genericReloginPrompt = before && before.ui && before.ui.source === 'generic_relogin'
  ? findGenericReloginPrompt({ activeOnly: opts.activeOnly !== false })
  : null;
```

Before the existing `ServerKickOutUICom` block, add:

```js
if (genericReloginPrompt && genericReloginPrompt.okNode && before.ui && isReloginPromptText(before.ui)) {
  try {
    smartClick(genericReloginPrompt.okNode);
    via = 'relogin_ok_node';
  } catch (_) {}
}
```

Keep the existing specialized component and `NetNode.reconnect` fallbacks unchanged. Because the generic finder requires one unique active relogin button, this code never selects the exit button and does not use child order or coordinates.

- [ ] **Step 4: Run the watcher test and verify GREEN**

Run:

```powershell
node scripts/test-guardian-runtime-watchers.js
```

Expected: exit code `0` and `guardian runtime watcher tests passed`. The generic popup increments `confirmCount` once, leaves `exitCount` at zero, records `via: relogin_ok_node`, and the strict-match cases remain untouched.

- [ ] **Step 5: Run the complete Go test suite**

Run:

```powershell
go test ./...
```

Expected: exit code `0` with every package passing. This covers embedded `button.js` integration, guardian Supervisor worker synchronization, QQ runtime delivery, and unrelated application regressions.

- [ ] **Step 6: Verify formatting, scope, and generated files**

Run:

```powershell
git diff --check
git diff -- resources/wmpf/button.js scripts/test-guardian-runtime-watchers.js
git status --short
```

Expected: no whitespace errors; only the two planned implementation files plus this plan are relevant. The untracked `data/debug-captures/` directory contains failed diagnostic listener output and must not be staged.

- [ ] **Step 7: Commit the implementation**

```powershell
git add -- resources/wmpf/button.js scripts/test-guardian-runtime-watchers.js
git commit -m "fix: handle guardian relogin popup"
```
