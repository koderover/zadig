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

package ssh

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

func NewSshCli(privateKey []byte, user, host string, port int64) (*ssh.Client, error) {
	var (
		clientConfig *ssh.ClientConfig
		client       *ssh.Client
		err          error
	)

	sign, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	auth := []ssh.AuthMethod{ssh.PublicKeys(sign)}
	clientConfig = &ssh.ClientConfig{
		User:    user,
		Auth:    auth,
		Timeout: 5 * time.Second,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err = ssh.Dial("tcp", addr, clientConfig)
	return client, err
}
