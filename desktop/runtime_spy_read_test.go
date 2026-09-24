package desktop

import (
	"strings"
	"testing"
)

func runtimeButtonFunctionBody(t *testing.T, name string) string {
	t.Helper()
	marker := "function " + name + "("
	start := strings.Index(runtimeButtonScript, marker)
	if start < 0 {
		t.Fatalf("function %s not found in runtime button script", name)
	}
	open := strings.Index(runtimeButtonScript[start:], "{")
	if open < 0 {
		t.Fatalf("function %s has no body", name)
	}
	open += start
	depth := 0
	for index := open; index < len(runtimeButtonScript); index++ {
		switch runtimeButtonScript[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return runtimeButtonScript[open+1 : index]
			}
		}
	}
	t.Fatalf("function %s body is not closed", name)
	return ""
}

func TestRuntimeButtonReadOnlyProfileFunctionsDoNotInstallSpies(t *testing.T) {
	for _, name := range []string{"getPlayerProfile", "getProtocolAccountProfile"} {
		if body := runtimeButtonFunctionBody(t, name); strings.Contains(body, "installRuntimeSpies()") {
			t.Fatalf("read-only function %s must not install runtime spies", name)
		}
	}
}

func TestRuntimeButtonExplicitSpyStartStillInstallsSpies(t *testing.T) {
	if body := runtimeButtonFunctionBody(t, "startRuntimeSpies"); !strings.Contains(body, "installRuntimeSpies()") {
		t.Fatal("explicit Spy start must retain the installer call")
	}
}

func TestRuntimeButtonNativeFriendSpyCapturesQQPlatformCalls(t *testing.T) {
	for _, marker := range []string{
		"function installNativeFriendApiSpies()",
		"function wrapNativeFriendApiMethod(",
		"apiKind: 'nativeFriend'",
		"G.qq",
		"G.GameGlobal && safeReadKey(G.GameGlobal, 'qq')",
		"G.BK",
		"safeReadKey(G.BK, 'QQ')",
		"safeReadKey(safeReadKey(G.GameGlobal, 'BK'), 'QQ')",
		"/friend|add|profile|card|open|url|launch|scheme|contact/i",
	} {
		if !strings.Contains(runtimeButtonScript, marker) {
			t.Fatalf("native friend spy marker missing: %s", marker)
		}
	}

	if body := runtimeButtonFunctionBody(t, "installRuntimeSpies"); !strings.Contains(body, "installNativeFriendApiSpies()") {
		t.Fatal("runtime spy installer must install native friend API spies")
	}
}

func TestRuntimeButtonNativeFriendSpyPreservesTargetFields(t *testing.T) {
	for _, marker := range []string{
		"function summarizeNativeFriendArgument(value)",
		"current !== undefined",
		"'openId'",
		"'verifyMsg'",
		"'url'",
		"'uri'",
		"'scheme'",
		"'uin'",
		"'gid'",
		"'userId'",
	} {
		if !strings.Contains(runtimeButtonScript, marker) {
			t.Fatalf("native friend argument summary marker missing: %s", marker)
		}
	}

	for _, name := range []string{"wrapNativeFriendCallbackOption", "wrapNativeFriendApiMethod"} {
		if body := runtimeButtonFunctionBody(t, name); !strings.Contains(body, "summarizeNativeFriendArgument(arg)") {
			t.Fatalf("native friend wrapper %s must use the target-field argument summary", name)
		}
	}
}
