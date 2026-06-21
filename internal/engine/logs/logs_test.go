// Package logs provides a pluggable logs engine for Plomvix.
// logs_test.go validates log ingestion (JSON and raw text), text-search
// queries, schema validation, and page-level storage correctness.
package logs

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/plomvix/plomvix/internal/storage/pager"
)

func testLogsPagerPath(tb testing.TB) string {
	return filepath.Join(tb.TempDir(), "logs_test.db")
}

func openTestLogsStore(t *testing.T) (*LogsStore, func()) {
	t.Helper()
	pg := pager.New(testLogsPagerPath(t))
	store := NewStore(pg)
	ctx := context.Background()
	if err := store.Open(ctx); err != nil {
		t.Fatalf("open store: %v", err)
	}
	return store, func() { store.Close(ctx) }
}

func readUint32LE(body []byte, offset int) uint32 {
	if offset+4 > len(body) {
		return 0
	}
	return uint32(body[offset]) | uint32(body[offset+1])<<8 |
		uint32(body[offset+2])<<16 | uint32(body[offset+3])<<24
}

// --- Store tests ---

func TestLogsStoreOpenCreatesFirstPage(t *testing.T) {
	store, cleanup := openTestLogsStore(t)
	defer cleanup()

	ctx := context.Background()
	pageCount, err := store.pager.PageCount(ctx)
	if err != nil {
		t.Fatalf("PageCount: %v", err)
	}
	if pageCount <= pager.FirstDataPageID {
		t.Fatalf("expected at least %d pages, got %d", pager.FirstDataPageID, pageCount)
	}

	// First data page should have header initialized: num_records=0, next_write_offset=8
	body, err := store.pager.ReadPage(ctx, pager.FirstDataPageID)
	if err != nil {
		t.Fatalf("ReadPage: %v", err)
	}
	numRecs := readUint32LE(body, 0)
	nextOffset := readUint32LE(body, 4)
	if numRecs != 0 {
		t.Errorf("num_records = %d, want 0", numRecs)
	}
	if nextOffset != logsHeaderSize {
		t.Errorf("next_write_offset = %d, want %d", nextOffset, logsHeaderSize)
	}
}

func TestLogsStoreAppendAndScanSingleRecord(t *testing.T) {
	store, cleanup := openTestLogsStore(t)
	defer cleanup()

	ctx := context.Background()
	rec := LogRecord{
		Timestamp:  1700000000,
		Severity:   SeverityError,
		Attributes: `{"host":"web-1"}`,
		Body:       "connection refused",
	}
	if err := store.AppendLog(ctx, rec); err != nil {
		t.Fatalf("AppendLog: %v", err)
	}

	results, err := store.ScanRange(ctx, 0, 0, "")
	if err != nil {
		t.Fatalf("ScanRange: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 record, got %d", len(results))
	}
	r := results[0]
	if r.Timestamp != 1700000000 {
		t.Errorf("Timestamp = %d, want 1700000000", r.Timestamp)
	}
	if r.Severity != SeverityError {
		t.Errorf("Severity = %d, want %d", r.Severity, SeverityError)
	}
	if r.Attributes != `{"host":"web-1"}` {
		t.Errorf("Attributes = %q, want %q", r.Attributes, `{"host":"web-1"}`)
	}
	if r.Body != "connection refused" {
		t.Errorf("Body = %q, want %q", r.Body, "connection refused")
	}
}

func TestLogsStoreTimeRangeFilter(t *testing.T) {
	store, cleanup := openTestLogsStore(t)
	defer cleanup()

	ctx := context.Background()
	recs := []LogRecord{
		{Timestamp: 100, Severity: SeverityInfo, Body: "first"},
		{Timestamp: 200, Severity: SeverityInfo, Body: "second"},
		{Timestamp: 300, Severity: SeverityInfo, Body: "third"},
	}
	for _, rec := range recs {
		if err := store.AppendLog(ctx, rec); err != nil {
			t.Fatalf("AppendLog: %v", err)
		}
	}

	results, err := store.ScanRange(ctx, 150, 250, "")
	if err != nil {
		t.Fatalf("ScanRange: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 record in range, got %d", len(results))
	}
	if results[0].Body != "second" {
		t.Errorf("Body = %q, want %q", results[0].Body, "second")
	}
}

