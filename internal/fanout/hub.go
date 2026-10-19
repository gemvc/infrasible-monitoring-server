package fanout

import (
	"log/slog"
)

// SubscriberHub is the Desktop fan-out boundary (stub in v1).
type SubscriberHub interface {
	PublishLive(agentID string, frame []byte)
	MarkStale(agentID string)
}

type noopHub struct{}

// NewNoopHub discards all events.
func NewNoopHub() SubscriberHub { return noopHub{} }

func (noopHub) PublishLive(string, []byte) {}
func (noopHub) MarkStale(string)           {}

type logHub struct {
	log *slog.Logger
}

// NewLogHub logs publish/stale at debug/info without secrets.
func NewLogHub(logger *slog.Logger) SubscriberHub {
	if logger == nil {
		logger = slog.Default()
	}
	return &logHub{log: logger}
}

func (h *logHub) PublishLive(agentID string, frame []byte) {
	h.log.Debug("hub.publish_live", "agent_id", agentID, "bytes", len(frame))
}

func (h *logHub) MarkStale(agentID string) {
	h.log.Info("hub.mark_stale", "agent_id", agentID)
}

// DropOldestQueue is a bounded buffer for slow Desktop consumers.
type DropOldestQueue struct {
	cap  int
	buf  [][]byte
	drop int
}

// NewDropOldestQueue creates a queue with capacity n (>=1).
func NewDropOldestQueue(n int) *DropOldestQueue {
	if n < 1 {
		n = 1
	}
	return &DropOldestQueue{cap: n, buf: make([][]byte, 0, n)}
}

// Push appends; if full, drops the oldest and returns dropped=true.
func (q *DropOldestQueue) Push(b []byte) (dropped bool) {
	cp := append([]byte(nil), b...)
	if len(q.buf) >= q.cap {
		q.buf = q.buf[1:]
		q.drop++
		dropped = true
	}
	q.buf = append(q.buf, cp)
	return dropped
}

// Len returns current buffered frames.
func (q *DropOldestQueue) Len() int { return len(q.buf) }

// Drops returns cumulative drop count.
func (q *DropOldestQueue) Drops() int { return q.drop }

// Pop removes and returns the oldest frame, or nil if empty.
func (q *DropOldestQueue) Pop() []byte {
	if len(q.buf) == 0 {
		return nil
	}
	b := q.buf[0]
	q.buf = q.buf[1:]
	return b
}
