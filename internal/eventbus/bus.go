package eventbus

import "sync"

type Bus struct {
	mu          sync.Mutex
	nextID      int64
	subscribers map[chan Event]struct{}
}

func New() *Bus {
	return &Bus{subscribers: map[chan Event]struct{}{}}
}

func (b *Bus) Publish(event Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	event.ID = b.nextID
	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (b *Bus) Subscribe() (chan Event, func()) {
	ch := make(chan Event, 32)

	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		if _, ok := b.subscribers[ch]; ok {
			delete(b.subscribers, ch)
			close(ch)
		}
		b.mu.Unlock()
	}

	return ch, cancel
}
