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

package step

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/jobexecutor/config"
	c "github.com/koderover/zadig/pkg/microservice/jobexecutor/core/service/cmd"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/types/step"
)

type GitStep struct {
	spec       *step.StepGitSpec
	envs       []string
	secretEnvs []string
	workspace  string
}

func NewGitStep(spec interface{}, workspace string, envs, secretEnvs []string) (*GitStep, error) {
	gitStep := &GitStep{workspace: workspace, envs: envs, secretEnvs: secretEnvs}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return gitStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &gitStep.spec); err != nil {
		return gitStep, fmt.Errorf("unmarshal spec %s to shell spec failed", yamlBytes)
	}
	return gitStep, nil
}

func (s *GitStep) Run(ctx context.Context) error {
	start := time.Now()
	log.Infof("Start git clone.")
	defer func() {
		log.Infof("Git clone ended. Duration: %.2f seconds.", time.Since(start).Seconds())
	}()
	return s.runGitCmds()
}

func (s *GitStep) RunGitGc(folder string) error {
	// envs := r.getUserEnvs()
	cmd := c.Gc()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// cmd.Env = envs
	cmd.Dir = path.Join(s.workspace, folder)
	return cmd.Run()
}

func (s *GitStep) runGitCmds() error {
	if err := os.MkdirAll(path.Join(config.Home(), "/.ssh"), os.ModePerm); err != nil {
		return fmt.Errorf("create ssh folder error: %v", err)
	}
	envs := s.envs
	// 如果存在github代码库，则设置代理，同时保证非github库不走代理
	if s.spec.Proxy != nil && s.spec.Proxy.EnableRepoProxy && s.spec.Proxy.Type == "http" {
		noProxy := ""
		proxyFlag := false
		for _, repo := range s.spec.Repos {
			if repo.EnableProxy {
				if !proxyFlag {
					envs = append(envs, fmt.Sprintf("http_proxy=%s", s.spec.Proxy.GetProxyURL()))
					envs = append(envs, fmt.Sprintf("https_proxy=%s", s.spec.Proxy.GetProxyURL()))
					proxyFlag = true
				}
			} else {
				uri, err := url.Parse(repo.Address)
				if err == nil {
					if noProxy != "" {
						noProxy += ","
					}
					noProxy += uri.Host
				}
			}
		}
		envs = append(envs, fmt.Sprintf("no_proxy=%s", noProxy))
	}

	// 获取git代码
	cmds := make([]*c.Command, 0)

	cmds = append(cmds, &c.Command{Cmd: c.SetConfig("user.email", "ci@koderover.com"), DisableTrace: true})
	cmds = append(cmds, &c.Command{Cmd: c.SetConfig("user.name", "koderover"), DisableTrace: true})

	// https://stackoverflow.com/questions/24952683/git-push-error-rpc-failed-result-56-http-code-200-fatal-the-remote-end-hun/36843260
	//cmds = append(cmds, &c.Command{Cmd: c.SetConfig("http.postBuffer", "524288000"), DisableTrace: true})
	cmds = append(cmds, &c.Command{Cmd: c.SetConfig("http.postBuffer", "2097152000"), DisableTrace: true})
	var tokens []string
	var hostNames = sets.NewString()
	for _, repo := range s.spec.Repos {
		if repo == nil || len(repo.RepoName) == 0 {
			continue
		}

		if repo.Source == types.ProviderGerrit {
			userpass, _ := base64.StdEncoding.DecodeString(repo.OauthToken)
			userpassPair := strings.Split(string(userpass), ":")
			var user, password string
			if len(userpassPair) > 1 {
				password = userpassPair[1]
			}
			user = userpassPair[0]
			repo.Username = user
			if password != "" {
				repo.Password = password
				tokens = append(tokens, repo.Password)
			}
		} else if repo.Source == types.ProviderCodehub {
			tokens = append(tokens, repo.Password)
		} else if repo.Source == types.ProviderOther {
			tokens = append(tokens, repo.PrivateAccessToken)
			tokens = append(tokens, repo.SSHKey)
		}
		tokens = append(tokens, repo.OauthToken)
		cmds = append(cmds, s.buildGitCommands(repo, hostNames)...)
	}
	// write ssh key
	if len(hostNames.List()) > 0 {
		if err := writeSSHConfigFile(hostNames, s.spec.Proxy); err != nil {
			return err
		}
	}

	for _, c := range cmds {
		cmdOutReader, err := c.Cmd.StdoutPipe()
		if err != nil {
			return err
		}

		outScanner := bufio.NewScanner(cmdOutReader)
		go func() {
			for outScanner.Scan() {
				fmt.Printf("%s\n", maskSecret(tokens, outScanner.Text()))
			}
		}()

		cmdErrReader, err := c.Cmd.StderrPipe()
		if err != nil {
			return err
		}

		errScanner := bufio.NewScanner(cmdErrReader)
		go func() {
			for errScanner.Scan() {
				fmt.Printf("%s\n", maskSecret(tokens, errScanner.Text()))
			}
		}()

		c.Cmd.Env = envs
		if !c.DisableTrace {
			fmt.Printf("%s\n", strings.Join(c.Cmd.Args, " "))
		}
		if err := c.Cmd.Run(); err != nil {
			if c.IgnoreError {
				continue
			}
			return err
		}
	}
	return nil
}

