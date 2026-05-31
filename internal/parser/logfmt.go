package parser

import (
    "bufio"
    "bytes"
    "fmt"

    "github.com/kr/logfmt"
)

type LogfmtParser struct{}

func (p *LogfmtParser) ContentType() string { return ContentTypeLogfmt }

func (p *LogfmtParser) Parse(data []byte) ([]map[string]interface{}, error) {
    data = bytes.TrimSpace(data)
    if len(data) == 0 {
        return nil, ErrEmptyInput
    }

    var records []map[string]interface{}
    scanner := bufio.NewScanner(bytes.NewReader(data))
    scanner.Buffer(make([]byte, 1024), 1024*1024)

    for scanner.Scan() {
        line := bytes.TrimSpace(scanner.Bytes())
        if len(line) == 0 {
            continue
        }

        record := make(map[string]interface{})
        if err := logfmt.Unmarshal(line, logfmt.HandlerFunc(func(key, val []byte) error {
            if val == nil {
                return nil
            }
            record[string(key)] = string(val)
            return nil
        })); err != nil {
            return nil, fmt.Errorf("%w: logfmt parse error: %v", ErrMalformedInput, err)
        }

        if len(record) > 0 {
            records = append(records, record)
        }
    }

    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("%w: scanner error: %v", ErrMalformedInput, err)
    }
    if len(records) == 0 {
        return nil, ErrEmptyInput
    }
    return records, nil
}
