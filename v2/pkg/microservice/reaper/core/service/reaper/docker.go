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
	"os/exec"
)

const dockerExe = "docker"

func dockerLogin(user, password, registry string) *exec.Cmd {
	return exec.Command(
		dockerExe,
		"login",
		"-u", user,
		"-p", password,
		registry,
	)
}

func dockerInfo() *exec.Cmd {
	return exec.Command(dockerExe, "info")
}

func dockerPush(fullImage string) *exec.Cmd {
	args := []string{
		"push",
		fullImage,
	}
	return exec.Command(dockerExe, args...)
}
