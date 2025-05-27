/*
Copyright 2022 The KodeRover Authors.

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

package step

import (
	"context"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/microservice/reaper/core/service/meta"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
)

const (
	ReploaceTestSuite  = "TestSuite"
	ReploaceTestSuites = "TestSuites"
)

type JunitReportStep struct {
	spec       *step.StepJunitReportSpec
	envs       []string
	secretEnvs []string
	workspace  string
}

func NewJunitReportStep(spec interface{}, workspace string, envs, secretEnvs []string) (*JunitReportStep, error) {
	junitReportStep := &JunitReportStep{workspace: workspace, envs: envs, secretEnvs: secretEnvs}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return junitReportStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &junitReportStep.spec); err != nil {
		return junitReportStep, fmt.Errorf("unmarshal spec %s to junit report spec failed", yamlBytes)
	}
	return junitReportStep, nil
}

func (s *JunitReportStep) Run(ctx context.Context) error {
	log.Info("Start merge ginkgo test results.")
	if err := os.MkdirAll(s.spec.DestDir, os.ModePerm); err != nil {
		return fmt.Errorf("create dest dir: %s error: %s", s.spec.DestDir, err)
	}

	envMap := util.MakeEnvMap(s.envs, s.secretEnvs)
	s.spec.ReportDir = util.ReplaceEnvWithValue(s.spec.ReportDir, envMap)

	reportDir := filepath.Join(s.workspace, s.spec.ReportDir)
	results, err := mergeGinkgoTestResults(s.spec.FileName, reportDir, s.spec.DestDir, time.Now())
	if err != nil {
		return fmt.Errorf("failed to merge test result: %s", err)
	}
	log.Info("Finish merge ginkgo test results.")

	log.Infof("Start archive %s.", s.spec.FileName)
	if s.spec.S3DestDir == "" || s.spec.FileName == "" {
		return nil
	}
	client, err := s3.NewClient(s.spec.S3Storage.Endpoint, s.spec.S3Storage.Ak, s.spec.S3Storage.Sk, s.spec.S3Storage.Region, s.spec.S3Storage.Insecure, s.spec.S3Storage.Provider)
	if err != nil {
		return fmt.Errorf("failed to create s3 client to upload file, err: %s", err)
	}

	absFilePath := path.Join(s.spec.DestDir, s.spec.FileName)

	if len(s.spec.S3Storage.Subfolder) > 0 {
		s.spec.S3DestDir = strings.TrimLeft(path.Join(s.spec.S3Storage.Subfolder, s.spec.S3DestDir), "/")
	}

	info, err := os.Stat(absFilePath)
	if err != nil {
		return fmt.Errorf("failed to upload file path [%s] to destination [%s], the error is: %s", absFilePath, s.spec.S3DestDir, err)
	}
	// if the given path is a directory
	if info.IsDir() {
		err := client.UploadDir(s.spec.S3Storage.Bucket, absFilePath, s.spec.S3DestDir)
		if err != nil {
			return err
		}
	} else {
		key := filepath.Join(s.spec.S3DestDir, info.Name())
		err := client.Upload(s.spec.S3Storage.Bucket, absFilePath, key)
		if err != nil {
			return err
		}
	}
	log.Infof("Finish archive to %s.", s.spec.FileName)
	if results.Failures > 0 || results.Errors > 0 {
		return fmt.Errorf("%d case(s) failed, %d case(s) error", results.Failures, results.Errors)
	}
	return nil
}

func mergeGinkgoTestResults(testResultFile, testResultPath, testUploadPath string, startTime time.Time) (*meta.TestSuite, error) {
	var (
		err           error
		newXMLBytes   []byte
		summaryResult = &meta.TestSuite{
			TestCases: []meta.TestCase{},
		}
	)

	if len(testResultPath) == 0 {
		return summaryResult, nil
	}

	files, err := ioutil.ReadDir(testResultPath)
	if err != nil || len(files) == 0 {
		return summaryResult, fmt.Errorf("test result files not found in path %s", testResultPath)
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
		return summaryResult, err
	}
	// 2. append header
	newXMLBytes = append([]byte(xml.Header), newXMLBytes...)

	// 3. process test suite tag
	newXMLStr := replaceTestSuiteTag(string(newXMLBytes), ReploaceTestSuite, strings.ToLower(ReploaceTestSuite))

	//4. write xml bytes into file
	err = ioutil.WriteFile(path.Join(testUploadPath, testResultFile), []byte(newXMLStr), 0644)
	if err != nil {
		return summaryResult, err
	}

	log.Infof("merge test results files %s succeeded", testResultFile)
	return summaryResult, nil
}

func getSecondSince(startTime time.Time) float64 {
	return float64(time.Since(startTime).Round(time.Millisecond).Nanoseconds()) / float64(time.Second)
}

func replaceTestSuiteTag(inputXML, replaceStr, newStr string) string {
	var replaceStrings = []string{replaceStr}

	outputXML := inputXML
	for _, str := range replaceStrings {
		outputXML = strings.Replace(outputXML, str, newStr, -1)
	}
	return outputXML
}
