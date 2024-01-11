// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/retail-ai-inc/bean/v2/helpers"
	"github.com/retail-ai-inc/bean/v2/test"

	"github.com/spf13/cobra"
)

// OutputType is a type of output of test results
type OutputType string

const (
	CLI  OutputType = "cli"
	JSON OutputType = "json"
	HTML OutputType = "html"
)

var outputs = []OutputType{CLI, JSON, HTML}

// flag names
const (
	TEST_PATH   = "test_path"
	OUTPUT_TYPE = "output"
	REPORT_PATH = "report_path"
	VERBOSE     = "verbose"
	RACE        = "race"
)

const (
	RECURSIVE_SUFFIX = "/..."
)

// DEFAULT_XXX is used for default values of flags
const (
	DEFAULT_TEST_PATH   = "./..."
	DEFAULT_OUTPUT_TYPE = string(CLI)
	DEFAULT_REPORT_PATH = "./report"
	DEFAULT_VERBOSE     = false
	DEFAULT_RACE        = false
)

func init() {
	// Add flags for the test command
	TestCmd.Flags().StringP(OUTPUT_TYPE, "o", DEFAULT_OUTPUT_TYPE, "specify a type of output of test results among 'cli', 'json', or 'html'")
	TestCmd.Flags().StringP(REPORT_PATH, "p", DEFAULT_REPORT_PATH, "if output type is not 'cli', specify a path where test results as report will be output")
	TestCmd.Flags().BoolP(VERBOSE, "v", DEFAULT_VERBOSE, "enable verbose output especially for passed or skipped tests as well as failed tests")
	TestCmd.Flags().BoolP(RACE, "r", DEFAULT_RACE, "enable a race detector")

	// Add the test command to the root command
	rootCmd.AddCommand(TestCmd)
}

// TestCmd represents the test command
var TestCmd = &cobra.Command{
	Use:   "test [test path]",
	Short: "Run tests and generate the result report",
	Long: `Run tests with go test command and generate a report by specifying a path under which to run tests (default "./...")
You also have several options for output type, report path and verbose output`,
	Args:         cobra.MaximumNArgs(1),
	RunE:         runTest,
	SilenceUsage: true,
}

func runTest(cmd *cobra.Command, args []string) error {
	opts := newTestOptions()

	if len(args) > 0 {
		opts.testPath = args[0]
	}
	if err := opts.validateTestPath(); err != nil {
		return err
	}

	rawType, err := cmd.Flags().GetString(OUTPUT_TYPE)
	if err != nil {
		return fmt.Errorf("failed to get output type: %w", err)
	}
	opts.oType = OutputType(rawType)
	if err := opts.validateOutputType(); err != nil {
		return err
	}

	opts.reportPath, err = cmd.Flags().GetString(REPORT_PATH)
	if err != nil {
		return fmt.Errorf("failed to get output path: %w", err)
	}
	if err := opts.validateReportPath(); err != nil {
		return err
	}

	opts.verbose, err = cmd.Flags().GetBool(VERBOSE)
	if err != nil {
		return fmt.Errorf("failed to get verbose flag: %w", err)
	}

	opts.race, err = cmd.Flags().GetBool(RACE)
	if err != nil {
		return fmt.Errorf("failed to get race flag: %w", err)
	}

	err = execute(opts)
	if err != nil {
		return err
	}

	return nil
}

type testOptions struct {
	testPath   string
	oType      OutputType
	reportPath string
	verbose    bool
	race       bool
}

func newTestOptions() *testOptions {
	return &testOptions{
		testPath:   DEFAULT_TEST_PATH,
		oType:      OutputType(DEFAULT_OUTPUT_TYPE),
		reportPath: DEFAULT_REPORT_PATH,
		verbose:    DEFAULT_VERBOSE,
		race:       DEFAULT_RACE,
	}
}

