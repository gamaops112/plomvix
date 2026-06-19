package sqlparser

import (
	"sync"

	vitess "vitess.io/vitess/go/vt/sqlparser"
)

// stmtWrapper wraps a Vitess AST node with enterprise caching.
type stmtWrapper struct {
	ast vitess.Statement
	sql string
	p   *vitess.Parser // for Sanitize re-parse

	normalizeOnce   sync.Once
	normalizedSQL   string
	fingerprintOnce sync.Once
	fingerprint     string
	stripOnce       sync.Once
	strippedSQL     string
	sanitizeOnce    sync.Once
	sanitizedSQL    string
}

var _ Statement = (*stmtWrapper)(nil)
