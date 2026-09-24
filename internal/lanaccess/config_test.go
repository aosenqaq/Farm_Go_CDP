package lanaccess

import (
	"net"
	"testing"
)

func TestConfigValidateRejectsUnsafeValues(t *testing.T) {
	for _, cfg := range []Config{
		{Enabled: true, Mode: ModeLAN, Port: 1023, PasswordHash: "hash"},
		{Enabled: true, Mode: ModeLAN, Port: 65536, PasswordHash: "hash"},
		{Enabled: true, Mode: "public", Port: 8788, PasswordHash: "hash"},
		{Enabled: true, Mode: ModeTunnel, Port: 8788},
	} {
		if err := cfg.Validate(); err == nil {
			t.Fatalf("Validate() accepted %#v", cfg)
		}
	}
}

func TestConfigValidateAcceptsDisabledConfigAndSafeEnabledConfig(t *testing.T) {
	for _, cfg := range []Config{
		{Enabled: false},
		{Enabled: true, Mode: ModeLAN, Port: 8788, PasswordHash: "hash"},
		{Enabled: true, Mode: ModeTunnel, Port: 65535, PasswordHash: "hash"},
	} {
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate(%#v) error = %v", cfg, err)
		}
	}
}

func TestConfigListenAddrFollowsMode(t *testing.T) {
	tests := []struct {
		config Config
		want   string
	}{
		{config: Config{Mode: ModeLAN, Port: 8788}, want: net.JoinHostPort("0.0.0.0", "8788")},
		{config: Config{Mode: ModeTunnel, Port: 8788}, want: net.JoinHostPort("127.0.0.1", "8788")},
	}

	for _, tt := range tests {
		if got := tt.config.ListenAddr(); got != tt.want {
			t.Errorf("ListenAddr() = %q, want %q", got, tt.want)
		}
	}
}

func TestAllowsRemoteAddrUsesModeBoundary(t *testing.T) {
	for _, remote := range []string{
		"127.0.0.1:1000",
		"10.1.2.3:1000",
		"172.16.1.1:1000",
		"192.168.1.4:1000",
	} {
		if !AllowsRemoteAddr(ModeLAN, remote) {
			t.Errorf("LAN denied %q", remote)
		}
	}

	for _, remote := range []string{
		"8.8.8.8:1000",
		"172.15.255.255:1000",
		"172.32.0.1:1000",
		"[::1]:1000",
		"not-an-address:1000",
		"192.168.1.4",
	} {
		if AllowsRemoteAddr(ModeLAN, remote) {
			t.Errorf("LAN allowed unsafe source %q", remote)
		}
	}

	if !AllowsRemoteAddr(ModeTunnel, "127.0.0.1:1000") {
		t.Fatal("tunnel denied IPv4 loopback")
	}
	for _, remote := range []string{"192.168.1.4:1000", "[::1]:1000", "127.0.0.1"} {
		if AllowsRemoteAddr(ModeTunnel, remote) {
			t.Errorf("tunnel allowed unsafe source %q", remote)
		}
	}
}
