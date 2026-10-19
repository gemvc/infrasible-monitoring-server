package tlsfront

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckKeyFileMode0600(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.key")
	bad := filepath.Join(dir, "bad.key")
	if err := os.WriteFile(ok, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CheckKeyFileMode(ok); err != nil {
		t.Fatalf("0600: %v", err)
	}
	if err := CheckKeyFileMode(bad); err == nil {
		t.Fatal("0644 must fail")
	}
	missing := filepath.Join(dir, "missing.key")
	if err := CheckKeyFileMode(missing); err == nil {
		t.Fatal("missing must fail")
	}
}
