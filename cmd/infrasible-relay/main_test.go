package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	mux := newLiveMux(cfg, stubReg{}, fanout.NewNoopHub(), obs, livesession.NewSessionMap(), slog.Default())

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
