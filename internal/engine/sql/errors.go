package sql

import (
	"errors"

	"github.com/plomvix/plomvix/internal/engine"
)

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

	// Mutation errors.
	ErrWhereRequired                 = errors.New("sql engine: UPDATE and DELETE require a WHERE clause")
	ErrHeapMutationUnsupported       = errors.New("sql engine: target table heap does not support mutation")
	ErrMissingRowID                  = engine.ErrMissingRowID // alias from engine package
	ErrStaleRowID                    = errors.New("sql engine: RowID is stale; heap generation has advanced (vacuum ran)")
	ErrWriteConflict                 = errors.New("sql engine: write-write conflict detected; concurrent transaction modified row")
	ErrVacuumBlockedByActivePins     = errors.New("sql engine: vacuum compaction blocked by active DML pins")
	ErrMultiRowMutationUnsupported   = errors.New("sql engine: multi-row mutation not supported in basic tier")
	ErrUnsupportedWhereExpr          = errors.New("sql engine: unsupported WHERE expression")
	ErrUnsupportedSetValue           = errors.New("sql engine: unsupported SET value expression")
	ErrRowNotFound                   = errors.New("sql engine: no row found matching predicate")
	ErrMutationLimitExceeded         = errors.New("sql engine: mutation row limit exceeded")
	ErrDeleteAllRequiresConfirmation = errors.New("sql engine: full-table DELETE requires AllowFullTableDelete opt-in")

	// Constructor validation errors.
	ErrNilCatalog       = errors.New("sql engine: catalog dependency is nil")
	ErrNilTableRegistry = errors.New("sql engine: tableRegistry dependency is nil")
	ErrNilTxManager     = errors.New("sql engine: txManager dependency is nil")
	ErrNilPlanner       = errors.New("sql engine: planner dependency is nil")
	ErrNilLogger        = errors.New("sql engine: logger dependency is nil")
	ErrInvalidBatchSize = errors.New("sql engine: maxBatchSize must be > 0")
)
