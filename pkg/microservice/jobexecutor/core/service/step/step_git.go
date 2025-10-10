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

	"github.com/koderover/zadig/v2/pkg/microservice/jobexecutor/config"
	c "github.com/koderover/zadig/v2/pkg/microservice/jobexecutor/core/service/cmd"
	codehostmodels "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	gittool "github.com/koderover/zadig/v2/pkg/tool/git"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
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
		return gitStep, fmt.Errorf("unmarshal spec %s to git spec failed", yamlBytes)
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
	cmd := c.GitGc()
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

	cmds = append(cmds, &c.Command{Cmd: c.GitSetConfig("user.email", "ci@koderover.com"), DisableTrace: true})
	cmds = append(cmds, &c.Command{Cmd: c.GitSetConfig("user.name", "koderover"), DisableTrace: true})

	// https://stackoverflow.com/questions/24952683/git-push-error-rpc-failed-result-56-http-code-200-fatal-the-remote-end-hun/36843260
	cmds = append(cmds, &c.Command{Cmd: c.GitSetConfig("http.postBuffer", "2097152000"), DisableTrace: true})
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
		} else if repo.Source == types.ProviderOther {
			tokens = append(tokens, repo.PrivateAccessToken)
			tokens = append(tokens, repo.SSHKey)
		}

		tokens = append(tokens, repo.OauthToken)
		cmds = append(cmds, s.buildGitCommands(repo, hostNames)...)
	}
	// write ssh key
	if err := writeSSHConfigFile(hostNames, s.spec.Proxy); err != nil {
		return err
	}

	for _, c := range cmds {
		cmdOutReader, err := c.Cmd.StdoutPipe()
		if err != nil {
			return err
		}

		outScanner := bufio.NewScanner(cmdOutReader)
		go func() {
			for outScanner.Scan() {
				fmt.Printf("%s   %s\n", time.Now().Format(setting.WorkflowTimeFormat), util.MaskSecret(tokens, outScanner.Text()))
			}
		}()

		cmdErrReader, err := c.Cmd.StderrPipe()
		if err != nil {
			return err
		}

		errScanner := bufio.NewScanner(cmdErrReader)
		go func() {
			for errScanner.Scan() {
				fmt.Printf("%s   %s\n", time.Now().Format(setting.WorkflowTimeFormat), util.MaskSecret(tokens, errScanner.Text()))
			}
		}()

		c.Cmd.Env = envs
		if !c.DisableTrace {
			fmt.Printf("%s   %s\n", time.Now().Format(setting.WorkflowTimeFormat), strings.Join(c.Cmd.Args, " "))
		}

		if err := c.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (s *GitStep) GetWorkDir(repo *types.Repository) string {
	workDir := filepath.Join(s.workspace, repo.RepoName)
	if len(repo.CheckoutPath) != 0 {
		workDir = filepath.Join(s.workspace, repo.CheckoutPath)
	}
	return workDir
}

func (s *GitStep) buildGitCommands(repo *types.Repository, hostNames sets.String) []*c.Command {

	cmds := make([]*c.Command, 0)

	if len(repo.RepoName) == 0 {
		return cmds
	}

	workDir := s.GetWorkDir(repo)
	defer func() {
		setCmdsWorkDir(workDir, cmds)
	}()

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
		cmds = append(cmds, &c.Command{Cmd: c.GitRemoteRemove(repo.RemoteName), DisableTrace: true, IgnoreError: true})
	}

	// namespace represents the real owner
	owner := repo.RepoNamespace
	if len(owner) == 0 {
		owner = repo.RepoOwner
	}
	if repo.Source == types.ProviderGitlab {
		// gitlab
		u, _ := url.Parse(repo.Address)
		host := strings.TrimSuffix(strings.Join([]string{u.Host, u.Path}, "/"), "/")
		cmds = append(cmds, &c.Command{
			Cmd:          c.GitRemoteAdd(repo.RemoteName, OAuthCloneURL(repo.Source, repo.OauthToken, host, owner, repo.RepoName, u.Scheme)),
			DisableTrace: true,
		})
	} else if repo.Source == types.ProviderGerrit {
		// gerrit
		u, _ := url.Parse(repo.Address)
		u.Path = fmt.Sprintf("/a/%s", repo.RepoName)
		u.User = url.UserPassword(repo.Username, repo.Password)

		cmds = append(cmds, &c.Command{
			Cmd:          c.GitRemoteAdd(repo.RemoteName, u.String()),
			DisableTrace: true,
		})
	} else if repo.Source == types.ProviderGitee || repo.Source == types.ProviderGiteeEE {
		// gitee
		cmds = append(cmds, &c.Command{Cmd: c.GitRemoteAdd(repo.RemoteName, HTTPSCloneURL(repo.Source, repo.OauthToken, owner, repo.RepoName, repo.Address)), DisableTrace: true})
	} else if repo.Source == types.ProviderOther {
		// other
		if repo.AuthType == types.SSHAuthType {
			host := getHost(repo.Address)
			if !hostNames.Has(host) {
				if err := writeSSHFile(repo.SSHKey, host); err != nil {
					log.Errorf("failed to write ssh file, err: %v", err)
				}
				hostNames.Insert(host)
			}
			remoteName := fmt.Sprintf("%s:%s/%s.git", repo.Address, repo.RepoOwner, repo.RepoName)
			// Including the case of the port
			if strings.Contains(repo.Address, ":") {
				remoteName = fmt.Sprintf("%s/%s/%s.git", repo.Address, repo.RepoOwner, repo.RepoName)
			}
			cmds = append(cmds, &c.Command{
				Cmd:          c.GitRemoteAdd(repo.RemoteName, remoteName),
				DisableTrace: true,
			})
		} else if repo.AuthType == types.PrivateAccessTokenAuthType {
			// private accessToken auth
			u, err := url.Parse(repo.Address)
			if err != nil {
				log.Errorf("failed to parse url,err:%s", err)
			} else {
				host := strings.TrimSuffix(strings.Join([]string{u.Host, u.Path}, "/"), "/")
				cmds = append(cmds, &c.Command{
					Cmd:          c.GitRemoteAdd(repo.RemoteName, OAuthCloneURL(repo.Source, repo.PrivateAccessToken, host, repo.RepoOwner, repo.RepoName, u.Scheme)),
					DisableTrace: true,
				})
			}
		}
	} else {
		// github
		cmds = append(cmds, &c.Command{Cmd: c.GitRemoteAdd(repo.RemoteName, HTTPSCloneURL(repo.Source, repo.OauthToken, owner, repo.RepoName, "")), DisableTrace: true})
	}

	ref := repo.Ref()
	if ref == "" {
		return cmds
	}

	cmds = append(cmds, &c.Command{Cmd: c.GitFetch(repo.RemoteName, ref)}, &c.Command{Cmd: c.GitCheckoutHead()})

	// PR rebase branch 请求
	if len(repo.MergeBranches) > 0 {
		cmds = append(
			cmds,
			&c.Command{Cmd: c.GitDeepenedFetch(repo.RemoteName, repo.BranchRef(), repo.Source)},
			&c.Command{Cmd: c.GitResetMerge()},
		)
		for _, branch := range repo.MergeBranches {
			ref := fmt.Sprintf("%s:%s", types.BranchRef(branch), branch)
			cmds = append(
				cmds,
				&c.Command{Cmd: c.GitDeepenedFetch(repo.RemoteName, ref, repo.Source)},
				&c.Command{Cmd: c.GitMerge(branch)},
			)
		}
	} else if len(repo.PRs) > 0 && len(repo.Branch) > 0 {
		cmds = append(
			cmds,
			&c.Command{Cmd: c.GitDeepenedFetch(repo.RemoteName, repo.BranchRef(), repo.Source)},
			&c.Command{Cmd: c.GitResetMerge()},
		)
		for _, pr := range repo.PRs {
			newBranch := fmt.Sprintf("pr%d", pr)
			ref := fmt.Sprintf("%s:%s", repo.PRRefByPRID(pr), newBranch)
			cmds = append(
				cmds,
				&c.Command{Cmd: c.GitDeepenedFetch(repo.RemoteName, ref, repo.Source)},
				&c.Command{Cmd: c.GitMerge(newBranch)},
			)
		}
	}

	if repo.SubModules {
		cmd := &c.Command{
			Cmd:       c.GitUpdateSubmodules(),
			BeforeRun: AddOAuthInSubmoduleURLs,
			BeforeRunArgs: []interface{}{
				s.GetWorkDir(repo),
				repo,
				s.spec.CodeHosts,
			},
		}
		cmds = append(cmds, cmd)
	}

	cmds = append(cmds, &c.Command{Cmd: c.GitShowLastLog()})

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
	addressArr := strings.Split(address, "@")
	if len(addressArr) == 2 {
		address = addressArr[1]
	}
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

func AddOAuthInSubmoduleURLs(args ...interface{}) error {
	if len(args) != 3 {
		return fmt.Errorf("invalid args length: %d", len(args))
	}

	repoPath, ok := args[0].(string)
	if !ok {
		return fmt.Errorf("invalid args[0] type: %T", args[0])
	}
	mainRepo, ok := args[1].(*types.Repository)
	if !ok {
		return fmt.Errorf("invalid args[1] type: %T", args[1])
	}
	codeHosts, ok := args[2].([]*codehostmodels.CodeHost)
	if !ok {
		return fmt.Errorf("invalid args[2] type: %T", args[2])
	}

	repoURL, err := gittool.GetRepoUrl(repoPath)
	if err != nil {
		return fmt.Errorf("unable to get repository URL: %v", err)
	}

	submoduleURLMap, err := gittool.GetSubmoduleURLs(repoPath)
	if err != nil {
		return fmt.Errorf("unable to get submodule URLs: %v", err)
	}

	for name, url := range submoduleURLMap {
		newURL, err := convertToOAuthURL(repoURL, url, mainRepo, codeHosts)
		if err != nil {
			return fmt.Errorf("unable to convert submodule URL to oauth url: %v", err)
		}
		submoduleURLMap[name] = newURL
	}

	err = gittool.UpdateSubmoduleURLs(repoPath, submoduleURLMap)
	if err != nil {
		return fmt.Errorf("unable to update submodule URLs: %v", err)
	}

	return nil
}

func convertToOAuthURL(repoURL, submoduleURL string, mainRepo *types.Repository, codeHosts []*codehostmodels.CodeHost) (string, error) {
	if strings.HasPrefix(submoduleURL, "git@") {
		log.Warnf("unsupported submodule URL protocol: %s, use ssh private key", submoduleURL)
		return submoduleURL, nil
	}

	parsedUrl, err := url.Parse(submoduleURL)
	if err != nil {
		return "", fmt.Errorf("unable to parse URL: %v", err)
	}

	if parsedUrl.Scheme == "" {
		return submoduleURL, nil
		// // It's a relative path
		// if strings.HasPrefix(repoURL, "git@") {
		// 	log.Warnf("unsupported main repo URL protocol: %s, use ssh private key", submoduleURL)
		// 	return submoduleURL, nil
		// }

		// u, err := url.Parse(repoURL)
		// if err != nil {
		// 	return "", fmt.Errorf("unable to parse repository URL: %v", err)
		// }

		// // Convert relative path to absolute path
		// u.Path = path.Join(u.Path, submoduleURL)

		// for _, codeHost := range codeHosts {
		// 	u = setAuthInSubmoduleURL(u, codeHost)
		// }

		// return u.String(), nil
	} else if parsedUrl.Scheme == "http" || parsedUrl.Scheme == "https" {
		// It's an http(s) protocol
		for _, codeHost := range codeHosts {
			newUrl, err := setAuthInSubmoduleURL(parsedUrl, mainRepo, codeHost)
			if err == nil {
				return newUrl.String(), nil
			}
		}
		log.Warnf("no matching codehost found for submodule URL: %s", submoduleURL)
		return submoduleURL, nil
	} else if parsedUrl.Scheme == "ssh" {
		// not process ssh protocol
		return submoduleURL, nil
	} else {
		return "", fmt.Errorf("unsupported URL protocol: %s", parsedUrl.Scheme)
	}
}

func setAuthInSubmoduleURL(u *url.URL, mainRepo *types.Repository, codeHost *codehostmodels.CodeHost) (*url.URL, error) {
	if mainRepo.Source == types.ProviderGitlab || mainRepo.Source == types.ProviderGitee || mainRepo.Source == types.ProviderGiteeEE {
		if strings.HasPrefix(u.String(), mainRepo.Address) {
			u.User = url.UserPassword(step.OauthTokenPrefix, mainRepo.OauthToken)
			return u, nil
		}
	} else if mainRepo.Source == types.ProviderGerrit {
		if strings.HasPrefix(u.String(), mainRepo.Address) {
			u.User = url.UserPassword(mainRepo.Username, mainRepo.Password)
			return u, nil
		}
	} else if mainRepo.Source == types.ProviderOther {
		if mainRepo.AuthType == types.SSHAuthType {
			// don't process ssh protocol
		} else if mainRepo.AuthType == types.PrivateAccessTokenAuthType {
			if strings.HasPrefix(u.String(), mainRepo.Address) {
				u.User = url.UserPassword(step.OauthTokenPrefix, mainRepo.PrivateAccessToken)
				return u, nil
			}
		}
	} else if mainRepo.Source == types.ProviderGithub {
		if strings.HasPrefix(u.String(), mainRepo.Address) {
			u.User = url.UserPassword("x-access-token", mainRepo.OauthToken)
			return u, nil
		}
	}

	if codeHost.Type == types.ProviderGitlab || codeHost.Type == types.ProviderGitee || codeHost.Type == types.ProviderGiteeEE {
		if strings.HasPrefix(u.String(), codeHost.Address) {
			u.User = url.UserPassword(step.OauthTokenPrefix, codeHost.AccessToken)
			return u, nil
		}
	} else if codeHost.Type == types.ProviderGerrit {
		// gerrit
		if strings.HasPrefix(u.String(), codeHost.Address) {
			u.User = url.UserPassword(codeHost.Username, codeHost.Password)
			return u, nil
		}
	} else if codeHost.Type == types.ProviderOther {
		if codeHost.AuthType == types.SSHAuthType {
			// don't process ssh protocol
		} else if codeHost.AuthType == types.PrivateAccessTokenAuthType {
			if strings.HasPrefix(u.String(), codeHost.Address) {
				u.User = url.UserPassword(step.OauthTokenPrefix, codeHost.PrivateAccessToken)
				return u, nil
			}
		}
	} else if codeHost.Type == types.ProviderGithub {
		if strings.HasPrefix(u.String(), codeHost.Address) {
			u.User = url.UserPassword("x-access-token", codeHost.AccessToken)
			return u, nil
		}
	}

	return nil, fmt.Errorf("no matching repo/codehost found for submodule URL: %s", u.String())
}
