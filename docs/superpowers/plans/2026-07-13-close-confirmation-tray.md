# Close Confirmation And Tray Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (\`- [ ]\`) syntax for tracking.

**Goal:** Require an explicit close decision and provide a recoverable Windows tray mode for Farm_Go.

**Architecture:** Keep the Wails lifecycle hook synchronous: normal closes are prevented and emit \`app:close-requested\`; an explicit exit consumes one backend approval before calling Wails quit. A root-level tray adapter owns the native icon and maps its menu callbacks onto \`App\` methods. A standalone React dialog listens for the request event inside the authorized UI and calls the generated App bindings.

**Tech Stack:** Go 1.25, Wails v2 runtime events/window APIs, \`github.com/getlantern/systray\`, React 18, TypeScript, Vitest, lucide-react.

---

## File Structure

- Create: \`desktop_lifecycle.go\` - close interception, one-time exit approval, and Wails window actions.
- Create: \`desktop_lifecycle_test.go\` - backend close and tray callback behavior with a fake desktop runtime.
- Create: \`tray.go\` - native tray adapter with show and exit menu callbacks.
- Create: \`frontend/src/components/CloseConfirmationDialog.tsx\` - presentational close-decision modal.
- Create: \`frontend/src/components/CloseConfirmationDialog.test.tsx\` - modal rendering and action tests.
- Modify: \`app.go\` - initialize desktop lifecycle state and start/stop the tray with application lifecycle.
- Modify: \`main.go\` - embed the existing application icon and register \`OnBeforeClose\`.
- Modify: \`go.mod\`, \`go.sum\` - add the native tray dependency.
- Modify: \`frontend/src/AuthorizedApp.tsx\` - subscribe to close requests, call App bindings, and render the dialog.
- Modify: \`frontend/src/App.test.tsx\` - mock and verify the close-request subscription cleanup.
- Modify: \`frontend/src/style.css\` - add dialog layout and responsive action styling.

### Task 1: Define And Test The Backend Close Controller

**Files:**
- Create: \`desktop_lifecycle.go\`
- Create: \`desktop_lifecycle_test.go\`
- Modify: \`app.go\`
- Modify: \`main.go\`

- [ ] **Step 1: Write the failing close-controller tests**

~~~go
func TestBeforeClosePreventsNormalCloseAndEmitsRequest(t *testing.T) {
  runtime := &fakeDesktopRuntime{}
  app := NewApp()
  app.desktopRuntime = runtime
  ctx := context.Background()

  if prevented := app.beforeClose(ctx); !prevented {
    t.Fatal("normal close should be prevented")
  }
  if got := runtime.events; !reflect.DeepEqual(got, []string{"app:close-requested"}) {
    t.Fatalf("events = %#v", got)
  }
}

func TestExitApplicationAllowsExactlyOneClose(t *testing.T) {
  runtime := &fakeDesktopRuntime{}
  app := NewApp()
  app.ctx = context.Background()
  app.desktopRuntime = runtime

  app.ExitApplication()
  if runtime.quitCalls != 1 { t.Fatalf("quit calls = %d", runtime.quitCalls) }
  if prevented := app.beforeClose(app.ctx); prevented { t.Fatal("approved close should proceed") }
  if prevented := app.beforeClose(app.ctx); !prevented { t.Fatal("later close should be prevented") }
}
~~~

- [ ] **Step 2: Run the focused backend tests to verify RED**

Run: \`go test . -run 'Test(BeforeClose|ExitApplication)' -count=1\`

Expected: FAIL because \`App.beforeClose\` and \`ExitApplication\` do not exist.

- [ ] **Step 3: Add an injectable desktop runtime and close methods**

~~~go
type desktopRuntime interface {
  Emit(context.Context, string)
  Quit(context.Context)
  Hide(context.Context)
  Show(context.Context)
}

type wailsDesktopRuntime struct{}

func (wailsDesktopRuntime) Emit(ctx context.Context, event string) { runtime.EventsEmit(ctx, event) }
func (wailsDesktopRuntime) Quit(ctx context.Context) { runtime.Quit(ctx) }
func (wailsDesktopRuntime) Hide(ctx context.Context) { runtime.WindowHide(ctx) }
func (wailsDesktopRuntime) Show(ctx context.Context) { runtime.WindowShow(ctx) }

func (a *App) beforeClose(ctx context.Context) bool {
  a.closeMu.Lock()
  approved := a.exitApproved
  a.exitApproved = false
  a.closeMu.Unlock()
  if approved { return false }
  a.desktopRuntime.Emit(ctx, "app:close-requested")
  return true
}

func (a *App) ExitApplication() {
  a.closeMu.Lock()
  a.exitApproved = true
  a.closeMu.Unlock()
  a.desktopRuntime.Quit(a.contextOrBackground())
}

func (a *App) MinimizeToTray() { a.desktopRuntime.Hide(a.contextOrBackground()) }
func (a *App) ShowMainWindow() { a.desktopRuntime.Show(a.contextOrBackground()) }
~~~

Add \`desktopRuntime desktopRuntime\`, \`closeMu sync.Mutex\`, and \`exitApproved bool\` to \`App\`; initialize \`desktopRuntime: wailsDesktopRuntime{}\` in \`NewApp\`. In \`main.go\`, add \`OnBeforeClose: app.beforeClose\` to the Wails options.

- [ ] **Step 4: Run the focused backend tests to verify GREEN**

Run: \`go test . -run 'Test(BeforeClose|ExitApplication)' -count=1\`

Expected: PASS.

- [ ] **Step 5: Commit the close controller**

~~~powershell
git add app.go main.go desktop_lifecycle.go desktop_lifecycle_test.go
git commit -m "feat: intercept close requests"
~~~

### Task 2: Add And Test The Windows Tray Adapter

**Files:**
- Create: \`tray.go\`
- Modify: \`desktop_lifecycle.go\`
- Modify: \`desktop_lifecycle_test.go\`
- Modify: \`app.go\`, \`main.go\`, \`go.mod\`, \`go.sum\`

- [ ] **Step 1: Add failing tests for tray callback routing**

~~~go
func TestTrayCallbacksRouteToWindowActions(t *testing.T) {
  runtime := &fakeDesktopRuntime{}
  app := NewApp()
  app.ctx = context.Background()
  app.desktopRuntime = runtime

  callbacks := app.trayCallbacks()
  callbacks.show()
  callbacks.quit()

  if runtime.showCalls != 1 || runtime.quitCalls != 1 {
    t.Fatalf("show/quit = %d/%d", runtime.showCalls, runtime.quitCalls)
  }
  if prevented := app.beforeClose(app.ctx); prevented {
    t.Fatal("tray exit should approve one close")
  }
}
~~~

- [ ] **Step 2: Run the tray callback test to verify RED**

Run: \`go test . -run TestTrayCallbacksRouteToWindowActions -count=1\`

Expected: FAIL because \`trayCallbacks\` does not exist.

- [ ] **Step 3: Add the tray package and production adapter**

Run: \`go get github.com/getlantern/systray@latest\`

Create \`tray.go\` with an adapter that launches \`systray.Run\` in a goroutine. Its ready callback uses the embedded \`appIcon\`, tooltip \`Farm_Go\`, and the following items:

~~~go
showItem := systray.AddMenuItem("显示主窗口", "恢复 Farm_Go")
exitItem := systray.AddMenuItem("退出程序", "退出 Farm_Go")
go func() {
  for {
    select {
    case <-showItem.ClickedCh:
      callbacks.show()
    case <-exitItem.ClickedCh:
      callbacks.quit()
      return
    }
  }
}()
~~~

Use \`func (a *App) trayCallbacks() trayCallbacks { return trayCallbacks{show: a.ShowMainWindow, quit: a.ExitApplication} }\`. Start the adapter from \`startup\` after \`a.ctx = ctx\`; call \`systray.Quit()\` from \`shutdown\`. Embed \`build/appicon.png\` in \`main.go\` as \`appIcon\`.

- [ ] **Step 4: Run the tray callback test to verify GREEN**

Run: \`go test . -run TestTrayCallbacksRouteToWindowActions -count=1\`

Expected: PASS without creating a native notification-area icon during tests.

- [ ] **Step 5: Commit the tray adapter**

~~~powershell
git add go.mod go.sum main.go app.go tray.go desktop_lifecycle.go desktop_lifecycle_test.go
git commit -m "feat: add Farm Go tray controls"
~~~

### Task 3: Add The Close Decision Dialog

**Files:**
- Create: \`frontend/src/components/CloseConfirmationDialog.tsx\`
- Create: \`frontend/src/components/CloseConfirmationDialog.test.tsx\`
- Modify: \`frontend/src/style.css\`

- [ ] **Step 1: Write failing dialog tests**

~~~tsx
it('renders exit as the autofocus primary action', () => {
  const html = renderToStaticMarkup(<CloseConfirmationDialog open onExit={() => undefined} onMinimize={() => undefined} onCancel={() => undefined} />);
  expect(html).toContain('关闭 Farm_Go');
  expect(html).toContain('最小化到托盘');
  expect(html).toContain('autofocus');
});

it('routes each explicit decision to its callback', () => {
  const onExit = vi.fn(); const onMinimize = vi.fn(); const onCancel = vi.fn();
  const renderer = create(<CloseConfirmationDialog open onExit={onExit} onMinimize={onMinimize} onCancel={onCancel} />);
  act(() => renderer.root.findByProps({ name: 'exit-application' }).props.onClick());
  act(() => renderer.root.findByProps({ name: 'minimize-to-tray' }).props.onClick());
  act(() => renderer.root.findByProps({ name: 'cancel-close' }).props.onClick());
  expect(onExit).toHaveBeenCalledOnce();
  expect(onMinimize).toHaveBeenCalledOnce();
  expect(onCancel).toHaveBeenCalledOnce();
});
~~~

- [ ] **Step 2: Run the focused dialog tests to verify RED**

Run: \`Set-Location frontend; npm test -- CloseConfirmationDialog\`

Expected: FAIL because the dialog module does not exist.

- [ ] **Step 3: Implement the modal and responsive styles**

~~~tsx
export function CloseConfirmationDialog({ open, onExit, onMinimize, onCancel }: CloseConfirmationDialogProps) {
  if (!open) return null;
  return <div className="dialog-backdrop close-confirmation-backdrop" role="presentation">
    <section className="close-confirmation-dialog" role="dialog" aria-modal="true" aria-labelledby="close-confirmation-title">
      <header><Power size={21} /><div><h2 id="close-confirmation-title">关闭 Farm_Go</h2><p>请选择退出程序或继续在后台运行。</p></div></header>
      <footer className="close-confirmation-actions">
        <button name="cancel-close" className="secondary-button" type="button" onClick={onCancel}>取消</button>
        <button name="minimize-to-tray" className="secondary-button" type="button" onClick={onMinimize}><Minimize2 size={17} />最小化到托盘</button>
        <button name="exit-application" className="primary-button" type="button" autoFocus onClick={onExit}><Power size={17} />退出程序</button>
      </footer>
    </section>
  </div>;
}
~~~

Add 8px-radius dialog styling and a \`max-width: 560px\` media rule that stacks the actions. Reuse \`dialog-backdrop\`, \`primary-button\`, and \`secondary-button\`; do not add a nested card surface.

- [ ] **Step 4: Run the focused dialog tests to verify GREEN**

Run: \`Set-Location frontend; npm test -- CloseConfirmationDialog\`

Expected: PASS.

- [ ] **Step 5: Commit the dialog component**

~~~powershell
git add frontend/src/components/CloseConfirmationDialog.tsx frontend/src/components/CloseConfirmationDialog.test.tsx frontend/src/style.css
git commit -m "feat: add close confirmation dialog"
~~~

### Task 4: Connect The Authorized UI To The Desktop Lifecycle

**Files:**
- Modify: \`frontend/src/AuthorizedApp.tsx\`
- Modify: \`frontend/src/App.test.tsx\`
- Modify: \`frontend/wailsjs/go/main/App.js\`
- Modify: \`frontend/wailsjs/go/main/App.d.ts\`

- [ ] **Step 1: Add a failing listener and action test**

Extend the existing \`AuthorizedApp\` module mock to export \`ExitApplication\` and \`MinimizeToTray\`. Call the stored \`app:close-requested\` callback, assert dialog text is rendered once, invoke the three named controls, then assert the respective binding mocks and listener cleanup were called.

- [ ] **Step 2: Run the frontend integration test to verify RED**

Run: \`Set-Location frontend; npm test -- App.test.tsx\`

Expected: FAIL because \`AuthorizedApp\` neither subscribes to \`app:close-requested\` nor renders the close dialog.

- [ ] **Step 3: Regenerate Wails bindings and connect the dialog**

Run: \`wails generate module\`

Keep generated binding updates only if the command adds:

~~~ts
export function ExitApplication() { return window['go']['main']['App']['ExitApplication'](); }
export function MinimizeToTray() { return window['go']['main']['App']['MinimizeToTray'](); }
~~~

In \`AuthorizedApp.tsx\`, import \`EventsOn\`, \`ExitApplication\`, \`MinimizeToTray\`, and \`CloseConfirmationDialog\`. Add \`const [closeConfirmationOpen, setCloseConfirmationOpen] = useState(false)\` and subscribe once:

~~~tsx
useEffect(() => EventsOn('app:close-requested', () => setCloseConfirmationOpen(true)), []);
~~~

Render the dialog beside the existing global dialogs. Its exit handler awaits \`ExitApplication()\`; its minimize handler awaits \`MinimizeToTray()\` and clears local dialog state; its cancel handler only clears local dialog state. Do not call browser \`window.close\`.

- [ ] **Step 4: Run the frontend integration test to verify GREEN**

Run: \`Set-Location frontend; npm test -- App.test.tsx\`

Expected: PASS.

- [ ] **Step 5: Run all automated verification**

~~~powershell
go test ./...
Set-Location frontend
npm test
npm run build
~~~

Expected: all Go tests, all Vitest tests, TypeScript checking, and the Vite production build PASS.

- [ ] **Step 6: Manually verify the Windows behavior**

Run: \`wails dev\`

Verify: native X opens one dialog; \`退出程序\` exits; \`最小化到托盘\` hides the window but leaves automation running; tray \`显示主窗口\` restores the app; tray \`退出程序\` exits without reopening the dialog.

- [ ] **Step 7: Commit the UI integration**

~~~powershell
git add frontend/src/AuthorizedApp.tsx frontend/src/App.test.tsx frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts
git commit -m "feat: confirm close or minimize to tray"
~~~
