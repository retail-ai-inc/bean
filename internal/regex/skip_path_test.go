package regex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetSamplingPathSkipper(t *testing.T) {
	t.Cleanup(func() { samplingSkipper.Store(nil) })

	assert.False(t, SkipSampling("/health"), "nothing is skipped by default")

	// A single pattern must work (it was ignored before).
	require.NoError(t, SetSamplingPathSkipper([]string{"^/health$"}))
	assert.True(t, SkipSampling("/health"))
	assert.False(t, SkipSampling("/users"))

	require.NoError(t, SetSamplingPathSkipper([]string{"^/health$", "^/metrics", "^/health$"}))
	assert.True(t, SkipSampling("/metrics/x"))

	err := SetSamplingPathSkipper([]string{"("})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sentry.skipTracesEndpoints")
	assert.True(t, SkipSampling("/health"), "the previous skipper is kept on error")
}

func TestPathSkipperInvalidPattern(t *testing.T) {
	_, err := InitAccessLogPathSkipper([]string{"[a-"})
	assert.ErrorContains(t, err, "accessLog.skipEndpoints")

	_, err = InitPrometheusPathSkipper([]string{"[a-"}, "/metrics")
	assert.ErrorContains(t, err, "prometheus.skipEndpoints")
}
