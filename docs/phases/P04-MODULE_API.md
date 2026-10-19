# P04 — Go module API (names, inputs, outputs)

**Parent brief:** [PROMPT.md](../PROMPT.md)  
Implement these responsibilities with the names below (or document intentional renames in your Relay `STATUS.md`).

Suggested module path: `github.com/<org>/infrasible-relay`.

---

## 1. `internal/protocol`

### Types

```go
type Message interface {
    Type() string
}

type AgentHello struct {
    Type            string   `json:"type"`
    ProtocolMajor   int      `json:"protocol_major"`
    ProtocolMinor   int      `json:"protocol_minor"`
    AgentID         string   `json:"agent_id"`
    AgentVersion    string   `json:"agent_version"`
    Channel         string   `json:"channel"`
    SchemaVersions  []string `json:"schema_versions"`
    Capabilities    []string `json:"capabilities"`
    SentAtUTC       string   `json:"sent_at_utc"`
}

type HelloAccepted struct {
    Type               string `json:"type"`
    ProtocolMajor      int    `json:"protocol_major"`
    ProtocolMinor      int    `json:"protocol_minor"`
    SchemaVersion      string `json:"schema_version"`
    AuthorizedAgentID  string `json:"authorized_agent_id"`
    AuthorizedChannel  string `json:"authorized_channel"`
    EndpointID         string `json:"endpoint_id"`
    HeartbeatSeconds   int    `json:"heartbeat_seconds"`
    MaxMessageBytes    int    `json:"max_message_bytes"`
}

type LiveEnvelope struct {
    Type    string          `json:"type"`
    AgentID string          `json:"agent_id"`
    Frame   json.RawMessage `json:"frame"` // opaque; do not over-parse
}

type Heartbeat struct {
    Type      string `json:"type"`
    SentAtUTC string `json:"sent_at_utc"`
}
```

### Functions

| Function | Input | Output | Notes |
|---|---|---|---|
| `DecodeAgentMessage(b []byte)` | raw WS text | `(Message, error)` | Dispatches on `type`; unknown → error |
| `EncodeHelloAccepted(h HelloAccepted)` | struct | `([]byte, error)` | Sets `type` if empty |
| `EncodeHeartbeat(sentAt time.Time)` | time | `([]byte, error)` | UTC RFC3339 |
| `ValidateAgentHello(h AgentHello, expectAgentID string)` | hello + ACL id | `error` | Protocol/channel/id checks |
| `NewHelloAccepted(agentID, endpointID string, hb, maxBytes int)` | ids + knobs | `HelloAccepted` | Defaults 15 / 262144 |

---

## 2. `internal/tlsfront`

```go
type TLSConfig struct {
    ListenAddr     string // e.g. ":8443"
    ServerCertFile string
    ServerKeyFile  string // must be mode 0600 at runtime check (warn/fail)
    ClientCAFile   string
    MinVersion     uint16 // tls.VersionTLS13
}

// Handler receives requests already on a verified TLS connection.
// Live path upgrades to WebSocket; extract peer certs from r.TLS.PeerCertificates.
func ListenAndServe(ctx context.Context, cfg TLSConfig, handler http.Handler) error
```

| Concern | Behavior |
|---|---|
| Client auth | `tls.RequireAndVerifyClientCert` |
| Cipher / version | TLS 1.3 minimum |
| Shutdown | `ctx` cancel → graceful `Shutdown` |
| Identity helper | `ClientIdentityFromRequest(*http.Request) (ClientIdentity, error)` — SHA-256 fingerprint of leaf cert |

```go
type ClientIdentity struct {
    FingerprintSHA256 string // hex or colon-hex; pick one and document
    Subject           string // for logs only
}
```

---

## 3. `internal/registrar`

```go
type ACLEntry struct {
    AgentID       string
    Channels      []string // contains "live"
    LiveEndpoints []string // must include this Relay endpoint_id
}

type Registry interface {
    Lookup(ctx context.Context, id ClientIdentity) (ACLEntry, error)
}

func NewFileRegistry(path string) (*FileRegistry, error)
func (f *FileRegistry) Lookup(ctx context.Context, id ClientIdentity) (ACLEntry, error)
func (f *FileRegistry) AuthorizeLive(entry ACLEntry, agentIDFromHello, endpointID string) error
```

