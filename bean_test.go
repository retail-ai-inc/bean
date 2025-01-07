// Copyright The RAI Inc.
// The RAI Authors
package bean

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/v2/config"
	"github.com/retail-ai-inc/bean/v2/internal/route"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestBean_UseErrorHandlerFuncs(t *testing.T) {
	b := &Bean{}
	assert.Empty(t, b.errorHandlerFuncs)

	b.UseErrorHandlerFuncs(func(err error, c echo.Context) (bool, error) {
		return true, nil
	})
	assert.Equal(t, 1, len(b.errorHandlerFuncs))
}

func TestDefaultHTTPErrorHandler(t *testing.T) {
	b := &Bean{}
	b.Echo = echo.New()

	b.UseErrorHandlerFuncs(
		func(err error, c echo.Context) (bool, error) {
			he, ok := err.(*fakeError)
			if !ok {
				return false, nil
			}
			err = c.JSON(http.StatusBadRequest, map[string]interface{}{
				"errorCode": "fake code",
				"errors":    he.Error(),
			})
			return ok, err
		},
		func(_ error, c echo.Context) (bool, error) {
			err := c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"errorCode": "default code",
				"errors":    "default catched!",
			})
			return true, err
		},
	)

	b.Echo.HTTPErrorHandler = b.DefaultHTTPErrorHandler()

	b.Echo.Any("/fake", func(c echo.Context) error {
		return newFakeError("fake error")
	})
	b.Echo.Any("/default", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusInternalServerError, "default error")
	})

	// With Debug=true plain response contains error message
	code, body := request(http.MethodGet, "/fake", b.Echo)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, `{"errorCode":"fake code","errors":"fake error"}`+"\n", body)
	// and special handling for HTTPError
	code, body = request(http.MethodGet, "/default", b.Echo)
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, `{"errorCode":"default code","errors":"default catched!"}`+"\n", body)
}

func request(method, path string, e *echo.Echo) (int, string) {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

type fakeError struct {
	Message string
}

func (f *fakeError) Error() string {
	return f.Message
}

func newFakeError(msg string) error {
	return &fakeError{
		Message: msg,
	}
}

func TestBean_ServeAt(t *testing.T) {
	type fields struct {
		sdTimeout time.Duration
	}
	tests := []struct {
		name        string
		fields      fields
		wantErr     bool
		wantLongSdn bool
		wantConnErr bool
		wantSuccess bool
		_           struct{}
	}{
		{
			name: "graceful shutdown success",
			fields: fields{
				sdTimeout: 0, // timeout will be 30s by default
			},
			wantErr:     false,
			wantLongSdn: true,
			wantConnErr: false,
			wantSuccess: true,
		},
		{
			name: "graceful shutdown timeout",
			fields: fields{
				sdTimeout: 1 * time.Second, // Ensure timeout is less than sleep duration
			},
			wantErr:     true,
			wantLongSdn: false,
			wantConnErr: false,
			wantSuccess: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Bean{
				Echo:     echo.New(),
				Config:   config.Config{},
				Validate: validator.New(),
			}
			b.Config.HTTP.ShutdownTimeout = tt.fields.sdTimeout

			sleepDur := 3 * time.Second
			b.Echo.GET("/fake", func(c echo.Context) error {
				ctx := c.Request().Context()
				t.Logf("Sleeping for %v\n", sleepDur)
				select {
				case <-time.After(sleepDur):
					t.Logf("Completed sleeping for %v\n", sleepDur)
					return c.String(http.StatusOK, "OK")

				case <-ctx.Done():
					t.Logf("Request canceled: %v\n", ctx.Err())
					return ctx.Err()
				}
			})

			// for ping
			b.Echo.GET("/", func(c echo.Context) error {
				return c.String(http.StatusOK, "Pong")
			})

			host := "localhost"
			port := strconv.Itoa(getFreePort(t))
			srvErr := make(chan error, 1)
			go func() {
				srvErr <- b.ServeAt(host, port)
				close(srvErr)
			}()

			ping := func() bool {
				resp, err := http.Get("http://" + host + ":" + port)
				if err != nil {
					return false
				}
				defer resp.Body.Close()
				return resp.StatusCode == http.StatusOK
			}

			// wait for server to start
			timeout := 5 * time.Second
			timer := time.NewTimer(timeout)
			defer timer.Stop()
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
		waitSrv:
			for {
				select {
				case <-timer.C:
					err := <-srvErr
					t.Fatalf("timeout waiting for server to start: %v", err)
				case <-ticker.C:
					if ping() {
						break waitSrv
					}
				}
			}

			// send sleep request
			type result struct {
				err     error
				success bool
			}
			readyToSleep := make(chan struct{}, 1)
			sleepRlt := make(chan result, 1)
			go func() {
				readyToSleep <- struct{}{}
				close(readyToSleep)
				resp, err := http.Get("http://" + host + ":" + port + "/fake")
				if err != nil {
					sleepRlt <- result{err, false}
					close(sleepRlt)
					return
				}
				defer resp.Body.Close()
				_, _ = io.ReadAll(resp.Body)
				sleepRlt <- result{nil, resp.StatusCode == http.StatusOK}
				close(sleepRlt)
			}()

			// wait for server to receive request
			<-readyToSleep
			time.Sleep(100 * time.Millisecond)

			// measure the time taken for shutdown
			start := time.Now()
			// send SIGTERM to server
			signalTERM(t)

			// check if shutdown is successful
			err := <-srvErr
			gotDur := time.Since(start)
			if (err != nil) != tt.wantErr {
				t.Errorf("Bean.ServeAt() error = %v, wantErr %v", err, tt.wantErr)
			}
			if (gotDur >= sleepDur) != tt.wantLongSdn {
				t.Errorf("shutdown took %v, wantLongSdn %v", gotDur, tt.wantLongSdn)
			}
			// check if server is down
			if ping() {
				t.Fatalf("server is still running after shutdown")
			}

			timer.Reset(timeout)
		waitRlt:
			for {
				select {
				case <-timer.C:
					t.Fatal("timeout waiting for server to start")
				case gotRlt := <-sleepRlt:
					if (gotRlt.err != nil) != tt.wantConnErr {
						t.Errorf("response error = %v, wantErr %v", gotRlt.err, tt.wantConnErr)
					}
					if gotRlt.success != tt.wantSuccess {
						t.Errorf("response success = %v, wantSuccess %v", gotRlt.success, tt.wantSuccess)
					}
					break waitRlt
				}
			}
		})
	}
}

