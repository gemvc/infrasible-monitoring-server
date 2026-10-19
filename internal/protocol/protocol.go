package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	TypeAgentHello    = "agent_hello"
	TypeHelloAccepted = "hello_accepted"
	TypeLive          = "live"
	TypeHeartbeat     = "heartbeat"

	ProtocolMajorRequired = 2
	SchemaVersionAccepted = "2.0"
	ChannelLive           = "live"

	DefaultHeartbeatSeconds = 15
	DefaultMaxMessageBytes  = 262144
	MaxHeartbeatSeconds     = 300
)

// Message is a decoded Agent→Relay application message.
type Message interface {
	Type() string
}

// AgentHello is the first application message on a Live session.
type AgentHello struct {
	TypeVal        string   `json:"type"`
	ProtocolMajor  int      `json:"protocol_major"`
	ProtocolMinor  int      `json:"protocol_minor"`
	AgentID        string   `json:"agent_id"`
	AgentVersion   string   `json:"agent_version"`
	Channel        string   `json:"channel"`
	SchemaVersions []string `json:"schema_versions"`
	Capabilities   []string `json:"capabilities"`
	SentAtUTC      string   `json:"sent_at_utc"`
}

func (h AgentHello) Type() string { return TypeAgentHello }

// HelloAccepted is Relay→Agent session negotiate reply.
type HelloAccepted struct {
	TypeVal           string `json:"type"`
	ProtocolMajor     int    `json:"protocol_major"`
	ProtocolMinor     int    `json:"protocol_minor"`
	SchemaVersion     string `json:"schema_version"`
	AuthorizedAgentID string `json:"authorized_agent_id"`
	AuthorizedChannel string `json:"authorized_channel"`
	EndpointID        string `json:"endpoint_id"`
	HeartbeatSeconds  int    `json:"heartbeat_seconds"`
	MaxMessageBytes   int    `json:"max_message_bytes"`
}

// LiveEnvelope carries an opaque live frame after Hello.
type LiveEnvelope struct {
	TypeVal string          `json:"type"`
	AgentID string          `json:"agent_id"`
	Frame   json.RawMessage `json:"frame"`
}

func (l LiveEnvelope) Type() string { return TypeLive }

// Heartbeat is application-layer liveness (both directions).
type Heartbeat struct {
	TypeVal   string `json:"type"`
	SentAtUTC string `json:"sent_at_utc"`
}

func (h Heartbeat) Type() string { return TypeHeartbeat }

type envelope struct {
	Type string `json:"type"`
}

// DecodeAgentMessage dispatches on JSON "type". Unknown types return an error.
func DecodeAgentMessage(b []byte) (Message, error) {
	var env envelope
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}
	switch env.Type {
	case TypeAgentHello:
		var h AgentHello
		if err := json.Unmarshal(b, &h); err != nil {
			return nil, fmt.Errorf("decode agent_hello: %w", err)
		}
		h.TypeVal = TypeAgentHello
		return h, nil
	case TypeLive:
		var l LiveEnvelope
		if err := json.Unmarshal(b, &l); err != nil {
			return nil, fmt.Errorf("decode live: %w", err)
		}
		l.TypeVal = TypeLive
		return l, nil
	case TypeHeartbeat:
		var hb Heartbeat
		if err := json.Unmarshal(b, &hb); err != nil {
			return nil, fmt.Errorf("decode heartbeat: %w", err)
		}
		hb.TypeVal = TypeHeartbeat
		return hb, nil
	case "":
		return nil, fmt.Errorf("missing message type")
	default:
		return nil, fmt.Errorf("unknown message type %q", env.Type)
	}
}

// EncodeHelloAccepted marshals a hello_accepted reply.
func EncodeHelloAccepted(h HelloAccepted) ([]byte, error) {
	if h.TypeVal == "" {
		h.TypeVal = TypeHelloAccepted
	}
	return json.Marshal(struct {
		Type              string `json:"type"`
		ProtocolMajor     int    `json:"protocol_major"`
		ProtocolMinor     int    `json:"protocol_minor"`
		SchemaVersion     string `json:"schema_version"`
		AuthorizedAgentID string `json:"authorized_agent_id"`
		AuthorizedChannel string `json:"authorized_channel"`
		EndpointID        string `json:"endpoint_id"`
		HeartbeatSeconds  int    `json:"heartbeat_seconds"`
		MaxMessageBytes   int    `json:"max_message_bytes"`
	}{
		Type:              h.TypeVal,
		ProtocolMajor:     h.ProtocolMajor,
		ProtocolMinor:     h.ProtocolMinor,
		SchemaVersion:     h.SchemaVersion,
		AuthorizedAgentID: h.AuthorizedAgentID,
		AuthorizedChannel: h.AuthorizedChannel,
		EndpointID:        h.EndpointID,
		HeartbeatSeconds:  h.HeartbeatSeconds,
		MaxMessageBytes:   h.MaxMessageBytes,
	})
}

// EncodeHeartbeat marshals a heartbeat with UTC RFC3339 sent_at_utc.
func EncodeHeartbeat(sentAt time.Time) ([]byte, error) {
	return json.Marshal(struct {
		Type      string `json:"type"`
		SentAtUTC string `json:"sent_at_utc"`
	}{
		Type:      TypeHeartbeat,
		SentAtUTC: sentAt.UTC().Format(time.RFC3339),
	})
}

// NewHelloAccepted builds a protocol-correct accept with defaults.
func NewHelloAccepted(agentID, endpointID string, hb, maxBytes int) HelloAccepted {
	if hb <= 0 {
		hb = DefaultHeartbeatSeconds
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxMessageBytes
	}
	return HelloAccepted{
		TypeVal:           TypeHelloAccepted,
		ProtocolMajor:     ProtocolMajorRequired,
		ProtocolMinor:     0,
		SchemaVersion:     SchemaVersionAccepted,
		AuthorizedAgentID: agentID,
		AuthorizedChannel: ChannelLive,
		EndpointID:        endpointID,
		HeartbeatSeconds:  hb,
		MaxMessageBytes:   maxBytes,
	}
}

// ValidateAgentHello fail-closes on protocol, channel, schema, and identity.
func ValidateAgentHello(h AgentHello, expectAgentID string) error {
	if h.TypeVal != "" && h.TypeVal != TypeAgentHello {
		return fmt.Errorf("type %q not agent_hello", h.TypeVal)
	}
	if h.ProtocolMajor != ProtocolMajorRequired {
		return fmt.Errorf("protocol_major %d not %d", h.ProtocolMajor, ProtocolMajorRequired)
	}
	if h.Channel != ChannelLive {
		return fmt.Errorf("channel %q not live", h.Channel)
	}
	if strings.TrimSpace(h.AgentID) == "" {
		return fmt.Errorf("agent_id empty")
	}
	if expectAgentID != "" && h.AgentID != expectAgentID {
		return fmt.Errorf("agent_id mismatch")
	}
	if strings.TrimSpace(h.AgentVersion) == "" {
		return fmt.Errorf("agent_version empty")
	}
	if !containsString(h.SchemaVersions, SchemaVersionAccepted) {
		return fmt.Errorf("schema_versions missing %s", SchemaVersionAccepted)
	}
	if h.SentAtUTC != "" {
		if _, err := time.Parse(time.RFC3339, h.SentAtUTC); err != nil {
			return fmt.Errorf("sent_at_utc: %w", err)
		}
	}
	return nil
}

func containsString(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
