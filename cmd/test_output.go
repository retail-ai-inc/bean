package cmd

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/retail-ai-inc/bean/test"
)

type report struct {
	Project    string           `json:"project"`
	TestPath   string           `json:"test_path"`
	ExecutedAt reportTime       `json:"executed_at"`
	Stats      stats            `json:"stats"`
	PkgResults []*packageResult `json:"results"`
}

type reportTime struct {
	time.Time
}

// MarshalJSON implements json.Marshaler interface to print as executed time on test report as JSON
func (rt reportTime) MarshalJSON() ([]byte, error) {
	formatted := rt.Format("2006-01-02 15:04:05 -0700 MST")
	return json.Marshal(formatted)
}

// FormattedHTML returns a formatted time string to print as executed time on test report as HTML
// it is used in a report template html file
func (rt reportTime) FormattedHTML() string {
	return rt.Format("2006-01-02 15:04:05 -0700 MST")
}

func (rt reportTime) toSuffix() string {
	return rt.Format("20060102150405") // to add as suffix of test report file name
}

type stats struct {
	Tests struct { // TODO: consider using `map[Result]uint` instead of `struct`
		Pass    uint `json:"PASS"`
		Fail    uint `json:"FAIL"`
		Skip    uint `json:"SKIP"`
		Unknown uint `json:"Unknown,omitempty"`
		Total   uint `json:"Total"`
	} `json:"tests"`
	Severities struct { // TODO: consider using `map[Result][Severity]uint` instead of `struct`
		Pass    map[test.Severity]uint `json:"PASS"`
		Fail    map[test.Severity]uint `json:"FAIL"`
		Skip    map[test.Severity]uint `json:"SKIP"`
		Unknown map[test.Severity]uint `json:"Unknown,omitempty"`
		Total   map[test.Severity]uint `json:"Total"`
	} `json:"severities"`
}

type packageResult struct {
	Package string        `json:"package"`
	Tests   []*testResult `json:"tests"`
}

type testResult struct {
	Test string           `json:"test,omitempty"`
	Subs []*subTestResult `json:"subs,omitempty"`
}

type subTestResult struct {
	Sub      string        `json:"sub,omitempty"`
	Result   result        `json:"result"`
	Severity test.Severity `json:"severity"`
	Details  []*detail     `json:"details,omitempty"`
}

type result string

const (
	RLT_PASS    result = "PASS"
	RLT_FAIL    result = "FAIL"
	RLT_SKIP    result = "SKIP"
	RLT_UNKNOWN result = "Unknown" // could be set if test is terminated for some reason before executing it
)

type detail struct {
	Time   time.Time `json:"time"`
	Output string    `json:"output"`
}

var SeverityRegex = regexp.MustCompile(fmt.Sprintf(`%s(\w+)`, test.SEVERITY_LOG_PREFIX))

type rowSpans struct {
	Package uint
	Test    uint
	SubTest uint
}

// event represents a unit of output from a test and a mirror of the test2json.event struct.
// FYI, refer to https://pkg.go.dev/cmd/test2json or https://cs.opensource.google/go/go/+/master:src/cmd/internal/test2json/test2json.go;l=31;bpv=0;bpt=0
type event struct {
	Time    time.Time `json:"Time"`
	Action  action    `json:"Action"`
	Package string    `json:"Package"`        // Package name
	Test    string    `json:"Test,omitempty"` // Test name
	Output  string    `json:"Output,omitempty"`
	Elapsed float64   `json:"Elapsed,omitempty"`
}

// action represents an action of a test.
// FYI, refer to https://pkg.go.dev/cmd/test2json
type action string

const (
	START  action = "start"
	RUN    action = "run"
	PAUSE  action = "pause"
	CONT   action = "cont"
	PASS   action = "pass"
	BENCH  action = "bench"
	FAIL   action = "fail"
	OUTPUT action = "output"
	SKIP   action = "skip" // NOTE: it is set if no tests in a package
)
