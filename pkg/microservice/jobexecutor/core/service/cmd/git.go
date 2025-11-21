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

	"github.com/koderover/zadig/v2/pkg/types"
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

// GitRemoteAdd sets the remote origin for the repository.
func GitRemoteAdd(remoteName, remote string) *exec.Cmd {
	return exec.Command(
		"git",
		"remote",
		"add",
		remoteName,
		remote,
	)
}

// GitRemoteSetUrl sets the remote origin url for the repository.
func GitRemoteSetUrl(remoteName, remote string) *exec.Cmd {
	return exec.Command(
		"git",
		"remote",
		"set-url",
		remoteName,
		remote,
	)
}

// GitRemoteRemove removes the remote origin for the repository.
func GitRemoteRemove(remoteName string) *exec.Cmd {
	return exec.Command(
		"git",
		"remote",
		"remove",
		remoteName,
	)
}

// GitCheckoutHead returns command git checkout -qf FETCH_HEAD
func GitCheckoutHead() *exec.Cmd {
	return exec.Command(
		"git",
		"checkout",
		"-qf",
		"FETCH_HEAD",
	)
}

func GitCheckoutCommit(commit string) *exec.Cmd {
	return exec.Command(
		"git",
		"checkout",
		commit,
	)
}

// GitFetch fetches changes by ref, ref can be a tag, branch or pr. --depth=1 is used to limit fetching
// to the last commit from the tip of each remote branch history.
// e.g. git fetch origin +refs/heads/onboarding --depth=1
func GitFetch(remoteName, ref string) *exec.Cmd {
	return exec.Command(
		"git",
		"fetch",
		remoteName,
		"+"+ref, // "+" means overwrite
		"--depth=1",
	)
}

// GitDeepenedFetch deepens the fetch history. It is similar with Fetch but accepts 500 more commit history than
// last Fetch operation by --deepen=500 option.
// e.g. git fetch origin +refs/heads/onboarding --deepen=500
func GitDeepenedFetch(remoteName, ref, source string) *exec.Cmd {
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

// GitResetMerge reset last merge
// It return command git reset --merge
func GitResetMerge() *exec.Cmd {
	return exec.Command(
		"git",
		"reset",
		"--merge",
	)

}

// GitMerge merge a branch
// e.g. git merge demo
func GitMerge(branch string) *exec.Cmd {
	return exec.Command(
		"git",
		"merge",
		branch,
		"--allow-unrelated-histories", // add this option to avoid "fatal: refusing to merge unrelated histories"
	)
}

// GitUpdateSubmodules returns command: git submodule update --init --recursive
func GitUpdateSubmodules() *exec.Cmd {
	cmd := exec.Command(
		"git",
		"submodule",
		"update",
		"--init",
		"--recursive",
		"--quiet",
	)
	return cmd
}

// GitSetConfig returns command: git config --global $KEY $VA
// e.g. git config --global user.name username
func GitSetConfig(key, value string) *exec.Cmd {
	return exec.Command(
		"git",
		"config",
		"--global",
		key,
		value,
	)
}

// GitGc returns command: git gc
func GitGc() *exec.Cmd {
	return exec.Command(
		"git",
		"gc",
	)
}

// GitShowLastLog returns command git --no-pager log --oneline -1
// It shows last commit messge with sha
func GitShowLastLog() *exec.Cmd {
	return exec.Command(
		"git",
		"--no-pager",
		"log",
		"--oneline",
		"-1",
	)
}

// GitListRemoteBranch returns command: git ls-remote --heads
func GitListRemoteBranch(remote string) *exec.Cmd {
	return exec.Command(
		"git",
		"ls-remote",
		"--heads",
		remote,
	)
}

// GitListRemoteTags returns command: git ls-remote --tags --refs
// --refs: only show refs, not peeled tags (^{}), which reduces output and improves performance
func GitListRemoteTags(remote string) *exec.Cmd {
	return exec.Command(
		"git",
		"ls-remote",
		"--tags",
		"--refs",
		remote,
	)
}
