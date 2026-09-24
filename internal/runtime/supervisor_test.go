package runtime

import (
	"context"
	"testing"
	"time"
)

type fakeLink struct {
	target           RuntimeTarget
	startCount       int
	stopCount        int
	stopAndWaitCount int
	status           Status
	calls            []string
}

func newFakeLink(target RuntimeTarget) *fakeLink {
	return &fakeLink{
		target: target,
		status: Status{Target: string(target), Phase: PhaseIdle},
	}
}

func (f *fakeLink) Target() RuntimeTarget {
	return f.target
}

func (f *fakeLink) Start(ctx context.Context) error {
	f.startCount++
	f.status = Status{Target: string(f.target), Phase: PhaseListening}
	return nil
}

func (f *fakeLink) Stop(ctx context.Context) error {
	f.stopCount++
	f.status = Status{Target: string(f.target), Phase: PhaseDisconnected}
	return nil
}

func (f *fakeLink) StopAndWait(ctx context.Context) error {
	f.stopAndWaitCount++
	return f.Stop(ctx)
}

func (f *fakeLink) Status() Status {
	return f.status
}

func (f *fakeLink) Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error) {
	f.calls = append(f.calls, method)
	return map[string]any{"method": method}, nil
}

func TestSupervisorSwitchesRuntimeLinks(t *testing.T) {
	manager := NewManager()
	qq := newFakeLink(RuntimeTargetQQWS)
	wx := newFakeLink(RuntimeTargetWeChatCDP)
	s := NewSupervisor(manager, map[RuntimeTarget]RuntimeLink{
		RuntimeTargetQQWS:      qq,
		RuntimeTargetWeChatCDP: wx,
	})

	if err := s.Switch(context.Background(), RuntimeTargetQQWS); err != nil {
		t.Fatalf("switch qq: %v", err)
	}
	if err := s.Switch(context.Background(), RuntimeTargetWeChatCDP); err != nil {
		t.Fatalf("switch wx: %v", err)
	}
	if qq.stopAndWaitCount != 1 || wx.startCount != 1 {
		t.Fatalf("unexpected counts qq=%#v wx=%#v", qq, wx)
	}
	if manager.Status().Target != "wechat_cdp" {
		t.Fatalf("expected active target in status, got %#v", manager.Status())
	}
}

func TestSupervisorRegisterStopsActiveLinkBeforeReplacingIt(t *testing.T) {
	oldWX := newFakeLink(RuntimeTargetWeChatCDP)
	newWX := newFakeLink(RuntimeTargetWeChatCDP)
	s := NewSupervisor(NewManager(), map[RuntimeTarget]RuntimeLink{
		RuntimeTargetWeChatCDP: oldWX,
	})

	if err := s.Switch(context.Background(), RuntimeTargetWeChatCDP); err != nil {
		t.Fatalf("switch wx: %v", err)
	}
	s.Register(RuntimeTargetWeChatCDP, newWX)

	if oldWX.stopAndWaitCount != 1 {
		t.Fatalf("expected replaced active link to stop once, got %#v", oldWX)
	}
	_, err := s.Call(context.Background(), "host.describe", []any{}, time.Second)
	if err == nil || err.Error() != "runtime link is not connected" {
		t.Fatalf("expected replacement to leave supervisor disconnected, got %v", err)
	}
}

func TestSupervisorStopUsesOwnerJoin(t *testing.T) {
	wx := newFakeLink(RuntimeTargetWeChatCDP)
	s := NewSupervisor(NewManager(), map[RuntimeTarget]RuntimeLink{RuntimeTargetWeChatCDP: wx})
	if err := s.Switch(context.Background(), RuntimeTargetWeChatCDP); err != nil {
		t.Fatalf("switch wx: %v", err)
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("stop supervisor: %v", err)
	}
	if wx.stopAndWaitCount != 1 {
		t.Fatalf("expected owner stop-and-wait, got %#v", wx)
	}
}

func TestSupervisorCallRequiresActiveLink(t *testing.T) {
	s := NewSupervisor(NewManager(), nil)

	_, err := s.Call(context.Background(), "host.describe", []any{}, time.Second)
	if err == nil || err.Error() != "runtime link is not connected" {
		t.Fatalf("expected not connected error, got %v", err)
	}
}

func TestSupervisorCallForwardsToActiveLink(t *testing.T) {
	wx := newFakeLink(RuntimeTargetWeChatCDP)
	s := NewSupervisor(NewManager(), map[RuntimeTarget]RuntimeLink{
		RuntimeTargetWeChatCDP: wx,
	})

	if err := s.Switch(context.Background(), RuntimeTargetWeChatCDP); err != nil {
		t.Fatalf("switch wx: %v", err)
	}
	got, err := s.Call(context.Background(), "host.describe", []any{}, time.Second)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if wx.calls[0] != "host.describe" {
		t.Fatalf("call was not forwarded: %#v", wx.calls)
	}
	if got.(map[string]any)["method"] != "host.describe" {
		t.Fatalf("unexpected result %#v", got)
	}
}
