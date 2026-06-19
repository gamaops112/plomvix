# Wire Protocol and API Server Setup

| Field | Value |
| :--- | :--- |
| **Source** | `agent/wire_setup.md` |
| **Package(s)** | `internal/server` |
| **Purpose** | Implement basic PostgreSQL Wire Protocol v3.0 compatibility to allow standard tools and drivers to connect to Plomvix, executing queries via the Simple Query protocol and returning mock responses for standard system catalog probes. |
| **Dependencies** | Lifecycle, Parser, Router/Planner, DML/Joins/Sort/Agg execution plans. |

## Honest Contracts & Known Trade-offs

1. **Simple Query Protocol Only in Basic Tier:** The basic tier only supports the Simple Query protocol (single client packet containing raw SQL strings under message type `'Q'`). Prepared statements, parameterized queries, and message pipelining (Extended Query Protocol) are rejected with protocol-level error responses.
2. **Trust (Passwordless) Authentication:** For the basic tier, the database operates under a "trust" authentication model (all connection requests from any username are accepted without password validation).
3. **No TLS/SSL Support in Basic Tier:** If a client requests an SSL handshake (message type `SSLRequest`), the server responds with `'N'` (SSL not supported) and proceeds with an unencrypted connection.
4. **Mocked Catalog Table Boundaries:** Many PG drivers and client applications execute system queries against `pg_catalog` tables (e.g. `pg_type`, `pg_settings`, `pg_database`) and built-in functions (e.g. `version()`, `current_schema()`) immediately upon startup. The planner must intercept these query patterns and return basic, hardcoded mock results to prevent connection failures.
5. **Session-Level Stateful Connection Mapping:** Each TCP connection represents a dedicated session. Connection-specific session states (like transaction boundaries and schema pins) are tracked using a local session context struct.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/server/server.go` | Implement the TCP listener loop, startup handshake parser, and client session dispatcher. |
| `internal/server/protocol.go` | Define PG v3 message frames (e.g. RowDescription, DataRow, CommandComplete, ReadyForQuery) and serialization logic. |
| `internal/server/catalog_mock.go` | Implement mock resolvers for basic PG catalog system table queries and functions. |
| `internal/server/server_test.go` | Integration tests verifying connection setup, query execution, and schema catalog mockup outputs using standard PG client drivers. |

---

## Key API & Concepts

### 1. PG Message Frame Formats (`internal/server/protocol.go`)

The PostgreSQL v3 protocol transmits messages prefixed by a 1-byte message type identifier followed by a 4-byte message length header (inclusive of the length field but exclusive of the type byte).

```go
package server

import (
	"encoding/binary"
	"io"
)

type MsgType byte

const (
	MsgQuery            MsgType = 'Q'
	MsgRowDescription   MsgType = 'T'
	MsgDataRow          MsgType = 'D'
	MsgCommandComplete  MsgType = 'C'
	MsgReadyForQuery    MsgType = 'Z'
	MsgAuthenticationRequest MsgType = 'R'
	MsgErrorResponse    MsgType = 'E'
)

// pgWriter wraps a raw network connection and formats output packets.
type pgWriter struct {
	w io.Writer
}

// WritePacket formats and transmits a standard PG packet frame.
func (pw *pgWriter) WritePacket(t MsgType, payload []byte) error {
	length := int32(len(payload) + 4)
	header := make([]byte, 5)
	header[0] = byte(t)
	binary.BigEndian.PutUint32(header[1:5], uint32(length))
	
	if _, err := pw.w.Write(header); err != nil {
		return err
	}
	_, err := pw.w.Write(payload)
	return err
}
```

### 2. Session Connection Lifecycle (`internal/server/server.go`)

Every client connection is assigned a Go routine handling the lifecycle and session-level command routing. To prevent protocol desync, the initial handshake (which lacks a 1-byte message type prefix) is processed first, before entering the standard 5-byte message frame loop.

```go
type Session struct {
	conn      net.Conn
	writer    *pgWriter
	engine    *sql.Engine
	inTx      bool
	currentDB string
}

