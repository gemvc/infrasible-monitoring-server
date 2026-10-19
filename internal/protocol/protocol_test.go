package protocol

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDecodeAgentHelloGolden(t *testing.T) {
	raw := `{
  "type": "agent_hello",
  "protocol_major": 2,
  "protocol_minor": 0,
  "agent_id": "agt_01HXYZ",
  "agent_version": "0.4.1",
  "channel": "live",
  "schema_versions": ["2.0"],
  "capabilities": ["live_1hz", "sample_quality", "metric_cadence_v1", "metric_freshness_v1"],
  "sent_at_utc": "2026-10-18T12:00:00Z"
}`
	msg, err := DecodeAgentMessage([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	h, ok := msg.(AgentHello)
	if !ok {
		t.Fatalf("got %T", msg)
	}
	if err := ValidateAgentHello(h, "agt_01HXYZ"); err != nil {
		t.Fatal(err)
	}
	if h.AgentVersion != "0.4.1" || h.ProtocolMinor != 0 {
		t.Fatalf("unexpected hello: %+v", h)
	}
}

func TestEncodeHelloAcceptedGolden(t *testing.T) {
	h := NewHelloAccepted("agt_01HXYZ", "relay-primary", 15, 262144)
	b, err := EncodeHelloAccepted(h)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["type"] != "hello_accepted" {
		t.Fatalf("type=%v", m["type"])
	}
	if m["authorized_agent_id"] != "agt_01HXYZ" {
		t.Fatalf("authorized_agent_id=%v", m["authorized_agent_id"])
	}
	if m["authorized_channel"] != "live" {
		t.Fatalf("authorized_channel=%v", m["authorized_channel"])
	}
	if int(m["heartbeat_seconds"].(float64)) != 15 {
		t.Fatalf("heartbeat_seconds=%v", m["heartbeat_seconds"])
	}
	if int(m["max_message_bytes"].(float64)) != 262144 {
		t.Fatalf("max_message_bytes=%v", m["max_message_bytes"])
	}
	if int(m["protocol_major"].(float64)) != 2 {
		t.Fatalf("protocol_major=%v", m["protocol_major"])
	}
}

func TestDecodeLiveAndHeartbeat(t *testing.T) {
	liveRaw := `{"type":"live","agent_id":"agt_01HXYZ","frame":{"observed_at_ms":1,"elapsed_ms":1000,"metrics":{}}}`
	msg, err := DecodeAgentMessage([]byte(liveRaw))
	if err != nil {
		t.Fatal(err)
	}
	l, ok := msg.(LiveEnvelope)
	if !ok || l.AgentID != "agt_01HXYZ" || len(l.Frame) == 0 {
		t.Fatalf("live=%+v", l)
	}

	hbRaw := `{"type":"heartbeat","sent_at_utc":"2026-10-18T12:00:00Z"}`
	msg, err = DecodeAgentMessage([]byte(hbRaw))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := msg.(Heartbeat); !ok {
		t.Fatalf("got %T", msg)
	}
}

func TestDecodeUnknownType(t *testing.T) {
	_, err := DecodeAgentMessage([]byte(`{"type":"live_sample"}`))
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidateAgentHelloTable(t *testing.T) {
	base := AgentHello{
		TypeVal:        TypeAgentHello,
		ProtocolMajor:  2,
		ProtocolMinor:  0,
		AgentID:        "agt_1",
		AgentVersion:   "0.1.0",
		Channel:        "live",
		SchemaVersions: []string{"2.0"},
		SentAtUTC:      "2026-10-18T12:00:00Z",
	}
	cases := []struct {
		name    string
		mutate  func(*AgentHello)
		expect  string
		wantErr bool
	}{
		{"ok", func(*AgentHello) {}, "agt_1", false},
		{"wrong major", func(h *AgentHello) { h.ProtocolMajor = 1 }, "agt_1", true},
		{"wrong channel", func(h *AgentHello) { h.Channel = "persistence" }, "agt_1", true},
		{"empty id", func(h *AgentHello) { h.AgentID = "" }, "agt_1", true},
		{"id mismatch", func(*AgentHello) {}, "other", true},
		{"no schema", func(h *AgentHello) { h.SchemaVersions = []string{"1.0"} }, "agt_1", true},
		{"bad time", func(h *AgentHello) { h.SentAtUTC = "not-a-time" }, "agt_1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := base
			tc.mutate(&h)
			err := ValidateAgentHello(h, tc.expect)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEncodeHeartbeatUTC(t *testing.T) {
	ts := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	b, err := EncodeHeartbeat(ts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"type":"heartbeat"`) {
		t.Fatalf("%s", b)
	}
	if !strings.Contains(string(b), "2026-10-18T12:00:00Z") {
		t.Fatalf("%s", b)
	}
}

func TestAuthorizedAgentIDExactEcho(t *testing.T) {
	// R3: never emit wrong authorized_agent_id
	h := NewHelloAccepted("exact-id", "ep", 15, 262144)
	if h.AuthorizedAgentID != "exact-id" {
		t.Fatal(h.AuthorizedAgentID)
	}
	b, _ := EncodeHelloAccepted(h)
	var got HelloAccepted
	_ = json.Unmarshal(b, &got)
	// json uses Type field name differently — check via map
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m["authorized_agent_id"] != "exact-id" {
		t.Fatalf("%v", m["authorized_agent_id"])
	}
}
