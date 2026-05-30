package wal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"go.uber.org/zap"

	"github.com/plomvix/plomvix/internal/logger"
)

type Reader struct {
	dir      string
	segments []string
}

func NewReader(dir string) (*Reader, error) {
	segments, err := listSegments(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to list WAL segments: %w", err)
	}
	sort.Strings(segments)
	return &Reader{dir: dir, segments: segments}, nil
}

func (r *Reader) ReadAll() ([]*Entry, error) {
	var result []*Entry

	for _, segName := range r.segments {
		path := filepath.Join(r.dir, segName)
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open WAL segment %s: %w", segName, err)
		}

		for {
			entry, decErr := DecodeEntry(f)
			if decErr != nil {
				if decErr == io.EOF {
					break
				}
				if decErr == ErrCorruptEntry {
					logger.Warn("corrupt WAL entry — stopping read of this segment",
						zap.String("segment", segName))
					break
				}
				f.Close()
				return nil, fmt.Errorf("I/O error reading segment %s: %w", segName, decErr)
			}
			result = append(result, entry)
		}

		f.Close()
	}

	return result, nil
}

func (r *Reader) SegmentCount() int {
	return len(r.segments)
}
