package app

import (
	"context"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/google/uuid"
	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/panjf2000/ants/v2"
	"github.com/retail-ai-inc/bean"
	"github.com/retail-ai-inc/bean/binder"
	"github.com/retail-ai-inc/bean/echoview"
	"github.com/retail-ai-inc/bean/gopool"
	"github.com/retail-ai-inc/bean/goview"
	"github.com/retail-ai-inc/bean/helpers"
	"github.com/retail-ai-inc/bean/middleware"
	"github.com/rs/dnscache"
)

// NetHttpFastTransporter Support a DNS cache version of the net/http Transport.
var NetHttpFastTransporter *http.Transport

// BeanConfig Hold the useful configuration settings of bean so that we can use it quickly from anywhere.
var BeanConfig bean.Config

func Run() {
	b := bean.New()
	b.Echo = NewEcho()
	// If `NetHttpFastTransporter` is on from env.json then initialize it.
	if BeanConfig.NetHttpFastTransporter.On {
		resolver := &dnscache.Resolver{}
		if BeanConfig.NetHttpFastTransporter.MaxIdleConns == nil {
			*BeanConfig.NetHttpFastTransporter.MaxIdleConns = 0
		}

		if BeanConfig.NetHttpFastTransporter.MaxIdleConnsPerHost == nil {
			*BeanConfig.NetHttpFastTransporter.MaxIdleConnsPerHost = 0
		}

		if BeanConfig.NetHttpFastTransporter.MaxConnsPerHost == nil {
			*BeanConfig.NetHttpFastTransporter.MaxConnsPerHost = 0
		}

		if BeanConfig.NetHttpFastTransporter.IdleConnTimeout == nil {
			*BeanConfig.NetHttpFastTransporter.IdleConnTimeout = 0
		}

		if BeanConfig.NetHttpFastTransporter.DNSCacheTimeout == nil {
			*BeanConfig.NetHttpFastTransporter.DNSCacheTimeout = 5 * time.Minute
		}

		NetHttpFastTransporter = &http.Transport{
			DialContext: func(ctx context.Context, network string, addr string) (conn net.Conn, err error) {
				separator := strings.LastIndex(addr, ":")
				ips, err := resolver.LookupHost(ctx, addr[:separator])
				if err != nil {
					return nil, err
				}

				for _, ip := range ips {
					conn, err = net.Dial(network, ip+addr[separator:])
					if err == nil {
						break
					}
				}

				return
			},
			MaxIdleConns:        *BeanConfig.NetHttpFastTransporter.MaxIdleConns,
			MaxIdleConnsPerHost: *BeanConfig.NetHttpFastTransporter.MaxIdleConnsPerHost,
			MaxConnsPerHost:     *BeanConfig.NetHttpFastTransporter.MaxConnsPerHost,
			IdleConnTimeout:     *BeanConfig.NetHttpFastTransporter.IdleConnTimeout,
		}

		// IMPORTANT: Refresh unused DNS cache in every 5 minutes by default unless set via env.json.
		go func() {
			t := time.NewTicker(*BeanConfig.NetHttpFastTransporter.DNSCacheTimeout)
			defer t.Stop()
			for range t.C {
				resolver.Refresh(true)
			}
		}()
	}

	// If `memory` database is on and `delKeyAPI` end point along with bearer token are properly set.
	if BeanConfig.Database.Memory.On && BeanConfig.Database.Memory.DelKeyAPI.EndPoint != "" {
		b.Echo.DELETE(BeanConfig.Database.Memory.DelKeyAPI.EndPoint, func(c echo.Context) error {
			// If you set empty `authBearerToken` string in env.json then bean will not check the `Authorization` header.
			if BeanConfig.Database.Memory.DelKeyAPI.AuthBearerToken != "" {
				tokenString := helpers.ExtractJWTFromHeader(c)
				if tokenString != BeanConfig.Database.Memory.DelKeyAPI.AuthBearerToken {
					return c.JSON(http.StatusUnauthorized, map[string]interface{}{
						"message": "Unauthorized!",
					})
				}
			}

			key := c.Param("key")
			err := b.DBConn.MemoryDB.Update(func(txn *badger.Txn) error {
				return txn.Delete([]byte(key))
			})
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{
					"message": err.Error(),
				})
			}

			return c.JSON(http.StatusOK, map[string]interface{}{
				"message": "Done",
			})
		})
	}
}

