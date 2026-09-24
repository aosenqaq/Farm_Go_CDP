package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

type OrderedDispatcher[T any] struct {
	transport string
	clone     func(T) T
	describe  func(T) string
	capacity  int
	priority  func(T) int

	mu           sync.Mutex
	cond         *sync.Cond
	handlers     map[uint64]func(T)
	handlerOrder []uint64
	nextID       uint64
	queue        []T
	dropped      uint64
	state        dispatcherState
	done         chan struct{}
	logStop      chan struct{}
	overflowLogs chan overflowLog
	workers      sync.WaitGroup
}

type OrderedDispatcherOptions[T any] struct {
	Capacity int
	Priority func(T) int
}

const (
	EventPriorityOrdinary = iota
	EventPriorityGuardian
	EventPriorityLifecycle
)

type overflowLog struct {
	event   string
	action  string
	dropped uint64
}

type dispatcherState uint8

const (
	dispatcherStopped dispatcherState = iota
	dispatcherRunning
	dispatcherStopping
)

func NewOrderedDispatcher[T any](transport string, clone func(T) T, describe func(T) string) *OrderedDispatcher[T] {
	return NewOrderedDispatcherWithOptions(transport, clone, describe, OrderedDispatcherOptions[T]{})
}

func NewOrderedDispatcherWithOptions[T any](transport string, clone func(T) T, describe func(T) string, options OrderedDispatcherOptions[T]) *OrderedDispatcher[T] {
	capacity := options.Capacity
	if capacity <= 0 {
		capacity = 256
	}
	dispatcher := &OrderedDispatcher[T]{
		transport: transport,
		clone:     clone,
		describe:  describe,
		capacity:  capacity,
		priority:  options.Priority,
		handlers:  make(map[uint64]func(T)),
	}
	dispatcher.cond = sync.NewCond(&dispatcher.mu)
	return dispatcher
}

func (d *OrderedDispatcher[T]) Start() {
	for {
		d.mu.Lock()
		switch d.state {
		case dispatcherRunning:
			d.mu.Unlock()
			return
		case dispatcherStopping:
			done := d.done
			d.mu.Unlock()
			<-done
			continue
		default:
			d.state = dispatcherRunning
			d.done = make(chan struct{})
			d.logStop = make(chan struct{})
			d.overflowLogs = make(chan overflowLog, 1)
			d.workers.Add(2)
			done := d.done
			d.mu.Unlock()
			go d.run()
			go d.runOverflowLogger()
			go d.finishGeneration(done)
			return
		}
	}
}

func (d *OrderedDispatcher[T]) RequestStop() {
	d.mu.Lock()
	if d.state == dispatcherRunning {
		d.state = dispatcherStopping
		for index := range d.queue {
			var zero T
			d.queue[index] = zero
		}
		d.queue = nil
		close(d.logStop)
		d.cond.Broadcast()
	}
	d.mu.Unlock()
}

func (d *OrderedDispatcher[T]) Close() {
	d.RequestStop()
}

func (d *OrderedDispatcher[T]) CloseAndWait() {
	_ = d.CloseAndWaitContext(context.Background())
}

func (d *OrderedDispatcher[T]) CloseAndWaitContext(ctx context.Context) error {
	d.RequestStop()
	d.mu.Lock()
	if d.state == dispatcherStopped {
		d.mu.Unlock()
		return nil
	}
	done := d.done
	d.mu.Unlock()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *OrderedDispatcher[T]) Wait() {
	d.mu.Lock()
	if d.state == dispatcherStopped {
		d.mu.Unlock()
		return
	}
	done := d.done
	d.mu.Unlock()
	<-done
}

func (d *OrderedDispatcher[T]) Subscribe(handler func(T)) func() {
	if handler == nil {
		return func() {}
	}
	d.mu.Lock()
	d.nextID++
	id := d.nextID
	d.handlers[id] = handler
	d.handlerOrder = append(d.handlerOrder, id)
	d.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			d.mu.Lock()
			delete(d.handlers, id)
			for index, orderedID := range d.handlerOrder {
				if orderedID == id {
					copy(d.handlerOrder[index:], d.handlerOrder[index+1:])
					d.handlerOrder = d.handlerOrder[:len(d.handlerOrder)-1]
					break
				}
			}
			d.mu.Unlock()
		})
	}
}

