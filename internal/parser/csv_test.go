package parser

import (
	"errors"
	"testing"
)

func TestCSVParserBasic(t *testing.T) {
	p := &CSVParser{}
	data := []byte("level,message\ninfo,server started\nwarn,high memory")
	records, err := p.Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	if records[0]["level"] != "info" {
		t.Errorf("records[0][level] = %q, want info", records[0]["level"])
	}
}

func TestCSVParserQuotedFields(t *testing.T) {
	p := &CSVParser{}
	data := []byte("level,message\ninfo,\"hello, world\"")
	records, err := p.Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0]["message"] != "hello, world" {
		t.Errorf("message = %q, want 'hello, world'", records[0]["message"])
	}
}

func TestCSVParserBlankInput(t *testing.T) {
	p := &CSVParser{}
	_, err := p.Parse([]byte(``))
	if err != ErrEmptyInput {
		t.Fatalf("expected ErrEmptyInput, got %v", err)
	}
}

func TestCSVParserHeaderOnly(t *testing.T) {
	p := &CSVParser{}
	_, err := p.Parse([]byte("level,message\n"))
	if err != ErrEmptyInput {
		t.Fatalf("expected ErrEmptyInput, got %v", err)
	}
}

func TestCSVParserMismatchedRow(t *testing.T) {
	p := &CSVParser{}
	data := []byte("level,message\ninfo,server started\nbroken\nwarn,ok")
	records, err := p.Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2 (mismatched row skipped)", len(records))
	}
}

func TestCSVParserEmptyHeaderField(t *testing.T) {
	p := &CSVParser{}
	_, err := p.Parse([]byte("level,\ninfo,x"))
	if !errors.Is(err, ErrMalformedInput) {
		t.Fatalf("expected ErrMalformedInput, got %v", err)
	}
}

func TestCSVParserContentType(t *testing.T) {
	p := &CSVParser{}
	if got := p.ContentType(); got != "text/csv" {
		t.Fatalf("got %q, want text/csv", got)
	}
}
