package registrar

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeACL(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "acl.json")
	if err := os.WriteFile(p, []byte(body), 0o640); err != nil {
		t.Fatal(err)
	}
	return p
}

const goodACL = `{
  "acl_version": 1,
  "endpoint_id": "relay-primary",
  "agents": [
    {
      "agent_id": "agt_01HEXAMPLE",
      "cert_fingerprint_sha256": "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899",
      "channels": ["live"],
      "live_endpoints": ["relay-primary"]
    }
  ]
}`

func TestLookupAllow(t *testing.T) {
	reg, err := NewFileRegistry(writeACL(t, goodACL))
	if err != nil {
		t.Fatal(err)
	}
	entry, err := reg.Lookup(context.Background(), ClientIdentity{
		FingerprintSHA256: "AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.AuthorizeLive(entry, "agt_01HEXAMPLE", "relay-primary"); err != nil {
		t.Fatal(err)
	}
}

func TestLookupUnknown(t *testing.T) {
	reg, err := NewFileRegistry(writeACL(t, goodACL))
	if err != nil {
		t.Fatal(err)
	}
	_, err = reg.Lookup(context.Background(), ClientIdentity{FingerprintSHA256: "00" + strings.Repeat("11", 31)})
	if !errors.Is(err, ErrUnknownIdentity) {
		t.Fatalf("err=%v", err)
	}
}

func TestAuthorizeMismatchAndChannel(t *testing.T) {
	reg, err := NewFileRegistry(writeACL(t, goodACL))
	if err != nil {
		t.Fatal(err)
	}
	entry, err := reg.Lookup(context.Background(), ClientIdentity{
		FingerprintSHA256: "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.AuthorizeLive(entry, "wrong", "relay-primary"); !errors.Is(err, ErrAgentMismatch) {
		t.Fatalf("err=%v", err)
	}
	if err := reg.AuthorizeLive(entry, "agt_01HEXAMPLE", "other-relay"); !errors.Is(err, ErrChannelDenied) {
		t.Fatalf("err=%v", err)
	}

	noLive := `{
  "acl_version": 1,
  "endpoint_id": "relay-primary",
  "agents": [{
    "agent_id": "agt_x",
    "cert_fingerprint_sha256": "11223344556677889900aabbccddeeff11223344556677889900aabbccddeeff",
    "channels": ["persistence"],
    "live_endpoints": ["relay-primary"]
  }]
}`
	reg2, err := NewFileRegistry(writeACL(t, noLive))
	if err != nil {
		t.Fatal(err)
	}
	e2, _ := reg2.Lookup(context.Background(), ClientIdentity{
		FingerprintSHA256: "11223344556677889900aabbccddeeff11223344556677889900aabbccddeeff",
	})
	if err := reg2.AuthorizeLive(e2, "agt_x", "relay-primary"); !errors.Is(err, ErrChannelDenied) {
		t.Fatalf("err=%v", err)
	}
}

func TestDuplicateFingerprintFailsLoad(t *testing.T) {
	dup := `{
  "acl_version": 1,
  "endpoint_id": "relay-primary",
  "agents": [
    {"agent_id":"a1","cert_fingerprint_sha256":"aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899","channels":["live"],"live_endpoints":["relay-primary"]},
    {"agent_id":"a2","cert_fingerprint_sha256":"aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899","channels":["live"],"live_endpoints":["relay-primary"]}
  ]
}`
	if _, err := NewFileRegistry(writeACL(t, dup)); err == nil {
		t.Fatal("expected load error")
	}
}

func TestFingerprintSHA256DER(t *testing.T) {
	fp := FingerprintSHA256DER([]byte("hello"))
	if len(fp) != 64 {
		t.Fatalf("len=%d", len(fp))
	}
	if NormalizeFingerprint("AB:CD") != "abcd" {
		t.Fatal(NormalizeFingerprint("AB:CD"))
	}
}
