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

package command

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/codehost"
	"github.com/koderover/zadig/lib/microservice/aslan/internal/log"
	"github.com/koderover/zadig/lib/setting"
)

type Repo struct {
	Source       string `yaml:"source"`
	Address      string `yaml:"address"`
	Owner        string `yaml:"owner"`
	Name         string `yaml:"name"`
	RemoteName   string `yaml:"remote_name"`
	Branch       string `yaml:"branch"`
	PR           int    `yaml:"pr"`
	Tag          string `yaml:"tag"`
	CheckoutPath string `yaml:"checkout_path"`
	SubModules   bool   `yaml:"submodules"`
	OauthToken   string `yaml:"oauthToken"`
	User         string `yaml:"-"`
	Password     string `yaml:"-"`
	CheckoutRef  string `yaml:"checkout_ref"`
}

// BranchRef returns branch refs format
// e.g. refs/heads/master
func (r *Repo) BranchRef() string {
	return fmt.Sprintf("refs/heads/%s", r.Branch)
}

type Command struct {
	Cmd *exec.Cmd
	// DisableTrace display command args
	DisableTrace bool
	// IgnoreError ingore command run error
	IgnoreError bool
}

func RunGitCmds(codehostDetail *codehost.Detail, repoOwner, repoName, branchName, remoteName string) error {
	var (
		tokens []string
		repo   *Repo
		envs   = make([]string, 0)
		cmds   = make([]*Command, 0)
	)
	repo = &Repo{
		Source:     codehostDetail.Source,
		Address:    codehostDetail.Address,
		Name:       repoName,
		Branch:     branchName,
		OauthToken: codehostDetail.OauthToken,
		RemoteName: remoteName,
		Owner:      repoOwner,
	}

	userpass, _ := base64.StdEncoding.DecodeString(repo.OauthToken)
	userpassPair := strings.Split(string(userpass), ":")
	var user, password string
	if len(userpassPair) > 1 {
		password = userpassPair[1]
	}
	user = userpassPair[0]
	repo.User = user
	if password != "" {
		repo.Password = password
		tokens = append(tokens, repo.Password)
	}
	tokens = append(tokens, repo.OauthToken)
	cmds = append(cmds, buildGitCommands(repo)...)

	if repo.Source == setting.SourceFromGithub {
		proxies, _ := commonrepo.NewProxyColl().List(&commonrepo.ProxyArgs{})
		if len(proxies) > 0 {
			url := proxies[0].GetProxyUrl()
			envs = append(envs, fmt.Sprintf("http_proxy=%s", url))
			envs = append(envs, fmt.Sprintf("https_proxy=%s", url))
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
			log.Trace(c.Cmd)
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

func buildGitCommands(repo *Repo) []*Command {
	cmds := make([]*Command, 0)

	if len(repo.Name) == 0 {
		return cmds
	}

	repoName := repo.Name
	if strings.Contains(repoName, "/") {
		repoName = strings.Replace(repoName, "/", "-", -1)
	}
	workDir := filepath.Join(config.S3StoragePath(), repoName)
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		os.MkdirAll(workDir, 0777)
	}

	// 预防非正常退出导致git被锁住
	_ = os.Remove(path.Join(workDir, "/.git/index.lock"))

	if isDirEmpty(filepath.Join(workDir, ".git")) {
		cmds = append(cmds, &Command{Cmd: InitGit(workDir)})
	} else {
		cmds = append(cmds, &Command{Cmd: RemoteRemove(repo.RemoteName), DisableTrace: true, IgnoreError: true})
	}

	if repo.Source == setting.SourceFromGitlab {
		u, _ := url.Parse(repo.Address)
		url := OAuthCloneURL(repo.OauthToken, u.Host, repo.Owner, repo.Name, u.Scheme)
		cmds = append(cmds, &Command{
			Cmd:          RemoteAdd(repo.RemoteName, url),
			DisableTrace: true,
		})
	} else if repo.Source == setting.SourceFromGerrit {
		u, _ := url.Parse(repo.Address)
		u.Path = fmt.Sprintf("/a/%s", repo.Name)
		u.User = url.UserPassword(repo.User, repo.Password)

		cmds = append(cmds, &Command{
			Cmd:          RemoteAdd(repo.RemoteName, u.String()),
			DisableTrace: true,
		})
	} else {
		// github
		cmd := RemoteAdd(repo.RemoteName, fmt.Sprintf("https://x-access-token:%s@%s/%s/%s.git", repo.OauthToken, "github.com", repo.Owner, repo.Name))
		if repo.OauthToken == "" {
			cmd = RemoteAdd(repo.RemoteName, repo.Address)
		}
		cmds = append(cmds, &Command{
			Cmd:          cmd,
			DisableTrace: true,
		})
	}

	cmds = append(cmds, &Command{Cmd: Fetch(repo.RemoteName, repo.BranchRef())})
	cmds = append(cmds, &Command{Cmd: CheckoutHead()})
	cmds = append(cmds, &Command{Cmd: ShowLastLog()})

	setCmdsWorkDir(workDir, cmds)

	return cmds
}

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

// RemoteRemove removes the remote origin for the repository.
func RemoteRemove(remoteName string) *exec.Cmd {
	return exec.Command(
		"git",
		"remote",
		"remove",
		remoteName,
	)
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

// CheckoutHead returns command git checkout -qf FETCH_HEAD
func CheckoutHead() *exec.Cmd {
	return exec.Command(
		"git",
		"checkout",
		"-qf",
		"FETCH_HEAD",
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

func OAuthCloneURL(token, address, owner, name, scheme string) string {
	return fmt.Sprintf("%s://%s:%s@%s/%s/%s.git", scheme, "oauth2", token, address, owner, name)
}

func isDirEmpty(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return true
	}
	defer f.Close()

	_, err = f.Readdir(1)
	return err == io.EOF
}

func setCmdsWorkDir(dir string, cmds []*Command) {
	for _, c := range cmds {
		c.Cmd.Dir = dir
	}
	return
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
