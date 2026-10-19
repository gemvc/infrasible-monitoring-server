package livesession

import "context"

// StatusCode mirrors WebSocket close codes for Conn.Close.
type StatusCode int

const (
	StatusNormalClosure   StatusCode = 1000
	StatusPolicyViolation StatusCode = 1008
	StatusMessageTooBig   StatusCode = 1009
	StatusInternalError   StatusCode = 1011
)

// Conn abstracts a text WebSocket for unit tests with fake peers.
type Conn interface {
	Read(ctx context.Context) ([]byte, error)
	Write(ctx context.Context, b []byte) error
	Close(code StatusCode, reason string) error
}
