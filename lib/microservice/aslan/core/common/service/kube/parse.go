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

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
)

const (
	NameSpaceRegexString   = "[^a-z0-9.-]"
	defaultNameRegexString = "^[a-zA-Z0-9-_]{1,50}$"

	// Aslan 系统提供渲染键值
	namespaceRegexString = `\$Namespace\$`
	envRegexString       = `\$EnvName\$`
	productRegexString   = `\$Product\$`
	serviceRegexString   = `\$Service\$`
)

var (
	NameSpaceRegex   = regexp.MustCompile(NameSpaceRegexString)
	defaultNameRegex = regexp.MustCompile(defaultNameRegexString)

	// Aslan 系统提供渲染键值
	namespaceRegex = regexp.MustCompile(namespaceRegexString)
	productRegex   = regexp.MustCompile(productRegexString)
	envNameRegex   = regexp.MustCompile(envRegexString)
	serviceRegex   = regexp.MustCompile(serviceRegexString)
)

// ParseSysKeys 渲染系统变量键值
func ParseSysKeys(namespace, envName, productName, serviceName, ori string) string {
	ori = envNameRegex.ReplaceAllLiteralString(ori, strings.ToLower(envName))
	ori = namespaceRegex.ReplaceAllLiteralString(ori, strings.ToLower(namespace))
	ori = productRegex.ReplaceAllLiteralString(ori, strings.ToLower(productName))
	ori = serviceRegex.ReplaceAllLiteralString(ori, strings.ToLower(serviceName))
	return ori
}

func ReplaceContainerImages(tmpl string, ori []*commonmodels.Container, replace []*commonmodels.Container) string {
	replaceMap := make(map[string]string)
	for _, container := range replace {
		replaceMap[container.Name] = container.Image
	}

	for _, container := range ori {
		imageRex := regexp.MustCompile("image:\\s*" + container.Image)
		if _, ok := replaceMap[container.Name]; !ok {
			continue
		}
		tmpl = imageRex.ReplaceAllLiteralString(tmpl, fmt.Sprintf("image: %s", replaceMap[container.Name]))
	}

	return tmpl
}
