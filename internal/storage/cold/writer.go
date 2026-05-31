package cold

import (
	"fmt"
	"os"
	"path/filepath"

	parquetgo "github.com/parquet-go/parquet-go"
)

// Writer writes ParquetRows to a single Parquet part file.
type Writer struct {
	path   string
	file   *os.File
	writer *parquetgo.GenericWriter[ParquetRow]
}

// NewWriter creates a new Parquet writer at the given path.
// Creates all parent directories if they do not exist.
func NewWriter(path string) (*Writer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("failed to create parquet directory: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create parquet file %s: %w", path, err)
	}
	return &Writer{
		path:   path,
		file:   f,
		writer: parquetgo.NewGenericWriter[ParquetRow](f),
	}, nil
}

// WriteRow writes a single row.
func (w *Writer) WriteRow(row ParquetRow) error {
	_, err := w.writer.Write([]ParquetRow{row})
	return err
}

// Close flushes and closes the file. Must be called after all rows are written.
func (w *Writer) Close() error {
	if err := w.writer.Close(); err != nil {
		w.file.Close()
		return fmt.Errorf("failed to close parquet writer: %w", err)
	}
	return w.file.Close()
}

// Path returns the file path.
func (w *Writer) Path() string { return w.path }
