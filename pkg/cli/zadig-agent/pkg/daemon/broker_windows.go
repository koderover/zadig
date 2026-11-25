package daemon

import (
	"context"
	"fmt"
)

type BrokerStatus struct{ ID, Status, Executable, Home, User string }

func UsesUpgradeBroker() bool        { return false }
func brokerAvailable(*Settings) bool { return false }
func BrokerOperation(context.Context, string, string) (*BrokerStatus, error) {
	return nil, fmt.Errorf("privileged upgrade helper is not available on Windows")
}
func PrepareBrokerUpgrade(context.Context, string, string, string, string) error {
	return fmt.Errorf("privileged upgrade helper is not available on Windows")
}
func installUpgradeBroker(_ *Settings, key string) error {
	if key != "" {
		return fmt.Errorf("privileged upgrade helper is not available on Windows")
	}
	return nil
}
func RunUpgradeBroker(context.Context) error {
	return fmt.Errorf("privileged upgrade helper is not available on Windows")
}

func uninstallUpgradeBroker(context.Context, *Settings) error { return nil }
