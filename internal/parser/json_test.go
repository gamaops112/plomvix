package parser

import (
    "errors"
    "testing"
)

func TestJSONParserBareArray(t *testing.T) {
    p := &JSONParser{}
    records, err := p.Parse([]byte(`[{"level":"info"},{"level":"warn"}]`))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(records) != 2 {
        t.Fatalf("got %d records, want 2", len(records))
    }
}

func TestJSONParserBareObject(t *testing.T) {
    p := &JSONParser{}
    records, err := p.Parse([]byte(`{"level":"info","message":"hello"}`))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(records) != 1 {
        t.Fatalf("got %d records, want 1", len(records))
    }
}

func TestJSONParserSprint5Wrapper(t *testing.T) {
    p := &JSONParser{}
    records, err := p.Parse([]byte(`{"records":[{"level":"info"},{"level":"warn"}]}`))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(records) != 2 {
        t.Fatalf("got %d records, want 2", len(records))
    }
}

func TestJSONParserEmptyBody(t *testing.T) {
    p := &JSONParser{}
    _, err := p.Parse([]byte(``))
    if !errors.Is(err, ErrEmptyInput) {
        t.Fatalf("expected ErrEmptyInput, got %v", err)
    }
}

func TestJSONParserEmptyArray(t *testing.T) {
    p := &JSONParser{}
    _, err := p.Parse([]byte(`[]`))
    if !errors.Is(err, ErrEmptyInput) {
        t.Fatalf("expected ErrEmptyInput, got %v", err)
    }
}

func TestJSONParserEmptyWrapper(t *testing.T) {
    p := &JSONParser{}
    _, err := p.Parse([]byte(`{"records":[]}`))
    if !errors.Is(err, ErrEmptyInput) {
        t.Fatalf("expected ErrEmptyInput, got %v", err)
    }
}

func TestJSONParserMalformed(t *testing.T) {
    p := &JSONParser{}
    _, err := p.Parse([]byte(`{invalid}`))
    if !errors.Is(err, ErrMalformedInput) {
        t.Fatalf("expected ErrMalformedInput, got %v", err)
    }
}

func TestJSONParserNonObject(t *testing.T) {
    p := &JSONParser{}
    _, err := p.Parse([]byte(`"just a string"`))
    if !errors.Is(err, ErrMalformedInput) {
        t.Fatalf("expected ErrMalformedInput, got %v", err)
    }
}

func TestJSONParserContentType(t *testing.T) {
    p := &JSONParser{}
    if got := p.ContentType(); got != "application/json" {
        t.Fatalf("got %q, want application/json", got)
    }
}
