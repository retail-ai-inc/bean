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
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	validatorV10 "github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/panjf2000/ants/v2"
	"github.com/retail-ai-inc/bean/v2/echoview"
	berror "github.com/retail-ai-inc/bean/v2/error"
	"github.com/retail-ai-inc/bean/v2/goview"
	"github.com/retail-ai-inc/bean/v2/helpers"
	"github.com/retail-ai-inc/bean/v2/internal/binder"
	"github.com/retail-ai-inc/bean/v2/internal/dbdrivers"
	"github.com/retail-ai-inc/bean/v2/internal/gopool"
	"github.com/retail-ai-inc/bean/v2/internal/middleware"
	broute "github.com/retail-ai-inc/bean/v2/internal/route"
	"github.com/retail-ai-inc/bean/v2/internal/validator"
	"github.com/retail-ai-inc/bean/v2/store/memory"
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
	errorHandlerFuncs []berror.ErrorHandlerFunc
	validate          *validatorV10.Validate
	Config            Config
}

type SentryConfig struct {
	On                  bool
	Debug               bool
	Dsn                 string
	Timeout             time.Duration
	TracesSampleRate    float64
	SkipTracesEndpoints []string
	ClientOptions       *sentry.ClientOptions
	ConfigureScope      func(scope *sentry.Scope)
}

type Config struct {
	ProjectName  string
	Environment  string
	DebugLogPath string
	Secret       string
	AccessLog    struct {
		On                bool
		BodyDump          bool
		Path              string
		BodyDumpMaskParam []string
		ReqHeaderParam    []string
		SkipEndpoints     []string
	}
	Prometheus struct {
		On            bool
		SkipEndpoints []string
	}
	HTTP struct {
		Port            string
		Host            string
		BodyLimit       string
		IsHttpsRedirect bool
		Timeout         time.Duration
		ErrorMessage    struct {
			E404 struct {
				Json []struct {
					Key   string
					Value string
				}
				Html struct {
					File string
				}
			}
			E405 struct {
				Json []struct {
					Key   string
					Value string
				}
				Html struct {
					File string
				}
			}
			E500 struct {
				Json []struct {
					Key   string
					Value string
				}
				Html struct {
					File string
				}
			}
			E504 struct {
				Json []struct {
					Key   string
					Value string
				}
				Html struct {
					File string
				}
			}
			Default struct {
				Json []struct {
					Key   string
					Value string
				}
				Html struct {
					File string
				}
			}
		}
		KeepAlive     bool
		AllowedMethod []string
		SSL           struct {
			On            bool
			CertFile      string
			PrivFile      string
			MinTLSVersion uint16
		}
	}
	NetHttpFastTransporter struct {
		On                  bool
		MaxIdleConns        *int
		MaxIdleConnsPerHost *int
		MaxConnsPerHost     *int
		IdleConnTimeout     *time.Duration
		DNSCacheTimeout     *time.Duration
	}
	HTML struct {
		ViewsTemplateCache bool
	}
	Database struct {
		Tenant struct {
			On bool
		}
		MySQL  dbdrivers.SQLConfig
		Mongo  dbdrivers.MongoConfig
		Redis  dbdrivers.RedisConfig
		Memory dbdrivers.MemoryConfig
	}
	Sentry   SentryConfig
	Security struct {
		HTTP struct {
			Header struct {
				XssProtection         string
				ContentTypeNosniff    string
				XFrameOptions         string
				HstsMaxAge            int
				ContentSecurityPolicy string
			}
		}
	}
	AsyncPool []struct {
		Name       string
		Size       *int
		BlockAfter *int
	}
}

// This is a global variable to hold the debug logger so that we can log data from service, repository or anywhere.
var BeanLogger echo.Logger

// If a command or service wants to use a different `host` parameter for tenant database connection
// then it's easy to do just by passing that parameter string name using `bean.TenantAlterDbHostParam`.
// Therfore, `bean` will overwrite all host string in `TenantConnections`.`Connections` JSON.
var TenantAlterDbHostParam string

// Hold the useful configuration settings of bean so that we can use it quickly from anywhere.
var BeanConfig Config

// Support a DNS cache version of the net/http Transport.
var NetHttpFastTransporter *http.Transport

func New() (b *Bean) {

	// Create a new echo instance
	e := NewEcho()

	b = &Bean{
		Echo:     e,
		validate: validatorV10.New(),
		Config:   BeanConfig,
	}

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
		e.DELETE(BeanConfig.Database.Memory.DelKeyAPI.EndPoint, func(c echo.Context) error {
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
			b.DBConn.MemoryDB.DelMemory(key)
			return c.JSON(http.StatusOK, map[string]interface{}{
				"message": "Done",
			})
		})
	}

	return b
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
	BeanLogger = e.Logger

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
		accessLogConfig := middleware.LoggerConfig{
			Skipper:       endPointsSkipper(BeanConfig.AccessLog.SkipEndpoints),
			BodyDump:      BeanConfig.AccessLog.BodyDump,
			RequestHeader: BeanConfig.AccessLog.ReqHeaderParam,
		}

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

	// Add context timeout.
	// If no timeout is set or timeout=0, skip adding the timeout middleware.
	timeoutDur := BeanConfig.HTTP.Timeout
	if timeoutDur > 0 {
		e.Use(ContextTimeout(timeoutDur))
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
		p := prometheus.NewPrometheus("echo", endPointsSkipper(BeanConfig.Prometheus.SkipEndpoints))
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
			e.Logger.Fatal("async pool initialization failed: ", err, ". Server 🚀  crash landed. Exiting...")
		}

		err = gopool.Register(asyncPool.Name, pool)
		if err != nil {
			e.Logger.Fatal("goroutine pool register failed: ", err, ". Server 🚀  crash landed. Exiting...")
		}
	}

	return e
}

