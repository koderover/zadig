package service

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf16"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/setting"
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
	server, err := commonservice.GetSystemServerURL()
	if err != nil {
		return nil, err
	}
	if err := validateAgentInstallURL(server); err != nil {
		return nil, err
	}
	base := strings.TrimRight(server, "/") + "/api/aslan/vm/agents/"
	if platform == "windows" {
		command := windowsAgentInstallCommand(base+"install.ps1", vm.Agent.Token, vm.Agent.Workspace, upgrade)
		return map[string]string{"powershell": command, "cmd": command}, nil
	}
	return map[string]string{"shell": unixAgentInstallCommand(base+"install.sh", vm.Agent.Token, vm.Agent.Workspace, upgrade)}, nil
}

func unixAgentInstallCommand(scriptURL, token, workspace string, upgrade bool) string {
	args := " --token " + shellQuote(token)
	if workspace != "" {
		args += " --work-dir " + shellQuote(workspace)
	}
	if upgrade {
		args += " --upgrade"
	}
	protocols := "=https"
	if strings.HasPrefix(scriptURL, "http://") {
		protocols = "=http,https"
	}
	return "curl -fsSL --proto-redir " + shellQuote(protocols) + " " + shellQuote(scriptURL) +
		" -o zadig-agent-install.sh && sh zadig-agent-install.sh" + args
}

func windowsAgentInstallCommand(scriptURL, token, workspace string, upgrade bool) string {
	args := " -Token " + powershellQuote(token)
	if workspace != "" {
		args += " -WorkDir " + powershellQuote(workspace)
	}
	if upgrade {
		args += " -Upgrade"
	}
	script := "& ([scriptblock]::Create((Invoke-WebRequest -UseBasicParsing -Uri " + powershellQuote(scriptURL) +
		" -ErrorAction Stop).Content))" + args
	// Both CMD and a parent PowerShell parse this launcher before the child executes it.
	if strings.ContainsAny(scriptURL+token+workspace, "%!$`\"\r\n\x00") {
		var encoded []byte
		for _, unit := range utf16.Encode([]rune(script)) {
			encoded = append(encoded, byte(unit), byte(unit>>8))
		}
		return "powershell.exe -NoProfile -EncodedCommand " + base64.StdEncoding.EncodeToString(encoded)
	}
	return `powershell.exe -NoProfile -Command "` + script + `"`
}
