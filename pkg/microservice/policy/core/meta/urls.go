package meta

import (
	_ "embed"

	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

//go:embed urls.yaml
var urls []byte
var defaultExemURLs *ExemptionURLs
var configMapExemURLs *ExemptionURLs
var useConfigMapUrls bool

type ExemptionURLs struct {
	Public       []*types.PolicyRule
	SystemAdmin  []*types.PolicyRule
	ProjectAdmin []*types.PolicyRule
}

func init() {
	if err := yaml.Unmarshal(urls, &defaultExemURLs); err != nil {
		log.DPanic(err)
	}
}

func DefaultExemptionsUrls() *ExemptionURLs {
	if !useConfigMapUrls || configMapExemURLs == nil {
		return defaultExemURLs
	}
	return configMapExemURLs
}

func RefreshConfigMapByte(b []byte) error {
	if err := yaml.Unmarshal(b, &configMapExemURLs); err != nil {
		log.Errorf("refresh err:%s", err)
		useConfigMapUrls = false
		return err
	}
	useConfigMapUrls = true
	return nil
}
