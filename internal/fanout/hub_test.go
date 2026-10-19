package fanout

import (
	"bytes"
	"log/slog"
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

func TestLogHubNoPanic(t *testing.T) {
	var buf bytes.Buffer
	h := NewLogHub(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	h.PublishLive("agt_1", []byte(`{"type":"live"}`))
	h.MarkStale("agt_1")
	NewNoopHub().PublishLive("x", nil)
	NewNoopHub().MarkStale("x")
}
