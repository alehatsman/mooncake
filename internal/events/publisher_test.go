package events

import (
	"sync"
	"testing"
	"time"
)

// TestPublisherBasic tests basic publisher functionality
func TestPublisherBasic(t *testing.T) {
	publisher := NewPublisher()
	defer publisher.Close()

	// Create a test subscriber
	received := make([]Event, 0)
	var mu sync.Mutex

	subscriber := &testSubscriber{
		onEvent: func(e Event) {
			mu.Lock()
			received = append(received, e)
			mu.Unlock()
		},
	}

	// Subscribe
	id := publisher.Subscribe(subscriber)
	if id < 0 {
		t.Fatal("Subscribe returned invalid ID")
	}

	// Publish some events
	event1 := Event{
		Type:      EventRunStarted,
		Timestamp: time.Now(),
		Data: RunStartedData{
			RootFile:   "test.yml",
			TotalSteps: 5,
		},
	}

	event2 := Event{
		Type:      EventRunCompleted,
		Timestamp: time.Now(),
		Data: RunCompletedData{
			TotalSteps:   5,
			SuccessSteps: 5,
			Success:      true,
		},
	}

	publisher.Publish(event1)
	publisher.Publish(event2)

	// Wait for all events to be processed
	publisher.Flush()

	// Check received events
	mu.Lock()
	defer mu.Unlock()

	if len(received) != 2 {
		t.Errorf("Expected 2 events, got %d", len(received))
	}

	if received[0].Type != EventRunStarted {
		t.Errorf("Expected first event to be RunStarted, got %v", received[0].Type)
	}

	if received[1].Type != EventRunCompleted {
		t.Errorf("Expected second event to be RunCompleted, got %v", received[1].Type)
	}
}

// TestPublisherMultipleSubscribers tests multiple subscribers
func TestPublisherMultipleSubscribers(t *testing.T) {
	publisher := NewPublisher()
	defer publisher.Close()

	// Create two test subscribers
	received1 := make([]Event, 0)
	received2 := make([]Event, 0)
	var mu1, mu2 sync.Mutex

	sub1 := &testSubscriber{
		onEvent: func(e Event) {
			mu1.Lock()
			received1 = append(received1, e)
			mu1.Unlock()
		},
	}

	sub2 := &testSubscriber{
		onEvent: func(e Event) {
			mu2.Lock()
			received2 = append(received2, e)
			mu2.Unlock()
		},
	}

	// Subscribe both
	publisher.Subscribe(sub1)
	publisher.Subscribe(sub2)

	// Publish event
	event := Event{
		Type:      EventStepStarted,
		Timestamp: time.Now(),
		Data: StepStartedData{
			StepID: "step-1",
			Name:   "Test step",
		},
	}

	publisher.Publish(event)

	// Wait for all events to be processed
	publisher.Flush()

	// Both subscribers should receive the event
	mu1.Lock()
	defer mu1.Unlock()
	mu2.Lock()
	defer mu2.Unlock()

	if len(received1) != 1 {
		t.Errorf("Subscriber 1: Expected 1 event, got %d", len(received1))
	}

	if len(received2) != 1 {
		t.Errorf("Subscriber 2: Expected 1 event, got %d", len(received2))
	}
}

// TestPublisherUnsubscribe tests unsubscribing
func TestPublisherUnsubscribe(t *testing.T) {
	publisher := NewPublisher()
	defer publisher.Close()

	received := make([]Event, 0)
	var mu sync.Mutex

	subscriber := &testSubscriber{
		onEvent: func(e Event) {
			mu.Lock()
			received = append(received, e)
			mu.Unlock()
		},
	}

	// Subscribe and then unsubscribe
	id := publisher.Subscribe(subscriber)
	publisher.Unsubscribe(id)

	// Publish event
	event := Event{
		Type:      EventStepStarted,
		Timestamp: time.Now(),
		Data:      StepStartedData{},
	}

	publisher.Publish(event)

	// Wait for all events to be processed
	publisher.Flush()

	// Should not receive event
	mu.Lock()
	defer mu.Unlock()

	if len(received) != 0 {
		t.Errorf("Expected 0 events after unsubscribe, got %d", len(received))
	}
}

// testSubscriber is a simple test implementation of Subscriber
type testSubscriber struct {
	onEvent func(Event)
}

func (t *testSubscriber) OnEvent(event Event) {
	if t.onEvent != nil {
		t.onEvent(event)
	}
}

// Close is a no-op for test subscriber - no cleanup required
func (t *testSubscriber) Close() {}

// TestSyncPublisher_Basic tests basic SyncPublisher functionality
func TestSyncPublisher_Basic(t *testing.T) {
	publisher := NewSyncPublisher()
	defer publisher.Close()

	// Create a test subscriber
	received := make([]Event, 0)

	subscriber := &testSubscriber{
		onEvent: func(e Event) {
			received = append(received, e)
		},
	}

	// Subscribe
	id := publisher.Subscribe(subscriber)
	if id < 0 {
		t.Fatal("Subscribe returned invalid ID")
	}

	// Publish some events
	event1 := Event{
		Type:      EventRunStarted,
		Timestamp: time.Now(),
		Data: RunStartedData{
			RootFile:   "test.yml",
			TotalSteps: 5,
		},
	}

	event2 := Event{
		Type:      EventRunCompleted,
		Timestamp: time.Now(),
		Data: RunCompletedData{
			TotalSteps:   5,
			SuccessSteps: 5,
			Success:      true,
		},
	}

	publisher.Publish(event1)
	publisher.Publish(event2)

	// No need to wait - SyncPublisher delivers synchronously

	// Check received events
	if len(received) != 2 {
		t.Errorf("Expected 2 events, got %d", len(received))
	}

	if received[0].Type != EventRunStarted {
		t.Errorf("Expected first event to be RunStarted, got %v", received[0].Type)
	}

	if received[1].Type != EventRunCompleted {
		t.Errorf("Expected second event to be RunCompleted, got %v", received[1].Type)
	}
}

