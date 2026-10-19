package desktophub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gemvc/infrasible-monitoring-server/internal/registrar"
)

var (
	// ErrUnknownDesktop is returned when the client cert is not in the Desktop ACL.
	ErrUnknownDesktop = errors.New("unknown_desktop")
	// ErrAgentNotAllowed is returned when a subscribe request names an unauthorized agent_id.
	ErrAgentNotAllowed = errors.New("agent_not_allowed")
)

// Entry is one Desktop operator binding (distinct from Agent ACL rows).
type Entry struct {
	OperatorID      string
	AllowedAgentIDs []string
}

// Registry looks up Desktop clients by certificate fingerprint.
type Registry interface {
	Lookup(ctx context.Context, id registrar.ClientIdentity) (Entry, error)
	EndpointID() string
}

// FileRegistry is a v1 JSON Desktop ACL loaded once at process start.
type FileRegistry struct {
	endpointID string
	byFP       map[string]Entry
}

type desktopACLFile struct {
	ACLVersion int    `json:"acl_version"`
	EndpointID string `json:"endpoint_id"`
	Desktops   []struct {
		OperatorID            string   `json:"operator_id"`
		CertFingerprintSHA256 string   `json:"cert_fingerprint_sha256"`
		AllowedAgentIDs       []string `json:"allowed_agent_ids"`
		Comment               string   `json:"comment"`
	} `json:"desktops"`
}

// NewFileRegistry loads and indexes the Desktop ACL file.
func NewFileRegistry(path string) (*FileRegistry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read desktop acl: %w", err)
	}
	var f desktopACLFile
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("parse desktop acl: %w", err)
	}
	if f.ACLVersion != 1 {
		return nil, fmt.Errorf("desktop acl_version %d not supported", f.ACLVersion)
	}
	if strings.TrimSpace(f.EndpointID) == "" {
		return nil, fmt.Errorf("desktop acl endpoint_id empty")
	}
	byFP := make(map[string]Entry, len(f.Desktops))
	for i, d := range f.Desktops {
		if strings.TrimSpace(d.OperatorID) == "" {
			return nil, fmt.Errorf("desktops[%d]: operator_id empty", i)
		}
		fp := registrar.NormalizeFingerprint(d.CertFingerprintSHA256)
		if len(fp) != 64 {
			return nil, fmt.Errorf("desktops[%d]: cert_fingerprint_sha256 must be 64 hex chars", i)
		}
		for _, c := range fp {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return nil, fmt.Errorf("desktops[%d]: cert_fingerprint_sha256 not hex", i)
			}
		}
		if _, dup := byFP[fp]; dup {
			return nil, fmt.Errorf("duplicate desktop fingerprint %s", fp)
		}
		allowed := make([]string, 0, len(d.AllowedAgentIDs))
		seen := make(map[string]struct{}, len(d.AllowedAgentIDs))
		for _, aid := range d.AllowedAgentIDs {
			aid = strings.TrimSpace(aid)
			if aid == "" {
				continue
			}
			if _, ok := seen[aid]; ok {
				continue
			}
			seen[aid] = struct{}{}
			allowed = append(allowed, aid)
		}
		if len(allowed) == 0 {
			return nil, fmt.Errorf("desktops[%d]: allowed_agent_ids empty", i)
		}
		byFP[fp] = Entry{OperatorID: d.OperatorID, AllowedAgentIDs: allowed}
	}
	return &FileRegistry{endpointID: f.EndpointID, byFP: byFP}, nil
}

// EndpointID returns the Desktop ACL file's endpoint_id.
func (f *FileRegistry) EndpointID() string { return f.endpointID }

// Lookup resolves fingerprint → Desktop entry.
func (f *FileRegistry) Lookup(ctx context.Context, id registrar.ClientIdentity) (Entry, error) {
	_ = ctx
	fp := registrar.NormalizeFingerprint(id.FingerprintSHA256)
	entry, ok := f.byFP[fp]
	if !ok {
		return Entry{}, ErrUnknownDesktop
	}
	return entry, nil
}

// AuthorizeSubscribe checks requested agent IDs against the Desktop allowlist.
func AuthorizeSubscribe(entry Entry, agentIDs []string) error {
	if len(agentIDs) == 0 {
		return ErrAgentNotAllowed
	}
	allowed := make(map[string]struct{}, len(entry.AllowedAgentIDs))
	for _, a := range entry.AllowedAgentIDs {
		allowed[a] = struct{}{}
	}
	for _, want := range agentIDs {
		want = strings.TrimSpace(want)
		if want == "" {
			return ErrAgentNotAllowed
		}
		if _, ok := allowed[want]; !ok {
			return ErrAgentNotAllowed
		}
	}
	return nil
}
