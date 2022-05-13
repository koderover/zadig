package meta

import _ "embed"

//go:embed role.yaml
var roles []byte

func PresetRolesBytes() []byte {
	return roles
}