func TestLogsStoreBodySubstringFilter(t *testing.T) {
	store, cleanup := openTestLogsStore(t)
	defer cleanup()

	ctx := context.Background()
	recs := []LogRecord{
		{Timestamp: 100, Severity: SeverityInfo, Body: "user login success"},
		{Timestamp: 200, Severity: SeverityError, Body: "connection timeout error"},
		{Timestamp: 300, Severity: SeverityWarn, Body: "disk space low"},
	}
	for _, rec := range recs {
		if err := store.AppendLog(ctx, rec); err != nil {
			t.Fatalf("AppendLog: %v", err)
		}
	}

	results, err := store.ScanRange(ctx, 0, 0, "error")
	if err != nil {
		t.Fatalf("ScanRange: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 record matching 'error', got %d", len(results))
	}
	if results[0].Body != "connection timeout error" {
		t.Errorf("Body = %q, want %q", results[0].Body, "connection timeout error")
	}
}

func TestLogsStoreMultiPageOverflow(t *testing.T) {
	store, cleanup := openTestLogsStore(t)
	defer cleanup()

	ctx := context.Background()
	// Create enough records to fill more than one page.
	// Each record is roughly: 8+1+2+2+4+20 = ~37 bytes. Max body size is 4084.
	// So ~110 records per page. 250 records should span 3 pages.
	for i := 0; i < 250; i++ {
		rec := LogRecord{
			Timestamp:  int64(100 + i),
			Severity:   SeverityInfo,
			Attributes: "{}",
			Body:       "log entry number something",
		}
		if err := store.AppendLog(ctx, rec); err != nil {
			t.Fatalf("AppendLog record %d: %v", i, err)
		}
	}

	results, err := store.ScanRange(ctx, 0, 0, "")
	if err != nil {
		t.Fatalf("ScanRange: %v", err)
	}
	if len(results) != 250 {
		t.Errorf("expected 250 records, got %d", len(results))
	}
}

func TestDecodeLogPageRoundTrip(t *testing.T) {
	store, cleanup := openTestLogsStore(t)
	defer cleanup()

	ctx := context.Background()
	original := LogRecord{
		Timestamp:  1700000000,
		Severity:   SeverityFatal,
		Attributes: `{"key1":"val1","key2":"val2"}`,
		Body:       "fatal: out of memory",
	}
	if err := store.AppendLog(ctx, original); err != nil {
		t.Fatalf("AppendLog: %v", err)
	}

	// Read the raw page and decode it.
	body, err := store.pager.ReadPage(ctx, pager.FirstDataPageID)
	if err != nil {
		t.Fatalf("ReadPage: %v", err)
	}
	recs := decodeLogPage(body)
	if len(recs) != 1 {
		t.Fatalf("expected 1 record from decode, got %d", len(recs))
	}
	decoded := recs[0]
	if decoded.Timestamp != original.Timestamp {
		t.Errorf("Timestamp = %d, want %d", decoded.Timestamp, original.Timestamp)
	}
	if decoded.Severity != original.Severity {
		t.Errorf("Severity = %d, want %d", decoded.Severity, original.Severity)
	}
	if decoded.Attributes != original.Attributes {
		t.Errorf("Attributes = %q, want %q", decoded.Attributes, original.Attributes)
	}
	if decoded.Body != original.Body {
		t.Errorf("Body = %q, want %q", decoded.Body, original.Body)
	}
}

// --- Engine tests ---

