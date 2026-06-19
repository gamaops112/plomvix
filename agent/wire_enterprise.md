# Wire Protocol and API Server Enterprise

| Field | Value |
| :--- | :--- |
| **Source** | `agent/wire_enterprise.md` |
| **Package(s)** | `internal/server` |
| **Purpose** | Production-harden the API server with Extended Query protocol support for parameterized queries, secure SSL/TLS connection negotiation, SCRAM-SHA-256 authentication handshakes, and connection throttling. |
| **Dependencies** | Wire Protocol Setup plan, Catalog Setup/Enterprise plans. |

## Honest Contracts & Known Trade-offs

1. **Extended Query Protocol Complexity:** In-flight session states must support prepared statement definitions mapping query text to placeholders (e.g. `$1`, `$2`) and parsing parameter types. These must be stored in connection-specific caches.
2. **TLS/SSL Encryption Certificate Injection:** Enabling SSL/TLS encrypts all connection traffic. The server certificate and private key paths must be explicitly injected from the global `ServerConfig` (e.g. `ServerConfig.SSLCertPath` and `ServerConfig.SSLKeyPath`) into the listener server and session initialization structures.
3. **SCRAM-SHA-256 Authentication Library Dependency:** Implementing SCRAM-SHA-256 from scratch introduces high crypto complexity (e.g., SASLprep, PBKDF2). To reduce scope risk and ensure protocol correctness, the implementation must utilize a vetted third-party library, specifically `github.com/xdg-go/scram`, for authentication challenge negotiation.
4. **Global Server-Level Throttling:** Active connections are bounded by `SQLEngineConfig.MaxConnections`. This limit must be managed via a **global, server-level semaphore** (such as an `atomic.Int64` counter or a buffered channel on the main `Server` listener struct) that intercepts connections inside the `Accept()` listener loop—not locally inside individual sessions.
5. **Session Isolation:** Prepared statement states, transactional states, and temporary tables must be isolated per connection session.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/server/extended.go` | Implement the Extended Query protocol parser ('P', 'B', 'E', 'S' message loops) and placeholder binding logic. |
| `internal/server/ssl.go` | Implement TLS/SSL handshake negotiation, loading certificates, and wrapped connection proxying. |
| `internal/server/auth.go` | Implement SCRAM-SHA-256 authentication challenge-response parsing and validation. |
| `internal/server/throttle.go` | Implement connection throttle gates using atomic counters or channel queues. |
| `internal/server/wire_enterprise_test.go` | Integration tests validating prepared statement bindings, SSL encryption handshakes, connection throttling limits, and telemetry logs. |

---

## Key API & Concepts

### 1. Extended Query Protocol Parsing (`internal/server/extended.go`)

The Extended Query protocol splits query execution into separate phases: Parse, Bind, and Execute.

```go
package server

type PreparedPlan struct {
	Name      string
	SQL       string
	ParamOIDs []PGOID
}

type Portal struct {
	Name      string
	SQL       string
	Parameters []engine.Datum
}

// In-session cache maps
type SessionCache struct {
	statements map[string]PreparedPlan
	portals    map[string]Portal
}

func (s *Session) handleExtendedMessage(ctx context.Context, mType MsgType, payload []byte) error {
	switch mType {
	case 'P': // Parse: parse query text and placeholder types
		return s.handleParse(payload)
	case 'B': // Bind: bind concrete parameter datums to portal
		return s.handleBind(payload)
	case 'E': // Execute: execute the bound portal
		return s.handleExecute(ctx, payload)
	case 'S': // Sync: commit state and send ReadyForQuery
		return s.handleSync()
	}
	return nil
}
```

### 2. TLS/SSL Secure Negotiation (`internal/server/ssl.go`)

The connection handshakes must detect SSL requests and negotiate TLS configurations dynamically using standard certificates.

```go
import (
	"crypto/tls"
	"net"
)

