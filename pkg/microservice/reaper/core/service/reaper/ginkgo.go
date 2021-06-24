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

package reaper

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/koderover/zadig/pkg/microservice/reaper/core/service/meta"
	"github.com/koderover/zadig/pkg/tool/log"
)

const (
	ReploaceTestSuite  = "TestSuite"
	ReploaceTestSuites = "TestSuites"
)

func getSecondSince(startTime time.Time) float64 {
	return float64(time.Since(startTime).Round(time.Millisecond).Nanoseconds()) / float64(time.Second)
}

func mergeGinkgoTestResults(testResultFile, testResultPath, testUploadPath string, startTime time.Time) error {
	var (
		err           error
		newXMLBytes   []byte
		summaryResult = &meta.TestSuite{
			TestCases: []meta.TestCase{},
		}
	)

	if len(testResultPath) == 0 {
		log.Warning("merge test result step skipped: no test result path found")
		return nil
	}

	files, err := ioutil.ReadDir(testResultPath)
	if err != nil || len(files) == 0 {
		return fmt.Errorf("test result files not found in path %s", testResultPath)
	}

	// sort and process xml files by modified time
	sort.SliceStable(files, func(i, j int) bool {
		return files[i].ModTime().Before(files[j].ModTime())
	})
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".xml" {
			filePath := path.Join(testResultPath, file.Name())
			log.Infof("name %s mod time: %v", file.Name(), file.ModTime())

			// 1. read file
			xmlBytes, err2 := ioutil.ReadFile(filePath)
			if err2 != nil {
				log.Warningf("Read file [%s], error: %v", filePath, err2)
				continue
			}

			// 2. xml unmarshal
			xmlContent := string(xmlBytes)
			if strings.Contains(xmlContent, ReploaceTestSuites) || strings.Contains(xmlContent, strings.ToLower(ReploaceTestSuites)) {
				var results *meta.TestSuites
				err2 = xml.Unmarshal(xmlBytes, &results)
				if err2 != nil {
					log.Warningf("Unmarshal xml file [%s], error: %v\n", filePath, err2)
					continue
				}
				for _, testSuite := range results.TestSuites {
					summaryResult.Tests += testSuite.Tests
					summaryResult.Failures += testSuite.Failures
					summaryResult.Errors += testSuite.Errors

					for _, tc := range testSuite.TestCases {
						if tc.Skipped != nil {
							summaryResult.Skips++
						}
					}
					summaryResult.TestCases = append(summaryResult.TestCases, testSuite.TestCases...)
				}
				summaryResult.SuiteType = ReploaceTestSuites
			} else {
				var result *meta.TestSuite
				err2 = xml.Unmarshal(xmlBytes, &result)
				if err2 != nil {
					log.Warningf("Unmarshal xml file [%s], error: %v\n", filePath, err2)
					continue
				}
				// 4. process summary result attribute
				summaryResult.Tests += result.Tests
				summaryResult.Failures += result.Failures
				summaryResult.Errors += result.Errors

				for _, tc := range result.TestCases {
					if tc.Skipped != nil {
						summaryResult.Skips++
					}
				}
				summaryResult.TestCases = append(summaryResult.TestCases, result.TestCases...)
				summaryResult.SuiteType = ReploaceTestSuite
			}
		}
	}
	summaryResult.Time = getSecondSince(startTime)
	if summaryResult.SuiteType == ReploaceTestSuite {
		summaryResult.Successes = summaryResult.Tests - summaryResult.Failures - summaryResult.Errors
		summaryResult.Tests = summaryResult.Tests + summaryResult.Skips
	} else {
		summaryResult.Successes = summaryResult.Tests - summaryResult.Failures - summaryResult.Errors - summaryResult.Skips
	}
	// 1. xml marshal indent
	newXMLBytes, err = xml.MarshalIndent(summaryResult, "  ", "    ")
	if err != nil {
		return err
	}
	// 2. append header
	newXMLBytes = append([]byte(xml.Header), newXMLBytes...)

	// 3. process test suite tag
	newXMLStr := replaceTestSuiteTag(string(newXMLBytes), ReploaceTestSuite, strings.ToLower(ReploaceTestSuite))

	//4. write xml bytes into file
	err = ioutil.WriteFile(path.Join(testUploadPath, testResultFile), []byte(newXMLStr), 0644)
	if err != nil {
		return err
	}

	log.Infof("merge test results files %s succeeded", testResultFile)
	return nil
}
