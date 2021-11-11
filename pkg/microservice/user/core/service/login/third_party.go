package login

import (
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/pkg/tool/log"
)

type enabledStatus struct {
	Enabled bool `json:"enabled"`
}

func ThirdPartyLoginEnabled() *enabledStatus {
	connectors, err := systemconfig.New().ListConnectors()
	if err != nil {
		log.Warnf("Failed to list connectors, err: %s", err)
	}
	return &enabledStatus{
		Enabled: len(connectors) > 0,
	}
}
