package livesession

import (
	"context"
	"io"
	"sync"
)

// FakeConn is an in-memory Conn for acceptance-style unit tests.
type FakeConn struct {
	mu          sync.Mutex
	inbox       [][]byte
	wait        chan struct{}
	outbox      [][]byte
	closed      bool
	closeCh     chan struct{}
	CloseCode   StatusCode
	CloseReason string
}

// NewFakeConn starts with optional pre-queued inbound messages.
func NewFakeConn(inbound ...[]byte) *FakeConn {
	f := &FakeConn{
		inbox:   append([][]byte(nil), inbound...),
		wait:    make(chan struct{}, 1),
		closeCh: make(chan struct{}),
	}
	return f
}

// PushInbound queues a message for Read.
func (f *FakeConn) PushInbound(b []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}
	cp := append([]byte(nil), b...)
	f.inbox = append(f.inbox, cp)
	select {
	case f.wait <- struct{}{}:
	default:
	}
}

func (f *FakeConn) Read(ctx context.Context) ([]byte, error) {
	for {
		f.mu.Lock()
		if f.closed {
			f.mu.Unlock()
			return nil, io.EOF
		}
		if len(f.inbox) > 0 {
			b := f.inbox[0]
			f.inbox = f.inbox[1:]
			f.mu.Unlock()
			return b, nil
		}
		wait := f.wait
		closeCh := f.closeCh
		f.mu.Unlock()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-closeCh:
			return nil, io.EOF
		case <-wait:
		}
	}
}

func (f *FakeConn) Write(ctx context.Context, b []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return io.EOF
	}
	f.outbox = append(f.outbox, append([]byte(nil), b...))
	return nil
}

func (f *FakeConn) Close(code StatusCode, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	f.CloseCode = code
	f.CloseReason = reason
	close(f.closeCh)
	return nil
}

// Outbox returns a copy of written messages.
func (f *FakeConn) Outbox() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([][]byte, len(f.outbox))
	for i, b := range f.outbox {
		out[i] = append([]byte(nil), b...)
	}
	return out
}
