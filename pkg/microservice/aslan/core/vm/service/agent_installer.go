package service

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
)

//go:embed installer/install.sh
var agentUnixInstaller string

//go:embed installer/install.ps1
var agentWindowsInstaller string

func GetAgentInstallScript(platform string) (string, error) {
	server, err := commonservice.GetSystemServerURL()
	if err != nil {
		return "", err
	}
	if err := validateAgentInstallURL(server); err != nil {
		return "", err
	}
	resources, err := agentResourceURL()
	if err != nil {
		return "", err
	}
	version, err := config.GetZadigAgentVersion()
	if err != nil {
		return "", err
	}
	return renderAgentInstallScript(platform, server, resources, version)
}

func renderAgentInstallScript(platform, server, resources, version string) (string, error) {
	source, quote := agentUnixInstaller, shellQuote
	switch platform {
	case "unix":
	case "windows":
		source, quote = agentWindowsInstaller, powershellQuote
	default:
		return "", fmt.Errorf("unsupported installer platform %s", platform)
	}
	return strings.NewReplacer(
		"'__ZADIG_SERVER_URL__'", quote(server),
		"'__ZADIG_RESOURCE_URL__'", quote(resources),
		"'__ZADIG_AGENT_VERSION__'", quote(version),
	).Replace(source), nil
}
