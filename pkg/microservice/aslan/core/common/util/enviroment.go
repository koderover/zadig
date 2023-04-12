package util

import (
	"fmt"

	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util/converter"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/util/sets"
)

func IsServiceVarsWildcard(serviceVars []string) bool {
	return len(serviceVars) == 1 && serviceVars[0] == "*"
}

func ClipVariableYamlNoErr(variableYaml string, validKeys []string) string {
	if len(variableYaml) == 0 {
		return variableYaml
	}
	if len(validKeys) == 0 {
		return ""
	}
	clippedYaml, err := ClipVariableYaml(variableYaml, validKeys)
	if err != nil {
		log.Errorf("failed to clip variable yaml, err: %s", err)
		return variableYaml
	}
	return clippedYaml
}

func ClipVariableYaml(variableYaml string, validKeys []string) (string, error) {
	if len(variableYaml) == 0 {
		return "", nil
	}
	valuesMap, err := converter.YamlToFlatMap([]byte(variableYaml))
	if err != nil {
		return "", fmt.Errorf("failed to get flat map for service variable, err: %s", err)
	}

	wildcard := IsServiceVarsWildcard(validKeys)
	if wildcard {
		return variableYaml, nil
	}
	keysSet := sets.NewString(validKeys...)
	validKvMap := make(map[string]interface{})
	for k, v := range valuesMap {
		if keysSet.Has(k) {
			validKvMap[k] = v
		}
	}

	if len(validKvMap) == 0 {
		return "", nil
	}

	validKvMap, err = converter.Expand(validKvMap)
	if err != nil {
		return "", err
	}

	bs, err := yaml.Marshal(validKvMap)
	return string(bs), err
}
