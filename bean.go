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
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	validatorV10 "github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	elog "github.com/labstack/gommon/log"
	pkgerrors "github.com/pkg/errors"
	"github.com/retail-ai-inc/bean/v2/config"
	"github.com/retail-ai-inc/bean/v2/echoview"
	berror "github.com/retail-ai-inc/bean/v2/error"
	"github.com/retail-ai-inc/bean/v2/goview"
	"github.com/retail-ai-inc/bean/v2/helpers"
	"github.com/retail-ai-inc/bean/v2/internal/binder"
	"github.com/retail-ai-inc/bean/v2/internal/dbdrivers"
	"github.com/retail-ai-inc/bean/v2/internal/gopool"
	"github.com/retail-ai-inc/bean/v2/internal/middleware"
	"github.com/retail-ai-inc/bean/v2/internal/regex"
	broute "github.com/retail-ai-inc/bean/v2/internal/route"
	"github.com/retail-ai-inc/bean/v2/internal/validator"
	blog "github.com/retail-ai-inc/bean/v2/log"
	"github.com/retail-ai-inc/bean/v2/store/memory"
	"github.com/retail-ai-inc/bean/v2/trace"
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

func New() *Bean {
	if config.Bean == nil {
		log.Fatal("config is not loaded")
	}

	// Create a new echo instance
	e, closeEcho := NewEcho()

	b := &Bean{
		Echo:     e,
		Validate: validatorV10.New(),
		Config:   *config.Bean,
	}

	// If `NetHttpFastTransporter` is on from env.json then initialize it.
	if config.Bean.NetHttpFastTransporter.On {
		resolver := &dnscache.Resolver{}
		if config.Bean.NetHttpFastTransporter.MaxIdleConns == nil {
			*config.Bean.NetHttpFastTransporter.MaxIdleConns = 0
		}

		if config.Bean.NetHttpFastTransporter.MaxIdleConnsPerHost == nil {
			*config.Bean.NetHttpFastTransporter.MaxIdleConnsPerHost = 0
		}

		if config.Bean.NetHttpFastTransporter.MaxConnsPerHost == nil {
			*config.Bean.NetHttpFastTransporter.MaxConnsPerHost = 0
		}

		if config.Bean.NetHttpFastTransporter.IdleConnTimeout == nil {
			*config.Bean.NetHttpFastTransporter.IdleConnTimeout = 0
		}

		if config.Bean.NetHttpFastTransporter.DNSCacheTimeout == nil {
			*config.Bean.NetHttpFastTransporter.DNSCacheTimeout = 5 * time.Minute
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
			MaxIdleConns:        *config.Bean.NetHttpFastTransporter.MaxIdleConns,
			MaxIdleConnsPerHost: *config.Bean.NetHttpFastTransporter.MaxIdleConnsPerHost,
			MaxConnsPerHost:     *config.Bean.NetHttpFastTransporter.MaxConnsPerHost,
			IdleConnTimeout:     *config.Bean.NetHttpFastTransporter.IdleConnTimeout,
		}

		// IMPORTANT: Refresh unused DNS cache in every 5 minutes by default unless set via env.json.
		go func() {
			t := time.NewTicker(*config.Bean.NetHttpFastTransporter.DNSCacheTimeout)
			defer t.Stop()
			for range t.C {
				resolver.Refresh(true)
			}
		}()
	}

	// If `memory` database is on and `delKeyAPI` end point along with bearer token are properly set.
	if config.Bean.Database.Memory.On && config.Bean.Database.Memory.DelKeyAPI.EndPoint != "" {
		e.DELETE(config.Bean.Database.Memory.DelKeyAPI.EndPoint, func(c echo.Context) error {
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

	return b
}

func NewEcho() (*echo.Echo, func() error) {
	if config.Bean == nil {
		log.Fatal("config is not loaded")
	}

	e := echo.New()
	closes := []func() error{}

	// Hide default `Echo` banner during startup.
	e.HideBanner = true

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

	// IMPORTANT: Configure debug log.
	if config.Bean.DebugLogPath != "" {
		if file, err := openFile(config.Bean.DebugLogPath); err != nil {
			e.Logger.Fatalf("Unable to open log file: %v Server 🚀  crash landed. Exiting...\n", err)
		} else {
			e.Logger.SetOutput(file)
		}
	}
	e.Logger.SetLevel(elog.DEBUG)

	// Initialize `BeanLogger` global variable using `e.Logger`.
	blog.Set(e.Logger)

	// Adds a `Server` header to the response.
	e.Use(middleware.ServerHeader(config.Bean.ProjectName, helpers.CurrVersion()))

	// Sets the maximum allowed size for a request body, return `413 - Request Entity Too Large` if the size exceeds the limit.
	e.Use(echomiddleware.BodyLimit(config.Bean.HTTP.BodyLimit))

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
	}))

	// Return `405 Method Not Allowed` if a wrong HTTP method been called for an API route.
	// Return `404 Not Found` if a wrong API route been called.
	e.Use(middleware.MethodNotAllowedAndRouteNotFound())

	// IMPORTANT: Configure access log and body dumper. (can be turn off)
	if config.Bean.AccessLog.On {
		accessLogConfig := middleware.LoggerConfig{
			Skipper:        regex.InitAccessLogPathSkipper(config.Bean.AccessLog.SkipEndpoints),
			BodyDump:       config.Bean.AccessLog.BodyDump,
			RequestHeader:  config.Bean.AccessLog.ReqHeaderParam,
			ResponseHeader: config.Bean.AccessLog.ResHeaderParam,
		}

		if config.Bean.AccessLog.Path != "" {
			if file, err := openFile(config.Bean.AccessLog.Path); err != nil {
				e.Logger.Fatalf("Unable to open log file: %v Server 🚀  crash landed. Exiting...\n", err)
			} else {
				accessLogConfig.Output = file
			}
		}
		if len(config.Bean.AccessLog.BodyDumpMaskParam) > 0 {
			accessLogConfig.MaskedParameters = config.Bean.AccessLog.BodyDumpMaskParam
		}
		accessLogger := middleware.AccessLoggerWithConfig(accessLogConfig)
		e.Use(accessLogger)
	}

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
			e.Logger.Error("Sentry initialization failed: client options is empty")
		} else {
			clientOption := config.Bean.Sentry.ClientOptions
			if clientOption.TracesSampleRate > 0 {
				clientOption.EnableTracing = true
			}
			if err := sentry.Init(*clientOption); err != nil {
				e.Logger.Fatal("Sentry initialization failed: ", err, ". Server 🚀  crash landed. Exiting...")
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
				regex.SetSamplingPathSkipper(config.Bean.Sentry.SkipTracesEndpoints)
				e.Use(middleware.SkipSampling)
			}
		}
	}
	closes = append(closes, flushSentry)

	// Some pre-build middleware initialization.
	e.Pre(echomiddleware.RemoveTrailingSlash())
	if config.Bean.HTTP.IsHttpsRedirect {
		e.Pre(echomiddleware.HTTPSRedirect())
	}
	e.Use(echomiddleware.Recover())

	// IMPORTANT: Request related middleware.
	// Set the `X-Request-ID` header field if it doesn't exist.
	// Whether to skip it depends on whether the `AccessLog.ReqHeaderParam`` parameter contains `X-Request-Id`.
	e.Use(echomiddleware.RequestIDWithConfig(echomiddleware.RequestIDConfig{
		Skipper: func(c echo.Context) bool {
			return !slices.Contains(config.Bean.AccessLog.ReqHeaderParam, echo.HeaderXRequestID)
		},
		Generator:        uuid.NewString,
		RequestIDHandler: middleware.RequestIDHandler,
		TargetHeader:     echo.HeaderXRequestID,
	}))

	// Enable prometheus metrics middleware. Metrics data should be accessed via `/metrics` endpoint.
	// This will help us to integrate `bean's` health into `k8s`.
	if config.Bean.Prometheus.On {
		const metricsPath = "/metrics" // fixed path
		skipper, err := regex.InitPrometheusPathSkipper(config.Bean.Prometheus.SkipEndpoints, metricsPath)
		if err != nil {
			e.Logger.Fatalf("Prometheus initialization failed: %v. Server 🚀  crash landed. Exiting...\n", err)
		}
		conf := echoprometheus.MiddlewareConfig{
			Skipper: skipper,
		}
		if config.Bean.Prometheus.Subsystem != "" {
			conf.Subsystem = config.Bean.Prometheus.Subsystem
		}
		e.Use(echoprometheus.NewMiddlewareWithConfig(conf))
		e.GET(metricsPath, echoprometheus.NewHandler())
	}

	// Register goroutine pool
	if len(config.Bean.AsyncPool) > 0 {
		for _, asyncPool := range config.Bean.AsyncPool {
			if asyncPool.Name == "" {
				continue
			}

			pool, err := gopool.NewPool(asyncPool.Size, asyncPool.BlockAfter)
			if err != nil {
				e.Logger.Fatal(err, ". Server 🚀  crash landed. Exiting...")
			}

			if err := gopool.Register(asyncPool.Name, pool); err != nil {
				e.Logger.Fatal(err, ". Server 🚀  crash landed. Exiting...")
			}
		}
	}
	closes = append(closes, gopool.ReleaseAllPools(config.Bean.AsyncPoolReleaseTimeout))

	return e, closer(closes)
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

	s := http.Server{
		Addr:    host + ":" + port,
		Handler: b.Echo,
	}

	// IMPORTANT: Keep-alive is default true but I kept this here to let you guys no that there is a settings
	// for it :)
	s.SetKeepAlivesEnabled(b.Config.HTTP.KeepAlive)

	// before bean bootstrap
	if b.BeforeServe != nil {
		b.BeforeServe()
	}

	// Keep all the route information in route.Routes
	broute.Init(b.Echo)

	// Start the server
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		var err error
		if b.Config.HTTP.SSL.On {
			s.TLSConfig = &tls.Config{
				MinVersion: b.Config.HTTP.SSL.MinTLSVersion,
			}
			err = s.ListenAndServeTLS(b.Config.HTTP.SSL.CertFile, b.Config.HTTP.SSL.PrivFile)
		} else {
			err = s.ListenAndServe()
		}
		// If shutdown is called, the server will return `http.ErrServerClosed` immediately.
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	b.ShutdownSrv = append(b.ShutdownSrv, func() error {
		var timeout time.Duration
		if b.Config.HTTP.ShutdownTimeout > 0 {
			timeout = b.Config.HTTP.ShutdownTimeout
		} else {
			timeout = 30 * time.Second
		}
		sdnCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err = s.Shutdown(sdnCtx)
		if err != nil {
			// Even after timeout, shutdown keeps handling ongoing requests in the background
			// while returning timeout error until main goroutine exits.
			err = pkgerrors.Wrapf(err, "failed to gracefully shutdown")
			return err
		}

		return nil
	})

	select {
	case srvErr := <-errCh:
		if srvErr != nil {
			return pkgerrors.Wrapf(srvErr, "error during server startup")
		}
	case <-ctx.Done(): // Wait for the interrupt signal or termination signal.
		err = b.ShutdownAll()
	}

	// Check if there might be any other error than `http.ErrServerClosed`
	// occurred during the server startup or shutdown.
	if srvErr := <-errCh; srvErr != nil {
		if err != nil {
			return errors.Join(err, srvErr)
		}
		return pkgerrors.Wrapf(srvErr, "error during server shutdown")
	}

	if err != nil {
		return err
	}

	return nil
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
			b.Echo.Logger.Error(err)
		}
	}
}

