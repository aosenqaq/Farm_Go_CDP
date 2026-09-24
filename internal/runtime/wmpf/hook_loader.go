package wmpf

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	farmruntime "Farm_Go/internal/runtime"
)

type RuntimeProcess struct {
	Name        string
	PID         int
	ParentPID   int
	Path        string
	CommandLine string
	Match       ProcessMatch
}

type ProcessFinder interface {
	List(profile Profile) ([]RuntimeProcess, error)
}

type FridaHookRunner interface {
	Start(ctx context.Context, pid int, hook FridaHookOptions) error
}

type FridaHookLoaderOptions struct {
	Profile           Profile
	Root              string
	Resources         fs.FS
	Python            string
	DebugWebSocketURL string
	Finder            ProcessFinder
	Runner            FridaHookRunner
	Interval          time.Duration
}

type FridaHookLoader struct {
	profile           Profile
	root              string
	resources         fs.FS
	debugWebSocketURL string
	finder            ProcessFinder
	runner            FridaHookRunner
	interval          time.Duration

	mu         sync.Mutex
	attached   map[int]bool
	watcherRun bool
}

func NewFridaHookLoader(options FridaHookLoaderOptions) *FridaHookLoader {
	if options.Finder == nil {
		options.Finder = PowerShellProcessFinder{}
	}
	if options.Runner == nil {
		options.Runner = PythonFridaHookRunner{Python: options.Python}
	}
	if options.Interval <= 0 {
		options.Interval = 2 * time.Second
	}
	return &FridaHookLoader{
		profile:           options.Profile,
		root:              options.Root,
		resources:         options.Resources,
		debugWebSocketURL: options.DebugWebSocketURL,
		finder:            options.Finder,
		runner:            options.Runner,
		interval:          options.Interval,
		attached:          map[int]bool{},
	}
}

func (l *FridaHookLoader) Start(ctx context.Context) error {
	if l.root == "" {
		return nil
	}
	if err := l.attachOnce(ctx); err != nil {
		return err
	}

	l.mu.Lock()
	if l.watcherRun {
		l.mu.Unlock()
		return nil
	}
	l.watcherRun = true
	l.mu.Unlock()

	go l.watch(ctx)
	return nil
}

func (l *FridaHookLoader) watch(ctx context.Context) {
	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = l.attachOnce(ctx)
		}
	}
}

func (l *FridaHookLoader) attachOnce(ctx context.Context) error {
	processes, err := l.finder.List(l.profile)
	if err != nil {
		return err
	}
	if len(processes) == 0 {
		return nil
	}
	var lastErr error
	attachedAny := false
	for _, process := range selectRuntimeProcesses(l.profile, processes) {
		if process.PID == 0 || process.Match.Version == "" {
			continue
		}

		l.mu.Lock()
		if l.attached[process.PID] {
			l.mu.Unlock()
			continue
		}
		l.attached[process.PID] = true
		l.mu.Unlock()

		if err := l.runner.Start(ctx, process.PID, FridaHookOptions{
			Root:              l.root,
			Resources:         l.resources,
			Target:            l.profile.Target,
			Version:           process.Match.Version,
			DebugWebSocketURL: l.debugWebSocketURL,
		}); err != nil {
			l.mu.Lock()
			delete(l.attached, process.PID)
			l.mu.Unlock()
			lastErr = err
			continue
		}
		attachedAny = true
	}
	if !attachedAny && lastErr != nil {
		return lastErr
	}
	return nil
}

func selectRuntimeProcesses(profile Profile, processes []RuntimeProcess) []RuntimeProcess {
	selected, ok := selectWMPFProcess(profile, processes)
	if !ok {
		return nil
	}
	return []RuntimeProcess{selected}
}

func selectWMPFProcess(profile Profile, processes []RuntimeProcess) (RuntimeProcess, bool) {
	namedProcesses := make([]RuntimeProcess, 0, len(processes))
	for _, process := range processes {
		if process.Name == "WeChatAppEx.exe" || process.Name == "" {
			namedProcesses = append(namedProcesses, process)
		}
	}

	matchedChildren := make([]RuntimeProcess, 0, len(namedProcesses))
	for _, process := range namedProcesses {
		if matchesRuntimeTarget(profile, process) {
			matchedChildren = append(matchedChildren, process)
		}
	}
	if len(matchedChildren) == 0 {
		return RuntimeProcess{}, false
	}

	if profile.Target == farmruntime.RuntimeTargetYYBCDP {
		return selectYYBWMPFProcess(processes, matchedChildren)
	}
	return selectWeChatWMPFProcess(processes, matchedChildren)
}

