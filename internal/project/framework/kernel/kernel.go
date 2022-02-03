/**#bean*/ /*#bean.replace({{ .Copyright }})**/

// IMPORTANT: PLEASE DO NOT UPDATE THIS FILE.
package kernel

import (
	"html/template"
	"os"
	"path/filepath"

	/**#bean*/
	"demo/framework/internals/binder"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/binder")**/
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
	"demo/framework/internals/echoview"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/echoview")**/
	/**#bean*/
	"demo/framework/internals/goview"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/goview")**/

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

	// Set custom request binder
	e.Binder = &binder.CustomBinder{}

	// Setup HTML view templating engine.
	viewsTemplateCache := viper.GetBool("html.viewsTemplateCache")
	e.Renderer = echoview.New(goview.Config{
		Root:         "views",
		Extension:    ".html",
		Master:       "templates/master",
		Partials:     []string{},
		Funcs:        make(template.FuncMap),
		DisableCache: !viewsTemplateCache,
		Delims:       goview.Delims{Left: "{{`{{`}}", Right: "{{`}}`}}"},
	})

	// IMPORTANT: Configure debug log.
	debugLogPath := viper.GetString("debugLog")
	if debugLogPath != "" {
		if file, err := openFile(debugLogPath); err != nil {
			e.Logger.Fatalf("Unable to open log file: %v Server 🚀  crash landed. Exiting...\n", err)
		} else {
			e.Logger.SetOutput(file)
		}
	}
	e.Logger.SetLevel(log.DEBUG)
	e.Logger.Info("ENVIRONMENT: ", viper.GetString("environment"))

	// IMPORTANT: Configure access log and body dumper. (can be turn off)
	if viper.GetBool("accessLog.on") {
		accessLogConfig := imiddleware.LoggerConfig{BodyDump: viper.GetBool("accessLog.bodyDump")}
		accessLogPath := viper.GetString("accessLog.path")
		if accessLogPath != "" {
			if file, err := openFile(debugLogPath); err != nil {
				e.Logger.Fatalf("Unable to open log file: %v Server 🚀  crash landed. Exiting...\n", err)
			} else {
				accessLogConfig.Output = file
			}
		}
		accessLogger := imiddleware.AccessLoggerWithConfig(accessLogConfig)
		e.Use(accessLogger)
	}

	// Some pre-build middleware initialization.
	e.Pre(imiddleware.Tracer())
	e.Pre(echomiddleware.RemoveTrailingSlash())
	if viper.GetBool("http.isHttpsRedirect") {
		e.Pre(echomiddleware.HTTPSRedirect())
	}
	e.Use(echomiddleware.Recover())

	// IMPORTANT: Request related middleware.
	// Time out middleware.
	e.Use(imiddleware.RequestTimeout(viper.GetDuration("http.timeout")))

	// Set the `X-Request-ID` header field if it doesn't exist.
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

	// IMPORTANT: Capturing error and send to sentry if needed.
	// Sentry `panic` error handler and APM initialization if activated from `env.json`
	if viper.GetBool("sentry.on") {
		// To initialize Sentry's handler, we need to initialize sentry first.
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              viper.GetString("sentry.dsn"),
			Debug:            true,
			AttachStacktrace: viper.GetBool("sentry.attachStacktrace"),
			TracesSampleRate: helpers.FloatInRange(viper.GetFloat64("sentry.tracesSampleRate"), 0.0, 1.0),
		}); err != nil {
			e.Logger.Fatal("Sentry initialization failed: ", err, ". Server 🚀  crash landed. Exiting...")
		}
		// Once it's done, let's attach the handler as one of our middleware.
		e.Use(sentryecho.New(sentryecho.Options{
			Repanic: true,
			Timeout: viper.GetDuration("sentry.timeout"),
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
