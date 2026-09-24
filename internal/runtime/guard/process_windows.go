//go:build windows

package guard

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const swShownormal = 1

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procGetWindowTextW      = user32.NewProc("GetWindowTextW")
	procGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procShowWindow          = user32.NewProc("ShowWindow")
	enumerateWindowsMu      sync.Mutex
	enumerateWindowsResult  map[int][]HostWindowSnapshot
	enumWindowsCallback     = windows.NewCallback(collectWindowSnapshot)
)

func ListHostProcessSnapshots() ([]HostProcessSnapshot, error) {
	processes, err := enumerateProcesses()
	if err != nil {
		return nil, err
	}
	windowsByPID := enumerateWindowsByPID()
	result := make([]HostProcessSnapshot, 0, len(processes))
	for _, snapshot := range processes {
		snapshot.Windows = windowsByPID[snapshot.PID]
		result = append(result, snapshot)
	}
	return result, nil
}

func StopHostPID(pid int) error {
	if pid <= 0 {
		return errors.New("pid is required")
	}
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	if err := windows.TerminateProcess(handle, 0); err != nil {
		return err
	}
	_, _ = windows.WaitForSingleObject(handle, 5000)
	return nil
}

func LaunchHost(request LaunchRequest) error {
	switch request.Mode {
	case "protocol":
		if strings.TrimSpace(request.Protocol) == "" {
			return errors.New("launch protocol is required")
		}
		return shellExecute(request.Protocol, "", "")
	case "yyb_shortcut":
		target := strings.TrimSpace(request.TargetDisplayName)
		if target == "" {
			target = yybTargetDisplayName
		}
		return shellExecute(target, request.Parameters, request.WorkingDirectory)
	default:
		return fmt.Errorf("unsupported_launch_mode: %s", request.Mode)
	}
}

func enumerateProcesses() (map[int]HostProcessSnapshot, error) {
	handle, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	if handle == windows.InvalidHandle {
		return nil, errors.New("CreateToolhelp32Snapshot returned invalid handle")
	}
	defer windows.CloseHandle(handle)

	processes := map[int]HostProcessSnapshot{}
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(handle, &entry); err != nil {
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return processes, nil
		}
		return nil, err
	}
	for {
		pid := int(entry.ProcessID)
		processes[pid] = HostProcessSnapshot{
			PID:            pid,
			ParentPID:      int(entry.ParentProcessID),
			ProcessName:    windows.UTF16ToString(entry.ExeFile[:]),
			ExecutablePath: queryProcessPath(pid),
		}
		entry.Size = uint32(unsafe.Sizeof(entry))
		if err := windows.Process32Next(handle, &entry); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				break
			}
			return nil, err
		}
	}
	return processes, nil
}

func queryProcessPath(pid int) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:size])
}

func enumerateWindowsByPID() map[int][]HostWindowSnapshot {
	enumerateWindowsMu.Lock()
	defer enumerateWindowsMu.Unlock()

	result := map[int][]HostWindowSnapshot{}
	enumerateWindowsResult = result
	defer func() { enumerateWindowsResult = nil }()

	_ = windows.EnumWindows(enumWindowsCallback, nil)
	return result
}

func collectWindowSnapshot(hwnd windows.HWND, _ uintptr) uintptr {
	var pid uint32
	_, _ = windows.GetWindowThreadProcessId(hwnd, &pid)
	if pid == 0 {
		return 1
	}
	title := windowText(hwnd)
	if strings.TrimSpace(title) == "" {
		return 1
	}
	enumerateWindowsResult[int(pid)] = append(enumerateWindowsResult[int(pid)], HostWindowSnapshot{
		HWND:    uint64(hwnd),
		PID:     int(pid),
		Title:   title,
		Visible: windows.IsWindowVisible(hwnd),
	})
	return 1
}

func windowText(hwnd windows.HWND) string {
	length, _, _ := procGetWindowTextLength.Call(uintptr(hwnd))
	if length == 0 {
		return ""
	}
	buf := make([]uint16, int(length)+1)
	procGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf)
}

func shellExecute(file string, args string, cwd string) error {
	filePtr, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	var argsPtr *uint16
	if strings.TrimSpace(args) != "" {
		argsPtr, err = windows.UTF16PtrFromString(args)
		if err != nil {
			return err
		}
	}
	var cwdPtr *uint16
	if strings.TrimSpace(cwd) != "" {
		cwdPtr, err = windows.UTF16PtrFromString(cwd)
		if err != nil {
			return err
		}
	}
	return windows.ShellExecute(0, nil, filePtr, argsPtr, cwdPtr, swShownormal)
}

const swMinimize = 6

const wmClose = 0x0010

var postWindowClose = func(hwnd uint64) error {
	result, _, callErr := procPostMessageW.Call(uintptr(hwnd), uintptr(wmClose), 0, 0)
	if result == 0 {
		return fmt.Errorf("PostMessageW WM_CLOSE failed for hwnd %d: %w", hwnd, callErr)
	}
	return nil
}

func CloseHostWindows(snapshots []HostWindowSnapshot) error {
	handles := minimizableWindowHandles(snapshots)
	if len(handles) == 0 {
		return errors.New("visible host window handle is required")
	}
	for _, handle := range handles {
		if err := postWindowClose(handle); err != nil {
			return err
		}
	}
	return nil
}

func MinimizeHostWindows(snapshots []HostWindowSnapshot) error {
	for _, handle := range minimizableWindowHandles(snapshots) {
		procShowWindow.Call(uintptr(handle), uintptr(swMinimize))
	}
	return nil
}
