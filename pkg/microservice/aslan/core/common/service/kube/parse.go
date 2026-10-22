/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kube

import (
	"fmt"
	"regexp"
	"strings"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
)

const (
	NameSpaceRegexString   = "[^a-z0-9.-]"
	DefaultNameRegexString = "^[a-zA-Z0-9-_]{1,50}$"

	// Aslan 系统提供渲染键值
	namespaceRegexString = `\$Namespace\$`
	envRegexString       = `\$EnvName\$`
	productRegexString   = `\$Product\$`
	serviceRegexString   = `\$Service\$`
	clusterRegexString   = `\$ClusterName\$`
)

var (
	NameSpaceRegex   = regexp.MustCompile(NameSpaceRegexString)
	DefaultNameRegex = regexp.MustCompile(DefaultNameRegexString)

	// Aslan 系统提供渲染键值
	namespaceRegex = regexp.MustCompile(namespaceRegexString)
	productRegex   = regexp.MustCompile(productRegexString)
	envNameRegex   = regexp.MustCompile(envRegexString)
	serviceRegex   = regexp.MustCompile(serviceRegexString)
	clusterRegex   = regexp.MustCompile(clusterRegexString)
)

// ParseSysKeys 渲染系统变量键值
func ParseSysKeys(namespace, envName, productName, serviceName, clusterName, ori string) string {
	ori = envNameRegex.ReplaceAllLiteralString(ori, strings.ToLower(envName))
	ori = namespaceRegex.ReplaceAllLiteralString(ori, strings.ToLower(namespace))
	ori = productRegex.ReplaceAllLiteralString(ori, strings.ToLower(productName))
	ori = serviceRegex.ReplaceAllLiteralString(ori, strings.ToLower(serviceName))
	ori = clusterRegex.ReplaceAllLiteralString(ori, strings.ToLower(clusterName))
	return ori
}

// ParseModuleImageKeys substitutes complete and split image variables from the
// final container images. Replacement values are prepared before scanning YAML;
// split errors are reported only when the corresponding variable is referenced.
// Unavailable images may remain unresolved when allowUnresolved is true.
// The returned YAML includes all successful replacements, even on error.
// Variables without replacement values remain unchanged.
func ParseModuleImageKeys(yaml string, containers []*commonmodels.Container, allowUnresolved bool) (string, error) {
	replacements := make(map[string]string)
	replacementErrors := make(map[string]error)
	for _, c := range containers {
		if c == nil || c.Name == "" || c.Image == "" {
			continue
		}
		if commonutil.IsModuleImagePlaceholder(c.Image) {
			continue
		}
		// Workflow task previews may carry a non-image sentinel.
		trimmedImage := strings.TrimSpace(c.Image)
		if strings.HasPrefix(trimmedImage, "{{") && strings.HasSuffix(trimmedImage, "}}") {
			continue
		}
		key := fmt.Sprintf("$%s-image$", c.Name)
		if _, exists := replacements[key]; exists {
			continue
		}
		replacements[key] = c.Image

		parts, err := commonutil.ParseModuleImageParts(c.Image)
		if err != nil {
			for _, variable := range commonutil.ModuleImagePartVariables {
				key := fmt.Sprintf("$%s-image-%s$", c.Name, variable.Part)
				replacementErrors[key] = fmt.Errorf("module %q variable %s: %w", c.Name, key, err)
			}
			continue
		}
		for _, variable := range commonutil.ModuleImagePartVariables {
			key := fmt.Sprintf("$%s-image-%s$", c.Name, variable.Part)
			value, ok := parts.PartValue(variable.Part)
			if !ok {
				replacementErrors[key] = fmt.Errorf("module %q variable %s: unsupported image part %q", c.Name, key, variable.Part)
				continue
			}
			replacements[key] = value
		}
	}

	yaml = commonutil.ModuleImageRegex.ReplaceAllStringFunc(yaml, func(key string) string {
		if value, exists := replacements[key]; exists {
			return value
		}
		return key
	})

	for _, key := range commonutil.ModuleImageRegex.FindAllString(yaml, -1) {
		if err := replacementErrors[key]; err != nil {
			return yaml, err
		}
	}

	if allowUnresolved {
		return yaml, nil
	}

	matches := commonutil.ModuleImageRegex.FindAllStringSubmatch(yaml, -1)
	if len(matches) == 0 {
		return yaml, nil
	}
	seen := make(map[string]struct{}, len(matches))
	unresolved := make([]string, 0, len(matches))
	for _, m := range matches {
		if _, ok := seen[m[0]]; ok {
			continue
		}
		seen[m[0]] = struct{}{}
		unresolved = append(unresolved, m[0])
	}
	return yaml, fmt.Errorf("unresolved module image placeholder(s): %s", strings.Join(unresolved, ", "))
}
