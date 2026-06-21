// Package logs provides a pluggable logs engine for Plomvix.
// compress.go implements block-level deflate compression for log records,
// with multi-page chunked BlockWriter/BlockReader helpers that bypass
// the Pager's single-page size limit.
package logs

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/plomvix/plomvix/internal/storage/pager"
)

const (
	// BlockMagic identifies a compressed log block.
	BlockMagic = 0x4C4F4743 // 'LOGC'
	// BlockHeaderSize is the fixed-size header prepended to compressed data.
	BlockHeaderSize = 32
	// ChunkSize is the maximum payload per physical page (DataPageBodySize).
	ChunkSize = pager.DataPageBodySize
)

// BlockHeader describes a compressed log block.
type BlockHeader struct {
	Magic              uint32
	UncompressedLength uint32
	CompressedLength   uint32
	RecordCount        uint32
	MinTimestamp       int64
	MaxTimestamp       int64
}

// CompressBlock serializes raw bytes, compresses them using DEFLATE,
// and prepends the block header.
func CompressBlock(rawBytes []byte, header *BlockHeader) ([]byte, error) {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("compress: create flate writer: %w", err)
	}
	if _, err := w.Write(rawBytes); err != nil {
		w.Close()
		return nil, fmt.Errorf("compress: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("compress: close: %w", err)
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

// DecompressBlock verifies the magic header and decompresses the DEFLATE payload.
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

	reader := flate.NewReader(bytes.NewReader(compressedBlock[BlockHeaderSize : BlockHeaderSize+header.CompressedLength]))
	defer reader.Close()

	decompressed := make([]byte, header.UncompressedLength)
	if _, err := io.ReadFull(reader, decompressed); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, nil, fmt.Errorf("decompress: %w", err)
	}
	return decompressed, header, nil
}

// BlockWriter chunk-writes compressed block bytes into 4084-byte
// physical pages using the Pager.
type BlockWriter struct {
	pager pager.Pager
}

// NewBlockWriter creates a BlockWriter backed by the given pager.
func NewBlockWriter(p pager.Pager) *BlockWriter {
	return &BlockWriter{pager: p}
}

// WriteBlock writes a compressed block across multiple physical pages
// starting at startPageID. It allocates new pages as needed.
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
			return fmt.Errorf("block writer: write page %d: %w", pageID, err)
		}
		pageID++
		offset = end
	}
	return nil
}

// BlockReader reads and reconstructs compressed blocks from physical pages.
type BlockReader struct {
	pager pager.Pager
}

// NewBlockReader creates a BlockReader backed by the given pager.
func NewBlockReader(p pager.Pager) *BlockReader {
	return &BlockReader{pager: p}
}

// ReadBlock reads a multi-page compressed block starting at startPageID.
func (br *BlockReader) ReadBlock(ctx context.Context, startPageID uint64) ([]byte, error) {
	firstPage, err := br.pager.ReadPage(ctx, startPageID)
	if err != nil {
		return nil, fmt.Errorf("block reader: read page %d: %w", startPageID, err)
	}
	if len(firstPage) < BlockHeaderSize {
		return nil, fmt.Errorf("page payload too small for block header (got %d bytes)", len(firstPage))
	}

	magic := binary.BigEndian.Uint32(firstPage[0:4])
	if magic != BlockMagic {
		return nil, fmt.Errorf("invalid block magic: %x", magic)
	}
	compressedLen := binary.BigEndian.Uint32(firstPage[8:12])
	totalBlockSize := BlockHeaderSize + int(compressedLen)

	pageCount := (totalBlockSize + ChunkSize - 1) / ChunkSize
	blockData := make([]byte, totalBlockSize)
	n := copy(blockData[0:], firstPage)
	_ = n

	for i := 1; i < pageCount; i++ {
		pageBody, err := br.pager.ReadPage(ctx, startPageID+uint64(i))
		if err != nil {
			return nil, fmt.Errorf("block reader: read page %d: %w", startPageID+uint64(i), err)
		}
		offset := i * ChunkSize
		copy(blockData[offset:], pageBody)
	}

	return blockData, nil
}
