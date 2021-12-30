{{ .Copyright }}
package middleware

import (
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	ierror "{{ .PkgPath }}/framework/internals/error"
	"{{ .PkgPath }}/framework/internals/latency"

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

// ServerStats is an in-mermory data struct which stored different status of the server.
type ServerStats struct {
	Uptime       time.Time           `json:"uptime"`
	RequestCount uint64              `json:"requestCount"`
	Latency      map[string][]string `json:"latency"`
	mutex        sync.RWMutex
}

// NewServerStats returns a `ServerStats`, normally being called in the server start up process.
func NewServerStats() *ServerStats {
	return &ServerStats{
		Uptime:  time.Now(),
		Latency: map[string][]string{},
	}
}

// GetServerStats is the handler of `/route/stats`.
func (s *ServerStats) GetServerStats(c echo.Context) error {

	adminClientID := viper.GetString("admin.clientId")
	adminClientSecret := viper.GetString("admin.clientSecret")

	requestClientID := c.Request().Header.Get("Client-Id")
	requestClientSecret := c.Request().Header.Get("Client-Secret")

	// Only admin a.k.a retail ai devops should able to hit this endpoint
	if adminClientID == requestClientID && adminClientSecret == requestClientSecret {

		allowedMethod := viper.GetStringSlice("http.allowedMethod")
		intervals := viper.GetIntSlice("http.uriLatencyIntervals")

		s.mutex.RLock()
		defer s.mutex.RUnlock()

		m := latency.GetAllAPILatency(c)

		for _, r := range c.Echo().Routes() {

			if r.Path == "/" {
				continue
			}

			if strings.Contains(r.Name, "glob..func1") {
				continue
			}

			// XXX: IMPORTANT - `allowedMethod` has to be a sorted slice.
			i := sort.SearchStrings(allowedMethod, r.Method)

			if i >= len(allowedMethod) || allowedMethod[i] != r.Method {
				continue
			}

			reducedLatency := latencyMapReduce(m, r.Path, intervals)
			latencyHuman := make([]string, len(intervals))

			for i, l := range reducedLatency {
				latencyHuman[i] = l.String()
			}

			s.Latency[r.Path] = latencyHuman
		}

		// -------------- TEMP: also returning "/ping" stats --------------
		pingLatency := latencyMapReduce(m, "/ping", intervals)
		latencyHuman := make([]string, len(intervals))
		for i, l := range pingLatency {
			latencyHuman[i] = l.String()
		}
		s.Latency["/ping"] = latencyHuman

		return c.JSON(http.StatusOK, s)
	}

	return c.JSON(http.StatusUnauthorized, map[string]interface{}{
		"errorCode": ierror.UNAUTHORIZED_ACCESS,
		"errors":    nil,
	})

}

func latencyMapReduce(m map[string]latency.Entry, uri string, intervals []int) []time.Duration {

	mappedEntries := []latency.Entry{}

	// Map the same uri request entries into one slice.
	for k, e := range m {
		if strings.HasPrefix(k, uri) {
			mappedEntries = append(mappedEntries, e)
		}
	}

	// Reduce the records to different intervals according to the config.
	// Example: [5mins, 10mins, 15mins]
	reducedAvg := make([]time.Duration, len(intervals))

	count := make([]int, len(intervals))

	for _, e := range mappedEntries {
		timestamp := time.Unix(e.Timestamp, 0)
		now := time.Now()

		for idx, interval := range intervals {
			if now.Sub(timestamp) < time.Minute*time.Duration(interval) {
				// Calculate cumulative latency in every iteration to avoid overflow.
				reducedAvg[idx] = reducedAvg[idx] + (e.Latency-reducedAvg[idx])/time.Duration(count[idx]+1)
				count[idx]++
			}
		}
	}

	return reducedAvg
}