func (s *GitStep) buildGitCommands(repo *types.Repository, hostNames sets.String) []*c.Command {

	cmds := make([]*c.Command, 0)

	if len(repo.RepoName) == 0 {
		return cmds
	}

	workDir := filepath.Join(s.workspace, repo.RepoName)
	if len(repo.CheckoutPath) != 0 {
		workDir = filepath.Join(s.workspace, repo.CheckoutPath)
	}

	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		os.MkdirAll(workDir, 0777)
	}

	// 预防非正常退出导致git被锁住
	indexLockPath := path.Join(workDir, "/.git/index.lock")
	if err := os.RemoveAll(indexLockPath); err != nil {
		log.Errorf("Failed to remove %s: %s", indexLockPath, err)
	}
	shallowLockPath := path.Join(workDir, "/.git/shallow.lock")
	if err := os.RemoveAll(shallowLockPath); err != nil {
		log.Errorf("Failed to remove %s: %s", shallowLockPath, err)
	}

	if isDirEmpty(filepath.Join(workDir, ".git")) {
		cmds = append(cmds, &c.Command{Cmd: c.InitGit(workDir)})
	} else {
		cmds = append(cmds, &c.Command{Cmd: c.RemoteRemove(repo.RemoteName), DisableTrace: true, IgnoreError: true})
	}

	// namespace represents the real owner
	owner := repo.RepoNamespace
	if len(owner) == 0 {
		owner = repo.RepoOwner
	}
	if repo.Source == types.ProviderGitlab {
		u, _ := url.Parse(repo.Address)
		host := strings.TrimSuffix(strings.Join([]string{u.Host, u.Path}, "/"), "/")
		cmds = append(cmds, &c.Command{
			Cmd:          c.RemoteAdd(repo.RemoteName, OAuthCloneURL(repo.Source, repo.OauthToken, host, owner, repo.RepoName, u.Scheme)),
			DisableTrace: true,
		})
	} else if repo.Source == types.ProviderGerrit {
		u, _ := url.Parse(repo.Address)
		u.Path = fmt.Sprintf("/a/%s", repo.RepoName)
		u.User = url.UserPassword(repo.Username, repo.Password)

		cmds = append(cmds, &c.Command{
			Cmd:          c.RemoteAdd(repo.RemoteName, u.String()),
			DisableTrace: true,
		})
	} else if repo.Source == types.ProviderCodehub {
		u, _ := url.Parse(repo.Address)
		host := strings.TrimSuffix(strings.Join([]string{u.Host, u.Path}, "/"), "/")
		user := url.QueryEscape(repo.Username)
		cmds = append(cmds, &c.Command{
			Cmd:          c.RemoteAdd(repo.RemoteName, fmt.Sprintf("%s://%s:%s@%s/%s/%s.git", u.Scheme, user, repo.Password, host, owner, repo.RepoName)),
			DisableTrace: true,
		})
	} else if repo.Source == types.ProviderGitee || repo.Source == types.ProviderGiteeEE {
		cmds = append(cmds, &c.Command{Cmd: c.RemoteAdd(repo.RemoteName, HTTPSCloneURL(repo.Source, repo.OauthToken, repo.RepoOwner, repo.RepoName, repo.Address)), DisableTrace: true})
	} else if repo.Source == types.ProviderOther {
		if repo.AuthType == types.SSHAuthType {
			host := getHost(repo.Address)
			if !hostNames.Has(host) {
				if err := writeSSHFile(repo.SSHKey, host); err != nil {
					log.Errorf("failed to write ssh file %s: %s", repo.SSHKey, err)
				}
				hostNames.Insert(host)
			}
			remoteName := fmt.Sprintf("%s:%s/%s.git", repo.Address, repo.RepoOwner, repo.RepoName)
			// Including the case of the port
			if strings.Contains(repo.Address, ":") {
				remoteName = fmt.Sprintf("%s/%s/%s.git", repo.Address, repo.RepoOwner, repo.RepoName)
			}
			cmds = append(cmds, &c.Command{
				Cmd:          c.RemoteAdd(repo.RemoteName, remoteName),
				DisableTrace: true,
			})
		} else if repo.AuthType == types.PrivateAccessTokenAuthType {
			u, err := url.Parse(repo.Address)
			if err != nil {
				log.Errorf("failed to parse url,err:%s", err)
			} else {
				host := strings.TrimSuffix(strings.Join([]string{u.Host, u.Path}, "/"), "/")
				cmds = append(cmds, &c.Command{
					Cmd:          c.RemoteAdd(repo.RemoteName, OAuthCloneURL(repo.Source, repo.PrivateAccessToken, host, repo.RepoOwner, repo.RepoName, u.Scheme)),
					DisableTrace: true,
				})
			}
		}
	} else {
		// github
		cmds = append(cmds, &c.Command{Cmd: c.RemoteAdd(repo.RemoteName, HTTPSCloneURL(repo.Source, repo.OauthToken, owner, repo.RepoName, "")), DisableTrace: true})
	}

	ref := repo.Ref()
	if ref == "" {
		return cmds
	}

	cmds = append(cmds, &c.Command{Cmd: c.Fetch(repo.RemoteName, ref)}, &c.Command{Cmd: c.CheckoutHead()})

	// PR rebase branch 请求
	if len(repo.PRs) > 0 && len(repo.Branch) > 0 {
		cmds = append(
			cmds,
			&c.Command{Cmd: c.DeepenedFetch(repo.RemoteName, repo.BranchRef(), repo.Source)},
			&c.Command{Cmd: c.ResetMerge()},
		)
		for _, pr := range repo.PRs {
			newBranch := fmt.Sprintf("pr%d", pr)
			ref := fmt.Sprintf("%s:%s", repo.PRRefByPRID(pr), newBranch)
			cmds = append(
				cmds,
				&c.Command{Cmd: c.DeepenedFetch(repo.RemoteName, ref, repo.Source)},
				&c.Command{Cmd: c.Merge(newBranch)},
			)
		}
	}

	if repo.SubModules {
		cmds = append(cmds, &c.Command{Cmd: c.UpdateSubmodules()})
	}

	cmds = append(cmds, &c.Command{Cmd: c.ShowLastLog()})

	setCmdsWorkDir(workDir, cmds)

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
