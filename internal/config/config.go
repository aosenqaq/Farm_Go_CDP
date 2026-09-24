package config

type Config struct {
	QQWS    QQWSConfig    `json:"qqws"`
	Runtime RuntimeConfig `json:"runtime"`
	CDP     CDPConfig     `json:"cdp"`
	WMPF    WMPFConfig    `json:"wmpf"`
	UI      UIConfig      `json:"ui"`
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

type RuntimeConfig struct {
	DefaultTarget string `json:"defaultTarget"`
	CurrentTarget string `json:"currentTarget"`
	AutoStart     bool   `json:"autoStart"`
}

type CDPConfig struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	TimeoutMS   int    `json:"timeoutMs"`
	ContextName string `json:"contextName"`
}

type WMPFConfig struct {
	DebugPort       int    `json:"debugPort"`
	LegacyDebugPort int    `json:"legacyDebugPort"`
	ConfigDir       string `json:"configDir"`
	YYBConfigDir    string `json:"yybConfigDir"`
	FridaEnabled    bool   `json:"fridaEnabled"`
}

func Default() Config {
	return Config{
		QQWS: QQWSConfig{
			Host:                "127.0.0.1",
			Port:                8787,
			Path:                "/runtime/qqws",
			ExpectedHostVersion: "farm-go-host-1",
		},
		Runtime: RuntimeConfig{
			DefaultTarget: "qq_ws",
			CurrentTarget: "qq_ws",
			AutoStart:     true,
		},
		CDP: CDPConfig{
			Host:        "127.0.0.1",
			Port:        62000,
			TimeoutMS:   8000,
			ContextName: "gameContext",
		},
		WMPF: WMPFConfig{
			DebugPort:       9420,
			LegacyDebugPort: 9421,
			ConfigDir:       "resources/wmpf/frida/config",
			YYBConfigDir:    "resources/wmpf/frida/config/yyb",
			FridaEnabled:    true,
		},
		UI: UIConfig{Theme: "system"},
	}
}