func (b *Bean) DefaultHTTPErrorHandler() echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
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

	masterMySQLDB, masterMySQLDBName, close, err := dbdrivers.InitMysqlMasterConn(b.Config.Database.MySQL)
	if err != nil {
		return fmt.Errorf("failed to initialize master mysql db: %w", err)
	}
	cleanups = append(cleanups, close)

	masterMongoDB, masterMongoDBName, close, err = dbdrivers.InitMongoMasterConn(b.Config.Database.Mongo, blog.Logger())
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

		tenantMySQLDBs, tenantMySQLDBNames, closeDBs, err = dbdrivers.InitMysqlTenantConns(b.Config.Database.MySQL, masterMySQLDB, TenantAlterDbHostParam, b.Config.Secret)
		if err != nil {
			return fmt.Errorf("failed to initialize tenant mysql dbs: %w", err)
		}
		cleanups = append(cleanups, closeDBs...)

		tenantMongoDBs, tenantMongoDBNames, closeDBs, err = dbdrivers.InitMongoTenantConns(b.Config.Database.Mongo, masterMySQLDB, TenantAlterDbHostParam, b.Config.Secret, blog.Logger())
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

// openFile opens and return the file, if doesn't exist, create it, or append to the file with the directory.
func openFile(path string) (*os.File, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(path), 0o764); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o664)
}

// ContextTimeout return custom context timeout middleware
func ContextTimeout(timeout time.Duration) echo.MiddlewareFunc {
	timeoutErrorHandler := func(err error, c echo.Context) error {
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return &echo.HTTPError{
					Code:     http.StatusGatewayTimeout,
					Message:  "gateway timeout",
					Internal: err,
				}
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
