package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Event is one decoded SSE frame from agentd's /v1/runs/{id}/events. The
// payload shape mirrors internal/agentd's seqEvent: a sequence number, an
// event-type tag, a timestamp, and a type-erased Data field that consumers
// can decode based on Type.
type Event struct {
	Seq       int64           `json:"seq"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// Stream subscribes to the SSE event stream for a run and forwards decoded
// events to sink. It returns when:
//   - the server closes the stream cleanly (run reached terminal state) → nil
//   - ctx is canceled → ctx.Err()
//   - a parse or transport error occurs → the error
//
// Sink is NEVER closed by Stream — the caller may stream multiple peers
// into the same channel and is responsible for the channel's lifecycle.
// If sink is full and ctx is canceled, Stream returns ctx.Err() rather
// than blocking forever.
func (c *Client) Stream(ctx context.Context, runID string, sink chan<- Event) error {
	if runID == "" {
		return errors.New("Stream: runID is empty")
	}
	// Note: no withTimeout wrapper — SSE is long-lived.
	req, err := c.authReq(ctx, http.MethodGet, c.BaseURL+"/v1/runs/"+runID+"/events", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := c.http.Do(req)
	if err != nil {
		return c.wrap("GET /v1/runs/"+runID+"/events", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := readSmallBody(resp)
		return c.httpErr("GET /v1/runs/"+runID+"/events", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "text/event-stream") {
		return fmt.Errorf("peer %s: stream %s: unexpected Content-Type %q", c.Name, runID, ct)
	}

	return parseSSE(ctx, resp.Body, sink)
}

// parseSSE reads SSE frames from r and forwards decoded events to sink.
// Public for testability — direct callers of Stream don't need it.
//
// SSE format (the subset agentd emits):
//
//	id: <seq>
//	data: <one or more lines>
//	<blank line>
//
// A blank line terminates the frame. Lines beginning with `:` are
// comments (ignored). Multi-line data fields are joined with '\n' per the
// SSE spec, though agentd today emits single-line JSON.
func parseSSE(ctx context.Context, r io.Reader, sink chan<- Event) error {
	scanner := bufio.NewScanner(r)
	// Per-event JSON can be sizeable (full apply step record).
	// Match the daemon-side 1 MiB cap.
	scanner.Buffer(make([]byte, 64<<10), 1<<20)

	var (
		curID   int64
		curData strings.Builder
	)
	flush := func() error {
		if curData.Len() == 0 {
			// Comment-only or empty frame — drop.
			curID = 0
			return nil
		}
		var ev Event
		if err := json.Unmarshal([]byte(curData.String()), &ev); err != nil {
			return fmt.Errorf("decode SSE data: %w (raw=%q)", err, curData.String())
		}
		if ev.Seq == 0 {
			// The daemon assigns Seq via the JSONL sink. If absent in the
			// payload, fall back to the SSE id: line.
			ev.Seq = curID
		}
		select {
		case sink <- ev:
		case <-ctx.Done():
			return ctx.Err()
		}
		curID = 0
		curData.Reset()
		return nil
	}

	for scanner.Scan() {
		// Cancel check on every line so a hostile peer that sends nothing
		// for a long time still respects ctx promptly.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		switch {
		case line == "":
			if err := flush(); err != nil {
				return err
			}
		case strings.HasPrefix(line, ":"):
			// Comment / keep-alive.
		case strings.HasPrefix(line, "id:"):
			v := strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				curID = n
			}
		case strings.HasPrefix(line, "data:"):
			v := strings.TrimPrefix(line, "data:")
			v = strings.TrimPrefix(v, " ")
			if curData.Len() > 0 {
				curData.WriteByte('\n')
			}
			curData.WriteString(v)
		// Other fields (event:, retry:) ignored; agentd doesn't emit them.
		}
	}

	if err := scanner.Err(); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return ctx.Err()
		}
		return fmt.Errorf("read SSE stream: %w", err)
	}
	// Flush any final frame not followed by a blank line.
	return flush()
}
