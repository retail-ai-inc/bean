// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package bean

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	validatorV10 "github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo-contrib/v5/echoprometheus"
	"github.com/labstack/echo/v5"
	echomiddleware "github.com/labstack/echo/v5/middleware"
	pkgerrors "github.com/pkg/errors"
	"github.com/retail-ai-inc/bean/v3/config"
	"github.com/retail-ai-inc/bean/v3/echoview"
	berror "github.com/retail-ai-inc/bean/v3/error"
	"github.com/retail-ai-inc/bean/v3/goview"
	"github.com/retail-ai-inc/bean/v3/helpers"
	"github.com/retail-ai-inc/bean/v3/internal/binder"
	"github.com/retail-ai-inc/bean/v3/internal/dbdrivers"
	"github.com/retail-ai-inc/bean/v3/internal/gopool"
	"github.com/retail-ai-inc/bean/v3/internal/middleware"
	"github.com/retail-ai-inc/bean/v3/internal/regex"
	"github.com/retail-ai-inc/bean/v3/internal/router"
	"github.com/retail-ai-inc/bean/v3/internal/validator"
	blog "github.com/retail-ai-inc/bean/v3/log"
	"github.com/retail-ai-inc/bean/v3/store/memory"
	"github.com/retail-ai-inc/bean/v3/trace"
	"github.com/rs/dnscache"
	"go.mongodb.org/mongo-driver/mongo"
	"gorm.io/gorm"
)

// All database connections are initialized using `DBDeps` structure.
type DBDeps struct {
	MasterMySQLDB      *gorm.DB
	MasterMySQLDBName  string
	TenantMySQLDBs     map[uint64]*gorm.DB
	TenantMySQLDBNames map[uint64]string
	MasterMongoDB      *mongo.Client
	MasterMongoDBName  string
	TenantMongoDBs     map[uint64]*mongo.Client
	TenantMongoDBNames map[uint64]string
	MasterRedisDB      *dbdrivers.RedisDBConn
	TenantRedisDBs     map[uint64]*dbdrivers.RedisDBConn
	MemoryDB           memory.Cache
}

type Bean struct {
	DBConn            *DBDeps
	Echo              *echo.Echo
	BeforeServe       func()
	ShutdownSrv       []func() error
	CleanupDBs        []func() error
	errorHandlerFuncs []berror.ErrorHandlerFunc
	Validate          *validatorV10.Validate
	Config            config.Config
}

// If a command or service wants to use a different `host` parameter for tenant database connection
// then it's easy to do just by passing that parameter string name using `bean.TenantAlterDbHostParam`.
// Therfore, `bean` will overwrite all host string in `TenantConnections`.`Connections` JSON.
var TenantAlterDbHostParam string

// Support a DNS cache version of the net/http Transport.
var NetHttpFastTransporter *http.Transport

func newNetHTTPFastTransporter() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defaultDialContext := transport.DialContext
	resolver := &dnscache.Resolver{}
	transportConfig := config.Bean.NetHttpFastTransporter

	transport.MaxIdleConns = valueOr(transportConfig.MaxIdleConns, 0)
	transport.MaxIdleConnsPerHost = valueOr(transportConfig.MaxIdleConnsPerHost, 0)
	transport.MaxConnsPerHost = valueOr(transportConfig.MaxConnsPerHost, 0)
	transport.IdleConnTimeout = valueOr(transportConfig.IdleConnTimeout, time.Duration(0))
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("split address %q: %w", address, err)
		}

		ips, err := resolver.LookupHost(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve host %q: %w", host, err)
		}

		var lastErr error
		for _, ip := range ips {
			conn, err := defaultDialContext(ctx, network, net.JoinHostPort(ip, port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}

		if lastErr == nil {
			lastErr = fmt.Errorf("DNS returned no addresses for %q", host)
		}

		return nil, lastErr
	}

	refreshInterval := valueOr(transportConfig.DNSCacheTimeout, 5*time.Minute)
	go func() {
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()
		for range ticker.C {
			resolver.Refresh(true)
		}
	}()

	return transport
}

func valueOr[T any](value *T, fallback T) T {
	if value == nil {
		return fallback
	}
	return *value
}

