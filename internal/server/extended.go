// Package server provides a PostgreSQL Wire Protocol v3.0 compatible server.
// extended.go implements the Extended Query protocol (Parse/Bind/Execute/Sync).
package server

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/plomvix/plomvix/internal/engine"
)

const (
	MsgParse         MsgType = 'P'
	MsgBind          MsgType = 'B'
	MsgExecute       MsgType = 'E'
	MsgDescribeStmt  MsgType = 'D'
	MsgSync          MsgType = 'S'
	MsgCloseStmt     MsgType = 'C'
	MsgParseComplete MsgType = '1'
	MsgBindComplete  MsgType = '2'
	MsgCloseComplete MsgType = '3'
)

// PreparedPlan holds a parsed query with parameter types.
type PreparedPlan struct {
	Name      string
	SQL       string
	ParamOIDs []PGOID
}

// Portal holds a bound query ready for execution.
type Portal struct {
	Name       string
	PlanName   string
	Parameters []engine.Datum
}

type sessionCache struct {
	statements map[string]PreparedPlan
	portals    map[string]Portal
}

func newSessionCache() *sessionCache {
	return &sessionCache{
		statements: make(map[string]PreparedPlan),
		portals:    make(map[string]Portal),
	}
}

// handleExtendedMessage dispatches Extended Query protocol messages.
func (s *Session) handleExtendedMessage(ctx context.Context, mType MsgType, payload []byte) error {
	switch mType {
	case MsgParse:
		return s.handleParse(payload)
	case MsgBind:
		return s.handleBind(payload)
	case MsgExecute:
		return s.handleExecute(ctx, payload)
	case MsgDescribeStmt:
		return s.handleDescribe(payload)
	case MsgSync:
		return s.handleSync()
	case MsgCloseStmt:
		return s.handleClose(payload)
	default:
		return s.writeError("ERROR", "0A000", "unsupported extended query message")
	}
}

// handleParse processes a Parse ('P') message.
// Format: [name\x00][query\x00][numParams:2][for each: OID:4]
func (s *Session) handleParse(payload []byte) error {
	// Read statement name.
	parts := splitNull(payload)
	if len(parts) < 2 {
		return s.writeError("ERROR", "42601", "invalid Parse message")
	}
	name := string(parts[0])
	sql := string(parts[1])

	if len(parts) < 3 {
		return s.writeError("ERROR", "42601", "Parse message too short")
	}
	rest := parts[2]
	var paramOIDs []PGOID
	if len(rest) >= 2 {
		numParams := int(binary.BigEndian.Uint16(rest[:2]))
		oidBytes := rest[2:]
		for i := 0; i < numParams && i*4+4 <= len(oidBytes); i++ {
			oid := PGOID(binary.BigEndian.Uint32(oidBytes[i*4:]))
			paramOIDs = append(paramOIDs, oid)
		}
	}

	s.cache.statements[name] = PreparedPlan{
		Name:      name,
		SQL:       sql,
		ParamOIDs: paramOIDs,
	}

	// Send ParseComplete.
	return s.writer.WritePacket(MsgParseComplete, nil)
}

// handleBind processes a Bind ('B') message.
// Format: [portal\x00][stmt\x00][numFormats:2][format:2]*[numParams:2][paramLen:4][paramBytes]*[numResultFormats:2][format:2]*
func (s *Session) handleBind(payload []byte) error {
	parts := splitNull(payload)
	if len(parts) < 3 {
		return s.writeError("ERROR", "42601", "invalid Bind message")
	}
	portalName := string(parts[0])
	stmtName := string(parts[1])

	plan, ok := s.cache.statements[stmtName]
	if !ok && stmtName != "" {
		return s.writeError("ERROR", "26000", "prepared statement does not exist")
	}

	// Parse parameter formats and values from the remaining bytes.
	rest := parts[2]
	var params []engine.Datum

	if len(rest) >= 2 {
		numParamFormats := int(binary.BigEndian.Uint16(rest[:2]))
		pos := 2 + numParamFormats*2 // skip format codes

		if pos+2 <= len(rest) {
			numParams := int(binary.BigEndian.Uint16(rest[pos:]))
			pos += 2
			for i := 0; i < numParams && pos+4 <= len(rest); i++ {
				paramLen := int32(binary.BigEndian.Uint32(rest[pos:]))
				pos += 4
				if paramLen == -1 {
					params = append(params, engine.Datum{Type: engine.TypeNull, Value: nil})
				} else if pos+int(paramLen) <= len(rest) {
					paramBytes := rest[pos : pos+int(paramLen)]
					pos += int(paramLen)
					// Simple text→datum conversion.
					d := textToDatum(string(paramBytes))
					params = append(params, d)
				}
			}
		}
	}
	_ = plan

	s.cache.portals[portalName] = Portal{
		Name:       portalName,
		PlanName:   stmtName,
		Parameters: params,
	}

	return s.writer.WritePacket(MsgBindComplete, nil)
}

// handleExecute processes an Execute ('E') message.
// Format: [portal\x00][maxRows:4]
func (s *Session) handleExecute(ctx context.Context, payload []byte) error {
	parts := splitNull(payload)
	if len(parts) < 1 {
		return s.writeError("ERROR", "42601", "invalid Execute message")
	}
	portalName := string(parts[0])

	portal, ok := s.cache.portals[portalName]
	if !ok {
		return s.writeError("ERROR", "34000", "portal does not exist")
	}

	plan, ok := s.cache.statements[portal.PlanName]
	if !ok {
		return s.writeError("ERROR", "26000", "prepared statement does not exist")
	}

	// Substitute parameters into the SQL and execute.
	sql := plan.SQL
	for i, param := range portal.Parameters {
		placeholder := fmt.Sprintf("$%d", i+1)
		sql = strings.Replace(sql, placeholder, datumToText(param), -1)
	}

	return s.executeSQL(ctx, sql)
}

// handleDescribe processes a Describe ('D') message.
func (s *Session) handleDescribe(_ []byte) error {
	// Respond with NoData indicator.
	return s.writer.WritePacket('n', nil)
}

// handleSync processes a Sync ('S') message and sends ReadyForQuery.
func (s *Session) handleSync() error {
	return s.writer.WriteReadyForQuery('I')
}

// handleClose processes a Close ('C') message.
func (s *Session) handleClose(payload []byte) error {
	_ = payload
	return s.writer.WritePacket(MsgCloseComplete, nil)
}

// splitNull splits bytes on null bytes. The last empty element is dropped per PG convention.
func splitNull(b []byte) [][]byte {
	parts := strings.Split(string(b), "\x00")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	var result [][]byte
	for _, p := range parts {
		result = append(result, []byte(p))
	}
	return result
}

// textToDatum converts a text string to an engine.Datum.
func textToDatum(s string) engine.Datum {
	// Try int64 first, then float, then string.
	var i int64
	if n, err := fmt.Sscanf(s, "%d", &i); n == 1 && err == nil {
		return engine.Datum{Type: engine.TypeInt64, Value: i}
	}
	var f float64
	if n, err := fmt.Sscanf(s, "%f", &f); n == 1 && err == nil {
		return engine.Datum{Type: engine.TypeFloat64, Value: f}
	}
	return engine.Datum{Type: engine.TypeString, Value: s}
}
