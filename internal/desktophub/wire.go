package desktophub

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gemvc/infrasible-monitoring-server/internal/protocol"
)

const (
	typeDesktopSubscribe  = "desktop_subscribe"
	typeSubscribeAccepted = "subscribe_accepted"
	typeStale             = "stale"
)

// Subscribe is the first Desktop→Relay application message (not Agent Hello).
type Subscribe struct {
	TypeVal       string   `json:"type"`
	ProtocolMajor int      `json:"protocol_major"`
	ProtocolMinor int      `json:"protocol_minor"`
	AgentIDs      []string `json:"agent_ids"`
	SentAtUTC     string   `json:"sent_at_utc"`
}

// SubscribeAccepted is Relay→Desktop session negotiate reply.
type SubscribeAccepted struct {
	TypeVal          string   `json:"type"`
	ProtocolMajor    int      `json:"protocol_major"`
	ProtocolMinor    int      `json:"protocol_minor"`
	EndpointID       string   `json:"endpoint_id"`
	HeartbeatSeconds int      `json:"heartbeat_seconds"`
	MaxMessageBytes  int      `json:"max_message_bytes"`
	AgentIDs         []string `json:"agent_ids"`
}

type wireEnvelope struct {
	Type string `json:"type"`
}

// DecodeSubscribe parses the first Desktop message.
func DecodeSubscribe(b []byte) (Subscribe, error) {
	var env wireEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		return Subscribe{}, fmt.Errorf("decode subscribe envelope: %w", err)
	}
	if env.Type != typeDesktopSubscribe {
		return Subscribe{}, fmt.Errorf("first message type %q not desktop_subscribe", env.Type)
	}
	var s Subscribe
	if err := json.Unmarshal(b, &s); err != nil {
		return Subscribe{}, fmt.Errorf("decode desktop_subscribe: %w", err)
	}
	s.TypeVal = typeDesktopSubscribe
	return s, nil
}

// ValidateSubscribe fail-closes on protocol and empty agent list.
func ValidateSubscribe(s Subscribe) error {
	if s.ProtocolMajor != protocol.ProtocolMajorRequired {
		return fmt.Errorf("protocol_major %d not %d", s.ProtocolMajor, protocol.ProtocolMajorRequired)
	}
	if len(s.AgentIDs) == 0 {
		return fmt.Errorf("agent_ids empty")
	}
	for _, id := range s.AgentIDs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("agent_ids contains empty id")
		}
	}
	if s.SentAtUTC != "" {
		if _, err := time.Parse(time.RFC3339, s.SentAtUTC); err != nil {
			return fmt.Errorf("sent_at_utc: %w", err)
		}
	}
	return nil
}

// EncodeSubscribeAccepted marshals a subscribe_accepted reply.
func EncodeSubscribeAccepted(a SubscribeAccepted) ([]byte, error) {
	if a.TypeVal == "" {
		a.TypeVal = typeSubscribeAccepted
	}
	return json.Marshal(a)
}

// NewSubscribeAccepted builds a protocol-correct accept.
func NewSubscribeAccepted(endpointID string, agentIDs []string, hb, maxBytes int) SubscribeAccepted {
	if hb <= 0 {
		hb = protocol.DefaultHeartbeatSeconds
	}
	if maxBytes <= 0 {
		maxBytes = protocol.DefaultMaxMessageBytes
	}
	ids := append([]string(nil), agentIDs...)
	return SubscribeAccepted{
		TypeVal:          typeSubscribeAccepted,
		ProtocolMajor:    protocol.ProtocolMajorRequired,
		ProtocolMinor:    0,
		EndpointID:       endpointID,
		HeartbeatSeconds: hb,
		MaxMessageBytes:  maxBytes,
		AgentIDs:         ids,
	}
}

// EncodeStale marshals a stale notice for one agent_id.
func EncodeStale(agentID string) ([]byte, error) {
	return json.Marshal(struct {
		Type    string `json:"type"`
		AgentID string `json:"agent_id"`
	}{
		Type:    typeStale,
		AgentID: agentID,
	})
}

// DecodePeerMessage accepts heartbeat only after subscribe (fail closed on other types).
func DecodePeerMessage(b []byte) (string, error) {
	var env wireEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		return "", err
	}
	switch env.Type {
	case protocol.TypeHeartbeat:
		return protocol.TypeHeartbeat, nil
	case "":
		return "", fmt.Errorf("missing message type")
	default:
		return "", fmt.Errorf("unexpected desktop message type %q", env.Type)
	}
}
