package desktophub

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gemvc/infrasible-monitoring-server/internal/registrar"
)

func TestFileRegistryLookupAndAuthorize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "desktop-acl.json")
	body := `{
  "acl_version": 1,
  "endpoint_id": "relay-primary",
  "desktops": [
    {
      "operator_id": "op_lab",
      "cert_fingerprint_sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "allowed_agent_ids": ["agt_lab_01", "agt_lab_02"],
      "comment": "lab"
    }
  ]
}`
	if err := os.WriteFile(path, []byte(body), 0o640); err != nil {
		t.Fatal(err)
	}
	reg, err := NewFileRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if reg.EndpointID() != "relay-primary" {
		t.Fatalf("endpoint=%s", reg.EndpointID())
	}
	entry, err := reg.Lookup(context.Background(), registrar.ClientIdentity{
		FingerprintSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if err != nil {
		t.Fatal(err)
	}
	if entry.OperatorID != "op_lab" {
		t.Fatalf("operator=%s", entry.OperatorID)
	}
	if err := AuthorizeSubscribe(entry, []string{"agt_lab_01"}); err != nil {
		t.Fatal(err)
	}
	if err := AuthorizeSubscribe(entry, []string{"agt_unknown"}); err == nil {
		t.Fatal("expected deny")
	}
	_, err = reg.Lookup(context.Background(), registrar.ClientIdentity{FingerprintSHA256: "bb"})
	if err != ErrUnknownDesktop {
		t.Fatalf("err=%v", err)
	}
}
