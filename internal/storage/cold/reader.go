package cold

import (
	"fmt"
	"io"
	"os"

	parquetgo "github.com/parquet-go/parquet-go"
)

// ReadFile reads all rows from a single Parquet file.
func ReadFile(path string) ([]ParquetRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open parquet file %s: %w", path, err)
	}
	defer f.Close()

	reader := parquetgo.NewGenericReader[ParquetRow](f)
	defer reader.Close()

	var rows []ParquetRow
	buf := make([]ParquetRow, 1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			rows = append(rows, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read parquet file %s: %w", path, err)
		}
	}
	return rows, nil
}

// ReadFileRange reads rows in [fromNs, toNs). Pass 0 for both to read all rows.
func ReadFileRange(path string, fromNs, toNs int64) ([]ParquetRow, error) {
	all, err := ReadFile(path)
	if err != nil {
		return nil, err
	}
	if fromNs == 0 && toNs == 0 {
		return all, nil
	}
	var filtered []ParquetRow
	for _, row := range all {
		if fromNs > 0 && row.TimestampNs < fromNs {
			continue
		}
		if toNs > 0 && row.TimestampNs >= toNs {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered, nil
}