// ErrConfigNotLoaded is returned by `New` and `NewEcho` when `config.Bean` is not loaded.
var ErrConfigNotLoaded = errors.New("config is not loaded")

// New creates a bean instance from `config.Bean`. On error, everything already initialized is cleaned up.
func New() (*Bean, error) {
	if config.Bean == nil {
		return nil, ErrConfigNotLoaded
	}

	// Create a new echo instance
	e, closeEcho, err := NewEcho()
	if err != nil {
		return nil, err
	}

	b := &Bean{
		Echo:     e,
		Validate: validatorV10.New(),
		Config:   *config.Bean,
	}

	// If `NetHttpFastTransporter` is on from env.json then initialize it.
	if config.Bean.NetHttpFastTransporter.On {
		NetHttpFastTransporter = newNetHTTPFastTransporter()
	}

	// If `memory` database is on and `delKeyAPI` end point along with bearer token are properly set.
	if config.Bean.Database.Memory.On && config.Bean.Database.Memory.DelKeyAPI.EndPoint != "" {
		e.DELETE(config.Bean.Database.Memory.DelKeyAPI.EndPoint, func(c *echo.Context) error {
			// If you set empty `authBearerToken` string in env.json then bean will not check the `Authorization` header.
			if config.Bean.Database.Memory.DelKeyAPI.AuthBearerToken != "" {
				tokenString := helpers.ExtractJWTFromHeader(c)
				if tokenString != config.Bean.Database.Memory.DelKeyAPI.AuthBearerToken {
					return c.JSON(http.StatusUnauthorized, map[string]interface{}{
						"message": "Unauthorized!",
					})
				}
			}

			key := c.Param("key")
			b.DBConn.MemoryDB.DelMemory(key)
			return c.JSON(http.StatusOK, map[string]interface{}{
				"message": "Done",
			})
		})
	}

	b.ShutdownSrv = append(b.ShutdownSrv, closeEcho)

	return b, nil
}

