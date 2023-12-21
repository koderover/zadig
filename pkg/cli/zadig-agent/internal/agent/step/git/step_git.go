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

package git

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/util/sets"

	gitcmd "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/git"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/step/helper"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common"
	agenttypes "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common/types"
	"github.com/koderover/zadig/v2/pkg/microservice/jobexecutor/config"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

type GitStep struct {
	spec       *step.StepGitSpec
	envs       []string
	secretEnvs []string
	dirs       *agenttypes.AgentWorkDirs
	Logger     *log.JobLogger
}

func NewGitStep(spec interface{}, dirs *agenttypes.AgentWorkDirs, envs, secretEnvs []string, logger *log.JobLogger) (*GitStep, error) {
	gitStep := &GitStep{dirs: dirs, envs: envs, secretEnvs: secretEnvs, Logger: logger}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return gitStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &gitStep.spec); err != nil {
		return gitStep, fmt.Errorf("unmarshal spec %s to script spec failed", yamlBytes)
	}
	return gitStep, nil
}

func (s *GitStep) Run(ctx context.Context) error {
	start := time.Now()
	s.Logger.Infof("Start git clone.")
	defer func() {
		s.Logger.Infof(fmt.Sprintf("Git clone ended. Duration: %.2f seconds.", time.Since(start).Seconds()))
	}()
	return s.runGitCmds()
}

func (s *GitStep) runGitCmds() error {
	// 获取git代码
	cmds := make([]*common.Command, 0)

	var hostNames = sets.NewString()
	for _, repo := range s.spec.Repos {
		if repo == nil || len(repo.RepoName) == 0 {
			continue
		}

		cmds = append(cmds, s.buildGitCommands(repo, hostNames)...)
	}

	// https://stackoverflow.com/questions/24952683/git-push-error-rpc-failed-result-56-http-code-200-fatal-the-remote-end-hun/36843260
	//cmds = append(cmds, &command.Command{Cmd: gitcmd.SetConfig("http.postBuffer", "524288000"), DisableTrace: true})
	for _, c := range cmds {
		cmdOutReader, err := c.Cmd.StdoutPipe()
		if err != nil {
			return err
		}
		cmdErrReader, err := c.Cmd.StderrPipe()
		if err != nil {
			return err
		}

		c.Cmd.Env = s.envs
		if !c.DisableTrace {
			s.Logger.Printf("%s\n", strings.Join(c.Cmd.Args, " "))
		}
		if err := c.Cmd.Start(); err != nil {
			if c.IgnoreError {
				continue
			}
			return err
		}

		var wg sync.WaitGroup
		needPersistentLog := true
		// write script output to log file
		wg.Add(1)
		go func() {
			defer wg.Done()

			helper.HandleCmdOutput(cmdOutReader, needPersistentLog, s.Logger.GetLogfilePath(), s.secretEnvs, log.GetSimpleLogger())
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()

			helper.HandleCmdOutput(cmdErrReader, needPersistentLog, s.Logger.GetLogfilePath(), s.secretEnvs, log.GetSimpleLogger())
		}()

		wg.Wait()
		if err := c.Cmd.Wait(); err != nil {
			if c.IgnoreError {
				continue
			}
			return err
		}
	}
	return nil
}

