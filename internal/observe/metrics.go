package observe

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds process-wide observe counters (no secrets).
type Metrics struct {
	connected   atomic.Int64
	helloAccept atomic.Int64
	frames      atomic.Int64
	drops       atomic.Int64

	mu           sync.Mutex
	authRejects  map[string]int64
	lastFrame    map[string]time.Time
	frameByAgent map[string]int64
	dropByAgent  map[string]int64
}

// NewMetrics constructs zeroed metrics.
func NewMetrics() *Metrics {
	return &Metrics{
		authRejects:  make(map[string]int64),
		lastFrame:    make(map[string]time.Time),
		frameByAgent: make(map[string]int64),
		dropByAgent:  make(map[string]int64),
	}
}

func (m *Metrics) IncConnected(delta int64) { m.connected.Add(delta) }

func (m *Metrics) IncHelloAccept() { m.helloAccept.Add(1) }

func (m *Metrics) IncAuthReject(reason string) {
	if reason == "" {
		reason = "unknown"
	}
	m.mu.Lock()
	m.authRejects[reason]++
	m.mu.Unlock()
}

func (m *Metrics) IncFrames(agentID string) {
	m.frames.Add(1)
	m.mu.Lock()
	m.frameByAgent[agentID]++
	m.mu.Unlock()
}

func (m *Metrics) IncDrops(agentID string) {
	m.drops.Add(1)
	m.mu.Lock()
	m.dropByAgent[agentID]++
	m.mu.Unlock()
}

func (m *Metrics) SetLastFrame(agentID string, t time.Time) {
	m.mu.Lock()
	m.lastFrame[agentID] = t
	m.mu.Unlock()
}

// Snapshot is a point-in-time view for logs / admin metrics.
type Snapshot struct {
	Connected     int64
	HelloAccepts  int64
	Frames        int64
	Drops         int64
	AuthRejects   map[string]int64
	LastFrameUnix map[string]int64
	FramesByAgent map[string]int64
	DropsByAgent  map[string]int64
}

func (m *Metrics) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	rejects := make(map[string]int64, len(m.authRejects))
	for k, v := range m.authRejects {
		rejects[k] = v
	}
	last := make(map[string]int64, len(m.lastFrame))
	for k, v := range m.lastFrame {
		last[k] = v.Unix()
	}
	fb := make(map[string]int64, len(m.frameByAgent))
	for k, v := range m.frameByAgent {
		fb[k] = v
	}
	db := make(map[string]int64, len(m.dropByAgent))
	for k, v := range m.dropByAgent {
		db[k] = v
	}
	return Snapshot{
		Connected:     m.connected.Load(),
		HelloAccepts:  m.helloAccept.Load(),
		Frames:        m.frames.Load(),
		Drops:         m.drops.Load(),
		AuthRejects:   rejects,
		LastFrameUnix: last,
		FramesByAgent: fb,
		DropsByAgent:  db,
	}
}
