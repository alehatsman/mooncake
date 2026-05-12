package agentd

import (
	"sync"
	"testing"
)

func TestHubSubscribeReceivesBroadcast(t *testing.T) {
	h := NewHub()
	defer h.Close()

	ch, lastSeq, unsub := h.Subscribe()
	defer unsub()
	if lastSeq != 0 {
		t.Errorf("want lastSeq=0 at subscribe time, got %d", lastSeq)
	}

	h.Broadcast(1, []byte("event-1"))
	msg := <-ch
	if msg.Seq != 1 || string(msg.Line) != "event-1" {
		t.Errorf("unexpected message: %+v", msg)
	}
}

func TestHubLastSeqReflectsLatest(t *testing.T) {
	h := NewHub()
	defer h.Close()

	h.Broadcast(1, []byte("a"))
	h.Broadcast(2, []byte("b"))
	h.Broadcast(3, []byte("c"))
	if got := h.LastSeq(); got != 3 {
		t.Errorf("want lastSeq=3, got %d", got)
	}
}

func TestHubSubscribeAfterBroadcastsCapturesLastSeq(t *testing.T) {
	h := NewHub()
	defer h.Close()

	h.Broadcast(1, []byte("a"))
	h.Broadcast(2, []byte("b"))

	ch, snapshotSeq, unsub := h.Subscribe()
	defer unsub()
	if snapshotSeq != 2 {
		t.Errorf("want snapshotSeq=2, got %d", snapshotSeq)
	}

	// New broadcast lands in the channel.
	h.Broadcast(3, []byte("c"))
	msg := <-ch
	if msg.Seq != 3 {
		t.Errorf("want seq=3, got %d", msg.Seq)
	}
}

func TestHubCloseTerminatesSubscriber(t *testing.T) {
	h := NewHub()
	ch, _, _ := h.Subscribe()
	h.Close()
	_, ok := <-ch
	if ok {
		t.Errorf("channel should be closed after Hub.Close")
	}
}

func TestHubUnsubscribeStopsDelivery(t *testing.T) {
	h := NewHub()
	defer h.Close()
	ch, _, unsub := h.Subscribe()
	unsub()
	// Channel should be closed.
	if _, ok := <-ch; ok {
		t.Errorf("channel should be closed after unsubscribe")
	}
}

func TestHubBroadcastDoesNotBlockOnSlowSubscriber(t *testing.T) {
	h := NewHub()
	defer h.Close()
	_, _, _ = h.Subscribe() // never read; capacity hubSubBuffer
	// Push way more than the buffer can hold. If Broadcast blocked, this
	// would hang and the test would time out.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < hubSubBuffer*4; i++ {
			h.Broadcast(int64(i+1), []byte("x"))
		}
	}()
	wg.Wait()
}
