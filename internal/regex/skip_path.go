package regex

import (
	"errors"
	"regexp"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

var skipSampling func(path string) bool = func(path string) bool {
	return false // no skip by default
}

// SkipSamping checks if the path should be skipped for sampling.
// It returns false if the skipper is not set.
func SkipSampling(path string) bool {
	return skipSampling(path)
}

// SetSamplingPathSkipper updates the skipper for sampling.
func SetSamplingPathSkipper(skipPaths []string) {

	uniquePaths := make(map[string]struct{})
	for _, path := range skipPaths {
		uniquePaths[path] = struct{}{}
	}

	traceSkipPaths := make([]*regexp.Regexp, 0, len(uniquePaths))
	for path := range uniquePaths {
		traceSkipPaths = append(traceSkipPaths, regexp.MustCompile(path))
	}

	if len(traceSkipPaths) > 1 {
		skipSampling = func(path string) bool {
			for _, r := range traceSkipPaths {
				if r.MatchString(path) {
					return true
				}
			}
			return false
		}
	}
}

func InitAccessLogPathSkipper(skipPaths []string) func(c echo.Context) bool {
	uniquePaths := make(map[string]struct{})
	for _, path := range skipPaths {
		uniquePaths[path] = struct{}{}
	}

	accessLogSkipPaths := make([]*regexp.Regexp, 0, len(uniquePaths))

	for path := range uniquePaths {
		accessLogSkipPaths = append(accessLogSkipPaths, regexp.MustCompile(path))
	}

	return pathSkipper(accessLogSkipPaths)
}

func InitPrometheusPathSkipper(skipPaths []string, metricsPath string) (func(c echo.Context) bool, error) {

	if metricsPath == "" {
		return func(c echo.Context) bool { return false }, errors.New("metrics path is empty")
	}

	uniquePaths := make(map[string]struct{})
	for _, path := range skipPaths {
		uniquePaths[path] = struct{}{}
	}
	uniquePaths[metricsPath] = struct{}{}

	prometheusSkipPaths := make([]*regexp.Regexp, 0, len(uniquePaths))
	for path := range uniquePaths {
		prometheusSkipPaths = append(prometheusSkipPaths, regexp.MustCompile(path))
	}

	return pathSkipper(prometheusSkipPaths), nil
}

func pathSkipper(skipPathRegexes []*regexp.Regexp) func(c echo.Context) bool {

	if len(skipPathRegexes) == 0 {
		return echomiddleware.DefaultSkipper
	}

	return func(c echo.Context) bool {
		path := c.Request().URL.Path
		for _, r := range skipPathRegexes {
			if r.MatchString(path) {
				return true
			}
		}

		return false // no skip by default
	}
}
