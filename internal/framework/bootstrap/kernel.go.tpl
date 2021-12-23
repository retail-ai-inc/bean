{{ .Copyright }}
// IMPORTANT: PLEASE DO NOT UPDATE THIS FILE.
package bootstrap

import (
	"fmt"
	"os"
	"time"

	"{{ .PkgName }}/framework/dbdrivers"
	"{{ .PkgName }}/framework/internals/binder"
	ierror "{{ .PkgName }}/framework/internals/error"
	"{{ .PkgName }}/framework/internals/global"
	"{{ .PkgName }}/framework/internals/helpers"
	imiddleware "{{ .PkgName }}/framework/internals/middleware"
	str "{{ .PkgName }}/framework/internals/string"
	"{{ .PkgName }}/framework/internals/template"
	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/mongo"
	"gorm.io/gorm"
)

func New() *echo.Echo {

	// Parse bean system files and directories.
	helpers.ParseBeanSystemFilesAndDirectorires()

	e := echo.New()

	// Initialize the global echo instance. This is useful to print log from `internals` packages.
	global.EchoInstance = e

	// This will handle invalid JSON and other errors.
	e.HTTPErrorHandler = ierror.HTTPErrorHandler

	// Hide default `Echo` banner during startup.
	e.HideBanner = true

	// Set viper path and read configuration. You must keep `env.json` file in the root of your project.
	viper.AddConfigPath(".")
	viper.SetConfigType("json")
	viper.SetConfigName("env")

	err := viper.ReadInConfig()
	if err != nil {
		fmt.Printf("Configuration file error: %v Server 🚀  crash landed. Exiting...\n", err)

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(1)
	}

	// IMPORTANT: Time out middleware. It has to be the first middleware to initialize.
	e.Use(imiddleware.RequestTimeout(viper.GetDuration("http.timeout") * time.Second))

	// Get log type (file or stdout) settings from config.
	isLogStdout := viper.GetBool("isLogStdout")
	logFile := viper.GetString("logFile")

	e.Logger.SetLevel(log.DEBUG)

	// Print logs on terminal a.k.a stdout instead console.log.
	if isLogStdout {
		logger := echomiddleware.LoggerWithConfig(echomiddleware.LoggerConfig{
			Format: helpers.JsonLogFormat(), // we need additional access log parameter
		})
		e.Use(logger)

	} else {
		// IMPORTANT: Set log output into file (console.log) instead `stdout`.
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(logFile), 0754); err != nil {
				fmt.Printf("Unable to create log file: %v Server 🚀  crash landed. Exiting...\n", err)
				os.Exit(1)
			}
		}

		logfp, err := os.OpenFile(logFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0664)
		if err != nil {
			fmt.Printf("Unable to open log file: %v Server 🚀  crash landed. Exiting...\n", err)
			os.Exit(1)
		}

		e.Logger.SetOutput(logfp)

		logger := echomiddleware.LoggerWithConfig(echomiddleware.LoggerConfig{
			Format: helpers.JsonLogFormat(), // we need additional access log parameter
			Output: logfp,
		})

		e.Use(logger)
	}

	// Set the environment parameter in `global.Environment`
	global.Environment = viper.GetString("environment")
	e.Logger.Info("ENVIRONMENT: ", global.Environment)

	// Some pre-build middleware initialization.
	e.Pre(echomiddleware.RemoveTrailingSlash())
	e.Use(echomiddleware.Recover())

	// Enable prometheus metrics middleware. Metrics data should be accessed via `/metrics` endpoint.
	// This will help us to integrate `bean's` health into `k8s`.
	isPrometheusMetrics := viper.GetBool("prometheus.isPrometheusMetrics")
	if isPrometheusMetrics {
		p := prometheus.NewPrometheus("echo", prometheusUrlSkipper)
		p.Use(e)
	}

	// Setup basic echo view template.
	e.Renderer = template.New(e)

	// CORS initialization and support only HTTP methods which are configured under `http.allowedMethod`
	// parameters in `env.json`.
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: viper.GetStringSlice("http.allowedMethod"),
	}))

	// Sentry `panic`` error handler initialization if activated from `env.json`
	isSentry := viper.GetBool("sentry.isSentry")
	if isSentry {
		// To initialize Sentry's handler, we need to initialize sentry first.
		err := sentry.Init(sentry.ClientOptions{
			Dsn:              viper.GetString("sentry.dsn"),
			AttachStacktrace: true,
		})
		if err != nil {
			e.Logger.Fatal("Sentry initialization failed: ", err, ". Server 🚀  crash landed. Exiting...")

			// Go does not use an integer return value from main to indicate exit status.
			// To exit with a non-zero status we should use os.Exit.
			os.Exit(1)
		}

		// Once it's done, let's attach the handler as one of our middleware.
		e.Use(sentryecho.New(sentryecho.Options{
			Repanic: true,
		}))
	}

	// Dump request body for logging purpose if activated from `env.json`
	isBodyDump := viper.GetBool("isBodyDump")
	if isBodyDump {
		bodyDumper := echomiddleware.BodyDumpWithConfig(echomiddleware.BodyDumpConfig{
			Handler: helpers.BodyDumpHandler,
		})

		e.Use(bodyDumper)
	}

	// Body limit middleware sets the maximum allowed size for a request body,
	// if the size exceeds the configured limit, it sends “413 - Request Entity Too Large” response.
	e.Use(echomiddleware.BodyLimit(viper.GetString("http.bodyLimit")))

	// ---------- HTTP headers security for XSS protection and alike ----------
	e.Use(echomiddleware.SecureWithConfig(echomiddleware.SecureConfig{
		XSSProtection:         "1; mode=block",                                               // Adds the X-XSS-Protection header with the value `1; mode=block`.
		ContentTypeNosniff:    "nosniff",                                                     // Adds the X-Content-Type-Options header with the value `nosniff`.
		XFrameOptions:         "SAMEORIGIN",                                                  // The X-Frame-Options header value to be set with a custom value.
		HSTSMaxAge:            31536000,                                                      // STS header is only included when the connection is HTTPS.
		ContentSecurityPolicy: viper.GetString("security.http.header.contentSecurityPolicy"), // Allows the Content-Security-Policy header value to be set with a custom value.
	}))
	// ---------- HTTP headers security for XSS protection and alike ----------

	// ------ HTTPS redirect middleware redirects http requests to https ------
	isHttpsRedirect := viper.GetBool("http.isHttpsRedirect")
	if isHttpsRedirect {
		e.Pre(echomiddleware.HTTPSRedirect())
	}
	// ------ HTTPS redirect middleware redirects http requests to https ------

	// Return `405 Method Not Allowed` if a wrong HTTP method been called for an API route.
	// Return `404 Not Found` if a wrong API route been called.
	e.Use(imiddleware.MethodNotAllowedAndRouteNotFound())

	// -------------- Special Middleware And Controller To Get Server Stats --------------
	serverStats := imiddleware.NewServerStats()

	e.Use(imiddleware.LatencyRecorder())
	e.GET("/route/stats", serverStats.GetServerStats)
	// -------------- Special Middleware And Controller To Get Server Stats --------------

	// Adds a `Server` header to the response.
	name := viper.GetString("name")
	version := viper.GetString("version")
	e.Use(imiddleware.ServerHeader(name, version))

	// `/ping` uri to response a `pong`.
	e.Use(imiddleware.Heartbeat())

	// Initialize all database driver.
	var masterMySQLDB *gorm.DB
	var masterMySQLDBName string
	var masterMongoDB *mongo.Client
	var masterMongoDBName string
	var masterRedisDB *redis.Client
	var masterRedisDBName int

	var tenantMySQLDBs map[uint64]*gorm.DB
	var tenantMySQLDBNames map[uint64]string
	var tenantMongoDBs map[uint64]*mongo.Client
	var tenantMongoDBNames map[uint64]string
	var tenantRedisDBs map[uint64]*redis.Client
	var tenantRedisDBNames map[uint64]int

	isTenant := viper.GetBool("database.mysql.isTenant")
	if isTenant {
		masterMySQLDB, masterMySQLDBName = dbdrivers.InitMysqlMasterConn()
		tenantMySQLDBs, tenantMySQLDBNames = dbdrivers.InitMysqlTenantConns(masterMySQLDB)
		tenantMongoDBs, tenantMongoDBNames = dbdrivers.InitMongoTenantConns(masterMySQLDB)
		tenantRedisDBs, tenantRedisDBNames = dbdrivers.InitRedisTenantConns(masterMySQLDB)

	} else {
		masterMySQLDB, masterMySQLDBName = dbdrivers.InitMysqlMasterConn()
		masterMongoDB, masterMongoDBName = dbdrivers.InitMongoMasterConn()
		masterRedisDB, masterRedisDBName = dbdrivers.InitRedisMasterConn()
	}

	masterBadgerDB := dbdrivers.InitBadgerConn(e)

	global.DBConn = &global.DBDeps{
		MasterMySQLDB:      masterMySQLDB,
		MasterMySQLDBName:  masterMySQLDBName,
		TenantMySQLDBs:     tenantMySQLDBs,
		TenantMySQLDBNames: tenantMySQLDBNames,
		MasterMongoDB:      masterMongoDB,
		MasterMongoDBName:  masterMongoDBName,
		TenantMongoDBs:     tenantMongoDBs,
		TenantMongoDBNames: tenantMongoDBNames,
		MasterRedisDB:      masterRedisDB,
		MasterRedisDBName:  masterRedisDBName,
		TenantRedisDBs:     tenantRedisDBs,
		TenantRedisDBNames: tenantRedisDBNames,
		BadgerDB:           masterBadgerDB,
	}

	// Set custom request binder
	e.Binder = &binder.CustomBinder{}

	return e
}

// `prometheusUrlSkipper` ignores metrics route on some endpoints.
func prometheusUrlSkipper(c echo.Context) bool {

	skipEndpoints := viper.GetStringSlice("prometheus.skipEndpoints")
	_, matches := str.MatchAllSubstringsInAString(c.Path(), skipEndpoints...)

	return matches > 0
}
