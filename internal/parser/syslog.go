package parser

import (
    "bufio"
    "bytes"
    "fmt"
    "time"

    "github.com/influxdata/go-syslog/v3/rfc3164"
    "github.com/influxdata/go-syslog/v3/rfc5424"
)

type SyslogParser struct{}

func (p *SyslogParser) ContentType() string { return ContentTypeSyslog }

func (p *SyslogParser) Parse(data []byte) ([]map[string]interface{}, error) {
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
        record, err := parseSyslogLine(line)
        if err != nil {
            return nil, ErrMalformedInput
        }
        records = append(records, record)
    }

    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("%w: scanner error: %v", ErrMalformedInput, err)
    }
    if len(records) == 0 {
        return nil, ErrEmptyInput
    }
    return records, nil
}

func parseSyslogLine(line []byte) (map[string]interface{}, error) {
    p5424 := rfc5424.NewParser()
    if msg, err := p5424.Parse(line); err == nil && msg != nil {
        if m, ok := msg.(*rfc5424.SyslogMessage); ok {
            return syslog5424ToRecord(m), nil
        }
    }

    p3164 := rfc3164.NewParser()
    if msg, err := p3164.Parse(line); err == nil && msg != nil {
        if m, ok := msg.(*rfc3164.SyslogMessage); ok {
            return syslog3164ToRecord(m), nil
        }
    }

    return nil, fmt.Errorf("line not valid RFC 5424 or RFC 3164 syslog: %q", string(line))
}

func syslog5424ToRecord(msg *rfc5424.SyslogMessage) map[string]interface{} {
    r := baseSyslogRecord(msg.Priority, msg.Timestamp, msg.Hostname, msg.Appname, msg.ProcID, msg.Message)
    if msg.MsgID != nil {
        r["msgid"] = *msg.MsgID
    }
    return r
}

func syslog3164ToRecord(msg *rfc3164.SyslogMessage) map[string]interface{} {
    return baseSyslogRecord(msg.Priority, msg.Timestamp, msg.Hostname, msg.Appname, msg.ProcID, msg.Message)
}

func baseSyslogRecord(priority *uint8, timestamp *time.Time, hostname, appname, procid, message *string) map[string]interface{} {
    r := map[string]interface{}{
        "priority": 0,
        "facility": 0,
        "severity": 0,
        "timestamp": "",
        "hostname": "",
        "appname": "",
        "procid": "",
        "msgid": "",
        "message": "",
    }
    if priority != nil {
        pri := int(*priority)
        r["priority"] = pri
        r["facility"] = pri / 8
        r["severity"] = pri % 8
    }
    if timestamp != nil {
        r["timestamp"] = timestamp.Format(time.RFC3339Nano)
        r["timestamp_ns"] = float64(timestamp.UnixNano())
    }
    if hostname != nil {
        r["hostname"] = *hostname
    }
    if appname != nil {
        r["appname"] = *appname
    }
    if procid != nil {
        r["procid"] = *procid
    }
    if message != nil {
        r["message"] = *message
    }
    return r
}
