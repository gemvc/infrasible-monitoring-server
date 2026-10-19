package fanout

// MultiHub fans PublishLive / MarkStale to every child hub.
// Used to keep LogHub observe behaviour while DesktopHub serves subscribers.
type MultiHub []SubscriberHub

// NewMultiHub returns a fan-out hub. Nil children are skipped.
func NewMultiHub(hubs ...SubscriberHub) SubscriberHub {
	out := make(MultiHub, 0, len(hubs))
	for _, h := range hubs {
		if h != nil {
			out = append(out, h)
		}
	}
	if len(out) == 0 {
		return NewNoopHub()
	}
	if len(out) == 1 {
		return out[0]
	}
	return out
}

func (m MultiHub) PublishLive(agentID string, frame []byte) {
	for _, h := range m {
		h.PublishLive(agentID, frame)
	}
}

func (m MultiHub) MarkStale(agentID string) {
	for _, h := range m {
		h.MarkStale(agentID)
	}
}
