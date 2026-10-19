package livesession

import "sync"

// SessionMap tracks one active Live session per agent_id.
type SessionMap struct {
	mu      sync.Mutex
	cur     map[string]uint64
	closers map[string]func()
	next    uint64
}

// NewSessionMap creates an empty map.
func NewSessionMap() *SessionMap {
	return &SessionMap{
		cur:     make(map[string]uint64),
		closers: make(map[string]func()),
	}
}

// Bind registers a new generation for agentID. If another session was active,
// its closer is invoked (outside the lock) and replaced=true.
func (m *SessionMap) Bind(agentID string, closer func()) (gen uint64, replaced bool) {
	m.mu.Lock()
	m.next++
	gen = m.next
	oldCloser, had := m.closers[agentID]
	replaced = had
	m.cur[agentID] = gen
	m.closers[agentID] = closer
	m.mu.Unlock()

	if oldCloser != nil {
		oldCloser()
	}
	return gen, replaced
}

// Unbind clears the mapping only if gen is still the active generation.
func (m *SessionMap) Unbind(agentID string, gen uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cur[agentID] != gen {
		return
	}
	delete(m.cur, agentID)
	delete(m.closers, agentID)
}

// Active reports whether agentID currently has a bound session.
func (m *SessionMap) Active(agentID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.cur[agentID]
	return ok
}
