{{ .Copyright }}
package middleware

import (
	"fmt"
	"net/http"
	"time"

	"{{ .PkgPath }}/framework/internals/async"
	"{{ .PkgPath }}/framework/internals/latency"

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

// LatencyRecorder records the latency of each API endpoint.
func LatencyRecorder() echo.MiddlewareFunc {

	return func(next echo.HandlerFunc) echo.HandlerFunc {

		return func(c echo.Context) error {

			// Find the max ttl by intervals
			var ttl int

			intervals := viper.GetIntSlice("http.uriLatencyIntervals")
			for _, v := range intervals {
				if v > ttl {
					ttl = v
				}
			}

			req := c.Request()
			res := c.Response()
			start := time.Now()

			if err := next(c); err != nil {
				return err
			}

			stop := time.Now()

			// Only count successful response.
			if res.Status == http.StatusOK {
				l := stop.Sub(start)
				t := time.Now().Unix()
				uri := req.RequestURI
				key := fmt.Sprintf("%s_%d", uri, t)

				apiLatency := latency.Entry{
					Latency:   l,
					Timestamp: t,
				}

				// Async insert into badgerdb (key: uri, val: latency, ttl: ttl).
				async.Execute(func(ctx echo.Context) {
					latency.SetAPILatencyWithTTL(ctx, key, apiLatency, time.Duration(ttl)*time.Minute)
				})
			}

			return nil
		}
	}

}