// NewEcho creates the echo instance with bean's middlewares from `config.Bean`. The returned function
// releases the resources (Sentry, goroutine pools). On error, everything already initialized is cleaned up.
func NewEcho() (*echo.Echo, func() error, error) {
	if config.Bean == nil {
		return nil, nil, ErrConfigNotLoaded
	}

	// IMPORTANT: Configure the logger. Application logs go to `debugLogPath` (stdout if empty), the access log
	// to `accessLog.path` (the same output if empty). `log.Logger()`, echo's logger and `c.Logger()` are the
	// same slog logger, so every log has the same format.
	var logOut io.Writer = os.Stdout
	if config.Bean.DebugLogPath != "" {
		file, err := helpers.OpenFile(config.Bean.DebugLogPath)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to open debug log file: %w", err)
		}
		logOut = file
	}
	blogger, err := blog.Init(logOut)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	slogger := blogger.Logger

	// The router keeps bean's routing conventions (handler named routes, `http.allowedMethod`). Requests to
	// unknown paths or methods are answered by the echo router with `echo.ErrNotFound` / `echo.ErrMethodNotAllowed`
	// and rendered by the HTTP error handler.
	e := echo.NewWithConfig(echo.Config{
		Logger: slogger,
		// Do not let `Group.Use` register catch-all 404 routes, otherwise a wrong method on a grouped route
		// would get 404 instead of 405. Unknown paths are handled by the router's 404 instead.
		NoGroupAutoRegister404Routes: true,
		Router: router.New(
			echo.NewRouter(echo.RouterConfig{AllowOverwritingRoute: true}),
			config.Bean.HTTP.AllowedMethod,
			slogger,
		),
	})
	closes := []func() error{}
	fail := func(err error) (*echo.Echo, func() error, error) {
		if cErr := closer(closes)(); cErr != nil {
			err = errors.Join(err, cErr)
		}
		return nil, nil, err
	}

	// Set custom request binder
	e.Binder = &binder.CustomBinder{}

	// Setup HTML view templating engine.
	viewsTemplateCache := config.Bean.HTML.ViewsTemplateCache
	e.Renderer = echoview.New(goview.Config{
		Root:         "views",
		Extension:    ".html",
		Master:       "templates/master",
		Partials:     []string{},
		Funcs:        make(template.FuncMap),
		DisableCache: !viewsTemplateCache,
		Delims:       goview.Delims{Left: "{{", Right: "}}"},
	})

	extractor, err := middleware.NewIPExtractor(config.Bean.HTTP.TrustIPRange)
	if err != nil {
		return fail(fmt.Errorf("invalid http.trustIPRange: %w", err))
	}
	if extractor != nil {
		e.IPExtractor = extractor
	}

	// Adds a `Server` header to the response.
	e.Use(middleware.ServerHeader(config.Bean.ProjectName, helpers.CurrVersion()))

	// Sets the maximum allowed size for a request body, return `413 - Request Entity Too Large` if the size exceeds the limit.
	bodyLimit, err := helpers.ParseByteSize(config.Bean.HTTP.BodyLimit)
	if err != nil {
		return fail(fmt.Errorf("invalid http.bodyLimit: %w", err))
	}
	e.Use(echomiddleware.BodyLimit(bodyLimit))

	// CORS initialization and support only HTTP methods which are configured under `http.allowedMethod` parameters in `env.json`.
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: config.Bean.HTTP.AllowedMethod,
	}))

	// Basic HTTP headers security like XSS protection...
	e.Use(echomiddleware.SecureWithConfig(echomiddleware.SecureConfig{
		XSSProtection:         config.Bean.Security.HTTP.Header.XssProtection,         // Adds the X-XSS-Protection header with the value `1; mode=block`.
		ContentTypeNosniff:    config.Bean.Security.HTTP.Header.ContentTypeNosniff,    // Adds the X-Content-Type-Options header with the value `nosniff`.
		XFrameOptions:         config.Bean.Security.HTTP.Header.XFrameOptions,         // The X-Frame-Options header value to be set with a custom value.
		HSTSMaxAge:            config.Bean.Security.HTTP.Header.HstsMaxAge,            // HSTS header is only included when the connection is HTTPS.
		ContentSecurityPolicy: config.Bean.Security.HTTP.Header.ContentSecurityPolicy, // Allows the Content-Security-Policy header value to be set with a custom value.
		ReferrerPolicy:        config.Bean.Security.HTTP.Header.ReferrerPolicy,        // To determine how your service share information about the source page (referrer URL) when users click on links or visit new pages. Optional. Default value "".
	}))

	// Add context timeout.
	// If no timeout is set or timeout=0, skip adding the timeout middleware.
	timeoutDur := config.Bean.HTTP.Timeout
	if timeoutDur > 0 {
		e.Use(ContextTimeout(timeoutDur))
	}

	flushSentry := func() error { return nil }
	// IMPORTANT: Capturing error and send to sentry if needed.
	// Sentry `panic` error handler and APM initialization if activated from `env.json`
	if config.Bean.Sentry.On {
		// Check the sentry client options is not nil
		if config.Bean.Sentry.ClientOptions == nil {
			slogger.Error("Sentry initialization failed: client options is empty")
		} else {
			clientOption := config.Bean.Sentry.ClientOptions
			if clientOption.TracesSampleRate > 0 {
				clientOption.EnableTracing = true
			}
			if err := sentry.Init(*clientOption); err != nil {
				return fail(fmt.Errorf("sentry initialization failed: %w", err))
			}
			flushSentry = func() error {
				notTimeout := sentry.Flush(config.Bean.Sentry.Timeout)
				if !notTimeout {
					return errors.New("sentry flush timeout")
				}
				return nil
			}

			// Configure custom scope
			if config.Bean.Sentry.ConfigureScope != nil {
				sentry.ConfigureScope(config.Bean.Sentry.ConfigureScope)
			}

			// Start tracing for the incoming request.
			e.Use(sentryecho.New(sentryecho.Options{
				Repanic: true,
				Timeout: config.Bean.Sentry.Timeout,
			}))
			e.Use(middleware.SetHubToContext)

			if helpers.FloatInRange(config.Bean.Sentry.TracesSampleRate, 0.0, 1.0) > 0.0 {
				if err := regex.SetSamplingPathSkipper(config.Bean.Sentry.SkipTracesEndpoints); err != nil {
					return fail(err)
				}
				e.Use(middleware.SkipSampling)
			}
		}
	}
	closes = append(closes, flushSentry)

	// IMPORTANT: Request related middleware.
	// Set the `X-Request-ID` header field if it doesn't exist.
	e.Use(echomiddleware.RequestIDWithConfig(echomiddleware.RequestIDConfig{
		// Whether to skip it depends on whether the `AccessLog.ReqHeaderParam` parameter contains `X-Request-Id`.
		Skipper: func(c *echo.Context) bool {
			return !slices.Contains(config.Bean.AccessLog.ReqHeaderParam, echo.HeaderXRequestID)
		},
		Generator:        uuid.NewString,
		RequestIDHandler: middleware.RequestIDHandler,
		TargetHeader:     echo.HeaderXRequestID,
	}))

	// IMPORTANT: Configure access log and body dumper. (can be turn off)
	if config.Bean.AccessLog.On {
		skipper, err := regex.InitAccessLogPathSkipper(config.Bean.AccessLog.SkipEndpoints)
		if err != nil {
			return fail(err)
		}
		accessLogConfig := middleware.LoggerConfig{
			Skipper:        skipper,
			BodyDump:       config.Bean.AccessLog.BodyDump,
			RequestHeader:  config.Bean.AccessLog.ReqHeaderParam,
			ResponseHeader: config.Bean.AccessLog.ResHeaderParam,
			Logger:         blog.Logger(),
			BodyLimit:      config.Bean.AccessLog.BodyLimit,
		}

		accessLogger := middleware.AccessLoggerWithConfig(accessLogConfig)
		e.Use(accessLogger)
	}

	// Some pre-build middleware initialization.
	e.Pre(echomiddleware.RemoveTrailingSlash())
	if config.Bean.HTTP.IsHttpsRedirect {
		e.Pre(echomiddleware.HTTPSRedirect())
	}
	// Enable prometheus metrics middleware. Metrics data should be accessed via `/metrics` endpoint.
	// This will help us to integrate `bean's` health into `k8s`.
	if config.Bean.Prometheus.On {
		const metricsPath = "/metrics" // fixed path
		skipper, err := regex.InitPrometheusPathSkipper(config.Bean.Prometheus.SkipEndpoints, metricsPath)
		if err != nil {
			return fail(fmt.Errorf("prometheus initialization failed: %w", err))
		}
		conf := echoprometheus.MiddlewareConfig{
			Skipper: skipper,
			// 404 requests (unknown routes) now pass through the middleware chain; use one `url` label for
			// all of them instead of the request path to avoid unbounded label cardinality.
			DoNotUseRequestPathFor404: true,
		}
		if config.Bean.Prometheus.Subsystem != "" {
			conf.Subsystem = config.Bean.Prometheus.Subsystem
		}
		e.Use(echoprometheus.NewMiddlewareWithConfig(conf))
		e.GET(metricsPath, echoprometheus.NewHandler())
	}

	// IMPORTANT: Register `Recover` after (= inside) the middlewares above, so that a panic is turned into an
	// error before it reaches them. Otherwise the panic unwinds through e.g. the Prometheus middleware and the
	// request is not counted in the metrics at all.
	e.Use(middleware.Recover())

	// Register goroutine pool. Register the release first so the pools are released on error too.
	closes = append(closes, gopool.ReleaseAllPools(config.Bean.AsyncPoolReleaseTimeout))
	if len(config.Bean.AsyncPool) > 0 {
		for _, asyncPool := range config.Bean.AsyncPool {
			if asyncPool.Name == "" {
				continue
			}

			pool, err := gopool.NewPool(asyncPool.Size, asyncPool.BlockAfter)
			if err != nil {
				return fail(fmt.Errorf("failed to create goroutine pool %q: %w", asyncPool.Name, err))
			}

			if err := gopool.Register(asyncPool.Name, pool); err != nil {
				pool.Release()
				return fail(fmt.Errorf("failed to register goroutine pool %q: %w", asyncPool.Name, err))
			}
		}
	}

	return e, closer(closes), nil
}

