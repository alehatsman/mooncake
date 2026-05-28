package agentd

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/alehatsman/mooncake/internal/events"
)

// seqEvent is the on-disk and on-wire form of an event: the original event
// fields plus a monotonic seq counter for replay→tail coordination.
type seqEvent struct {
	Seq       int64       `json:"seq"`
	Type      events.Type `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

// RunEventSink is the per-run events.Subscriber that bridges the publisher,
// the JSONL file on disk, and the in-memory Hub for SSE clients.
//
// OnEvent is non-blocking by design: events.ChannelPublisher silently drops
// events past its 100-event per-subscriber buffer (see internal/events/
// publisher.go), so a slow JSONL writer would lose data. We instead enqueue
// each encoded line and let a dedicated goroutine drain the queue to disk
// while OnEvent returns immediately.
//
// Close() drains the remaining queue, flushes the buffered writer, fsyncs the
// file, then closes the Hub. Worker MUST call Close() before writing the
// terminal run record — otherwise GET /v1/runs/{id} can report success while
// events.jsonl is still missing the tail.
type RunEventSink struct {
	runID string
	hub   *Hub
	file  *os.File
	bw    *bufio.Writer
	log   *slog.Logger

	mu      sync.Mutex
	cond    *sync.Cond
	queue   [][]byte // pending encoded lines
	seq     int64
	closing bool

	writerDone chan struct{}
}

// NewRunEventSink opens the run's events.jsonl for appending and starts the
// background writer goroutine.
func NewRunEventSink(runID string, eventsPath string, hub *Hub, log *slog.Logger) (*RunEventSink, error) {
	f, err := os.OpenFile(eventsPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	s := &RunEventSink{
		runID:      runID,
		hub:        hub,
		file:       f,
		bw:         bufio.NewWriter(f),
		log:        log,
		writerDone: make(chan struct{}),
	}
	s.cond = sync.NewCond(&s.mu)
	go s.writeLoop()
	return s, nil
}

// OnEvent serializes the event with a fresh seq, appends to the in-memory
// queue, signals the writer, and broadcasts to the Hub. Never blocks.
func (s *RunEventSink) OnEvent(ev events.Event) {
	s.mu.Lock()
	s.seq++
	seq := s.seq
	line, err := json.Marshal(seqEvent{
		Seq:       seq,
		Type:      ev.Type,
		Timestamp: ev.Timestamp,
		Data:      ev.Data,
	})
	if err != nil {
		s.mu.Unlock()
		s.log.Error("encode event", "run_id", s.runID, "err", err)
		return
	}
	line = append(line, '\n')
	s.queue = append(s.queue, line)
	s.cond.Signal()
	s.mu.Unlock()

	// Broadcast outside the lock so a slow subscriber can't gum up the queue.
	s.hub.Broadcast(seq, line)
}

// Close drains the queue, flushes, fsyncs, and closes the Hub. Blocks until
// the writer goroutine exits and the file is durable.
func (s *RunEventSink) Close() {
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		<-s.writerDone
		return
	}
	s.closing = true
	s.cond.Signal()
	s.mu.Unlock()

	<-s.writerDone

	if err := s.bw.Flush(); err != nil {
		s.log.Error("flush events.jsonl", "run_id", s.runID, "err", err)
	}
	if err := s.file.Sync(); err != nil {
		s.log.Error("fsync events.jsonl", "run_id", s.runID, "err", err)
	}
	if err := s.file.Close(); err != nil {
		s.log.Error("close events.jsonl", "run_id", s.runID, "err", err)
	}
	s.hub.Close()
}

func (s *RunEventSink) writeLoop() {
	defer close(s.writerDone)
	for {
		s.mu.Lock()
		for len(s.queue) == 0 && !s.closing {
			s.cond.Wait()
		}
		batch := s.queue
		s.queue = nil
		closing := s.closing
		s.mu.Unlock()

		for _, line := range batch {
			if _, err := s.bw.Write(line); err != nil {
				s.log.Error("write event line", "run_id", s.runID, "err", err)
			}
		}

		if closing {
			return
		}
	}
}
