/*
Copyright 2025 The KodeRover Authors.

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
	"bufio"
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/client"
	gitcmd "github.com/koderover/zadig/v2/pkg/microservice/jobexecutor/core/service/cmd"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

type Config struct {
	ID                 int            `json:"id"`
	Type               string         `json:"type"`
	Address            string         `json:"address"`
	AuthType           types.AuthType `json:"auth_type"`
	PrivateAccessToken string         `json:"private_access_token"`
	SSHKey             string         `json:"ssh_key"`
}

type Client struct {
	Config     Config
	SshKeyPath string
}

func (c *Config) Open(id int, logger *zap.SugaredLogger) (client.CodeHostClient, error) {
	sshKeyPath := ""
	if c.AuthType == types.SSHAuthType {
		_, hostName, _ := util.GetSSHUserAndHostAndPort(c.Address)
		hostName = strings.Replace(hostName, ".", "", -1)
		hostName = strings.Replace(hostName, ":", "", -1)
		pathName := fmt.Sprintf("/.ssh/id_rsa.%s", hostName)
		sshKeyPath = path.Join(config.Home(), pathName)
	}
	return &Client{
		Config:     *c,
		SshKeyPath: sshKeyPath,
	}, nil
}

func (c *Client) ListBranches(opt client.ListOpt) ([]*client.Branch, error) {
	branches, err := c.listRemoteBranches(opt.Namespace, opt.ProjectName, opt.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote branches, err: %s", err)
	}

	var res []*client.Branch
	for _, branch := range branches {
		res = append(res, &client.Branch{
			Name: branch,
		})
	}
	return res, nil
}

func (c *Client) ListTags(opt client.ListOpt) ([]*client.Tag, error) {
	tags, err := c.listRemoteTags(opt.Namespace, opt.ProjectName, opt.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote tags, err: %s", err)
	}

	var res []*client.Tag
	for _, tag := range tags {
		res = append(res, &client.Tag{
			Name: tag,
		})
	}
	return res, nil
}

func (c *Client) ListPrs(opt client.ListOpt) ([]*client.PullRequest, error) {
	return nil, fmt.Errorf("not support list prs")
}

func (c *Client) ListNamespaces(keyword string) ([]*client.Namespace, error) {
	return nil, fmt.Errorf("not support list namespaces")
}

func (c *Client) ListProjects(opt client.ListOpt) ([]*client.Project, error) {
	return nil, fmt.Errorf("not support list projects")
}

func (c *Client) ListCommits(opt client.ListOpt) ([]*client.Commit, error) {
	return nil, fmt.Errorf("not support list commits")
}

func (c *Client) initRepo(repoDir, namespace, projectName string) error {
	// lock := cache.NewRedisLock(fmt.Sprintf("init_repo:%d:%s:%s:%s", c.Config.ID, c.Config.Address, c.Config.Namespace, c.Config.RepoName))
	// if err := lock.Lock(); err != nil {
	// 	return fmt.Errorf("failed lock init repo %s, err: %s", c.Config.Address, err)
	// }
	// defer lock.Unlock()

	cmds := make([]*gitcmd.Command, 0)
	cmds = append(cmds, &gitcmd.Command{Cmd: gitcmd.InitGit(repoDir)})

	hasOriginRemote, err := util.HasOriginRemote(repoDir)
	if err != nil {
		return fmt.Errorf("failed to check if origin remote exists, err: %s", err)
	}

	if c.Config.AuthType == types.SSHAuthType {
		if _, err := util.WriteSSHFile(c.Config.SSHKey, c.Config.Address); err != nil {
			return fmt.Errorf("failed to write ssh file, err: %s", err)
		}

		remote := fmt.Sprintf("%s/%s/%s.git", c.Config.Address, namespace, projectName)

		if !hasOriginRemote {
			cmds = append(cmds, &gitcmd.Command{Cmd: gitcmd.GitRemoteAdd("origin", remote)})
		} else {
			cmds = append(cmds, &gitcmd.Command{Cmd: gitcmd.GitRemoteSetUrl("origin", remote)})
		}
	} else {
		u, err := url.Parse(c.Config.Address)
		if err != nil {
			log.Errorf("failed to parse url,err:%s", err)
		} else {
			host := strings.TrimSuffix(strings.Join([]string{u.Host, u.Path}, "/"), "/")
			if !hasOriginRemote {
				cmds = append(cmds, &gitcmd.Command{Cmd: gitcmd.GitRemoteAdd("origin", util.OAuthCloneURL(c.Config.Type, c.Config.PrivateAccessToken, host, namespace, projectName, u.Scheme)), DisableTrace: true})
			} else {
				cmds = append(cmds, &gitcmd.Command{Cmd: gitcmd.GitRemoteSetUrl("origin", util.OAuthCloneURL(c.Config.Type, c.Config.PrivateAccessToken, host, namespace, projectName, u.Scheme)), DisableTrace: true})
			}
		}
	}

	for _, c := range cmds {
		cmdErrReader, err := c.Cmd.StderrPipe()
		if err != nil {
			return err
		}

		errScanner := bufio.NewScanner(cmdErrReader)
		go func() {
			for errScanner.Scan() {
				log.Errorf("%s", errScanner.Text())
			}
		}()

		if !c.DisableTrace {
			log.Debugf("%s", strings.Join(c.Cmd.Args, " "))
		}

		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to run command %s, err: %s", strings.Join(c.Cmd.Args, " "), err)
		}
	}

	return nil
}

func (c *Client) listRemoteBranches(namespace, projectName, keyword string) ([]string, error) {
	var cmd *exec.Cmd
	if c.Config.AuthType == types.SSHAuthType {
		_, _, port := util.GetSSHUserAndHostAndPort(c.Config.Address)
		remote := util.GetSSHRemoteAddress(c.Config.Address, namespace, projectName)

		log.Debugf("port: %d, remote: %s", port, remote)

		script := `
        ssh-agent sh -c '
            echo "$SSH_PRIVATE_KEY" | ssh-add -;
            GIT_SSH_COMMAND="ssh -o IdentityFile=none -o StrictHostKeyChecking=no" \
                git ls-remote --heads ` + remote + `
        '
		`
		if port != 0 {
			script = fmt.Sprintf(`
        ssh-agent sh -c '
            echo "$SSH_PRIVATE_KEY" | ssh-add -;
            GIT_SSH_COMMAND="ssh -p %d -o IdentityFile=none -o StrictHostKeyChecking=no" \
                git ls-remote --heads %s
        '
		`, port, remote)
		}

		cmd = exec.Command("sh", "-c", script)
		cmd.Env = append(os.Environ(), "SSH_PRIVATE_KEY="+c.Config.SSHKey)
	} else if c.Config.AuthType == types.PrivateAccessTokenAuthType {
		u, err := url.Parse(c.Config.Address)
		if err != nil {
			err = fmt.Errorf("failed to parse url, err:%s", err)
			return nil, err
		}

		host := strings.TrimSuffix(strings.Join([]string{u.Host, u.Path}, "/"), "/")
		cmd = gitcmd.GitListRemoteBranch(util.OAuthCloneURL(c.Config.Type, c.Config.PrivateAccessToken, host, namespace, projectName, u.Scheme))
	} else {
		return nil, fmt.Errorf("unsupported auth type: %s", c.Config.AuthType)
	}

	var out bytes.Buffer
	cmd.Stdout = &out

	cmdErrReader, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	errScanner := bufio.NewScanner(cmdErrReader)
	go func() {
		for errScanner.Scan() {
			log.Errorf("%s", errScanner.Text())
		}
	}()

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var branches []string

	for _, line := range lines {
		// 格式：<hash>\trefs/heads/<branch>
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			continue
		}
		ref := parts[1]

		// 只取分支名，例如 refs/heads/main -> main
		if strings.HasPrefix(ref, "refs/heads/") {
			branch := strings.TrimPrefix(ref, "refs/heads/")
			if keyword != "" && !strings.Contains(branch, keyword) {
				continue
			}
			branches = append(branches, branch)
		}
	}

	return branches, nil
}

func (c *Client) listRemoteTags(namespace, projectName, keyword string) ([]string, error) {
	var cmd *exec.Cmd
	if c.Config.AuthType == types.SSHAuthType {
		_, _, port := util.GetSSHUserAndHostAndPort(c.Config.Address)
		remote := util.GetSSHRemoteAddress(c.Config.Address, namespace, projectName)

		script := `
        ssh-agent sh -c '
            echo "$SSH_PRIVATE_KEY" | ssh-add -;
            GIT_SSH_COMMAND="ssh -o IdentityFile=none -o StrictHostKeyChecking=no" \
                git ls-remote --tags --refs ` + remote + `
        '
		`
		if port != 0 {
			script = fmt.Sprintf(`
        ssh-agent sh -c '
            echo "$SSH_PRIVATE_KEY" | ssh-add -;
            GIT_SSH_COMMAND="ssh -p %d -o IdentityFile=none -o StrictHostKeyChecking=no" \
                git ls-remote --tags --refs %s
        '
		`, port, remote)
		}

		cmd = exec.Command("sh", "-c", script)
		cmd.Env = append(os.Environ(), "SSH_PRIVATE_KEY="+c.Config.SSHKey)
	} else if c.Config.AuthType == types.PrivateAccessTokenAuthType {
		u, err := url.Parse(c.Config.Address)
		if err != nil {
			err = fmt.Errorf("failed to parse url, err:%s", err)
			return nil, err
		}

		host := strings.TrimSuffix(strings.Join([]string{u.Host, u.Path}, "/"), "/")
		cmd = gitcmd.GitListRemoteTags(util.OAuthCloneURL(c.Config.Type, c.Config.PrivateAccessToken, host, namespace, projectName, u.Scheme))
	} else {
		return nil, fmt.Errorf("unsupported auth type: %s", c.Config.AuthType)
	}

	var out bytes.Buffer
	cmd.Stdout = &out

	cmdErrReader, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	errScanner := bufio.NewScanner(cmdErrReader)
	go func() {
		for errScanner.Scan() {
			log.Errorf("%s", errScanner.Text())
		}
	}()

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	tags := make([]string, 0)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	for _, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			continue
		}
		ref := parts[1]

		if strings.HasPrefix(ref, "refs/tags/") {
			tag := strings.TrimPrefix(ref, "refs/tags/")

			if keyword != "" && !strings.Contains(tag, keyword) {
				continue
			}

			tags = append(tags, tag)
		}
	}

	return tags, nil
}
