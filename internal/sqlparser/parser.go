// Package sqlparser provides the Global SQL Parser for Plomvix. It wraps the
// Vitess SQL parser to convert raw SQL text into a strongly-typed AST with
// engine-agnostic metadata (statement type, target tables) for downstream
// routing and planning.
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
}

// SyntaxError represents a fail-fast parsing error with location tracking.
type SyntaxError struct {
	Message string
	Line    int
	Column  int
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("syntax error at line %d:%d: %s", e.Line, e.Column, e.Message)
}

// Sentinel errors.
var (
	ErrEmptySQL = errors.New("sqlparser: empty SQL statement")
)

// Parser is the global SQL parsing service.
type Parser interface {
	Parse(sql string) (Statement, error)
	ParseMulti(sql string) ([]Statement, error)
}

// vitessParser implements Parser using Vitess functions.
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

// stmtWrapper wraps a Vitess AST node.
type stmtWrapper struct {
	ast vitess.Statement
	sql string
}

var _ Parser = (*vitessParser)(nil)
var _ Statement = (*stmtWrapper)(nil)