func (b *Bean) ServeAt(host, port string) error {
	b.Echo.Logger.Info("Starting " + b.Config.Environment + " " + b.Config.ProjectName + " at " + host + ":" + port + "...🚀")

	b.UseErrorHandlerFuncs(berror.DefaultErrorHandlerFunc)
	b.Echo.HTTPErrorHandler = b.DefaultHTTPErrorHandler()

	v, err := NewValidator(b.Validate)
	if err != nil {
		return err
	}
	b.Echo.Validator = v

	gracefulTimeout := b.Config.HTTP.ShutdownTimeout
	if gracefulTimeout <= 0 {
		gracefulTimeout = 30 * time.Second
	}

	var shutdownErr error
	sc := echo.StartConfig{
		Address:         host + ":" + port,
		HideBanner:      true,
		HidePort:        true,
		GracefulTimeout: gracefulTimeout,
		// Even after timeout, shutdown keeps handling ongoing requests in the background
		// while returning timeout error until main goroutine exits.
		OnShutdownError: func(err error) { shutdownErr = err },
		BeforeServeFunc: func(s *http.Server) error {
			// echo sets a 30s read timeout by default, which would break long uploads. Keep the
			// `net/http` default (no timeout) as before.
			s.ReadTimeout = 0
			// IMPORTANT: Keep-alive is default true but I kept this here to let you guys no that there is a settings
			// for it :)
			s.SetKeepAlivesEnabled(b.Config.HTTP.KeepAlive)
			return nil
		},
	}

	start := func(ctx context.Context) error { return sc.Start(ctx, b.Echo) }
	if b.Config.HTTP.SSL.On {
		// Read the files here: `StartTLS` reads file paths relative to the working directory only.
		cert, err := os.ReadFile(b.Config.HTTP.SSL.CertFile)
		if err != nil {
			return pkgerrors.Wrapf(err, "error during server startup")
		}
		key, err := os.ReadFile(b.Config.HTTP.SSL.PrivFile)
		if err != nil {
			return pkgerrors.Wrapf(err, "error during server startup")
		}
		sc.TLSConfig = &tls.Config{
			MinVersion: b.Config.HTTP.SSL.MinTLSVersion,
			NextProtos: []string{"h2", "http/1.1"}, // keep HTTP/2 support as `ListenAndServeTLS` did.
		}
		start = func(ctx context.Context) error { return sc.StartTLS(ctx, b.Echo, cert, key) }
	}

	// The server stops (gracefully) when `srvCtx` is canceled. `Start` returns once the graceful
	// shutdown is finished.
	srvCtx, stopSrv := context.WithCancel(context.Background())
	defer stopSrv()

	var srvErr error
	srvDone := make(chan struct{})

	b.ShutdownSrv = append(b.ShutdownSrv, func() error {
		stopSrv()
		<-srvDone
		if shutdownErr != nil {
			return pkgerrors.Wrapf(shutdownErr, "failed to gracefully shutdown")
		}
		return nil
	})

	// before bean bootstrap. The server shutdown is already registered, so `Shutdown` can be called from here on.
	if b.BeforeServe != nil {
		b.BeforeServe()
	}

	// Listen for the interrupt signal or termination signal before the server starts.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		defer close(srvDone)
		srvErr = start(srvCtx)
	}()

	select {
	case <-srvDone: // The server failed to start / serve, or was shut down by calling `Shutdown`.
		if srvErr != nil {
			return pkgerrors.Wrapf(srvErr, "error during server startup")
		}
		return nil
	case <-ctx.Done():
		err = b.ShutdownAll()
	}

	// Check if there might be any other error than `http.ErrServerClosed`
	// occurred during the server shutdown.
	<-srvDone
	if srvErr != nil {
		if err != nil {
			return errors.Join(err, srvErr)
		}
		return pkgerrors.Wrapf(srvErr, "error during server shutdown")
	}

	return err
}

