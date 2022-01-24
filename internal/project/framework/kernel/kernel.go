/**#bean*/ /*#bean.replace({{ .Copyright }})**/

// IMPORTANT: PLEASE DO NOT UPDATE THIS FILE.
package kernel

import (
	"os"
	"path/filepath"
	"time"

	/**#bean*/
	"demo/framework/internals/binder"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/binder")**/
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
	"github.com/google/uuid"
	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/spf13/viper"
)

func NewEcho() *echo.Echo {

	e := echo.New()

	// Hide default `Echo` banner during startup.
	e.HideBanner = true

	// Setup basic echo view template.
	e.Renderer = template.New(e)

	// Set custom request binder
	e.Binder = &binder.CustomBinder{}

	// Get log type (file or stdout) settings from config.
	debugLogLocation := viper.GetString("debugLog")
	requestLogLocation := viper.GetString("requestLog")
	bodydumpLogLocation := viper.GetString("bodydumpLog")

	// IMPORTANT: Different types of Loggers
	// Set debug log output location.
	if debugLogLocation != "" {
		file, err := openFile(debugLogLocation)
		if err != nil {
			e.Logger.Fatalf("Unable to open log file: %v Server 🚀  crash landed. Exiting...\n", err)
		}
		e.Logger.SetOutput(file)
	}
	e.Logger.SetLevel(log.DEBUG)
	e.Logger.Info("ENVIRONMENT: ", viper.GetString("environment"))

	// Set request log output location. (request logger is using the same echo logger but with different config)
	requestLoggerConfig := echomiddleware.LoggerConfig{}
	if requestLogLocation != "" {
		file, err := openFile(requestLogLocation)
		if err != nil {
			e.Logger.Fatalf("Unable to open log file: %v Server 🚀  crash landed. Exiting...\n", err)
		}
		requestLoggerConfig.Output = file
	}
	requestLogger := echomiddleware.LoggerWithConfig(requestLoggerConfig)
	e.Use(requestLogger)

	// Set bodydump log output location. (bodydumper using a custom logger to aovid overwriting the setting of the default logger)
	bodydumpLogger := log.New("bodydump")
	if bodydumpLogLocation != "" {
		file, err := openFile(bodydumpLogLocation)
		if err != nil {
			if err != nil {
				e.Logger.Fatalf("Unable to open log file: %v Server 🚀  crash landed. Exiting...\n", err)
			}
		}
		bodydumpLogger.SetOutput(file)
	}
	bodydumper := echomiddleware.BodyDumpWithConfig(echomiddleware.BodyDumpConfig{
		Handler: imiddleware.BodyDumpWithCustomLogger(bodydumpLogger),
	})
	e.Use(bodydumper)

	// Some pre-build middleware initialization.
	e.Pre(echomiddleware.RemoveTrailingSlash())
	if viper.GetBool("http.isHttpsRedirect") {
		e.Pre(echomiddleware.HTTPSRedirect())
	}
	e.Use(echomiddleware.Recover())

	// IMPORTANT: Request related middleware.
	// Time out middleware.
	e.Use(imiddleware.RequestTimeout(viper.GetDuration("http.timeout") * time.Second))

	// Attach an random uuid id to every request.
	e.Use(echomiddleware.RequestIDWithConfig(echomiddleware.RequestIDConfig{
		Generator: uuid.NewString,
	}))

	// Adds a `Server` header to the response.
	e.Use(imiddleware.ServerHeader(viper.GetString("name"), viper.GetString("version")))

	// Sets the maximum allowed size for a request body, return `413 - Request Entity Too Large` if the size exceeds the limit.
	e.Use(echomiddleware.BodyLimit(viper.GetString("http.bodyLimit")))

	// CORS initialization and support only HTTP methods which are configured under `http.allowedMethod` parameters in `env.json`.
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: viper.GetStringSlice("http.allowedMethod"),
	}))

	// Basic HTTP headers security like XSS protection...
	e.Use(echomiddleware.SecureWithConfig(echomiddleware.SecureConfig{
		XSSProtection:         viper.GetString("security.http.header.xssProtection"),         // Adds the X-XSS-Protection header with the value `1; mode=block`.
		ContentTypeNosniff:    viper.GetString("security.http.header.contentTypeNosniff"),    // Adds the X-Content-Type-Options header with the value `nosniff`.
		XFrameOptions:         viper.GetString("security.http.header.xFrameOptions"),         // The X-Frame-Options header value to be set with a custom value.
		HSTSMaxAge:            viper.GetInt("security.http.header.hstsMaxAge"),               // HSTS header is only included when the connection is HTTPS.
		ContentSecurityPolicy: viper.GetString("security.http.header.contentSecurityPolicy"), // Allows the Content-Security-Policy header value to be set with a custom value.
	}))

	// Return `405 Method Not Allowed` if a wrong HTTP method been called for an API route.
	// Return `404 Not Found` if a wrong API route been called.
	e.Use(imiddleware.MethodNotAllowedAndRouteNotFound())

	// IMPORTANT: Capturing error and send to sentry if needed.
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

	// Enable prometheus metrics middleware. Metrics data should be accessed via `/metrics` endpoint.
	// This will help us to integrate `bean's` health into `k8s`.
	isPrometheusMetrics := viper.GetBool("prometheus.isPrometheusMetrics")
	if isPrometheusMetrics {
		p := prometheus.NewPrometheus("echo", prometheusUrlSkipper)
		p.Use(e)
	}

	return e
}

// `prometheusUrlSkipper` ignores metrics route on some endpoints.
func prometheusUrlSkipper(c echo.Context) bool {

	skipEndpoints := viper.GetStringSlice("prometheus.skipEndpoints")
	_, matches := str.MatchAllSubstringsInAString(c.Path(), skipEndpoints...)

	return matches > 0
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