// TestSyncPublisher_MultipleSubscribers tests multiple subscribers
func TestSyncPublisher_MultipleSubscribers(t *testing.T) {
	publisher := NewSyncPublisher()
	defer publisher.Close()

	// Create two test subscribers
	received1 := make([]Event, 0)
	received2 := make([]Event, 0)

	sub1 := &testSubscriber{
		onEvent: func(e Event) {
			received1 = append(received1, e)
		},
	}

	sub2 := &testSubscriber{
		onEvent: func(e Event) {
			received2 = append(received2, e)
		},
	}

	// Subscribe both
	publisher.Subscribe(sub1)
	publisher.Subscribe(sub2)

	// Publish event
	event := Event{
		Type:      EventStepStarted,
		Timestamp: time.Now(),
		Data: StepStartedData{
			StepID: "step-1",
			Name:   "Test step",
		},
	}

	publisher.Publish(event)

	// Both subscribers should receive the event immediately
	if len(received1) != 1 {
		t.Errorf("Subscriber 1: Expected 1 event, got %d", len(received1))
	}

	if len(received2) != 1 {
		t.Errorf("Subscriber 2: Expected 1 event, got %d", len(received2))
	}
}

// TestSyncPublisher_Unsubscribe tests unsubscribing
func TestSyncPublisher_Unsubscribe(t *testing.T) {
	publisher := NewSyncPublisher()
	defer publisher.Close()

	received := make([]Event, 0)

	subscriber := &testSubscriber{
		onEvent: func(e Event) {
			received = append(received, e)
		},
	}

	// Subscribe and then unsubscribe
	id := publisher.Subscribe(subscriber)
	publisher.Unsubscribe(id)

	// Publish event
	event := Event{
		Type:      EventStepStarted,
		Timestamp: time.Now(),
		Data:      StepStartedData{},
	}

	publisher.Publish(event)

	// Should not receive event
	if len(received) != 0 {
		t.Errorf("Expected 0 events after unsubscribe, got %d", len(received))
	}
}

// TestSyncPublisher_Flush tests that Flush is a no-op
func TestSyncPublisher_Flush(t *testing.T) {
	publisher := NewSyncPublisher()
	defer publisher.Close()

	// Flush should return immediately (no-op for sync publisher)
	publisher.Flush()
	// If we get here without hanging, the test passes
}

// TestSyncPublisher_CloseIdempotent tests that Close can be called multiple times
func TestSyncPublisher_CloseIdempotent(t *testing.T) {
	publisher := NewSyncPublisher()

	// Close multiple times should not panic
	publisher.Close()
	publisher.Close()
	publisher.Close()
}

// TestSyncPublisher_PublishAfterClose tests publishing after close
func TestSyncPublisher_PublishAfterClose(t *testing.T) {
	publisher := NewSyncPublisher()

	received := make([]Event, 0)
	subscriber := &testSubscriber{
		onEvent: func(e Event) {
			received = append(received, e)
		},
	}

	publisher.Subscribe(subscriber)
	publisher.Close()

	// Publish after close should be no-op
	event := Event{
		Type:      EventStepStarted,
		Timestamp: time.Now(),
		Data:      StepStartedData{},
	}

	publisher.Publish(event)

	// Should not receive event
	if len(received) != 0 {
		t.Errorf("Expected 0 events after close, got %d", len(received))
	}
}

// TestSyncPublisher_SubscribeAfterClose tests subscribing after close
func TestSyncPublisher_SubscribeAfterClose(t *testing.T) {
	publisher := NewSyncPublisher()
	publisher.Close()

	subscriber := &testSubscriber{
		onEvent: func(e Event) {},
	}

	// Subscribe after close should return -1
	id := publisher.Subscribe(subscriber)
	if id != -1 {
		t.Errorf("Expected -1 for subscribe after close, got %d", id)
	}
}

// TestSyncPublisher_UnsubscribeNonExistent tests unsubscribing non-existent ID
func TestSyncPublisher_UnsubscribeNonExistent(t *testing.T) {
	publisher := NewSyncPublisher()
	defer publisher.Close()

	// Unsubscribe non-existent ID should not panic
	publisher.Unsubscribe(999)
}

// TestSyncPublisher_ConcurrentAccess tests concurrent access to SyncPublisher
func TestSyncPublisher_ConcurrentAccess(t *testing.T) {
	publisher := NewSyncPublisher()
	defer publisher.Close()

	var wg sync.WaitGroup
	numGoroutines := 10
	numEvents := 100

	// Create multiple subscribers concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			subscriber := &testSubscriber{
				onEvent: func(e Event) {},
			}
			id := publisher.Subscribe(subscriber)
			if id > 0 {
				publisher.Unsubscribe(id)
			}
		}()
	}

	// Publish events concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numEvents; j++ {
				event := Event{
					Type:      EventStepStarted,
					Timestamp: time.Now(),
					Data:      StepStartedData{},
				}
				publisher.Publish(event)
			}
		}()
	}

	wg.Wait()
}
