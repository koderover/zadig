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
	"io"
	"os"
	"strings"

	"github.com/koderover/zadig/pkg/microservice/reaper/core/service/cmd"
)

// helper function returns true if directory dir is empty.
func isDirEmpty(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return true
	}
	defer f.Close()

	_, err = f.Readdir(1)
	return err == io.EOF
}

func setCmdsWorkDir(dir string, cmds []*cmd.Command) {
	for _, c := range cmds {
		c.Cmd.Dir = dir
	}
}

func replaceTestSuiteTag(inputXML, replaceStr, newStr string) string {
	var replaceStrings = []string{replaceStr}

	outputXML := inputXML
	for _, str := range replaceStrings {
		outputXML = strings.Replace(outputXML, str, newStr, -1)
	}
	return outputXML
}
