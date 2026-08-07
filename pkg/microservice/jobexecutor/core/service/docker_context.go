/*
Copyright 2026 The KodeRover Authors.

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

package job

import (
	"os"
	"os/exec"
	"strings"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

const dockerBuildxContextName = "zadig-buildx"

func initializeDockerContext() {
	if strings.TrimSpace(os.Getenv(setting.DockerHost)) == "" {
		return
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return
	}

	if err := exec.Command("docker", "context", "inspect", dockerBuildxContextName).Run(); err != nil {
		createCmd := exec.Command("docker", "context", "create", dockerBuildxContextName)
		var stderr strings.Builder
		createCmd.Stderr = &stderr
		if err := createCmd.Run(); err != nil {
			stderrMessage := strings.Join(strings.Fields(stderr.String()), " ")
			if stderrMessage == "" {
				log.Warnf("Failed to initialize Docker context %q: %v", dockerBuildxContextName, err)
			} else {
				log.Warnf("Failed to initialize Docker context %q: %v, stderr: %s", dockerBuildxContextName, err, stderrMessage)
			}
			return
		}
	}

	if err := os.Setenv(setting.DockerContext, dockerBuildxContextName); err != nil {
		log.Warnf("Failed to select Docker context %q: %v", dockerBuildxContextName, err)
		return
	}

	// The connection and TLS configuration are now stored in the named context.
	// Leaving the source variables in the environment makes buildx reject an
	// implicit builder creation even when DOCKER_CONTEXT is selected.
	_ = os.Unsetenv(setting.DockerHost)
	_ = os.Unsetenv(setting.DockerTLSVerify)
	_ = os.Unsetenv(setting.DockerCertPath)
}
