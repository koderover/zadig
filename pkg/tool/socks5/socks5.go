package socks5

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"

	"golang.org/x/net/proxy"
)

// SSHProxyCommand uses the current executor to relay SSH through SOCKS5.
func SSHProxyCommand(proxyURL string) (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	args := []string{executable, proxyURL}
	for i, arg := range args {
		// OpenSSH expands percent tokens before passing the command to the shell.
		arg = strings.ReplaceAll(arg, "%", "%%")
		args[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return fmt.Sprintf("ProxyCommand %s socks5-proxy %s %%h %%p\n", args[0], args[1]), nil
}

// Run relays the command's stdin/stdout without interpreting SSH's binary data.
func Run(args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("socks5-proxy requires a proxy URL, destination host and port")
	}
	proxyURL, err := url.Parse(args[0])
	if err != nil || (proxyURL.Scheme != "socks5" && proxyURL.Scheme != "socks5h") {
		return fmt.Errorf("invalid SOCKS5 proxy URL")
	}
	dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
	if err != nil {
		return err
	}
	// Dial returns the raw TCP connection, allowing a half-close on stdin EOF.
	conn, err := dialer.Dial("tcp", net.JoinHostPort(args[1], args[2]))
	if err != nil {
		return err
	}
	defer conn.Close()
	closeWriter, ok := conn.(interface{ CloseWrite() error })
	if !ok {
		return fmt.Errorf("SOCKS5 connection does not support half-close")
	}
	written := make(chan error, 1)
	go func() {
		_, err := io.Copy(conn, os.Stdin)
		if err == nil {
			err = closeWriter.CloseWrite()
		}
		if err != nil {
			conn.Close()
		}
		written <- err
	}()
	if _, err := io.Copy(os.Stdout, conn); err != nil {
		return err
	}
	select {
	case err := <-written:
		return err
	default:
		return nil
	}
}
