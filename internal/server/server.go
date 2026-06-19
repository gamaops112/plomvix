// Package server provides a PostgreSQL Wire Protocol v3.0 compatible
// network server for Plomvix.
package server

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/sqlparser"
)

// ServerConfig holds all configuration for the PG wire protocol server.
type ServerConfig struct {
	Addr           string
	Router         Router
	Parser         sqlparser.Parser
	Logger         *slog.Logger
	SSL            SSLConfig
	MaxConnections int64 // 0 = unlimited
}

// Server is a PG Wire Protocol network server.
type Server struct {
	cfg     ServerConfig
	ln      net.Listener
	router  Router
	parser  sqlparser.Parser
	log     *slog.Logger
	throttle *Throttle

	mu     sync.Mutex
	active sync.WaitGroup
}

// Router is the query dispatch interface the server uses.
type Router interface {
	Route(ctx context.Context, userID uint64, stmt sqlparser.Statement) (*engine.Result, error)
}

// New creates a new PG wire protocol server.
func New(cfg ServerConfig) *Server {
	return &Server{
		cfg:      cfg,
		router:   cfg.Router,
		parser:   cfg.Parser,
		log:      cfg.Logger,
		throttle: NewThrottle(cfg.MaxConnections),
	}
}

// Name returns the component name for lifecycle.Manager.
func (s *Server) Name() string { return "pg-wire-server" }

// Addr returns the listener's network address, or nil if not started.
func (s *Server) Addr() net.Addr {
	if s.ln == nil {
		return nil
	}
	return s.ln.Addr()
}

// Start begins listening on the configured address.
func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("server: listen %s: %w", s.cfg.Addr, err)
	}
	s.ln = ln

	if s.log != nil {
		s.log.Info("pg-wire-server: listening", "addr", s.cfg.Addr)
	}

	go func() {
		<-ctx.Done()
		s.ln.Close()
	}()

	go s.acceptLoop(ctx)
	return nil
}

// Stop gracefully shuts down the server, waiting for active connections.
func (s *Server) Stop(ctx context.Context) error {
	if s.ln != nil {
		_ = s.ln.Close()
	}
	s.active.Wait()
	if s.log != nil {
		s.log.Info("pg-wire-server: stopped")
	}
	return nil
}

func (s *Server) acceptLoop(ctx context.Context) {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
		if !s.throttle.TryAcquire() {
			// Reject: too many connections.
			go func(c net.Conn) {
				w := newPGWriter(c)
				w.WriteErrorResponse("FATAL", "53300", "too many connections")
				c.Close()
			}(conn)
			continue
		}
		s.active.Add(1)
		go func() {
			defer s.active.Done()
			defer s.throttle.Release()
			s.handleConn(ctx, conn)
		}()
	}
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	session := &Session{
		conn:      conn,
		writer:    newPGWriter(conn),
		router:    s.router,
		parser:    s.parser,
		cache:     newSessionCache(),
		sslConfig: s.cfg.SSL,
		log:       s.log,
	}
	if err := session.Run(ctx); err != nil && s.log != nil {
		s.log.Warn("pg-wire-server: session error", "error", err)
	}
}

// Session represents a single client connection.
type Session struct {
	conn      net.Conn
	writer    *pgWriter
	router    Router
	parser    sqlparser.Parser
	cache     *sessionCache
	sslConfig SSLConfig
	log       *slog.Logger
}

