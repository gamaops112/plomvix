package parser

import "errors"

type Parser interface {
	Parse(data []byte) ([]map[string]interface{}, error)
	ContentType() string
}

var (
	ErrEmptyInput            = errors.New("input is empty")
	ErrMalformedInput        = errors.New("malformed input")
	ErrUnsupportedContentType = errors.New("unsupported content type")
)

const (
	ContentTypeJSON   = "application/json"
	ContentTypeCSV    = "text/csv"
	ContentTypeLogfmt = "text/x-logfmt"
	ContentTypeSyslog = "application/x-syslog"
)
