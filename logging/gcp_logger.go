package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
)

type GcpLogger struct {
	ProjectID string
	Options   *GcpLogOptions
}

type GcpLogOptions struct {
	LogType       string
	LogFile       string
	DumpBody      bool
	MaskedFields  []string
	RemoveEscapes bool
}

var DefaultGcpLogOptions = &GcpLogOptions{
	DumpBody:      false,
	RemoveEscapes: false,
}

func NewGcpLogger(projectID string, opts *GcpLogOptions) *GcpLogger {
	if opts == nil {
		opts = DefaultGcpLogOptions
	}
	return &GcpLogger{
		ProjectID: projectID,
		Options:   opts,
	}
}

func (l *GcpLogger) AppendTrace(ctx context.Context, buf *bytes.Buffer) {
	traceID := getSentryTraceID(ctx)
	if traceID == "" {
		return
	}

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		return
	}

	m["logging.googleapis.com/trace"] = fmt.Sprintf("projects/%s/traces/%s", l.ProjectID, traceID)

	newJSON, err := json.Marshal(m)
	if err != nil {
		return
	}

	buf.Reset()
	buf.Write(newJSON)
}

func (l *GcpLogger) Log(entry Entry) {
	m := map[string]interface{}{
		"timestamp": entry.Timestamp.UTC().Format(time.RFC3339Nano),
		"severity":  entry.Severity,
		"type":      l.Options.LogType,
	}

	// ----- GCP trace format -----
	if entry.TraceID != "" && l.ProjectID != "" {
		m["logging.googleapis.com/trace"] =
			fmt.Sprintf("projects/%s/traces/%s",
				l.ProjectID,
				entry.TraceID,
			)
	}

	// ----- HTTP request -----
	httpReq := map[string]interface{}{
		"requestMethod": entry.Method,
		"requestUrl":    entry.URL,
		"latency":       entry.Latency.String(),
	}

	if entry.Status != 0 {
		httpReq["status"] = entry.Status
	}

	m["httpRequest"] = httpReq

	if l.Options.DumpBody {
		reqBody := maskJSON(entry.RequestBody, l.Options.MaskedFields)
		respBody := maskJSON(entry.ResponseBody, l.Options.MaskedFields)
		respBody, _ = readBody(respBody, l.Options.RemoveEscapes)

		if len(reqBody) > 0 {
			m["requestBody"] = jsonOrNull(reqBody)
		}

		if len(respBody) > 0 {
			m["responseBody"] = jsonOrNull(respBody)
		}
	}

	if entry.Error != "" {
		m["error"] = entry.Error
	}

	b, _ := json.Marshal(m)

	// console output
	fmt.Println(string(b))

	// file output
	if l.Options.LogFile != "" {
		dir := filepath.Dir(l.Options.LogFile)
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Println("failed to create log directory:", err)
			return
		}

		f, err := os.OpenFile(
			l.Options.LogFile,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0644,
		)
		if err != nil {
			fmt.Println("failed to open log file:", err)
			return
		}
		defer f.Close()

		f.Write(b)
		f.Write([]byte("\n"))
	}
}

func jsonOrNull(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(bytes.TrimSpace(b))
}

func maskJSON(data []byte, masked []string) []byte {
	if len(masked) == 0 {
		return data
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return data
	}

	for _, k := range masked {
		if _, ok := obj[k]; ok {
			obj[k] = "****"
		}
	}

	b, err := json.Marshal(obj)
	if err != nil {
		return data
	}
	return b
}

// restoreEscapedJSON recursively unquotes escaped JSON strings (for response)
func restoreEscapedJSON(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, v2 := range val {
			val[k] = restoreEscapedJSON(v2)
		}
	case []interface{}:
		for i, v2 := range val {
			val[i] = restoreEscapedJSON(v2)
		}
	case string:
		prev := val
		for {
			unquoted, err := strconv.Unquote(`"` + prev + `"`)
			if err != nil || unquoted == prev {
				break
			}
			prev = unquoted
		}
		var tmp interface{}
		if err := json.Unmarshal([]byte(prev), &tmp); err == nil {
			return restoreEscapedJSON(tmp)
		}
		return prev
	}
	return v
}

// ReadBody reads a body and returns bytes for logging and a new ReadCloser for downstream use.
// limit only restricts the bytes returned for logging; original body is fully preserved.
func readBody(body []byte, removeEscapes bool) ([]byte, error) {
	if body == nil {
		return nil, nil
	}

	// Process for logging
	logData := body
	if removeEscapes {
		var outer interface{}
		if err := json.Unmarshal(body, &outer); err == nil {
			restored := restoreEscapedJSON(outer)
			if restoredData, err := json.Marshal(restored); err == nil {
				logData = restoredData
			}
		}
	}

	return logData, nil
}

func getSentryTraceID(ctx context.Context) string {
	span := sentry.SpanFromContext(ctx)
	if span == nil {
		return ""
	}
	return span.TraceID.String()
}
