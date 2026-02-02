package events

import (
	"sync"
)

// Publisher publishes events to subscribers
type Publisher interface {
	Publish(event Event)
	Subscribe(subscriber Subscriber) int
	Unsubscribe(id int)
	Flush() // Wait for all pending events to be processed
	Close()
}

// Subscriber receives events from a publisher
type Subscriber interface {
	OnEvent(event Event)
	Close()
}

// ChannelPublisher implements Publisher using buffered channels
type ChannelPublisher struct {
	subscribers map[int]chan Event
	nextID      int
	mu          sync.RWMutex
	closed      bool
	wg          sync.WaitGroup
}

// NewPublisher creates a new channel-based event publisher
func NewPublisher() Publisher {
	return &ChannelPublisher{
		subscribers: make(map[int]chan Event),
		nextID:      1,
	}
}

// Publish sends an event to all subscribers
// This is non-blocking - if a subscriber's channel is full, the event is dropped
func (p *ChannelPublisher) Publish(event Event) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return
	}

	for _, ch := range p.subscribers {
		// Non-blocking send - drop if channel is full
		select {
		case ch <- event:
		default:
			// Channel full, drop event
		}
	}
}

// Subscribe adds a new subscriber and returns its ID
func (p *ChannelPublisher) Subscribe(subscriber Subscriber) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return -1
	}

	id := p.nextID
	p.nextID++

	// Create buffered channel for this subscriber
	ch := make(chan Event, 100)
	p.subscribers[id] = ch

	// Start goroutine to forward events to subscriber
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for event := range ch {
			subscriber.OnEvent(event)
		}
	}()

	return id
}

// Unsubscribe removes a subscriber
func (p *ChannelPublisher) Unsubscribe(id int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if ch, ok := p.subscribers[id]; ok {
		close(ch)
		delete(p.subscribers, id)
	}
}

// Flush is a no-op for ChannelPublisher (async by design).
// Use SyncPublisher for tests that need synchronous event delivery.
func (p *ChannelPublisher) Flush() {
	// No-op - async publisher doesn't support flushing
}

// Close closes the publisher and all subscriber channels
func (p *ChannelPublisher) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true

	// Close all subscriber channels
	for _, ch := range p.subscribers {
		close(ch)
	}
	p.subscribers = make(map[int]chan Event)
	p.mu.Unlock()

	// Wait for all forwarding goroutines to finish
	p.wg.Wait()
}

// SyncPublisher implements Publisher with synchronous event delivery.
// Events are delivered immediately via direct OnEvent() calls.
// This is useful for tests to avoid race conditions with async delivery.
type SyncPublisher struct {
	subscribers map[int]Subscriber
	nextID      int
	mu          sync.RWMutex
	closed      bool
}

// NewSyncPublisher creates a new synchronous event publisher for testing.
func NewSyncPublisher() Publisher {
	return &SyncPublisher{
		subscribers: make(map[int]Subscriber),
		nextID:      1,
	}
}

// Publish sends an event to all subscribers synchronously.
func (p *SyncPublisher) Publish(event Event) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return
	}

	for _, sub := range p.subscribers {
		sub.OnEvent(event)
	}
}

// Subscribe adds a new subscriber and returns its ID.
func (p *SyncPublisher) Subscribe(subscriber Subscriber) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return -1
	}

	id := p.nextID
	p.nextID++
	p.subscribers[id] = subscriber
	return id
}

// Unsubscribe removes a subscriber.
func (p *SyncPublisher) Unsubscribe(id int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.subscribers, id)
}

// Flush is a no-op for SyncPublisher (already synchronous).
func (p *SyncPublisher) Flush() {
	// No-op - sync publisher delivers events immediately
}

// Close closes the publisher.
func (p *SyncPublisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}
	p.closed = true
	p.subscribers = make(map[int]Subscriber)
}