func selectWeChatWMPFProcess(processes []RuntimeProcess, matchedChildren []RuntimeProcess) (RuntimeProcess, bool) {
	hostPID, ok := mostFrequentParentPID(matchedChildren)
	if !ok {
		return RuntimeProcess{}, false
	}
	attachProcess, ok := findProcessByPID(processes, hostPID)
	if !ok {
		attachProcess, ok = findProcessByPID(matchedChildren, hostPID)
	}
	if !ok {
		return RuntimeProcess{}, false
	}

	versionSource := attachProcess
	for _, process := range matchedChildren {
		if process.ParentPID == hostPID && process.Match.Version != "" {
			versionSource = process
			break
		}
	}
	if attachProcess.Match.Version == "" {
		attachProcess.Match = versionSource.Match
	}
	return attachProcess, attachProcess.Match.Version != ""
}

func selectYYBWMPFProcess(processes []RuntimeProcess, matchedChildren []RuntimeProcess) (RuntimeProcess, bool) {
	parentPID, ok := mostFrequentParentPID(matchedChildren)
	if !ok {
		return RuntimeProcess{}, false
	}
	if parentProcess, ok := findProcessByPID(matchedChildren, parentPID); ok {
		return parentProcess, parentProcess.Match.Version != ""
	}

	parentCounts := parentPIDCounts(matchedChildren)
	candidates := append([]RuntimeProcess(nil), matchedChildren...)
	sort.SliceStable(candidates, func(i, j int) bool {
		return parentCounts[candidates[i].PID] > parentCounts[candidates[j].PID]
	})
	if len(candidates) == 0 || candidates[0].Match.Version == "" {
		return RuntimeProcess{}, false
	}
	return candidates[0], true
}

func matchesRuntimeTarget(profile Profile, process RuntimeProcess) bool {
	if process.Name != "" && process.Name != "WeChatAppEx.exe" {
		return false
	}
	switch profile.Target {
	case farmruntime.RuntimeTargetYYBCDP:
		return isYYBWMPFPath(process.Path)
	default:
		return isWeChatWMPFPath(process.Path) || !isYYBWMPFPath(process.Path)
	}
}

func mostFrequentParentPID(processes []RuntimeProcess) (int, bool) {
	counts := parentPIDCounts(processes)
	knownPIDs := map[int]bool{}
	for _, process := range processes {
		if process.PID != 0 {
			knownPIDs[process.PID] = true
		}
	}
	bestPID := 0
	bestCount := 0
	for pid, count := range counts {
		if count > bestCount || (count == bestCount && knownPIDs[pid] && !knownPIDs[bestPID]) {
			bestPID = pid
			bestCount = count
		}
	}
	return bestPID, bestPID != 0
}

func parentPIDCounts(processes []RuntimeProcess) map[int]int {
	counts := map[int]int{}
	for _, process := range processes {
		if process.ParentPID != 0 {
			counts[process.ParentPID]++
		}
	}
	return counts
}

func findProcessByPID(processes []RuntimeProcess, pid int) (RuntimeProcess, bool) {
	for _, process := range processes {
		if process.PID == pid {
			return process, true
		}
	}
	return RuntimeProcess{}, false
}

func isYYBWMPFPath(path string) bool {
	_, ok := extractYYBVersion(normalizePath(path))
	return ok
}

func isWeChatWMPFPath(path string) bool {
	_, ok := extractWeChatVersion(normalizePath(path))
	return ok
}

type PowerShellProcessFinder struct{}

type cimProcess struct {
	Name            string `json:"Name"`
	ProcessID       int    `json:"ProcessId"`
	ParentProcessID int    `json:"ParentProcessId"`
	ExecutablePath  string `json:"ExecutablePath"`
	CommandLine     string `json:"CommandLine"`
}

func (PowerShellProcessFinder) List(profile Profile) ([]RuntimeProcess, error) {
	query := `Get-CimInstance Win32_Process | Where-Object { $_.Name -in @('WeChatAppEx.exe','WMPFRuntime.exe') } | Select-Object Name,ProcessId,ParentProcessId,ExecutablePath,CommandLine | ConvertTo-Json -Depth 2`
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", query)
	applyHiddenWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return nil, nil
	}

	var raw []cimProcess
	if strings.HasPrefix(text, "[") {
		if err := json.Unmarshal([]byte(text), &raw); err != nil {
			return nil, err
		}
	} else {
		var single cimProcess
		if err := json.Unmarshal([]byte(text), &single); err != nil {
			return nil, err
		}
		raw = append(raw, single)
	}

	processes := make([]RuntimeProcess, 0, len(raw))
	for _, item := range raw {
		match, _ := profile.MatchProcessPath(item.ExecutablePath)
		processes = append(processes, RuntimeProcess{
			Name:        item.Name,
			PID:         item.ProcessID,
			ParentPID:   item.ParentProcessID,
			Path:        item.ExecutablePath,
			CommandLine: item.CommandLine,
			Match:       match,
		})
	}
	return processes, nil
}

type PythonFridaHookRunner struct {
	Python string
}

