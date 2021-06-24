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

package meta

type TestSuites struct {
	Tests      int          `bson:"tests"                   json:"tests"          xml:"tests,attr"`
	Skips      int          `bson:"skips"                   json:"skips"          xml:"skips,attr"`
	Failures   int          `bson:"failures"                json:"failures"       xml:"failures,attr"`
	Errors     int          `bson:"errors,omitempty"        json:"errors"         xml:"errors,attr,omitempty"`
	Time       float64      `bson:"time"                    json:"time"           xml:"time,attr"`
	TestSuites []*TestSuite `bson:"testsuite"               json:"testsuite"      xml:"testsuite"`
}

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
