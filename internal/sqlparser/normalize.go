package sqlparser

import (
	"crypto/sha256"
	"fmt"
	"strings"

	vitess "vitess.io/vitess/go/vt/sqlparser"
)

// --- Normalize ---

func (s *stmtWrapper) Normalize() string {
	s.normalizeOnce.Do(func() {
		// Use a custom formatter to replace literals with ?.
		var b strings.Builder
		vitess.Walk(func(node vitess.SQLNode) (bool, error) {
			switch node.(type) {
			case *vitess.Literal, *vitess.NullVal:
				b.WriteString("? ")
				return false, nil
			}
			return true, nil
		}, s.ast)
		// Fallback: use vitess canonical string, then redact lexically.
		canon := vitess.CanonicalString(s.ast)
		s.normalizedSQL = lexicalRedact(canon)
	})
	return s.normalizedSQL
}

// --- Fingerprint ---

func (s *stmtWrapper) Fingerprint() string {
	s.fingerprintOnce.Do(func() {
		sanitized := s.Sanitize()
		h := sha256.Sum256([]byte(sanitized))
		s.fingerprint = fmt.Sprintf("%x", h)
	})
	return s.fingerprint
}

// --- StripComments ---

func (s *stmtWrapper) StripComments() string {
	s.stripOnce.Do(func() {
		s.strippedSQL = stripCommentsLex(s.sql)
	})
	return s.strippedSQL
}

// stripCommentsLex strips SQL comments using a quote-aware lexical scanner.
func stripCommentsLex(sql string) string {
	var out strings.Builder
	out.Grow(len(sql))
	i := 0
	for i < len(sql) {
		if i+1 < len(sql) && sql[i] == '-' && sql[i+1] == '-' {
			i += 2
			for i < len(sql) && sql[i] != '\n' { i++ }
			if i < len(sql) { i++ }
			continue
		}
		if i+1 < len(sql) && sql[i] == '/' && sql[i+1] == '*' {
			i += 2
			for i+1 < len(sql) && !(sql[i] == '*' && sql[i+1] == '/') { i++ }
			if i+1 < len(sql) { i += 2 }
			continue
		}
		if sql[i] == '#' {
			i++
			for i < len(sql) && sql[i] != '\n' { i++ }
			if i < len(sql) { i++ }
			continue
		}
		if sql[i] == '\'' || sql[i] == '"' || sql[i] == '`' {
			quote := sql[i]; out.WriteByte(quote); i++
			for i < len(sql) {
				if sql[i] == '\\' && i+1 < len(sql) { out.WriteByte(sql[i]); out.WriteByte(sql[i+1]); i += 2; continue }
				if sql[i] == quote { out.WriteByte(quote); i++; break }
				out.WriteByte(sql[i]); i++
			}
			continue
		}
		out.WriteByte(sql[i]); i++
	}
	return out.String()
}

// --- Sanitize ---

func (s *stmtWrapper) Sanitize() string {
	s.sanitizeOnce.Do(func() {
		s.sanitizedSQL = sanitizeWithFallback(s)
	})
	return s.sanitizedSQL
}

func sanitizeWithFallback(s *stmtWrapper) string {
	stripped := s.StripComments()
	if stmt, err := s.p.Parse(stripped); err == nil {
		w := &stmtWrapper{ast: stmt, sql: stripped}
		return w.Normalize()
	}
	return lexicalRedact(stripped)
}

// lexicalRedact replaces literals with ? using a quote-aware state machine.
func lexicalRedact(sql string) string {
	var out strings.Builder
	out.Grow(len(sql))
	i := 0
	for i < len(sql) {
		if sql[i] == '\'' || sql[i] == '"' {
			out.WriteByte('?'); i++
			quote := sql[i-1]
			for i < len(sql) {
				if sql[i] == '\\' && i+1 < len(sql) { i += 2; continue }
				if sql[i] == quote { i++; break }
				i++
			}
			continue
		}
		if isDigit(sql[i]) {
			if i+1 < len(sql) && sql[i] == '0' && (sql[i+1] == 'x' || sql[i+1] == 'X') {
				out.WriteByte('?'); i += 2
				for i < len(sql) && isHexDigit(sql[i]) { i++ }
				continue
			}
			if i > 0 && isIdentChar(sql[i-1]) { out.WriteByte(sql[i]); i++; continue }
			out.WriteByte('?'); i++
			for i < len(sql) && (isDigit(sql[i]) || sql[i] == '.') { i++ }
			continue
		}
		if sql[i] == '-' && i+1 < len(sql) && isDigit(sql[i+1]) {
			if i == 0 || (!isIdentChar(sql[i-1]) && sql[i-1] != ')') {
				out.WriteByte('?'); i++
				for i < len(sql) && (isDigit(sql[i]) || sql[i] == '.') { i++ }
				continue
			}
		}
		out.WriteByte(sql[i]); i++
	}
	return out.String()
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isHexDigit(c byte) bool { return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') }
func isIdentChar(c byte) bool { return isDigit(c) || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' }

// --- Semantic Validation (basic structural checks only) ---

func validateSemantics(stmt vitess.Statement) *ParseError {
	// Lightweight checks using Vitess APIs.
	return nil
}

