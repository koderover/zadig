package util

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"k8s.io/helm/pkg/tlsutil"
	"k8s.io/helm/pkg/urlutil"
)

func IsValidIPv4(address string) bool {
	ip := net.ParseIP(address)
	return ip != nil && ip.To4() != nil
}

func NewTransport(addr, certFile, keyFile, caFile string, insecureSkipTLSverify bool, proxyURL string) (*http.Transport, error) {
	transport := &http.Transport{
		DisableCompression: true,
	}

	if (certFile != "" && keyFile != "") || caFile != "" {
		tlsConf, err := tlsutil.NewClientTLS(certFile, keyFile, caFile)
		if err != nil {
			return nil, errors.Wrap(err, "can't create TLS config for client")
		}
		tlsConf.BuildNameToCertificate()

		sni, err := urlutil.ExtractHostname(addr)
		if err != nil {
			return nil, err
		}
		tlsConf.ServerName = sni

		transport.TLSClientConfig = tlsConf
	}

	if insecureSkipTLSverify {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		} else {
			transport.TLSClientConfig.InsecureSkipVerify = true
		}
	}

	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy url %s, err: %s", proxyURL, err)
		}
		transport.Proxy = http.ProxyURL(proxy)
	}

	return transport, nil
}
