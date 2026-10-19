package desktophub

import (
	"sync"

	"github.com/gemvc/infrasible-monitoring-server/internal/fanout"
)

// event is a non-blocking fan-out unit for one subscriber.
type event struct {
	kind    string // "live" or "stale"
	agentID string
	frame   []byte // raw Agent live envelope bytes for kind=live
}

// subscriber is one Desktop WS session.
type subscriber struct {
	id     string
	agents map[string]struct{}
	ch     chan event
	cap    int
	drops  int
	mu     sync.Mutex
}

// Hub implements fanout.SubscriberHub and owns Desktop subscriber sessions.
// PublishLive / MarkStale never block: each subscriber uses a drop-oldest queue
// (same R6 contract as fanout.BufferedHub).
type Hub struct {
	queueCap int
	onDrop   fanout.DropFunc

	mu      sync.Mutex
	subs    map[string]*subscriber
	byAgent map[string]map[string]*subscriber
	seq     uint64
}

// NewHub creates a Desktop fan-out hub. queueCap defaults to 8.
func NewHub(queueCap int, onDrop fanout.DropFunc) *Hub {
	if queueCap < 1 {
		queueCap = 8
	}
	return &Hub{
		queueCap: queueCap,
		onDrop:   onDrop,
		subs:     make(map[string]*subscriber),
		byAgent:  make(map[string]map[string]*subscriber),
	}
}

// PublishLive fans the raw Agent live envelope to subscribers of agentID.
func (h *Hub) PublishLive(agentID string, frame []byte) {
	h.mu.Lock()
	set := h.byAgent[agentID]
	targets := make([]*subscriber, 0, len(set))
	for _, s := range set {
		targets = append(targets, s)
	}
	h.mu.Unlock()

	cp := append([]byte(nil), frame...)
	for _, s := range targets {
		if s.push(event{kind: "live", agentID: agentID, frame: cp}) && h.onDrop != nil {
			h.onDrop(agentID)
		}
	}
}

// MarkStale notifies subscribers that agentID's Live session ended.
func (h *Hub) MarkStale(agentID string) {
	h.mu.Lock()
	set := h.byAgent[agentID]
	targets := make([]*subscriber, 0, len(set))
	for _, s := range set {
		targets = append(targets, s)
	}
	h.mu.Unlock()

	for _, s := range targets {
		_ = s.push(event{kind: "stale", agentID: agentID})
	}
}

// Attach registers a subscriber for the given agent IDs. Returns an id and event channel.
func (h *Hub) Attach(agentIDs []string) (id string, events <-chan event, detach func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq++
	id = formatSubID(h.seq)
	s := &subscriber{
		id:     id,
		agents: make(map[string]struct{}, len(agentIDs)),
		ch:     make(chan event, h.queueCap),
		cap:    h.queueCap,
	}
	for _, a := range agentIDs {
		s.agents[a] = struct{}{}
		set := h.byAgent[a]
		if set == nil {
			set = make(map[string]*subscriber)
			h.byAgent[a] = set
		}
		set[id] = s
	}
	h.subs[id] = s
	return id, s.ch, func() { h.Detach(id) }
}

// Detach removes a subscriber and closes its event channel.
func (h *Hub) Detach(id string) {
	h.mu.Lock()
	s, ok := h.subs[id]
	if !ok {
		h.mu.Unlock()
		return
	}
	delete(h.subs, id)
	for a := range s.agents {
		if set := h.byAgent[a]; set != nil {
			delete(set, id)
			if len(set) == 0 {
				delete(h.byAgent, a)
			}
		}
	}
	h.mu.Unlock()
	close(s.ch)
}

// SubscriberCount returns active Desktop sessions (observe/test helper).
func (h *Hub) SubscriberCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

// Drops returns cumulative drop-oldest count for one subscriber (test helper).
func (h *Hub) Drops(id string) int {
	h.mu.Lock()
	s := h.subs[id]
	h.mu.Unlock()
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.drops
}

func (s *subscriber) push(ev event) (dropped bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case s.ch <- ev:
		return false
	default:
		// Drop oldest then push newest (R6).
		select {
		case <-s.ch:
			s.drops++
			dropped = true
		default:
		}
		select {
		case s.ch <- ev:
		default:
			// Still full (race with reader); count as drop and give up.
			s.drops++
			dropped = true
		}
		return dropped
	}
}

func formatSubID(n uint64) string {
	const hexdigits = "0123456789abcdef"
	var b [16]byte
	for i := 15; i >= 0; i-- {
		b[i] = hexdigits[n&0xf]
		n >>= 4
	}
	return "desk_" + string(b[:])
}

// Ensure Hub satisfies SubscriberHub at compile time.
var _ fanout.SubscriberHub = (*Hub)(nil)
