package test

import (
	"testing"
)

// Severity represents the level of importance or urgency of a test.
// It helps developers prioritize which tests to investigate first when some tests fails.
type Severity string

const (
	CRITICAL Severity = "Critical"
	HIGH     Severity = "High"
	MEDIUM   Severity = "Medium"
	LOW      Severity = "Low"
	TRIVIAL  Severity = "Trivial"
	NO_SET   Severity = "No_Set"
)

const (
	SEVERITY_LOG_PREFIX = "Severity#"
)

// SetSeverity sets the severity of a test.
func SetSeverity(t *testing.T, s Severity) {
	t.Helper()

	// it needs to be logged as it will be captured in the test results and output as `Severity` in the per-test log.
	t.Logf("%s%s", SEVERITY_LOG_PREFIX, s)
}
