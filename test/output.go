package test

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

type Report struct {
	Project    string           `json:"project"`
	TestPath   string           `json:"test_path"`
	ExecutedAt ReportTime       `json:"executed_at"`
	Stats      Stats            `json:"stats"`
	PkgResults []*PackageResult `json:"results"`
}

type ReportTime struct {
	time.Time
}

func (rt ReportTime) MarshalJSON() ([]byte, error) {
	formatted := rt.Format("2006-01-02 15:04:05 -0700 MST") // to print as executed time on test report as JSON
	return json.Marshal(formatted)
}

func (rt ReportTime) FormattedHTML() string {
	return rt.Format("2006-01-02 15:04:05 -0700 MST") // to print as executed time on test report as HTML
}

func (rt ReportTime) ToSuffix() string {
	return rt.Format("20060102150405") // to add as suffix of test report file name
}

type Stats struct {
	Tests struct { // TODO: consider using `map[Result]uint` instead of `struct`
		Pass    uint `json:"PASS"`
		Fail    uint `json:"FAIL"`
		Skip    uint `json:"SKIP"`
		Unknown uint `json:"Unknown,omitempty"`
		Total   uint `json:"Total"`
	} `json:"tests"`
	Severities struct { // TODO: consider using `map[Result][Severity]uint` instead of `struct`
		Pass    map[Severity]uint `json:"PASS"`
		Fail    map[Severity]uint `json:"FAIL"`
		Skip    map[Severity]uint `json:"SKIP"`
		Unknown map[Severity]uint `json:"Unknown,omitempty"`
		Total   map[Severity]uint `json:"Total"`
	} `json:"severities"`
}

type PackageResult struct {
	Package string        `json:"package"`
	Tests   []*TestResult `json:"tests"`
}

type TestResult struct {
	Test string           `json:"test,omitempty"`
	Subs []*SubTestResult `json:"subs,omitempty"`
}

type SubTestResult struct {
	Sub      string    `json:"sub,omitempty"`
	Result   Result    `json:"result"`
	Severity Severity  `json:"severity"`
	Details  []*Detail `json:"details,omitempty"`
}

type Result string

const (
	RLT_PASS    Result = "PASS"
	RLT_FAIL    Result = "FAIL"
	RLT_SKIP    Result = "SKIP"
	RLT_UNKNOWN Result = "Unknown" // could be set if test is terminated for some reason before executing it
)

type Detail struct {
	Time   time.Time `json:"time"`
	Output string    `json:"output"`
}

var SeverityRegex = regexp.MustCompile(fmt.Sprintf(`%s(\w+)`, SEVERITY_LOG_PREFIX))

// Event represents a unit of output from a test and a mirror of the test2json.event struct.
// FYI, refer to https://pkg.go.dev/cmd/test2json or https://cs.opensource.google/go/go/+/master:src/cmd/internal/test2json/test2json.go;l=31;bpv=0;bpt=0
type Event struct {
	Time    time.Time `json:"Time"`
	Action  Action    `json:"Action"`
	Package string    `json:"Package"`        // Package name
	Test    string    `json:"Test,omitempty"` // Test name
	Output  string    `json:"Output,omitempty"`
	Elapsed float64   `json:"Elapsed,omitempty"`
}

// Action represents an action of a test.
// FYI, refer to https://pkg.go.dev/cmd/test2json
type Action string

const (
	START  Action = "start"
	RUN    Action = "run"
	PAUSE  Action = "pause"
	CONT   Action = "cont"
	PASS   Action = "pass"
	BENCH  Action = "bench"
	FAIL   Action = "fail"
	OUTPUT Action = "output"
	SKIP   Action = "skip" // NOTE: it is set if no tests in a package
)
