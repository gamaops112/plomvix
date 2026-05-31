package parser

import (
    "testing"
)

func TestRegistryExactContentTypes(t *testing.T) {
    r := NewRegistry()
    tests := []struct {
        ct   string
        want Parser
    }{
        {"application/json", &JSONParser{}},
        {"text/csv", &CSVParser{}},
        {"text/x-logfmt", &LogfmtParser{}},
        {"application/x-syslog", &SyslogParser{}},
    }
    for _, tt := range tests {
        p := r.Get(tt.ct)
        if _, ok := p.(*JSONParser); ok {
            if _, ok := tt.want.(*JSONParser); !ok {
                t.Errorf("Get(%q) = JSONParser, want %T", tt.ct, tt.want)
            }
            continue
        }
        if _, ok := p.(*CSVParser); ok {
            if _, ok := tt.want.(*CSVParser); !ok {
                t.Errorf("Get(%q) = CSVParser, want %T", tt.ct, tt.want)
            }
            continue
        }
        if _, ok := p.(*LogfmtParser); ok {
            if _, ok := tt.want.(*LogfmtParser); !ok {
                t.Errorf("Get(%q) = LogfmtParser, want %T", tt.ct, tt.want)
            }
            continue
        }
        if _, ok := p.(*SyslogParser); ok {
            if _, ok := tt.want.(*SyslogParser); !ok {
                t.Errorf("Get(%q) = SyslogParser, want %T", tt.ct, tt.want)
            }
            continue
        }
        t.Errorf("Get(%q) = %T, unexpected type", tt.ct, p)
    }
}

func TestRegistryUnknownDefaultsToJSON(t *testing.T) {
    r := NewRegistry()
    p := r.Get("application/xml")
    if _, ok := p.(*JSONParser); !ok {
        t.Errorf("expected JSONParser for unknown type, got %T", p)
    }
}

func TestRegistryEmptyDefaultsToJSON(t *testing.T) {
    r := NewRegistry()
    p := r.Get("")
    if _, ok := p.(*JSONParser); !ok {
        t.Errorf("expected JSONParser for empty type, got %T", p)
    }
}

func TestRegistryStripsParameters(t *testing.T) {
    r := NewRegistry()
    p := r.Get("text/csv; charset=utf-8")
    if _, ok := p.(*CSVParser); !ok {
        t.Errorf("expected CSVParser after stripping params, got %T", p)
    }
}

func TestRegistryCaseInsensitive(t *testing.T) {
    r := NewRegistry()
    p := r.Get("APPLICATION/JSON")
    if _, ok := p.(*JSONParser); !ok {
        t.Errorf("expected JSONParser for UPPERCASE, got %T", p)
    }
}