func (b *Bean) UseMiddlewares(middlewares ...echo.MiddlewareFunc) {
	b.Echo.Use(middlewares...)
}

func (b *Bean) UseErrorHandlerFuncs(errHdlrFuncs ...berror.ErrorHandlerFunc) {
	if b.errorHandlerFuncs == nil {
		b.errorHandlerFuncs = []berror.ErrorHandlerFunc{}
	}
	b.errorHandlerFuncs = append(b.errorHandlerFuncs, errHdlrFuncs...)
}

func (b *Bean) UseValidation(validateFuncs ...validator.ValidatorFunc) {
	for _, validateFunc := range validateFuncs {
		if err := validateFunc(b.Validate); err != nil {
			b.Echo.Logger.Error(err.Error())
		}
	}
}

func (b *Bean) DefaultHTTPErrorHandler() echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Committed {
			return
		}

		for _, handle := range b.errorHandlerFuncs {
			handled, err := handle(err, c)
			if err != nil {
				trace.SentryCaptureExceptionWithEcho(c, err)
			}
			if handled {
				break
			}
		}
	}
}

// InitDB initialize all the database dependencies and store it in global variable `global.DBConn`.
func (b *Bean) InitDB() error {
	var masterMySQLDB *gorm.DB
	var masterMySQLDBName string
	var masterMongoDB *mongo.Client
	var masterMongoDBName string
	var masterRedisDB *dbdrivers.RedisDBConn
	var tenantMySQLDBs map[uint64]*gorm.DB
	var tenantMySQLDBNames map[uint64]string
	var tenantMongoDBs map[uint64]*mongo.Client
	var tenantMongoDBNames map[uint64]string
	var tenantRedisDBs map[uint64]*dbdrivers.RedisDBConn
	var masterMemoryDB memory.Cache
	var cleanups []func() error

	masterMySQLDB, masterMySQLDBName, close, err := dbdrivers.InitMysqlMasterConn(b.Config.Database.MySQL, appLogger())
	if err != nil {
		return fmt.Errorf("failed to initialize master mysql db: %w", err)
	}
	cleanups = append(cleanups, close)

	masterMongoDB, masterMongoDBName, close, err = dbdrivers.InitMongoMasterConn(b.Config.Database.Mongo, appLogger())
	if err != nil {
		return fmt.Errorf("failed to initialize master mongo db: %w", err)
	}
	cleanups = append(cleanups, close)

	var redisCloses []func() error
	masterRedisDB, redisCloses, err = dbdrivers.InitRedisMasterConn(b.Config.Database.Redis)
	if err != nil {
		return fmt.Errorf("failed to initialize master redis db: %w", err)
	}
	cleanups = append(cleanups, redisCloses...)

	if b.Config.Database.Tenant.On {
		var closeDBs []func() error

		tenantMySQLDBs, tenantMySQLDBNames, closeDBs, err = dbdrivers.InitMysqlTenantConns(b.Config.Database.MySQL, masterMySQLDB, TenantAlterDbHostParam, b.Config.Secret, appLogger())
		if err != nil {
			return fmt.Errorf("failed to initialize tenant mysql dbs: %w", err)
		}
		cleanups = append(cleanups, closeDBs...)

		tenantMongoDBs, tenantMongoDBNames, closeDBs, err = dbdrivers.InitMongoTenantConns(b.Config.Database.Mongo, masterMySQLDB, TenantAlterDbHostParam, b.Config.Secret, appLogger())
		if err != nil {
			return fmt.Errorf("failed to initialize tenant mongo dbs: %w", err)
		}
		cleanups = append(cleanups, closeDBs...)

		tenantRedisDBs, closeDBs, err = dbdrivers.InitRedisTenantConns(b.Config.Database.Redis, masterMySQLDB, TenantAlterDbHostParam, b.Config.Secret)
		if err != nil {
			return fmt.Errorf("failed to initialize tenant redis dbs: %w", err)
		}
		cleanups = append(cleanups, closeDBs...)
	}

	if b.Config.Database.Memory.On {
		masterMemoryDB = memory.NewMemoryCache()
		cleanups = append(cleanups, func() error {
			masterMemoryDB.CloseMemory()
			return nil // no error
		})
	}

	b.DBConn = &DBDeps{
		MasterMySQLDB:      masterMySQLDB,
		MasterMySQLDBName:  masterMySQLDBName,
		TenantMySQLDBs:     tenantMySQLDBs,
		TenantMySQLDBNames: tenantMySQLDBNames,
		MasterMongoDB:      masterMongoDB,
		MasterMongoDBName:  masterMongoDBName,
		TenantMongoDBs:     tenantMongoDBs,
		TenantMongoDBNames: tenantMongoDBNames,
		MasterRedisDB:      masterRedisDB,
		TenantRedisDBs:     tenantRedisDBs,
		MemoryDB:           masterMemoryDB,
	}

	b.CleanupDBs = append(b.CleanupDBs, cleanups...)

	return nil
}

