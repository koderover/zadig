package login

import "github.com/koderover/zadig/pkg/shared/client/systemconfig"

type enabledStatus struct {
	Enabled bool `json:"enabled"`
}

func ThirdPartyLoginEnabled() *enabledStatus {
	connectors, _ := systemconfig.New().ListConnectors()
	return &enabledStatus{
		Enabled: len(connectors) > 0,
	}
}