func (d *OrderedDispatcher[T]) Dispatch(event T) bool {
	if d.clone != nil {
		event = d.clone(event)
	}
	d.mu.Lock()
	if d.state != dispatcherRunning {
		d.mu.Unlock()
		return false
	}
	if len(d.queue) >= d.capacity {
		incomingPriority := d.eventPriority(event)
		if incomingPriority > EventPriorityOrdinary {
			evicted := -1
			evictedPriority := incomingPriority
			for index, queued := range d.queue {
				queuedPriority := d.eventPriority(queued)
				if queuedPriority < evictedPriority {
					evicted = index
					evictedPriority = queuedPriority
				}
			}
			if evicted >= 0 {
				var zero T
				copy(d.queue[evicted:], d.queue[evicted+1:])
				d.queue[len(d.queue)-1] = zero
				d.queue = d.queue[:len(d.queue)-1]
				d.dropped++
				dropped := d.dropped
				d.queue = append(d.queue, event)
				d.cond.Signal()
				d.mu.Unlock()
				d.queueOverflowLog(event, dropped, "evicted lower-priority event")
				return true
			}
		}
		d.dropped++
		dropped := d.dropped
		d.mu.Unlock()
		d.queueOverflowLog(event, dropped, "dropped incoming event")
		return false
	}
	d.queue = append(d.queue, event)
	d.cond.Signal()
	d.mu.Unlock()
	return true
}

func (d *OrderedDispatcher[T]) Dropped() uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dropped
}

func (d *OrderedDispatcher[T]) eventPriority(event T) int {
	if d.priority == nil {
		return EventPriorityOrdinary
	}
	return d.priority(event)
}

func (d *OrderedDispatcher[T]) queueOverflowLog(event T, dropped uint64, action string) {
	eventDescription := ""
	if d.describe != nil {
		eventDescription = d.describe(event)
	}
	d.mu.Lock()
	logs := d.overflowLogs
	running := d.state == dispatcherRunning
	d.mu.Unlock()
	if !running || logs == nil {
		return
	}
	select {
	case logs <- overflowLog{event: eventDescription, action: action, dropped: dropped}:
	default:
	}
}

func (d *OrderedDispatcher[T]) run() {
	defer d.workers.Done()
	for {
		d.mu.Lock()
		for len(d.queue) == 0 && d.state == dispatcherRunning {
			d.cond.Wait()
		}
		if d.state == dispatcherStopping {
			d.mu.Unlock()
			return
		}
		event := d.queue[0]
		var zero T
		d.queue[0] = zero
		d.queue = d.queue[1:]
		if len(d.queue) == 0 {
			d.queue = nil
		}
		handlers := make([]func(T), 0, len(d.handlers))
		for _, id := range d.handlerOrder {
			if handler := d.handlers[id]; handler != nil {
				handlers = append(handlers, handler)
			}
		}
		d.mu.Unlock()

		for _, handler := range handlers {
			d.invoke(handler, event)
			d.mu.Lock()
			stopping := d.state != dispatcherRunning
			d.mu.Unlock()
			if stopping {
				return
			}
		}
	}
}

func (d *OrderedDispatcher[T]) runOverflowLogger() {
	defer d.workers.Done()
	d.mu.Lock()
	logs := d.overflowLogs
	stop := d.logStop
	d.mu.Unlock()
	for {
		select {
		case record := <-logs:
			slog.Warn("runtime event queue overflow",
				"transport", d.transport,
				"event", record.event,
				"action", record.action,
				"dropped", record.dropped,
			)
		case <-stop:
			return
		}
	}
}

func (d *OrderedDispatcher[T]) finishGeneration(done chan struct{}) {
	d.workers.Wait()
	d.mu.Lock()
	d.state = dispatcherStopped
	d.done = nil
	d.logStop = nil
	d.overflowLogs = nil
	d.mu.Unlock()
	close(done)
}

func (d *OrderedDispatcher[T]) invoke(handler func(T), event T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			eventDescription := ""
			if d.describe != nil {
				eventDescription = d.describe(event)
			}
			slog.Warn("runtime event handler panicked",
				"transport", d.transport,
				"event", eventDescription,
				"panic", fmt.Sprint(recovered),
			)
		}
	}()
	if d.clone != nil {
		event = d.clone(event)
	}
	handler(event)
}

func CloneJSONMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = cloneJSONValue(value)
	}
	return clone
}

func cloneJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return CloneJSONMap(typed)
	case []any:
		clone := make([]any, len(typed))
		for index, item := range typed {
			clone[index] = cloneJSONValue(item)
		}
		return clone
	case []byte:
		return append([]byte(nil), typed...)
	default:
		return value
	}
}
