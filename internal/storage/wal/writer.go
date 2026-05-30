package wal

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Writer struct {
	dir            string
	maxSegmentSize int64
	currentFile    *os.File
	currentIndex   uint64
	currentSize    int64
	nextSeqID      uint64
	mu             sync.Mutex
}

func NewWriter(dir string, maxSegmentSize int64) (*Writer, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	w := &Writer{
		dir:            dir,
		maxSegmentSize: maxSegmentSize,
	}

	if err := w.initialize(); err != nil {
		return nil, err
	}

	return w, nil
}

func (w *Writer) initialize() error {
	segments, err := listSegments(w.dir)
	if err != nil {
		return err
	}

	if len(segments) == 0 {
		w.currentIndex = 1
		w.nextSeqID = 1
		return w.openSegment(w.currentIndex)
	}

	sort.Strings(segments)

	w.currentIndex, err = ParseSegmentIndex(segments[len(segments)-1])
	if err != nil {
		return fmt.Errorf("failed to parse last segment filename: %w", err)
	}

	highestSeqID := uint64(0)
	for i := len(segments) - 1; i >= 0; i-- {
		seqID, scanErr := highestSeqIDInSegment(filepath.Join(w.dir, segments[i]))
		if scanErr != nil {
			continue
		}
		if seqID > 0 {
			highestSeqID = seqID
			break
		}
	}
	w.nextSeqID = highestSeqID + 1

	lastPath := filepath.Join(w.dir, SegmentFileName(w.currentIndex))
	f, err := os.OpenFile(lastPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open last segment: %w", err)
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.currentFile = f
	w.currentSize = fi.Size()

	return nil
}

func (w *Writer) openSegment(index uint64) error {
	path := filepath.Join(w.dir, SegmentFileName(index))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open segment %s: %w", path, err)
	}
	w.currentFile = f
	return nil
}

func (w *Writer) rotate() error {
	if err := w.currentFile.Close(); err != nil {
		return fmt.Errorf("failed to close segment during rotation: %w", err)
	}
	w.currentIndex++
	w.currentSize = 0
	return w.openSegment(w.currentIndex)
}

func (w *Writer) Write(dataType DataType, payload []byte) (*Entry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	entry := Entry{
		SeqID:     w.nextSeqID,
		Timestamp: time.Now().UnixNano(),
		DataType:  dataType,
		Payload:   payload,
	}
	w.nextSeqID++
	entry.CRC32 = ComputeCRC32(&entry)

	encoded, err := EncodeEntry(&entry)
	if err != nil {
		return nil, fmt.Errorf("failed to encode WAL entry: %w", err)
	}

	if _, err := w.currentFile.Write(encoded); err != nil {
		return nil, fmt.Errorf("failed to write WAL entry: %w", err)
	}

	if err := w.currentFile.Sync(); err != nil {
		return nil, fmt.Errorf("failed to fsync WAL: %w", err)
	}

	w.currentSize += int64(len(encoded))

	if w.currentSize >= w.maxSegmentSize {
		if err := w.rotate(); err != nil {
			return nil, fmt.Errorf("failed to rotate WAL segment: %w", err)
		}
	}

	return &entry, nil
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.currentFile != nil {
		err := w.currentFile.Close()
		w.currentFile = nil
		return err
	}
	return nil
}

func (w *Writer) CurrentSegmentIndex() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.currentIndex
}

func (w *Writer) CurrentSize() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.currentSize
}

func listSegments(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var segments []string
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > 4 && e.Name()[len(e.Name())-4:] == ".wal" {
			if _, parseErr := ParseSegmentIndex(e.Name()); parseErr == nil {
				segments = append(segments, e.Name())
			}
		}
	}
	return segments, nil
}

func highestSeqIDInSegment(path string) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var highest uint64
	for {
		entry, err := DecodeEntry(f)
		if err != nil {
			break
		}
		if entry.SeqID > highest {
			highest = entry.SeqID
		}
	}
	return highest, nil
}