func TestValidateSchema(t *testing.T) {
	eng := NewLogsEngine(nil, nil)

	tests := []struct {
		name    string
		schema  string
		wantErr bool
	}{
		{
			name:    "valid schema",
			schema:  `[{"name":"time","type":"int64"},{"name":"severity","type":"string"},{"name":"attributes","type":"string"},{"name":"body","type":"string"}]`,
			wantErr: false,
		},
		{
			name:    "missing time",
			schema:  `[{"name":"severity","type":"string"},{"name":"body","type":"string"}]`,
			wantErr: true,
		},
		{
			name:    "missing severity",
			schema:  `[{"name":"time","type":"int64"},{"name":"body","type":"string"}]`,
			wantErr: true,
		},
		{
			name:    "missing body",
			schema:  `[{"name":"time","type":"int64"},{"name":"severity","type":"string"}]`,
			wantErr: true,
		},
		{
			name:    "time wrong type",
			schema:  `[{"name":"time","type":"string"},{"name":"severity","type":"string"},{"name":"body","type":"string"}]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := eng.ValidateSchema([]byte(tt.schema))
			if tt.wantErr && err == nil {
				t.Fatal("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseLogPayloadJSON(t *testing.T) {
	// JSON with level, message, and extra fields.
	body, sev, attrs := parseLogPayload(`{"level":"ERROR","message":"disk full","host":"web-1","pid":1234}`)
	if body != "disk full" {
		t.Errorf("body = %q, want %q", body, "disk full")
	}
	if sev != SeverityError {
		t.Errorf("severity = %d, want %d", sev, SeverityError)
	}
	if attrs != `{"host":"web-1","pid":1234}` {
		t.Errorf("attrs = %q, want %q", attrs, `{"host":"web-1","pid":1234}`)
	}
}

func TestParseLogPayloadJSONSeverity(t *testing.T) {
	// JSON with 'severity' key instead of 'level'.
	body, sev, _ := parseLogPayload(`{"severity":"WARN","message":"low memory"}`)
	if body != "low memory" {
		t.Errorf("body = %q, want %q", body, "low memory")
	}
	if sev != SeverityWarn {
		t.Errorf("severity = %d, want %d", sev, SeverityWarn)
	}
}

func TestParseLogPayloadJSONStatus(t *testing.T) {
	// JSON with 'status' key.
	body, sev, _ := parseLogPayload(`{"status":"DEBUG","msg":"starting up"}`)
	if body != "starting up" {
		t.Errorf("body = %q, want %q", body, "starting up")
	}
	if sev != SeverityDebug {
		t.Errorf("severity = %d, want %d", sev, SeverityDebug)
	}
}

func TestParseLogPayloadRawText(t *testing.T) {
	// Not valid JSON: should default to INFO, entire text as body.
	body, sev, attrs := parseLogPayload("just a plain text log line")
	if body != "just a plain text log line" {
		t.Errorf("body = %q, want %q", body, "just a plain text log line")
	}
	if sev != SeverityInfo {
		t.Errorf("severity = %d, want %d", sev, SeverityInfo)
	}
	if attrs != "" {
		t.Errorf("attrs = %q, want empty", attrs)
	}
}

func TestParseLogPayloadUnknownSeverity(t *testing.T) {
	// Unknown severity string defaults to INFO.
	_, sev, _ := parseLogPayload(`{"level":"TRACE","message":"verbose"}`)
	if sev != SeverityInfo {
		t.Errorf("severity = %d, want %d", sev, SeverityInfo)
	}
}

func TestParseLogPayloadJSONNoMessageKey(t *testing.T) {
	// No message/msg/body key: entire payload becomes body.
	body, sev, attrs := parseLogPayload(`{"level":"INFO","host":"web-1"}`)
	// Since there's no message key, the whole JSON is the body.
	if body != `{"level":"INFO","host":"web-1"}` {
		t.Errorf("body = %q, want full JSON", body)
	}
	if sev != SeverityInfo {
		t.Errorf("severity = %d, want %d", sev, SeverityInfo)
	}
	if attrs != "" {
		t.Errorf("attrs = %q, want empty", attrs)
	}
}

func TestParseSeverity(t *testing.T) {
	tests := []struct {
		input string
		want  uint8
	}{
		{"DEBUG", SeverityDebug},
		{"debug", SeverityDebug},
		{"INFO", SeverityInfo},
		{"info", SeverityInfo},
		{"information", SeverityInfo},
		{"WARN", SeverityWarn},
		{"warning", SeverityWarn},
		{"ERROR", SeverityError},
		{"error", SeverityError},
		{"FATAL", SeverityFatal},
		{"critical", SeverityFatal},
		{"unknown", SeverityInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseSeverity(tt.input)
			if got != tt.want {
				t.Errorf("parseSeverity(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestSeverityToString(t *testing.T) {
	tests := []struct {
		input uint8
		want  string
	}{
		{SeverityDebug, "DEBUG"},
		{SeverityInfo, "INFO"},
		{SeverityWarn, "WARN"},
		{SeverityError, "ERROR"},
		{SeverityFatal, "FATAL"},
		{uint8(99), "INFO"},
	}

	for _, tt := range tests {
		got := severityToString(tt.input)
		if got != tt.want {
			t.Errorf("severityToString(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