func (b *Bean) ServeAt(host, port string) {
	b.Echo.Logger.Info("Starting " + b.Config.Environment + " " + b.Config.ProjectName + " at " + host + ":" + port + "...🚀")

	b.UseErrorHandlerFuncs(berror.DefaultErrorHanderFunc)
	b.Echo.HTTPErrorHandler = b.DefaultHTTPErrorHandler()

	b.Echo.Validator = &validator.DefaultValidator{Validator: b.validate}

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
	if b.Config.HTTP.SSL.On {
		s.TLSConfig = &tls.Config{
			MinVersion: b.Config.HTTP.SSL.MinTLSVersion,
		}

		if err := s.ListenAndServeTLS(b.Config.HTTP.SSL.CertFile, b.Config.HTTP.SSL.PrivFile); err != nil && err != http.ErrServerClosed {
			b.Echo.Logger.Fatal(err)
		}

	} else {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			b.Echo.Logger.Fatal(err)
		}
	}
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
		if err := validateFunc(b.validate); err != nil {
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
				if BeanConfig.Sentry.On {
					SentryCaptureException(c, err)
				} else {
					c.Logger().Error(err)
				}
			}
			if handled {
				break
			}
		}
	}
}

// InitDB initialize all the database dependencies and store it in global variable `global.DBConn`.
func (b *Bean) InitDB() {
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

	if b.Config.Database.Tenant.On {
		masterMySQLDB, masterMySQLDBName = dbdrivers.InitMysqlMasterConn(b.Config.Database.MySQL)
		tenantMySQLDBs, tenantMySQLDBNames = dbdrivers.InitMysqlTenantConns(b.Config.Database.MySQL, masterMySQLDB, TenantAlterDbHostParam, b.Config.Secret)
		tenantMongoDBs, tenantMongoDBNames = dbdrivers.InitMongoTenantConns(b.Config.Database.Mongo, masterMySQLDB, TenantAlterDbHostParam, b.Config.Secret)
		masterRedisDB = dbdrivers.InitRedisMasterConn(b.Config.Database.Redis)
		tenantRedisDBs = dbdrivers.InitRedisTenantConns(b.Config.Database.Redis, masterMySQLDB, TenantAlterDbHostParam, b.Config.Secret)
	} else {
		masterMySQLDB, masterMySQLDBName = dbdrivers.InitMysqlMasterConn(b.Config.Database.MySQL)
		masterMongoDB, masterMongoDBName = dbdrivers.InitMongoMasterConn(b.Config.Database.Mongo)
		masterRedisDB = dbdrivers.InitRedisMasterConn(b.Config.Database.Redis)
	}

	if b.Config.Database.Memory.On {
		masterMemoryDB = memory.NewMemoryCache()
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
}

// The bean Logger to have debug log from anywhere.
func Logger() echo.Logger {
	return BeanLogger
}

// SentryCaptureException  This is a global function to send sentry exception if you configure the sentry through env.json. You cann pass a proper context or nil.
// if you want to capture exception in async function, please use async.CaptureException.
func SentryCaptureException(c echo.Context, err error) {
	if err == nil {
		return
	}

	if !BeanConfig.Sentry.On {
		return
	}

	if c != nil {
		// If the function get a proper context then push the request headers and URI along with other meaningful info.
		if hub := sentryecho.GetHubFromContext(c); hub != nil {
			hub.CaptureException(err)
		} else {
			Logger().Warnf("echo context is missing hub information")
		}

		return
	}

	// If someone call the function from service/repository without a proper context.
	sentry.CurrentHub().Clone().CaptureException(err)
}

// This is a global function to send sentry message if you configure the sentry through env.json. You cann pass a proper context or nil.
func SentryCaptureMessage(c echo.Context, msg string) {
	if !BeanConfig.Sentry.On {
		return
	}

	if c != nil {
		// If the function get a proper context then push the request headers and URI along with other meaningful info.
		if hub := sentryecho.GetHubFromContext(c); hub != nil {
			hub.CaptureMessage(msg)
		}

		return
	}

	// If someone call the function from service/repository without a proper context.
	sentry.CurrentHub().Clone().CaptureMessage(msg)
}

// To clean up any bean resources before the program terminates.
// Call this function using `defer` like `defer Cleanup()`
func Cleanup() {
	if BeanConfig.Sentry.On {
		// Flush buffered sentry events if any.
		sentry.Flush(BeanConfig.Sentry.Timeout)
	}
}

// Modify event through beforeSend function.
func DefaultBeforeSend(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
	// Example: enriching the event by adding aditional data.
	switch err := hint.OriginalException.(type) {
	case *validator.ValidationError:
		return event
	case *berror.APIError:
		if err.Ignorable {
			return nil
		}
		event.Contexts["Error"] = map[string]interface{}{
			"HTTPStatusCode": err.HTTPStatusCode,
			"GlobalErrCode":  err.GlobalErrCode,
			"Message":        err.Error(),
		}
		return event
	case *echo.HTTPError:
		return event
	default:
		return event
	}
}

// Modify breadcrumbs through beforeBreadcrumb function.
func DefaultBeforeBreadcrumb(breadcrumb *sentry.Breadcrumb, hint *sentry.BreadcrumbHint) *sentry.Breadcrumb {
	// Example: discard the breadcrumb by return nil.
	// if breadcrumb.Category == "example" {
	// 	return nil
	// }
	return breadcrumb
}

// endPointsSkipper ignores endpoints which are listed in skipEndpoints for logging or
// metrics data collection.
func endPointsSkipper(skipEndpoints []string) func(c echo.Context) bool {
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
