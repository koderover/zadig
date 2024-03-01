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

package service

import (
	"os/exec"
	"strings"
)

const dockerExe = "docker"

func dockerVersion() *exec.Cmd {
	return exec.Command(dockerExe, "version")
}

func dockerInfo() *exec.Cmd {
	return exec.Command(dockerExe, "info")
}

// Docker returns build command
//
// e.g. docker build --rm=true -t name:tag -f Dockerfile --build-arg APP_BIN=... .
func dockerBuild(dockerfile, fullImage, buildArgs string) *exec.Cmd {

	args := []string{"build", "--rm=true"}
	if buildArgs != "" {
		for _, val := range strings.Fields(buildArgs) {
			if val != "" {
				args = append(args, val)
			}
		}

	}
	args = append(args, []string{"-t", fullImage, "-f", dockerfile, "."}...)
	return exec.Command(dockerExe, args...)
}

func dockerPull(image string) *exec.Cmd {
	args := []string{"pull", image}
	return exec.Command(dockerExe, args...)
}

func dockerTag(sourceFullImage, targetFullImage string) *exec.Cmd {
	args := []string{
		"tag",
		sourceFullImage,
		targetFullImage,
	}
	return exec.Command(dockerExe, args...)
}

func dockerPush(fullImage string) *exec.Cmd {
	args := []string{
		"push",
		fullImage,
	}
	return exec.Command(dockerExe, args...)
}
