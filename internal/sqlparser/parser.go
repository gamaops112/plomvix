// Package sqlparser provides the Global SQL Parser for Plomvix. It wraps the
// Vitess SQL parser to convert raw SQL text into a strongly-typed AST with
// engine-agnostic metadata for downstream routing and planning.
//
// Enterprise tier adds: ParseError with byte offsets, non-mutating normalization,
// SHA-256 fingerprinting, quote-aware sanitization with lexical fallback,
// multi-error recovery (ParseScript), and lightweight semantic pre-validation.
//
// Dialect: Vitess v0.24+ parses MySQL-compatible SQL.
package sqlparser

import (
	"errors"
	"fmt"

	vitess "vitess.io/vitess/go/vt/sqlparser"
)

// StmtType classifies the parsed SQL statement.
type StmtType int

const (
	StmtSelect StmtType = iota
	StmtInsert
	StmtUpdate
	StmtDelete
	StmtDDL
	StmtUnknown
)

// Statement is the engine-agnostic wrapper around a parsed SQL AST.
type Statement interface {
	Type() StmtType
	TargetTables() []string
	RawAST() any
	String() string

	// Enterprise methods.
	Normalize() string
	Fingerprint() string
	Sanitize() string
	StripComments() string

	// DDL accessor (returns nil for non-DDL statements).
	RawDDL() vitess.DDLStatement
	// DML accessors.
	RawInsert() *vitess.Insert
	RawUpdate() *vitess.Update
	RawDelete() *vitess.Delete
}

// ParseError supersedes the Basic tier's SyntaxError with byte offset support.
type ParseError struct {
	Message string
	Line    int    // 1-based
	Column  int    // 1-based, rune count
	Offset  int    // byte offset; -1 if unknown
	Kind    string // "syntax" or "semantic"
	Cause   error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d, column %d (%s): %s", e.Line, e.Column, e.Kind, e.Message)
}

func (e *ParseError) Unwrap() error { return e.Cause }

// Sentinel errors.
var (
	ErrEmptySQL           = errors.New("sqlparser: empty SQL statement")
	ErrSemanticValidation = errors.New("sqlparser: semantic validation failed")
)

// Parser is the global SQL parsing service.
type Parser interface {
	Parse(sql string) (Statement, error)
	ParseMulti(sql string) ([]Statement, error)
	ParseScript(sql string) ([]Statement, []*ParseError)
}

// vitessParser implements Parser using Vitess.
type vitessParser struct {
	p *vitess.Parser
}

// New creates a new Parser backed by Vitess.
func New() (Parser, error) {
	p, err := vitess.New(vitess.Options{})
	if err != nil {
		return nil, fmt.Errorf("sqlparser: create Vitess parser: %w", err)
	}
	return &vitessParser{p: p}, nil
}

var _ Parser = (*vitessParser)(nil)
