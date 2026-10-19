package relayversion

import (
	"os"
	"strings"
	"testing"
)

func TestRelayVersionMatchesVERSIONMD(t *testing.T) {
	b, err := os.ReadFile("VERSION.md")
	if err != nil {
		t.Fatalf("read VERSION.md: %v", err)
	}
	want := strings.TrimSpace(string(b))
	if want == "" {
		t.Fatal("VERSION.md empty")
	}
	if relayVersion != want {
		t.Fatalf("relayVersion %q != VERSION.md %q", relayVersion, want)
	}
	if Version != relayVersion {
		t.Fatalf("Version %q != relayVersion %q", Version, relayVersion)
	}
}

func TestProtocolVersion(t *testing.T) {
	if ProtocolMajor != 2 || ProtocolMinor != 0 {
		t.Fatalf("protocol %d.%d, want 2.0", ProtocolMajor, ProtocolMinor)
	}
}
