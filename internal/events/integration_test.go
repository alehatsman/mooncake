package events

import (
	"sync"
	"testing"
	"time"
)

// TestEventSystemIntegration tests the event system end-to-end
func TestEventSystemIntegration(t *testing.T) {
	publisher := NewPublisher()
	defer publisher.Close()

	// Create a test subscriber that collects all events
	collector := &eventCollector{
		events: make([]Event, 0),
	}

	publisher.Subscribe(collector)

	// Simulate a run lifecycle
	publisher.Publish(Event{
		Type:      EventRunStarted,
		Timestamp: time.Now(),
		Data: RunStartedData{
			RootFile:   "test.yml",
			TotalSteps: 3,
		},
	})

	publisher.Publish(Event{
		Type:      EventPlanLoaded,
		Timestamp: time.Now(),
		Data: PlanLoadedData{
			RootFile:   "test.yml",
			TotalSteps: 3,
		},
	})

	// Simulate step execution
	publisher.Publish(Event{
		Type:      EventStepStarted,
		Timestamp: time.Now(),
		Data: StepStartedData{
			StepID:     "step-1",
			Name:       "Test step",
			Level:      0,
			GlobalStep: 1,
			Action:     "shell",
		},
	})

	publisher.Publish(Event{
		Type:      EventStepStdout,
		Timestamp: time.Now(),
		Data: StepOutputData{
			StepID:     "step-1",
			Stream:     "stdout",
			Line:       "Hello, world!",
			LineNumber: 1,
		},
	})

	publisher.Publish(Event{
		Type:      EventStepCompleted,
		Timestamp: time.Now(),
		Data: StepCompletedData{
			StepID:     "step-1",
			Name:       "Test step",
			Level:      0,
			DurationMs: 100,
			Changed:    true,
		},
	})

	publisher.Publish(Event{
		Type:      EventRunCompleted,
		Timestamp: time.Now(),
		Data: RunCompletedData{
			TotalSteps:   3,
			SuccessSteps: 3,
			DurationMs:   300,
			Success:      true,
		},
	})

	// Wait for events to be processed
	time.Sleep(100 * time.Millisecond)

	// Verify events were collected
	collector.mu.Lock()
	defer collector.mu.Unlock()

	if len(collector.events) != 6 {
		t.Errorf("Expected 6 events, got %d", len(collector.events))
	}

	// Verify event types in order
	expectedTypes := []EventType{
		EventRunStarted,
		EventPlanLoaded,
		EventStepStarted,
		EventStepStdout,
		EventStepCompleted,
		EventRunCompleted,
	}

	for i, expectedType := range expectedTypes {
		if i >= len(collector.events) {
			t.Errorf("Missing event at index %d", i)
			continue
		}
		if collector.events[i].Type != expectedType {
			t.Errorf("Event %d: expected type %v, got %v", i, expectedType, collector.events[i].Type)
		}
	}
}

// TestMultipleSubscribers tests that multiple subscribers can receive events
func TestMultipleSubscribers(t *testing.T) {
	publisher := NewPublisher()
	defer publisher.Close()

	// Create two collectors
	collector1 := &eventCollector{events: make([]Event, 0)}
	collector2 := &eventCollector{events: make([]Event, 0)}

	publisher.Subscribe(collector1)
	publisher.Subscribe(collector2)

	// Publish some events
	for i := 0; i < 5; i++ {
		publisher.Publish(Event{
			Type:      EventStepStarted,
			Timestamp: time.Now(),
			Data: StepStartedData{
				StepID: "step-" + string(rune(i)),
				Name:   "Step",
			},
		})
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Both collectors should have all events
	collector1.mu.Lock()
	collector2.mu.Lock()
	defer collector1.mu.Unlock()
	defer collector2.mu.Unlock()

	if len(collector1.events) != 5 {
		t.Errorf("Collector 1: expected 5 events, got %d", len(collector1.events))
	}

	if len(collector2.events) != 5 {
		t.Errorf("Collector 2: expected 5 events, got %d", len(collector2.events))
	}
}

// TestEventOrdering tests that events are delivered in order
func TestEventOrdering(t *testing.T) {
	publisher := NewPublisher()
	defer publisher.Close()

	collector := &eventCollector{events: make([]Event, 0)}
	publisher.Subscribe(collector)

	// Publish events with sequence numbers
	for i := 0; i < 100; i++ {
		publisher.Publish(Event{
			Type:      EventStepStarted,
			Timestamp: time.Now(),
			Data: StepStartedData{
				StepID:     string(rune(i)),
				GlobalStep: i,
			},
		})
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Verify order
	collector.mu.Lock()
	defer collector.mu.Unlock()

	for i := 0; i < len(collector.events); i++ {
		data, ok := collector.events[i].Data.(StepStartedData)
		if !ok {
			t.Errorf("Event %d: wrong data type", i)
			continue
		}
		if data.GlobalStep != i {
			t.Errorf("Event %d: expected GlobalStep %d, got %d", i, i, data.GlobalStep)
		}
	}
}

// eventCollector is a test subscriber that collects all events
type eventCollector struct {
	events []Event
	mu     sync.Mutex
}

func (c *eventCollector) OnEvent(event Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
}

func (c *eventCollector) Close() {}
