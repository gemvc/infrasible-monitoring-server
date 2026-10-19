package desktophub

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/gemvc/infrasible-monitoring-server/internal/livesession"
	"github.com/gemvc/infrasible-monitoring-server/internal/observe"
	"github.com/gemvc/infrasible-monitoring-server/internal/protocol"
	"github.com/gemvc/infrasible-monitoring-server/internal/registrar"
)

// SessionConfig knobs for a Desktop subscriber session.
type SessionConfig struct {
	EndpointID       string
	HeartbeatSeconds int
	MaxMessageBytes  int
	SubscribeTimeout time.Duration
	Logger           *slog.Logger
	// QueryAgentID is optional (?agent_id=) and merges into the subscribe set.
	QueryAgentID string
}

// ServeDesktop runs Desktop trust + subscribe on an already-upgraded WebSocket.
// It does not touch Agent Hello / livesession.ServeAgent.
func ServeDesktop(
	ctx context.Context,
	conn livesession.Conn,
	id registrar.ClientIdentity,
	reg Registry,
	hub *Hub,
	obs *observe.Metrics,
	cfg SessionConfig,
) {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	if hub == nil {
		_ = conn.Close(livesession.StatusInternalError, "no_hub")
		return
	}
	if obs == nil {
		obs = observe.NewMetrics()
	}
	if cfg.HeartbeatSeconds <= 0 {
		cfg.HeartbeatSeconds = protocol.DefaultHeartbeatSeconds
	}
	if cfg.MaxMessageBytes <= 0 {
		cfg.MaxMessageBytes = protocol.DefaultMaxMessageBytes
	}
	if cfg.SubscribeTimeout <= 0 {
		cfg.SubscribeTimeout = 10 * time.Second
	}

	closeReason := "session_end"
	closeCode := livesession.StatusNormalClosure
	defer func() {
		_ = conn.Close(closeCode, closeReason)
	}()

	entry, err := reg.Lookup(ctx, id)
	if err != nil {
		reason := "unknown_identity"
		if !errors.Is(err, ErrUnknownDesktop) {
			reason = "lookup_error"
		}
		obs.IncAuthReject(reason)
		log.Info("desktop.auth.reject", "reason", reason)
		closeCode = livesession.StatusPolicyViolation
		closeReason = reason
		return
	}
	if reg.EndpointID() != "" && cfg.EndpointID != "" && reg.EndpointID() != cfg.EndpointID {
		obs.IncAuthReject("endpoint_mismatch")
		log.Info("desktop.auth.reject", "reason", "endpoint_mismatch")
		closeCode = livesession.StatusPolicyViolation
		closeReason = "endpoint_mismatch"
		return
	}

	subCtx, cancelSub := context.WithTimeout(ctx, cfg.SubscribeTimeout)
	raw, err := conn.Read(subCtx)
	cancelSub()
	if err != nil {
		obs.IncAuthReject("subscribe_timeout")
		log.Info("desktop.auth.reject", "reason", "subscribe_timeout", "operator_id", entry.OperatorID)
		closeCode = livesession.StatusPolicyViolation
		closeReason = "subscribe_timeout"
		return
	}
	if len(raw) > cfg.MaxMessageBytes {
		obs.IncAuthReject("oversized")
		closeCode = livesession.StatusMessageTooBig
		closeReason = "oversized"
		return
	}

	sub, err := DecodeSubscribe(raw)
	if err != nil {
		obs.IncAuthReject("subscribe_invalid")
		log.Info("desktop.auth.reject", "reason", "subscribe_invalid", "err", err.Error())
		closeCode = livesession.StatusPolicyViolation
		closeReason = "subscribe_invalid"
		return
	}
	if err := ValidateSubscribe(sub); err != nil {
		obs.IncAuthReject("subscribe_invalid")
		log.Info("desktop.auth.reject", "reason", "subscribe_invalid", "err", err.Error())
		closeCode = livesession.StatusPolicyViolation
		closeReason = "subscribe_invalid"
		return
	}

	agentIDs := normalizeAgentIDs(sub.AgentIDs, cfg.QueryAgentID)
	if err := AuthorizeSubscribe(entry, agentIDs); err != nil {
		obs.IncAuthReject("agent_not_allowed")
		log.Info("desktop.auth.reject", "reason", "agent_not_allowed", "operator_id", entry.OperatorID)
		closeCode = livesession.StatusPolicyViolation
		closeReason = "agent_not_allowed"
		return
	}

	accepted := NewSubscribeAccepted(cfg.EndpointID, agentIDs, cfg.HeartbeatSeconds, cfg.MaxMessageBytes)
	payload, err := EncodeSubscribeAccepted(accepted)
	if err != nil {
		closeCode = livesession.StatusInternalError
		closeReason = "encode_error"
		return
	}
	writeCtx, cancelWrite := context.WithTimeout(ctx, 5*time.Second)
	err = conn.Write(writeCtx, payload)
	cancelWrite()
	if err != nil {
		closeCode = livesession.StatusInternalError
		closeReason = "write_error"
		return
	}

	_, events, detach := hub.Attach(agentIDs)
	defer detach()
	log.Info("desktop.session.accepted", "operator_id", entry.OperatorID, "agents", len(agentIDs))

	sessionCtx, cancelSession := context.WithCancel(ctx)
	defer cancelSession()

	hb := time.Duration(cfg.HeartbeatSeconds) * time.Second
	silence := 3 * hb
	lastPeer := time.Now()

	hbTicker := time.NewTicker(hb)
	defer hbTicker.Stop()
	silenceTicker := time.NewTicker(hb)
	defer silenceTicker.Stop()

	readCh := make(chan readResult, 1)
	go func() {
		defer close(readCh)
		for {
			data, err := conn.Read(sessionCtx)
			select {
			case readCh <- readResult{data: data, err: err}:
			case <-sessionCtx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-sessionCtx.Done():
			closeReason = "cancelled"
			return
		case <-hbTicker.C:
			b, err := protocol.EncodeHeartbeat(time.Now().UTC())
			if err != nil {
				continue
			}
			wctx, cancel := context.WithTimeout(sessionCtx, 5*time.Second)
			err = conn.Write(wctx, b)
			cancel()
			if err != nil {
				closeReason = "write_error"
				return
			}
		case <-silenceTicker.C:
			if time.Since(lastPeer) > silence {
				obs.IncAuthReject("silence")
				log.Info("desktop.session.silence", "operator_id", entry.OperatorID)
				closeCode = livesession.StatusPolicyViolation
				closeReason = "silence"
				return
			}
		case ev, ok := <-events:
			if !ok {
				closeReason = "detached"
				return
			}
			var out []byte
			var encErr error
			switch ev.kind {
			case "live":
				out = ev.frame
			case "stale":
				out, encErr = EncodeStale(ev.agentID)
			default:
				continue
			}
			if encErr != nil {
				continue
			}
			wctx, cancel := context.WithTimeout(sessionCtx, 5*time.Second)
			err = conn.Write(wctx, out)
			cancel()
			if err != nil {
				closeReason = "write_error"
				return
			}
		case rr, ok := <-readCh:
			if !ok {
				closeReason = "read_closed"
				return
			}
			if rr.err != nil {
				closeReason = "read_error"
				return
			}
			lastPeer = time.Now()
			if len(rr.data) > cfg.MaxMessageBytes {
				obs.IncAuthReject("oversized")
				closeCode = livesession.StatusMessageTooBig
				closeReason = "oversized"
				return
			}
			msgType, err := DecodePeerMessage(rr.data)
			if err != nil {
				obs.IncAuthReject("bad_message")
				closeCode = livesession.StatusPolicyViolation
				closeReason = "bad_message"
				return
			}
			_ = msgType // heartbeat only
		}
	}
}

type readResult struct {
	data []byte
	err  error
}

func normalizeAgentIDs(fromMsg []string, queryAgentID string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(fromMsg)+1)
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range fromMsg {
		add(id)
	}
	add(queryAgentID)
	return out
}