func NewEcho() *echo.Echo {

	e := echo.New()

	// Hide default `Echo` banner during startup.
	e.HideBanner = true

	// Set custom request binder
	e.Binder = &binder.CustomBinder{}

	// Setup HTML view templating engine.
	viewsTemplateCache := BeanConfig.HTML.ViewsTemplateCache
	e.Renderer = echoview.New(goview.Config{
		Root:         "views",
		Extension:    ".html",
		Master:       "templates/master",
		Partials:     []string{},
		Funcs:        make(template.FuncMap),
		DisableCache: !viewsTemplateCache,
		Delims:       goview.Delims{Left: "{{", Right: "}}"},
	})

	// IMPORTANT: Configure debug log.
	if BeanConfig.DebugLogPath != "" {
		if file, err := openFile(BeanConfig.DebugLogPath); err != nil {
			e.Logger.Fatalf("Unable to open log file: %v Server 🚀  crash landed. Exiting...\n", err)
		} else {
			e.Logger.SetOutput(file)
		}
	}
	e.Logger.SetLevel(log.DEBUG)

	// Initialize `BeanLogger` global variable using `e.Logger`.
	bean.BeanLogger = e.Logger

	// Adds a `Server` header to the response.
	e.Use(middleware.ServerHeader(BeanConfig.ProjectName, helpers.CurrVersion()))

	// Sets the maximum allowed size for a request body, return `413 - Request Entity Too Large` if the size exceeds the limit.
	e.Use(echomiddleware.BodyLimit(BeanConfig.HTTP.BodyLimit))

	// CORS initialization and support only HTTP methods which are configured under `http.allowedMethod` parameters in `env.json`.
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: BeanConfig.HTTP.AllowedMethod,
	}))

	// Basic HTTP headers security like XSS protection...
	e.Use(echomiddleware.SecureWithConfig(echomiddleware.SecureConfig{
		XSSProtection:         BeanConfig.Security.HTTP.Header.XssProtection,         // Adds the X-XSS-Protection header with the value `1; mode=block`.
		ContentTypeNosniff:    BeanConfig.Security.HTTP.Header.ContentTypeNosniff,    // Adds the X-Content-Type-Options header with the value `nosniff`.
		XFrameOptions:         BeanConfig.Security.HTTP.Header.XFrameOptions,         // The X-Frame-Options header value to be set with a custom value.
		HSTSMaxAge:            BeanConfig.Security.HTTP.Header.HstsMaxAge,            // HSTS header is only included when the connection is HTTPS.
		ContentSecurityPolicy: BeanConfig.Security.HTTP.Header.ContentSecurityPolicy, // Allows the Content-Security-Policy header value to be set with a custom value.
	}))

	// Return `405 Method Not Allowed` if a wrong HTTP method been called for an API route.
	// Return `404 Not Found` if a wrong API route been called.
	e.Use(middleware.MethodNotAllowedAndRouteNotFound())

	// IMPORTANT: Configure access log and body dumper. (can be turn off)
	if BeanConfig.AccessLog.On {
		accessLogConfig := middleware.LoggerConfig{BodyDump: BeanConfig.AccessLog.BodyDump}
		if BeanConfig.AccessLog.Path != "" {
			if file, err := openFile(BeanConfig.AccessLog.Path); err != nil {
				e.Logger.Fatalf("Unable to open log file: %v Server 🚀  crash landed. Exiting...\n", err)
			} else {
				accessLogConfig.Output = file
			}
			if len(BeanConfig.AccessLog.BodyDumpMaskParam) > 0 {
				accessLogConfig.MaskedParameters = BeanConfig.AccessLog.BodyDumpMaskParam
			}
		}
		accessLogger := middleware.AccessLoggerWithConfig(accessLogConfig)
		e.Use(accessLogger)
	}

	// IMPORTANT: Capturing error and send to sentry if needed.
	// Sentry `panic` error handler and APM initialization if activated from `env.json`
	if BeanConfig.Sentry.On {
		// Check the sentry client options is not nil
		if BeanConfig.Sentry.ClientOptions == nil {
			e.Logger.Fatal("Sentry initialization failed: client options is empty")
		}

		if err := sentry.Init(*BeanConfig.Sentry.ClientOptions); err != nil {
			e.Logger.Fatal("Sentry initialization failed: ", err, ". Server 🚀  crash landed. Exiting...")
		}

		// Configure custom scope
		if BeanConfig.Sentry.ConfigureScope != nil {
			sentry.ConfigureScope(BeanConfig.Sentry.ConfigureScope)
		}

		e.Use(sentryecho.New(sentryecho.Options{
			Repanic: true,
			Timeout: BeanConfig.Sentry.Timeout,
		}))

		if helpers.FloatInRange(BeanConfig.Sentry.TracesSampleRate, 0.0, 1.0) > 0.0 {
			e.Pre(middleware.Tracer())
		}
	}

	// Some pre-build middleware initialization.
	e.Pre(echomiddleware.RemoveTrailingSlash())
	if BeanConfig.HTTP.IsHttpsRedirect {
		e.Pre(echomiddleware.HTTPSRedirect())
	}
	e.Use(echomiddleware.Recover())

	// IMPORTANT: Request related middleware.
	// Set the `X-Request-ID` header field if it doesn't exist.
	e.Use(echomiddleware.RequestIDWithConfig(echomiddleware.RequestIDConfig{
		Generator: uuid.NewString,
	}))

	// Enable prometheus metrics middleware. Metrics data should be accessed via `/metrics` endpoint.
	// This will help us to integrate `bean's` health into `k8s`.
	if BeanConfig.Prometheus.On {
		p := prometheus.NewPrometheus("echo", prometheusUrlSkipper(BeanConfig.Prometheus.SkipEndpoints))
		p.Use(e)
	}

	// Register goroutine pool
	for _, asyncPool := range BeanConfig.AsyncPool {
		if asyncPool.Name == "" {
			continue
		}

		poolSize := -1
		if asyncPool.Size != nil {
			poolSize = *asyncPool.Size
		}

		blockAfter := 0
		if asyncPool.BlockAfter != nil {
			blockAfter = *asyncPool.BlockAfter
		}

		pool, err := ants.NewPool(poolSize, ants.WithMaxBlockingTasks(blockAfter))
		if err != nil {
			e.Logger.Fatal("ants pool initialization failed: ", err, ". Server 🚀  crash landed. Exiting...")
		}

		err = gopool.Register(asyncPool.Name, pool)
		if err != nil {
			e.Logger.Fatal("goroutine pool register failed: ", err, ". Server 🚀  crash landed. Exiting...")
		}
	}

	return e
}

// openFile opens and return the file, if doesn't exist, create it, or append to the file with the directory.
func openFile(path string) (*os.File, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(path), 0764); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0664)
}

// prometheusUrlSkipper ignores metrics route on some endpoints.
func prometheusUrlSkipper(skipEndpoints []string) func(c echo.Context) bool {
	return func(c echo.Context) bool {
		path := c.Request().URL.Path
		for _, endpoint := range skipEndpoints {
			if regexp.MustCompile(endpoint).MatchString(path) {
				return true
			}
		}

		return false
	}
}
