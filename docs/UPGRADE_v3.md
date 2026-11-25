# Bean v2 → v3 升级指南

[English](UPGRADE_v3.en.md) | 中文

Bean v3 的核心变化是把底层 Web 框架从 **Echo v4** 升级到 **Echo v5**。Echo v5 本身有大量不兼容改动（`Context` 从接口变为结构体、日志改用 `log/slog`、错误类型调整等），因此 bean 也随之升级大版本，模块路径改为 `github.com/retail-ai-inc/bean/v3`。

借这次大版本升级，bean 还统一了日志、重做了 404 / 405 的处理，并修复了一批 v2 中就存在的问题（其中包括几个安全问题）。本文完整记录这些改动，以及基于 bean v2 的项目升级到 v3 时**必须修改**和**需要核对**的地方。

**如何阅读本文**：先看第 1 节的概览，按第 3 节的步骤升级；第 4、5 节是必须处理的代码和配置改动；第 6 节逐项说明行为变化，每一项都标明了"变化 / 影响 / 需要做什么"，没有标"需要做什么"的项目无需处理；最后用第 10 节的检查清单核对。

- [1. 改动概览](#1-改动概览)
- [2. 环境要求](#2-环境要求)
- [3. 升级步骤](#3-升级步骤)
- [4. 必须修改的代码](#4-必须修改的代码)
- [5. 配置变更（env.json）](#5-配置变更envjson)
- [6. 行为变化](#6-行为变化)
- [7. bean CLI 与项目模板](#7-bean-cli-与项目模板)
- [8. 新增功能](#8-新增功能)
- [9. 公开 API 变更一览](#9-公开-api-变更一览)
- [10. 升级检查清单](#10-升级检查清单)

---

## 1. 改动概览

| 类别 | 主要内容 | 下游项目需要做什么 | 详见 |
| --- | --- | --- | --- |
| 依赖与模块路径 | `bean/v3`、`echo/v5`、`echo-contrib/v5`、sentry-go/echo v0.50+、go-redis v9、mongo-driver v2；Go 1.25+ | 更新 `go.mod`，执行批量替换 | [2](#2-环境要求)、[3](#3-升级步骤) |
| 编译不兼容 | `*echo.Context`、`HTTPError`、`bean.New()` 返回 error、日志 API、Binder / Renderer 签名、路由 API、Redis / MongoDB 驱动类型等 | **必须修改代码**（使用 Mongo 的代码需要按 mongo-driver v2 迁移） | [4](#4-必须修改的代码) |
| 配置 | `accessLog.bodyDumpMaskParam` 改名；`http` 配置可省略但 `keepAlive` 默认值改变；新增 `debugLogLevel` | **必须修改 `env.json`**（改名），核对默认值 | [5](#5-配置变更envjson) |
| 日志格式 | 应用日志、Echo 日志与访问日志统一为同一种 JSON 格式，字段名改变 | 调整日志平台的解析规则和告警 | [6.2](#62-日志) |
| HTTP 响应 | 404 / 405 改由路由判断并增加 `errorMsg`；JSON 请求体错误由 500 改为 400 | 核对依赖响应格式的客户端 | [6.1](#61-http-响应与错误处理) |
| 安全修复 | 模板渲染跨请求串数据、AES 不校验 MAC、TLS 热加载不校验主机名、JWT 默认校验失效、日志明文记录凭据 | 核对证书、JWT 用法 | [6.4](#64-安全修复) |
| 数据库与缓存 | Redis 若干方法修复、读副本降级、初始化失败时释放连接、租户配置校验、Mongo 超时 | 核对 `MSetJSON` 用法、集群 `MGet` 返回值 | [6.5](#65-数据库与缓存) |
| 异步任务与出站请求 | 异步任务保留请求上下文并统一在池中执行；出站日志不再阻塞流式响应、不再泄漏连接 | 确认出站响应体会被关闭；替换已废弃的 `ExecuteWithTimeout` | [6.6](#66-异步任务)、[6.7](#67-出站请求) |
| bean CLI | `gen spec`、`test`、`create *` 多处修复；所有命令出错时返回退出码 1；生成的代码经过 gofmt | CI 中 `bean test` 的退出码和报告文件名 | [7](#7-bean-cli-与项目模板) |
| 废弃的工具函数 | 可由标准库替代的 `helpers` / `str` 函数标记为 Deprecated | 可选：按提示替换 | [6.10](#610-已废弃的工具函数) |
| 新增功能 | `helpers.ProcessCSV`、`log.WithLevel` 等 | 可选使用 | [8](#8-新增功能) |

## 2. 环境要求

| 项目 | v2 | v3 |
| --- | --- | --- |
| Go | 1.24+ | **1.25+**（Echo v5 要求），bean 自身 `go.mod` 为 1.26 |
| bean | `github.com/retail-ai-inc/bean/v2` | `github.com/retail-ai-inc/bean/v3` |
| Echo | `github.com/labstack/echo/v4` | `github.com/labstack/echo/v5` |
| echo-contrib | `github.com/labstack/echo-contrib` | `github.com/labstack/echo-contrib/v5` |
| sentry-go/echo | ≤ v0.44.x | **v0.50.0+**（基于 Echo v5） |
| gommon | `github.com/labstack/gommon`（日志） | 不再依赖 |
| Redis 驱动 | `github.com/go-redis/redis/v8` | `github.com/redis/go-redis/v9`，见 [4.13](#413-redis--mongodb-驱动升级) |
| MongoDB 驱动 | `go.mongodb.org/mongo-driver`（v1） | `go.mongodb.org/mongo-driver/v2`，见 [4.13](#413-redis--mongodb-驱动升级) |
| `github.com/pkg/errors`、`github.com/json-iterator/go` | 依赖 | 不再依赖，见 [4.14](#414-不再依赖-githubcompkgerrors) |

## 3. 升级步骤

**第 1 步：更新依赖**

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

**第 2 步：批量替换**

在项目根目录执行以下替换（macOS / Linux 均可用 `perl`）：

```bash
find . -type f -name '*.go' -not -path './vendor/*' -print0 | xargs -0 perl -pi -e 's#retail-ai-inc/bean/v2\b#retail-ai-inc/bean/v3#g; s#labstack/echo/v4#labstack/echo/v5#g; s#labstack/echo-contrib/#labstack/echo-contrib/v5/#g; s/(?<![\*\w.])echo\.Context\b/*echo.Context/g; s#\becho\.Map\b#map[string]any#g; s#github.com/go-redis/redis/v8#github.com/redis/go-redis/v9#g; s#go.mongodb.org/mongo-driver/(?!v2/)#go.mongodb.org/mongo-driver/v2/#g'
```

它完成了：

| 替换前 | 替换后 |
| --- | --- |
| `github.com/retail-ai-inc/bean/v2/...` | `github.com/retail-ai-inc/bean/v3/...` |
| `github.com/labstack/echo/v4` | `github.com/labstack/echo/v5` |
| `github.com/labstack/echo-contrib/xxx` | `github.com/labstack/echo-contrib/v5/xxx` |
| `func(c echo.Context) error` | `func(c *echo.Context) error` |
| `echo.Map{...}` | `map[string]any{...}` |
| `github.com/go-redis/redis/v8` | `github.com/redis/go-redis/v9`（见 4.13） |
| `go.mongodb.org/mongo-driver/...` | `go.mongodb.org/mongo-driver/v2/...`（还需要手工修改，见 4.13） |

**第 3 步：修复编译错误**

执行 `go build ./... && go vet ./...`，剩余的编译错误按[第 4 节](#4-必须修改的代码)处理。由 `bean init` 生成的项目，还需要按 [4.12](#412-由-bean-init-生成的项目需要的修改) 修改几个生成的文件。

**第 4 步：修改配置**

按[第 5 节](#5-配置变更envjson)修改每个环境的 `env.json`。

**第 5 步：测试与核对**

执行 `go test ./...`，再按[第 6 节](#6-行为变化)逐项核对行为变化，最后用[第 10 节](#10-升级检查清单)的检查清单确认。

## 4. 必须修改的代码

本节的改动不处理就无法编译（或在启动时失败）。

### 4.1 Handler / 中间件签名

`echo.Context` 变成了结构体，所有 handler、中间件、bean 的错误处理函数都要用指针：

```go
// v2
func (h *userHandler) Get(c echo.Context) error { ... }

// v3
func (h *userHandler) Get(c *echo.Context) error { ... }
```

bean 中接收 Context 的公开函数签名同步变为 `*echo.Context`，用法不变：
`trace.StartSpanWithEcho`、`trace.SentryCaptureExceptionWithEcho`、`trace.SentryCaptureMessageWithEcho`、`async.ExecuteWithContext`、`helpers.DecodeJWT`、`helpers.ExtractJWTFromHeader`、`helpers.PostDataStripTags`、`echoview.Render`，以及 `berror.ErrorHandlerFunc` 类型和 `berror.APIErrorHandlerFunc`、`HTTPErrorHandlerFunc`、`ValidationErrorHandlerFunc`、`DefaultErrorHandlerFunc`。

### 4.2 日志统一为 slog

v2 中有三套日志：bean 的应用日志（gommon logger）、Echo 日志、访问日志，格式各不相同。v3 统一到标准库 `log/slog`：

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

日志格式的变化见 [6.2](#62-日志)。

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

Validator 接口（`Validate(i interface{}) error`）不变。bean 自带的 `echoview.ViewEngine.Render` 也已改为新签名。

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
| `*middleware.PanicStackError`（Recover 中间件产生的错误） | bean 使用自己的 recover 中间件，panic 变为 `*berror.PanicError`，见 [6.3](#63-监控prometheus--sentry) |

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
| `config.Bean.AccessLog.BodyDumpMaskParam` undefined | 改为 `config.Bean.AccessLog.MaskFields`，见 [5.1](#51-accesslogbodydumpmaskparam-改名为-accesslogmaskfields) |
| `config.Bean.HTTP.ErrorMessage.E404` 等的匿名结构体类型 | 合并为 `config.ErrorPage` 类型，字段名不变 |

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

- `config.Bean` 未加载时返回 `bean.ErrConfigNotLoaded`。
- `accessLog.skipEndpoints`、`prometheus.skipEndpoints`、`sentry.skipTracesEndpoints` 中的正则表达式无效时，v2 会 panic，v3 会从 `New()` 返回包含配置项名称的错误。
- `env.json` 中还留有已删除的配置项时返回错误，见 [5.1](#51-accesslogbodydumpmaskparam-改名为-accesslogmaskfields)。

### 4.12 由 `bean init` 生成的项目需要的修改

v2 的 `bean init` 生成的项目中，以下文件需要修改：

| 文件 | 修改 |
| --- | --- |
| `commands/start.go`、`commands/route.go`、`commands/gopher/gopher.go` | `bean.New()` 按 [4.11](#411-beannew--beannewecho-返回-error) 处理 error |
| `commands/route.go` | `b.Echo.Routes()` 改为 `b.Echo.Router().Routes()`，否则 `route list` 和 `bean gen spec` 无法编译 |
| `commands/root.go` | 建议改用 `config.LoadConfig("env.json")` 加载配置（v3 模板的写法），它会检查已删除的配置项并设置默认值 |
| 引用了 `bean.BeanConfig`、`bean.Logger()`、`bean.SentryCaptureException`、`bean.DefaultBeforeSend`、`bean.Cleanup` 的文件 | 这些在 v2 中就已不存在（v2 模板生成的项目本身无法编译），改为 `config.Bean`、`log.Logger()`、`trace.SentryCaptureException`、`trace.DefaultBeforeSend`；`bean.Cleanup` 删除即可 |
| handler / middleware 中的日志调用 | 按 [4.2](#42-日志统一为-slog) 改为 `...Context` 写法 |

也可以用 v3 的 `bean init` 生成一个新项目作为参照。

### 4.13 Redis / MongoDB 驱动升级

bean v3 使用的驱动：

| 驱动 | v2 | v3 |
| --- | --- | --- |
| Redis | `github.com/go-redis/redis/v8`（已停止维护） | `github.com/redis/go-redis/v9` |
| MongoDB | `go.mongodb.org/mongo-driver`（v1） | `go.mongodb.org/mongo-driver/v2` |

`DBDeps` 中的 `MasterRedisDB` / `TenantRedisDBs`（`redis.UniversalClient`）、`store/redis` 返回的 `redis.Pipeliner` / `*redis.Script`，以及 `MasterMongoDB` / `TenantMongoDBs`（`*mongo.Client`）都是新驱动的类型，项目中直接使用这些类型的代码需要迁移：

- **go-redis v9**：基本只需替换 import 路径（第 3 节的命令已包含），常用方法的签名不变。go-redis v9 建立连接时会自动重试拨号。
- **mongo-driver v2**：除了 import 路径，还需要手工修改，常见的有：
  - `bson/primitive` 包并入 `bson`：`primitive.ObjectID` → `bson.ObjectID`，`primitive.M` / `primitive.D` → `bson.M` / `bson.D`；
  - `mongo.Connect(ctx, opts)` → `mongo.Connect(opts)`；
  - 选项构造函数返回 builder：声明为 `*options.FindOptions` 等类型的变量或参数改为 `*options.FindOptionsBuilder` 等；
  - `SingleResult.DecodeBytes()` → `Raw()`，`Collection.Distinct` 返回 `*mongo.DistinctResult`。

  完整的变化见官方迁移指南：<https://github.com/mongodb/mongo-go-driver/blob/v2.0.0/docs/migration-2.0.md>。

### 4.14 不再依赖 `github.com/pkg/errors`

bean 内部不再使用 `github.com/pkg/errors`（已归档），附带堆栈的错误改由新增的 `stacktrace.WithStack` 创建：`%+v` 仍会打印堆栈，Sentry 仍能取到堆栈，`errors.Is` / `errors.As` / `errors.Unwrap` 可以照常使用。但 `pkg/errors` 的 `errors.Cause(err)` 无法再解开 bean 返回的错误，请改用 `errors.Is` / `errors.As`。

`helpers/json` 改为直接使用标准库 `encoding/json`（不再依赖 `github.com/json-iterator/go`），并已标记为 Deprecated，建议直接使用 `encoding/json`。

## 5. 配置变更（env.json）

### 5.1 `accessLog.bodyDumpMaskParam` 改名为 `accessLog.maskFields`

- **变化**：旧名称容易误解：`bodyDump` 让人以为它只对请求/响应体生效、而且只在开启 `bodyDump` 时生效；`Param` 让人以为它针对 URL 参数。实际上它会对所有结构化访问日志（`ACCESS`、`DUMP`、`OUTBOUND_API`）中匹配的字段脱敏，包括 `request_header` / `response_header`，与 `bodyDump` 是否开启无关。新名称 `maskFields` 与 `log.WithMaskFields` 一致。脱敏行为本身没有变化。
- **影响**：如果 `bodyDumpMaskParam` 仍然有值，**启动失败**并报错 `config: accessLog.bodyDumpMaskParam has been removed, rename it to accessLog.maskFields`，以免脱敏在不知不觉中失效。`config.LoadConfig` 和 `bean.New()` 都会做这项检查，所以即使项目自己用 viper 读取配置也会生效。空的 `"bodyDumpMaskParam": []`（旧版项目模板生成的默认值）会被忽略，但建议删除或改名。
- **需要做什么**：修改每个环境的 `env.json`；Go 代码中读取 `config.Bean.AccessLog.BodyDumpMaskParam` 的地方改为 `config.Bean.AccessLog.MaskFields`。

```diff
 "accessLog": {
-  "bodyDumpMaskParam": ["password", "token"],
+  "maskFields": ["password", "token"],
 }
```

### 5.2 `http` 配置可以省略，`keepAlive` 默认值改变

- **变化**：v2 中没有配置 `http.bodyLimit` 时启动会 panic，没有配置 `http.port` 时会监听随机端口。v3 为以下几项提供默认值（取自项目模板）：

  | 配置项 | 默认值 |
  | --- | --- |
  | `http.host` | `0.0.0.0` |
  | `http.port` | `8888` |
  | `http.bodyLimit` | `1M` |
  | `http.keepAlive` | `true` |

  其它 `http` 配置项省略时的行为不变（例如不设置请求超时、`allowedMethod` 为空时允许所有方法）。
- **影响**：v2 中省略 `keepAlive` 等于 `false`（关闭 keep-alive），v3 中省略时为 `true`。
- **需要做什么**：没有配置 `http.keepAlive` 的项目，如果确实要关闭 keep-alive，需要显式配置 `"keepAlive": false`。

### 5.3 新增 `debugLogLevel`

- **变化**：新配置项 `debugLogLevel`：应用日志的最低级别，`debug`（默认）/ `info` / `warn` / `error`，不影响访问日志。应用日志的输出位置仍由 `debugLogPath` 决定。
- **需要做什么**：可选。生产环境如果不需要 debug 日志，可以设置为 `info`。

### 5.4 其它配置项的变化

| 配置项 | 变化 |
| --- | --- |
| `http.bodyLimit` | 格式不变，仍可写 `"1M"`、`"512K"` 等，bean 会换算成字节数（规则与 v2 相同：`1M` = 1,000,000 字节，`1Mi` = 1,048,576 字节）。配置值无法解析时启动失败。 |
| `http.errorMessage.e405.html.file` | 现在生效（v2 中 405 总是使用 `errors/html/405`）。 |
| `http.errorMessage.*.json` | `value` 一律按字符串输出。配置改为启动时解析一次，不再在每次出错时查询 viper。`e404` / `e405` 的自定义 JSON 现在也对路由产生的 404 / 405 生效，见 [6.1.1](#611-404--405-改由-echo-路由判断)。 |
| `accessLog.skipEndpoints` / `prometheus.skipEndpoints` / `sentry.skipTracesEndpoints` | 正则表达式无效时启动返回错误（v2 会 panic）。`sentry.skipTracesEndpoints` 只配置一条时也生效（v2 中只有配置两条及以上才会跳过采样，判断条件写成了 `> 1`）。 |
| `http.keepAlive` / `http.shutdownTimeout` / `http.ssl.*` | 含义不变，见 [6.8](#68-服务启停与路由)。 |
| `database.*.ssl.hotReload` | 开启后会校验证书的主机名，见 [6.4.3](#643-数据库-tls-hotreload-校验服务器主机名)。 |
| `database.mongo.connectTimeout` | 现在也用于启动时的连接和 ping，见 [6.5.5](#655-mongodb)。 |

## 6. 行为变化

以下变化由 bean 内部处理，大多数项目无需改代码；标有"需要做什么"的项目请逐项核对。

### 6.1 HTTP 响应与错误处理

#### 6.1.1 404 / 405 改由 Echo 路由判断

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

**需要做什么**：

- 检查启动日志中是否有 `route is not registered because its method is not listed in http.allowedMethod` 警告，这说明有路由使用了未允许的方法，没有被注册。
- 如果有客户端依赖 404 / 405 的 JSON 响应格式，确认新增的 `errorMsg` 字段没有影响。

#### 6.1.2 JSON 请求体错误返回 400

- **变化**：v2 中请求体 JSON 语法错误、含未知字段或类型不匹配时，`c.Bind` 返回普通 error，客户端收到 **500**，并且错误被上报到 Sentry。v3 返回 400 的 `*berror.APIError`，响应为 `{"errorCode":"100001","errorMsg":"invalid JSON request body: ..."}`（`PROBLEM_PARSING_JSON`），不上报 Sentry。
- **同时修正的误拒**：嵌入结构体的字段、没有 json tag 的字段、绑定到 map 时的任意字段，v2 都会被当作未知字段拒绝（`UnknownFieldsError`），v3 会正常接受。字段名的大小写仍需与 json tag 完全一致。
- **新增的拒绝**：JSON 值后面带多余数据（如 `{"a":1} garbage`）现在会被拒绝。
- **其它**：请求体只解析一次（v2 解析三次），`Bind` 之后仍可再次读取请求体。
- **需要做什么**：如果有客户端依赖"请求体 JSON 错误时返回 500"，改为处理 400 和 `errorCode` `100001`；如果代码按字符串 `"UnknownFieldsError"` 判断过错误，改为用 `errors.As` 判断 `*berror.APIError`。`berror.APIErrorHandlerFunc` 必须在错误处理器中注册（bean 默认已注册）。

#### 6.1.3 超时返回 504

`http.timeout` 超时时返回 `echo.NewHTTPError(504, "gateway timeout").Wrap(context.DeadlineExceeded)`，上报到 Sentry 的仍是内部错误。

#### 6.1.4 bean 错误处理器的调整

- `berror.APIError`（状态码取 `HTTPStatusCode`）和校验错误（400）实现了 `echo.HTTPStatusCoder`。Prometheus 指标等 Echo 中间件会记录它们的真实状态码，v2 中一律记为 500。
- `berror.HTTPErrorHandlerFunc` 不再处理 `APIError` 和校验错误（它们有各自的处理函数），与注册顺序无关。
- `http.errorMessage.*` 改为从 `config.Bean` 读取（启动时解析一次），见 [5.4](#54-其它配置项的变化)。
- 配置结构中 5 个相同的匿名结构体合并为 `config.ErrorPage` 类型，字段名不变。

### 6.2 日志

#### 6.2.1 日志格式

应用日志、Echo 日志、访问日志使用同一种 JSON 格式（代码写法见 [4.2](#42-日志统一为-slog)）：

| | v2 | v3 |
| --- | --- | --- |
| 应用日志 | `{"time":…,"level":"INFO","prefix":"echo","file":"x.go","line":"12","message":…}` | `{"timestamp":…,"severity":"INFO","message":…,"file":"pkg/x.go","line":12,…}` |
| Echo / `c.Logger()` 日志 | `{"time":…,"level":"INFO","message":…}` | 同上 |
| 访问日志 | `{"timestamp":…,"severity":"INFO","level":"ACCESS",…}` | 不变 |

- `severity` 取值：`DEBUG`、`INFO`、`WARNING`（slog 的 Warn）、`ERROR`、`CRITICAL`（兼容层的 `Fatal*` / `Panic*`）。
- 应用日志中的 `level` 字段不再出现；访问日志中的 `level` 仍是 `ACCESS` / `DUMP` / `OUTBOUND_API` 等分类。
- `line` 由字符串变为数字，`file` 包含所在目录（`pkg/x.go`）。
- 使用 `...Context` 方法时自动带上请求 ID（`id`）和 trace ID。
- `log.Shutdown` 只关闭访问日志的异步写入，之后应用日志仍可正常输出。

**需要做什么**：如果日志平台的解析规则、索引或告警条件依赖 `time` / `level` / `prefix` 等字段名，需要同步调整。

#### 6.2.2 访问日志（body dump）的内存有上限

- **变化**：v2 会把整个请求体、响应体读入内存再截断，大文件上传/下载时内存占用与正文大小相同。v3 只在内存中保留前 `accessLog.bodyLimit + 1` 字节（默认 8KB），其余部分直接流式传给 handler / 客户端。日志字段和内容不变（截断本来就按 `accessLog.bodyLimit` 进行）。
- **影响**：客户端中途断开上传时，v2 会把已读到的部分当作完整请求体交给 handler，v3 中 handler 会收到读取错误；日志中的 `request_body_error` / `bytes_in_read` 照常记录。

### 6.3 监控（Prometheus / Sentry）

- **发生 panic 的请求计入 Prometheus 指标**：v2 中 `Recover` 中间件注册在 Prometheus 中间件外层，panic 会直接穿过 Prometheus 中间件，导致这类请求在指标中完全缺失，5xx 错误率偏低。v3 把 `Recover` 移到 bean 内置中间件的最内层，panic 先被转换为 error，因此会以 `code="500"` 计入 `echo_requests_total` 等指标。
- **`APIError` / 校验错误按真实状态码计入指标**（v2 中一律为 500），见 [6.1.4](#614-bean-错误处理器的调整)。
- **404 共用一个 `url` 标签**，见 [6.1.1](#611-404--405-改由-echo-路由判断)。
- **panic 上报到 Sentry 时显示 panic 发生处的堆栈**：v2 中 panic 被转换为只带文本堆栈的错误，Sentry 无法解析，显示的是上报那一刻（错误处理器）的堆栈。v3 使用 bean 自己的 recover 中间件，panic 被转换为 `*berror.PanicError`：
  - `Value` 字段为 panic 的值；如果是 error（如 `runtime.Error`），可以通过 `errors.As` / `errors.Is` 取到。
  - 通过 `StackTrace()` 提供 panic 现场的堆栈，Sentry 中最内层的帧就是发生 panic 的函数。
  - `Error()` 只返回 `[PANIC RECOVER] <panic 的值>`，不再包含整段 goroutine 堆栈，访问日志的 `error` 字段因此变短；需要本地堆栈时用 `%+v` 格式化（未开启 Sentry 时错误日志就是这样输出的）。
- **`sentry.skipTracesEndpoints` 只配置一条时也生效**，见 [5.4](#54-其它配置项的变化)。
- **需要做什么**：如果项目中用 `errors.As` 判断过 Echo 的 `*middleware.PanicStackError`，需要改为 `*berror.PanicError`。5xx 错误率的告警阈值可能需要重新评估（panic 的请求现在会被计入）。

### 6.4 安全修复

以下问题在 v2 中就存在，v3 已修复。

#### 6.4.1 HTML 模板渲染（`goview` / `echoview`）并发安全

- **变化**：v2 的模板引擎在请求之间共享可变状态。并发渲染时，`{{include "..."}}` 可能拿**其他请求的数据**渲染（跨用户数据泄露）；`data["template"]` 指定的布局会写进共享配置，影响后续请求；同一个模板换一个布局渲染会报 `"templates/xxx" is undefined`。v3 中布局按请求区分，缓存按布局分开，每次渲染使用独立的模板副本，模板写法不变。
- **其它**：渲染出错时不再写出状态码和半截页面；`goview.M` 可以作为数据传入；`echoview.Render` 在中间件模式下使用传入的状态码（v2 总是 200）。
- **需要做什么**：如果有页面依赖了"上一个请求设置的布局会沿用到下一个请求"这一 bug，需要在数据中显式传入 `"template"`。

#### 6.4.2 `aes.BeanAESDecrypt` 校验 MAC

- **变化**：v2 解密时不校验密文中的 `mac`，被篡改的密文也能"成功"解密（存在填充预言攻击的条件）。v3 会在解密前用常量时间比较校验 MAC，不匹配时返回 `aes.ErrInvalidMAC`。
- **兼容性**：密文格式不变，`BeanAESEncrypt`（以及 Laravel `Encrypter`）生成的已有密文（例如租户数据库密码）可以照常解密；只有 `mac` 不正确的密文会被拒绝。
- **其它**：格式不正确的输入返回 error 而不是 panic，空字符串也可以加密。
- **需要做什么**：通常无需处理。如果代码用 `recover` 兜过解密的 panic，可以改为直接判断 error。

#### 6.4.3 数据库 TLS `hotReload` 校验服务器主机名

- **变化**：v2 开启 `ssl.hotReload` 后不校验证书的主机名（MySQL 总是不校验；Redis / Mongo 用 IP 地址连接时不校验），任何受信任 CA 签发的证书都会被接受。v3 要求证书对连接使用的主机名有效；用 IP 地址连接时，证书需要对配置中的某个 `host` 有效（IP 匹配证书的 IP SAN）。
- **影响**：如果数据库证书与配置的 `host` 不匹配，开启 `hotReload` 后会**连接失败**。Redis 集群中运行时发现、以 IP 访问且不在配置 `host` 中的节点，其证书需要对配置的某个 `host` 有效。
- **需要做什么**：开启了 `ssl.hotReload` 的环境，确认证书对配置的 `host` 有效，必要时修正证书或 `host`。

#### 6.4.4 `helpers.DecodeJWT` 的默认校验始终生效

- **变化**：v2 中只要传入任意 `opts`（例如 `jwt.WithAudience`），默认的"只允许 HS256""必须有 `exp`""严格解码"就全部失效，没有 `exp` 的 token 会永久有效。v3 中默认选项始终生效，`opts` 追加在后面（仍可用 `jwt.WithValidMethods` 覆盖允许的算法）。
- **`helpers.ExtractJWTFromHeader`**：要求 `Bearer` 后有空格（v2 中 `Bearerxabc` 也会取出 `abc`），scheme 不区分大小写。
- **影响**：传入了 `opts` 的调用，没有 `exp` 或使用其它算法签名的 token 会返回 `ErrJWTokenInvalid`；`Authorization` 头不规范的请求取不到 token。
- **需要做什么**：调用 `DecodeJWT` 并传入了 `opts` 的地方，确认 token 都带 `exp` 且用 HS256 签名（或显式传入 `jwt.WithValidMethods`）；需要接受没有 `exp` 的 token 时请直接使用 `jwt.ParseWithClaims`。

#### 6.4.5 日志中不再明文记录凭据

- **变化**：出站请求日志（`OUTBOUND_API`）中，`Authorization`、`Proxy-Authorization`、`Cookie`、`Set-Cookie` 以及名称包含 token / secret / password / api-key / apikey / session 的请求头和响应头，即使配置在允许记录的头列表中，日志中的值也会替换为 `[REDACTED]`。
- **其它**：memory 数据库的 `delKeyAPI` 端点改用常量时间比较 bearer token。

### 6.5 数据库与缓存

#### 6.5.1 Redis 方法修复

| 方法 | v2 的问题 | v3 |
| --- | --- | --- |
| `RPush` | 每次调用都失败（`can't marshal *[]string`） | 正常工作；空列表为 no-op |
| `store/redis` 的 `MSetJSON` | 不做 JSON 编码：传入 struct 会失败，写入的值也无法被 `GetJSON` / `MGetJSON` 读取 | 与 `SetJSON` 一致，对每个值做 `json.Marshal` |
| `store/redis` 的 `MGetJSON` | 用字符串拼接构造 JSON 数组，复杂度 O(n²) | 改用缓冲区 |
| 集群模式 `MGet` | 有任意 key 不存在时整批返回 nil；不存在的 key 返回 `""` | 按 key 返回，不存在的为 nil（与单机模式一致） |
| 集群模式 `Keys` | 只在一个随机节点上执行 | 在所有 master 上执行 |
| 集群模式多 key 的 `DelKey` | 多个 key 不在同一个 slot 时报 CROSSSLOT | 用 pipeline 逐个删除 |
| 集群模式 `MSet` / `MSetWithTTL` | key 不是字符串时 panic | 返回 `ErrRedisInvalidParameter` |
| `store/redis` 的 `HGets`（缓存设置了前缀时） | 返回的 map 以加了前缀的 Redis key 为键（传入 `h1` 得到 `app_h1`），`result["h1"]` 总是空 | 以调用方传入的 key 为键 |
| `store/redis` 租户不存在时的错误 | 普通 error | 包装了 `redis.ErrTenantNotFound`，可以用 `errors.Is` 判断（错误信息不变） |

**需要做什么**：

- 如果项目为了绕开 v2 的问题，向 `MSetJSON` 传入了预先序列化好的 JSON 字符串（或 `[]byte`），v3 会再编码一次（字符串变成带引号的 JSON 字符串，`[]byte` 变成 base64），之后 `GetJSON` / `MGetJSON` 读出的数据就不对了，需要改为传入原始对象（struct、map 等）；缓存中已有的数据不受影响。
- 集群模式下用 `MGet` 的结果是否为 `nil` 判断"未命中"的代码，需要改为逐个判断元素。
- 使用 `store/redis` 的 `HGets` 并按带前缀的 key 取值的地方，改为按传入的 key 取值。

#### 6.5.2 Redis 读副本

- **变化**：在读副本上未命中（`redis.Nil`）不再到主库重查（v2 中每次未命中都会多查一次主库）；`WRONGTYPE` 等服务端错误直接返回，不再被主库的结果掩盖；只有连接错误（以及副本返回 `LOADING` / `MASTERDOWN`）才转到主库，并且该副本在 10 秒内不再使用（v2 中副本宕机时每次读取都要先等拨号超时）。
- **其它**：`HGets` 不再因为某个字段不存在而掩盖其它命令的网络错误；`SRandMemberN` 在副本失败时也会转到主库。

#### 6.5.3 初始化失败时释放连接

初始化中途失败时（某个租户连接失败、Redis 读副本连接失败、ping 失败等），已经建立的连接会先关闭再返回 error（v2 中会泄漏）。`InitDB` 中途失败时，之前已初始化的数据库也会被关闭。

#### 6.5.4 租户连接配置的校验

- **变化**：`TenantConnections.Connections` 中的字段类型不正确或缺失时，启动返回带租户 ID 的 error，而不是 panic；`"port": 3306` 这样的数字也被接受。空字符串仍然允许（与 v2 相同）。
- **需要做什么**：如果有租户的配置写错，v3 启动时会明确报错，按错误信息修正对应租户的配置即可。

#### 6.5.5 MongoDB

- 初次连接和 ping 使用 `mongo.connectTimeout`（未设置时为 10s；v2 固定为 10s，如果配置了较小的值，启动时会更早超时）。
- `host` 为多个主机（`a,b`）时每个主机都使用配置的端口（v2 中只有最后一个）。
- 关闭时能正常结束会话（v2 中关闭时使用的 context 已被取消）。
- `maxConnectionLifeTime` 仍对应驱动的最大空闲时间（mongo 驱动没有连接最长寿命的选项）。
- **需要做什么**：`mongo.connectTimeout` 配置了较小值的环境，确认启动时的连接能在该时间内完成。

#### 6.5.6 TLS 证书文件监听

开启 `ssl.hotReload` 时，同一证书文件的所有连接共用一个文件监听，连接关闭时停止（v2 中每个连接各起一个，永不停止）。

#### 6.5.7 memory 缓存

`CloseMemory` 可以重复调用（v2 第二次调用会永久阻塞），关闭后再调用 `memory.NewMemoryCache()` 会得到新的缓存；通配符删除（`DelMemory("prefix*")`）改为线性时间（v2 是指数级的回溯匹配）。

### 6.6 异步任务

- **`async.ExecuteWithContext` / `ExecuteWithTimeout` 保留请求上下文**：v2 中这两个函数的任务上下文丢失了请求 ID（异步任务的日志没有 `id`）和请求的 Sentry trace，`ExecuteWithTimeout` 还让任务和请求共用同一个 Sentry hub（任务中设置的 user / tag 会影响请求）。v3 中它们与 `ExecuteContext` 行为一致：上下文不随请求取消，带请求 ID、克隆的 hub 和延续的 trace。
- **在协程池中执行**：这两个函数未指定池名时在默认池中执行（v2 直接启动 goroutine），因此优雅关停时会等待它们（最长 `asyncPoolReleaseTimeout`）；关停开始后提交的任务会被拒绝并记录错误日志。
- **错误日志**：`ExecuteWithTimeout` 中任务返回的 error 除上报 Sentry 外也会记录日志（`Async task failed.`）。
- **协程池的释放**：释放协程池时不再阻塞池中任务查询协程池；默认池在服务关闭后被释放，同一进程中再次创建 bean 实例时会自动恢复。
- **`async.Execute` 也在池中执行**：未指定池名时在默认池中执行（v2 直接启动 goroutine），优雅关停时会等待它。
- **已废弃**：`async.ExecuteWithTimeout` 和 `async.CaptureException` 标记为 Deprecated，请改用 `async.ExecuteContext(ctx, fn, async.WithTimeout(d))` 和 `trace.SentryCaptureException`（仍可使用）。
- **panic 日志**：异步任务 panic 时，日志中包含堆栈（v2 只记录 panic 的值，未开启 Sentry 时堆栈丢失）。
- **`sync` 包**：`sync.NewPool` / `NewResultPool` 的任务使用克隆的 Sentry hub（带请求信息），v2 中实际用的是调用方的 hub；span 的描述改为调用方的函数名。
- **需要做什么**：有长时间运行的异步任务的服务，确认优雅关停的时间（`asyncPoolReleaseTimeout`）足够；留意新增的 `Async task failed.` 错误日志；可选：把 `ExecuteWithTimeout` 替换为 `ExecuteContext`。

### 6.7 出站请求

- **连接泄漏**：`transport/http` 的日志 Transport 在 `DumpBody` 时，关闭响应体不会关闭底层连接（连接泄漏），v3 修复；101 响应（WebSocket 升级）不再被包装。
- **请求体**：只读取 `BodyLimit` 字节用于日志（v2 读取整个请求体）；读取出错时请求失败，而不是发送截断的 body。
- **流式响应**：开启 `DumpBody` 时，`OUTBOUND_API` 日志改为在响应体读到末尾、读取出错或被关闭时输出（v2 在返回响应前先读取最多 `BodyLimit` 字节，SSE / 长轮询等流式响应会被阻塞）；读取出错时增加 `response_body_error` 字段。
- **凭据脱敏**：见 [6.4.5](#645-日志中不再明文记录凭据)。
- **追踪透传**：`trace.PropagateToGRPC` 不再覆盖 context 中已有的 outgoing metadata（如 authorization）；`PropagateToHTTP` 重复调用不会产生重复的 header，header 为 nil 时也不会 panic。
- **其它**：`NewLoggingTransport` 在未加载 bean 配置时不再 panic。
- **需要做什么**：使用日志 Transport 并开启 `DumpBody` 的地方，确认响应体都被读完或关闭，否则不会输出 `OUTBOUND_API` 日志（不关闭响应体本身就会泄漏连接）。

### 6.8 服务启停与路由

- **服务启停**：`ServeAt` 改用 Echo v5 的 `echo.StartConfig` 启动和优雅关闭，配置项（`http.keepAlive`、`http.shutdownTimeout`、`http.ssl.*`）含义不变；不设置读超时（与 v2 一致）；HTTPS 仍支持 HTTP/2，证书路径可以是绝对路径。`http.Server` 内部错误日志改为通过 Echo 的 slog 输出。`BeforeServe` 回调执行时，服务器的关闭函数已注册，可以在其中或之后安全地调用 `b.Shutdown()`；收到退出信号的处理在服务启动前就已注册。
- **路由名保持 v4 行为**：Echo v5 默认路由名为 `GET:/path`，bean 通过自定义 Router 仍以 handler 函数名命名（如 `myapp/handlers.(*userHandler).Get-fm`），`route list` 和 `gen spec` 的输出与 v2 一致。显式指定 `Name` 的路由不受影响。

### 6.9 工具函数

- `str.Substring` 按字符（rune）计算长度（v2 中用字节长度判断，处理中文时会返回多余的字符甚至 panic）。
- `str.LeftPadToLength` / `RightPadToLength` 在 `padStr` 为空、长度为负、填充不足时不再 panic，正常输入的结果不变。
- `helpers.JitterBackoff` 在 `min` 小于 2ns 时不再 panic。
- `helpers.StructStripTags` 不再丢失大整数（超过 2^53）的精度。

### 6.10 已废弃的工具函数

以下函数标记为 Deprecated，仍可使用，可按提示替换为标准库：

| 函数 | 替代 |
| --- | --- |
| `helpers.Max` | 内置 `max` |
| `helpers.FloatInRange` | `min(max(i, lo), hi)` |
| `helpers.Int` / `helpers.Float` | `cmp.Ordered` |
| `helpers.HasStringInSlice` / `HasTargetInSlice` | `slices.Contains` / `ContainsFunc` |
| `helpers.FindStringInSlice` | `slices.Index` |
| `helpers.DeleteStringFromSlice` | `slices.Delete` |
| `str.Contains` / `str.IsEqualsAny` | `slices.Contains` |
| `str.StringToPointer` | `new(s)`（Go 1.26） |
| `helpers/json` 包 | `encoding/json` |

`helpers.Join` 不废弃（它能连接元素为自定义 string 类型的切片，`strings.Join` 做不到）。

## 7. bean CLI 与项目模板

| 命令 | v2 的问题 | v3 |
| --- | --- | --- |
| `bean init` | 生成的项目无法编译（引用了已不存在的 `bean.BeanConfig`、`bean.Logger()` 等，使用模块名而不是模块路径作为 import 路径） | 生成的项目可以直接编译运行 |
| `bean create service` | `bean create repo post` 后再 `bean create service post`，`route.go` 中写入无参的 `NewPostService()`，编译失败；只传一个 `--repo` 时 service 忽略了它，`route.go` 却传入了它 | 不加 `--repo` 时注入同名 repository（存在的话）；加 `--repo` 时注入指定的 repository（不存在则先创建，重名自动去重），service 文件与 `route.go` 的调用始终一致 |
| `bean create handler / repo / service` | 按行号向 `routers/route.go` 插入代码，位置差一行（结构体为空时插到 `}` 之外），找不到插入点时静默跳过 | 解析 `routers/route.go` 后插入并 gofmt；无法插入时报错并提示需要手工添加的内容 |
| `bean create dockerfile` | 生成的文件不可用（air 的模块已改名、CMD 使用模块路径、容器名不合法、compose 文件未关闭） | 生成可用的 Dockerfile / docker-compose.yml，并新增 `.air.toml`；Go 版本取自项目的 go.mod，端口取自 env.json |
| `bean gen spec` | 模块路径带域名（如 `example.com/demo`）时输出空的 `{}`；应用向 stdout 输出日志时解析失败 | 正确输出 |
| `bean test gen` | spec 中有 PUT / PATCH 路由时整体失败 | 支持 PUT / PATCH，其它方法使用 resty 的 `Execute` |
| `bean test -o json\|html` | 模块路径不含 `/` 时测试失败也返回 0；含 `/` 时报告文件写不出来 | 测试失败时退出码为 1；报告文件名中的 `/` 替换为 `_`（例如 `test_report_example.com_demo_<时间>.json`） |
| 所有命令 | 出错时打印错误后直接退出进程，部分错误的退出码为 0 | 出错时返回错误，以退出码 1 结束，不再打印整页用法说明 |
| `bean init` / `bean create *` | 生成的 Go 文件没有经过 gofmt；在已有项目中执行 `bean init` 会先覆盖 Makefile、.gitignore 再失败 | 生成的 Go 文件都经过 gofmt；在已有 `env.json` 的目录中执行 `bean init` 会直接报错 |
| `bean create command` | 变量名直接用命令名，`type`、`fmt` 等名字生成的代码无法编译；名字为空时 panic | 变量名改为 `<name>Cmd`；Go 关键字和 `root` 不能作为命令名；名字为空时报错 |

**需要做什么**：

- 重新安装 CLI：`go install github.com/retail-ai-inc/bean/v3/cmd/bean@latest`。
- CI 中使用 `bean test -o json|html` 的地方：测试失败时命令会返回 1；按新的报告文件名收集报告。
- 已生成的项目不受这些修复影响；需要的话可以删除旧的 Dockerfile 等文件后重新生成。

## 8. 新增功能

| 功能 | 说明 |
| --- | --- |
| `helpers.ProcessCSV` / `ProcessCSVFile` | 并发处理大型 CSV 文件：按批次交给多个 worker 处理，内存占用有上限，遇到第一个错误、panic 或 ctx 取消时停止。详见 [README](README.md#processing-large-csv-files)。 |
| `helpers.ParseByteSize` | 解析 `"1M"`、`"512Ki"` 等大小字符串，规则与 `http.bodyLimit` 相同。 |
| `log.WithLevel` / `log.ParseLevel` / `log.SetLogger` / `log.LevelCritical` | 设置应用日志级别、解析级别字符串、替换全局 logger。 |
| `BeanLogger.SetLevel` / `Level` | 运行时调整应用日志级别。 |
| `berror.PanicError` | panic 转换成的错误，带 panic 现场的堆栈，见 [6.3](#63-监控prometheus--sentry)。 |
| `berror.IsRouterError` | 判断错误是否由路由产生（404 / 405）。 |
| `berror.APIError.StatusCode` | `APIError` 实现了 `echo.HTTPStatusCoder`。 |
| `aes.ErrInvalidMAC` | 密文的 MAC 不匹配时返回。 |
| `stacktrace.WithStack` | 给错误附加调用方的堆栈（替代 `pkg/errors.WithStack`），见 [4.14](#414-不再依赖-githubcompkgerrors)。 |
| `redis.ErrTenantNotFound`（`store/redis`） | 租户没有 Redis 连接时返回的错误。 |
| `bean.ErrConfigNotLoaded` | `config.Bean` 未加载时 `New()` / `NewEcho()` 返回。 |
| `config.ErrorPage` / `ErrorPage.JSONBody` | 错误页面配置的类型。 |
| `config.CheckRemovedKeys` | 检查 `env.json` 中是否还有已删除的配置项（`LoadConfig` 和 `New()` 已自动调用）。 |
| `config.SetDefaults` 与 `config.DefaultHTTPHost` 等常量 | `http` 配置的默认值，见 [5.2](#52-http-配置可以省略keepalive-默认值改变)。 |
| `debugLogLevel` 配置项 | 见 [5.3](#53-新增-debugloglevel)。 |

## 9. 公开 API 变更一览

**签名变更**（除下表外，所有接收 `echo.Context` 的函数都改为接收 `*echo.Context`，见 [4.1](#41-handler--中间件签名)）：

| v2 | v3 |
| --- | --- |
| `bean.New() *Bean` | `bean.New() (*Bean, error)` |
| `bean.NewEcho() (*echo.Echo, func() error)` | `bean.NewEcho() (*echo.Echo, func() error, error)` |
| `log.Init(logger echo.Logger) BeanLogger` | `log.Init(w io.Writer) (*BeanLogger, error)` |
| `log.Logger() BeanLogger` | `log.Logger() *BeanLogger` |
| `log.NewLogger(elogger echo.Logger, options ...LoggerOptions) (*logger, error)` | `log.NewLogger(w io.Writer, options ...LoggerOptions) (*BeanLogger, error)` |
| `type log.BeanLogger interface` | `type log.BeanLogger struct`（内嵌 `*slog.Logger`） |
| `echoview.ViewEngine.Render(w, name, data, c)` | `echoview.ViewEngine.Render(c, w, name, data)` |

**删除**：

| 删除的 API | 替代 |
| --- | --- |
| `BeanLogger` 接口上的 `SetOutput` / `Output` / `SetPrefix` / `Prefix` / `SetHeader` / `SetLevel(elog.Lvl)` 等 gommon 方法 | 见 [4.2](#42-日志统一为-slog) |
| `config.Bean.AccessLog.BodyDumpMaskParam` | `config.Bean.AccessLog.MaskFields` |
| 404 / 405 中间件（`MethodNotAllowedAndRouteNotFound`，内部） | Echo 路由，见 [6.1.1](#611-404--405-改由-echo-路由判断) |

**新增**：见[第 8 节](#8-新增功能)。

**依赖变化**：go-redis v8 → `github.com/redis/go-redis/v9`，mongo-driver v1 → v2（见 [4.13](#413-redis--mongodb-驱动升级)）；不再依赖 `github.com/pkg/errors`、`github.com/json-iterator/go`（见 [4.14](#414-不再依赖-githubcompkgerrors)）。

**已废弃（仍可使用）**：

- `log.Logger()` 上的 `Print*`、`Debugf/Debugj`、`Infof/Infoj`、`Warnf/Warnj`、`Errorf/Errorj`、`Fatal*`、`Panic*`，计划在 v4 删除；
- `async.ExecuteWithTimeout`、`async.CaptureException`，见 [6.6](#66-异步任务)；
- 可由标准库替代的工具函数，见 [6.10](#610-已废弃的工具函数)。

## 10. 升级检查清单

**编译**

- [ ] Go 版本 ≥ 1.25
- [ ] 执行第 3 节的批量替换
- [ ] `go.mod` 中只保留 `bean/v3`、`echo/v5`、`echo-contrib/v5`，并执行 `go mod tidy`
- [ ] 日志改为 slog 写法（`Debugf` 等可暂时保留，已废弃）；`Error(err)` 这类非 string 消息的调用需要修改
- [ ] 检查所有 `*echo.HTTPError` 类型断言，改用 `echo.StatusCode(err)` / `echo.HTTPStatusCoder`
- [ ] 自定义 `b.Echo.HTTPErrorHandler` 调换参数顺序
- [ ] `commands/route.go` 中 `b.Echo.Routes()` 改为 `b.Echo.Router().Routes()`
- [ ] 自定义 Binder / Renderer 调整签名
- [ ] `bean.New()` / `bean.NewEcho()` 处理返回的 error
- [ ] 测试中的 `log.Init(echo.New().Logger)`、`echo.NewResponse(w, e)` 等改为 v3 写法
- [ ] 用 `errors.As` 判断 `*middleware.PanicStackError` 的地方改为 `*berror.PanicError`
- [ ] 使用 Redis 类型的代码改为 `github.com/redis/go-redis/v9`；使用 Mongo 的代码按 mongo-driver v2 迁移（见 4.13）
- [ ] 使用 `pkg/errors` 的 `errors.Cause` 解开 bean 返回的错误的地方，改用 `errors.Is` / `errors.As`（见 4.14）
- [ ] `go build ./... && go test ./...` 通过

**配置**

- [ ] `env.json` 中的 `accessLog.bodyDumpMaskParam` 改名为 `accessLog.maskFields`
- [ ] `env.json` 中没有配置 `http.keepAlive` 的项目：v3 默认开启 keep-alive（v2 默认关闭），需要关闭时显式配置 `"keepAlive": false`（见 5.2）
- [ ] 开启了数据库 `ssl.hotReload` 的环境：确认证书对配置的 `host` 有效（见 6.4.3）
- [ ] `mongo.connectTimeout` 配置了较小值的环境：该值现在也用于启动时的连接和 ping（见 6.5.5）

**代码核对**

- [ ] 调用 `helpers.DecodeJWT` 并传入了 `opts` 的地方：确认 token 都带 `exp`（见 6.4.4）
- [ ] 搜索 `MSetJSON(` 的调用：确认传入的是原始对象，而不是预先序列化好的 JSON 字符串或 `[]byte`（见 6.5.1）
- [ ] 集群模式下用 `MGet` 的结果是否为 `nil` 判断未命中的代码，改为逐个判断（见 6.5.1）
- [ ] 使用 `store/redis` 的 `HGets` 并按带前缀的 key 取值的地方，改为按传入的 key 取值（见 6.5.1）
- [ ] 使用 `transport/http` 日志 Transport 并开启 `DumpBody` 的地方：确认响应体都被读完或关闭，否则不会输出 `OUTBOUND_API` 日志（见 6.7）
- [ ] 依赖"上一个请求的布局会沿用"的页面，显式传入 `"template"`（见 6.4.1）

**运行验证**

- [ ] 检查启动日志中是否有 `route is not registered because its method is not listed in http.allowedMethod` 警告
- [ ] 启动服务，验证 404 / 405 / 500 / 超时的响应和访问日志
- [ ] 有长时间运行的异步任务的服务：确认优雅关停的时间足够（见 6.6）

**客户端与运维**

- [ ] 日志平台的解析规则、告警条件按新的日志字段调整（见 6.2.1）
- [ ] 如果有客户端依赖 404 / 405 的 JSON 响应格式，确认新增的 `errorMsg` 字段没有影响
- [ ] 如果有客户端依赖"请求体 JSON 错误时返回 500"，改为处理 400 和 `errorCode` `100001`（见 6.1.2）
- [ ] Prometheus 5xx 告警阈值：panic 的请求和 `APIError` 的真实状态码现在都会被计入（见 6.3）
- [ ] CI 中使用 `bean test -o json|html` 的地方：测试失败时命令会返回 1；按新的报告文件名（`/` 替换为 `_`）收集报告（见第 7 节）
