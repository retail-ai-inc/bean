package transport

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/retail-ai-inc/bean/v2/logging"
)

type HTTPLoggingTransport struct {
	Base   http.RoundTripper
	Logger logging.Logger
}

func NewHTTPLoggingTransport(
	base http.RoundTripper,
	logger logging.Logger,
) http.RoundTripper {

	if base == nil {
		base = http.DefaultTransport
	}

	return &HTTPLoggingTransport{
		Base:   base,
		Logger: logger,
	}
}

func (t *HTTPLoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	// ----- capture request body -----
	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(reqBody))
	}

	resp, err := t.Base.RoundTrip(req)
	latency := time.Since(start)

	severity := "INFO"
	status := 0
	errStr := ""

	if err != nil {
		severity = "ERROR"
		errStr = err.Error()
	} else {
		status = resp.StatusCode
	}

	// ----- capture response body -----
	var respBody []byte
	if err == nil && resp != nil && resp.Body != nil {
		respBody, _ = io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(respBody))
	}

	entry := logging.Entry{
		Timestamp:    time.Now(),
		Severity:     severity,
		Method:       req.Method,
		URL:          req.URL.String(),
		Status:       status,
		Latency:      latency,
		Error:        errStr,
		RequestBody:  reqBody,
		ResponseBody: respBody,
		Context:      req.Context(),
	}

	t.Logger.Log(entry)

	return resp, err
}
