package parser

import "strings"

type Registry struct {
    parsers       map[string]Parser
    defaultParser Parser
}

func NewRegistry() *Registry {
    jsonParser := &JSONParser{}
    return &Registry{
        parsers: map[string]Parser{
            ContentTypeJSON:   jsonParser,
            ContentTypeCSV:    &CSVParser{},
            ContentTypeLogfmt: &LogfmtParser{},
            ContentTypeSyslog: &SyslogParser{},
        },
        defaultParser: jsonParser,
    }
}

func (r *Registry) Get(contentType string) Parser {
    if idx := strings.IndexByte(contentType, ';'); idx >= 0 {
        contentType = contentType[:idx]
    }
    contentType = strings.TrimSpace(strings.ToLower(contentType))
    if p, ok := r.parsers[contentType]; ok {
        return p
    }
    return r.defaultParser
}

func NormalizeContentType(contentType string) string {
    if idx := strings.IndexByte(contentType, ';'); idx >= 0 {
        contentType = contentType[:idx]
    }
    return strings.TrimSpace(strings.ToLower(contentType))
}
