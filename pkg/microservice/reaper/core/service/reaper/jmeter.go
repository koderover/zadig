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
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/koderover/zadig/pkg/tool/log"
)

func JmeterTestResults(testResultFile, testResultPath, testUploadPath string) error {
	var err error
	if len(testResultPath) == 0 {
		log.Warning("jmeter test result step skipped: no test result path found")
		return nil
	}

	files, err := ioutil.ReadDir(testResultPath)
	if err != nil || len(files) == 0 {
		return fmt.Errorf("test result files not found in path %s", testResultPath)
	}

	// sort and process cvs files by modified time
	sort.SliceStable(files, func(i, j int) bool {
		return files[i].ModTime().Before(files[j].ModTime())
	})
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".csv" {
			filePath := path.Join(testResultPath, file.Name())
			log.Infof("name %s mod time: %v", file.Name(), file.ModTime())
			csvFileReader, err := os.Open(filePath)
			if err != nil {
				log.Warningf("open file [%s] , error: %v", filePath, err)
				continue
			}
			defer csvFileReader.Close()

			csvReader := csv.NewReader(csvFileReader)
			row, err := csvReader.Read()
			if err != nil {
				log.Warningf("Read file [%s] first row , error: %v", filePath, err)
				continue
			}
			if len(row) != 11 {
				log.Warningf("csv file type match error", filePath, err)
				continue
			}
			csvFileWrite, err := os.Create(path.Join(testUploadPath, testResultFile))
			if err != nil {
				log.Warningf("write file [%s], error: %v", filePath, err)
				continue
			}
			defer csvFileWrite.Close()
			csvWriter := csv.NewWriter(csvFileWrite)
			err = csvWriter.Write(row)
			if err != nil {
				log.Warningf("write file [%s] first row , error: %v", path.Join(testUploadPath, testResultFile), err)
				continue
			}
			rows, err := csvReader.ReadAll()
			if err != nil {
				log.Warningf("Read file [%s], error: %v", filePath, err)
				continue
			}
			err = csvWriter.WriteAll(rows)
			if err != nil {
				log.Warningf("write file [%s] all other row , error: %v", path.Join(testUploadPath, testResultFile), err)
				continue
			}
			csvWriter.Flush()

			break
		}
	}
	log.Infof("perfermace test results files %s succeeded", testResultFile)
	return nil
}