func (s *Session) Run(ctx context.Context) error {
	defer s.conn.Close()

	// 1. Process initial handshake (reads 8-byte chunk for SSLRequest or StartupMessage length)
	if err := s.handleHandshake(); err != nil {
		s.sendError(err)
		return err
	}

	// 2. Main stateful command loop
	buf := make([]byte, 5)
	for {
		// Read message header (1-byte type + 4-byte length)
		if _, err := io.ReadFull(s.conn, buf); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		mType := MsgType(buf[0])
		length := int32(binary.BigEndian.Uint32(buf[1:5])) - 4
		
		payload := make([]byte, length)
		if _, err := io.ReadFull(s.conn, payload); err != nil {
			return err
		}

		// Handle client terminate message
		if mType == 'X' { // Terminate message
			return nil // Graceful close
		}

		if err := s.handleMessage(ctx, mType, payload); err != nil {
			s.sendError(err)
		}
	}
}

func (s *Session) handleHandshake() error {
	// Read 4-byte length header
	buf := make([]byte, 4)
	if _, err := io.ReadFull(s.conn, buf); err != nil {
		return err
	}
	length := int32(binary.BigEndian.Uint32(buf))
	
	// Read payload (length includes the length field itself, so we read length - 4 bytes)
	payload := make([]byte, length-4)
	if _, err := io.ReadFull(s.conn, payload); err != nil {
		return err
	}

	// Negotiate SSL Request
	if len(payload) == 4 && binary.BigEndian.Uint32(payload) == 80877103 {
		// SSLRequest: basic tier responds with 'N' (no SSL) and retries handshake
		if _, err := s.conn.Write([]byte{'N'}); err != nil {
			return err
		}
		return s.handleHandshake()
	}

	// Parse StartupMessage (v3.0 is 196608)
	if len(payload) < 4 {
		return errors.New("invalid startup message length")
	}
	protocolVersion := binary.BigEndian.Uint32(payload[0:4])
	if protocolVersion != 196608 {
		return fmt.Errorf("unsupported protocol version: %d", protocolVersion)
	}

	// Extract database and user parameters (null-terminated string pairs key\x00value\x00...)
	// ... (parse key/value parameters)

	// Send AuthOK (AuthTrust code = 0)
	if err := s.writer.WritePacket(MsgAuthenticationRequest, []byte{0, 0, 0, 0}); err != nil {
		return err
	}

	// Send mandatory ParameterStatus ('S') settings to satisfy client drivers
	params := []struct{ k, v string }{
		{"server_version", "15.0.0"},
		{"client_encoding", "UTF8"},
		{"DateStyle", "ISO, YMD"},
		{"standard_conforming_strings", "on"},
	}
	for _, p := range params {
		payload := append([]byte(p.k), 0)
		payload = append(payload, []byte(p.v)...)
		payload = append(payload, 0)
		if err := s.writer.WritePacket('S', payload); err != nil {
			return err
		}
	}

	// Send BackendKeyData ('K') - 4-byte PID + 4-byte secret key
	keyData := make([]byte, 8)
	binary.BigEndian.PutUint32(keyData[0:4], 42) // Mock PID
	binary.BigEndian.PutUint32(keyData[4:8], 99) // Mock Key
	if err := s.writer.WritePacket('K', keyData); err != nil {
		return err
	}

	// Send ReadyForQuery ('Z') with idle transaction indicator ('I')
	return s.writer.WritePacket(MsgReadyForQuery, []byte{'I'})
}
```

### 3. Row Serialization & PG OID Mappings (`internal/server/protocol.go`)

To return data, column schemas must map Plomvix types to standard Postgres Object IDs (OIDs) and encode row datums as text strings.

```go
type PGOID int32

const (
	OIDBool    PGOID = 16
	OIDInt64   PGOID = 20
	OIDFloat64 PGOID = 701
	OIDString  PGOID = 1043
	OIDNull    PGOID = 0
)

func engineTypeToOID(t engine.DataType) PGOID {
	switch t {
	case engine.TypeBool:
		return OIDBool
	case engine.TypeInt64:
		return OIDInt64
	case engine.TypeFloat64:
		return OIDFloat64
	case engine.TypeString:
		return OIDString
	default:
		return OIDNull
	}
}

// SerializeRowDescription serializes schema column metadata.
func (pw *pgWriter) SerializeRowDescription(schema engine.Schema) error {
	// Format 'T' (RowDescription) packet payload
	// [2-bytes: col count]
	// Loop:
	//   [String: column name \x00]
	//   [4-bytes: Table OID (0)]
	//   [2-bytes: Column Attribute index (0)]
	//   [4-bytes: Type OID]
	//   [2-bytes: Type Size]
	//   [4-bytes: Type Modifier (-1)]
	//   [2-bytes: Format Code (0 = Text)]
	return nil
}

