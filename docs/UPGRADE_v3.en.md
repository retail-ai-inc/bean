# Bean v2 → v3 Upgrade Guide
English | [中文](UPGRADE_v3.md)

The core change in Bean v3 is upgrading the underlying web framework from **Echo v4** to **Echo v5**. Echo v5 itself has many breaking changes (`Context` changed from an interface to a struct, logging moved to `log/slog`, error types changed, etc.), so bean bumps its major version as well, and the module path becomes `github.com/retail-ai-inc/bean/v3`.

This document lists the changes needed to upgrade a project based on bean v2 to v3.

- [1. Requirements](#1-requirements)
- [2. Dependencies and module path](#2-dependencies-and-module-path)
- [3. Mechanical replacements (most of the changes)](#3-mechanical-replacements-most-of-the-changes)
- [4. Changes that need manual work](#4-changes-that-need-manual-work)
- [5. Behavior changes in bean itself](#5-behavior-changes-in-bean-itself)
- [6. Upgrade checklist](#6-upgrade-checklist)

---

## 1. Requirements

| Item | v2 | v3 |
| --- | --- | --- |
| Go | 1.24+ | **1.25+** (required by Echo v5); bean's own `go.mod` uses 1.26 |
| Echo | `github.com/labstack/echo/v4` | `github.com/labstack/echo/v5` |
| echo-contrib | `github.com/labstack/echo-contrib` | `github.com/labstack/echo-contrib/v5` |
| sentry-go/echo | ≤ v0.44.x | **v0.50.0+** (based on Echo v5) |

## 2. Dependencies and module path

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

## 3. Mechanical replacements (most of the changes)

Run the following replacements in the project root (`perl` works on both macOS and Linux):

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

After the replacement, run `go build ./...` and fix the remaining compile errors as described in section 4.

## 4. Changes that need manual work

### 4.1 Handler / middleware signatures

`echo.Context` is now a struct, so all handlers, middleware, and bean error handler functions must take a pointer:

```go
// v2
func (h *userHandler) Get(c echo.Context) error { ... }

// v3
func (h *userHandler) Get(c *echo.Context) error { ... }
```

bean's public functions that take a Context now take `*echo.Context` as well; usage is unchanged:
`trace.StartSpanWithEcho`, `trace.SentryCaptureExceptionWithEcho`, `trace.SentryCaptureMessageWithEcho`, `async.ExecuteWithContext`, `helpers.DecodeJWT`, `helpers.ExtractJWTFromHeader`, `helpers.PostDataStripTags`, `echoview.Render`.

### 4.2 Unified logging with slog

v2 had three separate logs, each with its own format: bean's application log (gommon logger), the Echo log, and the access log. v3 unifies them on the standard library `log/slog`:

- `log.Logger()`, `e.Logger`, and `c.Logger()` are the **same** logger, and all logs (including access logs) use the same JSON format.
- When you use the `...Context` methods and pass the request's context, the request ID (`id`) and Sentry trace ID are added automatically; there is no need to add them by hand.
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

**Compatibility layer (deprecated):** `log.Logger()` still provides v2's `Print*`, `Debugf/Debugj`, `Infof/Infoj`, `Warnf/Warnj`, `Errorf/Errorj`, `Fatal*`, and `Panic*` methods. They also emit the new format, so you can migrate gradually; they are scheduled for removal in v4. The `*j` methods take a `map[string]any` argument, so passing `elog.JSON` still compiles.

Usages that need manual changes:

| v2 | v3 |
| --- | --- |
| `log.Logger().Debug(a, b)` / `Info(...)` / `Warn(...)` / `Error(err)` | The first argument must be a string message, followed by key-value pairs, e.g. `Error("failed", "error", err)` |
| `log.Logger().SetLevel(elog.INFO)` / `Level()` | `SetLevel(slog.LevelInfo)` / `Level()`, or use the `debugLogLevel` setting |
| `log.Logger().SetOutput(w)` / `Output()` / `SetPrefix` / `SetHeader` | Removed; the output destination is determined by `debugLogPath`, or create a logger with `log.NewLogger(w, ...)` |
| `log.BeanLogger` (interface) | `*log.BeanLogger` (struct embedding `*slog.Logger`) |
| `log.Init(e.Logger)` (takes effect only once, panics on failure) | `log.Init(w io.Writer) (*log.BeanLogger, error)`; calling it again replaces the global logger |
| `log.NewLogger(e.Logger, opts...)` | `log.NewLogger(w io.Writer, opts...)`; adds `log.WithLevel` |

Most projects don't call `log.Init` themselves (`bean.New()` initializes it internally), so this mainly affects test code:

```go
// v2
_ = log.Init(echo.New().Logger)

// v3: write to a buffer so you can assert on the output
var buf bytes.Buffer
l, _ := log.NewLogger(&buf)
log.SetLogger(l)
```

**Log format changes** (if your log platform's parsing rules or alerts depend on field names, update them accordingly):

| | v2 | v3 |
| --- | --- | --- |
| Application log | `{"time":…,"level":"INFO","prefix":"echo","file":"x.go","line":"12","message":…}` | `{"timestamp":…,"severity":"INFO","message":…,"file":"pkg/x.go","line":12,…}` |
| Echo / `c.Logger()` log | `{"time":…,"level":"INFO","message":…}` | Same as above |
| Access log | `{"timestamp":…,"severity":"INFO","level":"ACCESS",…}` | Unchanged |

- `severity` values: `DEBUG`, `INFO`, `WARNING` (slog's Warn), `ERROR`, `CRITICAL` (the compatibility layer's `Fatal*` / `Panic*`).
- The `level` field no longer appears in application logs; in access logs, `level` is still the category such as `ACCESS` / `DUMP` / `OUTBOUND_API`.
- New setting `debugLogLevel`: the minimum level of the application log, `debug` (default) / `info` / `warn` / `error`. It does not affect the access log.

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

**Important**: predefined errors such as `echo.ErrNotFound`, `echo.ErrMethodNotAllowed`, and `echo.ErrUnauthorized` are **no longer `*echo.HTTPError`** in v5; they only implement the `echo.HTTPStatusCoder` interface. If your custom error handling is written like this, 404/405 will be treated as 500:

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

bean's built-in `berror.HTTPErrorHandlerFunc` already handles this.

### 4.4 Custom Echo global error handler

bean's `berror.ErrorHandlerFunc` (`func(err error, c *echo.Context) (bool, error)`) keeps the **same parameter order**.
But if you replaced `b.Echo.HTTPErrorHandler` directly, Echo's signature has its parameters **swapped**:

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

When wrapping a custom ResponseWriter, implement `Unwrap() http.ResponseWriter` so that `echo.UnwrapResponse` and `http.ResponseController` can find the underlying writer.

### 4.6 Routing

```go
// v2
for _, r := range b.Echo.Routes() { ... }

// v3
for _, r := range b.Echo.Router().Routes() { ... }
```

The return value of registration methods such as `e.GET()` changes from `*echo.Route` to `echo.RouteInfo` (a value type). `e.Reverse()`, `e.URI()`, `e.URL()`, and `e.Host()` are removed; use `e.Router().Routes().Reverse(name, params...)` instead.

> **Projects generated by `bean init`**: change `b.Echo.Routes()` in `commands/route.go` to `b.Echo.Router().Routes()`, otherwise `route list` and `bean gen spec` won't compile.

### 4.7 Custom Binder / Renderer / Validator

```go
// Binder: parameter order swapped
// v2
func (b *MyBinder) Bind(i interface{}, c echo.Context) error
// v3
func (b *MyBinder) Bind(c *echo.Context, i interface{}) error

// Renderer: Context moves to the first parameter
// v2
func (r *MyRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error
// v3
func (r *MyRenderer) Render(c *echo.Context, w io.Writer, name string, data interface{}) error
```

The Validator interface (`Validate(i interface{}) error`) is unchanged.

### 4.8 Echo middleware

Common changes (for the full list, see Echo's [API_CHANGES_V5.md](https://github.com/labstack/echo/blob/master/API_CHANGES_V5.md)):

| v4 | v5 |
| --- | --- |
| `middleware.Logger()` / `LoggerWithConfig` | Removed; use `middleware.RequestLogger()` or bean's built-in access log |
| `middleware.Timeout()` | Removed; use `middleware.ContextTimeout()` (built into bean, see `http.timeout`) |
| `middleware.BodyLimit("2M")` | `middleware.BodyLimit(2 * middleware.MB)` (the argument is in bytes) |
| `middleware.DefaultSkipper(c echo.Context)` | `middleware.DefaultSkipper(c *echo.Context)` |
| `RequestIDHandler func(echo.Context, string)` | `func(*echo.Context, string)` |
| `TrailingSlashConfig` | Split into `AddTrailingSlashConfig` / `RemoveTrailingSlashConfig` |
| `middleware.DefaultCORSConfig` and other `Default*Config` variables | Removed; construct the Config directly |

### 4.9 Test code

```go
// v2
c := e.NewContext(req, rec)                              // returns echo.Context
c.SetResponse(echo.NewResponse(w, e))
c.SetParamNames("id"); c.SetParamValues("1")

// v3
c := e.NewContext(req, rec)                              // returns *echo.Context
c.SetResponse(echo.NewResponse(w, e.Logger))             // the second argument is now *slog.Logger
c.SetPathValues(echo.PathValues{{Name: "id", Value: "1"}})
```

### 4.10 Other common compile errors

| Compile error | Fix |
| --- | --- |
| `e.HideBanner` / `e.HidePort` / `e.Debug` undefined | Remove them (bean starts its own `http.Server` and does not print a banner) |
| `e.Start(...)` / `e.Shutdown(...)` | Handled by bean's `ServeAt`; projects usually don't need to call them |
| `c.ParamNames()` / `c.ParamValues()` | Use `c.PathValues()` instead |
| `echo.FormParam[T]` | Renamed to `echo.FormValue[T]` |
| `echo.PathParamsBinder(c)` | Renamed to `echo.PathValuesBinder(c)` |
| `echo.GetPath(r)` | Use `r.URL.RawPath` / `r.URL.Path` instead |

### 4.11 `bean.New()` / `bean.NewEcho()` return an error

In v2, configuration errors (`http.trustIPRange`, `http.bodyLimit`, Sentry initialization, Prometheus, goroutine pools, etc.) made `New()` call `log.Fatal` and exit the process; this could not be verified in tests, and resources already initialized were not cleaned up. v3 returns an error instead and, on failure, releases the resources already initialized (such as registered goroutine pools):

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

If `config.Bean` has not been loaded, `bean.ErrConfigNotLoaded` is returned.

If a regular expression in `accessLog.skipEndpoints`, `prometheus.skipEndpoints`, or `sentry.skipTracesEndpoints` is invalid, v2 panics, while v3 returns an error from `New()` that includes the name of the setting.

> **Projects generated by `bean init`**: handle the error from `bean.New()` in `commands/start.go`, `commands/route.go`, and `commands/gopher/gopher.go` as shown above.

### 4.12 Setting `accessLog.bodyDumpMaskParam` renamed to `accessLog.maskFields`

The old name was misleading: `bodyDump` suggested that it applied only to request/response bodies, and only when `bodyDump` was enabled; `Param` suggested that it targeted URL parameters. In fact, it masks matching fields in all structured access logs (`ACCESS`, `DUMP`, `OUTBOUND_API`), including `request_header` / `response_header`, regardless of whether `bodyDump` is enabled. The new name `maskFields` matches `log.WithMaskFields`. The masking behavior itself is unchanged.

Update each project's `env.json`:

```diff
 "accessLog": {
-  "bodyDumpMaskParam": ["password", "token"],
+  "maskFields": ["password", "token"],
 }
```

- If `bodyDumpMaskParam` still has values, **startup fails** with the error `config: accessLog.bodyDumpMaskParam has been removed, rename it to accessLog.maskFields`, so that masking does not silently stop working. Both `config.LoadConfig` and `bean.New()` perform this check, so it applies even if the project reads the config with viper itself.
- An empty `"bodyDumpMaskParam": []` (the default generated by older project templates) is ignored, but we recommend removing or renaming it.
- In Go code, replace reads of `config.Bean.AccessLog.BodyDumpMaskParam` with `config.Bean.AccessLog.MaskFields`.

> **Projects generated by `bean init`**: `commands/root.go` now loads the config with `config.LoadConfig("env.json")` instead of calling viper directly.

## 5. Behavior changes in bean itself

The following changes are handled inside bean. Projects usually don't need code changes, but you should be aware of them:

1. **Route names keep the v4 behavior**: Echo v5's default route name is `GET:/path`; bean uses a custom Router to keep naming routes after the handler function (e.g. `myapp/handlers.(*userHandler).Get-fm`), so the output of `route list` and `gen spec` is the same as in v2. Routes with an explicit `Name` are not affected.
2. **The `http.bodyLimit` format is unchanged**: you can still write `"1M"`, `"512K"`, etc.; bean converts them to bytes. Startup fails if the value cannot be parsed.
3. **Timeouts return 504**: when `http.timeout` is exceeded, bean returns `echo.NewHTTPError(504, "gateway timeout").Wrap(context.DeadlineExceeded)`; what is reported to Sentry is still the internal error.
4. **404 / 405 are now determined by the Echo router**: see [5.1](#51-how-404--405-are-handled) for details.
5. **Access log (body dump) memory is bounded**: v2 read the entire request and response bodies into memory before truncating them, so for large uploads/downloads memory usage equaled the body size. v3 keeps only the first `accessLog.bodyLimit + 1` bytes (8KB by default) in memory and streams the rest directly to the handler / client. Log fields and content are unchanged (truncation was always based on `accessLog.bodyLimit`). Also, when a client disconnects mid-upload, v2 passed the part read so far to the handler as if it were the complete request body, while in v3 the handler receives a read error; `request_body_error` / `bytes_in_read` are logged as before.
6. **Unified logging**: application logs, Echo logs, and access logs use the same JSON format; see [4.2](#42-unified-logging-with-slog) for details. `log.Shutdown` only shuts down asynchronous access-log writing; application logs can still be written afterwards.
7. **Server startup and shutdown**: `ServeAt` now uses Echo v5's `echo.StartConfig` for startup and graceful shutdown. The settings (`http.keepAlive`, `http.shutdownTimeout`, `http.ssl.*`) keep their meaning; no read timeout is set (same as v2); HTTPS still supports HTTP/2, and certificate paths can be absolute. The internal error log of `http.Server` now goes through Echo's slog logger. When the `BeforeServe` callback runs, the server's shutdown function is already registered, so it is safe to call `b.Shutdown()` in or after the callback.
8. **Error handling**:
   - `berror.APIError` (status code taken from `HTTPStatusCode`) and validation errors (400) implement `echo.HTTPStatusCoder`. Echo middleware such as the Prometheus metrics records their real status codes; v2 recorded all of them as 500.
   - `berror.HTTPErrorHandlerFunc` no longer handles `APIError` or validation errors (they have their own handler functions), regardless of registration order.
   - `http.errorMessage.*` is now read from `config.Bean` (parsed once at startup) instead of querying viper on every error. `http.errorMessage.e405.html.file` now takes effect (v2 always used `errors/html/405` for 405). `value` in the JSON settings is always output as a string.
   - The 5 identical anonymous structs in the config structure are merged into the `config.ErrorPage` type; field names are unchanged.
9. **`sentry.skipTracesEndpoints` also works with a single entry**: in v2, sampling was skipped only when two or more entries were configured (the condition was written as `> 1`).
10. **Project template**: projects generated by `bean init` fix several issues that already broke compilation in v2 (references to `bean.BeanConfig`, `bean.Logger()`, `bean.SentryCaptureException`, `bean.DefaultBeforeSend`, and `bean.Cleanup`, which no longer exist, using the module name instead of the module path as the import path, etc.), and now use `config.Bean`, `log.Logger()`, `trace.SentryCaptureException`, and `trace.DefaultBeforeSend`.
11. **Requests that panic are counted in Prometheus metrics**: in v2, the `Recover` middleware was registered outside the Prometheus middleware, so a panic passed straight through the Prometheus middleware; such requests were missing entirely from the metrics, and the 5xx error rate was understated. v3 moves `Recover` to the innermost position of bean's built-in middleware, so a panic is first converted to an error and is counted with `code="500"` in metrics such as `echo_requests_total`.
12. **Panics reported to Sentry show the stack where the panic occurred**: in v2, a panic was converted to an error carrying only a text stack trace, which Sentry could not parse, so Sentry showed the stack at the moment of reporting (in the error handler). v3 uses bean's own recover middleware, and a panic is converted to `*berror.PanicError`:
    - The `Value` field holds the panic value; if it is an error (e.g. `runtime.Error`), it can be retrieved with `errors.As` / `errors.Is`.
    - `StackTrace()` provides the stack at the panic site; the innermost frame in Sentry is the function that panicked.
    - `Error()` returns only `[PANIC RECOVER] <panic value>` and no longer includes the whole goroutine stack, so the `error` field in the access log is shorter. To get the local stack, format with `%+v` (this is how the error log is written when Sentry is not enabled).
    - If your project used `errors.As` to check for Echo's `*middleware.PanicStackError`, change it to `*berror.PanicError`.
13. **The `http` section of `env.json` can be omitted**: in v2, startup panicked if `http.bodyLimit` was not configured. v3 provides defaults for the following settings (taken from the project template): `http.host` = `0.0.0.0`, `http.port` = `8888`, `http.bodyLimit` = `1M`, `http.keepAlive` = `true`. Note: in v2, omitting `keepAlive` meant `false` (keep-alive disabled), while in v3 omitting it means `true`; if you really want keep-alive disabled, configure `"keepAlive": false` explicitly. The behavior when other `http` settings are omitted is unchanged (e.g. no request timeout is set, and an empty `allowedMethod` allows all methods).
14. **`bean create service` injects the repository correctly**: in v2, running `bean create service post` after `bean create repo post` generated a `NewPostService` that required a repository argument, but `routers/route.go` received the no-argument call `services.NewPostService()`, so the build failed; when a single `--repo` whose name differed from the service was given, the generated service ignored that repository while `route.go` passed it in, which also failed to compile. In v3: without `--repo`, the repository with the same name is injected (if it exists); with `--repo`, the specified repositories are injected (created first if they don't exist, with duplicate names removed automatically), and the service file always matches the call in `route.go`. Projects already generated are not affected.
15. **HTML template rendering (`goview` / `echoview`) is concurrency-safe**: v2's template engine shared mutable state across requests. Under concurrent rendering, `{{include "..."}}` could render with **another request's data** (a cross-user data leak); the layout specified by `data["template"]` was written into the shared config and affected subsequent requests; and rendering the same template with a different layout failed with `"templates/xxx" is undefined`. In v3, layouts are per request, caches are separated by layout, and each render uses its own copy of the template; templates are written the same way as before. Also: when rendering fails, the status code and a partial page are no longer written; `goview.M` can be passed as data; and `echoview.Render` uses the given status code in middleware mode (v2 always used 200).
16. **`aes.BeanAESDecrypt` verifies the MAC**: v2 did not verify the `mac` in the ciphertext when decrypting, so tampered ciphertext could also be decrypted "successfully" (creating the conditions for a padding oracle attack). v3 verifies the MAC with a constant-time comparison before decrypting and returns `aes.ErrInvalidMAC` on a mismatch. The ciphertext format is unchanged: existing ciphertext produced by `BeanAESEncrypt` (and by Laravel's `Encrypter`), such as tenant database passwords, decrypts as before; only ciphertext with an incorrect `mac` is rejected. Malformed input returns an error instead of panicking, and empty strings can be encrypted.
17. **Database TLS `hotReload` verifies the server hostname**: in v2, with `ssl.hotReload` enabled, the certificate's hostname was not verified (never for MySQL; for Redis / Mongo when connecting by IP address), so a certificate issued by any trusted CA was accepted. v3 requires the certificate to be valid for the hostname used for the connection; when connecting by IP address, the certificate must be valid for one of the configured `host` values (an IP matches the certificate's IP SAN). If a database certificate doesn't match the configured `host`, connections fail once `hotReload` is enabled; fix the certificate or the `host`. Note: for Redis cluster nodes that are discovered at runtime, accessed by IP, and not listed in the configured `host`, the certificate must be valid for one of the configured `host` values.
18. **`helpers.DecodeJWT` default validation always applies**: in v2, passing any `opts` (e.g. `jwt.WithAudience`) disabled all of the defaults ("HS256 only", "`exp` required", and "strict decoding"), so tokens without `exp` were valid forever. In v3, the default options always apply and `opts` are appended after them (you can still override the allowed algorithms with `jwt.WithValidMethods`); to accept tokens without `exp`, use `jwt.ParseWithClaims` directly. `helpers.ExtractJWTFromHeader` requires a space after `Bearer` (in v2, `Bearerxabc` also yielded `abc`), and the scheme is case-insensitive.
19. **JSON request body errors return 400**: in v2, when the request body had a JSON syntax error, unknown fields, or a type mismatch, `c.Bind` returned a plain error, the client received **500**, and the error was reported to Sentry. v3 returns a 400 `*berror.APIError` with the response `{"errorCode":"100001","errorMsg":"invalid JSON request body: ..."}` (`PROBLEM_PARSING_JSON`) and does not report it to Sentry. False rejections are also fixed: fields of embedded structs, fields without a json tag, and arbitrary fields when binding to a map were all rejected as unknown fields in v2. Extra data after the JSON value (e.g. `{"a":1} garbage`) is now rejected.
20. **`async.ExecuteWithContext` / `ExecuteWithTimeout` preserve the request context**: in v2, the task context of these two functions lost the request ID (async task logs had no `id`) and the request's Sentry trace, and `ExecuteWithTimeout` also made the task share the same Sentry hub as the request. In v3 they behave like `ExecuteContext`: the context is not canceled along with the request, and it carries the request ID, a cloned hub, and the continued trace. Also: when no pool name is given, they run in the default pool (v2 started a goroutine directly), so graceful shutdown waits for them; in `ExecuteWithTimeout`, an error returned by the task is logged in addition to being reported to Sentry.
21. **Redis**: in v2, `RPush` failed on every call (`can't marshal *[]string`); this is fixed in v3, and an empty list is a no-op. In v2, `MSetJSON` in `store/redis` did not JSON-encode values (passing a struct failed, and the written values could not be read by `GetJSON`); in v3 it is consistent with `SetJSON` and calls `json.Marshal` on each value. **Note**: if your project worked around the v2 issue by passing pre-serialized JSON strings (or `[]byte`) to `MSetJSON`, v3 encodes them again (a string becomes a quoted JSON string, and `[]byte` becomes base64), so data read back with `GetJSON` / `MGetJSON` will be wrong; pass the original objects (structs, maps, etc.) instead. Data already in the cache is not affected. In cluster mode, when any key passed to `MGet` doesn't exist, v2 returned nil for the whole batch, while v3 returns results per key (nil for missing keys); `Keys` now queries all masters; and multi-key `DelKey` no longer fails with CROSSSLOT.
22. **Outbound requests**: with `DumpBody`, closing the response body in the logging Transport of `transport/http` did not close the underlying connection (a connection leak); this is fixed in v3. Only `BodyLimit` bytes of the request body are read for logging, and if reading fails, the request fails instead of sending a truncated body. `trace.PropagateToGRPC` no longer overwrites outgoing metadata already in the context (such as authorization), and calling `PropagateToHTTP` repeatedly no longer produces duplicate headers.
23. **bean CLI**: `bean gen spec` no longer outputs an empty `{}` for module paths that contain a domain (e.g. `example.com/demo`); `bean test gen` supports PUT / PATCH; `bean test -o json|html` exits with code 1 when tests fail (in v2, a test failure still returned 0 when the module path contained no `/`, and the report could not be written when it did), and `/` in the report file name is replaced with `_` (e.g. `test_report_example.com_demo_<time>.json`), so adjust CI steps that rely on the exit code or collect reports by file name; `bean create dockerfile` generates a working Dockerfile / docker-compose.yml and also adds `.air.toml`; `bean create handler|repo|service` inserts code by parsing `routers/route.go`, and when it cannot insert the code, it reports an error showing what needs to be added by hand.
24. **Database initialization**: when initialization fails midway (a tenant connection fails, a Redis read-replica connection fails, a ping fails, etc.), the connections already established are closed before the error is returned (they leaked in v2). If a field in `TenantConnections.Connections` has the wrong type or is missing, startup returns an error that includes the tenant ID instead of panicking; numbers such as `"port": 3306` are also accepted. With `ssl.hotReload` enabled, all connections that use the same certificate file share a single file watcher, which stops when the connections are closed (v2 started one per connection and never stopped them).
25. **Mongo**: the initial connection and ping use `mongo.connectTimeout` (10s if not set; v2 always used 10s, so if you configured a smaller value, startup now times out sooner); when `host` lists multiple hosts (`a,b`), every host uses the configured port (in v2 only the last one did); sessions are ended properly on close. `maxConnectionLifeTime` still maps to the driver's max idle time (the mongo driver has no option for a maximum connection lifetime).
26. **Redis read replicas**: a miss on a read replica (`redis.Nil`) is no longer retried on the master (v2 made an extra query to the master on every miss); server errors such as `WRONGTYPE` are returned directly instead of being masked by the master's result; only connection errors fall back to the master, and that replica is then not used for 10 seconds (in v2, when a replica was down, every read first waited for the dial timeout). `HGets` no longer masks network errors from other commands because one field doesn't exist.
27. **Outbound request logs**: with `DumpBody` enabled, the `OUTBOUND_API` log is now written when the response body has been read to the end, when reading fails, or when the body is closed (v2 read up to `BodyLimit` bytes before returning the response, which blocked streaming responses such as SSE / long polling). Therefore, **the response body must be fully read or closed** for this log to be written (not closing the response body leaks the connection anyway); a `response_body_error` field is added when reading fails. In addition, the values of `Authorization`, `Proxy-Authorization`, `Cookie`, `Set-Cookie`, and any request or response header whose name contains token / secret / password / api-key / apikey / session are replaced with `[REDACTED]` in the log, even if those headers are in the list of headers allowed to be logged.
28. **Other**: `CloseMemory` can be called more than once (in v2 the second call blocked forever), and calling `memory.NewMemoryCache()` after closing returns a new cache; wildcard deletion (`DelMemory("prefix*")`) now runs in linear time. `str.Substring` measures length in characters (runes); `str.LeftPadToLength` / `RightPadToLength` and `helpers.JitterBackoff` no longer panic on edge-case inputs; `helpers.StructStripTags` no longer loses precision for large integers.

### 5.1 How 404 / 405 are handled

v2 used a global middleware that, on every request, iterated over all routes and matched the path itself to decide on 404 / 405. v3 removes this middleware; instead, the Echo router decides directly (returning `echo.ErrNotFound` / `echo.ErrMethodNotAllowed`) and hands the error to bean's error handler to write the response.

The status code rules are the same as in v2:

| Request | Result |
| --- | --- |
| Path does not exist | 404 |
| Path exists, but the method is not registered | 405 (since v3, an `Allow` header is also returned) |
| The route's method is not in `http.allowedMethod` | The route is **not registered** (a WARN log is printed at startup); requests return 404 / 405 according to the two rows above |
| `e.Any(...)` / `g.Any(...)` | Registered only for the methods in `http.allowedMethod`; other methods return 405; `route list` lists one entry per method |
| Path exists under a Group, but the method is wrong | 405 (by default Echo v5 returns 404 because of the Group's catch-all route; bean disables this behavior) |

Changes to note:

1. **JSON response body**: an `errorMsg` field is added, e.g. a 404 is `{"errorCode":"100003","errorMsg":"Not Found","errors":null}` (v2: `{"errorCode":"100003","errors":null}`). Custom JSON in `http.errorMessage.e404` / `e405` in `env.json` **now also applies to 404 / 405 generated by the router** (it did not in v2).
2. **HTML response**: the `errors/html/404` and `errors/html/405` templates (or the file specified by `http.errorMessage.e404.html.file`) are used as well; the `stacktrace` variable in the template is `Not Found` / `Method Not Allowed` (empty in v2).
3. **Even if `berror.HTTPErrorHandlerFunc` is not registered**, 404 / 405 generated by the router are returned in the format above and do not become 500. If your project registers its own global error handler function, it now also receives these errors; use `berror.IsRouterError(c)` to tell them apart.
4. **404 / 405 requests go through global middleware**: in v2, these requests were answered directly in that middleware and never reached middleware such as the access log, Request ID, Sentry, and Prometheus; in v3 they go through them normally, so 404 / 405 requests appear in the access log. In Prometheus, all 404s generated by the router share a single `url` label, so that scanning requests don't blow up the number of metrics. These requests are no longer logged as ERROR.
5. **Group middleware does not run for unknown paths**: for example, an authentication middleware on the `/api` Group does not run for `/api/unknown`, which returns 404 directly, the same as in v2.
6. **`OPTIONS` requests**: `OPTIONS` requests that are not CORS preflight requests return `204` with an `Allow` header when the path exists (v2 returned 405). CORS preflight requests are still handled by the CORS middleware.

## 6. Upgrade checklist

- [ ] Go version ≥ 1.25
- [ ] Run the bulk replacements from section 3
- [ ] Keep only `bean/v3`, `echo/v5`, and `echo-contrib/v5` in `go.mod`, and run `go mod tidy`
- [ ] Switch logging to the slog style (`Debugf` etc. can stay for now but are deprecated); calls with a non-string message such as `Error(err)` must be changed
- [ ] Update your log platform's parsing rules and alert conditions for the new log fields (see 4.2)
- [ ] Check all `*echo.HTTPError` type assertions and switch to `echo.StatusCode(err)` / `echo.HTTPStatusCoder`
- [ ] Swap the parameter order of any custom `b.Echo.HTTPErrorHandler`
- [ ] Change `b.Echo.Routes()` in `commands/route.go` to `b.Echo.Router().Routes()`
- [ ] Update the signatures of custom Binders / Renderers
- [ ] Handle the error returned by `bean.New()` / `bean.NewEcho()`
- [ ] Rename `accessLog.bodyDumpMaskParam` to `accessLog.maskFields` in `env.json`
- [ ] Projects that don't set `http.keepAlive` in `env.json`: v3 enables keep-alive by default (v2 disabled it); to disable it, set `"keepAlive": false` explicitly (see section 5, item 13)
- [ ] Update `log.Init(echo.New().Logger)`, `echo.NewResponse(w, e)`, etc. in tests to the v3 style
- [ ] `go build ./... && go test ./...` passes
- [ ] Check the startup log for `route is not registered because its method is not listed in http.allowedMethod` warnings
- [ ] If any clients depend on the JSON format of 404 / 405 responses, confirm that the new `errorMsg` field doesn't affect them
- [ ] Start the service and verify the responses and access logs for 404 / 405 / 500 / timeouts
- [ ] Environments with database `ssl.hotReload` enabled: confirm that the certificates are valid for the configured `host` (see section 5, item 17)
- [ ] Places that call `helpers.DecodeJWT` with `opts`: confirm that all tokens have `exp` (see section 5, item 18)
- [ ] If any clients rely on "500 when the request body JSON is invalid", change them to handle 400 and `errorCode` `100001` (see section 5, item 19)
- [ ] Search for calls to `MSetJSON(`: confirm that they pass the original objects, not pre-serialized JSON strings or `[]byte` (see section 5, item 21)
- [ ] CI jobs that use `bean test -o json|html`: the command now returns 1 when tests fail; collect reports by the new file names (`/` replaced with `_`) (see section 5, item 23)
- [ ] Places that use the `transport/http` logging Transport with `DumpBody` enabled: confirm that response bodies are always fully read or closed, otherwise no `OUTBOUND_API` log is written (see section 5, item 27)
- [ ] Environments where `mongo.connectTimeout` is set to a small value: the value is now also used for the connection and ping at startup (see section 5, item 25)
