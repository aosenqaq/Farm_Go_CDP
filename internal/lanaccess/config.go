package lanaccess

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
)

type Mode string

const (
	ModeLAN    Mode = "lan"
	ModeTunnel Mode = "tunnel"
)

type Config struct {
	Enabled      bool
	Mode         Mode
	Port         int
	PasswordHash string
}

func (c Config) ListenAddr() string {
	host := "0.0.0.0"
	if c.Mode == ModeTunnel {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(c.Port))
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Mode != ModeLAN && c.Mode != ModeTunnel {
		return fmt.Errorf("unsupported LAN access mode %q", c.Mode)
	}
	if c.Port < 1024 || c.Port > 65535 {
		return fmt.Errorf("LAN access port %d is outside 1024-65535", c.Port)
	}
	if c.PasswordHash == "" {
		return errors.New("LAN access password is required")
	}
	return nil
}

func AllowsRemoteAddr(mode Mode, remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false
	}

	addr, err := netip.ParseAddr(host)
	if err != nil || addr.Is6() {
		return false
	}
	if mode == ModeTunnel {
		return addr.IsLoopback()
	}
	return mode == ModeLAN && (addr.IsLoopback() || addr.IsPrivate())
}
