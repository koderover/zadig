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
	url := base + "/" + agentupgrade.BinaryName(platform, arch, version)
	if platform == "windows" {
		command := windowsAgentInstallCommand(url, server, vm.Agent.Token, vm.Agent.Workspace, upgrade)
		var encoded []byte
		for _, unit := range utf16.Encode([]rune(command)) {
			encoded = append(encoded, byte(unit), byte(unit>>8))
		}
		return map[string]string{
			"powershell": command,
			"cmd":        "powershell.exe -NoProfile -EncodedCommand " + base64.StdEncoding.EncodeToString(encoded),
		}, nil
	}
	return map[string]string{"shell": unixAgentInstallCommand(url, server, vm.Agent.Token, vm.Agent.Workspace, upgrade)}, nil
}

func unixAgentInstallCommand(url, server, token, workspace string, upgrade bool) string {
	protocols := "=https"
	if strings.HasPrefix(url, "http://") {
		protocols = "=http,https"
	}
	lines := []string{
		`(agent_file=$(mktemp) && agent_stage= &&`,
		`trap 'rm -f "$agent_file"; if [ -n "${agent_stage:-}" ]; then "$@" rm -f "$agent_stage"; fi' EXIT &&`,
		`if [ "$(id -u)" -eq 0 ]; then set --; else set -- sudo; fi &&`,
		"curl --proto " + shellQuote(protocols) + " --proto-redir " + shellQuote(protocols) + " -fsSL " + shellQuote(url) + ` -o "$agent_file" &&`,
	}
	if upgrade {
		lines = append(lines,
			`if [ -e /usr/local/bin/zadig-agent ] || [ -L /usr/local/bin/zadig-agent ]; then`,
			`  [ -f /usr/local/bin/zadig-agent ] && [ ! -L /usr/local/bin/zadig-agent ] || { echo 'Refusing to replace a non-regular agent file' >&2; exit 1; }`,
			`  "$@" /usr/local/bin/zadig-agent stop && sleep 6 &&`,
			`  chmod 700 "$agent_file" && "$@" "$agent_file" wait-for-stop &&`,
			`  "$@" cp -p /usr/local/bin/zadig-agent "/usr/local/bin/zadig-agent.previous-$(date +%s)-$$" || exit 1`,
			`fi &&`,
		)
	} else {
		lines = append(lines, `if [ -e /usr/local/bin/zadig-agent ] || [ -L /usr/local/bin/zadig-agent ]; then echo 'Agent already exists; use the recovery or upgrade command' >&2; exit 1; fi &&`)
	}
	lines = append(lines,
		`"$@" mkdir -p /usr/local/bin &&`,
		`agent_stage=$("$@" mktemp /usr/local/bin/.zadig-agent-install-XXXXXX) &&`,
		`"$@" install -m 755 "$agent_file" "$agent_stage" && "$@" mv -f "$agent_stage" /usr/local/bin/zadig-agent &&`,
	)
	start := `"$@" /usr/local/bin/zadig-agent install --server-url ` + shellQuote(server) + " --token " + shellQuote(token)
	if workspace != "" {
		start += " --work-dir " + shellQuote(workspace)
	}
	lines = append(lines, start+")")
	return strings.Join(lines, "\n")
}

func windowsAgentInstallCommand(url, server, token, workspace string, upgrade bool) string {
	lines := []string{
		`& {`,
		`$ErrorActionPreference = 'Stop'`,
		`$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())`,
		`if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Run PowerShell as administrator' }`,
		`[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12`,
		`$agent = Join-Path $HOME 'zadig-agent.exe'`,
		`$download = "$agent.download-$([guid]::NewGuid().ToString('N')).exe"`,
	}
	if !upgrade {
		lines = append(lines, `if (Test-Path -LiteralPath $agent) { throw 'Agent already exists; use the recovery or upgrade command' }`)
	}
	lines = append(lines,
		`$service = Get-Service -Name zadig-agent -ErrorAction SilentlyContinue`,
		`$credential = $null`,
		`if (-not $service) {`,
		`  $credential = Get-Credential -UserName ([Security.Principal.WindowsIdentity]::GetCurrent().Name) -Message 'Windows Service account credentials'`,
		`  if (-not $credential) { throw 'Service account credentials are required' }`,
		`}`,
		`try {`,
		"  Invoke-WebRequest -UseBasicParsing -Uri "+powershellQuote(url)+" -OutFile $download",
	)
	if upgrade {
		lines = append(lines,
			`  if (Test-Path -LiteralPath $agent) {`,
			`    if ((Get-Item -LiteralPath $agent).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Refusing to replace a linked agent file' }`,
			`    & $agent stop`,
			`    if ($LASTEXITCODE -ne 0) { throw 'Could not stop the existing agent' }`,
			`    if ($service -and $service.Status -ne 'Stopped') { Stop-Service -Name zadig-agent; $service.WaitForStatus('Stopped', [TimeSpan]::FromSeconds(60)) }`,
			`    Start-Sleep -Seconds 6`,
			`    & $download wait-for-stop`,
			`    if ($LASTEXITCODE -ne 0) { throw 'Existing agent has not stopped' }`,
			`    $backup = "$agent.previous-$([guid]::NewGuid().ToString('N'))"`,
			`    Move-Item -LiteralPath $agent -Destination $backup`,
			`    Write-Host "Previous executable: $backup"`,
			`  }`,
		)
	}
	lines = append(lines,
		`  Move-Item -LiteralPath $download -Destination $agent`,
		"  $agentArgs = @('install', '--server-url', "+powershellQuote(server)+", '--token', "+powershellQuote(token)+")",
	)
	if workspace != "" {
		lines = append(lines, "  $agentArgs += @('--work-dir', "+powershellQuote(workspace)+")")
	}
	lines = append(lines,
		`  if ($credential) {`,
		`    $previousEncoding = $OutputEncoding`,
		`    try {`,
		`      $OutputEncoding = [System.Text.UTF8Encoding]::new($false)`,
		`      $credential.GetNetworkCredential().Password | & $agent @agentArgs --service-user $credential.UserName --password-stdin`,
		`    } finally { $credential = $null; $OutputEncoding = $previousEncoding }`,
		`  } else { & $agent @agentArgs }`,
		`  if ($LASTEXITCODE -ne 0) { throw 'Agent installation/start failed' }`,
		`} finally { Remove-Item -LiteralPath $download -Force -ErrorAction SilentlyContinue }`,
		`}`,
	)
	return strings.Join(lines, "\n")
}
