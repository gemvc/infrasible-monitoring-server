package livesession

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gemvc/infrasible-monitoring-server/internal/fanout"
	"github.com/gemvc/infrasible-monitoring-server/internal/observe"
	"github.com/gemvc/infrasible-monitoring-server/internal/protocol"
	"github.com/gemvc/infrasible-monitoring-server/internal/registrar"
)

type memReg struct {
	entry registrar.ACLEntry
	err   error
}

func (m memReg) Lookup(context.Context, registrar.ClientIdentity) (registrar.ACLEntry, error) {
	return m.entry, m.err
}

type recordingHub struct {
	mu      sync.Mutex
	frames  []string
	stale   []string
	publish chan struct{}
}

func (h *recordingHub) PublishLive(agentID string, frame []byte) {
	h.mu.Lock()
	h.frames = append(h.frames, agentID+":"+string(frame))
	h.mu.Unlock()
	if h.publish != nil {
		select {
		case h.publish <- struct{}{}:
		default:
		}
	}
}

func (h *recordingHub) MarkStale(agentID string) {
	h.mu.Lock()
	h.stale = append(h.stale, agentID)
	h.mu.Unlock()
}

func helloJSON(agentID string) []byte {
	h := protocol.AgentHello{
		TypeVal:        protocol.TypeAgentHello,
		ProtocolMajor:  2,
		ProtocolMinor:  0,
		AgentID:        agentID,
		AgentVersion:   "0.4.1",
		Channel:        "live",
		SchemaVersions: []string{"2.0"},
		Capabilities:   []string{"live_1hz"},
		SentAtUTC:      "2026-10-18T12:00:00Z",
	}
	b, _ := json.Marshal(map[string]any{
		"type":            h.TypeVal,
		"protocol_major":  h.ProtocolMajor,
		"protocol_minor":  h.ProtocolMinor,
		"agent_id":        h.AgentID,
		"agent_version":   h.AgentVersion,
		"channel":         h.Channel,
		"schema_versions": h.SchemaVersions,
		"capabilities":    h.Capabilities,
		"sent_at_utc":     h.SentAtUTC,
	})
	return b
}

