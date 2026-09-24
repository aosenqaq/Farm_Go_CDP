package automation

import (
	"context"
	"errors"
	"sync"
	"time"
)

type fakeRuntimeCaller struct {
	mu        sync.Mutex
	value     any
	err       error
	responses map[string]any
	calls     []runtimeCall
	timeout   time.Duration
}

type fakeRuntimeResponseSequence []any

type runtimeCall struct {
	method string
	args   []any
}

func (f *fakeRuntimeCaller) Call(_ context.Context, method string, args []any, timeout time.Duration) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, runtimeCall{method: method, args: args})
	f.timeout = timeout
	if f.responses != nil {
		value, ok := f.responses[method]
		if !ok {
			return nil, errors.New("unexpected method: " + method)
		}
		if sequence, ok := value.(fakeRuntimeResponseSequence); ok {
			if len(sequence) == 0 {
				return nil, errors.New("empty response sequence for method: " + method)
			}
			value = sequence[0]
			if len(sequence) > 1 {
				f.responses[method] = fakeRuntimeResponseSequence(sequence[1:])
			} else {
				f.responses[method] = value
			}
		}
		if err, ok := value.(error); ok {
			return nil, err
		}
		return value, nil
	}
	return f.value, f.err
}
