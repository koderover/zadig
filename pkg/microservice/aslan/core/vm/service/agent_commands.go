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
	version, err = agentupgrade.Version(version)
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
	if platform == "windows" {
		url := base + "/install-v" + version + ".ps1"
		command := "$installer = Join-Path $env:TEMP ('zadig-agent-' + [guid]::NewGuid().ToString('N') + '.ps1'); " +
			"[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; " +
			"try { Invoke-WebRequest -UseBasicParsing -Uri " + powershellQuote(url) + " -OutFile $installer; & $installer -ServerUrl " + powershellQuote(server) +
			" -ResourceUrl " + powershellQuote(base) + " -Token " + powershellQuote(vm.Agent.Token) + " -Arch " + powershellQuote(arch)
		if vm.Agent.Workspace != "" {
			command += " -WorkDir " + powershellQuote(vm.Agent.Workspace)
		}
		if upgrade {
			command += " -Upgrade"
		}
		command += " } finally { Remove-Item -LiteralPath $installer -Force -ErrorAction SilentlyContinue }"
		// The existing response retains both shells. CMD delegates to PowerShell
		// without bypassing its execution policy or interpreting registration data.
		var encoded []byte
		for _, unit := range utf16.Encode([]rune(command)) {
			encoded = append(encoded, byte(unit), byte(unit>>8))
		}
		cmdCommand := "powershell.exe -NoProfile -EncodedCommand " + base64.StdEncoding.EncodeToString(encoded)
		return map[string]string{"powershell": command, "cmd": cmdCommand}, nil
	}
	url := base + "/install-v" + version + ".sh"
	protocols := "=https"
	if strings.HasPrefix(base, "http://") {
		protocols = "=http,https"
	}
	command := "(installer=$(mktemp) && trap 'rm -f \"$installer\"' EXIT HUP INT TERM && " +
		"curl --proto " + shellQuote(protocols) + " --proto-redir " + shellQuote(protocols) + " -fsSL " + shellQuote(url) + " -o \"$installer\" && " +
		"if [ \"$(id -u)\" -eq 0 ]; then set -- sh; else set -- sudo sh; fi && \"$@\" \"$installer\" --server-url " + shellQuote(server) + " --resource-url " + shellQuote(base) +
		" --token " + shellQuote(vm.Agent.Token) + " --os " + shellQuote(platform) + " --arch " + shellQuote(arch)
	if vm.Agent.Workspace != "" {
		command += " --work-dir " + shellQuote(vm.Agent.Workspace)
	}
	if upgrade {
		command += " --upgrade true"
	}
	command += ")"
	return map[string]string{"shell": command}, nil
}
