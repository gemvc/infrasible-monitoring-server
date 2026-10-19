package observe

import (
	"testing"
	"time"
)

func TestMetricsSnapshotCounters(t *testing.T) {
	m := NewMetrics()
	m.IncConnected(1)
	m.IncHelloAccept()
	m.IncAuthReject("unknown_identity")
	m.IncAuthReject("")
	m.IncFrames("agt_1")
	m.IncDrops("agt_1")
	m.SetLastFrame("agt_1", time.Unix(1_700_000_000, 0))

	s := m.Snapshot()
	if s.Connected != 1 || s.HelloAccepts != 1 || s.Frames != 1 || s.Drops != 1 {
		t.Fatalf("counters=%+v", s)
	}
	if s.AuthRejects["unknown_identity"] != 1 || s.AuthRejects["unknown"] != 1 {
		t.Fatalf("rejects=%v", s.AuthRejects)
	}
	if s.FramesByAgent["agt_1"] != 1 || s.DropsByAgent["agt_1"] != 1 {
		t.Fatalf("by-agent frames=%v drops=%v", s.FramesByAgent, s.DropsByAgent)
	}
	if s.LastFrameUnix["agt_1"] != 1_700_000_000 {
		t.Fatalf("lastFrame=%v", s.LastFrameUnix)
	}
}

func TestSnapshotCopiesMaps(t *testing.T) {
	m := NewMetrics()
	m.IncAuthReject("x")
	s1 := m.Snapshot()
	s1.AuthRejects["x"] = 99
	s2 := m.Snapshot()
	if s2.AuthRejects["x"] != 1 {
		t.Fatalf("snapshot must not share map: %v", s2.AuthRejects)
	}
}
