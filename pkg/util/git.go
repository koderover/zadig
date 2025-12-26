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

package util

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

// git@github.com or git@github.com:2000
// return github.com
func GetSSHUserAndHostAndPort(address string) (string, string, int) {
	address = strings.TrimPrefix(address, "ssh://")
	addressArr := strings.Split(address, "@")
	user := ""
	if len(addressArr) == 2 {
		user = addressArr[0]
		address = addressArr[1]
	}
	hostArr := strings.Split(address, ":")
	if len(hostArr) == 2 {
		portStr := strings.Split(hostArr[1], "/")[0]
		port, err := strconv.Atoi(portStr)
		if err != nil {
			log.Errorf("GetSSHHostAndPort: failed to convert port to int, address: %s, err: %s", address, err)
			return user, hostArr[0], 0
		}
		return user, hostArr[0], port
	}
	return user, hostArr[0], 0
}

func GetSSHRemoteAddress(address, namespace, projectName string) string {
	address = strings.TrimPrefix(address, "ssh://")
	addressArr := strings.Split(address, "@")
	user := ""
	if len(addressArr) == 2 {
		// has user
		user = addressArr[0]
		address = addressArr[1]
	}

	subpath := ""
	addressArr2 := strings.Split(address, "/")
	if len(addressArr2) > 1 {
		// has subpath
		// for example, ssh://git@gitee.com:2222/kr-test-org1
		subpath = strings.Join(addressArr2[1:], "/")
	}

	// now address is like gitee.com:2222
	hostArr := strings.Split(addressArr2[0], ":")
	host := hostArr[0]

	path := fmt.Sprintf("%s/%s.git", namespace, projectName)
	if subpath != "" {
		path = fmt.Sprintf("%s/%s/%s.git", subpath, namespace, projectName)
	}

	remote := fmt.Sprintf("git@%s:%s", host, path)
	if user != "" {
		remote = fmt.Sprintf("%s@%s:%s", user, host, path)
	}

	return remote
}

func WriteSSHFile(sshKey, hostName string) (string, error) {
	if sshKey == "" {
		return "", fmt.Errorf("ssh cannot be empty")
	}

	if hostName == "" {
		return "", fmt.Errorf("hostName cannot be empty")
	}

	hostName = strings.Replace(hostName, ".", "", -1)
	hostName = strings.Replace(hostName, ":", "", -1)
	pathName := fmt.Sprintf("/.ssh/id_rsa.%s", hostName)
	file := filepath.Join(config.Home(), pathName)

	// mkdir if not exists
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return "", fmt.Errorf("failed to mkdir .ssh, err: %s", err)
	}

	return file, ioutil.WriteFile(file, []byte(sshKey), 0400)
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

func HasOriginRemote(repoDir string) (bool, error) {
	f, err := os.Open(filepath.Join(repoDir, ".git/config"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to open .git/config, err: %s", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == `[remote "origin"]` {
			return true, nil
		}
	}

	return false, scanner.Err()
}