func (r PythonFridaHookRunner) Start(ctx context.Context, pid int, hook FridaHookOptions) error {
	source, err := BuildFridaHookScript(hook)
	if err != nil {
		return err
	}
	python := r.Python
	if python == "" {
		python = "python"
	}

	cmd := newPythonFridaCommand(ctx, python, pid, source)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	ready := make(chan error, 1)
	go waitForFridaReady(stdout, stderr, ready)
	go func() {
		_ = cmd.Wait()
	}()

	select {
	case err := <-ready:
		return err
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		return errors.New("timed out waiting for frida hook to load")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func newPythonFridaCommand(ctx context.Context, python string, pid int, source string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, python, "-c", pythonFridaHelper)
	applyHiddenWindow(cmd)
	cmd.Env = append(cmd.Environ(),
		fmt.Sprintf("FARM_GO_FRIDA_PID=%d", pid),
		fmt.Sprintf("FARM_GO_FRIDA_PARENT_PID=%d", os.Getpid()),
		"FARM_GO_FRIDA_HOOK_B64="+base64.StdEncoding.EncodeToString([]byte(source)),
		"FARM_GO_FRIDA_LOG="+fridaLogPath(pid),
	)
	return cmd
}

func waitForFridaReady(stdout io.Reader, stderr io.Reader, ready chan<- error) {
	errs := make(chan string, 1)
	go func() {
		raw, _ := io.ReadAll(stderr)
		errs <- strings.TrimSpace(string(raw))
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "FRIDA_LOADED") {
			ready <- nil
			return
		}
		if strings.HasPrefix(line, "FRIDA_ERROR") {
			ready <- errors.New(strings.TrimSpace(strings.TrimPrefix(line, "FRIDA_ERROR")))
			return
		}
	}
	if err := scanner.Err(); err != nil {
		ready <- err
		return
	}
	if text := <-errs; text != "" {
		ready <- errors.New(text)
		return
	}
	ready <- errors.New("frida helper exited before loading hook")
}

func fridaLogPath(pid int) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("farm_go_frida_%d.log", pid))
}

const pythonFridaHelper = `
import base64
import os
import sys
import time

log_path = os.environ.get("FARM_GO_FRIDA_LOG", "")
def log(line):
    if not log_path:
        return
    with open(log_path, "a", encoding="utf-8") as file:
        file.write(str(line) + "\n")

def open_parent_handle(parent_pid):
    if parent_pid <= 0 or os.name != "nt":
        return None
    try:
        import ctypes
        kernel32 = ctypes.windll.kernel32
        kernel32.OpenProcess.restype = ctypes.c_void_p
        process_query_limited_information = 0x1000
        synchronize = 0x00100000
        return kernel32.OpenProcess(
            process_query_limited_information | synchronize,
            False,
            parent_pid,
        )
    except Exception as exc:
        log("FRIDA_PARENT_MONITOR_UNAVAILABLE " + str(exc))
        return None

def parent_has_exited(parent_pid, parent_handle):
    if parent_pid <= 0:
        return False
    if os.name == "nt" and parent_handle:
        try:
            import ctypes
            wait_object_0 = 0x00000000
            status = ctypes.windll.kernel32.WaitForSingleObject(parent_handle, 0)
            return status == wait_object_0
        except Exception as exc:
            log("FRIDA_PARENT_MONITOR_ERROR " + str(exc))
            return False
    try:
        os.kill(parent_pid, 0)
        return False
    except OSError:
        return True

def close_parent_handle(parent_handle):
    if os.name != "nt" or not parent_handle:
        return
    try:
        import ctypes
        ctypes.windll.kernel32.CloseHandle(parent_handle)
    except Exception:
        pass

try:
    parent_pid = int(os.environ.get("FARM_GO_FRIDA_PARENT_PID", "0") or "0")
    parent_handle = open_parent_handle(parent_pid)
    import frida
    pid = int(os.environ["FARM_GO_FRIDA_PID"])
    source = base64.b64decode(os.environ["FARM_GO_FRIDA_HOOK_B64"]).decode("utf-8")
    session = frida.attach(pid)
    script = session.create_script(source)
    script.on("message", lambda message, data: log("FRIDA_MESSAGE " + str(message)))
    script.load()
    log("FRIDA_LOADED " + str(pid))
    print("FRIDA_LOADED " + str(pid), flush=True)
    while True:
        if parent_has_exited(parent_pid, parent_handle):
            log("FRIDA_PARENT_EXITED " + str(parent_pid))
            try:
                session.detach()
            except Exception as exc:
                log("FRIDA_DETACH_ERROR " + str(exc))
            close_parent_handle(parent_handle)
            sys.exit(0)
        time.sleep(1)
except Exception as exc:
    log("FRIDA_ERROR " + str(exc))
    print("FRIDA_ERROR " + str(exc), flush=True)
    sys.exit(1)
`