func (o *testOptions) validateTestPath() error {
	testPath := o.testPath
	if strings.HasSuffix(o.testPath, RECURSIVE_SUFFIX) {
		testPath = strings.TrimSuffix(o.testPath, RECURSIVE_SUFFIX)
	}
	info, err := os.Stat(testPath)
	if err != nil {
		return fmt.Errorf("failed to get test path '%s': %w", testPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("test path '%s' is not a directory", testPath)
	}

	return nil
}

func (o *testOptions) validateOutputType() error {
	if !helpers.HasTargetInSlice(outputs, o.oType) {
		return fmt.Errorf("output type '%s' is not a valid output type. it must be one of '%s'", o.oType, helpers.Join(outputs, "', '"))
	}

	return nil
}

func (o *testOptions) validateReportPath() error {

	if o.oType == CLI {
		return nil
	}

	if o.reportPath != DEFAULT_REPORT_PATH {
		info, err := os.Stat(o.reportPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("failed to get report path '%s': %w", o.reportPath, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("report path '%s' is not a directory", o.reportPath)
		}
	}

	return nil
}

func (o *testOptions) buildTestCmdArgs() []string {
	args := []string{"test", "-count=1", o.testPath} // set default args

	if o.oType != CLI {
		// NOTE: even when the output type is HTML, add the -json flag to the go test command and use that output for reporting
		args = append(args, "-json")
	}
	if o.verbose {
		args = append(args, "-v")
		log.Println("verbose output is enabled.")
	}
	if o.race {
		args = append(args, "-race")
		log.Println("race detector is enabled.")
	}

	return args
}

func execute(opts *testOptions) error {
	switch opts.oType {
	case CLI:
		log.Println("output you want is on CLI.")
		err := runTestAndShowResultOnCLI(opts)
		return err

	case JSON:
		log.Println("output you want is on JSON.")
		log.Printf("test result will be output to %s\n", opts.reportPath)
		report, err := runTestAndOrganizeResult(opts)
		if err != nil {
			return err
		}
		filePath, err := outputReportJSON(report, opts.reportPath)
		if err != nil {
			return err
		}
		log.Printf("report generated successfully at %s\n", filePath)
		return nil

	case HTML:
		log.Println("output you want is on HTML.")
		log.Printf("test result will be output at %s\n", opts.reportPath)
		report, err := runTestAndOrganizeResult(opts)
		if err != nil {
			return err
		}
		filePath, err := outputReportHTML(report, opts.reportPath)
		if err != nil {
			return err
		}
		log.Printf("report generated successfully at %s\n", filePath)
		return nil

	default:
		return fmt.Errorf("output type '%s' is not a valid output type. it must be one of '%s'", opts.oType, helpers.Join(outputs, "', '"))
	}
}

func runTestAndShowResultOnCLI(opts *testOptions) error {
	log.Printf("running tests at %s and the result will be output on CLI\n", opts.testPath)

	cmd := exec.Command("go", opts.buildTestCmdArgs()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Printf("tests failed (%s).\n", err)
	} else {
		log.Printf("all tests passed.\n")
	}
	return err
}

func runTestAndOrganizeResult(opts *testOptions) (*report, error) {
	log.Printf("running tests at %s\n", opts.testPath)

	cmd := exec.Command("go", opts.buildTestCmdArgs()...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	executedAt := time.Now() // record the time when the test command is executed for the report
	err := cmd.Run()
	if err != nil {
		log.Printf("tests failed (%s)\n", err)
	} else {
		log.Printf("all tests passed.\n")
	}
	if stderr.String() != "" { // capture problematic errors prior to test execution that are output to stderr
		return nil, fmt.Errorf("failed to run tests (%s)", stderr.String())
	}

	pkgRlts, err := organizeResult(stdout, opts.verbose)
	if err != nil {
		return nil, err
	}
	stats := takeStats(pkgRlts)
	project, err := getModuleName()
	if err != nil {
		log.Println(err) // log the error and continue
	}

	repo := &report{
		Project:    project,
		TestPath:   opts.testPath,
		ExecutedAt: reportTime{Time: executedAt},
		Stats:      stats,
		PkgResults: pkgRlts,
	}

	return repo, nil
}

func organizeResult(stdout bytes.Buffer, verbose bool) ([]*packageResult, error) {
	log.Println("organizing test result...")

	events, err := unmarshalTestOutput(stdout)
	if err != nil {
		return nil, err
	}

	// group events by package name
	eventsByPkg := make(map[string][]*event)
	for _, e := range events {
		eventsByPkg[e.Package] = append(eventsByPkg[e.Package], e)
	}

	var pkgRlts []*packageResult
	for pkgName, pEvents := range eventsByPkg {
		pr := &packageResult{
			Package: pkgName,
			Tests:   nil,
		}

		// group events by test name
		eventsByTest := make(map[string][]*event)
		for _, e := range pEvents {
			var testName string
			if e.Test != "" && strings.Contains(e.Test, "/") {
				testName = strings.Split(e.Test, "/")[0]
			} else {
				testName = e.Test
			}
			eventsByTest[testName] = append(eventsByTest[testName], e)
		}

		var testRlts []*testResult
		for testName, tEvents := range eventsByTest {
			tr := &testResult{
				Test: testName,
			}

			// group events by sub test name
			eventsBySubTest := make(map[string][]*event)
			for _, e := range tEvents {
				var subTestName string
				if e.Test != "" && strings.Contains(e.Test, "/") && tr.Test == strings.Split(e.Test, "/")[0] {
					subTestName = strings.Split(e.Test, "/")[1]
				}
				eventsBySubTest[subTestName] = append(eventsBySubTest[subTestName], e)
			}

			var SubResults []*subTestResult
			for subTestName, sEvents := range eventsBySubTest {
				sr := &subTestResult{
					Sub:      subTestName,
					Result:   RLT_UNKNOWN,
					Severity: test.NO_SET,
					Details:  nil,
				}

				for _, e := range sEvents {
					// set result for test
					// find the first action and set it as the result for the test
					// because, once outputs are grouped by test, either action of PASS, SKIP, or FAIL should come out just once in either of outputs
					if sr.Result == RLT_UNKNOWN { // if result is still initialized
						switch e.Action {
						case FAIL:
							sr.Result = RLT_FAIL
						case PASS:
							sr.Result = RLT_PASS
						case SKIP:
							sr.Result = RLT_SKIP
						}
					}
					// set severity for test
					// find the first severity log and set it as the severity for the test
					// because, once outputs are grouped by test, severity log for test should come out just once in either of outputs
					if sr.Severity == test.NO_SET { // if severity is still initialized
						if e.Action == OUTPUT && e.Output != "" && SeverityRegex.MatchString(e.Output) {
							matches := SeverityRegex.FindStringSubmatch(e.Output)
							if len(matches) == 2 {
								sr.Severity = test.Severity(strings.TrimSpace(matches[1]))
							}
						}
					}
				}

				// do not append sub test results without sub test name or severity is not set unless the result is `Unknown`
				if sr.Sub == "" && sr.Severity == test.NO_SET &&
					// when test is terminated for some reason before executing it, some clues could be left in the output with `Unknown` result
					sr.Result != RLT_UNKNOWN {
					continue
				}

				var details []*detail
				for _, e := range tEvents {

					// do not append details without event's action or output is empty
					if e.Action != OUTPUT || e.Output == "" {
						continue

					}

					// do not append details for tests that are passed or skipped if verbose flag is not set
					if !verbose {
						if sr.Result == RLT_PASS || sr.Result == RLT_SKIP {
							continue
						}
					}

					details = append(details, &detail{
						Time:   e.Time,
						Output: e.Output,
					})
				}

				// do not append sub test results without details if result is not `Pass` or `Skip`
				if sr.Result != RLT_PASS && sr.Result != RLT_SKIP &&
					len(details) == 0 {
					continue
				}

				// sort outputs by time in old-to-new order
				sort.Slice(details, func(i, j int) bool {
					return details[i].Time.Before(details[j].Time)
				})

				sr.Details = details
				SubResults = append(SubResults, sr)
			}

			if len(SubResults) == 0 {
				continue
			}

			// sort test results by sub test name
			sort.Slice(SubResults, func(i, j int) bool {
				return SubResults[i].Sub < SubResults[j].Sub
			})

			tr.Subs = SubResults
			testRlts = append(testRlts, tr)
		}

		// do not append package results without test results
		if len(testRlts) == 0 {
			continue
		}

		// sort test results by test name
		sort.Slice(testRlts, func(i, j int) bool {
			return testRlts[i].Test < testRlts[j].Test
		})

		pr.Tests = testRlts
		pkgRlts = append(pkgRlts, pr)
	}

	if len(pkgRlts) == 0 {
		return nil, fmt.Errorf("no tests to report")
	}

	// sort package results by package name
	sort.Slice(pkgRlts, func(i, j int) bool {
		return pkgRlts[i].Package < pkgRlts[j].Package
	})

	return pkgRlts, nil
}

func unmarshalTestOutput(stdout bytes.Buffer) ([]*event, error) {
	rawData := stdout.String()
	// split the output by rawLines once
	rawLines := strings.Split(rawData, "\n")

	// unmarshal json line to struct iteratively
	var events []*event
	for _, l := range rawLines {
		if l == "" {
			continue
		}
		var e *event
		err := json.Unmarshal([]byte(l), &e)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal json line %s: %w", l, err)
		}
		events = append(events, e)
	}

	return events, nil
}

func takeStats(pkgRlts []*packageResult) stats {
	var stats stats
	stats.Severities.Pass = make(map[test.Severity]uint)
	stats.Severities.Fail = make(map[test.Severity]uint)
	stats.Severities.Skip = make(map[test.Severity]uint)
	stats.Severities.Unknown = make(map[test.Severity]uint)
	stats.Severities.Total = make(map[test.Severity]uint)

	for _, pr := range pkgRlts {
		for _, tr := range pr.Tests {
			for _, sr := range tr.Subs {

				severity := sr.Severity
				switch sr.Result {
				case RLT_PASS:
					stats.Tests.Pass++
					stats.Tests.Total++
					stats.Severities.Pass[severity]++
					stats.Severities.Total[severity]++
				case RLT_FAIL:
					stats.Tests.Fail++
					stats.Tests.Total++
					stats.Severities.Fail[severity]++
					stats.Severities.Total[severity]++
				case RLT_SKIP:
					stats.Tests.Skip++
					stats.Tests.Total++
					stats.Severities.Skip[severity]++
					stats.Severities.Total[severity]++
				case RLT_UNKNOWN:
					stats.Tests.Unknown++
					stats.Tests.Total++
					stats.Severities.Unknown[severity]++
					stats.Severities.Total[severity]++
					log.Printf("no result set for test. there could be a problem with %s\n", filepath.Join(pr.Package, tr.Test, sr.Sub)) // log the error and continue
				default:
					log.Printf("unknown test result: %s\n", sr.Result) // log the error and continue
				}
			}
		}
	}
	return stats
}

// getModuleName returns the name of the module of the current project.
// if finding the module name fails, it returns "unknown" as the module name.
func getModuleName() (string, error) {
	cmd := exec.Command("go", "list", "-m")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "unknown", fmt.Errorf("failed to get module name: %w", err)
	}
	moduleName := out.String()
	moduleName = moduleName[:len(moduleName)-1] // remove the trailing newline

	return moduleName, nil
}

func outputReportJSON(repo *report, outputPath string) (string, error) {
	log.Println("generating report as JSON...")

	reportJSON, err := json.MarshalIndent(repo, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal report to JSON: %w", err)
	}

	if err = makeOutputDir(outputPath); err != nil {
		return "", err
	}

	filePath := filepath.Join(outputPath, fmt.Sprintf("test_report_%s_%s.json", repo.Project, repo.ExecutedAt.toSuffix()))
	err = os.WriteFile(filePath, reportJSON, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to output report to %s: %w", filePath, err)
	}

	return filePath, nil
}

func outputReportHTML(repo *report, outputPath string) (string, error) {
	log.Println("generating report as HTML...")

	tmplFS, err := fs.Sub(InternalFS, "internal/_test/views")
	if err != nil {
		return "", fmt.Errorf("failed to get template files: %w", err)
	}

	tmpl, err := template.ParseFS(tmplFS, "report.tpl.html")
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML template: %w", err)
	}

	if err = makeOutputDir(outputPath); err != nil {
		return "", err
	}

	filePath := filepath.Join(outputPath, fmt.Sprintf("test_report_%s_%s.html", repo.Project, repo.ExecutedAt.toSuffix()))
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create HTML file: %w", err)
	}
	defer file.Close()

	type tmplData struct {
		Project        string
		TestPath       string
		ExecutedAt     reportTime
		Severities     []test.Severity
		SeverityColors map[test.Severity]string
		Results        []result
		ResultColors   map[result]string
		Stats          stats
		PkgResults     []*packageResult
		RowSpanMap     map[string]map[string]map[string]rowSpans
	}

	data := tmplData{
		Project:    repo.Project,
		TestPath:   repo.TestPath,
		ExecutedAt: repo.ExecutedAt,
		Stats:      repo.Stats,
		Severities: []test.Severity{
			test.CRITICAL,
			test.HIGH,
			test.MEDIUM,
			test.LOW,
			test.TRIVIAL,
			test.NO_SET,
		},
		SeverityColors: map[test.Severity]string{
			test.CRITICAL: "crimson",
			test.HIGH:     "orange",
			test.MEDIUM:   "yellowgreen",
			test.LOW:      "royalblue",
			test.TRIVIAL:  "skyblue",
			test.NO_SET:   "gray",
		},
		Results: []result{
			RLT_PASS,
			RLT_FAIL,
			RLT_SKIP,
			RLT_UNKNOWN,
		},
		ResultColors: map[result]string{
			RLT_PASS:    "forestgreen",
			RLT_FAIL:    "crimson",
			RLT_SKIP:    "goldenrod",
			RLT_UNKNOWN: "gray",
		},
		PkgResults: repo.PkgResults,
		RowSpanMap: calcRowSpans(repo),
	}

	err = tmpl.Execute(file, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return filePath, nil
}

