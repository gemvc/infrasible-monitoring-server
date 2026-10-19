package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gemvc/infrasible-monitoring-server/internal/config"
	"github.com/gemvc/infrasible-monitoring-server/internal/fanout"
	"github.com/gemvc/infrasible-monitoring-server/internal/livesession"
	"github.com/gemvc/infrasible-monitoring-server/internal/observe"
	"github.com/gemvc/infrasible-monitoring-server/internal/registrar"
)

type stubReg struct{}

func (stubReg) Lookup(context.Context, registrar.ClientIdentity) (registrar.ACLEntry, error) {
	return registrar.ACLEntry{}, registrar.ErrUnknownIdentity
}

func TestAcceptanceR9_PersistencePOST(t *testing.T) {
	obs := observe.NewMetrics()
	cfg := config.Config{
		LivePath:         "/v2/agents/live",
		EndpointID:       "relay-primary",
		HeartbeatSeconds: 15,
		MaxMessageBytes:  262144,
	}
	mux := newLiveMux(cfg, stubReg{}, fanout.NewNoopHub(), obs, livesession.NewSessionMap(), slog.Default(), nil, nil)

	body := `{"final":true,"agent_id":"agt_1","metrics":{}}`
	req := httptest.NewRequest(http.MethodPost, cfg.LivePath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d want 405", rr.Code)
	}
	if obs.Snapshot().AuthRejects["wrong_method"] < 1 {
		t.Fatalf("rejects=%v", obs.Snapshot().AuthRejects)
	}
	if obs.Snapshot().HelloAccepts != 0 || obs.Snapshot().Frames != 0 {
		t.Fatalf("must not accept Live session: snap=%+v", obs.Snapshot())
	}
}

func TestAcceptanceP3_ConfigValidateCLI(t *testing.T) {
	dir := t.TempDir()
	cert, key := writeSelfSigned(t, dir)
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
  "client_ca_file": "` + cert + `",
  "acl_file": "` + acl + `",
  "heartbeat_seconds": 15,
  "max_message_bytes": 262144,
  "hub": "noop"
}`
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateConfigFile(cfgPath); err != nil {
		t.Fatalf("P3: config validate must exit 0 path: %v", err)
	}

	badACL := filepath.Join(dir, "bad-acl.json")
	if err := os.WriteFile(badACL, []byte(`{"acl_version":1,"endpoint_id":"other","agents":[]}`), 0o640); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(dir, "mismatch.json")
	bodyBad := `{
  "config_version": 1,
  "listen_addr": ":8443",
  "live_path": "/v2/agents/live",
  "endpoint_id": "relay-primary",
  "server_cert_file": "` + cert + `",
  "server_key_file": "` + key + `",
  "client_ca_file": "` + cert + `",
  "acl_file": "` + badACL + `",
  "heartbeat_seconds": 15,
  "max_message_bytes": 262144,
  "hub": "noop"
}`
	if err := os.WriteFile(badPath, []byte(bodyBad), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateConfigFile(badPath); err == nil {
		t.Fatal("P3: endpoint mismatch must fail")
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
