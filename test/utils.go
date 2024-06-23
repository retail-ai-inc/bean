package test

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/retail-ai-inc/bean/v2/helpers"
)

// httpClientWithRetry and httpClientWithoutRetry represent resty http client connections provided as singletons
var httpClientWithRetry, httpClientWithoutRetry *resty.Client
var httpClientOnceWithRetry, httpClientOnceWithoutRetry sync.Once

// NewHTTPClientWithRetry returns a http client with retry mechanism
func NewHTTPClientWithRetry() *resty.Client {
	httpClientOnceWithRetry.Do(func() {
		httpClientWithRetry = createHttpClient(true)
	})

	return httpClientWithRetry
}

// NewHTTPClientWithoutRetry returns a http client without retry mechanism
func NewHTTPClientWithoutRetry() *resty.Client {
	httpClientOnceWithoutRetry.Do(func() {
		httpClientWithoutRetry = createHttpClient(false)
	})

	return httpClientWithoutRetry
}

// createHttpClient creates a new http client with or without retry mechanism based on the withRetry flag
func createHttpClient(withRetry bool) *resty.Client {
	httpClient := resty.New()
	timeout := TestCfg.HTTPClient.Timeout
	if timeout == 0 {
		// set a default timeout
		timeout = time.Second * 10
	}
	httpClient.SetTimeout(timeout)

	if withRetry {
		retryCount := TestCfg.HTTPClient.RetryCount
		retryWaitTime := TestCfg.HTTPClient.RetryWaitTime
		retryMaxWaitTime := TestCfg.HTTPClient.RetryMaxWaitTime
		if retryCount == 0 {
			// set a default retry count
			retryCount = 3
		}
		if retryWaitTime == 0 {
			// set a default retry wait time
			retryWaitTime = time.Second * 5
		}
		if retryMaxWaitTime == 0 {
			// set a default retry max wait time
			retryMaxWaitTime = time.Second * 10
		}

		httpClient.SetRetryCount(retryCount).
			SetRetryWaitTime(retryWaitTime).
			SetRetryMaxWaitTime(retryMaxWaitTime).
			AddRetryCondition(
				func(r *resty.Response, err error) bool {
					return r.StatusCode() != http.StatusOK
				},
			)
	}

	return httpClient
}

// RespBody is a common response body.
type RespBody struct {
	ErrorCode string                 `json:"errorCode,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

func UnmarshalRespBody(t *testing.T, respBody []byte) RespBody {
	t.Helper()

	var target RespBody
	if err := json.Unmarshal(respBody, &target); err != nil {
		t.Errorf("unable to unmarshal response body: %v\n", err)
	}

	return target
}

func SkipTestIfInSkipList(t *testing.T, testName string) {
	t.Helper()

	if isTestInSkipList(t, testName) {
		t.Skip(testName)
	}
}

func isTestInSkipList(t *testing.T, testName string) bool {
	t.Helper()

	if TestCfg.Skip == nil {
		t.Log("skip list is empty")
		return false
	}
	return helpers.HasTargetInSlice(TestCfg.Skip, testName)
}