func TestAcceptanceR1_UnknownIdentity(t *testing.T) {
	conn := NewFakeConn(helloJSON("agt_1"))
	obs := observe.NewMetrics()
	hub := &recordingHub{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ServeAgent(context.Background(), conn, registrar.ClientIdentity{FingerprintSHA256: "aa"},
			memReg{err: registrar.ErrUnknownIdentity}, hub, obs, SessionConfig{EndpointID: "relay-primary"})
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	if obs.Snapshot().AuthRejects["unknown_identity"] < 1 {
		t.Fatalf("rejects=%v", obs.Snapshot().AuthRejects)
	}
	if len(hub.frames) != 0 {
		t.Fatal("must not ingest frames")
	}
}

func TestAcceptanceR2_HappyPath(t *testing.T) {
	agentID := "agt_01HEXAMPLE"
	live := []byte(`{"type":"live","agent_id":"agt_01HEXAMPLE","frame":{"observed_at_ms":1,"elapsed_ms":1000,"metrics":{}}}`)
	hb := []byte(`{"type":"heartbeat","sent_at_utc":"2026-10-18T12:00:01Z"}`)
	conn := NewFakeConn(helloJSON(agentID))
	obs := observe.NewMetrics()
	hub := &recordingHub{publish: make(chan struct{}, 1)}
	reg := memReg{entry: registrar.ACLEntry{
		AgentID: agentID, Channels: []string{"live"}, LiveEndpoints: []string{"relay-primary"},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ServeAgent(ctx, conn, registrar.ClientIdentity{FingerprintSHA256: "aa"},
			reg, hub, obs, SessionConfig{
				EndpointID: "relay-primary", HeartbeatSeconds: 15, MaxMessageBytes: 262144,
			})
	}()

	// wait for hello_accepted
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		out := conn.Outbox()
		if len(out) > 0 {
			var m map[string]any
			if json.Unmarshal(out[0], &m) == nil && m["type"] == "hello_accepted" {
				if m["authorized_agent_id"] != agentID {
					t.Fatalf("R3 echo fail: %v", m["authorized_agent_id"])
				}
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	conn.PushInbound(live)
	select {
	case <-hub.publish:
	case <-time.After(2 * time.Second):
		t.Fatal("no live publish")
	}
	conn.PushInbound(hb)
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done
	if obs.Snapshot().HelloAccepts < 1 || obs.Snapshot().Frames < 1 {
		t.Fatalf("snap=%+v", obs.Snapshot())
	}
	if len(hub.stale) != 1 || hub.stale[0] != agentID {
		t.Fatalf("stale=%v", hub.stale)
	}
}

func TestAcceptanceR5_DisconnectMarkStale(t *testing.T) {
	agentID := "agt_1"
	live := []byte(`{"type":"live","agent_id":"agt_1","frame":{"observed_at_ms":1,"elapsed_ms":1000,"metrics":{}}}`)
	conn := NewFakeConn(helloJSON(agentID))
	obs := observe.NewMetrics()
	hub := &recordingHub{publish: make(chan struct{}, 1)}
	reg := memReg{entry: registrar.ACLEntry{
		AgentID: agentID, Channels: []string{"live"}, LiveEndpoints: []string{"ep"},
	}}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ServeAgent(context.Background(), conn, registrar.ClientIdentity{FingerprintSHA256: "aa"},
			reg, hub, obs, SessionConfig{EndpointID: "ep", HeartbeatSeconds: 15, MaxMessageBytes: 262144})
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(conn.Outbox()) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	conn.PushInbound(live)
	select {
	case <-hub.publish:
	case <-time.After(2 * time.Second):
		t.Fatal("no live publish")
	}
	published := len(hub.frames)
	_ = conn.Close(StatusNormalClosure, "agent_gone")
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session hung after disconnect")
	}
	hub.mu.Lock()
	stale := append([]string(nil), hub.stale...)
	frames := len(hub.frames)
	hub.mu.Unlock()
	if len(stale) != 1 || stale[0] != agentID {
		t.Fatalf("MarkStale=%v", stale)
	}
	if frames != published {
		t.Fatalf("history replay from Relay not allowed: before=%d after=%d", published, frames)
	}
}

func TestAcceptanceR6_SlowHubOtherAgentsOK(t *testing.T) {
	obs := observe.NewMetrics()
	hub := fanout.NewBufferedHub(2, obs.IncDrops)

	agentA := "agt_a"
	agentB := "agt_b"
	liveA := func(n int) []byte {
		return []byte(`{"type":"live","agent_id":"agt_a","frame":{"observed_at_ms":` + strconv.Itoa(n) + `,"elapsed_ms":1000,"metrics":{}}}`)
	}
	liveB := []byte(`{"type":"live","agent_id":"agt_b","frame":{"observed_at_ms":1,"elapsed_ms":1000,"metrics":{}}}`)

	regA := memReg{entry: registrar.ACLEntry{
		AgentID: agentA, Channels: []string{"live"}, LiveEndpoints: []string{"ep"},
	}}
	regB := memReg{entry: registrar.ACLEntry{
		AgentID: agentB, Channels: []string{"live"}, LiveEndpoints: []string{"ep"},
	}}
	sessions := NewSessionMap()
	cfg := SessionConfig{EndpointID: "ep", HeartbeatSeconds: 15, MaxMessageBytes: 262144, Sessions: sessions}

	cA := NewFakeConn(helloJSON(agentA))
	cB := NewFakeConn(helloJSON(agentB))
	doneA := make(chan struct{})
	doneB := make(chan struct{})
	go func() {
		defer close(doneA)
		ServeAgent(context.Background(), cA, registrar.ClientIdentity{FingerprintSHA256: "aa"}, regA, hub, obs, cfg)
	}()
	go func() {
		defer close(doneB)
		ServeAgent(context.Background(), cB, registrar.ClientIdentity{FingerprintSHA256: "bb"}, regB, hub, obs, cfg)
	}()

	waitAccept := func(c *FakeConn) {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if len(c.Outbox()) > 0 {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatal("hello_accepted timeout")
	}
	waitAccept(cA)
	waitAccept(cB)

	cA.PushInbound(liveA(1))
	cA.PushInbound(liveA(2))
	cA.PushInbound(liveA(3)) // capacity 2 → one drop for A
	cB.PushInbound(liveB)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snap := obs.Snapshot()
		if snap.Drops >= 1 && snap.FramesByAgent[agentB] >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	snap := obs.Snapshot()
	if snap.Drops < 1 || snap.DropsByAgent[agentA] < 1 {
		t.Fatalf("expected drops for agt_a: %+v", snap)
	}
	if snap.DropsByAgent[agentB] != 0 {
		t.Fatalf("agt_b must be unaffected: %+v", snap)
	}
	if hub.Len(agentB) < 1 {
		t.Fatal("agt_b frame missing from hub")
	}

	_ = cA.Close(StatusNormalClosure, "done")
	_ = cB.Close(StatusNormalClosure, "done")
	<-doneA
	<-doneB
}

func TestAcceptanceD3_NoopHubHappyPath(t *testing.T) {
	agentID := "agt_noop"
	live := []byte(`{"type":"live","agent_id":"agt_noop","frame":{"observed_at_ms":1,"elapsed_ms":1000,"metrics":{}}}`)
	conn := NewFakeConn(helloJSON(agentID))
	obs := observe.NewMetrics()
	reg := memReg{entry: registrar.ACLEntry{
		AgentID: agentID, Channels: []string{"live"}, LiveEndpoints: []string{"ep"},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ServeAgent(ctx, conn, registrar.ClientIdentity{FingerprintSHA256: "aa"},
			reg, fanout.NewNoopHub(), obs, SessionConfig{
				EndpointID: "ep", HeartbeatSeconds: 15, MaxMessageBytes: 262144,
			})
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		out := conn.Outbox()
		if len(out) > 0 {
			var m map[string]any
			if json.Unmarshal(out[0], &m) == nil && m["type"] == "hello_accepted" {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	conn.PushInbound(live)
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if obs.Snapshot().Frames >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
	if obs.Snapshot().HelloAccepts < 1 || obs.Snapshot().Frames < 1 {
		t.Fatalf("noop hub must still pass R2: %+v", obs.Snapshot())
	}
}

func TestAcceptanceR4_FirstNotHello(t *testing.T) {
	live := []byte(`{"type":"live","agent_id":"agt_1","frame":{}}`)
	conn := NewFakeConn(live)
	obs := observe.NewMetrics()
	hub := &recordingHub{}
	reg := memReg{entry: registrar.ACLEntry{
		AgentID: "agt_1", Channels: []string{"live"}, LiveEndpoints: []string{"relay-primary"},
	}}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ServeAgent(context.Background(), conn, registrar.ClientIdentity{FingerprintSHA256: "aa"},
			reg, hub, obs, SessionConfig{EndpointID: "relay-primary"})
	}()
	<-done
	if len(hub.frames) != 0 {
		t.Fatal("frames processed")
	}
	if obs.Snapshot().AuthRejects["hello_invalid"] < 1 && obs.Snapshot().AuthRejects["first_not_hello"] < 1 {
		// Decode succeeds as live → first_not_hello path uses hello_invalid reason in code
		if conn.CloseReason != "first_not_hello" && conn.CloseReason != "hello_invalid" {
			t.Fatalf("reason=%s rejects=%v", conn.CloseReason, obs.Snapshot().AuthRejects)
		}
	}
}

func TestAcceptanceR7_ReplaceSession(t *testing.T) {
	agentID := "agt_1"
	reg := memReg{entry: registrar.ACLEntry{
		AgentID: agentID, Channels: []string{"live"}, LiveEndpoints: []string{"ep"},
	}}
	sessions := NewSessionMap()
	cfg := SessionConfig{EndpointID: "ep", HeartbeatSeconds: 15, Sessions: sessions}

	c1 := NewFakeConn(helloJSON(agentID))
	c2 := NewFakeConn(helloJSON(agentID))
	hub := &recordingHub{}
	obs := observe.NewMetrics()

	done1 := make(chan struct{})
	go func() {
		defer close(done1)
		ServeAgent(context.Background(), c1, registrar.ClientIdentity{FingerprintSHA256: "aa"}, reg, hub, obs, cfg)
	}()
	// wait accept
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(c1.Outbox()) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		ServeAgent(context.Background(), c2, registrar.ClientIdentity{FingerprintSHA256: "aa"}, reg, hub, obs, cfg)
	}()

	select {
	case <-done1:
	case <-time.After(2 * time.Second):
		t.Fatal("old session not replaced")
	}
	if !sessions.Active(agentID) {
		t.Fatal("expected active after replace")
	}
	// tear down second
	_ = c2.Close(StatusNormalClosure, "test")
	select {
	case <-done2:
	case <-time.After(2 * time.Second):
		t.Fatal("second session hung")
	}
}

func TestAcceptanceR8_Oversized(t *testing.T) {
	agentID := "agt_1"
	reg := memReg{entry: registrar.ACLEntry{
		AgentID: agentID, Channels: []string{"live"}, LiveEndpoints: []string{"ep"},
	}}
	conn := NewFakeConn(helloJSON(agentID))
	obs := observe.NewMetrics()
	hub := &recordingHub{publish: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ServeAgent(ctx, conn, registrar.ClientIdentity{FingerprintSHA256: "aa"}, reg, hub, obs,
			SessionConfig{EndpointID: "ep", MaxMessageBytes: 64, HeartbeatSeconds: 15})
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(conn.Outbox()) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	big := []byte(`{"type":"live","agent_id":"agt_1","frame":{"pad":"` + string(make([]byte, 200)) + `"}}`)
	for i := range big {
		if big[i] == 0 {
			big[i] = 'x'
		}
	}
	conn.PushInbound(big)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected close on oversized")
	}
	if conn.CloseReason != "oversized" {
		t.Fatalf("reason=%s", conn.CloseReason)
	}
}

func TestSessionMapBindUnbind(t *testing.T) {
	m := NewSessionMap()
	closed := false
	gen, replaced := m.Bind("a", func() { closed = true })
	if replaced {
		t.Fatal("first bind")
	}
	_, replaced = m.Bind("a", func() {})
	if !replaced || !closed {
		t.Fatalf("replaced=%v closed=%v", replaced, closed)
	}
	m.Unbind("a", gen) // stale gen — should not clear new
	if !m.Active("a") {
		t.Fatal("should still be active")
	}
}
