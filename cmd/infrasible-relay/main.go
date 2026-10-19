package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/coder/websocket"

	relayversion "github.com/gemvc/infrasible-monitoring-server"
	"github.com/gemvc/infrasible-monitoring-server/internal/config"
	"github.com/gemvc/infrasible-monitoring-server/internal/fanout"
	"github.com/gemvc/infrasible-monitoring-server/internal/livesession"
	"github.com/gemvc/infrasible-monitoring-server/internal/observe"
	"github.com/gemvc/infrasible-monitoring-server/internal/registrar"
	"github.com/gemvc/infrasible-monitoring-server/internal/tlsfront"
)

func main() {
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "run")
	}
	switch os.Args[1] {
	case "version", "-version", "--version":
		fmt.Println(relayversion.Version)
		return
	case "config":
		configCmd(os.Args[2:])
		return
	case "run":
		runCmd(os.Args[2:])
		return
	default:
		// allow `infrasible-relay --config=...` as run
		if strings.HasPrefix(os.Args[1], "-") {
			runCmd(os.Args[1:])
			return
		}
		fmt.Fprintf(os.Stderr, "usage: infrasible-relay <run|config validate|version> [--config=path]\n")
		os.Exit(2)
	}
}

func configCmd(args []string) {
	if len(args) < 1 || args[0] != "validate" {
		fmt.Fprintf(os.Stderr, "usage: infrasible-relay config validate --config=path\n")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("config validate", flag.ExitOnError)
	cfgPath := fs.String("config", "/etc/infrasible-relay/config.json", "path to config.json")
	_ = fs.Parse(args[1:])
	c, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := config.Validate(c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	reg, err := registrar.NewFileRegistry(c.ACLFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if reg.EndpointID() != c.EndpointID {
		fmt.Fprintf(os.Stderr, "acl endpoint_id %q != config endpoint_id %q\n", reg.EndpointID(), c.EndpointID)
		os.Exit(1)
	}
	fmt.Println("ok")
}

func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	cfgPath := fs.String("config", "/etc/infrasible-relay/config.json", "path to config.json")
	_ = fs.Parse(args)

	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("RELAY_LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	c, err := config.Load(*cfgPath)
	if err != nil {
		log.Error("config.load", "err", err.Error())
		os.Exit(1)
	}
	if err := config.Validate(c); err != nil {
		log.Error("config.validate", "err", err.Error())
		os.Exit(1)
	}
	reg, err := registrar.NewFileRegistry(c.ACLFile)
	if err != nil {
		log.Error("acl.load", "err", err.Error())
		os.Exit(1)
	}
	if reg.EndpointID() != c.EndpointID {
		log.Error("acl.endpoint_mismatch", "acl", reg.EndpointID(), "config", c.EndpointID)
		os.Exit(1)
	}

	var hub fanout.SubscriberHub
	switch c.Hub {
	case "noop":
		hub = fanout.NewNoopHub()
	default:
		hub = fanout.NewLogHub(log)
	}
	obs := observe.NewMetrics()
	sessions := livesession.NewSessionMap()

	mux := http.NewServeMux()
	mux.HandleFunc(c.LivePath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			// R9: Persistence-shaped POST not accepted as Live
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			obs.IncAuthReject("wrong_method")
			return
		}
		id, err := tlsfront.ClientIdentityFromRequest(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			obs.IncAuthReject("missing_client_cert")
			return
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			CompressionMode: websocket.CompressionDisabled,
		})
		if err != nil {
			log.Info("ws.accept_failed", "err", err.Error())
			return
		}
		ws := &wsConn{c: conn}
		livesession.ServeAgent(r.Context(), ws, id, reg, hub, obs, livesession.SessionConfig{
			EndpointID:       c.EndpointID,
			HeartbeatSeconds: c.HeartbeatSeconds,
			MaxMessageBytes:  c.MaxMessageBytes,
			Logger:           log,
			Sessions:         sessions,
		})
	})

	// Observe snapshot — default localhost-only; lab may set RELAY_ADMIN_ADDR=0.0.0.0:9090
	adminAddr := os.Getenv("RELAY_ADMIN_ADDR")
	if adminAddr == "" {
		adminAddr = "127.0.0.1:9090"
	}
	admin := http.NewServeMux()
	admin.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(obs.Snapshot())
	})
	go func() {
		log.Info("admin.listen", "addr", adminAddr)
		if err := http.ListenAndServe(adminAddr, admin); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Info("admin.metrics", "err", err.Error())
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("relay.start",
		"version", relayversion.Version,
		"listen", c.ListenAddr,
		"live_path", c.LivePath,
		"endpoint_id", c.EndpointID,
	)
	err = tlsfront.ListenAndServe(ctx, tlsfront.TLSConfig{
		ListenAddr:     c.ListenAddr,
		ServerCertFile: c.ServerCertFile,
		ServerKeyFile:  c.ServerKeyFile,
		ClientCAFile:   c.ClientCAFile,
	}, mux)
	if err != nil {
		log.Error("relay.stop", "err", err.Error())
		os.Exit(1)
	}
}

type wsConn struct {
	c *websocket.Conn
}

func (w *wsConn) Read(ctx context.Context) ([]byte, error) {
	_, data, err := w.c.Read(ctx)
	return data, err
}

func (w *wsConn) Write(ctx context.Context, b []byte) error {
	return w.c.Write(ctx, websocket.MessageText, b)
}

func (w *wsConn) Close(code livesession.StatusCode, reason string) error {
	return w.c.Close(websocket.StatusCode(code), reason)
}