| Error | When |
|---|---|
| `ErrUnknownIdentity` | Fingerprint missing |
| `ErrChannelDenied` | No live / wrong endpoint |
| `ErrAgentMismatch` | Hello id ≠ ACL id |

File schema: [P05-REGISTRAR_FILE.md](P05-REGISTRAR_FILE.md).

---

## 4. `internal/livesession`

```go
type Conn interface {
    Read(ctx context.Context) ([]byte, error)
    Write(ctx context.Context, b []byte) error
    Close(code StatusCode, reason string) error
}

type SessionConfig struct {
    EndpointID       string
    HeartbeatSeconds int
    MaxMessageBytes  int
    HelloTimeout     time.Duration // ~10s
}

type SessionMap struct { /* sync */ }

func (m *SessionMap) Bind(agentID string, gen uint64, closer func()) (replaced bool)
func (m *SessionMap) Unbind(agentID string, gen uint64)

func ServeAgent(
    ctx context.Context,
    conn Conn,
    id ClientIdentity,
    reg Registry,
    hub fanout.SubscriberHub,
    obs *observe.Metrics,
    cfg SessionConfig,
) // blocks until session ends
```

Session algorithm: see PROMPT §6.5 and Agent MONITORING_RELAY §5.

---

## 5. `internal/fanout`

```go
type SubscriberHub interface {
    PublishLive(agentID string, frame []byte)
    MarkStale(agentID string)
}

func NewNoopHub() SubscriberHub
func NewLogHub(logger *slog.Logger) SubscriberHub

// Optional helper for real hubs later:
type DropOldestQueue struct { /* cap N */ }
func (q *DropOldestQueue) Push(b []byte) (dropped bool)
```

v1: `LogHub` / `NoopHub` only. See [P06](P06-DESKTOP_STUB.md).

---

## 6. `internal/observe`

```go
type Metrics struct { /* atomic counters */ }

func NewMetrics() *Metrics

func (m *Metrics) IncConnected(delta int64)
func (m *Metrics) IncAuthReject(reason string)
func (m *Metrics) IncFrames(agentID string)
func (m *Metrics) IncDrops(agentID string)
func (m *Metrics) SetLastFrame(agentID string, t time.Time)
func (m *Metrics) Snapshot() Snapshot // for /metrics or logs
```

Optional: `HTTPHandler() http.Handler` bound to localhost only.

---

## 7. `internal/config`

```go
type Config struct {
    ConfigVersion     int    `json:"config_version"`
    ListenAddr        string `json:"listen_addr"`
    LivePath          string `json:"live_path"`
    EndpointID        string `json:"endpoint_id"`
    ServerCertFile    string `json:"server_cert_file"`
    ServerKeyFile     string `json:"server_key_file"`
    ClientCAFile      string `json:"client_ca_file"`
    ACLFile           string `json:"acl_file"`
    HeartbeatSeconds  int    `json:"heartbeat_seconds"`
    MaxMessageBytes   int    `json:"max_message_bytes"`
    Hub               string `json:"hub"` // "log" | "noop"
}

func Load(path string) (Config, error)
func Validate(c Config) error
```

Unknown JSON fields: **fail** (match Agent strictness) or document allowlist — prefer fail.

---

## 8. `cmd/infrasible-relay`

```go
func main() {
    // flag: --config=/etc/infrasible-relay/config.json
    // Load + Validate
    // Build registry, hub, metrics, session map
    // mux: LivePath → upgrade → ServeAgent
    // ListenAndServe
}
```

CLI subcommands (recommended):

| Command | Behavior |
|---|---|
| `infrasible-relay run --config=…` | Default daemon |
| `infrasible-relay config validate --config=…` | Exit 0/1 JSON or text |
| `infrasible-relay version` | Print product semver from `version.go` |

---

## 9. Testing map

| Package | Tests |
|---|---|
| `protocol` | Decode/encode golden JSON; reject unknowns |
| `registrar` | File fixtures; deny paths |
| `livesession` | Fake Conn; Hello then live; silence close; replace Bind |
| `fanout` | DropOldestQueue; LogHub no panic |
| `tlsfront` | httptest + self-signed mTLS fixture (optional heavy) |
| Integration | Scripted lab against real Agent binary |