func makeOutputDir(dirPath string) error {
	// if a directory in the path exists, MkdirAll does nothing and return nil
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to make output directory: '%s': %w", dirPath, err)
	}
	return nil
}

func calcRowSpans(repo *report) map[string]map[string]map[string]rowSpans {
	rowSpanMap := make(map[string]map[string]map[string]rowSpans, len(repo.PkgResults))

	for _, pr := range repo.PkgResults {
		testMap := make(map[string]map[string]rowSpans)
		pkgCount := uint(0)

		// count package rows
		for _, tr := range pr.Tests {
			subMap := make(map[string]rowSpans, len(tr.Subs))
			testCount := uint(0)

			// count test rows
			for _, sr := range tr.Subs {
				// count sub test rows
				subCount := uint(len(sr.Details))
				subCount = helpers.Max(subCount, 1) // fix sub count
				testCount += subCount
				// set row span for sub test
				subMap[sr.Sub] = rowSpans{SubTest: subCount}
			}

			testCount = helpers.Max(testCount, 1) // fix test count
			pkgCount += testCount

			// retroactively set row span for test in return
			for _, sr := range tr.Subs {
				rowSpan := rowSpans{
					Test:    testCount,
					SubTest: subMap[sr.Sub].SubTest,
				}
				subMap[sr.Sub] = rowSpan
			}

			testMap[tr.Test] = subMap
		}

		pkgCount = helpers.Max(pkgCount, 1) // fix package count
		rowSpanMap[pr.Package] = testMap

		// retroactively set row span for package
		if rowSpanMap[pr.Package] == nil {
			rowSpanMap[pr.Package] = make(map[string]map[string]rowSpans, len(pr.Tests))
		}
		for _, tr := range pr.Tests {
			if rowSpanMap[pr.Package][tr.Test] == nil {
				rowSpanMap[pr.Package][tr.Test] = make(map[string]rowSpans, len(tr.Subs))
			}
			for _, sr := range tr.Subs {
				rowSpan := rowSpanMap[pr.Package][tr.Test][sr.Sub]
				rowSpan.Package = pkgCount
				rowSpanMap[pr.Package][tr.Test][sr.Sub] = rowSpan
			}
		}
	}

	return rowSpanMap
}