func (s *GitStep) buildGitCommands(repo *types.Repository, hostNames sets.String) []*common.Command {

	cmds := make([]*common.Command, 0)

	if len(repo.RepoName) == 0 {
		return cmds
	}

	workDir := filepath.Join(s.dirs.Workspace, repo.RepoName)
	if len(repo.CheckoutPath) != 0 {
		workDir = filepath.Join(s.dirs.Workspace, repo.CheckoutPath)
	}

	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		err = os.MkdirAll(workDir, 0777)
		if err != nil {
			s.Logger.Errorf("Failed to create dir %s: %v", workDir, err)
		}
	}

	// 预防非正常退出导致git被锁住
	indexLockPath := path.Join(workDir, "/.git/index.lock")
	if err := os.RemoveAll(indexLockPath); err != nil {
		s.Logger.Errorf("Failed to remove %s: %s", indexLockPath, err)
	}
	shallowLockPath := path.Join(workDir, "/.git/shallow.lock")
	if err := os.RemoveAll(shallowLockPath); err != nil {
		s.Logger.Errorf("Failed to remove %s: %s", shallowLockPath, err)
	}

	if helper.IsDirEmpty(filepath.Join(workDir, ".git")) {
		cmds = append(cmds, &common.Command{Cmd: gitcmd.InitGit(workDir)})
	} else {
		cmds = append(cmds, &common.Command{Cmd: gitcmd.RemoteRemove(repo.RemoteName), DisableTrace: true, IgnoreError: true})
	}

	if runtime.GOOS == "windows" {
		cmds = append(cmds, &common.Command{Cmd: gitcmd.SetConfig("user.email", "zadig@koderover.com"), DisableTrace: true})
	}

	// namespace represents the real owner
	owner := repo.RepoNamespace
	if len(owner) == 0 {
		owner = repo.RepoOwner
	}
	if repo.Source == types.ProviderGitlab {
		u, _ := url.Parse(repo.Address)
		host := strings.TrimSuffix(strings.Join([]string{u.Host, u.Path}, "/"), "/")
		cmds = append(cmds, &common.Command{
			Cmd:          gitcmd.RemoteAdd(repo.RemoteName, OAuthCloneURL(repo.Source, repo.OauthToken, host, owner, repo.RepoName, u.Scheme)),
			DisableTrace: true,
		})
	} else if repo.Source == types.ProviderGerrit {
		u, _ := url.Parse(repo.Address)
		u.Path = fmt.Sprintf("/a/%s", repo.RepoName)
		u.User = url.UserPassword(repo.Username, repo.Password)

		cmds = append(cmds, &common.Command{
			Cmd:          gitcmd.RemoteAdd(repo.RemoteName, u.String()),
			DisableTrace: true,
		})
	} else if repo.Source == common.ProviderCodehub {
		u, _ := url.Parse(repo.Address)
		host := strings.TrimSuffix(strings.Join([]string{u.Host, u.Path}, "/"), "/")
		user := url.QueryEscape(repo.Username)
		cmds = append(cmds, &common.Command{
			Cmd:          gitcmd.RemoteAdd(repo.RemoteName, fmt.Sprintf("%s://%s:%s@%s/%s/%s.git", u.Scheme, user, repo.Password, host, owner, repo.RepoName)),
			DisableTrace: true,
		})
	} else if repo.Source == types.ProviderGitee || repo.Source == types.ProviderGiteeEE {
		cmds = append(cmds, &common.Command{Cmd: gitcmd.RemoteAdd(repo.RemoteName, HTTPSCloneURL(repo.Source, repo.OauthToken, repo.RepoOwner, repo.RepoName, repo.Address)), DisableTrace: true})
	} else if repo.Source == types.ProviderOther {
		if repo.AuthType == types.SSHAuthType {
			host := getHost(repo.Address)
			if !hostNames.Has(host) {
				if err := writeSSHFile(repo.SSHKey, host); err != nil {
					s.Logger.Errorf("failed to write ssh file %s: %s", repo.SSHKey, err)
				}
				hostNames.Insert(host)
			}
			remoteName := fmt.Sprintf("%s:%s/%s.git", repo.Address, repo.RepoOwner, repo.RepoName)
			// Including the case of the port
			if strings.Contains(repo.Address, ":") {
				remoteName = fmt.Sprintf("%s/%s/%s.git", repo.Address, repo.RepoOwner, repo.RepoName)
			}
			cmds = append(cmds, &common.Command{
				Cmd:          gitcmd.RemoteAdd(repo.RemoteName, remoteName),
				DisableTrace: true,
			})
		} else if repo.AuthType == types.PrivateAccessTokenAuthType {
			u, err := url.Parse(repo.Address)
			if err != nil {
				s.Logger.Errorf("failed to parse url,err:%s", err)
			} else {
				host := strings.TrimSuffix(strings.Join([]string{u.Host, u.Path}, "/"), "/")
				cmds = append(cmds, &common.Command{
					Cmd:          gitcmd.RemoteAdd(repo.RemoteName, OAuthCloneURL(repo.Source, repo.PrivateAccessToken, host, repo.RepoOwner, repo.RepoName, u.Scheme)),
					DisableTrace: true,
				})
			}
		}
	} else {
		// github
		cmds = append(cmds, &common.Command{Cmd: gitcmd.RemoteAdd(repo.RemoteName, HTTPSCloneURL(repo.Source, repo.OauthToken, owner, repo.RepoName, "")), DisableTrace: true})
	}

	ref := repo.Ref()
	if ref == "" {
		return cmds
	}

	cmds = append(cmds, &common.Command{Cmd: gitcmd.Fetch(repo.RemoteName, ref)}, &common.Command{Cmd: gitcmd.CheckoutHead()})

	// PR rebase branch 请求
	if len(repo.PRs) > 0 && len(repo.Branch) > 0 {
		cmds = append(
			cmds,
			&common.Command{Cmd: gitcmd.DeepenedFetch(repo.RemoteName, repo.BranchRef(), repo.Source)},
			&common.Command{Cmd: gitcmd.ResetMerge()},
		)
		for _, pr := range repo.PRs {
			newBranch := fmt.Sprintf("pr%d", pr)
			ref := fmt.Sprintf("%s:%s", repo.PRRefByPRID(pr), newBranch)
			cmds = append(
				cmds,
				&common.Command{Cmd: gitcmd.DeepenedFetch(repo.RemoteName, ref, repo.Source)},
				&common.Command{Cmd: gitcmd.Merge(newBranch)},
			)
		}
	}

	if repo.SubModules {
		cmds = append(cmds, &common.Command{Cmd: gitcmd.UpdateSubmodules()})
	}

	cmds = append(cmds, &common.Command{Cmd: gitcmd.ShowLastLog()})

	helper.SetCmdsWorkDir(workDir, cmds)

	return cmds
}

