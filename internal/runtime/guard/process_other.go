//go:build !windows

package guard

import "errors"

func ListHostProcessSnapshots() ([]HostProcessSnapshot, error) {
	return nil, errors.New("unsupported_platform")
}

func StopHostPID(pid int) error {
	return errors.New("unsupported_platform")
}

func LaunchHost(request LaunchRequest) error {
	return errors.New("unsupported_platform")
}

func CloseHostWindows([]HostWindowSnapshot) error {
	return errors.New("unsupported_platform")
}

func MinimizeHostWindows([]HostWindowSnapshot) error {
	return errors.New("unsupported_platform")
}
