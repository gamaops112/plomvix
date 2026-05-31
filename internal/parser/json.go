package parser

import (
    "bytes"
    "encoding/json"
    "fmt"
)

type JSONParser struct{}

func (p *JSONParser) ContentType() string { return ContentTypeJSON }

func (p *JSONParser) Parse(data []byte) ([]map[string]interface{}, error) {
    data = bytes.TrimSpace(data)
    if len(data) == 0 || bytes.Equal(data, []byte("null")) {
        return nil, ErrEmptyInput
    }

    if data[0] == '[' {
        var records []map[string]interface{}
        if err := json.Unmarshal(data, &records); err != nil {
            return nil, fmt.Errorf("%w: %v", ErrMalformedInput, err)
        }
        return validateRecords(records)
    }

    if data[0] == '{' {
        var obj map[string]interface{}
        if err := json.Unmarshal(data, &obj); err != nil {
            return nil, fmt.Errorf("%w: %v", ErrMalformedInput, err)
        }

        if rawRecords, ok := obj["records"]; ok {
            marshalled, err := json.Marshal(rawRecords)
            if err != nil {
                return nil, fmt.Errorf("%w: invalid records field", ErrMalformedInput)
            }
            var records []map[string]interface{}
            if err := json.Unmarshal(marshalled, &records); err != nil {
                return nil, fmt.Errorf("%w: records must be an array of objects", ErrMalformedInput)
            }
            return validateRecords(records)
        }

        if len(obj) == 0 {
            return nil, ErrEmptyInput
        }
        return []map[string]interface{}{obj}, nil
    }

    return nil, fmt.Errorf("%w: expected JSON object, array, or records wrapper", ErrMalformedInput)
}

func validateRecords(records []map[string]interface{}) ([]map[string]interface{}, error) {
    if len(records) == 0 {
        return nil, ErrEmptyInput
    }
    for i, record := range records {
        if record == nil || len(record) == 0 {
            return nil, fmt.Errorf("%w: record %d must be a non-empty object", ErrMalformedInput, i)
        }
    }
    return records, nil
}
