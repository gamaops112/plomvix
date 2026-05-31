package parser

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

type CSVParser struct{}

func (p *CSVParser) ContentType() string { return ContentTypeCSV }

func (p *CSVParser) Parse(data []byte) ([]map[string]interface{}, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, ErrEmptyInput
	}

	r := csv.NewReader(bytes.NewReader(data))
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1

	headers, err := r.Read()
	if err == io.EOF {
		return nil, ErrEmptyInput
	}
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read CSV header: %v", ErrMalformedInput, err)
	}
	if len(headers) == 0 {
		return nil, ErrEmptyInput
	}
	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
		if headers[i] == "" {
			return nil, fmt.Errorf("%w: header field %d is empty", ErrMalformedInput, i)
		}
	}

	var records []map[string]interface{}
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: failed to read CSV row: %v", ErrMalformedInput, err)
		}
		if isEmptyCSVRow(row) {
			continue
		}
		if len(row) != len(headers) {
			continue
		}
		record := make(map[string]interface{}, len(headers))
		for i, h := range headers {
			record[h] = row[i]
		}
		records = append(records, record)
	}

	if len(records) == 0 {
		return nil, ErrEmptyInput
	}
	return records, nil
}

func isEmptyCSVRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}