// ShutdownAll closes all the server and database related resources.
func (b *Bean) ShutdownAll() error {
	var err error

	// Close the server related resources first.
	if srvErr := b.Shutdown(); srvErr != nil {
		err = errors.Join(err, srvErr)
	}

	// Close the database related resources next.
	if dbErr := b.CleanupDB(); dbErr != nil {
		err = errors.Join(err, dbErr)
	}

	// Flush and stop async access log sink after other resources are closed,
	// so shutdown-phase logs can still be written.
	logTimeout := b.Config.HTTP.ShutdownTimeout
	if logTimeout <= 0 {
		logTimeout = 5 * time.Second
	}
	logCtx, cancel := context.WithTimeout(context.Background(), logTimeout)
	defer cancel()
	if logErr := blog.Shutdown(logCtx); logErr != nil {
		err = errors.Join(err, logErr)
	}

	if err != nil {
		return fmt.Errorf("failed to shutdown: %w", err)
	}

	return nil
}

// Shutdown closes all the server related resources.
func (b *Bean) Shutdown() error {
	if len(b.ShutdownSrv) == 0 {
		b.Echo.Logger.Info("No server shutdown function found.")
		return nil
	}

	b.Echo.Logger.Info("Shutting down server...🛬")

	if err := closer(b.ShutdownSrv)(); err != nil {
		return err
	}

	b.Echo.Logger.Info("Server has been shutdown gracefully.")
	return nil
}

