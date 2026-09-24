# Wails Desktop Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first phase of Farm_Go as a polished Windows desktop application using Wails, with a Go backend, React/Vite frontend, SQLite storage, and a QQ WS runtime diagnostics flow.

**Architecture:** Wails owns the desktop window and bridges the React UI to Go services. Go remains the core application layer: it starts the QQ WS server, manages runtime status, persists settings/events in SQLite, and exposes backend methods to the UI through Wails bindings. The first phase is local-only desktop software; it does not provide a browser WebUI or remote access surface.

**Tech Stack:** Wails v2, Go, React, TypeScript, Vite, Tailwind CSS, shadcn-style components, lucide-react, SQLite via `modernc.org/sqlite`, `log/slog`, `github.com/gorilla/websocket`.

---

## Scope Decisions

- Use Wails instead of Tauri.
- Do not use `agmmnn/tauri-controls`; it depends on Tauri and is not applicable in a Wails app.
- Do not build a browser WebUI in phase 1.
- Do not implement WMPF, Frida, CDP, or real farm tasks in phase 1.
- Keep the phase focused on local desktop operation and QQ WS diagnostics.
- Prioritize a clean, modern UI over pure-Go widgets.

## Target User Experience

When the user starts `Farm_Go.exe`, a desktop window opens immediately. The app shows runtime status, connection instructions, logs, settings, and a diagnostics panel. A QQ host can connect to the local QQ WS endpoint, complete a handshake, and respond to `host.describe` plus one `gameCtl` probe. The user can see connection state and diagnostic results without opening a browser.

## File Structure

Create the project structure below:

```text
Farm_Go/
  README.md
  TECH_STACK.md
  go.mod
  go.sum
  main.go
  app.go
  internal/
    app/
      app.go
      lifecycle.go
    config/
      config.go
    diagnostics/
      service.go
    eventbus/
      bus.go
      event.go
    logging/
      logger.go
    runtime/
      status.go
      manager.go
      qqws/
        adapter.go
        protocol.go
    storage/
      db.go
      migrations.go
      settings.go
      events.go
      diagnostics.go
  frontend/
    package.json
    src/
      App.tsx
      main.tsx
      styles.css
      lib/
        cn.ts
        api.ts
      components/
        AppShell.tsx
        StatusBadge.tsx
        LogViewer.tsx
      views/
        OverviewView.tsx
        ConnectionView.tsx
        DiagnosticsView.tsx
        SettingsView.tsx
```

Responsibilities:

- `app.go`: Wails binding surface; exposes methods the frontend can call.
- `internal/app`: owns startup/shutdown orchestration.
- `internal/runtime`: runtime state model and manager.
- `internal/runtime/qqws`: QQ host WebSocket server, handshake, pending request tracking.
- `internal/diagnostics`: `host.describe` and `gameCtl` probe orchestration.
- `internal/storage`: SQLite connection, migrations, settings, events, diagnostic records.
- `internal/eventbus`: in-memory pub/sub for status and log updates.
- `frontend/src/views`: screen-level UI.
- `frontend/src/components`: reusable UI pieces.

## Task 1: Scaffold Wails App

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `app.go`
- Create: `frontend/package.json`

- [ ] **Step 1: Install and verify Wails CLI**

Run:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails doctor
```

Expected:

```text
Wails CLI installed
System checks complete
```

If `wails` is not on PATH, add `%USERPROFILE%\go\bin` to PATH and rerun `wails doctor`.

- [ ] **Step 2: Initialize the project**

Run from `E:\desktop\Farm_Go`:

```powershell
wails init -n Farm_Go -t react-ts
```

Expected:

```text
Project 'Farm_Go' generated
```

If Wails refuses to initialize into a non-empty folder, initialize into a temporary folder and copy the generated Wails files into `E:\desktop\Farm_Go` without deleting `TECH_STACK.md` or this plan.

- [ ] **Step 3: Run the starter app**

Run:

```powershell
wails dev
```

Expected:

```text
Vite server started
Wails application opened
```

Manual check: a desktop window opens.

- [ ] **Step 4: Commit scaffold**

Run:

```powershell
git status --short
git add .
git commit -m "chore: scaffold Wails desktop app"
```

Expected:

```text
[main ...] chore: scaffold Wails desktop app
```

If this folder is not a git repository, skip the commit and record that in the task notes.

## Task 2: Add UI Styling Foundation

**Files:**
- Modify: `frontend/package.json`
- Create: `frontend/src/styles.css`
- Create: `frontend/src/lib/cn.ts`
- Modify: `frontend/src/main.tsx`
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Install UI dependencies**

Run:

```powershell
cd frontend
npm install tailwindcss @tailwindcss/vite clsx tailwind-merge lucide-react
```

Expected:

```text
added ... packages
```

- [ ] **Step 2: Configure Vite for Tailwind**

Modify `frontend/vite.config.ts`:

```ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
});
```

- [ ] **Step 3: Create global styles**

Create `frontend/src/styles.css`:

```css
@import "tailwindcss";

