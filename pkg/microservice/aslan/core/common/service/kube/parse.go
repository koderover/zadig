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

	// $<module>-image$ — per service-module image substitution.
	// Module names follow the K8s container name spec; we allow the broader
	// [A-Za-z0-9_-]+ to match what parseContainer already accepts.
	moduleImageRegexString = `\$([A-Za-z0-9_-]+)-image\$`
)

var (
	NameSpaceRegex   = regexp.MustCompile(NameSpaceRegexString)
	DefaultNameRegex = regexp.MustCompile(DefaultNameRegexString)

	// Aslan 系统提供渲染键值
	namespaceRegex = regexp.MustCompile(namespaceRegexString)
	productRegex   = regexp.MustCompile(productRegexString)
	envNameRegex   = regexp.MustCompile(envRegexString)
	serviceRegex   = regexp.MustCompile(serviceRegexString)

	moduleImageRegex = regexp.MustCompile(moduleImageRegexString)
)

// ParseSysKeys 渲染系统变量键值
func ParseSysKeys(namespace, envName, productName, serviceName, ori string) string {
	ori = envNameRegex.ReplaceAllLiteralString(ori, strings.ToLower(envName))
	ori = namespaceRegex.ReplaceAllLiteralString(ori, strings.ToLower(namespace))
	ori = productRegex.ReplaceAllLiteralString(ori, strings.ToLower(productName))
	ori = serviceRegex.ReplaceAllLiteralString(ori, strings.ToLower(serviceName))
	return ori
}

// ParseModuleImageKeys substitutes $<module>-image$ placeholders in yaml using
// the provided container list (container.Name -> container.Image). Containers
// with an empty or placeholder-shaped Image are skipped — they cannot resolve
// anything.
//
// If allowUnresolved is false, any $<module>-image$ placeholder still present
// after substitution causes an error naming the offending module(s). Callers
// on the initial-deploy path (no built image yet) should pass true.
func ParseModuleImageKeys(yaml string, containers []*commonmodels.Container, allowUnresolved bool) (string, error) {
	for _, c := range containers {
		if c == nil || c.Name == "" || c.Image == "" {
			continue
		}
		if commonutil.IsModuleImagePlaceholder(c.Image) {
			continue
		}
		// Also skip the Zadig workflow-task-create placeholder convention
		// (e.g. "{{ NOT BE RENDERED }}"). Substituting it into the YAML
		// produces flow-style garbage that downstream YAML decoders choke
		// on. For built-in workload kinds, ReplaceWorkloadImages still
		// writes this placeholder into container.image structurally, so the
		// preview yaml round-trips. For non-built-in kinds (CRDs, etc.),
		// the $<name>-image$ placeholder stays as text — paired with
		// AllowUnresolvedModuleImages on the caller, this is fine.
		trimmedImage := strings.TrimSpace(c.Image)
		if strings.HasPrefix(trimmedImage, "{{") && strings.HasSuffix(trimmedImage, "}}") {
			continue
		}
		pattern := regexp.MustCompile(`\$` + regexp.QuoteMeta(c.Name) + `-image\$`)
		yaml = pattern.ReplaceAllLiteralString(yaml, c.Image)
	}

	if allowUnresolved {
		return yaml, nil
	}

	matches := moduleImageRegex.FindAllStringSubmatch(yaml, -1)
	if len(matches) == 0 {
		return yaml, nil
	}
	seen := make(map[string]struct{}, len(matches))
	unresolved := make([]string, 0, len(matches))
	for _, m := range matches {
		if _, ok := seen[m[1]]; ok {
			continue
		}
		seen[m[1]] = struct{}{}
		unresolved = append(unresolved, m[1])
	}
	return yaml, fmt.Errorf("unresolved $<module>-image$ placeholder(s): %s", strings.Join(unresolved, ", "))
}
