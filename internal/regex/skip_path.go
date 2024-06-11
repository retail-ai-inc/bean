package regex

import (
	"regexp"
)

var traceSkipPaths []*regexp.Regexp

func CompileTraceSkipPaths(skipPaths []string) {
	for _, path := range skipPaths {
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

func CompileAccessLogSkipPaths(skipPaths []string) {
	for _, path := range skipPaths {
		AccessLogSkipPaths = append(AccessLogSkipPaths, regexp.MustCompile(path))
	}
}

var PrometheusSkipPaths []*regexp.Regexp

func CompilePrometheusSkipPaths(skipPaths []string) {
	for _, path := range skipPaths {
		PrometheusSkipPaths = append(PrometheusSkipPaths, regexp.MustCompile(path))
	}
}
