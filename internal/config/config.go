package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/gemvc/infrasible-monitoring-server/internal/protocol"
	"github.com/gemvc/infrasible-monitoring-server/internal/tlsfront"
)

// Config is the Relay process configuration (config_version 1).
type Config struct {
	ConfigVersion    int    `json:"config_version"`
	ListenAddr       string `json:"listen_addr"`
	LivePath         string `json:"live_path"`
	EndpointID       string `json:"endpoint_id"`
	ServerCertFile   string `json:"server_cert_file"`
	ServerKeyFile    string `json:"server_key_file"`
	ClientCAFile     string `json:"client_ca_file"`
	ACLFile          string `json:"acl_file"`
	HeartbeatSeconds int    `json:"heartbeat_seconds"`
	MaxMessageBytes  int    `json:"max_message_bytes"`
	Hub              string `json:"hub"`
	// Desktop subscriber (optional unless hub=desktop).
	DesktopLivePath       string `json:"desktop_live_path,omitempty"`
	DesktopClientCAFile   string `json:"desktop_client_ca_file,omitempty"`
	DesktopACLFile        string `json:"desktop_acl_file,omitempty"`
	DesktopQueueCapacity  int    `json:"desktop_queue_capacity,omitempty"`
}

// Load reads and parses config.json with unknown-field rejection.
func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var c Config
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return c, nil
}

// Validate fail-closes on invalid or missing required fields.
func Validate(c Config) error {
	if c.ConfigVersion != 1 {
		return fmt.Errorf("config_version %d not supported", c.ConfigVersion)
	}
	if strings.TrimSpace(c.ListenAddr) == "" {
		return fmt.Errorf("listen_addr empty")
	}
	if !strings.HasPrefix(c.LivePath, "/") {
		return fmt.Errorf("live_path must start with /")
	}
	if strings.TrimSpace(c.EndpointID) == "" {
		return fmt.Errorf("endpoint_id empty")
	}
	for _, p := range []struct {
		name, path string
	}{
		{"server_cert_file", c.ServerCertFile},
		{"server_key_file", c.ServerKeyFile},
		{"client_ca_file", c.ClientCAFile},
		{"acl_file", c.ACLFile},
	} {
		if strings.TrimSpace(p.path) == "" {
			return fmt.Errorf("%s empty", p.name)
		}
		if _, err := os.Stat(p.path); err != nil {
			return fmt.Errorf("%s: %w", p.name, err)
		}
	}
	if err := tlsfront.CheckKeyFileMode(c.ServerKeyFile); err != nil {
		return fmt.Errorf("server_key_file: %w", err)
	}
	if c.HeartbeatSeconds <= 0 || c.HeartbeatSeconds > protocol.MaxHeartbeatSeconds {
		return fmt.Errorf("heartbeat_seconds out of range")
	}
	if c.MaxMessageBytes <= 0 || c.MaxMessageBytes > 1<<20 {
		return fmt.Errorf("max_message_bytes out of range")
	}
	switch c.Hub {
	case "log", "noop", "desktop":
	default:
		return fmt.Errorf("hub must be log, noop, or desktop")
	}
	if c.Hub == "desktop" {
		if !strings.HasPrefix(c.DesktopLivePath, "/") {
			return fmt.Errorf("desktop_live_path must start with /")
		}
		if c.DesktopLivePath == c.LivePath {
			return fmt.Errorf("desktop_live_path must differ from live_path")
		}
		for _, p := range []struct {
			name, path string
		}{
			{"desktop_client_ca_file", c.DesktopClientCAFile},
			{"desktop_acl_file", c.DesktopACLFile},
		} {
			if strings.TrimSpace(p.path) == "" {
				return fmt.Errorf("%s empty", p.name)
			}
			if _, err := os.Stat(p.path); err != nil {
				return fmt.Errorf("%s: %w", p.name, err)
			}
		}
		if c.DesktopQueueCapacity < 0 {
			return fmt.Errorf("desktop_queue_capacity invalid")
		}
	} else if strings.TrimSpace(c.DesktopLivePath) != "" ||
		strings.TrimSpace(c.DesktopClientCAFile) != "" ||
		strings.TrimSpace(c.DesktopACLFile) != "" {
		// Allow staging Desktop paths on non-desktop hubs only when files exist.
		if strings.TrimSpace(c.DesktopLivePath) != "" && !strings.HasPrefix(c.DesktopLivePath, "/") {
			return fmt.Errorf("desktop_live_path must start with /")
		}
		for _, p := range []struct {
			name, path string
		}{
			{"desktop_client_ca_file", c.DesktopClientCAFile},
			{"desktop_acl_file", c.DesktopACLFile},
		} {
			if strings.TrimSpace(p.path) == "" {
				continue
			}
			if _, err := os.Stat(p.path); err != nil {
				return fmt.Errorf("%s: %w", p.name, err)
			}
		}
	}
	return nil
}
