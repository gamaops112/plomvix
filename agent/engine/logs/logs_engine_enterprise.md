# Logs Engine Enterprise (Compression, Tokenization, and Retention)

| Field | Value |
| :--- | :--- |
| **Source** | `agent/engine/logs/logs_engine_enterprise.md` |
| **Package(s)** | `internal/engine/logs` |
| **Purpose** | Implement full-text log tokenization and inverted index searches, block-level compression (ZSTD/LZ4), concurrent ingestion locking, and automated log retention cleanup workers. |
| **Dependencies** | Logs Engine Setup plan. |

## Honest Contracts & Known Trade-offs

1. **In-Memory Token Index Memory Bounding:** The inverted text index is kept in memory. To prevent high-cardinality OOM crashes (e.g. from unique UUIDs, hashes, or timestamps in log bodies), the engine only indexes log records within a configurable recent time window (e.g., the last 12 hours) and applies an LRU eviction policy for index keys. Its footprint is strictly capped via `LogIndexMaxMemoryMB`.
2. **Read Lock Concurrency:** Concurrent log `INSERT` operations acquire a **Read Lock** on a shared `sync.RWMutex`, ensuring high ingestion throughput. The background flusher and index indexers acquire the **Write Lock** only when swapping active page buffers, keeping lock durations in the sub-millisecond range.
3. **Block Compression Latency Trade-off:** Compressing log records via ZSTD (or gzip/deflate fallback) reduces disk consumption by up to 10x, but introduces minor CPU decompression latency and memory allocation overhead at query time.
4. **Coarse Retention Deletion:** The retention worker deletes historical logs by freeing raw page blocks where the maximum timestamp is older than the configured threshold (e.g., `LogsRetentionDays = 7`). Deletions are coarse-grained (page-level), so some points slightly newer than the boundary may be freed if they reside on the same page block.
5. **Tombstone Tolerance:** The `executeSelect` scan loop must explicitly catch `ErrInvalidPageID` from the Pager and silently skip the record. This acknowledges that the Token Index is eventually consistent and may point to pages that have been reclaimed by the Retention Worker.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/engine/logs/token.go` | Implement the tokenization scanner splitting log bodies into lowercase alphanumeric search terms. |
| `internal/engine/logs/index.go` | Implement the concurrent in-memory inverted token index and page-level Bloom filter checks. |
| `internal/engine/logs/compress.go` | Create the block compression layer utilizing ZSTD or standard deflate compression on page blocks. |
| `internal/engine/logs/retention.go` | Implement the background log retention worker that periodically sweeps and frees expired page blocks. |

---

## Key API & Concepts

### 1. Log Tokenizer (`internal/engine/logs/token.go`)

Splits raw message text or JSON payloads into search terms by breaking on non-alphanumeric boundaries (spaces, punctuation, brackets) and lowercasing:

```go
package logs

import (
	"strings"
	"unicode"
)

// Tokenize splits text into unique alphanumeric search tokens
func Tokenize(text string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(unicode.ToLower(r))
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return deduplicate(tokens)
}

