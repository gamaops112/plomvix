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
)
