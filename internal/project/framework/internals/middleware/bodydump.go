package middleware

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/color"
	"github.com/labstack/gommon/log"
	"github.com/valyala/fasttemplate"
)

func BodyDumpWithCustomLogger(l *log.Logger) func(c echo.Context, reqBody, resBody []byte) {
	format := `{"time":"${time_rfc3339_nano}","id":"${id}","remote_ip":"${remote_ip}",` +
		`"host":"${host}","method":"${method}","uri":"${uri}","status":${status},` +
		`"request_body":${request_body},"response_body":${response_body}}` + "\n"
	customTimeFormat := "2006-01-02 15:04:05.00000"
	template := fasttemplate.New(format, "${", "}")
	colorer := color.New()
	colorer.SetOutput(l.Output())
	pool := &sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 256))
		},
	}

	return func(c echo.Context, reqBody, resBody []byte) {
		buf := pool.Get().(*bytes.Buffer)
		buf.Reset()
		defer pool.Put(buf)

		if _, err := template.ExecuteFunc(buf, func(w io.Writer, tag string) (int, error) {
			switch tag {
			case "time_unix":
				return buf.WriteString(strconv.FormatInt(time.Now().Unix(), 10))
			case "time_unix_nano":
				return buf.WriteString(strconv.FormatInt(time.Now().UnixNano(), 10))
			case "time_rfc3339":
				return buf.WriteString(time.Now().Format(time.RFC3339))
			case "time_rfc3339_nano":
				return buf.WriteString(time.Now().Format(time.RFC3339Nano))
			case "time_custom":
				return buf.WriteString(time.Now().Format(customTimeFormat))
			case "id":
				id := c.Request().Header.Get(echo.HeaderXRequestID)
				if id == "" {
					id = c.Response().Header().Get(echo.HeaderXRequestID)
				}
				return buf.WriteString(id)
			case "remote_ip":
				return buf.WriteString(c.RealIP())
			case "host":
				return buf.WriteString(c.Request().Host)
			case "uri":
				return buf.WriteString(c.Request().RequestURI)
			case "method":
				return buf.WriteString(c.Request().Method)
			case "path":
				p := c.Request().URL.Path
				if p == "" {
					p = "/"
				}
				return buf.WriteString(p)
			case "status":
				n := c.Response().Status
				s := colorer.Green(n)
				switch {
				case n >= 500:
					s = colorer.Red(n)
				case n >= 400:
					s = colorer.Yellow(n)
				case n >= 300:
					s = colorer.Cyan(n)
				}
				return buf.WriteString(s)
			case "request_body":
				if len(reqBody) > 0 {
					return buf.Write(reqBody)
				}
				return buf.WriteString(`""`)
			case "response_body":
				return buf.WriteString(strings.TrimSuffix(string(resBody), "\n"))
			}
			return 0, nil
		}); err != nil {
			c.Logger().Error(err)
			return
		}

		if l.Output() == nil {
			c.Logger().Output().Write(buf.Bytes())
			return
		}
		l.Output().Write(buf.Bytes())
	}
}