:root {
  color: #17201a;
  background: #f5f7f2;
  font-family:
    Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont,
    "Segoe UI", sans-serif;
}

body {
  margin: 0;
  min-width: 960px;
  min-height: 640px;
  overflow: hidden;
}

button,
input,
textarea,
select {
  font: inherit;
}
```

- [ ] **Step 4: Create class merge helper**

Create `frontend/src/lib/cn.ts`:

```ts
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
```

- [ ] **Step 5: Import styles**

Modify `frontend/src/main.tsx` so it imports the stylesheet:

```ts
import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./styles.css";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
```

- [ ] **Step 6: Verify UI compiles**

Run:

```powershell
cd ..
wails dev
```

Expected:

```text
Vite server started
Wails application opened
```

Manual check: window opens without a blank screen.

## Task 3: Define Core Go Models

**Files:**
- Create: `internal/runtime/status.go`
- Create: `internal/eventbus/event.go`
- Create: `internal/config/config.go`

- [ ] **Step 1: Create runtime status model**

Create `internal/runtime/status.go`:

```go
package runtime

type Phase string

const (
	PhaseIdle         Phase = "idle"
	PhaseListening    Phase = "listening"
	PhaseHandshaking  Phase = "handshaking"
	PhaseReady        Phase = "ready"
	PhaseDisconnected Phase = "disconnected"
	PhaseError        Phase = "error"
)

type Status struct {
	Target      string `json:"target"`
	Phase       Phase  `json:"phase"`
	Connected   bool   `json:"connected"`
	Ready       bool   `json:"ready"`
	InstanceID  string `json:"instanceId,omitempty"`
	HostVersion string `json:"hostVersion,omitempty"`
	LastSeenAt  string `json:"lastSeenAt,omitempty"`
	LastError   string `json:"lastError,omitempty"`
}

func InitialStatus() Status {
	return Status{
		Target: "qqws",
		Phase:  PhaseIdle,
	}
}
```

- [ ] **Step 2: Create event model**

Create `internal/eventbus/event.go`:

```go
package eventbus

import "time"

type Level string