func getFreePort(t *testing.T) int {
	t.Helper()

	if addr, rErr := net.ResolveTCPAddr("tcp", "localhost:0"); rErr != nil {
		t.Fatalf("failed to resolve tcp address: %v", rErr)
	} else {
		if ln, lnErr := net.ListenTCP("tcp", addr); lnErr != nil {
			t.Fatalf("failed to listen on tcp address: %v", lnErr)
		} else {
			defer ln.Close()
			return ln.Addr().(*net.TCPAddr).Port
		}
	}

	t.Fatal("failed to get free port")
	return 0
}

func signalTERM(t *testing.T) {
	t.Helper()

	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("failed to find process: %v", err)
	}
	if err := p.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send signal to process: %v", err)
	}
}

func Test_NewEcho(t *testing.T) {
	tests := []struct {
		name       string
		timeout    time.Duration
		sleepTime  time.Duration
		wantStatus int
		wantBody   string
	}{
		{
			name:       "success",
			timeout:    50 * time.Millisecond,
			sleepTime:  10 * time.Millisecond,
			wantStatus: http.StatusOK,
			wantBody:   "success",
		},
		{
			name:       "timeout exceeded",
			timeout:    10 * time.Millisecond,
			sleepTime:  50 * time.Millisecond,
			wantStatus: http.StatusGatewayTimeout,
			wantBody:   "gateway timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Arrange
			reset := setConf(t, tt.timeout)
			defer reset()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			// Act
			e, close := NewEcho()
			defer func() {
				_ = close()
			}()
			e.GET("/", func(c echo.Context) error {
				if err := c.Request().Context().Err(); err != nil {
					return c.String(http.StatusInternalServerError, "unexpected error before sleep")
				}
				time.Sleep(tt.sleepTime)
				if err := c.Request().Context().Err(); err != nil {
					return err
				}
				return c.String(http.StatusOK, "success")
			})
			route.Init(e)
			e.ServeHTTP(rec, req)

			// Assert
			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Contains(t, rec.Body.String(), tt.wantBody)
		})
	}
}

func setConf(t *testing.T, timeout time.Duration) func() {
	t.Helper()

	originalConf := config.Bean
	viper.SetConfigType("json")
	err := viper.ReadConfig(bytes.NewBufferString(`
	{
		"http": {
			"bodyLimit": "1M",
			"timeout": "` + timeout.String() + `",
      "allowedMethod": ["GET"]
		}
	}`))
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if config.Bean == nil {
		config.Bean = &config.Config{}
	}
	if err := viper.Unmarshal(config.Bean); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	return func() {
		config.Bean = originalConf
	}
}
