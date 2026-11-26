package regex

import (
	"errors"
	"fmt"
	"regexp"
	"sync/atomic"

	"github.com/labstack/echo/v5"
	echomiddleware "github.com/labstack/echo/v5/middleware"
)

// samplingSkipper is global because it is also used outside the request scope (`async`, `sync` packages).
var samplingSkipper atomic.Pointer[pathMatcher]

// SkipSampling checks if the path should be skipped for sampling.
// It returns false if the skipper is not set.
func SkipSampling(path string) bool {
	m := samplingSkipper.Load()
	return m != nil && m.match(path)
}

// SetSamplingPathSkipper sets the path patterns (regular expressions) whose traces are not sampled.
func SetSamplingPathSkipper(skipPaths []string) error {
	m, err := newPathMatcher("sentry.skipTracesEndpoints", skipPaths)
	if err != nil {
		return err
	}
	samplingSkipper.Store(m)
	return nil
}

func InitAccessLogPathSkipper(skipPaths []string) (func(c *echo.Context) bool, error) {
	m, err := newPathMatcher("accessLog.skipEndpoints", skipPaths)
	if err != nil {
		return nil, err
	}
	return pathSkipper(m), nil
}

func InitPrometheusPathSkipper(skipPaths []string, metricsPath string) (func(c *echo.Context) bool, error) {
	if metricsPath == "" {
		return func(c *echo.Context) bool { return false }, errors.New("metrics path is empty")
	}

	m, err := newPathMatcher("prometheus.skipEndpoints", append(append([]string{}, skipPaths...), metricsPath))
	if err != nil {
		return nil, err
	}
	return pathSkipper(m), nil
}

// pathMatcher matches a path against a set of regular expressions.
type pathMatcher struct {
	regexes []*regexp.Regexp
}

func newPathMatcher(configKey string, patterns []string) (*pathMatcher, error) {
	seen := make(map[string]struct{}, len(patterns))
	m := &pathMatcher{regexes: make([]*regexp.Regexp, 0, len(patterns))}
	for _, p := range patterns {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}

		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid %s pattern %q: %w", configKey, p, err)
		}
		m.regexes = append(m.regexes, re)
	}
	return m, nil
}

func (m *pathMatcher) match(path string) bool {
	for _, re := range m.regexes {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

func pathSkipper(m *pathMatcher) func(c *echo.Context) bool {
	if len(m.regexes) == 0 {
		return echomiddleware.DefaultSkipper
	}

	return func(c *echo.Context) bool {
		return m.match(c.Request().URL.Path)
	}
}
