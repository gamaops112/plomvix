package parser

import (
    "testing"
)

func TestLogfmtParserBasic(t *testing.T) {
    p := &LogfmtParser{}
    data := []byte(`level=info msg="server started" host=web-01`)
    records, err := p.Parse(data)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(records) != 1 {
        t.Fatalf("got %d records, want 1", len(records))
    }
    if records[0]["level"] != "info" {
        t.Errorf("level = %q, want info", records[0]["level"])
    }
    if records[0]["host"] != "web-01" {
        t.Errorf("host = %q, want web-01", records[0]["host"])
    }
}

func TestLogfmtParserMultipleLines(t *testing.T) {
    p := &LogfmtParser{}
    data := []byte("level=info msg=first\nlevel=warn msg=second")
    records, err := p.Parse(data)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(records) != 2 {
        t.Fatalf("got %d records, want 2", len(records))
    }
}

func TestLogfmtParserBlankLines(t *testing.T) {
    p := &LogfmtParser{}
    data := []byte("level=info\n\nlevel=warn\n")
    records, err := p.Parse(data)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(records) != 2 {
        t.Fatalf("got %d records, want 2", len(records))
    }
}

func TestLogfmtParserDuplicateKeys(t *testing.T) {
    p := &LogfmtParser{}
    data := []byte(`key=a key=b`)
    records, err := p.Parse(data)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(records) != 1 {
        t.Fatalf("got %d records, want 1", len(records))
    }
    if records[0]["key"] != "b" {
        t.Errorf("key = %q, want 'b' (last value wins)", records[0]["key"])
    }
}

func TestLogfmtParserBlankInput(t *testing.T) {
    p := &LogfmtParser{}
    _, err := p.Parse([]byte(``))
    if err != ErrEmptyInput {
        t.Fatalf("expected ErrEmptyInput, got %v", err)
    }
}

func TestLogfmtParserMalformed(t *testing.T) {
    p := &LogfmtParser{}
    // A line with no = signs should still parse as empty record
    data := []byte("plaintextwithoutquals")
    records, err := p.Parse(data)
    if err != ErrEmptyInput {
        t.Fatalf("expected ErrEmptyInput, got %v", err)
    }
    _ = records
}

func TestLogfmtParserContentType(t *testing.T) {
    p := &LogfmtParser{}
    if got := p.ContentType(); got != "text/x-logfmt" {
        t.Fatalf("got %q, want text/x-logfmt", got)
    }
}
