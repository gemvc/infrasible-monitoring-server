package desktophub

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/gemvc/infrasible-monitoring-server/internal/livesession"
	"github.com/gemvc/infrasible-monitoring-server/internal/observe"
	"github.com/gemvc/infrasible-monitoring-server/internal/registrar"
)

type memReg struct {
	endpoint string
	entry    Entry
	err      error
}

func (m memReg) Lookup(context.Context, registrar.ClientIdentity) (Entry, error) {
	if m.err != nil {
		return Entry{}, m.err
	}
	return m.entry, nil
}

func (m memReg) EndpointID() string { return m.endpoint }

type fakeConn struct {
	mu      sync.Mutex
	reads   [][]byte
	writes  [][]byte
	closed  bool
	readErr error
	block   chan struct{}
}

func (f *fakeConn) Read(ctx context.Context) ([]byte, error) {
	f.mu.Lock()
	if len(f.reads) > 0 {
		b := f.reads[0]
		f.reads = f.reads[1:]
		f.mu.Unlock()
		return b, nil
	}
	err := f.readErr
	block := f.block
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if block != nil {
		select {
		case <-block:
			return nil, context.Canceled
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func (f *fakeConn) Write(_ context.Context, b []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes = append(f.writes, append([]byte(nil), b...))
	return nil
}

func (f *fakeConn) Close(livesession.StatusCode, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeConn) writeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.writes)
}

func (f *fakeConn) firstType() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.writes) == 0 {
		return ""
	}
	var env struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(f.writes[0], &env)
	return env.Type
}

func TestServeDesktopHappyPathAndStale(t *testing.T) {
	hub := NewHub(4, nil)
	conn := &fakeConn{
		reads: [][]byte{
			[]byte(`{"type":"desktop_subscribe","protocol_major":2,"protocol_minor":0,"agent_ids":["agt_1"],"sent_at_utc":"2026-10-20T00:00:00Z"}`),
		},
		block: make(chan struct{}),
	}
	reg := memReg{
		endpoint: "relay-primary",
		entry:    Entry{OperatorID: "op_1", AllowedAgentIDs: []string{"agt_1"}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ServeDesktop(ctx, conn, registrar.ClientIdentity{FingerprintSHA256: "aa"}, reg, hub, observe.NewMetrics(), SessionConfig{
			EndpointID:       "relay-primary",
			HeartbeatSeconds: 15,
			MaxMessageBytes:  262144,
		})
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if conn.writeCount() >= 1 && conn.firstType() == typeSubscribeAccepted {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if conn.firstType() != typeSubscribeAccepted {
		t.Fatalf("first write type=%q writes=%d", conn.firstType(), conn.writeCount())
	}
	if hub.SubscriberCount() != 1 {
		t.Fatalf("subscribers=%d", hub.SubscriberCount())
	}

	live := []byte(`{"type":"live","agent_id":"agt_1","frame":{"observed_at_ms":1}}`)
	hub.PublishLive("agt_1", live)
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if conn.writeCount() >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	conn.mu.Lock()
	foundLive := false
	for _, w := range conn.writes {
		if string(w) == string(live) {
			foundLive = true
		}
	}
	conn.mu.Unlock()
	if !foundLive {
		t.Fatal("expected live frame forwarded to Desktop")
	}

	hub.MarkStale("agt_1")
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn.mu.Lock()
		for _, w := range conn.writes {
			var env struct {
				Type    string `json:"type"`
				AgentID string `json:"agent_id"`
			}
			_ = json.Unmarshal(w, &env)
			if env.Type == typeStale && env.AgentID == "agt_1" {
				conn.mu.Unlock()
				cancel()
				close(conn.block)
				<-done
				return
			}
		}
		conn.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	close(conn.block)
	<-done
	t.Fatal("expected stale notice")
}

func TestServeDesktopRejectsUnknownIdentity(t *testing.T) {
	hub := NewHub(2, nil)
	conn := &fakeConn{block: make(chan struct{})}
	reg := memReg{endpoint: "relay-primary", err: ErrUnknownDesktop}
	ServeDesktop(context.Background(), conn, registrar.ClientIdentity{FingerprintSHA256: "bb"}, reg, hub, observe.NewMetrics(), SessionConfig{
		EndpointID: "relay-primary",
	})
	if !conn.closed {
		t.Fatal("expected close")
	}
	if hub.SubscriberCount() != 0 {
		t.Fatal("must not attach")
	}
}

func TestServeDesktopRejectsDisallowedAgent(t *testing.T) {
	hub := NewHub(2, nil)
	conn := &fakeConn{
		reads: [][]byte{
			[]byte(`{"type":"desktop_subscribe","protocol_major":2,"agent_ids":["agt_other"]}`),
		},
	}
	reg := memReg{
		endpoint: "relay-primary",
		entry:    Entry{OperatorID: "op_1", AllowedAgentIDs: []string{"agt_1"}},
	}
	ServeDesktop(context.Background(), conn, registrar.ClientIdentity{FingerprintSHA256: "cc"}, reg, hub, observe.NewMetrics(), SessionConfig{
		EndpointID: "relay-primary",
	})
	if hub.SubscriberCount() != 0 {
		t.Fatal("must not attach disallowed agent")
	}
}