// SerializeDataRow serializes query row values.
func (pw *pgWriter) SerializeDataRow(row engine.Row) error {
	// Format 'D' (DataRow) packet payload
	// [2-bytes: field count]
	// Loop:
	//   [4-bytes: field length, -1 for NULL]
	//   [Bytes: field data serialized as text]
	return nil
}

// SendErrorResponse formats error details inside byte-identified null-terminated fields.
func (pw *pgWriter) SendErrorResponse(severity, code, message string) error {
	var payload []byte
	
	// Severity field 'S'
	payload = append(payload, 'S')
	payload = append(payload, []byte(severity)...)
	payload = append(payload, 0)
	
	// SQLSTATE Code 'C'
	payload = append(payload, 'C')
	payload = append(payload, []byte(code)...)
	payload = append(payload, 0)
	
	// Message field 'M'
	payload = append(payload, 'M')
	payload = append(payload, []byte(message)...)
	payload = append(payload, 0)
	
	payload = append(payload, 0) // Final terminating null byte
	
	return pw.WritePacket(MsgErrorResponse, payload)
}
```

### 4. Basic Catalog Mocking (`internal/server/catalog_mock.go`)

To support client startup validations (such as `SELECT version()`), query inputs are checked against common catalog names and intercepted:

```go
func isCatalogQuery(sql string) bool {
	// Detect queries selecting from pg_catalog or using built-in metadata functions
	normalized := strings.ToLower(sql)
	return strings.Contains(normalized, "pg_catalog") || strings.Contains(normalized, "version()")
}

func executeMockCatalog(sql string) (engine.Schema, []engine.Row, error) {
	// Return static dataset mappings representing Postgres version strings, 
	// standard parameters (client_encoding, DateStyle), or mock types.
	return engine.Schema{}, nil, nil
}
```

---

## Tasks

1. **Implement server listener and lifecycle:** Setup the TCP socket server in `internal/server` and register it with `internal/lifecycle` to support graceful shutdowns.
2. **Parse startup packets with SSL negotiation:** Implement `handleHandshake()` to read the initial 8-byte length & version chunk before the main message loop. Respond with `'N'` for `SSLRequest` packets, extract database name parameters from `StartupMessage`, and send the AuthenticationOK (AuthTrust, code `0`) message.
3. **Send mandatory startup packets:** Following AuthOK, write `ParameterStatus` (`'S'`) packets detailing `server_version`, `client_encoding`, `DateStyle`, and `standard_conforming_strings`, followed by `BackendKeyData` (`'K'`) and `ReadyForQuery` (`'Z'`).
4. **Implement robust message frames and error formatting:** Build PG message serializers for `RowDescription` (`'T'`), `DataRow` (`'D'`), and `ErrorResponse` (`'E'`). Format the Error Response using byte-identified, null-terminated fields (e.g. `'S'`, `'C'`, `'M'`).
5. **Implement Simple Query Processor & Terminate:** Code command routing for message type `'Q'`:
   - Send the query string to the executor.
   - If the query is a catalog query, route it to `executeMockCatalog`.
   - Stream row outputs via `RowDescription` and `DataRow` packets.
   - Conclude query execution by writing `CommandComplete` and `ReadyForQuery`.
   - Handle `'X'` (Terminate) message loops gracefully by closing the session loop.
6. **Implement basic PG Catalog mockup:** Create metadata responses for standard setup queries:
   - `SELECT version();`
   - `SELECT pg_catalog.pg_settings...;`
   - `SHOW ALL` or standard encoding checks (`client_encoding`, `standard_conforming_strings`).
7. **Integration Verification tests:** Write tests using a standard PostgreSQL driver (like `pgx` in Go) that connects to Plomvix, performs basic queries (select, insert, join), and asserting successful client-to-server data transmission.

---

## Completion Criteria

- [ ] Plomvix boots a network TCP listener.
- [ ] Standard Postgres drivers handshake and authenticate successfully under passwordless trust.
- [ ] Standard `psql` shell connects and displays SQL query outputs.
- [ ] PG catalog queries like `version()` return mock metadata successfully without failing startup routines.
- [ ] Connections are cleaned up gracefully when client closes sessions or during server shutdown.
