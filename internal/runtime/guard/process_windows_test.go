//go:build windows

package guard

import (
	"errors"
	"os"
	"os/exec"
	"reflect"
	"testing"
)

func TestCloseHostWindowsPostsCloseOnlyToVisibleHandles(t *testing.T) {
	original := postWindowClose
	t.Cleanup(func() { postWindowClose = original })

	var closed []uint64
	postWindowClose = func(hwnd uint64) error {
		closed = append(closed, hwnd)
		return nil
	}

	err := CloseHostWindows([]HostWindowSnapshot{
		{HWND: 10, Visible: true},
		{HWND: 0, Visible: true},
		{HWND: 11, Visible: false},
		{HWND: 12, Visible: true},
	})
	if err != nil {
		t.Fatalf("close windows: %v", err)
	}
	want := []uint64{10, 12}
	if !reflect.DeepEqual(closed, want) {
		t.Fatalf("closed = %#v, want %#v", closed, want)
	}
}

func TestCloseHostWindowsStopsAfterPostError(t *testing.T) {
	original := postWindowClose
	t.Cleanup(func() { postWindowClose = original })

	wantErr := errors.New("post failed")
	var closed []uint64
	postWindowClose = func(hwnd uint64) error {
		closed = append(closed, hwnd)
		return wantErr
	}

	err := CloseHostWindows([]HostWindowSnapshot{{HWND: 10, Visible: true}, {HWND: 12, Visible: true}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if !reflect.DeepEqual(closed, []uint64{10}) {
		t.Fatalf("closed = %#v, want first handle only", closed)
	}
}

func TestEnumerateWindowsByPIDDoesNotExhaustCallbacks(t *testing.T) {
	const helperEnv = "FARM_GO_ENUM_WINDOWS_CALLBACK_TEST"
	if os.Getenv(helperEnv) == "1" {
		for range 5000 {
			enumerateWindowsByPID()
		}
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestEnumerateWindowsByPIDDoesNotExhaustCallbacks$")
	cmd.Env = append(os.Environ(), helperEnv+"=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("repeated window enumeration terminated the process: %v\n%s", err, output)
	}
}
