package runtime

import (
	"context"
	"time"
)

type RuntimeTarget string

const (
	RuntimeTargetQQWS      RuntimeTarget = "qq_ws"
	RuntimeTargetWeChatCDP RuntimeTarget = "wechat_cdp"
	RuntimeTargetYYBCDP    RuntimeTarget = "yyb_cdp"
)

type RuntimeLink interface {
	Target() RuntimeTarget
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Status() Status
	Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error)
}

type RuntimeLinkOwnerStopper interface {
	StopAndWait(ctx context.Context) error
}
