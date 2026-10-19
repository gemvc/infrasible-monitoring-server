package tlsfront

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gemvc/infrasible-monitoring-server/internal/registrar"
)

// TLSConfig is the Live listener TLS settings.
type TLSConfig struct {
	ListenAddr     string
	ServerCertFile string
	ServerKeyFile  string
	ClientCAFile   string
	MinVersion     uint16 // default tls.VersionTLS13
}

// ListenAndServe starts an mTLS HTTPS server until ctx is cancelled.
func ListenAndServe(ctx context.Context, cfg TLSConfig, handler http.Handler) error {
	if cfg.MinVersion == 0 {
		cfg.MinVersion = tls.VersionTLS13
	}
	cert, err := tls.LoadX509KeyPair(cfg.ServerCertFile, cfg.ServerKeyFile)
	if err != nil {
		return fmt.Errorf("load server cert: %w", err)
	}
	caPEM, err := os.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return fmt.Errorf("read client ca: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return fmt.Errorf("parse client ca: no certificates")
	}
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		MinVersion:   cfg.MinVersion,
	}
	ln, err := tls.Listen("tcp", cfg.ListenAddr, tlsCfg)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		// TLSConfig already applied on listener; keep empty here.
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		<-errCh
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// ClientIdentityFromRequest extracts the leaf client cert fingerprint.
func ClientIdentityFromRequest(r *http.Request) (registrar.ClientIdentity, error) {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return registrar.ClientIdentity{}, fmt.Errorf("missing peer certificate")
	}
	return registrar.FingerprintFromCert(r.TLS.PeerCertificates[0])
}

// CheckKeyFileMode returns an error if path is not a regular file with mode 0600.
func CheckKeyFileMode(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("server key is not a regular file")
	}
	perm := st.Mode().Perm()
	if perm&0o077 != 0 {
		return fmt.Errorf("server key mode %#o must be 0600 (group/other bits clear)", perm)
	}
	return nil
}

// LocalListenerAddr is a test helper when ListenAddr is ":0".
func LocalListenerAddr(ln net.Listener) string {
	return ln.Addr().String()
}
