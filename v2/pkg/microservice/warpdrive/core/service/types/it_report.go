/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

type ItReport struct {
	PipelineName         string                  `bson:"pipeline_name"           json:"pipeline_name"`
	PipelineTaskID       int64                   `bson:"pipeline_task_id"        json:"pipeline_task_id"`
	TestName             string                  `bson:"test_name"               json:"test_name"`
	TestJobName          string                  `bson:"test_job_name"           json:"test_job_name"`
	TestSuiteSummary     TestSuiteSummary        `bson:"testsuite"               json:"testsuite"`
	PerformanceTestSuite []*PerformanceTestSuite `bson:"performace_test_suite"   json:"performace_test_suite"`
	Created              int64                   `bson:"create,omitempty"        json:"create,omitempty"`
	Error                string                  `bson:"error,omitempty"         json:"error,omitempty"`
}

type TestSuiteSummary struct {
	Tests    int     `bson:"tests"         json:"tests"          xml:"tests,attr"`
	Failures int     `bson:"failures"      json:"failures"       xml:"failures,attr"`
	Skips    int     `bson:"skips"         json:"skips"          xml:"skips,attr"`
	Errors   int     `bson:"errors"        json:"errors"         xml:"errors,attr"`
	Time     float64 `bson:"time"          json:"time"           xml:"time,attr"`
}

type TestReport struct {
	FunctionTestSuite     *TestSuite                `bson:"function_test_suite,omitempty"         json:"functionTestSuite,omitempty"`
	PerformanceTestSuites []*PerformanceTestSuite   `bson:"performance_test_suite,omitempty"      json:"performanceTestSuite,omitempty"`
	Security              map[string]map[string]int `bson:"security,omitempty"                    json:"security,omitempty"`
}

// TestSuites ...
type TestSuites struct {
	Tests      int          `bson:"tests"                   json:"tests"          xml:"tests,attr"`
	Skips      int          `bson:"skips"                   json:"skips"          xml:"skips,attr"`
	Failures   int          `bson:"failures"                json:"failures"       xml:"failures,attr"`
	Errors     int          `bson:"errors,omitempty"        json:"errors"         xml:"errors,attr,omitempty"`
	Time       float64      `bson:"time"                    json:"time"           xml:"time,attr"`
	TestSuites []*TestSuite `bson:"testsuite"               json:"testsuite"      xml:"testsuite"`
}

// TestSuite ...
type TestSuite struct {
	// 总数=tests+skips 成功=tests-failures-errors
	Tests     int        `bson:"tests"                   json:"tests"                    xml:"tests,attr"`
	Failures  int        `bson:"failures"                json:"failures"                 xml:"failures,attr"`
	Successes int        `bson:"successes,omitempty"     json:"successes,omitempty"      xml:"successes,attr,omitempty"`
	Skips     int        `bson:"skips"                   json:"skips"                    xml:"skips,attr"`
	Errors    int        `bson:"errors,omitempty"        json:"errors"                   xml:"errors,attr,omitempty"`
	Time      float64    `bson:"time"                    json:"time"                     xml:"time,attr"`
	SystemOut string     `bson:"system_out,omitempty"    json:"system_out"               xml:"system-out,omitempty"`
	SystemErr string     `bson:"system-err,omitempty"    json:"system_err"               xml:"system-err,omitempty"`
	TestCases []TestCase `bson:"testcase"                json:"testcase"                 xml:"testcase"`
	SuiteType string     `bson:"-"                       json:"-"                        xml:"-"`
	Name      string     `bson:"name"                    json:"-"                        xml:"-"`
}

type Skipped struct {
}

type Failure struct {
	Message string `bson:"message"  json:"message" xml:"message,attr"`
	Type    string `bson:"type"     json:"type"    xml:"type,attr"`
	Text    string `bson:"text"     json:"text"    xml:",chardata"`
}

type TestCase struct {
	Name      string   `bson:"tc_name"                 json:"tc_name"      xml:"name,attr"`
	ClassName string   `bson:"classname"               json:"classname"    xml:"classname,attr"`
	Time      float64  `bson:"time"                    json:"time"         xml:"time,attr"`
	Failure   *Failure `bson:"failure,omitempty"       json:"failure"      xml:"failure,omitempty"`
	Skipped   *Skipped `bson:"skipped,omitempty"       json:"skipped"      xml:"skipped,omitempty"`
	SystemOut string   `bson:"system_out,omitempty"    json:"system_out"   xml:"system-out,omitempty"`
	SystemErr string   `bson:"system-err,omitempty"    json:"system_err"   xml:"system-err,omitempty"`
	Error     *Error   `bson:"error,omitempty"         json:"error"        xml:"error,omitempty"`
}

type Error struct {
	Message string `bson:"message"  json:"message" xml:"message,attr"`
	Type    string `bson:"type"     json:"type"    xml:"type,attr"`
	Text    string `bson:"text"     json:"text"    xml:",chardata"`
}

type PerformanceTestSuite struct {
	Label      string `bson:"label"        json:"label"`
	Samples    string `bson:"samples"      json:"samples"`
	Average    string `bson:"average"      json:"average"`
	Min        string `bson:"min"          json:"min"`
	Max        string `bson:"max"          json:"max"`
	Line       string `bson:"line"         json:"line"`
	StdDev     string `bson:"std_dev"      json:"stdDev"`
	Error      string `bson:"error"        json:"error"`
	Throughput string `bson:"throughput"   json:"throughput"`
	ReceivedKb string `bson:"received_kb"  json:"receivedKb"`
	AvgByte    string `bson:"avg_byte"     json:"avgByte"`
}
