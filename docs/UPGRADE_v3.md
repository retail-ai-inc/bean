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

### 4.2 日志：`c.Logger()` / `e.Logger` 变为 `*slog.Logger`

Echo v5 删除了 `echo.Logger` 接口，`e.Logger` 和 `c.Logger()` 现在是标准库的 `*slog.Logger`，没有 `Debugf`、`Errorf`、`Infoj` 这类方法。

**推荐：业务代码统一使用 bean 的日志**（API 与 v2 完全一致）：

```go
import "github.com/retail-ai-inc/bean/v3/log"

log.Logger().Debugf("user %d logged in", userID)
log.Logger().Errorj(elog.JSON{"message": "failed", "error": err.Error()})
```

如果继续用 `c.Logger()`，需要改成 slog 写法：

```go
// v2
c.Logger().Errorf("failed to get user: %v", err)
c.Logger().Error(err)

// v3
c.Logger().Error("failed to get user", "error", err)
c.Logger().Error(err.Error())
```

> bean 已把 echo 的 slog 输出指向与 bean 日志相同的目标（`debugLogPath` 或标准输出），格式为 JSON，消息字段为 `message`。

`log` 包的相关变化：

| v2 | v3 |
| --- | --- |
| `log.BeanLogger` 内嵌 `echo.Logger` | 内嵌 `log.LeveledLogger`（方法集与原 `echo.Logger` 相同） |
| `log.Init(e.Logger)` | `log.Init(elog.New("echo"))`（`elog` 为 `github.com/labstack/gommon/log`） |
| `log.NewLogger(e.Logger, ...)` | `log.NewLogger(elog.New("echo"), ...)` |

一般项目不会自己调用 `log.Init`（`bean.New()` 内部已初始化），主要影响测试代码：

```go
// v2
_ = log.Init(echo.New().Logger)

// v3
_ = log.Init(elog.New("echo"))
```

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

## 5. bean 自身的行为变化

以下变化由 bean 内部处理，项目通常无需改代码，但请了解：

1. **路由名保持 v4 行为**：Echo v5 默认路由名为 `GET:/path`，bean 通过自定义 Router 仍以 handler 函数名命名（如 `myapp/handlers.(*userHandler).Get-fm`），`route list` 和 `gen spec` 的输出与 v2 一致。显式指定 `Name` 的路由不受影响。
2. **`http.bodyLimit` 配置格式不变**：仍可写 `"1M"`、`"512K"` 等，bean 会换算成字节数。配置值无法解析时启动失败。
3. **超时返回 504**：`http.timeout` 超时时返回 `echo.NewHTTPError(504, "gateway timeout").Wrap(context.DeadlineExceeded)`，上报到 Sentry 的仍是内部错误。
4. **404 / 405 改由 Echo 路由判断**：详见 [5.1](#51-404--405-的处理方式)。
5. **访问日志（body dump）内存有上限**：v2 会把整个请求体、响应体读入内存再截断，大文件上传/下载时内存占用与正文大小相同。v3 只在内存中保留前 `accessLog.bodyLimit + 1` 字节（默认 8KB），其余部分直接流式传给 handler / 客户端。日志字段和内容不变（截断本来就按 `accessLog.bodyLimit` 进行）。另外，客户端中途断开上传时，v2 会把已读到的部分当作完整请求体交给 handler，v3 中 handler 会收到读取错误；日志中的 `request_body_error` / `bytes_in_read` 照常记录。
6. **Echo 自身日志**：Echo v5 的 slog 日志（含 `c.Logger()`）与 bean 日志输出到同一目标，格式为 JSON，但字段与 gommon 日志略有不同（slog 为 `time` / `level` / `message`，没有 `prefix` / `file` / `line`）。

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
- [ ] `c.Logger().Xxxf(...)` 改为 `log.Logger().Xxxf(...)` 或 slog 写法
- [ ] 检查所有 `*echo.HTTPError` 类型断言，改用 `echo.StatusCode(err)` / `echo.HTTPStatusCoder`
- [ ] 自定义 `b.Echo.HTTPErrorHandler` 调换参数顺序
- [ ] `commands/route.go` 中 `b.Echo.Routes()` 改为 `b.Echo.Router().Routes()`
- [ ] 自定义 Binder / Renderer 调整签名
- [ ] 测试中的 `log.Init(echo.New().Logger)`、`echo.NewResponse(w, e)` 等改为 v3 写法
- [ ] `go build ./... && go test ./...` 通过
- [ ] 检查启动日志中是否有 `route is not registered because its method is not listed in http.allowedMethod` 警告
- [ ] 如果有客户端依赖 404 / 405 的 JSON 响应格式，确认新增的 `errorMsg` 字段没有影响
- [ ] 启动服务，验证 404 / 405 / 500 / 超时的响应和访问日志
