package parser

import (
    "testing"
)

func TestSyslogParserRFC5424(t *testing.T) {
    p := &SyslogParser{}
    data := []byte(`<34>1 2024-01-15T10:30:00Z web-01 myapp 1234 ID47 - message here`)
    records, err := p.Parse(data)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(records) != 1 {
        t.Fatalf("got %d records, want 1", len(records))
    }
    if records[0]["hostname"] != "web-01" {
        t.Errorf("hostname = %q, want web-01", records[0]["hostname"])
    }
    if records[0]["appname"] != "myapp" {
        t.Errorf("appname = %q, want myapp", records[0]["appname"])
    }
    if records[0]["procid"] != "1234" {
        t.Errorf("procid = %q, want 1234", records[0]["procid"])
    }
    if records[0]["msgid"] != "ID47" {
        t.Errorf("msgid = %q, want ID47", records[0]["msgid"])
    }
    if records[0]["message"] != "message here" {
        t.Errorf("message = %q, want 'message here'", records[0]["message"])
    }
}

func TestSyslogParserRFC3164(t *testing.T) {
    p := &SyslogParser{}
    data := []byte(`<34>Jan 15 10:30:00 web-01 myapp[1234]: message here`)
    records, err := p.Parse(data)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(records) != 1 {
        t.Fatalf("got %d records, want 1", len(records))
    }
    if records[0]["hostname"] != "web-01" {
        t.Errorf("hostname = %q, want web-01", records[0]["hostname"])
    }
    if records[0]["message"] != "message here" {
        t.Errorf("message = %q, want 'message here'", records[0]["message"])
    }
}

func TestSyslogParserMultipleLines(t *testing.T) {
    p := &SyslogParser{}
    data := []byte("<34>1 2024-01-15T10:30:00Z host1 app1 1 - - first\n<34>1 2024-01-15T10:31:00Z host2 app2 2 - - second")
    records, err := p.Parse(data)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(records) != 2 {
        t.Fatalf("got %d records, want 2", len(records))
    }
}

func TestSyslogParserPriority34(t *testing.T) {
    p := &SyslogParser{}
    data := []byte(`<34>1 2024-01-15T10:30:00Z host app 1 - - test`)
    records, err := p.Parse(data)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    pri, _ := records[0]["priority"].(int)
    if pri != 34 {
        t.Errorf("priority = %d, want 34", pri)
    }
    fac, _ := records[0]["facility"].(int)
    if fac != 4 {
        t.Errorf("facility = %d, want 4", fac)
    }
    sev, _ := records[0]["severity"].(int)
    if sev != 2 {
        t.Errorf("severity = %d, want 2", sev)
    }
}

func TestSyslogParserBlankInput(t *testing.T) {
    p := &SyslogParser{}
    _, err := p.Parse([]byte(``))
    if err != ErrEmptyInput {
        t.Fatalf("expected ErrEmptyInput, got %v", err)
    }
}

func TestSyslogParserMalformed(t *testing.T) {
    p := &SyslogParser{}
    _, err := p.Parse([]byte(`not syslog at all`))
    if err != ErrMalformedInput {
        t.Fatalf("expected ErrMalformedInput, got %v", err)
    }
}

func TestSyslogParserContentType(t *testing.T) {
    p := &SyslogParser{}
    if got := p.ContentType(); got != "application/x-syslog" {
        t.Fatalf("got %q, want application/x-syslog", got)
    }
}
