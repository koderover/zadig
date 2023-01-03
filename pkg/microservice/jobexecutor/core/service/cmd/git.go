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

package cmd

import (
	"os/exec"

	"github.com/koderover/zadig/pkg/types"
)

// InitGit creates an empty git repository.
// it returns command git init
func InitGit(dir string) *exec.Cmd {
	cmd := exec.Command(
		"git",
		"init",
	)
	cmd.Dir = dir
	return cmd
}

// RemoteAdd sets the remote origin for the repository.
func RemoteAdd(remoteName, remote string) *exec.Cmd {
	return exec.Command(
		"git",
		"remote",
		"add",
		remoteName,
		remote,
	)
}

// RemoteRemove removes the remote origin for the repository.
func RemoteRemove(remoteName string) *exec.Cmd {
	return exec.Command(
		"git",
		"remote",
		"remove",
		remoteName,
	)
}

// CheckoutHead returns command git checkout -qf FETCH_HEAD
func CheckoutHead() *exec.Cmd {
	return exec.Command(
		"git",
		"checkout",
		"-qf",
		"FETCH_HEAD",
	)
}

// Fetch fetches changes by ref, ref can be a tag, branch or pr. --depth=1 is used to limit fetching
// to the last commit from the tip of each remote branch history.
// e.g. git fetch origin +refs/heads/onboarding --depth=1
func Fetch(remoteName, ref string) *exec.Cmd {
	return exec.Command(
		"git",
		"fetch",
		remoteName,
		"+"+ref, // "+" means overwrite
		"--depth=1",
	)
}

// DeepenedFetch deepens the fetch history. It is similar with Fetch but accepts 500 more commit history than
// last Fetch operation by --deepen=500 option.
// e.g. git fetch origin +refs/heads/onboarding --deepen=500
func DeepenedFetch(remoteName, ref, source string) *exec.Cmd {
	cmdArgs := []string{
		"fetch",
		remoteName,
		"+" + ref, // "+" means overwrite
	}
	if source != types.ProviderGerrit {
		cmdArgs = append(cmdArgs, "--deepen=500")
	}
	return exec.Command(
		"git",
		cmdArgs...,
	)
}

// ResetMerge reset last merge
// It return command git reset --merge
func ResetMerge() *exec.Cmd {
	return exec.Command(
		"git",
		"reset",
		"--merge",
	)

}

// Merge merge a branch
// e.g. git merge demo
func Merge(branch string) *exec.Cmd {
	return exec.Command(
		"git",
		"merge",
		branch,
		"--allow-unrelated-histories", // add this option to avoid "fatal: refusing to merge unrelated histories"
	)
}

// UpdateSubmodules returns command: git submodule update --init --recursive
func UpdateSubmodules() *exec.Cmd {
	cmd := exec.Command(
		"git",
		"submodule",
		"update",
		"--init",
		"--recursive",
	)
	return cmd
}

// SetConfig returns command: git config --global $KEY $VA
// e.g. git config --global user.name username
func SetConfig(key, value string) *exec.Cmd {
	return exec.Command(
		"git",
		"config",
		"--global",
		key,
		value,
	)
}

// SetConfig returns command: git config --global $KEY $VA
// e.g. git config --global user.name username
func Gc() *exec.Cmd {
	return exec.Command(
		"git",
		"gc",
	)
}

// ShowLastLog returns command git --no-pager log --oneline -1
// It shows last commit messge with sha
func ShowLastLog() *exec.Cmd {
	return exec.Command(
		"git",
		"--no-pager",
		"log",
		"--oneline",
		"-1",
	)
}