func (s *Session) handleSSLRequest() error {
	// 1. Read 4-byte length and 4-byte SSL magic code (80877103)
	// 2. Respond with 'S' to indicate SSL support is enabled
	if _, err := s.conn.Write([]byte{'S'}); err != nil {
		return err
	}

	// 3. Upgrade TCP connection to TLS using paths injected from ServerConfig
	tlsCert, err := tls.LoadX509KeyPair(s.sslCertFile, s.sslKeyFile)
	if err != nil {
		return err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS12,
	}

	tlsConn := tls.Server(s.conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		return err
	}

	s.conn = tlsConn // Replace raw connection with TLS encrypted connection
	return nil
}
```

### 3. SCRAM-SHA-256 Authentication (`internal/server/auth.go`)

To authenticate users, we negotiate a SASL challenge-response mechanism using stored credentials.

```go
type SASLState int

const (
	SASLInitial SASLState = iota
	SASLChallenge
	SASLCompleted
)

func (s *Session) handleSASLAuth(payload []byte) error {
	// 1. Parse client initial client-first-message
	// 2. Query catalog table to fetch user salt, iteration count, and stored keys
	// 3. Formulate server-first-message containing salt and iteration count
	// 4. Validate client-final-message credentials (ClientSignature)
	// 5. Send AuthenticationSASLFinal containing ServerSignature
	return nil
}

### 4. Global Server-Level Connection Throttle (`internal/server/server.go`)

Throttling must occur globally at the server accept loop level, rather than being handled locally within individual connection routines.

```go
type Server struct {
	listener     net.Listener
	engine       *sql.Engine
	activeConns  atomic.Int64
	maxConns     int64
}

func (srv *Server) AcceptLoop(ctx context.Context) error {
	for {
		conn, err := srv.listener.Accept()
		if err != nil {
			return err
		}

		if srv.activeConns.Load() >= srv.maxConns {
			// Reject immediately at accept loop to prevent session resource leaks
			go srv.rejectConnection(conn)
			continue
		}

		srv.activeConns.Add(1)
		go func() {
			defer srv.activeConns.Add(-1)
			srv.handleConnection(ctx, conn)
		}()
	}
}
```
```

---

## Tasks

1. **Implement TLS/SSL server upgrades:** Load TLS certificate paths from `ServerConfig.SSLCertPath` and `ServerConfig.SSLKeyPath` during server startup, negotiate `SSLRequest` upgrades, and wrap accepted net.Conn into tls.Conn.
2. **Implement SCRAM SASL authentication:** Integrate `github.com/xdg-go/scram` to process client-first/server-first SASL exchanges. Retrieve stored client key salts and iterations from the catalog tables to validate client signatures.
3. **Code Extended Query handler:** Support the full prepared statement lifecycles:
   - **Parse (`'P'`):** Cache queries with parameterized arguments.
   - **Bind (`'B'`):** Read binary or text parameter arrays, convert them to `engine.Datum` parameters, and build named portals.
   - **Execute (`'E'`):** Evaluate the query portal and return rows.
   - **Sync (`'S'`):** Terminate the pipeline message batch.
4. **Implement global server throttling:** Maintain an atomic connection counter (`activeConns`) or buffered channel on the main `Server` struct. In the `Accept()` listener loop, reject connection attempts exceeding `MaxConnections` immediately with a PG error response.
5. **Add Structured Telemetry:** Log connection metrics to `INFO` using slog on connection open/close, detailing connection client IP, auth types, encryption status, and query batch latencies.
6. **Enterprise integration tests:** Integration tests verifying parameterized driver query binding, TLS encryption validation, SASL login success/rejections, and connection pool bounds.

---

## Completion Criteria

- [ ] Clients successfully establish secure TLS encrypted connection links.
- [ ] Database rejects invalid authentication queries and logs SASL challenges.
- [ ] ORMs and parameterized queries execute correctly using the Parse-Bind-Execute pipeline.
- [ ] Overflow connections are rejected with PG error responses when limits are met.
- [ ] Performance logging reports active connections and pipeline latencies.
