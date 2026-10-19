package desktophub

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestHubFanoutAndStale(t *testing.T) {
	h := NewHub(4, nil)
	_, ch1, detach1 := h.Attach([]string{"agt_a", "agt_b"})
	defer detach1()
	_, ch2, detach2 := h.Attach([]string{"agt_a"})
	defer detach2()

	h.PublishLive("agt_a", []byte(`{"type":"live","agent_id":"agt_a","frame":{}}`))
	h.PublishLive("agt_b", []byte(`{"type":"live","agent_id":"agt_b","frame":{}}`))

	gotA1 := false
	gotB1 := false
	gotA2 := false
	deadline := time.After(2 * time.Second)
	for !(gotA1 && gotB1 && gotA2) {
		select {
		case ev := <-ch1:
			if ev.kind != "live" {
				t.Fatalf("ch1 kind=%s", ev.kind)
			}
			if ev.agentID == "agt_a" {
				gotA1 = true
			}
			if ev.agentID == "agt_b" {
				gotB1 = true
			}
		case ev := <-ch2:
			if ev.agentID != "agt_a" || ev.kind != "live" {
				t.Fatalf("ch2 unexpected %+v", ev)
			}
			gotA2 = true
		case <-deadline:
			t.Fatalf("timeout a1=%v b1=%v a2=%v", gotA1, gotB1, gotA2)
		}
	}

	// ch2 must not see agt_b
	select {
	case ev := <-ch2:
		t.Fatalf("ch2 leaked agt_b event: %+v", ev)
	default:
	}

	h.MarkStale("agt_a")
	select {
	case ev := <-ch1:
		if ev.kind != "stale" || ev.agentID != "agt_a" {
			t.Fatalf("stale ch1=%+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("expected stale on ch1")
	}
	select {
	case ev := <-ch2:
		if ev.kind != "stale" || ev.agentID != "agt_a" {
			t.Fatalf("stale ch2=%+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("expected stale on ch2")
	}
}

func TestHubDropOldest(t *testing.T) {
	var drops atomic.Int64
	h := NewHub(2, func(agentID string) {
		if agentID == "agt_x" {
			drops.Add(1)
		}
	})
	id, ch, detach := h.Attach([]string{"agt_x"})
	defer detach()

	h.PublishLive("agt_x", []byte("1"))
	h.PublishLive("agt_x", []byte("2"))
	h.PublishLive("agt_x", []byte("3")) // drop oldest

	if drops.Load() < 1 {
		t.Fatalf("expected drop callback, drops=%d", drops.Load())
	}
	if h.Drops(id) < 1 {
		t.Fatalf("subscriber drops=%d", h.Drops(id))
	}

	first := <-ch
	second := <-ch
	if string(first.frame) == "1" {
		t.Fatal("oldest frame 1 should have been dropped")
	}
	if string(first.frame) != "2" || string(second.frame) != "3" {
		t.Fatalf("got %q %q want 2 then 3", first.frame, second.frame)
	}
}

func TestHubDetach(t *testing.T) {
	h := NewHub(2, nil)
	id, ch, detach := h.Attach([]string{"agt_z"})
	if h.SubscriberCount() != 1 {
		t.Fatalf("count=%d", h.SubscriberCount())
	}
	detach()
	if h.SubscriberCount() != 0 {
		t.Fatalf("count after detach=%d", h.SubscriberCount())
	}
	_, open := <-ch
	if open {
		t.Fatal("channel should be closed")
	}
	_ = id
	// Publish after detach must not panic.
	h.PublishLive("agt_z", []byte("x"))
	h.MarkStale("agt_z")
}
