package guard

import (
	"reflect"
	"testing"
)

func TestMinimizableWindowHandlesUsesOnlyVisibleWindowsWithHandles(t *testing.T) {
	got := minimizableWindowHandles([]HostWindowSnapshot{
		{HWND: 10, Visible: true},
		{HWND: 0, Visible: true},
		{HWND: 11, Visible: false},
		{HWND: 12, Visible: true},
	})
	want := []uint64{10, 12}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("handles = %#v, want %#v", got, want)
	}
}
