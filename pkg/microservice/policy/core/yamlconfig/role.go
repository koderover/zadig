package yamlconfig

import _ "embed"

//go:embed role.yaml
var roles []byte
var useConfigMapRoles bool

var configMapRoles []byte

func PresetRolesBytes() []byte {
	if useConfigMapRoles {
		return configMapRoles
	}
	return roles
}

func RefreshRoles(b []byte) {
	useConfigMapRoles = true
	configMapRoles = b
}
