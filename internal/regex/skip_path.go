package regex

import (
	"fmt"
	"regexp"
)

var traceSkipPaths []*regexp.Regexp

func CompileTraceSkipPaths(skipPaths []string, addPaths ...string) {
	uniquePaths := make(map[string]struct{})
	for _, path := range skipPaths {
		uniquePaths[path] = struct{}{}
	}
	for _, path := range addPaths {
		uniquePaths[path] = struct{}{}
	}
	for path := range uniquePaths {
		traceSkipPaths = append(traceSkipPaths, regexp.MustCompile(path))
	}
}

// MatchAnyTraceSkipPath checks if the path should be skipped from tracing.
// It returns false if compiling is not done beforehand and regexes are empty.
func MatchAnyTraceSkipPath(path string) bool {
	if len(traceSkipPaths) == 0 {
		return false
	}

	for _, r := range traceSkipPaths {
		if r.MatchString(path) {
			return true
		}
	}

	return false
}

var AccessLogSkipPaths []*regexp.Regexp

func CompileAccessLogSkipPaths(skipPaths []string, addPaths ...string) {
	uniquePaths := make(map[string]struct{})
	for _, path := range skipPaths {
		uniquePaths[path] = struct{}{}
	}
	for _, path := range addPaths {
		uniquePaths[path] = struct{}{}
	}
	for path := range uniquePaths {
		AccessLogSkipPaths = append(AccessLogSkipPaths, regexp.MustCompile(path))
	}
}

var PrometheusSkipPaths []*regexp.Regexp

func CompilePrometheusSkipPaths(skipPaths []string, metricsPath string) error {

	if metricsPath == "" {
		return fmt.Errorf("metrics path is empty")
	}

	uniquePaths := make(map[string]struct{})
	for _, path := range skipPaths {
		uniquePaths[path] = struct{}{}
	}
	uniquePaths[metricsPath] = struct{}{}
	for path := range uniquePaths {
		PrometheusSkipPaths = append(PrometheusSkipPaths, regexp.MustCompile(path))
	}
	return nil
}
