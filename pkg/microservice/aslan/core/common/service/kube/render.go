/*
Copyright 2022 The KodeRover Authors.

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

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
)

func RenderValueForString(origin string, rs *commonmodels.RenderSet) string {
	if rs == nil {
		return origin
	}
	rs.SetKVAlias()
	for _, v := range rs.KVs {
		if v.State == "unused" {
			continue
		}
		origin = replaceAliasValue(origin, v)
	}
	return origin
}

func replaceAliasValue(origin string, v *templatemodels.RenderKV) string {
	for {
		idx := strings.Index(origin, v.Alias)
		if idx < 0 {
			break
		}
		spaces := ""
		start := 0
		for i := idx - 1; i >= 0; i-- {
			if string(origin[i]) != " " {
				break
			}
			start++
		}
		spaces = origin[idx-start : idx]
		valueStr := strings.Replace(v.Value, "\n", fmt.Sprintf("%s%s", "\n", spaces), -1)
		origin = strings.Replace(origin, v.Alias, valueStr, 1)
	}
	return origin
}

func RenderService(prod *commonmodels.Product, render *commonmodels.RenderSet, service *commonmodels.ProductService) (yaml *string, err error) {
	opt := &commonrepo.ServiceFindOption{
		ServiceName: service.ServiceName,
		ProductName: service.ProductName,
		Type:        service.Type,
		Revision:    service.Revision,
	}
	svcTmpl, err := commonrepo.NewServiceColl().Find(opt)
	if err != nil {
		return nil, err
	}

	// 渲染配置集
	parsedYaml := RenderValueForString(svcTmpl.Yaml, render)
	// 渲染系统变量键值
	parsedYaml = ParseSysKeys(prod.Namespace, prod.EnvName, prod.ProductName, service.ServiceName, parsedYaml)
	// 替换服务模板容器镜像为用户指定镜像
	parsedYaml = replaceContainerImages(parsedYaml, svcTmpl.Containers, service.Containers)

	return &parsedYaml, nil
}

func replaceContainerImages(tmpl string, ori []*commonmodels.Container, replace []*commonmodels.Container) string {

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
