package fanout

import (
	"bytes"
	"log/slog"
	"sync/atomic"
	"testing"
)

func TestDropOldestQueue(t *testing.T) {
	q := NewDropOldestQueue(2)
	if q.Push([]byte("a")) {
		t.Fatal("first should not drop")
	}
	if q.Push([]byte("b")) {
		t.Fatal("second should not drop")
	}
	if !q.Push([]byte("c")) {
		t.Fatal("third should drop oldest")
	}
	if q.Drops() != 1 || q.Len() != 2 {
		t.Fatalf("drops=%d len=%d", q.Drops(), q.Len())
	}
	if string(q.Pop()) != "b" || string(q.Pop()) != "c" {
		t.Fatal("order wrong after drop")
	}
}

func TestAcceptanceR6_BufferedHubIsolation(t *testing.T) {
	var dropsA, dropsB atomic.Int64
	h := NewBufferedHub(2, func(agentID string) {
		switch agentID {
		case "agt_a":
			dropsA.Add(1)
		case "agt_b":
			dropsB.Add(1)
		}
	})
	h.PublishLive("agt_a", []byte("a1"))
	h.PublishLive("agt_a", []byte("a2"))
	h.PublishLive("agt_a", []byte("a3")) // drop a1
	h.PublishLive("agt_b", []byte("b1"))
	h.PublishLive("agt_b", []byte("b2"))

	if dropsA.Load() != 1 {
		t.Fatalf("agt_a drops=%d", dropsA.Load())
	}
	if dropsB.Load() != 0 {
		t.Fatalf("agt_b drops=%d (other agent must be unaffected)", dropsB.Load())
	}
	if h.Len("agt_a") != 2 || h.Drops("agt_a") != 1 {
		t.Fatalf("agt_a len=%d drops=%d", h.Len("agt_a"), h.Drops("agt_a"))
	}
	if h.Len("agt_b") != 2 || h.Drops("agt_b") != 0 {
		t.Fatalf("agt_b len=%d drops=%d", h.Len("agt_b"), h.Drops("agt_b"))
	}
	if string(h.Pop("agt_a")) != "a2" || string(h.Pop("agt_a")) != "a3" {
		t.Fatal("agt_a order after drop-oldest")
	}
}

func TestLogHubNoPanic(t *testing.T) {
	var buf bytes.Buffer
	h := NewLogHub(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	h.PublishLive("agt_1", []byte(`{"type":"live"}`))
	h.MarkStale("agt_1")
	if !bytes.Contains(buf.Bytes(), []byte("hub.publish_live")) {
		t.Fatal("D1: expected publish log")
	}
	if !bytes.Contains(buf.Bytes(), []byte("hub.mark_stale")) {
		t.Fatal("D2: expected mark_stale log")
	}
	NewNoopHub().PublishLive("x", nil)
	NewNoopHub().MarkStale("x")
}
