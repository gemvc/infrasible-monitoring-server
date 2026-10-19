package livesession

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gemvc/infrasible-monitoring-server/internal/fanout"
	"github.com/gemvc/infrasible-monitoring-server/internal/observe"
	"github.com/gemvc/infrasible-monitoring-server/internal/protocol"
	"github.com/gemvc/infrasible-monitoring-server/internal/registrar"
)

// SessionConfig knobs for a Live session.
type SessionConfig struct {
	EndpointID       string
	HeartbeatSeconds int
	MaxMessageBytes  int
	HelloTimeout     time.Duration
	Logger           *slog.Logger
	Sessions         *SessionMap
}

// ServeAgent runs trust layers B–D on an already-upgraded WebSocket Conn.
// It blocks until the session ends.
func ServeAgent(
	ctx context.Context,
	conn Conn,
	id registrar.ClientIdentity,
	reg registrar.Registry,
	hub fanout.SubscriberHub,
	obs *observe.Metrics,
	cfg SessionConfig,
) {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	if hub == nil {
		hub = fanout.NewNoopHub()
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
	if cfg.HelloTimeout <= 0 {
		cfg.HelloTimeout = 10 * time.Second
	}
	if cfg.Sessions == nil {
		cfg.Sessions = NewSessionMap()
	}

	closeReason := "session_end"
	closeCode := StatusNormalClosure
	defer func() {
		_ = conn.Close(closeCode, closeReason)
	}()

	entry, err := reg.Lookup(ctx, id)
	if err != nil {
		reason := "unknown_identity"
		if !errors.Is(err, registrar.ErrUnknownIdentity) {
			reason = "lookup_error"
		}
		obs.IncAuthReject(reason)
		log.Info("auth.reject", "reason", reason)
		closeCode = StatusPolicyViolation
		closeReason = reason
		return
	}

	helloCtx, cancelHello := context.WithTimeout(ctx, cfg.HelloTimeout)
	raw, err := conn.Read(helloCtx)
	cancelHello()
	if err != nil {
		obs.IncAuthReject("hello_timeout")
		log.Info("auth.reject", "reason", "hello_timeout", "agent_id", entry.AgentID)
		closeCode = StatusPolicyViolation
		closeReason = "hello_timeout"
		return
	}
	if len(raw) > cfg.MaxMessageBytes {
		obs.IncAuthReject("oversized")
		closeCode = StatusMessageTooBig
		closeReason = "oversized"
		return
	}

	msg, err := protocol.DecodeAgentMessage(raw)
	if err != nil {
		obs.IncAuthReject("hello_invalid")
		log.Info("auth.reject", "reason", "hello_invalid", "err", err.Error())
		closeCode = StatusPolicyViolation
		closeReason = "hello_invalid"
		return
	}
	hello, ok := msg.(protocol.AgentHello)
	if !ok {
		obs.IncAuthReject("hello_invalid")
		log.Info("auth.reject", "reason", "first_not_hello", "agent_id", entry.AgentID)
		closeCode = StatusPolicyViolation
		closeReason = "first_not_hello"
		return
	}
	if err := protocol.ValidateAgentHello(hello, entry.AgentID); err != nil {
		obs.IncAuthReject("hello_invalid")
		log.Info("auth.reject", "reason", "hello_invalid", "agent_id", entry.AgentID, "err", err.Error())
		closeCode = StatusPolicyViolation
		closeReason = "hello_invalid"
		return
	}
	if err := registrar.AuthorizeLive(entry, hello.AgentID, cfg.EndpointID); err != nil {
		reason := "channel_denied"
		if errors.Is(err, registrar.ErrAgentMismatch) {
			reason = "agent_mismatch"
		}
		obs.IncAuthReject(reason)
		log.Info("auth.reject", "reason", reason, "agent_id", hello.AgentID)
		closeCode = StatusPolicyViolation
		closeReason = reason
		return
	}

	accepted := protocol.NewHelloAccepted(hello.AgentID, cfg.EndpointID, cfg.HeartbeatSeconds, cfg.MaxMessageBytes)
	payload, err := protocol.EncodeHelloAccepted(accepted)
	if err != nil {
		closeCode = StatusInternalError
		closeReason = "encode_error"
		return
	}
	writeCtx, cancelWrite := context.WithTimeout(ctx, 5*time.Second)
	err = conn.Write(writeCtx, payload)
	cancelWrite()
	if err != nil {
		closeCode = StatusInternalError
		closeReason = "write_error"
		return
	}

	agentID := hello.AgentID
	obs.IncHelloAccept()
	obs.IncConnected(1)
	defer obs.IncConnected(-1)

	sessionCtx, cancelSession := context.WithCancel(ctx)
	defer cancelSession()

	gen, replaced := cfg.Sessions.Bind(agentID, cancelSession)
	defer cfg.Sessions.Unbind(agentID, gen)
	if replaced {
		log.Info("session.replaced", "agent_id", agentID)
	}
	log.Info("session.accepted", "agent_id", agentID, "endpoint_id", cfg.EndpointID)

	staleOnce := false
	defer func() {
		if !staleOnce {
			staleOnce = true
			hub.MarkStale(agentID)
		}
	}()

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
				log.Info("session.silence", "agent_id", agentID)
				closeCode = StatusPolicyViolation
				closeReason = "silence"
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
				closeCode = StatusMessageTooBig
				closeReason = "oversized"
				return
			}
			msg, err := protocol.DecodeAgentMessage(rr.data)
			if err != nil {
				obs.IncAuthReject("bad_message")
				closeCode = StatusPolicyViolation
				closeReason = "bad_message"
				return
			}
			switch m := msg.(type) {
			case protocol.LiveEnvelope:
				if m.AgentID != agentID {
					obs.IncAuthReject("agent_mismatch")
					closeCode = StatusPolicyViolation
					closeReason = "agent_mismatch"
					return
				}
				hub.PublishLive(agentID, rr.data)
				obs.IncFrames(agentID)
				obs.SetLastFrame(agentID, time.Now())
			case protocol.Heartbeat:
				// peer liveness already updated via lastPeer
			default:
				obs.IncAuthReject("unexpected_type")
				closeCode = StatusPolicyViolation
				closeReason = "unexpected_type"
				return
			}
		}
	}
}

type readResult struct {
	data []byte
	err  error
}
