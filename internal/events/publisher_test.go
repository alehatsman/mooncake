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

	// Give events time to be processed
	time.Sleep(50 * time.Millisecond)

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

	// Give events time to be processed
	time.Sleep(50 * time.Millisecond)

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

	// Give time for processing
	time.Sleep(50 * time.Millisecond)

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

func (t *testSubscriber) Close() {}
