//go:build linux || darwin
// +build linux darwin

/*
Copyright 2023 The KodeRover Authors.

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

/*
Copyright 2023 The KodeRover Authors.

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

package script

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common/types"
)

func generateScript(spec *StepShellSpec, dirs *types.AgentWorkDirs, jobOutput []string, logger *log.JobLogger) (string, error) {
	if len(spec.Scripts) == 0 {
		return "", nil
	}
	scripts := []string{}
	scripts = append(scripts, spec.Scripts...)

	// add job output to script
	if len(jobOutput) > 0 {
		scripts = append(scripts, outputScript(dirs.JobOutputsDir, jobOutput)...)
	}

	userScriptFile := config.GetUserScriptFilePath(dirs.JobScriptDir)
	if err := ioutil.WriteFile(userScriptFile, []byte(strings.Join(scripts, "\n")), 0700); err != nil {
		return "", fmt.Errorf("write script file error: %v", err)
	}
	return userScriptFile, nil
}

// generate script to save outputs variable to file
func outputScript(outputsDir string, outputs []string) []string {
	resp := []string{"set +ex"}
	for _, output := range outputs {
		resp = append(resp, fmt.Sprintf("echo $%s > %s", output, path.Join(outputsDir, output)))
	}
	return resp
}