func deduplicate(in []string) []string {
	m := make(map[string]struct{})
	var out []string
	for _, s := range in {
		if len(s) < 2 { // skip tiny noise tokens
			continue
		}
		if _, ok := m[s]; !ok {
			m[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
```

### 2. Inverted Token Index (`internal/engine/logs/index.go`)

Logs are indexed by terms. To manage memory, the index enforces memory limit constraints using a Least Recently Used (LRU) key eviction mechanism.

```go
package logs

import (
	"container/list"
	"strings"
	"sync"
)

type RecordLocator struct {
	BlockPageID uint64
	RecordIndex uint32
}

type TokenIndex struct {
	mu          sync.RWMutex
	termLocs    map[string][]RecordLocator
	lruList     *list.List
	lruMap      map[string]*list.Element
	maxMemBytes int64
	curMemBytes int64
}

func NewTokenIndex(maxMemBytes int64) *TokenIndex {
	return &TokenIndex{
		termLocs:    make(map[string][]RecordLocator),
		lruList:     list.New(),
		lruMap:      make(map[string]*list.Element),
		maxMemBytes: maxMemBytes,
	}
}

func (idx *TokenIndex) Insert(token string, loc RecordLocator) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	term := strings.ToLower(token)
	locs := idx.termLocs[term]

	// Estimate added size overhead: 16 bytes per RecordLocator entry
	addedSize := int64(16)
	if len(locs) == 0 {
		// Track key string overhead (approx. length + map overhead)
		addedSize += int64(len(term)) + 64
	}

	// Enforce memory boundary by evicting oldest keys
	for idx.curMemBytes+addedSize >= idx.maxMemBytes && idx.lruList.Len() > 0 {
		idx.evictOldest()
	}

	// Graceful degradation: if still over limit, drop indexing for this record locator
	if idx.curMemBytes+addedSize >= idx.maxMemBytes {
		return
	}

	idx.termLocs[term] = append(locs, loc)
	idx.curMemBytes += addedSize

	// Update LRU ordering
	if elem, ok := idx.lruMap[term]; ok {
		idx.lruList.MoveToFront(elem)
	} else {
		elem := idx.lruList.PushFront(term)
		idx.lruMap[term] = elem
	}
}

func (idx *TokenIndex) evictOldest() {
	elem := idx.lruList.Back()
	if elem == nil {
		return
	}
	term := elem.Value.(string)
	locs := idx.termLocs[term]

	freedSize := int64(len(locs))*16 + int64(len(term)) + 64
	delete(idx.termLocs, term)
	delete(idx.lruMap, term)
	idx.lruList.Remove(elem)

	idx.curMemBytes -= freedSize
}

func (idx *TokenIndex) Search(token string) []RecordLocator {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	term := strings.ToLower(token)
	locs, exists := idx.termLocs[term]
	if !exists {
		return nil
	}
	copied := make([]RecordLocator, len(locs))
	copy(copied, locs)
	return copied
}
```

### 3. Block Compression Layout (`internal/engine/logs/compress.go`)

Log records are packed into blocks and compressed using ZSTD. To bypass the Pager's single-page size limits, a block-level reader/writer layer manages multi-page chunking.

```go
package logs

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/plomvix/plomvix/internal/storage/pager"
)

const (
	BlockMagic      = 0x4C4F4743 // 'LOGC'
	BlockHeaderSize = 32
	ChunkSize       = 4084 // Maximum page payload length (PageSize - HeaderSize)
)

type BlockHeader struct {
	Magic              uint32
	UncompressedLength uint32
	CompressedLength   uint32
	RecordCount        uint32
	MinTimestamp       int64
	MaxTimestamp       int64
}

// CompressBlock serializes raw records, compresses them using ZSTD, and prepends the header.
func CompressBlock(rawBytes []byte, header *BlockHeader) ([]byte, error) {
	var buf bytes.Buffer
	encoder, err := zstd.NewWriter(&buf)
	if err != nil {
		return nil, err
	}
	if _, err := encoder.Write(rawBytes); err != nil {
		encoder.Close()
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}

	compressedBytes := buf.Bytes()
	header.Magic = BlockMagic
	header.UncompressedLength = uint32(len(rawBytes))
	header.CompressedLength = uint32(len(compressedBytes))

	out := make([]byte, BlockHeaderSize+len(compressedBytes))
	binary.BigEndian.PutUint32(out[0:4], header.Magic)
	binary.BigEndian.PutUint32(out[4:8], header.UncompressedLength)
	binary.BigEndian.PutUint32(out[8:12], header.CompressedLength)
	binary.BigEndian.PutUint32(out[12:16], header.RecordCount)
	binary.BigEndian.PutUint64(out[16:24], uint64(header.MinTimestamp))
	binary.BigEndian.PutUint64(out[24:32], uint64(header.MaxTimestamp))
	copy(out[BlockHeaderSize:], compressedBytes)

	return out, nil
}

// DecompressBlock verifies the magic header and decompresses the ZSTD payload.
func DecompressBlock(compressedBlock []byte) ([]byte, *BlockHeader, error) {
	if len(compressedBlock) < BlockHeaderSize {
		return nil, nil, fmt.Errorf("block bytes smaller than header size")
	}
	header := &BlockHeader{
		Magic:              binary.BigEndian.Uint32(compressedBlock[0:4]),
		UncompressedLength: binary.BigEndian.Uint32(compressedBlock[4:8]),
		CompressedLength:   binary.BigEndian.Uint32(compressedBlock[8:12]),
		RecordCount:        binary.BigEndian.Uint32(compressedBlock[12:16]),
		MinTimestamp:       int64(binary.BigEndian.Uint64(compressedBlock[16:24])),
		MaxTimestamp:       int64(binary.BigEndian.Uint64(compressedBlock[24:32])),
	}
	if header.Magic != BlockMagic {
		return nil, nil, fmt.Errorf("invalid block magic: %x", header.Magic)
	}

	decoder, err := zstd.NewReader(bytes.NewReader(compressedBlock[BlockHeaderSize : BlockHeaderSize+header.CompressedLength]))
	if err != nil {
		return nil, nil, err
	}
	defer decoder.Close()

	decompressed := make([]byte, header.UncompressedLength)
	if _, err := io.ReadFull(decoder, decompressed); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, nil, err
	}
	return decompressed, header, nil
}

// BlockWriter chunk-writes compressed block bytes into 4084-byte physical pages using Pager.
type BlockWriter struct {
	pager pager.Pager
}

func NewBlockWriter(p pager.Pager) *BlockWriter {
	return &BlockWriter{pager: p}
}

func (bw *BlockWriter) WriteBlock(ctx context.Context, startPageID uint64, data []byte) error {
	totalBytes := len(data)
	offset := 0
	pageID := startPageID

	for offset < totalBytes {
		end := offset + ChunkSize
		if end > totalBytes {
			end = totalBytes
		}
		chunk := data[offset:end]

		var pageBody [ChunkSize]byte
		copy(pageBody[:], chunk)

		if err := bw.pager.WritePage(ctx, pageID, pageBody[:]); err != nil {
			return err
		}
		pageID++
		offset = end
	}
	return nil
}

// BlockReader reads and reconstructs compressed blocks from physical pages using Pager.
type BlockReader struct {
	pager pager.Pager
}

func NewBlockReader(p pager.Pager) *BlockReader {
	return &BlockReader{pager: p}
}

func (br *BlockReader) ReadBlock(ctx context.Context, startPageID uint64) ([]byte, error) {
	firstPage, err := br.pager.ReadPage(ctx, startPageID)
	if err != nil {
		return nil, err
	}
	if len(firstPage) < BlockHeaderSize {
		return nil, fmt.Errorf("page payload too small for block header")
	}

	magic := binary.BigEndian.Uint32(firstPage[0:4])
	if magic != BlockMagic {
		return nil, fmt.Errorf("invalid block magic: %x", magic)
	}
	compressedLen := binary.BigEndian.Uint32(firstPage[8:12])
	totalBlockSize := BlockHeaderSize + int(compressedLen)

	pageCount := (totalBlockSize + ChunkSize - 1) / ChunkSize
	blockData := make([]byte, totalBlockSize)
	copy(blockData[0:ChunkSize], firstPage)

	for i := 1; i < pageCount; i++ {
		pageBody, err := br.pager.ReadPage(ctx, startPageID+uint64(i))
		if err != nil {
			return nil, err
		}
		offset := i * ChunkSize
		copy(blockData[offset:], pageBody)
	}

	return blockData, nil
}
```

### 4. Log Retention Worker (`internal/engine/logs/retention.go`)

To avoid scanning raw physical pages that might have been freed, the store maintains an in-memory Directory of compressed block mappings. The Retention Worker evaluates directory entries and frees expired pages:

```go
package logs

import (
	"context"
	"sync"
	"time"

	"github.com/plomvix/plomvix/internal/storage/pager"
)

type BlockDirectoryEntry struct {
	BlockPageID  uint64
	MaxTimestamp int64
	PageCount    uint32
}

type LogsStore struct {
	mu             sync.RWMutex
	blockDirectory []BlockDirectoryEntry // In-memory directory to avoid reading freed pager pages
	// ... existing store fields
}

type RetentionWorker struct {
	retentionDays int
	store         *LogsStore
	pager         pager.Pager
}

func NewRetentionWorker(days int, store *LogsStore, p pager.Pager) *RetentionWorker {
	return &RetentionWorker{
		retentionDays: days,
		store:         store,
		pager:         p,
	}
}

func (w *RetentionWorker) Start(ctx context.Context) error {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		for {
			select {
			case <-ticker.C:
				_ = w.Sweep(ctx)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
	return nil
}

func (w *RetentionWorker) Sweep(ctx context.Context) error {
	cutoff := time.Now().AddDate(0, 0, -w.retentionDays).Unix()

	w.store.mu.Lock()
	defer w.store.mu.Unlock()

	var activeBlocks []BlockDirectoryEntry

	for _, entry := range w.store.blockDirectory {
		if entry.MaxTimestamp < cutoff {
			// Free all physical pages spanned by the block using the Pager API
			for i := uint32(0); i < entry.PageCount; i++ {
				pageID := entry.BlockPageID + uint64(i)
				_ = w.pager.FreePage(ctx, pageID)
			}
		} else {
			activeBlocks = append(activeBlocks, entry)
		}
	}

	w.store.blockDirectory = activeBlocks
	return nil
}
```

---

## Tasks

1. **Implement Tokenizer:** Create `internal/engine/logs/token.go` to split raw bodies into lowercase alphanumeric tokens.
2. **Build Token Index:** Create `internal/engine/logs/index.go` establishing memory limits, lookup lists, and LRU eviction policies for high-cardinality values.
3. **Build ZSTD Block Compression:** Create `internal/engine/logs/compress.go` implementing block-level compression, decompression, and chunked BlockReader/BlockWriter helpers.
4. **Build Retention Worker:** Create `internal/engine/logs/retention.go`. Implement the background loop that deletes block pages older than `LogsRetentionDays` using the block directory and the Pager API.
5. **Optimize Select Path:** Integrate token indexes in `internal/engine/logs/engine.go` select scans to bypass sequential page reads, handling missing block/page tombstones gracefully.

---

## Completion Criteria

- [ ] Log message tokenizer correctly parses complex strings into normalized token slices.
- [ ] Tag and text queries filter using the TokenIndex, reading only matched locator pages.
- [ ] ZSTD compression block tests prove raw log bytes compress by >= 5x inside physical page spans.
- [ ] Retention workers free outdated log pages sequentially on schedule without leaking page pointers.
