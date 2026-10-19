package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadValidateHappy(t *testing.T) {
	dir := t.TempDir()
	cert, key := writeSelfSigned(t, dir)
	ca := cert
	acl := filepath.Join(dir, "acl.json")
	if err := os.WriteFile(acl, []byte(`{"acl_version":1,"endpoint_id":"relay-primary","agents":[]}`), 0o640); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "config.json")
	body := `{
  "config_version": 1,
  "listen_addr": ":8443",
  "live_path": "/v2/agents/live",
  "endpoint_id": "relay-primary",
  "server_cert_file": "` + cert + `",
  "server_key_file": "` + key + `",
  "client_ca_file": "` + ca + `",
  "acl_file": "` + acl + `",
  "heartbeat_seconds": 15,
  "max_message_bytes": 262144,
  "hub": "noop"
}`
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(c); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownFieldFails(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.json")
	_ = os.WriteFile(p, []byte(`{"config_version":1,"extra":true}`), 0o644)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error")
	}
}

func TestAcceptanceP1_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	cert, key := writeSelfSigned(t, dir)
	acl := filepath.Join(dir, "acl.json")
	_ = os.WriteFile(acl, []byte(`{"acl_version":1,"endpoint_id":"relay-primary","agents":[]}`), 0o640)
	cfgPath := filepath.Join(dir, "bad.json")
	body := `{
  "config_version": 1,
  "listen_addr": "",
  "live_path": "/v2/agents/live",
  "endpoint_id": "relay-primary",
  "server_cert_file": "` + cert + `",
  "server_key_file": "` + key + `",
  "client_ca_file": "` + cert + `",
  "acl_file": "` + acl + `",
  "heartbeat_seconds": 15,
  "max_message_bytes": 262144,
  "hub": "noop"
}`
	_ = os.WriteFile(cfgPath, []byte(body), 0o644)
	c, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(c); err == nil {
		t.Fatal("P1: invalid config must fail validate")
	}
}

func TestAcceptanceP2_KeyModeNot0600(t *testing.T) {
	dir := t.TempDir()
	cert, key := writeSelfSigned(t, dir)
	if err := os.Chmod(key, 0o644); err != nil {
		t.Fatal(err)
	}
	acl := filepath.Join(dir, "acl.json")
	_ = os.WriteFile(acl, []byte(`{"acl_version":1,"endpoint_id":"relay-primary","agents":[]}`), 0o640)
	cfgPath := filepath.Join(dir, "config.json")
	body := `{
  "config_version": 1,
  "listen_addr": ":8443",
  "live_path": "/v2/agents/live",
  "endpoint_id": "relay-primary",
  "server_cert_file": "` + cert + `",
  "server_key_file": "` + key + `",
  "client_ca_file": "` + cert + `",
  "acl_file": "` + acl + `",
  "heartbeat_seconds": 15,
  "max_message_bytes": 262144,
  "hub": "noop"
}`
	_ = os.WriteFile(cfgPath, []byte(body), 0o644)
	c, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(c); err == nil {
		t.Fatal("P2: key mode 0644 must fail validate")
	}
}

func writeSelfSigned(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPath = filepath.Join(dir, "server.crt")
	keyPath = filepath.Join(dir, "server.key")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}
