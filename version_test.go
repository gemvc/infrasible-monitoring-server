package relayversion

import (
	"os"
	"strings"
	"testing"
)

func TestVersionMatchesVERSIONMD(t *testing.T) {
	b, err := os.ReadFile("VERSION.md")
	if err != nil {
		t.Fatalf("read VERSION.md: %v", err)
	}
	want := strings.TrimSpace(string(b))
	if Version != want {
		t.Fatalf("Version %q != VERSION.md %q", Version, want)
	}
}

func TestProtocolMajorIs2(t *testing.T) {
	if ProtocolMajor != 2 {
		t.Fatalf("ProtocolMajor = %d, want 2", ProtocolMajor)
	}
}
