package fanout

import (
	"log/slog"
	"sync"
)

// SubscriberHub is the Desktop fan-out boundary (stub in v1).
type SubscriberHub interface {
	PublishLive(agentID string, frame []byte)
	MarkStale(agentID string)
}

// DropFunc is called when a buffered publish drops the oldest frame for agentID.
type DropFunc func(agentID string)

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

// BufferedHub keeps a per-agent drop-oldest queue so a stalled Desktop consumer
// never blocks Agent publishers. Other agents have independent queues.
type BufferedHub struct {
	cap    int
	onDrop DropFunc
	mu     sync.Mutex
	qs     map[string]*DropOldestQueue
	stale  []string
}

// NewBufferedHub creates a hub with per-agent capacity n. onDrop may be nil.
func NewBufferedHub(n int, onDrop DropFunc) *BufferedHub {
	if n < 1 {
		n = 1
	}
	return &BufferedHub{
		cap:    n,
		onDrop: onDrop,
		qs:     make(map[string]*DropOldestQueue),
	}
}

func (h *BufferedHub) PublishLive(agentID string, frame []byte) {
	h.mu.Lock()
	q, ok := h.qs[agentID]
	if !ok {
		q = NewDropOldestQueue(h.cap)
		h.qs[agentID] = q
	}
	dropped := q.Push(frame)
	h.mu.Unlock()
	if dropped && h.onDrop != nil {
		h.onDrop(agentID)
	}
}

func (h *BufferedHub) MarkStale(agentID string) {
	h.mu.Lock()
	h.stale = append(h.stale, agentID)
	delete(h.qs, agentID)
	h.mu.Unlock()
}

// Len returns buffered frames for agentID.
func (h *BufferedHub) Len(agentID string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if q := h.qs[agentID]; q != nil {
		return q.Len()
	}
	return 0
}

// Drops returns cumulative drops for agentID (0 if unknown).
func (h *BufferedHub) Drops(agentID string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if q := h.qs[agentID]; q != nil {
		return q.Drops()
	}
	return 0
}

// Pop removes and returns the oldest frame for agentID, or nil.
func (h *BufferedHub) Pop(agentID string) []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	if q := h.qs[agentID]; q != nil {
		return q.Pop()
	}
	return nil
}

// StaleCalls returns MarkStale agent IDs (test/observe helper).
func (h *BufferedHub) StaleCalls() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.stale))
	copy(out, h.stale)
	return out
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
