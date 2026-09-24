package wmpf

import (
	"testing"

	farmruntime "Farm_Go/internal/runtime"
)

func TestWeChatProfileAcceptsRadiumWMPFPath(t *testing.T) {
	profile := ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP)
	path := `C:\Users\tester\AppData\Roaming\Tencent\xwechat\xplugin\plugins\RadiumWMPF\6543\extracted\runtime\WeChatAppEx.exe`

	match, ok := profile.MatchProcessPath(path)

	if !ok {
		t.Fatalf("expected WeChat path to match")
	}
	if match.Version != "6543" {
		t.Fatalf("unexpected wechat version %#v", match)
	}
}

func TestWeChatProfileRejectsYYBPath(t *testing.T) {
	profile := ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP)
	path := `C:\Program Files\Tencent\Androws\WmpfRuntime\5.10.2700.327\runtime\WMPFRuntime.exe`

	if _, ok := profile.MatchProcessPath(path); ok {
		t.Fatalf("wechat profile should reject yyb path")
	}
}

func TestYYBProfileAcceptsRuntimePathAndVersion(t *testing.T) {
	profile := ProfileForTarget(farmruntime.RuntimeTargetYYBCDP)
	path := `C:\Program Files\Tencent\Androws\WmpfRuntime\5.10.2700.327\runtime\WMPFRuntime.exe`

	match, ok := profile.MatchProcessPath(path)

	if !ok {
		t.Fatalf("expected YYB path to match")
	}
	if match.Version != "5.10.2700.327" {
		t.Fatalf("unexpected yyb version %#v", match)
	}
}

func TestYYBProfileAcceptsRootAndrowsRuntimePath(t *testing.T) {
	profile := ProfileForTarget(farmruntime.RuntimeTargetYYBCDP)
	path := `D:\Androws\WmpfRuntime\5.10.2700.327\runtime\WeChatAppEx.exe`

	match, ok := profile.MatchProcessPath(path)

	if !ok {
		t.Fatalf("expected root-level Androws YYB path to match")
	}
	if match.Version != "5.10.2700.327" {
		t.Fatalf("unexpected yyb version %#v", match)
	}
}

func TestYYBProfileRejectsWeChatPath(t *testing.T) {
	profile := ProfileForTarget(farmruntime.RuntimeTargetYYBCDP)
	path := `C:\Users\tester\AppData\Roaming\Tencent\xwechat\xplugin\plugins\RadiumWMPF\6543\extracted\runtime\WeChatAppEx.exe`

	if _, ok := profile.MatchProcessPath(path); ok {
		t.Fatalf("yyb profile should reject wechat path")
	}
}
