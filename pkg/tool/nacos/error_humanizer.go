package nacos

import (
	"fmt"
	"net/url"
	"strings"
)

func humanizeNacosError(operation, serverAddr string, err error) error {
	if err == nil {
		return nil
	}

	raw := strings.ToLower(err.Error())
	addr := displayNacosAddress(serverAddr)

	switch {
	case strings.Contains(raw, "parse nacos server address failed"),
		strings.Contains(raw, "missing protocol scheme"),
		strings.Contains(raw, "invalid uri"):
		return fmt.Errorf("%s失败：Nacos 地址格式不正确，请检查地址配置", operation)
	case strings.Contains(raw, "no such host"):
		return fmt.Errorf("%s失败：无法解析 Nacos 地址 %s，请检查地址是否填写正确", operation, addr)
	case strings.Contains(raw, "certificate signed by unknown authority"):
		return fmt.Errorf("%s失败：HTTPS 证书校验失败，请检查 Nacos 服务证书是否受信任", operation)
	case strings.Contains(raw, "x509:"):
		return fmt.Errorf("%s失败：HTTPS 证书校验失败，请检查 Nacos 服务证书配置是否正确", operation)
	case strings.Contains(raw, "connection refused"):
		return fmt.Errorf("%s失败：连接被拒绝，请检查服务地址、端口或 Nacos 服务状态", operation)
	case strings.Contains(raw, "i/o timeout"),
		strings.Contains(raw, "context deadline exceeded"),
		strings.Contains(raw, "client.timeout exceeded"):
		return fmt.Errorf("%s失败：连接超时，请检查网络连通性或 Nacos 服务状态", operation)
	case containsNacosAuthError(raw):
		return fmt.Errorf("%s失败：用户名或密码错误，或当前账号无权限访问 Nacos", operation)
	default:
		return fmt.Errorf("%s失败：请检查 Nacos 地址、账号密码和服务状态", operation)
	}
}

func containsNacosAuthError(raw string) bool {
	for _, keyword := range []string{
		"401",
		"403",
		"unauthorized",
		"forbidden",
		"unknown user",
		"user not found",
		"invalid password",
		"password error",
		"access denied",
		"permission denied",
		"login failed",
	} {
		if strings.Contains(raw, keyword) {
			return true
		}
	}

	return false
}

func displayNacosAddress(serverAddr string) string {
	parsed, err := url.Parse(serverAddr)
	if err == nil && parsed.Host != "" {
		return parsed.Host
	}

	return serverAddr
}
