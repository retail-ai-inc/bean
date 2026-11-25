# Bean v2 → v3 升级指南

Bean v3 的核心变化是把底层 Web 框架从 **Echo v4** 升级到 **Echo v5**。Echo v5 本身有大量不兼容改动（`Context` 从接口变为结构体、日志改用 `log/slog`、错误类型调整等），因此 bean 也随之升级大版本，模块路径改为 `github.com/retail-ai-inc/bean/v3`。

本文列出基于 bean v2 的项目升级到 v3 需要做的改动。

- [1. 环境要求](#1-环境要求)
- [2. 依赖与模块路径](#2-依赖与模块路径)
- [3. 机械替换（覆盖大部分改动）](#3-机械替换覆盖大部分改动)
- [4. 需要手工处理的改动](#4-需要手工处理的改动)
- [5. bean 自身的行为变化](#5-bean-自身的行为变化)
- [6. 升级检查清单](#6-升级检查清单)

---

## 1. 环境要求

| 项目 | v2 | v3 |
| --- | --- | --- |
| Go | 1.24+ | **1.25+**（Echo v5 要求），bean 自身 `go.mod` 为 1.26 |
| Echo | `github.com/labstack/echo/v4` | `github.com/labstack/echo/v5` |
| echo-contrib | `github.com/labstack/echo-contrib` | `github.com/labstack/echo-contrib/v5` |
| sentry-go/echo | ≤ v0.44.x | **v0.50.0+**（基于 Echo v5） |

## 2. 依赖与模块路径

```bash
go get github.com/retail-ai-inc/bean/v3@v3.0.0
```

```bash
go get github.com/labstack/echo/v5@latest
```

然后把 `go.mod` 中的 `github.com/retail-ai-inc/bean/v2`、`github.com/labstack/echo/v4` 删除，执行：

```bash
go mod tidy
```

bean CLI 也需要重新安装：

```bash
go install github.com/retail-ai-inc/bean/v3/cmd/bean@latest
```

## 3. 机械替换（覆盖大部分改动）

在项目根目录执行以下替换（macOS / Linux 均可用 `perl`）：

```bash
find . -type f -name '*.go' -not -path './vendor/*' -print0 | xargs -0 perl -pi -e 's#retail-ai-inc/bean/v2\b#retail-ai-inc/bean/v3#g; s#labstack/echo/v4#labstack/echo/v5#g; s#labstack/echo-contrib/#labstack/echo-contrib/v5/#g; s/(?<![\*\w.])echo\.Context\b/*echo.Context/g; s#\becho\.Map\b#map[string]any#g'
```

它完成了：

| 替换前 | 替换后 |
| --- | --- |
| `github.com/retail-ai-inc/bean/v2/...` | `github.com/retail-ai-inc/bean/v3/...` |
| `github.com/labstack/echo/v4` | `github.com/labstack/echo/v5` |
| `github.com/labstack/echo-contrib/xxx` | `github.com/labstack/echo-contrib/v5/xxx` |
| `func(c echo.Context) error` | `func(c *echo.Context) error` |
| `echo.Map{...}` | `map[string]any{...}` |

替换后执行 `go build ./...`，剩余的编译错误按第 4 节处理。

## 4. 需要手工处理的改动

### 4.1 Handler / 中间件签名

`echo.Context` 变成了结构体，所有 handler、中间件、bean 的错误处理函数都要用指针：

```go
// v2
func (h *userHandler) Get(c echo.Context) error { ... }

// v3
func (h *userHandler) Get(c *echo.Context) error { ... }
```

bean 中接收 Context 的公开函数签名同步变为 `*echo.Context`，用法不变：
`trace.StartSpanWithEcho`、`trace.SentryCaptureExceptionWithEcho`、`trace.SentryCaptureMessageWithEcho`、`async.ExecuteWithContext`、`helpers.DecodeJWT`、`helpers.ExtractJWTFromHeader`、`helpers.PostDataStripTags`、`echoview.Render`。

### 4.2 日志统一为 slog

v2 中有三套日志：bean 的业务日志（gommon logger）、Echo 日志、访问日志，格式各不相同。v3 统一到标准库 `log/slog`：

- `log.Logger()`、`e.Logger`、`c.Logger()` 是**同一个** logger，所有日志（包括访问日志）都是同一种 JSON 格式。
- 使用 `...Context` 方法并传入请求的 context 时，自动带上请求 ID（`id`）和 Sentry trace ID，不需要再手动拼接。
- 不再依赖 `github.com/labstack/gommon`。

**推荐写法（slog）：**

```go
import "github.com/retail-ai-inc/bean/v3/log"

// v2
log.Logger().Debugf("user %d logged in", userID)
log.Logger().Errorj(elog.JSON{"message": "failed to get user", "error": err.Error(), "id": reqID})
c.Logger().Errorf("failed to get user: %v", err)

// v3
log.Logger().DebugContext(ctx, "user logged in", "user_id", userID)
log.Logger().ErrorContext(ctx, "failed to get user", "error", err) // id / trace 自动添加
c.Logger().ErrorContext(c.Request().Context(), "failed to get user", "error", err)
```

**兼容层（已废弃）：** `log.Logger()` 上仍保留 v2 的 `Print*`、`Debugf/Debugj`、`Infof/Infoj`、`Warnf/Warnj`、`Errorf/Errorj`、`Fatal*`、`Panic*` 方法，底层同样输出新格式，可以逐步迁移，计划在 v4 删除。`*j` 方法的参数类型为 `map[string]any`，传入 `elog.JSON` 也能编译。

需要手工修改的写法：

| v2 | v3 |
| --- | --- |
| `log.Logger().Debug(a, b)` / `Info(...)` / `Warn(...)` / `Error(err)` | 第一个参数必须是 string 消息，其后是键值对，例如 `Error("failed", "error", err)` |
| `log.Logger().SetLevel(elog.INFO)` / `Level()` | `SetLevel(slog.LevelInfo)` / `Level()`，或使用 `debugLogLevel` 配置 |
| `log.Logger().SetOutput(w)` / `Output()` / `SetPrefix` / `SetHeader` | 已删除；输出目标由 `debugLogPath` 决定，或用 `log.NewLogger(w, ...)` 创建 |
| `log.BeanLogger`（接口） | `*log.BeanLogger`（结构体，内嵌 `*slog.Logger`） |
| `log.Init(e.Logger)`（只生效一次，失败时 panic） | `log.Init(w io.Writer) (*log.BeanLogger, error)`；再次调用会替换全局 logger |
| `log.NewLogger(e.Logger, opts...)` | `log.NewLogger(w io.Writer, opts...)`；新增 `log.WithLevel` |

一般项目不会自己调用 `log.Init`（`bean.New()` 内部已初始化），主要影响测试代码：

```go
// v2
_ = log.Init(echo.New().Logger)

// v3：输出到缓冲区以便断言
var buf bytes.Buffer
l, _ := log.NewLogger(&buf)
log.SetLogger(l)
```

**日志格式变化**（如果日志平台的解析规则或告警依赖字段名，需要同步调整）：

| | v2 | v3 |
| --- | --- | --- |
| 业务日志 | `{"time":…,"level":"INFO","prefix":"echo","file":"x.go","line":"12","message":…}` | `{"timestamp":…,"severity":"INFO","message":…,"file":"pkg/x.go","line":12,…}` |
| Echo / `c.Logger()` 日志 | `{"time":…,"level":"INFO","message":…}` | 同上 |
| 访问日志 | `{"timestamp":…,"severity":"INFO","level":"ACCESS",…}` | 不变 |

- `severity` 取值：`DEBUG`、`INFO`、`WARNING`（slog 的 Warn）、`ERROR`、`CRITICAL`（兼容层的 `Fatal*` / `Panic*`）。
- 业务日志中的 `level` 字段不再出现；访问日志中的 `level` 仍是 `ACCESS` / `DUMP` / `OUTBOUND_API` 等分类。
- 新配置项 `debugLogLevel`：业务日志的最低级别，`debug`（默认）/ `info` / `warn` / `error`，不影响访问日志。

### 4.3 `echo.HTTPError`

```go
// v2
return echo.NewHTTPError(http.StatusBadRequest, "invalid", detail)
return &echo.HTTPError{Code: 504, Message: "timeout", Internal: err}
he.Internal

// v3
return echo.NewHTTPError(http.StatusBadRequest, "invalid")           // message 只能是 string
return echo.NewHTTPError(504, "timeout").Wrap(err)                   // 用 Wrap 携带内部错误
he.Unwrap()                                                          // 取内部错误
```

**特别注意**：`echo.ErrNotFound`、`echo.ErrMethodNotAllowed`、`echo.ErrUnauthorized` 等预定义错误在 v5 中**不再是 `*echo.HTTPError`**，只实现了 `echo.HTTPStatusCoder` 接口。自定义错误处理中如果这样写，404/405 会被当成 500：

```go
// v3 中无法匹配 echo.ErrNotFound
var he *echo.HTTPError
if errors.As(err, &he) { code = he.Code }

// v3 正确写法
code := echo.StatusCode(err) // 不是 HTTP 错误时返回 0
// 或
var sc echo.HTTPStatusCoder
if errors.As(err, &sc) { code = sc.StatusCode() }
```

bean 自带的 `berror.HTTPErrorHandlerFunc` 已按此方式处理。

### 4.4 自定义 Echo 全局错误处理器

bean 的 `berror.ErrorHandlerFunc`（`func(err error, c *echo.Context) (bool, error)`）**参数顺序不变**。
但如果直接替换了 `b.Echo.HTTPErrorHandler`，Echo 的签名参数顺序**调换**了：

```go
// v2
b.Echo.HTTPErrorHandler = func(err error, c echo.Context) { ... }

// v3
b.Echo.HTTPErrorHandler = func(c *echo.Context, err error) { ... }
```

判断响应是否已提交：

```go
// v2
if c.Response().Committed { return }

// v3
if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Committed { return }
```

### 4.5 `c.Response()` 返回 `http.ResponseWriter`

```go
// v2
c.Response().Writer
c.Response().Status
c.Response().Size
c.Response().Writer = myWriter

// v3
c.Response()                                   // 本身就是 http.ResponseWriter
r, _ := echo.UnwrapResponse(c.Response())      // 需要 Status / Size / Committed 时
r.Status; r.Size
c.SetResponse(myWriter)                        // 替换 writer
```

自定义 ResponseWriter 包装时建议实现 `Unwrap() http.ResponseWriter`，这样 `echo.UnwrapResponse` 和 `http.ResponseController` 才能找到底层 writer。

### 4.6 路由

```go
// v2
for _, r := range b.Echo.Routes() { ... }

// v3
for _, r := range b.Echo.Router().Routes() { ... }
```

`e.GET()` 等注册方法的返回值从 `*echo.Route` 变为 `echo.RouteInfo`（值类型）。`e.Reverse()`、`e.URI()`、`e.URL()`、`e.Host()` 已删除，改用 `e.Router().Routes().Reverse(name, params...)`。

> **由 `bean init` 生成的项目**：请把 `commands/route.go` 中的 `b.Echo.Routes()` 改为 `b.Echo.Router().Routes()`，否则 `route list` 和 `bean gen spec` 无法编译。

### 4.7 自定义 Binder / Renderer / Validator

```go
// Binder：参数顺序调换
// v2
func (b *MyBinder) Bind(i interface{}, c echo.Context) error
// v3
func (b *MyBinder) Bind(c *echo.Context, i interface{}) error

// Renderer：Context 放到第一个参数
// v2
func (r *MyRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error
// v3
func (r *MyRenderer) Render(c *echo.Context, w io.Writer, name string, data interface{}) error
```

Validator 接口（`Validate(i interface{}) error`）不变。

### 4.8 Echo 中间件

常见变化（完整列表见 Echo 的 [API_CHANGES_V5.md](https://github.com/labstack/echo/blob/master/API_CHANGES_V5.md)）：

| v4 | v5 |
| --- | --- |
| `middleware.Logger()` / `LoggerWithConfig` | 已删除，改用 `middleware.RequestLogger()` 或 bean 自带的访问日志 |
| `middleware.Timeout()` | 已删除，改用 `middleware.ContextTimeout()`（bean 已内置，见 `http.timeout`） |
| `middleware.BodyLimit("2M")` | `middleware.BodyLimit(2 * middleware.MB)`（参数为字节数） |
| `middleware.DefaultSkipper(c echo.Context)` | `middleware.DefaultSkipper(c *echo.Context)` |
| `RequestIDHandler func(echo.Context, string)` | `func(*echo.Context, string)` |
| `TrailingSlashConfig` | 拆分为 `AddTrailingSlashConfig` / `RemoveTrailingSlashConfig` |
| `middleware.DefaultCORSConfig` 等 `Default*Config` 变量 | 已删除，直接构造 Config |

### 4.9 测试代码

```go
// v2
c := e.NewContext(req, rec)                              // 返回 echo.Context
c.SetResponse(echo.NewResponse(w, e))
c.SetParamNames("id"); c.SetParamValues("1")

// v3
c := e.NewContext(req, rec)                              // 返回 *echo.Context
c.SetResponse(echo.NewResponse(w, e.Logger))             // 第二个参数改为 *slog.Logger
c.SetPathValues(echo.PathValues{{Name: "id", Value: "1"}})
```

### 4.10 其它常见编译错误

| 编译错误 | 处理方式 |
| --- | --- |
| `e.HideBanner` / `e.HidePort` / `e.Debug` undefined | 删除（bean 使用自己的 `http.Server` 启动，不会打印 banner） |
| `e.Start(...)` / `e.Shutdown(...)` | 由 bean 的 `ServeAt` 负责，项目中一般无需调用 |
| `c.ParamNames()` / `c.ParamValues()` | 改用 `c.PathValues()` |
| `echo.FormParam[T]` | 改名为 `echo.FormValue[T]` |
| `echo.PathParamsBinder(c)` | 改名为 `echo.PathValuesBinder(c)` |
| `echo.GetPath(r)` | 改用 `r.URL.RawPath` / `r.URL.Path` |

### 4.11 `bean.New()` / `bean.NewEcho()` 返回 error

v2 中配置错误（`http.trustIPRange`、`http.bodyLimit`、Sentry 初始化、Prometheus、协程池等）会在 `New()` 内部直接 `log.Fatal` 退出进程，无法在测试中验证，也不会清理已初始化的资源。v3 改为返回 error，并在出错时释放已初始化的资源（如已注册的协程池）：

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

`config.Bean` 未加载时返回 `bean.ErrConfigNotLoaded`。

`accessLog.skipEndpoints`、`prometheus.skipEndpoints`、`sentry.skipTracesEndpoints` 中的正则表达式无效时，v2 会 panic，v3 会从 `New()` 返回包含配置项名称的错误。

> **由 `bean init` 生成的项目**：`commands/start.go`、`commands/route.go`、`commands/gopher/gopher.go` 中的 `bean.New()` 需要按上面的方式处理 error。

### 4.12 配置项 `accessLog.bodyDumpMaskParam` 改名为 `accessLog.maskFields`

旧名称容易误解：`bodyDump` 让人以为它只对请求/响应体生效、而且只在开启 `bodyDump` 时生效；`Param` 让人以为它针对 URL 参数。实际上它会对所有结构化访问日志（`ACCESS`、`DUMP`、`OUTBOUND_API`）中匹配的字段脱敏，包括 `request_header` / `response_header`，与 `bodyDump` 是否开启无关。新名称 `maskFields` 与 `log.WithMaskFields` 一致。脱敏行为本身没有变化。

修改每个项目的 `env.json`：

```diff
 "accessLog": {
-  "bodyDumpMaskParam": ["password", "token"],
+  "maskFields": ["password", "token"],
 }
```

- 如果 `bodyDumpMaskParam` 仍然有值，**启动失败**并报错 `config: accessLog.bodyDumpMaskParam has been removed, rename it to accessLog.maskFields`，以免脱敏在不知不觉中失效。`config.LoadConfig` 和 `bean.New()` 都会做这项检查，所以即使项目自己用 viper 读取配置也会生效。
- 空的 `"bodyDumpMaskParam": []`（旧版项目模板生成的默认值）会被忽略，但建议删除或改名。
- Go 代码中读取 `config.Bean.AccessLog.BodyDumpMaskParam` 的地方改为 `config.Bean.AccessLog.MaskFields`。

> **由 `bean init` 生成的项目**：`commands/root.go` 现在用 `config.LoadConfig("env.json")` 加载配置，不再直接调用 viper。

## 5. bean 自身的行为变化

以下变化由 bean 内部处理，项目通常无需改代码，但请了解：

1. **路由名保持 v4 行为**：Echo v5 默认路由名为 `GET:/path`，bean 通过自定义 Router 仍以 handler 函数名命名（如 `myapp/handlers.(*userHandler).Get-fm`），`route list` 和 `gen spec` 的输出与 v2 一致。显式指定 `Name` 的路由不受影响。
2. **`http.bodyLimit` 配置格式不变**：仍可写 `"1M"`、`"512K"` 等，bean 会换算成字节数。配置值无法解析时启动失败。
3. **超时返回 504**：`http.timeout` 超时时返回 `echo.NewHTTPError(504, "gateway timeout").Wrap(context.DeadlineExceeded)`，上报到 Sentry 的仍是内部错误。
4. **404 / 405 改由 Echo 路由判断**：详见 [5.1](#51-404--405-的处理方式)。
5. **访问日志（body dump）内存有上限**：v2 会把整个请求体、响应体读入内存再截断，大文件上传/下载时内存占用与正文大小相同。v3 只在内存中保留前 `accessLog.bodyLimit + 1` 字节（默认 8KB），其余部分直接流式传给 handler / 客户端。日志字段和内容不变（截断本来就按 `accessLog.bodyLimit` 进行）。另外，客户端中途断开上传时，v2 会把已读到的部分当作完整请求体交给 handler，v3 中 handler 会收到读取错误；日志中的 `request_body_error` / `bytes_in_read` 照常记录。
6. **日志统一**：业务日志、Echo 日志、访问日志使用同一种 JSON 格式，详见 [4.2](#42-日志统一为-slog)。`log.Shutdown` 只关闭访问日志的异步写入，之后业务日志仍可正常输出。
7. **服务启停**：`ServeAt` 改用 Echo v5 的 `echo.StartConfig` 启动和优雅关闭，配置项（`http.keepAlive`、`http.shutdownTimeout`、`http.ssl.*`）含义不变；不设置读超时（与 v2 一致）；HTTPS 仍支持 HTTP/2，证书路径可以是绝对路径。`http.Server` 内部错误日志改为通过 Echo 的 slog 输出。`BeforeServe` 回调执行时，服务器的关闭函数已注册，可以在其中或之后安全地调用 `b.Shutdown()`。
8. **错误处理**：
   - `berror.APIError`（状态码取 `HTTPStatusCode`）和校验错误（400）实现了 `echo.HTTPStatusCoder`。Prometheus 指标等 Echo 中间件会记录它们的真实状态码，v2 中一律记为 500。
   - `berror.HTTPErrorHandlerFunc` 不再处理 `APIError` 和校验错误（它们有各自的处理函数），与注册顺序无关。
   - `http.errorMessage.*` 改为从 `config.Bean` 读取（启动时解析一次），不再在每次出错时查询 viper。`http.errorMessage.e405.html.file` 现在生效（v2 中 405 总是使用 `errors/html/405`）。JSON 配置中的 `value` 一律按字符串输出。
   - 配置结构中 5 个相同的匿名结构体合并为 `config.ErrorPage` 类型，字段名不变。
9. **`sentry.skipTracesEndpoints` 只配置一条时也生效**：v2 中只有配置两条及以上才会跳过采样（判断条件写成了 `> 1`）。
10. **项目模板**：`bean init` 生成的项目已修正多处在 v2 中就无法编译的问题（引用了已不存在的 `bean.BeanConfig`、`bean.Logger()`、`bean.SentryCaptureException`、`bean.DefaultBeforeSend`、`bean.Cleanup`，以及使用模块名而不是模块路径作为 import 路径等），改为使用 `config.Bean`、`log.Logger()`、`trace.SentryCaptureException`、`trace.DefaultBeforeSend`。
11. **发生 panic 的请求计入 Prometheus 指标**：v2 中 `Recover` 中间件注册在 Prometheus 中间件外层，panic 会直接穿过 Prometheus 中间件，导致这类请求在指标中完全缺失，5xx 错误率偏低。v3 把 `Recover` 移到 bean 内置中间件的最内层，panic 先被转换为 error，因此会以 `code="500"` 计入 `echo_requests_total` 等指标。
12. **panic 上报到 Sentry 时显示 panic 发生处的堆栈**：v2 中 panic 被转换为只带文本堆栈的错误，Sentry 无法解析，显示的是上报那一刻（错误处理器）的堆栈。v3 使用 bean 自己的 recover 中间件，panic 被转换为 `*berror.PanicError`：
    - `Value` 字段为 panic 的值；如果是 error（如 `runtime.Error`），可以通过 `errors.As` / `errors.Is` 取到。
    - 通过 `StackTrace()` 提供 panic 现场的堆栈，Sentry 中最内层的帧就是发生 panic 的函数。
    - `Error()` 只返回 `[PANIC RECOVER] <panic 的值>`，不再包含整段 goroutine 堆栈，访问日志的 `error` 字段因此变短；需要本地堆栈时用 `%+v` 格式化（未开启 Sentry 时错误日志就是这样输出的）。
    - 如果项目中用 `errors.As` 判断过 Echo 的 `*middleware.PanicStackError`，需要改为 `*berror.PanicError`。
13. **`env.json` 的 `http` 配置可以省略**：v2 中没有配置 `http.bodyLimit` 时启动会 panic。v3 为以下几项提供默认值（取自项目模板）：`http.host` = `0.0.0.0`、`http.port` = `8888`、`http.bodyLimit` = `1M`、`http.keepAlive` = `true`。注意：v2 中省略 `keepAlive` 等于 `false`（关闭 keep-alive），v3 中省略时为 `true`；如果确实要关闭，需要显式配置 `"keepAlive": false`。其它 `http` 配置项省略时的行为不变（例如不设置请求超时、`allowedMethod` 为空时允许所有方法）。
14. **`bean create service` 正确注入 repository**：v2 中 `bean create repo post` 后再执行 `bean create service post`，生成的 `NewPostService` 需要 repository 参数，但 `routers/route.go` 里写入的是无参调用 `services.NewPostService()`，导致编译失败；只传一个 `--repo` 且名字与 service 不同时，生成的 service 忽略了该 repository，`route.go` 却传入了它，同样无法编译。v3 中：不加 `--repo` 时注入同名 repository（存在的话）；加 `--repo` 时注入指定的 repository（不存在则先创建，重名自动去重），service 文件与 `route.go` 的调用始终一致。已生成的项目不受影响。
15. **HTML 模板渲染（`goview` / `echoview`）并发安全**：v2 的模板引擎在请求之间共享可变状态。并发渲染时，`{{include "..."}}` 可能拿**其他请求的数据**渲染（跨用户数据泄露）；`data["template"]` 指定的布局会写进共享配置，影响后续请求；同一个模板换一个布局渲染会报 `"templates/xxx" is undefined`。v3 中布局按请求区分，缓存按布局分开，每次渲染使用独立的模板副本，模板写法不变。另外：渲染出错时不再写出状态码和半截页面；`goview.M` 可以作为数据传入；`echoview.Render` 在中间件模式下使用传入的状态码（v2 总是 200）。
16. **`aes.BeanAESDecrypt` 校验 MAC**：v2 解密时不校验密文中的 `mac`，被篡改的密文也能"成功"解密（存在填充预言攻击的条件）。v3 会在解密前用常量时间比较校验 MAC，不匹配时返回 `aes.ErrInvalidMAC`。密文格式不变，`BeanAESEncrypt`（以及 Laravel `Encrypter`）生成的已有密文（例如租户数据库密码）可以照常解密；只有 `mac` 不正确的密文会被拒绝。格式不正确的输入返回 error 而不是 panic，空字符串也可以加密。
17. **数据库 TLS `hotReload` 校验服务器主机名**：v2 开启 `ssl.hotReload` 后不校验证书的主机名（MySQL 总是不校验；Redis / Mongo 用 IP 地址连接时不校验），任何受信任 CA 签发的证书都会被接受。v3 要求证书对连接使用的主机名有效；用 IP 地址连接时，证书需要对配置中的某个 `host` 有效（IP 匹配证书的 IP SAN）。如果数据库证书与配置的 `host` 不匹配，开启 `hotReload` 后会连接失败，需要修正证书或 `host`。注意：Redis 集群中运行时发现、以 IP 访问且不在配置 `host` 中的节点，其证书需要对配置的某个 `host` 有效。
18. **`helpers.DecodeJWT` 的默认校验始终生效**：v2 中只要传入任意 `opts`（例如 `jwt.WithAudience`），默认的"只允许 HS256""必须有 `exp`""严格解码"就全部失效，没有 `exp` 的 token 会永久有效。v3 中默认选项始终生效，`opts` 追加在后面（仍可用 `jwt.WithValidMethods` 覆盖允许的算法）；需要接受没有 `exp` 的 token 时请直接使用 `jwt.ParseWithClaims`。`helpers.ExtractJWTFromHeader` 要求 `Bearer` 后有空格（v2 中 `Bearerxabc` 也会取出 `abc`），scheme 不区分大小写。
19. **JSON 请求体错误返回 400**：v2 中请求体 JSON 语法错误、含未知字段或类型不匹配时，`c.Bind` 返回普通 error，客户端收到 **500**，并且错误被上报到 Sentry。v3 返回 400 的 `*berror.APIError`，响应为 `{"errorCode":"100001","errorMsg":"invalid JSON request body: ..."}`（`PROBLEM_PARSING_JSON`），不上报 Sentry。同时修正了误拒：嵌入结构体的字段、没有 json tag 的字段、绑定到 map 时的任意字段，v2 都会被当作未知字段拒绝；JSON 值后面带多余数据（如 `{"a":1} garbage`）现在会被拒绝。
20. **`async.ExecuteWithContext` / `ExecuteWithTimeout` 保留请求上下文**：v2 中这两个函数的任务上下文丢失了请求 ID（异步任务的日志没有 `id`）和请求的 Sentry trace，`ExecuteWithTimeout` 还让任务和请求共用同一个 Sentry hub。v3 中它们与 `ExecuteContext` 行为一致：上下文不随请求取消，带请求 ID、克隆的 hub 和延续的 trace。另外：未指定池名时在默认池中执行（v2 直接启动 goroutine），因此优雅关停时会等待它们；`ExecuteWithTimeout` 中任务返回的 error 除上报 Sentry 外也会记录日志。
21. **Redis**：`RPush` 在 v2 中每次调用都会失败（`can't marshal *[]string`），v3 修复，空列表为 no-op；`store/redis` 的 `MSetJSON` 在 v2 中不做 JSON 编码（传入 struct 会失败，写入的值也无法被 `GetJSON` 读取），v3 与 `SetJSON` 一致，对每个值做 `json.Marshal`。**注意**：如果项目为了绕开 v2 的问题，向 `MSetJSON` 传入了预先序列化好的 JSON 字符串（或 `[]byte`），v3 会再编码一次（字符串变成带引号的 JSON 字符串，`[]byte` 变成 base64），之后 `GetJSON` / `MGetJSON` 读出的数据就不对了，需要改为传入原始对象（struct、map 等）；缓存中已有的数据不受影响；集群模式下 `MGet` 有任意 key 不存在时 v2 整批返回 nil，v3 按 key 返回（不存在的为 nil），`Keys` 改为查询所有 master，多 key 的 `DelKey` 不再因 CROSSSLOT 失败。
22. **出站请求**：`transport/http` 的日志 Transport 在 `DumpBody` 时，关闭响应体不会关闭底层连接（连接泄漏），v3 修复；请求体只读取 `BodyLimit` 字节用于日志，读取出错时请求失败而不是发送截断的 body。`trace.PropagateToGRPC` 不再覆盖 context 中已有的 outgoing metadata（如 authorization）；`PropagateToHTTP` 重复调用不会产生重复的 header。
23. **bean CLI**：`bean gen spec` 对带域名的模块路径（如 `example.com/demo`）不再输出空的 `{}`；`bean test gen` 支持 PUT / PATCH；`bean test -o json|html` 在测试失败时退出码为 1（v2 中模块路径不含 `/` 时测试失败也返回 0，含 `/` 时报告无法写出），报告文件名中的 `/` 替换为 `_`（例如 `test_report_example.com_demo_<时间>.json`），CI 中依赖退出码或按文件名收集报告的地方需要相应调整；`bean create dockerfile` 生成可用的 Dockerfile / docker-compose.yml，并新增 `.air.toml`；`bean create handler|repo|service` 通过解析 `routers/route.go` 插入代码，无法插入时报错并提示需要手工添加的内容。

### 5.1 404 / 405 的处理方式

v2 用一个全局中间件在每个请求上遍历全部路由、自行匹配路径来判断 404 / 405。v3 删除了这个中间件，改为由 Echo 路由直接判断（返回 `echo.ErrNotFound` / `echo.ErrMethodNotAllowed`），再交给 bean 的错误处理器输出。

状态码规则与 v2 相同：

| 请求 | 结果 |
| --- | --- |
| 路径不存在 | 404 |
| 路径存在，但方法未注册 | 405（v3 起同时返回 `Allow` 头） |
| 路由的方法不在 `http.allowedMethod` 中 | 该路由**不会被注册**（启动时打印 WARN 日志），请求按上面两条返回 404 / 405 |
| `e.Any(...)` / `g.Any(...)` | 只为 `http.allowedMethod` 中的方法注册，其它方法返回 405；`route list` 中按方法逐条列出 |
| Group 下路径存在、方法不对 | 405（Echo v5 默认会因 Group 的兜底路由返回 404，bean 已关闭该行为） |

需要注意的变化：

1. **JSON 响应体**：增加了 `errorMsg` 字段，例如 404 为 `{"errorCode":"100003","errorMsg":"Not Found","errors":null}`（v2 为 `{"errorCode":"100003","errors":null}`）。`env.json` 中 `http.errorMessage.e404` / `e405` 的自定义 JSON **现在也对路由产生的 404 / 405 生效**（v2 中不生效）。
2. **HTML 响应**：同样使用 `errors/html/404`、`errors/html/405` 模板（或 `http.errorMessage.e404.html.file` 指定的文件），模板中的 `stacktrace` 变量为 `Not Found` / `Method Not Allowed`，v2 为空。
3. **即使没有注册 `berror.HTTPErrorHandlerFunc`**，路由产生的 404 / 405 也会按上述格式返回，不会变成 500。如果项目注册了自己的全局错误处理函数，它现在也会收到这类错误，可以用 `berror.IsRouterError(c)` 区分。
4. **404 / 405 请求会经过全局中间件**：v2 中这类请求在中间件处直接返回，不会进入访问日志、Request ID、Sentry、Prometheus 等中间件；v3 中会正常经过，因此访问日志中能看到 404 / 405 请求。Prometheus 中所有路由产生的 404 共用同一个 `url` 标签，避免扫描请求造成指标数量膨胀。这类请求不会再作为 ERROR 写入日志。
5. **Group 中间件不会对未知路径执行**：例如 `/api` Group 上的鉴权中间件，对 `/api/unknown` 不会执行，直接返回 404，与 v2 行为一致。
6. **`OPTIONS` 请求**：非 CORS 预检的 `OPTIONS` 请求，在路径存在时返回 `204` 并带 `Allow` 头（v2 中返回 405）。CORS 预检仍由 CORS 中间件处理。

## 6. 升级检查清单

- [ ] Go 版本 ≥ 1.25
- [ ] 执行第 3 节的批量替换
- [ ] `go.mod` 中只保留 `bean/v3`、`echo/v5`、`echo-contrib/v5`，并执行 `go mod tidy`
- [ ] 日志改为 slog 写法（`Debugf` 等可暂时保留，已废弃）；`Error(err)` 这类非 string 消息的调用需要修改
- [ ] 日志平台的解析规则、告警条件按新的日志字段调整（见 4.2）
- [ ] 检查所有 `*echo.HTTPError` 类型断言，改用 `echo.StatusCode(err)` / `echo.HTTPStatusCoder`
- [ ] 自定义 `b.Echo.HTTPErrorHandler` 调换参数顺序
- [ ] `commands/route.go` 中 `b.Echo.Routes()` 改为 `b.Echo.Router().Routes()`
- [ ] 自定义 Binder / Renderer 调整签名
- [ ] `bean.New()` / `bean.NewEcho()` 处理返回的 error
- [ ] `env.json` 中的 `accessLog.bodyDumpMaskParam` 改名为 `accessLog.maskFields`
- [ ] `env.json` 中没有配置 `http.keepAlive` 的项目：v3 默认开启 keep-alive（v2 默认关闭），需要关闭时显式配置 `"keepAlive": false`（见第 5 节第 13 条）
- [ ] 测试中的 `log.Init(echo.New().Logger)`、`echo.NewResponse(w, e)` 等改为 v3 写法
- [ ] `go build ./... && go test ./...` 通过
- [ ] 检查启动日志中是否有 `route is not registered because its method is not listed in http.allowedMethod` 警告
- [ ] 如果有客户端依赖 404 / 405 的 JSON 响应格式，确认新增的 `errorMsg` 字段没有影响
- [ ] 启动服务，验证 404 / 405 / 500 / 超时的响应和访问日志
- [ ] 开启了数据库 `ssl.hotReload` 的环境：确认证书对配置的 `host` 有效（见第 5 节第 17 条）
- [ ] 调用 `helpers.DecodeJWT` 并传入了 `opts` 的地方：确认 token 都带 `exp`（见第 5 节第 18 条）
- [ ] 如果有客户端依赖"请求体 JSON 错误时返回 500"，改为处理 400 和 `errorCode` `100001`（见第 5 节第 19 条）
- [ ] 搜索 `MSetJSON(` 的调用：确认传入的是原始对象，而不是预先序列化好的 JSON 字符串或 `[]byte`（见第 5 节第 21 条）
- [ ] CI 中使用 `bean test -o json|html` 的地方：测试失败时命令会返回 1；按新的报告文件名（`/` 替换为 `_`）收集报告（见第 5 节第 23 条）
