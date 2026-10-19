package fanout

import "testing"

type recHub struct {
	live  []string
	stale []string
}

func (r *recHub) PublishLive(agentID string, frame []byte) {
	r.live = append(r.live, agentID+":"+string(frame))
}

func (r *recHub) MarkStale(agentID string) {
	r.stale = append(r.stale, agentID)
}

func TestMultiHub(t *testing.T) {
	a, b := &recHub{}, &recHub{}
	h := NewMultiHub(a, b)
	h.PublishLive("agt", []byte("f"))
	h.MarkStale("agt")
	if len(a.live) != 1 || len(b.live) != 1 {
		t.Fatalf("live a=%v b=%v", a.live, b.live)
	}
	if len(a.stale) != 1 || len(b.stale) != 1 {
		t.Fatalf("stale a=%v b=%v", a.stale, b.stale)
	}
}