func writeSSHFile(sshKey, hostName string) error {
	if sshKey == "" {
		return fmt.Errorf("ssh cannot be empty")
	}

	if hostName == "" {
		return fmt.Errorf("hostName cannot be empty")
	}

	hostName = strings.Replace(hostName, ".", "", -1)
	hostName = strings.Replace(hostName, ":", "", -1)
	pathName := fmt.Sprintf("/.ssh/id_rsa.%s", hostName)
	file := path.Join(config.Home(), pathName)
	return ioutil.WriteFile(file, []byte(sshKey), 0400)
}

func writeSSHConfigFile(hostNames sets.String, proxy *step.Proxy) error {
	out := "\nHOST *\nStrictHostKeyChecking=no\nUserKnownHostsFile=/dev/null\n"
	for _, hostName := range hostNames.List() {
		name := strings.Replace(hostName, ".", "", -1)
		name = strings.Replace(name, ":", "", -1)
		out += fmt.Sprintf("\nHost %s\nIdentityFile ~/.ssh/id_rsa.%s\n", hostName, name)
		if proxy.EnableRepoProxy && proxy.Type == "socks5" {
			out = out + fmt.Sprintf("ProxyCommand nc -x %s %%h %%p\n", proxy.GetProxyURL())
		}
	}
	file := path.Join(config.Home(), "/.ssh/config")
	return ioutil.WriteFile(file, []byte(out), 0600)
}

// git@github.com or git@github.com:2000
// return github.com
func getHost(address string) string {
	address = strings.TrimPrefix(address, "ssh://")
	address = strings.TrimPrefix(address, "git@")
	hostArr := strings.Split(address, ":")
	return hostArr[0]
}

// SSHCloneURL returns Oauth clone url
// e.g.
// https://oauth2:ACCESS_TOKEN@somegitlab.com/owner/name.git
func OAuthCloneURL(source, token, address, owner, name, scheme string) string {
	if strings.ToLower(source) == types.ProviderGitlab || strings.ToLower(source) == types.ProviderOther {
		// address 需要传过来
		return fmt.Sprintf("%s://%s:%s@%s/%s/%s.git", scheme, step.OauthTokenPrefix, token, address, owner, name)
	}
	//	GITHUB
	return "github"
}

// HTTPSCloneURL returns HTTPS clone url
func HTTPSCloneURL(source, token, owner, name string, optionalGiteeAddr string) string {
	if strings.ToLower(source) == types.ProviderGitee || strings.ToLower(source) == types.ProviderGiteeEE {
		addrSegment := strings.Split(optionalGiteeAddr, "://")
		return fmt.Sprintf("%s://%s:%s@%s/%s/%s.git", addrSegment[0], step.OauthTokenPrefix, token, addrSegment[1], owner, name)
	}
	//return fmt.Sprintf("https://x-access-token:%s@%s/%s/%s.git", g.GetInstallationToken(owner), g.GetGithubHost(), owner, name)
	return fmt.Sprintf("https://x-access-token:%s@%s/%s/%s.git", token, "github.com", owner, name)
}

func maskSecret(secrets []string, message string) string {
	out := message

	for _, val := range secrets {
		if len(val) == 0 {
			continue
		}
		out = strings.Replace(out, val, "********", -1)
	}
	return out
}
