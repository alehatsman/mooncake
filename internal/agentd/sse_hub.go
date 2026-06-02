package agentd

import (
	"sync"
)

// HubMessage is one broadcast — a seq number and the already-encoded event
// line. The sink encodes once and broadcasts the same bytes to all subscribers
// AND to the JSONL file, so consumers see byte-identical content.
type HubMessage struct {
	Seq  int64
	Line []byte
}

type hubSub struct {
	ch chan HubMessage
}

// Hub is a per-run in-memory broadcaster. The worker's event sink calls
// Broadcast for every event; HTTP SSE handlers Subscribe to receive live
// events from the same run.
//
// Subscribers get a buffered channel. If a subscriber falls behind, the hub
// drops messages (does not block the broadcaster); the client will see the
// missing seqs as a gap and can reconnect with Last-Event-ID to backfill from
// JSONL.
type Hub struct {
	mu          sync.Mutex
	lastSeq     int64
	subscribers map[int]*hubSub
	nextSubID   int
	closed      bool
}

func NewHub() *Hub {
	return &Hub{subscribers: make(map[int]*hubSub)}
}

const hubSubBuffer = 256

// Subscribe registers a subscriber and returns its channel plus an atomic
// snapshot of lastSeq taken under the same lock — so callers can replay JSONL
// up to lastSeq and then forward channel messages with seq > lastSeq to bridge
// the replay→tail boundary without races.
func (h *Hub) Subscribe() (ch <-chan HubMessage, lastSeq int64, unsubscribe func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	sub := &hubSub{ch: make(chan HubMessage, hubSubBuffer)}

	if h.closed {
		// Hub already closed (subscribers map is nil); deliver no events.
		// Must check this BEFORE touching the map, or the assignment below
		// panics with "assignment to entry in nil map".
		close(sub.ch)
		return sub.ch, h.lastSeq, func() {}
	}

	id := h.nextSubID
	h.nextSubID++
	h.subscribers[id] = sub

	return sub.ch, h.lastSeq, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subscribers[id]; ok {
			delete(h.subscribers, id)
			close(sub.ch)
		}
	}
}

// Broadcast publishes a message to all current subscribers. Non-blocking: if
// any subscriber's buffer is full, the message is dropped for that subscriber.
func (h *Hub) Broadcast(seq int64, line []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.lastSeq = seq
	msg := HubMessage{Seq: seq, Line: line}
	for _, sub := range h.subscribers {
		select {
		case sub.ch <- msg:
		default:
			// Drop on overflow. The next SSE reconnect with Last-Event-ID
			// will backfill from JSONL.
		}
	}
}

// LastSeq returns the highest seq broadcast so far. Useful for replay
// coordination (snapshot lastSeq, then read JSONL up to that seq).
func (h *Hub) LastSeq() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastSeq
}

// Close terminates all current subscribers' channels and refuses future
// broadcasts. Idempotent.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for _, sub := range h.subscribers {
		close(sub.ch)
	}
	h.subscribers = nil
}
