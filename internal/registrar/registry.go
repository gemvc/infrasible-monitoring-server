package registrar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

var (
	ErrUnknownIdentity = errors.New("unknown_identity")
	ErrChannelDenied   = errors.New("channel_denied")
	ErrAgentMismatch   = errors.New("agent_mismatch")
)

// ACLEntry is the authorized binding for a client certificate.
type ACLEntry struct {
	AgentID       string
	Channels      []string
	LiveEndpoints []string
}

// Registry looks up ACL by client identity.
type Registry interface {
	Lookup(ctx context.Context, id ClientIdentity) (ACLEntry, error)
}

// FileRegistry is a v1 JSON ACL loaded once at process start.
type FileRegistry struct {
	endpointID string
	byFP       map[string]ACLEntry
}

type aclFile struct {
	ACLVersion int    `json:"acl_version"`
	EndpointID string `json:"endpoint_id"`
	Agents     []struct {
		AgentID               string   `json:"agent_id"`
		CertFingerprintSHA256 string   `json:"cert_fingerprint_sha256"`
		Channels              []string `json:"channels"`
		LiveEndpoints         []string `json:"live_endpoints"`
		Comment               string   `json:"comment"`
	} `json:"agents"`
}

// NewFileRegistry loads and indexes the ACL file.
func NewFileRegistry(path string) (*FileRegistry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read acl: %w", err)
	}
	var f aclFile
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("parse acl: %w", err)
	}
	if f.ACLVersion != 1 {
		return nil, fmt.Errorf("acl_version %d not supported", f.ACLVersion)
	}
	if strings.TrimSpace(f.EndpointID) == "" {
		return nil, fmt.Errorf("acl endpoint_id empty")
	}
	byFP := make(map[string]ACLEntry, len(f.Agents))
	seenAgents := make(map[string]struct{}, len(f.Agents))
	for i, a := range f.Agents {
		if strings.TrimSpace(a.AgentID) == "" {
			return nil, fmt.Errorf("agents[%d]: agent_id empty", i)
		}
		fp := NormalizeFingerprint(a.CertFingerprintSHA256)
		if len(fp) != 64 {
			return nil, fmt.Errorf("agents[%d]: cert_fingerprint_sha256 must be 64 hex chars", i)
		}
		for _, c := range fp {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return nil, fmt.Errorf("agents[%d]: cert_fingerprint_sha256 not hex", i)
			}
		}
		if _, dup := byFP[fp]; dup {
			return nil, fmt.Errorf("duplicate fingerprint %s", fp)
		}
		if _, dup := seenAgents[a.AgentID]; dup {
			return nil, fmt.Errorf("duplicate agent_id %s", a.AgentID)
		}
		seenAgents[a.AgentID] = struct{}{}
		byFP[fp] = ACLEntry{
			AgentID:       a.AgentID,
			Channels:      append([]string(nil), a.Channels...),
			LiveEndpoints: append([]string(nil), a.LiveEndpoints...),
		}
	}
	return &FileRegistry{endpointID: f.EndpointID, byFP: byFP}, nil
}

// EndpointID returns the ACL file's endpoint_id.
func (f *FileRegistry) EndpointID() string { return f.endpointID }

// Lookup resolves fingerprint → ACL entry (registration only; AuthorizeLive for channel).
func (f *FileRegistry) Lookup(ctx context.Context, id ClientIdentity) (ACLEntry, error) {
	_ = ctx
	fp := NormalizeFingerprint(id.FingerprintSHA256)
	entry, ok := f.byFP[fp]
	if !ok {
		return ACLEntry{}, ErrUnknownIdentity
	}
	return entry, nil
}

// AuthorizeLive checks Hello agent_id, live channel, and this endpoint.
func AuthorizeLive(entry ACLEntry, agentIDFromHello, endpointID string) error {
	if agentIDFromHello != entry.AgentID {
		return ErrAgentMismatch
	}
	if !contains(entry.Channels, "live") {
		return ErrChannelDenied
	}
	if !contains(entry.LiveEndpoints, endpointID) {
		return ErrChannelDenied
	}
	return nil
}

// AuthorizeLive is a method alias for call sites that already hold *FileRegistry.
func (f *FileRegistry) AuthorizeLive(entry ACLEntry, agentIDFromHello, endpointID string) error {
	return AuthorizeLive(entry, agentIDFromHello, endpointID)
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