// CleanupDB closes all the database (master and tenant) related resources.
func (b *Bean) CleanupDB() error {
	if len(b.CleanupDBs) == 0 {
		b.Echo.Logger.Info("No database cleanup function found.")
		return nil
	}

	b.Echo.Logger.Info("Cleaning up databases...🧹")

	if err := closer(b.CleanupDBs)(); err != nil {
		return err
	}

	b.Echo.Logger.Info("Databases have been cleaned up successfully.")
	return nil
}

func closer(closers []func() error) func() error {
	return func() error {
		if len(closers) == 0 {
			return nil
		}

		var err error
		// Execute the last added closer first.
		for i := len(closers) - 1; i >= 0; i-- {
			if closers[i] == nil {
				continue // Skip nil closer
			}
			if cErr := closers[i](); cErr != nil {
				err = errors.Join(err, cErr)
			}
		}
		if err != nil {
			return err
		}

		return nil
	}
}

// ContextTimeout return custom context timeout middleware
func ContextTimeout(timeout time.Duration) echo.MiddlewareFunc {
	timeoutErrorHandler := func(c *echo.Context, err error) error {
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return echo.NewHTTPError(http.StatusGatewayTimeout, "gateway timeout").Wrap(err)
			}
			return err
		}
		return nil
	}

	return echomiddleware.ContextTimeoutWithConfig(echomiddleware.ContextTimeoutConfig{
		Timeout:      timeout,
		ErrorHandler: timeoutErrorHandler,
	})
}

// NewValidator creates a new validator instance.
// Currently, it only supports a default validator.
func NewValidator(v *validatorV10.Validate) (*validator.DefaultValidator, error) {
	return validator.NewDefaultValidator(v)
}

// appLogger returns the application logger, or slog's default logger if bean's logger is not initialized.
func appLogger() *slog.Logger {
	if l := blog.Logger(); l != nil {
		return l.Logger
	}
	return slog.Default()
}
