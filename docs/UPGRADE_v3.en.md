# Bean v2 → v3 Upgrade Guide

English | [中文](UPGRADE_v3.md)

The core change in Bean v3 is upgrading the underlying web framework from **Echo v4** to **Echo v5**. Echo v5 itself has many breaking changes (`Context` changed from an interface to a struct, logging moved to `log/slog`, error types changed, etc.), so bean bumps its major version as well, and the module path becomes `github.com/retail-ai-inc/bean/v3`.

With this major version, bean also unifies logging, reworks 404 / 405 handling, and fixes a number of problems that already existed in v2 (including several security issues). This document records all of these changes in full, together with what a project based on bean v2 **must change** and **needs to check** when upgrading to v3.

**How to read this document**: start with the overview in section 1 and upgrade following the steps in section 3; sections 4 and 5 cover the code and configuration changes you must make; section 6 explains each behavior change, and every item is labeled "Change / Impact / What to do" — items without "What to do" need no action; finally, verify with the checklist in section 10.

- [1. Overview of changes](#1-overview-of-changes)
- [2. Requirements](#2-requirements)
- [3. Upgrade steps](#3-upgrade-steps)
- [4. Required code changes](#4-required-code-changes)
- [5. Configuration changes (env.json)](#5-configuration-changes-envjson)
- [6. Behavior changes](#6-behavior-changes)
- [7. bean CLI and project templates](#7-bean-cli-and-project-templates)
- [8. New features](#8-new-features)
- [9. Public API changes at a glance](#9-public-api-changes-at-a-glance)
- [10. Upgrade checklist](#10-upgrade-checklist)

---

## 1. Overview of changes

| Category | Main changes | What downstream projects need to do | Details |
| --- | --- | --- | --- |
| Dependencies and module path | `bean/v3`, `echo/v5`, `echo-contrib/v5`, sentry-go/echo v0.50+; Go 1.25+ | Update `go.mod` and run the bulk replacement | [2](#2-requirements), [3](#3-upgrade-steps) |
| Compile-time incompatibilities | `*echo.Context`, `HTTPError`, `bean.New()` returns an error, logging API, Binder / Renderer signatures, routing API, etc. | **Code changes required** | [4](#4-required-code-changes) |
| Configuration | `accessLog.bodyDumpMaskParam` renamed; `http` settings can be omitted but the `keepAlive` default changed; new `debugLogLevel` | **`env.json` changes required** (rename); check the defaults | [5](#5-configuration-changes-envjson) |
| Log format | The application log, Echo log and access log are unified into a single JSON format, and field names changed | Adjust parsing rules and alerts on your log platform | [6.2](#62-logging) |
| HTTP responses | 404 / 405 are now determined by the router and include `errorMsg`; JSON request body errors changed from 500 to 400 | Check clients that depend on the response format | [6.1](#61-http-responses-and-error-handling) |
| Security fixes | Template rendering leaking data across requests, AES not verifying the MAC, TLS hot reload not verifying the hostname, JWT default validation being disabled, credentials logged in plain text | Check certificates and JWT usage | [6.4](#64-security-fixes) |
| Databases and caches | Several Redis method fixes, read replica fallback, connections released when initialization fails, tenant config validation, Mongo timeouts | Check `MSetJSON` usage and cluster `MGet` return values | [6.5](#65-databases-and-caches) |
| Async tasks and outbound requests | Async tasks keep the request context and run in a pool; outbound logging no longer blocks streaming responses or leaks connections | Make sure outbound response bodies are closed | [6.6](#66-async-tasks), [6.7](#67-outbound-requests) |
| bean CLI | Several fixes to `gen spec`, `test`, `create *` | Exit code and report file name of `bean test` in CI | [7](#7-bean-cli-and-project-templates) |
| New features | `helpers.ProcessCSV`, `log.WithLevel`, etc. | Optional | [8](#8-new-features) |

## 2. Requirements

| Item | v2 | v3 |
| --- | --- | --- |
| Go | 1.24+ | **1.25+** (required by Echo v5); bean's own `go.mod` uses 1.26 |
| bean | `github.com/retail-ai-inc/bean/v2` | `github.com/retail-ai-inc/bean/v3` |
| Echo | `github.com/labstack/echo/v4` | `github.com/labstack/echo/v5` |
| echo-contrib | `github.com/labstack/echo-contrib` | `github.com/labstack/echo-contrib/v5` |
| sentry-go/echo | ≤ v0.44.x | **v0.50.0+** (based on Echo v5) |
| gommon | `github.com/labstack/gommon` (logging) | No longer a dependency |

## 3. Upgrade steps

**Step 1: Update dependencies**

```bash
go get github.com/retail-ai-inc/bean/v3@v3.0.0
```

```bash
go get github.com/labstack/echo/v5@latest
```

Then remove `github.com/retail-ai-inc/bean/v2` and `github.com/labstack/echo/v4` from `go.mod` and run:

```bash
go mod tidy
```

The bean CLI also needs to be reinstalled:

```bash
go install github.com/retail-ai-inc/bean/v3/cmd/bean@latest
```

**Step 2: Bulk replacement**

Run the following replacement in the project root (`perl` works on both macOS and Linux):

```bash
find . -type f -name '*.go' -not -path './vendor/*' -print0 | xargs -0 perl -pi -e 's#retail-ai-inc/bean/v2\b#retail-ai-inc/bean/v3#g; s#labstack/echo/v4#labstack/echo/v5#g; s#labstack/echo-contrib/#labstack/echo-contrib/v5/#g; s/(?<![\*\w.])echo\.Context\b/*echo.Context/g; s#\becho\.Map\b#map[string]any#g'
```

It performs the following:

| Before | After |
| --- | --- |
| `github.com/retail-ai-inc/bean/v2/...` | `github.com/retail-ai-inc/bean/v3/...` |
| `github.com/labstack/echo/v4` | `github.com/labstack/echo/v5` |
| `github.com/labstack/echo-contrib/xxx` | `github.com/labstack/echo-contrib/v5/xxx` |
| `func(c echo.Context) error` | `func(c *echo.Context) error` |
| `echo.Map{...}` | `map[string]any{...}` |

**Step 3: Fix compile errors**

Run `go build ./... && go vet ./...` and fix the remaining compile errors according to [section 4](#4-required-code-changes). Projects generated by `bean init` also need a few generated files changed as described in [4.12](#412-changes-needed-in-projects-generated-by-bean-init).

**Step 4: Update the configuration**

Update the `env.json` of every environment according to [section 5](#5-configuration-changes-envjson).

**Step 5: Test and verify**

Run `go test ./...`, then check the behavior changes one by one according to [section 6](#6-behavior-changes), and finally confirm with the checklist in [section 10](#10-upgrade-checklist).

## 4. Required code changes

Without the changes in this section, the project will not compile (or will fail at startup).

### 4.1 Handler / middleware signatures

`echo.Context` is now a struct, so all handlers, middleware and bean error handler functions must take a pointer:

```go
// v2
func (h *userHandler) Get(c echo.Context) error { ... }

// v3
func (h *userHandler) Get(c *echo.Context) error { ... }
```

The public bean functions that take a Context now take `*echo.Context` accordingly; their usage is unchanged:
`trace.StartSpanWithEcho`, `trace.SentryCaptureExceptionWithEcho`, `trace.SentryCaptureMessageWithEcho`, `async.ExecuteWithContext`, `helpers.DecodeJWT`, `helpers.ExtractJWTFromHeader`, `helpers.PostDataStripTags`, `echoview.Render`, as well as the `berror.ErrorHandlerFunc` type and `berror.APIErrorHandlerFunc`, `HTTPErrorHandlerFunc`, `ValidationErrorHandlerFunc`, `DefaultErrorHandlerFunc`.

### 4.2 Logging unified on slog

v2 had three loggers: bean's application log (gommon logger), the Echo log and the access log, each with a different format. v3 unifies them on the standard library `log/slog`:

- `log.Logger()`, `e.Logger` and `c.Logger()` are the **same** logger, and all logs (including the access log) use the same JSON format.
- When you use the `...Context` methods and pass the request context, the request ID (`id`) and Sentry trace ID are added automatically; there is no need to add them by hand.
- No longer depends on `github.com/labstack/gommon`.

**Recommended style (slog):**

```go
import "github.com/retail-ai-inc/bean/v3/log"

// v2
log.Logger().Debugf("user %d logged in", userID)
log.Logger().Errorj(elog.JSON{"message": "failed to get user", "error": err.Error(), "id": reqID})
c.Logger().Errorf("failed to get user: %v", err)

// v3
log.Logger().DebugContext(ctx, "user logged in", "user_id", userID)
log.Logger().ErrorContext(ctx, "failed to get user", "error", err) // id / trace added automatically
c.Logger().ErrorContext(c.Request().Context(), "failed to get user", "error", err)
```

**Compatibility layer (deprecated):** `log.Logger()` still provides the v2 `Print*`, `Debugf/Debugj`, `Infof/Infoj`, `Warnf/Warnj`, `Errorf/Errorj`, `Fatal*` and `Panic*` methods. They also write the new format underneath, so you can migrate gradually; they are scheduled for removal in v4. The parameter type of the `*j` methods is `map[string]any`, so passing an `elog.JSON` still compiles.

Calls that must be changed by hand:

| v2 | v3 |
| --- | --- |
| `log.Logger().Debug(a, b)` / `Info(...)` / `Warn(...)` / `Error(err)` | The first argument must be a string message, followed by key-value pairs, e.g. `Error("failed", "error", err)` |
| `log.Logger().SetLevel(elog.INFO)` / `Level()` | `SetLevel(slog.LevelInfo)` / `Level()`, or use the `debugLogLevel` setting |
| `log.Logger().SetOutput(w)` / `Output()` / `SetPrefix` / `SetHeader` | Removed; the output destination is determined by `debugLogPath`, or create a logger with `log.NewLogger(w, ...)` |
| `log.BeanLogger` (interface) | `*log.BeanLogger` (struct embedding `*slog.Logger`) |
| `log.Init(e.Logger)` (takes effect only once, panics on failure) | `log.Init(w io.Writer) (*log.BeanLogger, error)`; calling it again replaces the global logger |
| `log.NewLogger(e.Logger, opts...)` | `log.NewLogger(w io.Writer, opts...)`; new `log.WithLevel` |

Most projects do not call `log.Init` themselves (`bean.New()` initializes it internally), so this mainly affects test code:

```go
// v2
_ = log.Init(echo.New().Logger)

// v3: write to a buffer so you can assert on it
var buf bytes.Buffer
l, _ := log.NewLogger(&buf)
log.SetLogger(l)
```

For the log format changes, see [6.2](#62-logging).

### 4.3 `echo.HTTPError`

```go
// v2
return echo.NewHTTPError(http.StatusBadRequest, "invalid", detail)
return &echo.HTTPError{Code: 504, Message: "timeout", Internal: err}
he.Internal

// v3
return echo.NewHTTPError(http.StatusBadRequest, "invalid")           // message must be a string
return echo.NewHTTPError(504, "timeout").Wrap(err)                   // use Wrap to carry the internal error
he.Unwrap()                                                          // get the internal error
```

**Important**: predefined errors such as `echo.ErrNotFound`, `echo.ErrMethodNotAllowed` and `echo.ErrUnauthorized` are **no longer `*echo.HTTPError`** in v5; they only implement the `echo.HTTPStatusCoder` interface. If a custom error handler is written like this, 404/405 will be treated as 500:

```go
// does not match echo.ErrNotFound in v3
var he *echo.HTTPError
if errors.As(err, &he) { code = he.Code }

// correct in v3
code := echo.StatusCode(err) // returns 0 if it is not an HTTP error
// or
var sc echo.HTTPStatusCoder
if errors.As(err, &sc) { code = sc.StatusCode() }
```

bean's built-in `berror.HTTPErrorHandlerFunc` already handles errors this way.

### 4.4 Custom Echo global error handler

The **parameter order** of bean's `berror.ErrorHandlerFunc` (`func(err error, c *echo.Context) (bool, error)`) is **unchanged**.
But if you replaced `b.Echo.HTTPErrorHandler` directly, the parameter order of Echo's signature has been **swapped**:

```go
// v2
b.Echo.HTTPErrorHandler = func(err error, c echo.Context) { ... }

// v3
b.Echo.HTTPErrorHandler = func(c *echo.Context, err error) { ... }
```

Checking whether the response has been committed:

```go
// v2
if c.Response().Committed { return }

// v3
if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Committed { return }
```

### 4.5 `c.Response()` returns `http.ResponseWriter`

```go
// v2
c.Response().Writer
c.Response().Status
c.Response().Size
c.Response().Writer = myWriter

// v3
c.Response()                                   // is itself an http.ResponseWriter
r, _ := echo.UnwrapResponse(c.Response())      // when you need Status / Size / Committed
r.Status; r.Size
c.SetResponse(myWriter)                        // replace the writer
```

Custom ResponseWriter wrappers should implement `Unwrap() http.ResponseWriter` so that `echo.UnwrapResponse` and `http.ResponseController` can find the underlying writer.

### 4.6 Routing

```go
// v2
for _, r := range b.Echo.Routes() { ... }

// v3
for _, r := range b.Echo.Router().Routes() { ... }
```

The return value of registration methods such as `e.GET()` changed from `*echo.Route` to `echo.RouteInfo` (a value type). `e.Reverse()`, `e.URI()`, `e.URL()` and `e.Host()` have been removed; use `e.Router().Routes().Reverse(name, params...)` instead.

### 4.7 Custom Binder / Renderer / Validator

```go
// Binder: parameter order swapped
// v2
func (b *MyBinder) Bind(i interface{}, c echo.Context) error
// v3
func (b *MyBinder) Bind(c *echo.Context, i interface{}) error

// Renderer: Context moved to the first parameter
// v2
func (r *MyRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error
// v3
func (r *MyRenderer) Render(c *echo.Context, w io.Writer, name string, data interface{}) error
```

The Validator interface (`Validate(i interface{}) error`) is unchanged. bean's built-in `echoview.ViewEngine.Render` has also been changed to the new signature.

### 4.8 Echo middleware

Common changes (for the full list, see Echo's [API_CHANGES_V5.md](https://github.com/labstack/echo/blob/master/API_CHANGES_V5.md)):

| v4 | v5 |
| --- | --- |
| `middleware.Logger()` / `LoggerWithConfig` | Removed; use `middleware.RequestLogger()` or bean's built-in access log instead |
| `middleware.Timeout()` | Removed; use `middleware.ContextTimeout()` instead (built into bean, see `http.timeout`) |
| `middleware.BodyLimit("2M")` | `middleware.BodyLimit(2 * middleware.MB)` (argument is a number of bytes) |
| `middleware.DefaultSkipper(c echo.Context)` | `middleware.DefaultSkipper(c *echo.Context)` |
| `RequestIDHandler func(echo.Context, string)` | `func(*echo.Context, string)` |
| `TrailingSlashConfig` | Split into `AddTrailingSlashConfig` / `RemoveTrailingSlashConfig` |
| `Default*Config` variables such as `middleware.DefaultCORSConfig` | Removed; construct the Config directly |
| `*middleware.PanicStackError` (error produced by the Recover middleware) | bean uses its own recover middleware, and a panic becomes a `*berror.PanicError`; see [6.3](#63-monitoring-prometheus--sentry) |

### 4.9 Test code

```go
// v2
c := e.NewContext(req, rec)                              // returns echo.Context
c.SetResponse(echo.NewResponse(w, e))
c.SetParamNames("id"); c.SetParamValues("1")

// v3
c := e.NewContext(req, rec)                              // returns *echo.Context
c.SetResponse(echo.NewResponse(w, e.Logger))             // second argument is now a *slog.Logger
c.SetPathValues(echo.PathValues{{Name: "id", Value: "1"}})
```

### 4.10 Other common compile errors

| Compile error | Fix |
| --- | --- |
| `e.HideBanner` / `e.HidePort` / `e.Debug` undefined | Delete them (bean starts its own `http.Server` and does not print a banner) |
| `e.Start(...)` / `e.Shutdown(...)` | Handled by bean's `ServeAt`; projects usually do not need to call them |
| `c.ParamNames()` / `c.ParamValues()` | Use `c.PathValues()` instead |
| `echo.FormParam[T]` | Renamed to `echo.FormValue[T]` |
| `echo.PathParamsBinder(c)` | Renamed to `echo.PathValuesBinder(c)` |
| `echo.GetPath(r)` | Use `r.URL.RawPath` / `r.URL.Path` instead |
| `config.Bean.AccessLog.BodyDumpMaskParam` undefined | Change to `config.Bean.AccessLog.MaskFields`; see [5.1](#51-accesslogbodydumpmaskparam-renamed-to-accesslogmaskfields) |
| Anonymous struct types of `config.Bean.HTTP.ErrorMessage.E404` etc. | Merged into the `config.ErrorPage` type; field names unchanged |

### 4.11 `bean.New()` / `bean.NewEcho()` return an error

In v2, configuration errors (`http.trustIPRange`, `http.bodyLimit`, Sentry initialization, Prometheus, goroutine pools, etc.) made `New()` call `log.Fatal` internally and exit the process; this could not be tested and did not clean up already-initialized resources. v3 returns an error instead, and releases already-initialized resources (such as registered goroutine pools) on failure:

```go
// v2
b := bean.New()

// v3
b, err := bean.New()
if err != nil {
    log.Fatalf("Failed to initialize bean: %v", err)
}
```

```go
// v2
e, closeEcho := bean.NewEcho()

// v3
e, closeEcho, err := bean.NewEcho()
```

- Returns `bean.ErrConfigNotLoaded` when `config.Bean` has not been loaded.
- When a regular expression in `accessLog.skipEndpoints`, `prometheus.skipEndpoints` or `sentry.skipTracesEndpoints` is invalid, v2 panics, while v3 returns an error from `New()` that includes the name of the setting.
- Returns an error when `env.json` still contains a removed setting; see [5.1](#51-accesslogbodydumpmaskparam-renamed-to-accesslogmaskfields).

### 4.12 Changes needed in projects generated by `bean init`

In projects generated by the v2 `bean init`, the following files need to be changed:

| File | Change |
| --- | --- |
| `commands/start.go`, `commands/route.go`, `commands/gopher/gopher.go` | Handle the error from `bean.New()` as described in [4.11](#411-beannew--beannewecho-return-an-error) |
| `commands/route.go` | Change `b.Echo.Routes()` to `b.Echo.Router().Routes()`, otherwise `route list` and `bean gen spec` will not compile |
| `commands/root.go` | Recommended: load the configuration with `config.LoadConfig("env.json")` (as the v3 template does); it checks for removed settings and sets defaults |
| Files that reference `bean.BeanConfig`, `bean.Logger()`, `bean.SentryCaptureException`, `bean.DefaultBeforeSend`, `bean.Cleanup` | These did not exist even in v2 (projects generated from the v2 template did not compile in the first place); change them to `config.Bean`, `log.Logger()`, `trace.SentryCaptureException`, `trace.DefaultBeforeSend`; simply delete `bean.Cleanup` |
| Logging calls in handlers / middleware | Change to the `...Context` style as described in [4.2](#42-logging-unified-on-slog) |

You can also generate a new project with the v3 `bean init` to use as a reference.

## 5. Configuration changes (env.json)

### 5.1 `accessLog.bodyDumpMaskParam` renamed to `accessLog.maskFields`

- **Change**: the old name was misleading: `bodyDump` suggested that it only applies to request/response bodies, and only when `bodyDump` is enabled; `Param` suggested that it targets URL parameters. In fact it masks matching fields in all structured access logs (`ACCESS`, `DUMP`, `OUTBOUND_API`), including `request_header` / `response_header`, regardless of whether `bodyDump` is enabled. The new name `maskFields` matches `log.WithMaskFields`. The masking behavior itself is unchanged.
- **Impact**: if `bodyDumpMaskParam` still has a value, **startup fails** with the error `config: accessLog.bodyDumpMaskParam has been removed, rename it to accessLog.maskFields`, so that masking does not silently stop working. Both `config.LoadConfig` and `bean.New()` perform this check, so it applies even if the project reads the configuration with viper itself. An empty `"bodyDumpMaskParam": []` (the default generated by older project templates) is ignored, but it is recommended to delete or rename it.
- **What to do**: update the `env.json` of every environment; in Go code, change reads of `config.Bean.AccessLog.BodyDumpMaskParam` to `config.Bean.AccessLog.MaskFields`.

```diff
 "accessLog": {
-  "bodyDumpMaskParam": ["password", "token"],
+  "maskFields": ["password", "token"],
 }
```

### 5.2 `http` settings can be omitted, `keepAlive` default changed

- **Change**: in v2, startup panicked when `http.bodyLimit` was not set, and the server listened on a random port when `http.port` was not set. v3 provides defaults for the following settings (taken from the project template):

  | Setting | Default |
  | --- | --- |
  | `http.host` | `0.0.0.0` |
  | `http.port` | `8888` |
  | `http.bodyLimit` | `1M` |
  | `http.keepAlive` | `true` |

  The behavior when other `http` settings are omitted is unchanged (for example, no request timeout is set, and all methods are allowed when `allowedMethod` is empty).
- **Impact**: in v2, omitting `keepAlive` meant `false` (keep-alive disabled); in v3, omitting it means `true`.
- **What to do**: projects that do not set `http.keepAlive` and really want keep-alive disabled must explicitly set `"keepAlive": false`.

### 5.3 New `debugLogLevel`

- **Change**: new setting `debugLogLevel`: the minimum level of the application log, `debug` (default) / `info` / `warn` / `error`; it does not affect the access log. The application log destination is still determined by `debugLogPath`.
- **What to do**: optional. If production does not need debug logs, set it to `info`.

### 5.4 Changes to other settings

| Setting | Change |
| --- | --- |
| `http.bodyLimit` | Format unchanged: you can still write `"1M"`, `"512K"`, etc., and bean converts it to a number of bytes (same rules as v2: `1M` = 1,000,000 bytes, `1Mi` = 1,048,576 bytes). Startup fails if the value cannot be parsed. |
| `http.errorMessage.e405.html.file` | Now takes effect (in v2, 405 always used `errors/html/405`). |
| `http.errorMessage.*.json` | `value` is always output as a string. The configuration is now parsed once at startup instead of querying viper on every error. Custom JSON for `e404` / `e405` now also applies to 404 / 405 produced by the router; see [6.1.1](#611-404--405-now-determined-by-the-echo-router). |
| `accessLog.skipEndpoints` / `prometheus.skipEndpoints` / `sentry.skipTracesEndpoints` | An invalid regular expression makes startup return an error (v2 panicked). `sentry.skipTracesEndpoints` also takes effect when only one entry is configured (in v2, sampling was skipped only with two or more entries, because the condition was written as `> 1`). |
| `http.keepAlive` / `http.shutdownTimeout` / `http.ssl.*` | Meaning unchanged; see [6.8](#68-server-startup-shutdown-and-routing). |
| `database.*.ssl.hotReload` | When enabled, the certificate hostname is now verified; see [6.4.3](#643-database-tls-hotreload-verifies-the-server-hostname). |
| `database.mongo.connectTimeout` | Now also used for the connection and ping at startup; see [6.5.5](#655-mongodb). |

## 6. Behavior changes

The following changes are handled inside bean, and most projects do not need code changes; please check each item marked "What to do".

### 6.1 HTTP responses and error handling

#### 6.1.1 404 / 405 now determined by the Echo router

v2 used a global middleware that, on every request, iterated over all routes and matched the path itself to decide 404 / 405. v3 removes this middleware; the Echo router decides directly (returning `echo.ErrNotFound` / `echo.ErrMethodNotAllowed`), and the error is then rendered by bean's error handler.

The status code rules are the same as in v2:

| Request | Result |
| --- | --- |
| Path does not exist | 404 |
| Path exists, but the method is not registered | 405 (since v3 also with an `Allow` header) |
| The route's method is not in `http.allowedMethod` | The route is **not registered** (a WARN log is printed at startup), and requests return 404 / 405 according to the two rows above |
| `e.Any(...)` / `g.Any(...)` | Registered only for the methods in `http.allowedMethod`; other methods return 405; `route list` lists them one per method |
| Path exists under a Group, wrong method | 405 (by default Echo v5 returns 404 because of the Group's catch-all route; bean disables this behavior) |

Changes to note:

1. **JSON response body**: an `errorMsg` field has been added, e.g. a 404 is `{"errorCode":"100003","errorMsg":"Not Found","errors":null}` (v2: `{"errorCode":"100003","errors":null}`). Custom JSON in `http.errorMessage.e404` / `e405` of `env.json` **now also applies to 404 / 405 produced by the router** (it did not in v2).
2. **HTML response**: the `errors/html/404` and `errors/html/405` templates are likewise used (or the file specified by `http.errorMessage.e404.html.file`); the `stacktrace` variable in the template is `Not Found` / `Method Not Allowed` (empty in v2).
3. **Even if `berror.HTTPErrorHandlerFunc` is not registered**, 404 / 405 produced by the router are returned in the format above and do not become 500. If the project registers its own global error handler function, it now also receives these errors; use `berror.IsRouterError(c)` to tell them apart.
4. **404 / 405 requests go through the global middleware**: in v2 these requests returned directly at the middleware and never reached the access log, Request ID, Sentry, Prometheus and other middleware; in v3 they go through normally, so 404 / 405 requests appear in the access log. In Prometheus, all 404s produced by the router share a single `url` label, so that scanning requests do not blow up the number of metrics. These requests are no longer logged as ERROR.
5. **Group middleware does not run for unknown paths**: for example, an authentication middleware on the `/api` Group does not run for `/api/unknown`; a 404 is returned directly, the same as in v2.
6. **`OPTIONS` requests**: an `OPTIONS` request that is not a CORS preflight returns `204` with an `Allow` header when the path exists (v2 returned 405). CORS preflight requests are still handled by the CORS middleware.

**What to do**:

- Check the startup log for the warning `route is not registered because its method is not listed in http.allowedMethod`; it means a route uses a method that is not allowed and was not registered.
- If any client depends on the JSON response format of 404 / 405, make sure the new `errorMsg` field does not affect it.

#### 6.1.2 JSON request body errors return 400

- **Change**: in v2, when the request body JSON had a syntax error, contained unknown fields or had a type mismatch, `c.Bind` returned a plain error, the client received a **500**, and the error was reported to Sentry. v3 returns a 400 `*berror.APIError` with the response `{"errorCode":"100001","errorMsg":"invalid JSON request body: ..."}` (`PROBLEM_PARSING_JSON`), and does not report it to Sentry.
- **Wrongful rejections also fixed**: fields of embedded structs, fields without a json tag, and arbitrary fields when binding to a map were all rejected as unknown fields in v2 (`UnknownFieldsError`); v3 accepts them. The case of field names must still match the json tag exactly.
- **New rejections**: extra data after the JSON value (e.g. `{"a":1} garbage`) is now rejected.
- **Other**: the request body is parsed only once (v2 parsed it three times), and the body can still be read again after `Bind`.
- **What to do**: if any client depends on "a 500 is returned when the request body JSON is invalid", change it to handle 400 and `errorCode` `100001`; if code checked errors by the string `"UnknownFieldsError"`, change it to check for `*berror.APIError` with `errors.As`. `berror.APIErrorHandlerFunc` must be registered in the error handlers (bean registers it by default).

#### 6.1.3 Timeouts return 504

When `http.timeout` expires, `echo.NewHTTPError(504, "gateway timeout").Wrap(context.DeadlineExceeded)` is returned; the internal error is still what gets reported to Sentry.

#### 6.1.4 Changes to the bean error handlers

- `berror.APIError` (status code taken from `HTTPStatusCode`) and validation errors (400) implement `echo.HTTPStatusCoder`. Echo middleware such as the Prometheus metrics records their real status codes; in v2 they were always recorded as 500.
- `berror.HTTPErrorHandlerFunc` no longer handles `APIError` and validation errors (they have their own handler functions), regardless of registration order.
- `http.errorMessage.*` is now read from `config.Bean` (parsed once at startup); see [5.4](#54-changes-to-other-settings).
- Five identical anonymous structs in the configuration structure have been merged into the `config.ErrorPage` type; field names are unchanged.

### 6.2 Logging

#### 6.2.1 Log format

The application log, Echo log and access log use the same JSON format (for how to write the code, see [4.2](#42-logging-unified-on-slog)):

| | v2 | v3 |
| --- | --- | --- |
| Application log | `{"time":…,"level":"INFO","prefix":"echo","file":"x.go","line":"12","message":…}` | `{"timestamp":…,"severity":"INFO","message":…,"file":"pkg/x.go","line":12,…}` |
| Echo / `c.Logger()` log | `{"time":…,"level":"INFO","message":…}` | Same as above |
| Access log | `{"timestamp":…,"severity":"INFO","level":"ACCESS",…}` | Unchanged |

- `severity` values: `DEBUG`, `INFO`, `WARNING` (slog's Warn), `ERROR`, `CRITICAL` (the compatibility layer's `Fatal*` / `Panic*`).
- The `level` field no longer appears in the application log; in the access log, `level` is still the category `ACCESS` / `DUMP` / `OUTBOUND_API`, etc.
- `line` changed from a string to a number, and `file` includes the directory (`pkg/x.go`).
- The request ID (`id`) and trace ID are added automatically when you use the `...Context` methods.
- `log.Shutdown` only stops the asynchronous writing of the access log; the application log can still be written afterwards.

**What to do**: if the parsing rules, indexes or alert conditions of your log platform depend on field names such as `time` / `level` / `prefix`, adjust them accordingly.

#### 6.2.2 Access log (body dump) memory is bounded

- **Change**: v2 read the entire request and response bodies into memory before truncating them, so memory usage for large uploads/downloads equaled the body size. v3 keeps only the first `accessLog.bodyLimit + 1` bytes (8KB by default) in memory, and streams the rest directly to the handler / client. The log fields and content are unchanged (truncation was already done according to `accessLog.bodyLimit`).
- **Impact**: when a client disconnects in the middle of an upload, v2 passed the part read so far to the handler as if it were the complete request body; in v3 the handler receives a read error. `request_body_error` / `bytes_in_read` are logged as usual.

### 6.3 Monitoring (Prometheus / Sentry)

- **Requests that panic are counted in Prometheus metrics**: in v2 the `Recover` middleware was registered outside the Prometheus middleware, so a panic passed straight through the Prometheus middleware; these requests were completely missing from the metrics and the 5xx error rate was understated. v3 moves `Recover` to the innermost of bean's built-in middleware, so the panic is first converted to an error and is counted with `code="500"` in metrics such as `echo_requests_total`.
- **`APIError` / validation errors are counted with their real status codes** (always 500 in v2); see [6.1.4](#614-changes-to-the-bean-error-handlers).
- **404s share a single `url` label**; see [6.1.1](#611-404--405-now-determined-by-the-echo-router).
- **Panics reported to Sentry show the stack where the panic occurred**: in v2 a panic was converted to an error carrying only a text stack trace, which Sentry could not parse, so Sentry showed the stack at the moment of reporting (in the error handler). v3 uses bean's own recover middleware, and a panic is converted to a `*berror.PanicError`:
  - The `Value` field holds the panic value; if it is an error (such as a `runtime.Error`), it can be retrieved with `errors.As` / `errors.Is`.
  - `StackTrace()` provides the stack at the point of the panic, and the innermost frame in Sentry is the function that panicked.
  - `Error()` returns only `[PANIC RECOVER] <panic value>` and no longer includes the full goroutine stack, so the `error` field of the access log is shorter; when you need the stack locally, format it with `%+v` (this is how the error log is written when Sentry is not enabled).
- **`sentry.skipTracesEndpoints` also takes effect when only one entry is configured**; see [5.4](#54-changes-to-other-settings).
- **What to do**: if the project used `errors.As` to check for Echo's `*middleware.PanicStackError`, change it to `*berror.PanicError`. The alert thresholds for the 5xx error rate may need to be re-evaluated (requests that panic are now counted).

### 6.4 Security fixes

The following problems already existed in v2 and are fixed in v3.

#### 6.4.1 HTML template rendering (`goview` / `echoview`) is concurrency-safe

- **Change**: the v2 template engine shared mutable state between requests. During concurrent rendering, `{{include "..."}}` could render with **another request's data** (cross-user data leakage); the layout specified by `data["template"]` was written into the shared configuration and affected subsequent requests; rendering the same template with a different layout failed with `"templates/xxx" is undefined`. In v3 the layout is per request, the cache is separated by layout, and each render uses its own copy of the template; the way templates are written is unchanged.
- **Other**: when rendering fails, the status code and a half-rendered page are no longer written; `goview.M` can be passed as data; `echoview.Render` uses the given status code in middleware mode (v2 always used 200).
- **What to do**: if any page relied on the bug where "the layout set by the previous request carries over to the next request", pass `"template"` explicitly in the data.

#### 6.4.2 `aes.BeanAESDecrypt` verifies the MAC

- **Change**: v2 did not verify the `mac` in the ciphertext when decrypting, so tampered ciphertext could also be "successfully" decrypted (creating the conditions for a padding oracle attack). v3 verifies the MAC with a constant-time comparison before decrypting, and returns `aes.ErrInvalidMAC` if it does not match.
- **Compatibility**: the ciphertext format is unchanged; existing ciphertext produced by `BeanAESEncrypt` (and by Laravel's `Encrypter`), such as tenant database passwords, decrypts as before; only ciphertext with an incorrect `mac` is rejected.
- **Other**: malformed input returns an error instead of panicking, and the empty string can also be encrypted.
- **What to do**: usually nothing. If code used `recover` to catch panics from decryption, it can check the error directly instead.

#### 6.4.3 Database TLS `hotReload` verifies the server hostname

- **Change**: with `ssl.hotReload` enabled, v2 did not verify the certificate hostname (never for MySQL; for Redis / Mongo when connecting by IP address), so any certificate issued by a trusted CA was accepted. v3 requires the certificate to be valid for the hostname used for the connection; when connecting by IP address, the certificate must be valid for one of the configured `host` values (an IP matches the certificate's IP SAN).
- **Impact**: if the database certificate does not match the configured `host`, **connections fail** once `hotReload` is enabled. For Redis cluster nodes discovered at runtime, accessed by IP and not in the configured `host`, the certificate must be valid for one of the configured `host` values.
- **What to do**: in environments with `ssl.hotReload` enabled, make sure the certificates are valid for the configured `host`, and fix the certificates or `host` if necessary.

#### 6.4.4 Default validation in `helpers.DecodeJWT` always applies

- **Change**: in v2, passing any `opts` (e.g. `jwt.WithAudience`) disabled all of the defaults — "only HS256 allowed", "`exp` required" and "strict decoding" — so a token without `exp` was valid forever. In v3 the default options always apply, and `opts` are appended after them (you can still override the allowed algorithms with `jwt.WithValidMethods`).
- **`helpers.ExtractJWTFromHeader`**: requires a space after `Bearer` (in v2, `Bearerxabc` would also yield `abc`); the scheme is case-insensitive.
- **Impact**: for calls that pass `opts`, tokens without `exp` or signed with another algorithm return `ErrJWTokenInvalid`; requests with a malformed `Authorization` header yield no token.
- **What to do**: where `DecodeJWT` is called with `opts`, make sure all tokens carry `exp` and are signed with HS256 (or pass `jwt.WithValidMethods` explicitly); if you need to accept tokens without `exp`, use `jwt.ParseWithClaims` directly.

#### 6.4.5 Credentials are no longer logged in plain text

- **Change**: in the outbound request log (`OUTBOUND_API`), the values of `Authorization`, `Proxy-Authorization`, `Cookie`, `Set-Cookie` and of request and response headers whose names contain token / secret / password / api-key / apikey / session are replaced with `[REDACTED]` in the log, even if they are in the list of headers allowed to be logged.
- **Other**: the `delKeyAPI` endpoint of the memory database now compares the bearer token in constant time.

### 6.5 Databases and caches

#### 6.5.1 Redis method fixes

| Method | Problem in v2 | v3 |
| --- | --- | --- |
| `RPush` | Failed on every call (`can't marshal *[]string`) | Works; an empty list is a no-op |
| `MSetJSON` in `store/redis` | Did not JSON-encode: passing a struct failed, and the written values could not be read by `GetJSON` / `MGetJSON` | Same as `SetJSON`: each value is passed through `json.Marshal` |
| `MGetJSON` in `store/redis` | Built the JSON array by string concatenation, O(n²) | Uses a buffer |
| Cluster mode `MGet` | Returned nil for the whole batch if any key was missing; missing keys returned `""` | Returns per key, with nil for missing keys (same as standalone mode) |
| Cluster mode `Keys` | Ran on only one random node | Runs on all masters |
| Cluster mode multi-key `DelKey` | CROSSSLOT error when the keys were not in the same slot | Deletes the keys one by one with a pipeline |
| Cluster mode `MSet` / `MSetWithTTL` | Panicked when a key was not a string | Returns `ErrRedisInvalidParameter` |

**What to do**:

- If, to work around the v2 problem, the project passed pre-serialized JSON strings (or `[]byte`) to `MSetJSON`, v3 encodes them again (a string becomes a quoted JSON string, `[]byte` becomes base64), and the data later read by `GetJSON` / `MGetJSON` will be wrong; change it to pass the original objects (struct, map, etc.). Data already in the cache is not affected.
- In cluster mode, code that checks whether the result of `MGet` is `nil` to detect a "miss" must be changed to check each element individually.

#### 6.5.2 Redis read replicas

- **Change**: a miss on a read replica (`redis.Nil`) no longer re-queries the primary (in v2 every miss caused an extra query to the primary); server errors such as `WRONGTYPE` are returned directly instead of being masked by the primary's result; only connection errors (and a replica returning `LOADING` / `MASTERDOWN`) fall back to the primary, and that replica is then not used for 10 seconds (in v2, when a replica was down, every read first had to wait for the dial timeout).
- **Other**: `HGets` no longer masks network errors of other commands because some field does not exist; `SRandMemberN` also falls back to the primary when the replica fails.

#### 6.5.3 Connections released when initialization fails

When initialization fails partway (a tenant connection fails, a Redis read replica connection fails, a ping fails, etc.), the connections already established are closed before the error is returned (in v2 they leaked). When `InitDB` fails partway, the databases initialized before that point are also closed.

#### 6.5.4 Validation of tenant connection settings

- **Change**: when a field in `TenantConnections.Connections` has the wrong type or is missing, startup returns an error that includes the tenant ID instead of panicking; numbers such as `"port": 3306` are also accepted. Empty strings are still allowed (same as v2).
- **What to do**: if a tenant's configuration is wrong, v3 reports a clear error at startup; fix that tenant's configuration according to the error message.

#### 6.5.5 MongoDB

- The initial connection and ping use `mongo.connectTimeout` (10s when not set; v2 always used 10s, so if a smaller value is configured, startup will time out earlier).
- When `host` contains multiple hosts (`a,b`), each host uses the configured port (in v2 only the last one did).
- Sessions are ended properly on close (in v2 the context used at close time had already been canceled).
- `maxConnectionLifeTime` still maps to the driver's max idle time (the mongo driver has no option for a maximum connection lifetime).
- **What to do**: in environments where `mongo.connectTimeout` is set to a small value, make sure the connection at startup can complete within that time.

#### 6.5.6 TLS certificate file watching

When `ssl.hotReload` is enabled, all connections using the same certificate file share a single file watcher, which stops when the connections are closed (in v2 each connection started its own watcher, which never stopped).

#### 6.5.7 Memory cache

`CloseMemory` can be called more than once (in v2 the second call blocked forever), and calling `memory.NewMemoryCache()` after closing returns a new cache; wildcard deletion (`DelMemory("prefix*")`) now runs in linear time (v2 used exponential backtracking matching).

### 6.6 Async tasks

- **`async.ExecuteWithContext` / `ExecuteWithTimeout` keep the request context**: in v2 the task context of these two functions lost the request ID (async task logs had no `id`) and the request's Sentry trace, and `ExecuteWithTimeout` also made the task share the same Sentry hub with the request (user / tags set in the task affected the request). In v3 they behave the same as `ExecuteContext`: the context is not canceled with the request, and it carries the request ID, a cloned hub and the continued trace.
- **Run in a goroutine pool**: when no pool name is given, these two functions run in the default pool (v2 started a goroutine directly), so graceful shutdown waits for them (up to `asyncPoolReleaseTimeout`); tasks submitted after shutdown has started are rejected and an error is logged.
- **Error log**: an error returned by a task in `ExecuteWithTimeout` is logged (`Async task failed.`) in addition to being reported to Sentry.
- **Releasing goroutine pools**: releasing a pool no longer blocks tasks in that pool from looking up pools; the default pool is released after the server shuts down, and is restored automatically when a bean instance is created again in the same process.
- **What to do**: for services with long-running async tasks, make sure the graceful shutdown time (`asyncPoolReleaseTimeout`) is long enough; watch for the new `Async task failed.` error log.

### 6.7 Outbound requests

- **Connection leak**: with `DumpBody`, closing the response body in the logging Transport of `transport/http` did not close the underlying connection (a connection leak); v3 fixes this. 101 responses (WebSocket upgrades) are no longer wrapped.
- **Request body**: only `BodyLimit` bytes are read for logging (v2 read the entire request body); if reading fails, the request fails instead of sending a truncated body.
- **Streaming responses**: with `DumpBody` enabled, the `OUTBOUND_API` log is now written when the response body is read to the end, a read error occurs, or the body is closed (v2 read up to `BodyLimit` bytes before returning the response, which blocked streaming responses such as SSE / long polling); a `response_body_error` field is added when reading fails.
- **Credential redaction**: see [6.4.5](#645-credentials-are-no-longer-logged-in-plain-text).
- **Trace propagation**: `trace.PropagateToGRPC` no longer overwrites outgoing metadata already in the context (such as authorization); repeated calls to `PropagateToHTTP` do not produce duplicate headers, and it does not panic when the header is nil.
- **Other**: `NewLoggingTransport` no longer panics when the bean configuration has not been loaded.
- **What to do**: wherever the logging Transport is used with `DumpBody` enabled, make sure response bodies are always read to the end or closed; otherwise no `OUTBOUND_API` log is written (and not closing the response body leaks the connection anyway).

### 6.8 Server startup, shutdown and routing

- **Server startup and shutdown**: `ServeAt` now uses Echo v5's `echo.StartConfig` to start and gracefully shut down; the meaning of the settings (`http.keepAlive`, `http.shutdownTimeout`, `http.ssl.*`) is unchanged; no read timeout is set (same as v2); HTTPS still supports HTTP/2, and certificate paths can be absolute. The internal error log of `http.Server` is now written through Echo's slog. When the `BeforeServe` callback runs, the server's shutdown function has already been registered, so `b.Shutdown()` can be called safely in or after it; handling of exit signals is registered before the server starts.
- **Route names keep the v4 behavior**: Echo v5's default route name is `GET:/path`, but bean, through a custom Router, still names routes after the handler function (e.g. `myapp/handlers.(*userHandler).Get-fm`), so the output of `route list` and `gen spec` is the same as in v2. Routes with an explicit `Name` are not affected.

### 6.9 Utility functions

- `str.Substring` computes lengths in characters (runes) (v2 used byte length, which returned extra characters or even panicked with Chinese text).
- `str.LeftPadToLength` / `RightPadToLength` no longer panic when `padStr` is empty, the length is negative, or the padding is insufficient; results for normal input are unchanged.
- `helpers.JitterBackoff` no longer panics when `min` is less than 2ns.
- `helpers.StructStripTags` no longer loses precision for large integers (above 2^53).

## 7. bean CLI and project templates

| Command | Problem in v2 | v3 |
| --- | --- | --- |
| `bean init` | Generated projects did not compile (they referenced `bean.BeanConfig`, `bean.Logger()`, etc., which no longer existed, and used the module name instead of the module path as the import path) | Generated projects compile and run as is |
| `bean create service` | After `bean create repo post`, running `bean create service post` wrote a parameterless `NewPostService()` into `route.go`, which failed to compile; when only one `--repo` was passed, the service ignored it while `route.go` passed it in | Without `--repo`, the repository with the same name is injected (if it exists); with `--repo`, the specified repositories are injected (created first if they do not exist, duplicates removed automatically); the service file and the call in `route.go` always match |
| `bean create handler / repo / service` | Inserted code into `routers/route.go` by line number, off by one line (inserted outside the `}` when the struct was empty), and silently skipped when the insertion point could not be found | Parses `routers/route.go`, inserts the code and runs gofmt; when it cannot insert, it reports an error showing what needs to be added by hand |
| `bean create dockerfile` | The generated files were unusable (air's module had been renamed, CMD used the module path, the container name was invalid, the compose file handle was never closed) | Generates a working Dockerfile / docker-compose.yml and adds `.air.toml`; the Go version is taken from the project's go.mod and the port from env.json |
| `bean gen spec` | Output an empty `{}` when the module path contained a domain (e.g. `example.com/demo`); parsing failed when the application wrote logs to stdout | Correct output |
| `bean test gen` | Failed entirely when the spec contained PUT / PATCH routes | Supports PUT / PATCH; other methods use resty's `Execute` |
| `bean test -o json\|html` | Returned 0 even when tests failed if the module path contained no `/`; the report file could not be written if it contained `/` | Exit code is 1 when tests fail; `/` in the report file name is replaced with `_` (e.g. `test_report_example.com_demo_<time>.json`) |

**What to do**:

- Reinstall the CLI: `go install github.com/retail-ai-inc/bean/v3/cmd/bean@latest`.
- Where `bean test -o json|html` is used in CI: the command returns 1 when tests fail; collect reports using the new report file names.
- Already generated projects are not affected by these fixes; if needed, delete the old Dockerfile and other files and regenerate them.

## 8. New features

| Feature | Description |
| --- | --- |
| `helpers.ProcessCSV` / `ProcessCSVFile` | Process large CSV files concurrently: rows are handed to multiple workers in batches, memory usage is bounded, and processing stops at the first error, panic or ctx cancellation. See the [README](README.md#processing-large-csv-files). |
| `helpers.ParseByteSize` | Parses size strings such as `"1M"` and `"512Ki"`, with the same rules as `http.bodyLimit`. |
| `log.WithLevel` / `log.ParseLevel` / `log.SetLogger` / `log.LevelCritical` | Set the application log level, parse level strings, replace the global logger. |
| `BeanLogger.SetLevel` / `Level` | Adjust the application log level at runtime. |
| `berror.PanicError` | The error a panic is converted to, carrying the stack at the point of the panic; see [6.3](#63-monitoring-prometheus--sentry). |
| `berror.IsRouterError` | Reports whether an error was produced by the router (404 / 405). |
| `berror.APIError.StatusCode` | `APIError` implements `echo.HTTPStatusCoder`. |
| `aes.ErrInvalidMAC` | Returned when the MAC of the ciphertext does not match. |
| `bean.ErrConfigNotLoaded` | Returned by `New()` / `NewEcho()` when `config.Bean` has not been loaded. |
| `config.ErrorPage` / `ErrorPage.JSONBody` | Type for the error page configuration. |
| `config.CheckRemovedKeys` | Checks whether `env.json` still contains removed settings (called automatically by `LoadConfig` and `New()`). |
| `config.SetDefaults` and constants such as `config.DefaultHTTPHost` | Defaults for the `http` settings; see [5.2](#52-http-settings-can-be-omitted-keepalive-default-changed). |
| `debugLogLevel` setting | See [5.3](#53-new-debugloglevel). |

## 9. Public API changes at a glance

**Signature changes** (besides the table below, all functions that took `echo.Context` now take `*echo.Context`; see [4.1](#41-handler--middleware-signatures)):

| v2 | v3 |
| --- | --- |
| `bean.New() *Bean` | `bean.New() (*Bean, error)` |
| `bean.NewEcho() (*echo.Echo, func() error)` | `bean.NewEcho() (*echo.Echo, func() error, error)` |
| `log.Init(logger echo.Logger) BeanLogger` | `log.Init(w io.Writer) (*BeanLogger, error)` |
| `log.Logger() BeanLogger` | `log.Logger() *BeanLogger` |
| `log.NewLogger(elogger echo.Logger, options ...LoggerOptions) (*logger, error)` | `log.NewLogger(w io.Writer, options ...LoggerOptions) (*BeanLogger, error)` |
| `type log.BeanLogger interface` | `type log.BeanLogger struct` (embeds `*slog.Logger`) |
| `echoview.ViewEngine.Render(w, name, data, c)` | `echoview.ViewEngine.Render(c, w, name, data)` |

**Removed**:

| Removed API | Replacement |
| --- | --- |
| gommon methods on the `BeanLogger` interface such as `SetOutput` / `Output` / `SetPrefix` / `Prefix` / `SetHeader` / `SetLevel(elog.Lvl)` | See [4.2](#42-logging-unified-on-slog) |
| `config.Bean.AccessLog.BodyDumpMaskParam` | `config.Bean.AccessLog.MaskFields` |
| 404 / 405 middleware (`MethodNotAllowedAndRouteNotFound`, internal) | The Echo router; see [6.1.1](#611-404--405-now-determined-by-the-echo-router) |

**Added**: see [section 8](#8-new-features).

**Deprecated (still usable)**: `Print*`, `Debugf/Debugj`, `Infof/Infoj`, `Warnf/Warnj`, `Errorf/Errorj`, `Fatal*` and `Panic*` on `log.Logger()`, scheduled for removal in v4.

## 10. Upgrade checklist

**Compilation**

- [ ] Go version ≥ 1.25
- [ ] Run the bulk replacement from section 3
- [ ] Keep only `bean/v3`, `echo/v5`, `echo-contrib/v5` in `go.mod`, and run `go mod tidy`
- [ ] Switch logging to the slog style (`Debugf` etc. can stay for now, but are deprecated); calls with a non-string message such as `Error(err)` must be changed
- [ ] Check all `*echo.HTTPError` type assertions and use `echo.StatusCode(err)` / `echo.HTTPStatusCoder` instead
- [ ] Swap the parameter order of a custom `b.Echo.HTTPErrorHandler`
- [ ] Change `b.Echo.Routes()` to `b.Echo.Router().Routes()` in `commands/route.go`
- [ ] Adjust the signatures of custom Binders / Renderers
- [ ] Handle the error returned by `bean.New()` / `bean.NewEcho()`
- [ ] Change `log.Init(echo.New().Logger)`, `echo.NewResponse(w, e)` etc. in tests to the v3 style
- [ ] Change `errors.As` checks for `*middleware.PanicStackError` to `*berror.PanicError`
- [ ] `go build ./... && go test ./...` passes

**Configuration**

- [ ] Rename `accessLog.bodyDumpMaskParam` to `accessLog.maskFields` in `env.json`
- [ ] Projects that do not set `http.keepAlive` in `env.json`: v3 enables keep-alive by default (v2 disabled it); to disable it, set `"keepAlive": false` explicitly (see 5.2)
- [ ] Environments with database `ssl.hotReload` enabled: make sure the certificates are valid for the configured `host` (see 6.4.3)
- [ ] Environments where `mongo.connectTimeout` is set to a small value: the value is now also used for the connection and ping at startup (see 6.5.5)

**Code review**

- [ ] Where `helpers.DecodeJWT` is called with `opts`: make sure all tokens carry `exp` (see 6.4.4)
- [ ] Search for calls to `MSetJSON(`: make sure the original objects are passed, not pre-serialized JSON strings or `[]byte` (see 6.5.1)
- [ ] In cluster mode, code that checks whether the result of `MGet` is `nil` to detect a miss: change it to check each element (see 6.5.1)
- [ ] Wherever the `transport/http` logging Transport is used with `DumpBody` enabled: make sure response bodies are always read to the end or closed, otherwise no `OUTBOUND_API` log is written (see 6.7)
- [ ] Pages that rely on "the previous request's layout carries over": pass `"template"` explicitly (see 6.4.1)

**Runtime verification**

- [ ] Check the startup log for the warning `route is not registered because its method is not listed in http.allowedMethod`
- [ ] Start the service and verify the responses and access logs for 404 / 405 / 500 / timeouts
- [ ] Services with long-running async tasks: make sure the graceful shutdown time is long enough (see 6.6)

**Clients and operations**

- [ ] Adjust the parsing rules and alert conditions of your log platform to the new log fields (see 6.2.1)
- [ ] If any client depends on the JSON response format of 404 / 405, make sure the new `errorMsg` field does not affect it
- [ ] If any client depends on "a 500 is returned when the request body JSON is invalid", change it to handle 400 and `errorCode` `100001` (see 6.1.2)
- [ ] Prometheus 5xx alert thresholds: requests that panic and the real status codes of `APIError` are now both counted (see 6.3)
- [ ] Where `bean test -o json|html` is used in CI: the command returns 1 when tests fail; collect reports using the new report file names (`/` replaced with `_`) (see section 7)
