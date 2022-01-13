/**#bean*/ /*#bean.replace({{ .Copyright }})**/

// IMPORTANT: PLEASE DO NOT UPDATE THIS FILE.
package bootstrap

import (
	"os"
	"path/filepath"
	"time"

	/**#bean*/
	"demo/framework/internals/binder"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/binder")**/
	/**#bean*/
	"demo/framework/internals/global"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/global")**/
	/**#bean*/
	"demo/framework/internals/helpers"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/helpers")**/
	/**#bean*/
	imiddleware "demo/framework/internals/middleware"
	/*#bean.replace(imiddleware "{{ .PkgPath }}/framework/internals/middleware")**/
	/**#bean*/
	str "demo/framework/internals/string"
	/*#bean.replace(str "{{ .PkgPath }}/framework/internals/string")**/
	/**#bean*/
	"demo/framework/internals/template"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/template")**/
	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/spf13/viper"
)

func New() *echo.Echo {

	// Parse bean system files and directories.
	helpers.ParseBeanSystemFilesAndDirectorires()

	e := echo.New()

	// Initialize the global echo instance. This is useful to print log from `internals` packages.
	global.EchoInstance = e

	// Hide default `Echo` banner during startup.
	e.HideBanner = true

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Configuration file error: %v Server 🚀  crash landed. Exiting...\n", err)
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
				log.Fatalf("Unable to create log file: %v Server 🚀  crash landed. Exiting...\n", err)
			}
		}

		logfp, err := os.OpenFile(logFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0664)
		if err != nil {
			log.Printf("Unable to open log file: %v Server 🚀  crash landed. Exiting...\n", err)
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

	// Sentry `panic` error handler and APM initialization if activated from `env.json`
	isSentry := viper.GetBool("sentry.isSentry")
	if isSentry {
		sentryDsn := viper.GetString("sentry.dsn")
		if isValidSentryDSN := str.IsValidUrl(sentryDsn); !isValidSentryDSN {
			e.Logger.Fatal("Sentry invalid DSN: ", sentryDsn, ". Server 🚀  crash landed. Exiting...")
		}

		sentryAttachStacktrace := viper.GetBool("sentry.attachStacktrace")
		sentryapmTracesSampleRate := viper.GetFloat64("sentry.apmTracesSampleRate")

		if sentryapmTracesSampleRate > 1.0 {
			sentryapmTracesSampleRate = 1.0
		} else if sentryapmTracesSampleRate < 0.0 {
			sentryapmTracesSampleRate = 0.0
		}

		// To initialize Sentry's handler, we need to initialize sentry first.
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              sentryDsn,
			AttachStacktrace: sentryAttachStacktrace,
			TracesSampleRate: sentryapmTracesSampleRate,
		}); err != nil {
			e.Logger.Fatal("Sentry initialization failed: ", err, ". Server 🚀  crash landed. Exiting...")
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
		XSSProtection:         viper.GetString("security.http.header.xssProtection"),         // Adds the X-XSS-Protection header with the value `1; mode=block`.
		ContentTypeNosniff:    viper.GetString("security.http.header.contentTypeNosniff"),    // Adds the X-Content-Type-Options header with the value `nosniff`.
		XFrameOptions:         viper.GetString("security.http.header.xFrameOptions"),         // The X-Frame-Options header value to be set with a custom value.
		HSTSMaxAge:            viper.GetInt("security.http.header.hstsMaxAge"),               // HSTS header is only included when the connection is HTTPS.
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
