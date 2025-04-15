package job

import commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"

const (
	stepNameInstallDeps = "tools"
	stepNameGit         = "git"
	stepNamePerforce    = "perforce"
	stepNameShell       = "shell"
	stepNameBatchFile   = "batchfile"
	stepNamePowershell  = "powershell"
)

type keyValMap struct {
	keyValMap map[string]*commonmodels.KeyVal
}

func newKeyValMap() *keyValMap {
	return &keyValMap{}
}

func (m *keyValMap) Insert(keyVals ...*commonmodels.KeyVal) {
	if m.keyValMap == nil {
		m.keyValMap = make(map[string]*commonmodels.KeyVal)
	}
	for _, keyVal := range keyVals {
		if _, ok := m.keyValMap[keyVal.Key]; ok {
			continue
		}
		m.keyValMap[keyVal.Key] = keyVal
	}
}

func (m *keyValMap) List() []*commonmodels.KeyVal {
	ret := make([]*commonmodels.KeyVal, 0)
	for _, kv := range m.keyValMap {
		ret = append(ret, kv)
	}
	return ret
}
