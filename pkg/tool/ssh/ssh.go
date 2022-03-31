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