const (
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

type Event struct {
	ID        int64          `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Level     Level          `json:"level"`
	Source    string         `json:"source"`
	Type      string         `json:"type"`
	Message   string         `json:"message"`
	Data      map[string]any `json:"data,omitempty"`
}
```

- [ ] **Step 3: Create config model**

Create `internal/config/config.go`:

```go
package config

type Config struct {
	QQWS QQWSConfig `json:"qqws"`
	UI   UIConfig   `json:"ui"`
}

type QQWSConfig struct {
	Host                string `json:"host"`
	Port                int    `json:"port"`
	Path                string `json:"path"`
	HostToken           string `json:"hostToken"`
	ExpectedHostVersion string `json:"expectedHostVersion"`
}

type UIConfig struct {
	Theme string `json:"theme"`
}

func Default() Config {
	return Config{
		QQWS: QQWSConfig{
			Host:                "127.0.0.1",
			Port:                8787,
			Path:                "/runtime/qqws",
			ExpectedHostVersion: "farm-go-host-1",
		},
		UI: UIConfig{
			Theme: "system",
		},
	}
}
```

- [ ] **Step 4: Verify Go package compiles**

Run:

```powershell
go test ./...
```

Expected:

```text
ok ...
```

If packages have no tests yet, expected output may include `? ... [no test files]`.

## Task 4: Add SQLite Storage

**Files:**
- Create: `internal/storage/db.go`
- Create: `internal/storage/migrations.go`
- Create: `internal/storage/settings.go`
- Create: `internal/storage/events.go`
- Create: `internal/storage/diagnostics.go`

- [ ] **Step 1: Install SQLite dependency**

Run:

```powershell
go get modernc.org/sqlite
```

Expected:

```text
go: added modernc.org/sqlite ...
```

- [ ] **Step 2: Create database opener**

Create `internal/storage/db.go`:

```go
package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", filepath.Join(dataDir, "farm_go.db"))
	if err != nil {
		return nil, err
	}

	store := &Store{db: db}
	if err := store.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
```

- [ ] **Step 3: Create migrations**

Create `internal/storage/migrations.go`:

```go
package storage

import "context"

func (s *Store) Migrate(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS runtime_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts TEXT NOT NULL,
			level TEXT NOT NULL,
			source TEXT NOT NULL,
			event_type TEXT NOT NULL,
			message TEXT NOT NULL,
			data_json TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS diagnostic_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts TEXT NOT NULL,
			method TEXT NOT NULL,
			params_json TEXT,
			ok INTEGER NOT NULL,
			duration_ms INTEGER NOT NULL,
			result_json TEXT,
			error TEXT
		);`,
	}

	for _, query := range queries {
		if _, err := s.db.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Verify migration**

Run:

```powershell
go test ./internal/storage
```

Expected:

```text
? .../internal/storage [no test files]
```

- [ ] **Step 5: Add repository methods in follow-up tasks**

Keep `settings.go`, `events.go`, and `diagnostics.go` focused:

```go
package storage
```

This empty package stub keeps file ownership clear before repository methods are added in Tasks 8 and 9.

## Task 5: Build Runtime Manager

**Files:**
- Create: `internal/eventbus/bus.go`
- Create: `internal/runtime/manager.go`

- [ ] **Step 1: Create event bus**

Create `internal/eventbus/bus.go`:

```go
package eventbus

import "sync"

type Bus struct {
	mu          sync.Mutex
	nextID      int64
	subscribers map[chan Event]struct{}
}

func New() *Bus {
	return &Bus{subscribers: map[chan Event]struct{}{}}
}

func (b *Bus) Publish(event Event) {
	b.mu.Lock()
	b.nextID++
	event.ID = b.nextID
	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
	b.mu.Unlock()
}

func (b *Bus) Subscribe() (chan Event, func()) {
	ch := make(chan Event, 32)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		delete(b.subscribers, ch)
		close(ch)
		b.mu.Unlock()
	}
	return ch, cancel
}
```

- [ ] **Step 2: Create runtime manager**

Create `internal/runtime/manager.go`:

```go
package runtime

import "sync"

type Manager struct {
	mu     sync.RWMutex
	status Status
}

func NewManager() *Manager {
	return &Manager{status: InitialStatus()}
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) SetStatus(status Status) {
	m.mu.Lock()
	m.status = status
	m.mu.Unlock()
}
```

- [ ] **Step 3: Verify**

Run:

```powershell
go test ./internal/eventbus ./internal/runtime
```

Expected:

```text
? ... [no test files]
```

## Task 6: Add QQ WS Protocol

**Files:**
- Create: `internal/runtime/qqws/protocol.go`

- [ ] **Step 1: Define protocol messages**

Create `internal/runtime/qqws/protocol.go`:

```go
package qqws

type Message struct {
	Type      string         `json:"type"`
	RequestID string         `json:"requestId,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	OK        *bool          `json:"ok,omitempty"`
	Result    any            `json:"result,omitempty"`
	Error     *RPCError      `json:"error,omitempty"`
}

type RPCError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type HelloPayload struct {
	HostVersion  string   `json:"hostVersion"`
	InstanceID   string   `json:"instanceId"`
	Capabilities []string `json:"capabilities"`
}

const (
	MessageHello       = "hello"
	MessageHelloOK     = "hello.ok"
	MessageRPCRequest  = "rpc.request"
	MessageRPCResponse = "rpc.response"
	MessagePing        = "ping"
	MessagePong        = "pong"
)
```

- [ ] **Step 2: Verify**

Run:

```powershell
go test ./internal/runtime/qqws
```

Expected:

```text
? .../internal/runtime/qqws [no test files]
```

## Task 7: Implement QQ WS Adapter Skeleton

**Files:**
- Create: `internal/runtime/qqws/adapter.go`

- [ ] **Step 1: Install WebSocket dependency**

Run:

```powershell
go get github.com/gorilla/websocket
```

Expected:

```text
go: added github.com/gorilla/websocket ...
```

- [ ] **Step 2: Create adapter**

Create `internal/runtime/qqws/adapter.go`:

```go
package qqws

import (
	"context"
	"net/http"
	"time"

	"Farm_Go/internal/config"
	farmruntime "Farm_Go/internal/runtime"
	"github.com/gorilla/websocket"
)

type Adapter struct {
	cfg     config.QQWSConfig
	manager *farmruntime.Manager
	server  *http.Server
}

func New(cfg config.QQWSConfig, manager *farmruntime.Manager) *Adapter {
	return &Adapter{cfg: cfg, manager: manager}
}

func (a *Adapter) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc(a.cfg.Path, a.handleWebSocket)

	a.server = &http.Server{
		Addr:              a.cfg.Host + ":" + portString(a.cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	a.manager.SetStatus(farmruntime.Status{
		Target: "qqws",
		Phase:  farmruntime.PhaseListening,
	})

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = a.server.Shutdown(shutdownCtx)
	}()

	err := a.server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (a *Adapter) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("token") != a.cfg.HostToken {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
}

func portString(port int) string {
	return fmt.Sprintf("%d", port)
}
```

- [ ] **Step 3: Fix missing import**

Add `fmt` to the import block in `adapter.go`:

```go
import (
	"context"
	"fmt"
	"net/http"
	"time"

	"Farm_Go/internal/config"
	farmruntime "Farm_Go/internal/runtime"
	"github.com/gorilla/websocket"
)
```

- [ ] **Step 4: Verify**

Run:

```powershell
go test ./internal/runtime/qqws
```

Expected:

```text
ok ...
```

## Task 8: Expose Backend Methods to Wails

**Files:**
- Modify: `app.go`
- Create: `internal/app/app.go`

- [ ] **Step 1: Create application service**

Create `internal/app/app.go`:

```go
package app

import (
	"context"

	farmruntime "Farm_Go/internal/runtime"
)

type Service struct {
	runtime *farmruntime.Manager
}

func NewService(runtime *farmruntime.Manager) *Service {
	return &Service{runtime: runtime}
}

func (s *Service) RuntimeStatus(ctx context.Context) farmruntime.Status {
	return s.runtime.Status()
}
```

- [ ] **Step 2: Bind service in root app**

Modify root `app.go` to expose:

```go
package main

import (
	"context"

	farmapp "Farm_Go/internal/app"
	farmruntime "Farm_Go/internal/runtime"
)

type App struct {
	ctx     context.Context
	service *farmapp.Service
}

func NewApp() *App {
	manager := farmruntime.NewManager()
	return &App{service: farmapp.NewService(manager)}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) RuntimeStatus() farmruntime.Status {
	return a.service.RuntimeStatus(a.ctx)
}
```

- [ ] **Step 3: Generate Wails bindings**

Run:

```powershell
wails generate module
```

Expected:

```text
Bindings generated
```

- [ ] **Step 4: Verify**

Run:

```powershell
wails dev
```

Expected: desktop window opens and backend compiles.

## Task 9: Build Desktop UI Shell

**Files:**
- Modify: `frontend/src/App.tsx`
- Create: `frontend/src/components/AppShell.tsx`
- Create: `frontend/src/components/StatusBadge.tsx`
- Create: `frontend/src/views/OverviewView.tsx`
- Create: `frontend/src/views/ConnectionView.tsx`
- Create: `frontend/src/views/DiagnosticsView.tsx`
- Create: `frontend/src/views/SettingsView.tsx`

- [ ] **Step 1: Create app shell**

Create `frontend/src/components/AppShell.tsx`:

```tsx
import { Activity, Cable, FileText, Settings } from "lucide-react";
import { ReactNode } from "react";

type Tab = "overview" | "connection" | "diagnostics" | "settings";

type AppShellProps = {
  activeTab: Tab;
  onTabChange: (tab: Tab) => void;
  children: ReactNode;
};

const nav = [
  { id: "overview" as const, label: "总览", icon: Activity },
  { id: "connection" as const, label: "连接", icon: Cable },
  { id: "diagnostics" as const, label: "调试", icon: FileText },
  { id: "settings" as const, label: "设置", icon: Settings },
];

export function AppShell({ activeTab, onTabChange, children }: AppShellProps) {
  return (
    <div className="grid h-screen grid-cols-[220px_1fr] bg-[#f5f7f2] text-[#17201a]">
      <aside className="border-r border-[#d9dfd2] bg-[#eef2e8] px-3 py-4">
        <div className="mb-6 px-2">
          <div className="text-lg font-semibold">Farm_Go</div>
          <div className="text-xs text-[#62705f]">QQ WS Diagnostics</div>
        </div>
        <nav className="space-y-1">
          {nav.map((item) => {
            const Icon = item.icon;
            const active = item.id === activeTab;
            return (
              <button
                key={item.id}
                onClick={() => onTabChange(item.id)}
                className={[
                  "flex h-10 w-full items-center gap-2 rounded-md px-3 text-left text-sm transition",
                  active
                    ? "bg-[#1f6f43] text-white shadow-sm"
                    : "text-[#344034] hover:bg-[#dfe7d8]",
                ].join(" ")}
              >
                <Icon size={17} />
                {item.label}
              </button>
            );
          })}
        </nav>
      </aside>
      <main className="min-w-0 overflow-hidden p-6">{children}</main>
    </div>
  );
}
```

- [ ] **Step 2: Create status badge**

Create `frontend/src/components/StatusBadge.tsx`:

```tsx
type StatusBadgeProps = {
  phase: string;
};

export function StatusBadge({ phase }: StatusBadgeProps) {
  const ready = phase === "ready";
  return (
    <span
      className={[
        "inline-flex h-7 items-center rounded-full px-3 text-xs font-medium",
        ready ? "bg-[#dff3df] text-[#17612e]" : "bg-[#edf0ea] text-[#5d6a5a]",
      ].join(" ")}
    >
      {phase}
    </span>
  );
}
```

- [ ] **Step 3: Create initial views**

Create `frontend/src/views/OverviewView.tsx`:

```tsx
import { StatusBadge } from "../components/StatusBadge";

type OverviewViewProps = {
  status: {
    phase: string;
    connected: boolean;
    ready: boolean;
    instanceId?: string;
    hostVersion?: string;
    lastError?: string;
  };
};

export function OverviewView({ status }: OverviewViewProps) {
  return (
    <section className="space-y-5">
      <header>
        <h1 className="text-2xl font-semibold">总览</h1>
        <p className="mt-1 text-sm text-[#62705f]">本机 QQ WS 运行时状态</p>
      </header>
      <div className="grid grid-cols-3 gap-4">
        <div className="rounded-md border border-[#d9dfd2] bg-white p-4">
          <div className="text-xs text-[#62705f]">状态</div>
          <div className="mt-3"><StatusBadge phase={status.phase} /></div>
        </div>
        <div className="rounded-md border border-[#d9dfd2] bg-white p-4">
          <div className="text-xs text-[#62705f]">实例</div>
          <div className="mt-3 truncate text-sm">{status.instanceId || "未连接"}</div>
        </div>
        <div className="rounded-md border border-[#d9dfd2] bg-white p-4">
          <div className="text-xs text-[#62705f]">Host 版本</div>
          <div className="mt-3 truncate text-sm">{status.hostVersion || "-"}</div>
        </div>
      </div>
    </section>
  );
}
```

Create placeholder views:

```tsx
export function ConnectionView() {
  return <section className="text-sm">连接面板</section>;
}
```

```tsx
export function DiagnosticsView() {
  return <section className="text-sm">调试面板</section>;
}
```

```tsx
export function SettingsView() {
  return <section className="text-sm">设置面板</section>;
}
```

- [ ] **Step 4: Wire `App.tsx`**

Modify `frontend/src/App.tsx`:

```tsx
import { useEffect, useState } from "react";
import { RuntimeStatus } from "../wailsjs/go/main/App";
import { AppShell } from "./components/AppShell";
import { ConnectionView } from "./views/ConnectionView";
import { DiagnosticsView } from "./views/DiagnosticsView";
import { OverviewView } from "./views/OverviewView";
import { SettingsView } from "./views/SettingsView";

type Tab = "overview" | "connection" | "diagnostics" | "settings";

type RuntimeStatusDto = {
  phase: string;
  connected: boolean;
  ready: boolean;
  instanceId?: string;
  hostVersion?: string;
  lastError?: string;
};

function App() {
  const [activeTab, setActiveTab] = useState<Tab>("overview");
  const [status, setStatus] = useState<RuntimeStatusDto>({
    phase: "idle",
    connected: false,
    ready: false,
  });

  useEffect(() => {
    RuntimeStatus().then(setStatus).catch(console.error);
  }, []);

  return (
    <AppShell activeTab={activeTab} onTabChange={setActiveTab}>
      {activeTab === "overview" && <OverviewView status={status} />}
      {activeTab === "connection" && <ConnectionView />}
      {activeTab === "diagnostics" && <DiagnosticsView />}
      {activeTab === "settings" && <SettingsView />}
    </AppShell>
  );
}

export default App;
```

- [ ] **Step 5: Verify window UI**

Run:

```powershell
wails dev
```

Expected: app opens with a sidebar and overview cards.

## Task 10: Add Diagnostics Backend

**Files:**
- Create: `internal/diagnostics/service.go`
- Modify: `internal/app/app.go`
- Modify: `app.go`

- [ ] **Step 1: Create diagnostics service**

Create `internal/diagnostics/service.go`:

```go
package diagnostics

import (
	"context"
	"errors"
	"time"
)

type Result struct {
	Method     string `json:"method"`
	OK         bool   `json:"ok"`
	DurationMS int64  `json:"durationMs"`
	Result     any    `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Call(ctx context.Context, method string, params map[string]any) Result {
	start := time.Now()
	if method == "" {
		return Result{
			Method:     method,
			OK:         false,
			DurationMS: time.Since(start).Milliseconds(),
			Error:      "method is required",
		}
	}

	if method != "host.describe" && method != "gameCtl.probe" {
		err := errors.New("unsupported diagnostic method")
		return Result{
			Method:     method,
			OK:         false,
			DurationMS: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		}
	}

	return Result{
		Method:     method,
		OK:         false,
		DurationMS: time.Since(start).Milliseconds(),
		Error:      "runtime is not connected",
	}
}
```

- [ ] **Step 2: Expose diagnostics through app service**

Modify `internal/app/app.go`:

```go
package app

import (
	"context"

	"Farm_Go/internal/diagnostics"
	farmruntime "Farm_Go/internal/runtime"
)

type Service struct {
	runtime     *farmruntime.Manager
	diagnostics *diagnostics.Service
}

func NewService(runtime *farmruntime.Manager) *Service {
	return &Service{
		runtime:     runtime,
		diagnostics: diagnostics.NewService(),
	}
}

func (s *Service) RuntimeStatus(ctx context.Context) farmruntime.Status {
	return s.runtime.Status()
}

func (s *Service) RunDiagnostic(ctx context.Context, method string, params map[string]any) diagnostics.Result {
	return s.diagnostics.Call(ctx, method, params)
}
```

- [ ] **Step 3: Expose Wails method**

Modify root `app.go`:

```go
func (a *App) RunDiagnostic(method string, params map[string]any) diagnostics.Result {
	return a.service.RunDiagnostic(a.ctx, method, params)
}
```

Add import:

```go
import "Farm_Go/internal/diagnostics"
```

- [ ] **Step 4: Regenerate bindings and verify**

Run:

```powershell
wails generate module
go test ./...
wails dev
```

Expected: Go tests pass and app opens.

## Task 11: Build Diagnostics UI

**Files:**
- Modify: `frontend/src/views/DiagnosticsView.tsx`

- [ ] **Step 1: Implement diagnostic form**

Modify `frontend/src/views/DiagnosticsView.tsx`:

```tsx
import { Play } from "lucide-react";
import { useState } from "react";
import { RunDiagnostic } from "../../wailsjs/go/main/App";

export function DiagnosticsView() {
  const [method, setMethod] = useState("host.describe");
  const [params, setParams] = useState("{}");
  const [result, setResult] = useState<string>("");
  const [error, setError] = useState<string>("");

  async function run() {
    setError("");
    setResult("");
    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(params);
    } catch {
      setError("参数不是合法 JSON");
      return;
    }

    const response = await RunDiagnostic(method, parsed);
    setResult(JSON.stringify(response, null, 2));
  }

  return (
    <section className="grid h-full grid-rows-[auto_1fr] gap-5">
      <header>
        <h1 className="text-2xl font-semibold">调试</h1>
        <p className="mt-1 text-sm text-[#62705f]">执行 QQ WS 诊断调用</p>
      </header>
      <div className="grid grid-cols-[420px_1fr] gap-4 overflow-hidden">
        <div className="rounded-md border border-[#d9dfd2] bg-white p-4">
          <label className="text-xs font-medium text-[#62705f]">方法</label>
          <select
            value={method}
            onChange={(event) => setMethod(event.target.value)}
            className="mt-2 h-10 w-full rounded-md border border-[#cbd5c4] px-3"
          >
            <option value="host.describe">host.describe</option>
            <option value="gameCtl.probe">gameCtl.probe</option>
          </select>
          <label className="mt-4 block text-xs font-medium text-[#62705f]">参数 JSON</label>
          <textarea
            value={params}
            onChange={(event) => setParams(event.target.value)}
            className="mt-2 h-48 w-full resize-none rounded-md border border-[#cbd5c4] p-3 font-mono text-sm"
          />
          {error && <div className="mt-3 text-sm text-[#b42318]">{error}</div>}
          <button
            onClick={run}
            className="mt-4 inline-flex h-10 items-center gap-2 rounded-md bg-[#1f6f43] px-4 text-sm font-medium text-white"
          >
            <Play size={16} />
            执行
          </button>
        </div>
        <pre className="overflow-auto rounded-md border border-[#d9dfd2] bg-[#17201a] p-4 text-sm text-[#e9f2e3]">
          {result || "等待执行"}
        </pre>
      </div>
    </section>
  );
}
```

- [ ] **Step 2: Verify invalid JSON handling**

Run:

```powershell
wails dev
```

Manual check:

- Open 调试.
- Enter `{bad`.
- Click 执行.
- Expected: UI shows `参数不是合法 JSON`.

- [ ] **Step 3: Verify backend call**

Manual check:

- Select `host.describe`.
- Enter `{}`.
- Click 执行.
- Expected: result JSON shows `runtime is not connected`.

## Task 12: Add Connection View

**Files:**
- Modify: `internal/app/app.go`
- Modify: `app.go`
- Modify: `frontend/src/views/ConnectionView.tsx`

- [ ] **Step 1: Add connection info DTO**

Modify `internal/app/app.go`:

```go
type ConnectionInfo struct {
	URL                 string `json:"url"`
	ExpectedHostVersion string `json:"expectedHostVersion"`
	TokenPreview        string `json:"tokenPreview"`
}

func (s *Service) ConnectionInfo(ctx context.Context) ConnectionInfo {
	return ConnectionInfo{
		URL:                 "ws://127.0.0.1:8787/runtime/qqws",
		ExpectedHostVersion: "farm-go-host-1",
		TokenPreview:        "not generated",
	}
}
```

- [ ] **Step 2: Expose through Wails**

Modify root `app.go`:

```go
func (a *App) ConnectionInfo() farmapp.ConnectionInfo {
	return a.service.ConnectionInfo(a.ctx)
}
```

- [ ] **Step 3: Regenerate bindings**

Run:

```powershell
wails generate module
```

Expected:

```text
Bindings generated
```

- [ ] **Step 4: Build connection view**

Modify `frontend/src/views/ConnectionView.tsx`:

```tsx
import { Copy } from "lucide-react";
import { useEffect, useState } from "react";
import { ConnectionInfo } from "../../wailsjs/go/main/App";

type Info = {
  url: string;
  expectedHostVersion: string;
  tokenPreview: string;
};

export function ConnectionView() {
  const [info, setInfo] = useState<Info | null>(null);

  useEffect(() => {
    ConnectionInfo().then(setInfo).catch(console.error);
  }, []);

  return (
    <section className="space-y-5">
      <header>
        <h1 className="text-2xl font-semibold">连接</h1>
        <p className="mt-1 text-sm text-[#62705f]">QQ host WebSocket 接入信息</p>
      </header>
      <div className="rounded-md border border-[#d9dfd2] bg-white p-4">
        <div className="text-xs text-[#62705f]">WebSocket 地址</div>
        <div className="mt-2 flex items-center gap-2">
          <code className="flex-1 rounded-md bg-[#eef2e8] px-3 py-2 text-sm">
            {info?.url || "loading"}
          </code>
          <button className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-[#cbd5c4]">
            <Copy size={16} />
          </button>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="rounded-md border border-[#d9dfd2] bg-white p-4">
          <div className="text-xs text-[#62705f]">Expected Host Version</div>
          <div className="mt-2 text-sm">{info?.expectedHostVersion || "-"}</div>
        </div>
        <div className="rounded-md border border-[#d9dfd2] bg-white p-4">
          <div className="text-xs text-[#62705f]">Token</div>
          <div className="mt-2 text-sm">{info?.tokenPreview || "-"}</div>
        </div>
      </div>
    </section>
  );
}
```

- [ ] **Step 5: Verify**

Run:

```powershell
wails dev
```

Expected: 连接 page shows WebSocket URL.

## Task 13: Add Build Script and Release Check

**Files:**
- Create: `scripts/build.ps1`
- Modify: `README.md`

- [ ] **Step 1: Create build script**

Create `scripts/build.ps1`:

```powershell
$ErrorActionPreference = "Stop"

Push-Location $PSScriptRoot\..
try {
  wails build -clean
} finally {
  Pop-Location
}
```

- [ ] **Step 2: Build release**

Run:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\build.ps1
```

Expected:

```text
Built target ...
```

- [ ] **Step 3: Smoke test release exe**

Run the generated exe under `build\bin`.

Manual checks:

- Window opens.
- Sidebar navigation works.
- Overview page renders.
- Connection page renders.
- Diagnostics page rejects invalid JSON.
- Diagnostics page returns `runtime is not connected` before QQ host connects.

## Task 14: Update Technical Decision Document

**Files:**
- Modify: `TECH_STACK.md`

- [ ] **Step 1: Replace outdated decisions**

Update `TECH_STACK.md` so it matches this plan:

- Replace browser WebUI phase 1 with Wails desktop UI.
- Replace Go `embed` WebUI strategy with Wails frontend packaging.
- Keep Go backend core.
- Keep SQLite.
- Keep QQ WS as first runtime.
- Remove "不做 Tauri 桌面壳" only if the line is still present; state "不使用 Tauri，使用 Wails".
- State that `tauri-controls` is not used because it depends on Tauri.

- [ ] **Step 2: Verify decision consistency**

Run:

```powershell
Select-String -LiteralPath TECH_STACK.md -Pattern "Tauri|Wails|WebUI|qqws|SQLite|单 exe"
```

Expected:

- `Wails` appears as the phase 1 desktop shell.
- `Tauri` only appears in a note explaining it is not used.
- `WebUI` does not appear as a phase 1 deliverable.
- `qqws` and `SQLite` remain first-phase decisions.

## Phase 1 Acceptance Checklist

- [ ] `wails dev` opens a desktop window.
- [ ] UI has 总览、连接、调试、设置 navigation.
- [ ] UI styling is modern enough for a product-like Windows utility.
- [ ] Go backend exposes runtime status through Wails bindings.
- [ ] SQLite database initializes successfully.
- [ ] QQ WS adapter can listen on `127.0.0.1:8787/runtime/qqws`.
- [ ] Connection page shows local connection details.
- [ ] Diagnostics page validates JSON input.
- [ ] Diagnostics backend returns structured success/failure results.
- [ ] Release build succeeds with `wails build -clean`.

## Self-Review

Spec coverage:

- Wails desktop direction is covered by Tasks 1, 2, 8, 9, 11, and 13.
- Beautiful UI requirement is covered by Tasks 2, 9, 11, and 12.
- QQ WS priority is covered by Tasks 6, 7, and the acceptance checklist.
- SQLite-first storage is covered by Task 4.
- No WebUI/Tauri phase 1 constraint is covered by Scope Decisions and Task 14.

Placeholder scan:

- No `TBD` or `TODO` placeholders remain.
- The plan intentionally keeps real QQ host business behavior out of phase 1.

Type consistency:

- Runtime status uses `phase`, `connected`, `ready`, `instanceId`, `hostVersion`, and `lastError` consistently in Go and React.
- Diagnostic method names are `host.describe` and `gameCtl.probe`.

