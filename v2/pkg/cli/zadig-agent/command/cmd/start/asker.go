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

package start

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common"
)

func AskInstaller(serverURL *string, token *string, workDir *string) error {
	if *serverURL == "" {
		*serverURL = ask("Please enter the Zadig server URL (e.g. http://zadig.koderover.com):", false)
	}

	if *token == "" {
		*token = ask("Please enter the Zadig agent token:", false)
	}

	//if *workDir == "" {
	//	*workDir = ask("Please enter the Zadig agent work directory:", true)
	//}
	defaultAgentWorkDir, err := config.GetAgentWorkDir("")
	if err != nil {
		return fmt.Errorf("failed to get agent config file path: %v", err)
	}
	if *workDir == "" {
		log.Infof("workDir is empty, use default workDir: %s", defaultAgentWorkDir)
		*workDir = defaultAgentWorkDir
	}

	viper.Set(common.ZADIG_SERVER_URL, serverURL)
	viper.Set(common.AGENT_TOKEN, token)
	viper.Set(common.AGENT_WORK_DIRECTORY, *workDir)
	return nil
}

func ask(notice string, allowEmptyOptional bool) string {
	answer := new(string)
	for {
		if askOnce(notice, answer, allowEmptyOptional) {
			break
		}
	}
	return *answer
}

func askOnce(prompt string, result *string, allowEmptyOptional bool) bool {
	println(prompt)
	if *result != "" {
		print("["+*result, "]: ")
	}

	reader := bufio.NewReader(os.Stdin)

	data, _, err := reader.ReadLine()
	if err != nil {
		panic(err)
	}
	newResult := string(data)
	newResult = strings.TrimSpace(newResult)

	if allowEmptyOptional || newResult != "" {
		*result = newResult
		return true
	}

	if *result != "" {
		return true
	}
	return false
}
