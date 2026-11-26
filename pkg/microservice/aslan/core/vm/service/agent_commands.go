package service

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf16"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/agentupgrade"
	"github.com/spf13/viper"
)

func agentResourceURL() (string, error) {
	base := viper.GetString(setting.ZadigAgentRepoURL)
	if base == "" {
		server, err := commonservice.GetSystemServerURL()
		if err != nil {
			return "", err
		}
		base = strings.TrimRight(server, "/") + "/agent-resources"
	}
	if err := validateAgentInstallURL(base); err != nil {
		return "", fmt.Errorf("invalid agent resource URL: %w", err)
	}
	return strings.TrimRight(base, "/"), nil
}

func validateAgentInstallURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("agent installation requires an HTTP(S) URL without credentials, query or fragment")
	}
	return nil
}

func shellQuote(s string) string      { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
func powershellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func agentInstallCommands(vm *commonmodels.PrivateKey, platform, arch string, upgrade bool) (map[string]string, error) {
	if (platform != "linux" && platform != "darwin" && platform != "windows") ||
		(arch != "amd64" && arch != "arm64") || (platform == "windows" && arch != "amd64") {
		return nil, fmt.Errorf("unsupported platform %s/%s", platform, arch)
	}
	if vm.Agent == nil || vm.Agent.Token == "" {
		return nil, fmt.Errorf("agent registration token is missing")
	}
	base, err := agentResourceURL()
	if err != nil {
		return nil, err
	}
	version, err := config.GetZadigAgentVersion()
	if err != nil {
		return nil, err
	}
	server, err := commonservice.GetSystemServerURL()
	if err != nil {
		return nil, err
	}
	if err := validateAgentInstallURL(server); err != nil {
		return nil, err
	}
	downloadURL := base + "/" + agentupgrade.BinaryName(platform, arch, version)
	if platform == "windows" {
		command := windowsAgentInstallCommand(base+"/install.ps1", downloadURL, server, vm.Agent.Token, vm.Agent.Workspace, upgrade)
		var encoded []byte
		for _, unit := range utf16.Encode([]rune(command)) {
			encoded = append(encoded, byte(unit), byte(unit>>8))
		}
		return map[string]string{
			"powershell": command,
			"cmd":        "powershell.exe -NoProfile -EncodedCommand " + base64.StdEncoding.EncodeToString(encoded),
		}, nil
	}
	return map[string]string{"shell": unixAgentInstallCommand(base+"/install.sh", downloadURL, server, vm.Agent.Token, vm.Agent.Workspace, upgrade)}, nil
}

func unixAgentInstallCommand(scriptURL, downloadURL, server, token, workspace string, upgrade bool) string {
	if !upgrade {
		start := "sudo -H /usr/local/bin/zadig-agent install --server-url " + shellQuote(server) + " --token " + shellQuote(token)
		if workspace != "" {
			start += " --work-dir " + shellQuote(workspace)
		}
		return strings.Join([]string{
			"curl -fL " + shellQuote(downloadURL) + " -o zadig-agent.download",
			"sudo install -m 755 zadig-agent.download /usr/local/bin/zadig-agent",
			start,
		}, "\n")
	}
	args := []string{"--server-url " + shellQuote(server), "--token " + shellQuote(token), "--download-url " + shellQuote(downloadURL)}
	if workspace != "" {
		args = append(args, "--work-dir "+shellQuote(workspace))
	}
	args = append(args, "--upgrade")
	protocols := "=https"
	if strings.HasPrefix(scriptURL, "http://") {
		protocols = "=http,https"
	}
	// The frontend joins command lines; set -e also makes direct multiline execution safe.
	return strings.Join([]string{
		`(set -e`,
		`f=$(mktemp)`,
		`trap 'rm -f "$f"' EXIT`,
		"curl --proto " + shellQuote(protocols) + " --proto-redir " + shellQuote(protocols) +
			" -fsSL " + shellQuote(scriptURL) + ` -o "$f"`,
		`sh "$f" ` + strings.Join(args, " ") + ")",
	}, "\n")
}

func windowsAgentInstallCommand(scriptURL, downloadURL, server, token, workspace string, upgrade bool) string {
	if !upgrade {
		start := "& $agent install --service-user LocalSystem --auto-start=true --server-url " + powershellQuote(server) + " --token " + powershellQuote(token)
		if workspace != "" {
			start += " --work-dir " + powershellQuote(workspace)
		}
		return strings.Join([]string{
			`$agent = Join-Path $HOME 'zadig-agent.exe'`,
			"Invoke-WebRequest -UseBasicParsing -Uri " + powershellQuote(downloadURL) + " -OutFile $agent -ErrorAction Stop",
			start,
		}, "\n")
	}
	args := " -ServerUrl " + powershellQuote(server) + " -Token " + powershellQuote(token) + " -DownloadUrl " + powershellQuote(downloadURL)
	if workspace != "" {
		args += " -WorkDir " + powershellQuote(workspace)
	}
	args += " -Upgrade"
	return "& { $ErrorActionPreference = 'Stop'; [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; " +
		"& ([scriptblock]::Create((Invoke-WebRequest -UseBasicParsing -Uri " + powershellQuote(scriptURL) +
		" -ErrorAction Stop).Content))" + args + " }"
}
