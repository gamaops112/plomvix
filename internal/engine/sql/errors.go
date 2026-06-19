package sql

import "errors"

// DML sentinel errors.
var (
	ErrUnsupportedDML          = errors.New("sql engine: unsupported DML statement type")
	ErrBatchInsertUnsupported  = errors.New("sql engine: batch insert not supported")
	ErrInsertSelectUnsupported = errors.New("sql engine: insert select not supported")
	ErrColumnCountMismatch     = errors.New("sql engine: column count mismatch in insert")
	ErrUnknownColumn           = errors.New("sql engine: unknown column")
	ErrDuplicateColumn         = errors.New("sql engine: duplicate column in insert list")
	ErrTypeMismatch            = errors.New("sql engine: type mismatch")
	ErrUnsupportedInsertValue  = errors.New("sql engine: unsupported insert value expression")
	ErrHeapInsertUnsupported   = errors.New("sql engine: target table heap does not support insertions")

	// Enterprise errors.
	ErrBatchTooLarge    = errors.New("sql engine: batch insert exceeds maxBatchSize")
	ErrNotNullViolation = errors.New("sql engine: NOT NULL constraint violation")
	ErrTxConflict       = errors.New("sql engine: WriteTxID monotonic conflict")

	// Constructor validation errors.
	ErrNilCatalog       = errors.New("sql engine: catalog dependency is nil")
	ErrNilTableRegistry = errors.New("sql engine: tableRegistry dependency is nil")
	ErrNilTxManager     = errors.New("sql engine: txManager dependency is nil")
	ErrNilPlanner       = errors.New("sql engine: planner dependency is nil")
	ErrNilLogger        = errors.New("sql engine: logger dependency is nil")
	ErrInvalidBatchSize = errors.New("sql engine: maxBatchSize must be > 0")
)