// Run processes the client connection lifecycle.
func (s *Session) Run(ctx context.Context) error {
	// 1. Startup handshake (SSL negotiation + authentication + parameters)
	if err := s.handleStartup(); err != nil {
		s.writeError("FATAL", "08000", err.Error())
		return err
	}

	// 2. Main command loop
	buf := make([]byte, 5)
	for {
		if _, err := io.ReadFull(s.conn, buf); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		mType := MsgType(buf[0])
		length := int32(binary.BigEndian.Uint32(buf[1:5])) - 4
		if length < 0 || length > 1<<24 {
			return fmt.Errorf("server: invalid message length %d", length)
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(s.conn, payload); err != nil {
			return err
		}

		switch mType {
		case MsgTerminate:
			return nil
		case MsgQuery:
			if err := s.handleQuery(ctx, payload); err != nil {
				s.writeError("ERROR", "42601", err.Error())
			}
		case MsgParse, MsgBind, MsgExecute, MsgDescribeStmt, MsgSync, MsgCloseStmt:
			if err := s.handleExtendedMessage(ctx, mType, payload); err != nil {
				s.writeError("ERROR", "42601", err.Error())
			}
		default:
			s.writeError("ERROR", "0A000", "unsupported protocol message type")
		}
	}
}

// handleStartup processes the PG startup handshake.
func (s *Session) handleStartup() error {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(s.conn, lenBuf); err != nil {
		return fmt.Errorf("server: startup read: %w", err)
	}
	length := int32(binary.BigEndian.Uint32(lenBuf))
	if length < 4 {
		return errors.New("server: startup message too short")
	}
	payload := make([]byte, length-4)
	if _, err := io.ReadFull(s.conn, payload); err != nil {
		return fmt.Errorf("server: startup payload: %w", err)
	}

	// SSLRequest.
	if len(payload) >= 4 && binary.BigEndian.Uint32(payload[:4]) == 80877103 {
		tlsCfg, err := s.sslConfig.tlsConfig()
		if err == nil && tlsCfg != nil {
			// SSL available: respond 'S', upgrade.
			if _, err := s.conn.Write([]byte{'S'}); err != nil {
				return err
			}
			tlsConn := tls.Server(s.conn, tlsCfg)
			if err := tlsConn.Handshake(); err != nil {
				return fmt.Errorf("server: TLS handshake: %w", err)
			}
			s.conn = tlsConn
			s.writer = newPGWriter(s.conn)
			if s.log != nil {
				s.log.Info("pg-wire-server: TLS established")
			}
		} else {
			// SSL not available: respond 'N', continue unencrypted.
			if err := s.writer.WriteByte(MsgSSLRequestN); err != nil {
				return err
			}
		}
		return s.handleStartup()
	}

	if len(payload) < 4 {
		return errors.New("server: invalid startup message")
	}
	protoVer := binary.BigEndian.Uint32(payload[:4])
	if protoVer != 196608 {
		return fmt.Errorf("server: unsupported protocol version %d", protoVer)
	}

	// Extract username from startup parameters (key=value pairs).
	var username string
	pairs := payload[4:] // skip version
	for i := 0; i+1 < len(pairs); {
		end := -1
		for j := i; j < len(pairs); j++ {
			if pairs[j] == 0 {
				end = j
				break
			}
		}
		if end < 0 {
			break
		}
		key := string(pairs[i:end])
		i = end + 1
		end2 := -1
		for j := i; j < len(pairs); j++ {
			if pairs[j] == 0 {
				end2 = j
				break
			}
		}
		if end2 < 0 {
			break
		}
		value := string(pairs[i:end2])
		i = end2 + 1
		if key == "user" {
			username = value
		}
	}
	_ = username

	// SASL SCRAM-SHA-256 if requested via startup parameter.
	// For now, fall through to trust authentication.
	if err := s.writer.WriteAuthOK(); err != nil {
		return err
	}

	for _, p := range []struct{ k, v string }{
		{"server_version", "15.0.0"},
		{"client_encoding", "UTF8"},
		{"DateStyle", "ISO, YMD"},
		{"standard_conforming_strings", "on"},
	} {
		if err := s.writer.WriteParameterStatus(p.k, p.v); err != nil {
			return err
		}
	}
	if err := s.writer.WriteBackendKeyData(42, 99); err != nil {
		return err
	}
	return s.writer.WriteReadyForQuery('I')
}

// handleQuery processes a Simple Query ('Q') message.
func (s *Session) handleQuery(ctx context.Context, payload []byte) error {
	sql := string(payload)
	if sql == "" {
		return s.writer.WriteReadyForQuery('I')
	}
	return s.executeSQL(ctx, sql)
}

// executeSQL parses and executes a SQL string, streaming results.
func (s *Session) executeSQL(ctx context.Context, sql string) error {
	// Check catalog mock queries first.
	if isCatalogQuery(sql) {
		schema, rows, tag, ok := executeMockCatalog(sql)
		if ok {
			if err := s.writer.WriteRowDescription(schema); err != nil {
				return err
			}
			for _, row := range rows {
				if err := s.writer.WriteDataRow(row); err != nil {
					return err
				}
			}
			if err := s.writer.WriteCommandComplete(tag); err != nil {
				return err
			}
			return s.writer.WriteReadyForQuery('I')
		}
	}

	stmt, err := s.parser.Parse(sql)
	if err != nil {
		return s.writeError("ERROR", "42601", err.Error())
	}

	result, err := s.router.Route(ctx, 0, stmt)
	if err != nil {
		return s.writeError("ERROR", "42000", err.Error())
	}

	if result.Stream != nil {
		schema := result.Stream.Schema()
		if err := s.writer.WriteRowDescription(schema); err != nil {
			return err
		}
		defer result.Stream.Close()
		for {
			row, err := result.Stream.Next(ctx)
			if err != nil {
				break
			}
			if err := s.writer.WriteDataRow(row); err != nil {
				return err
			}
		}
	}

	tag := fmt.Sprintf("SELECT %d", result.RowsAffected)
	if result.RowsAffected == 0 && result.Message != "" {
		tag = result.Message
	}
	if err := s.writer.WriteCommandComplete(tag); err != nil {
		return err
	}
	return s.writer.WriteReadyForQuery('I')
}

func (s *Session) writeError(severity, code, message string) error {
	return s.writer.WriteErrorResponse(severity, code, message)
}
