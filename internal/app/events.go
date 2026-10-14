package app

import "sync"

type eventBroker struct {
	mu          sync.Mutex
	subscribers map[chan string]struct{}
}

func newEventBroker() *eventBroker {
	return &eventBroker{
		subscribers: make(map[chan string]struct{}),
	}
}

func (b *eventBroker) subscribe() (<-chan string, func()) {
	ch := make(chan string, 16)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	unsubscribe := func() {
		b.mu.Lock()
		delete(b.subscribers, ch)
		b.mu.Unlock()
	}
	return ch, unsubscribe
}

func (b *eventBroker) publish(kind string) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subscribers {
		select {
		case ch <- kind:
		default:
		}
	}
}

// SubscribeEvents registers a listener channel for real-time project/workflow updates.
func (s *Service) SubscribeEvents() (<-chan string, func()) {
	if s == nil || s.events == nil {
		ch := make(chan string)
		return ch, func() {}
	}
	return s.events.subscribe()
}

func (s *Service) notifyEvent(kind string) {
	if s != nil && s.events != nil {
		s.events.publish(kind)
	}
}
