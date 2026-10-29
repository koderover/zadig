/*
Copyright 2026 The KodeRover Authors.

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

package service

import (
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/util"
)

type deliveryImageFields struct {
	patterns map[*yaml.Node]string
	parts    map[string]bool
}

// templateYaml retains image placeholders to locate fields in serviceYaml.
// Referenced fields must match the source and have nonempty target image parts.
func replaceDeliveryServiceImagesWithTemplate(serviceYaml, templateYaml, namespace string, images []*commonmodels.DeliveryVersionImage) (string, error) {
	// 1. 解析模板和服务 YAML，每个文档（以 --- 分隔）对应一个资源节点。
	// templateDocs 保留镜像占位符；serviceDocs 保存待校验和替换的实际值。
	templateDocs, err := deliveryImageDocuments(templateYaml)
	if err != nil {
		return "", err
	}
	serviceDocs, err := deliveryImageDocuments(serviceYaml)
	if err != nil {
		return "", err
	}

	// 2. 按模块整理源镜像和目标镜像，分别用于校验当前值和生成替换值。
	sources := make([]*commonmodels.Container, 0, len(images))
	targets := make([]*commonmodels.Container, 0, len(images))
	for _, image := range images {
		if image != nil {
			sources = append(sources, &commonmodels.Container{Name: image.ContainerName, Image: image.SourceImage})
			targets = append(targets, &commonmodels.Container{Name: image.ContainerName, Image: image.TargetImage})
		}
	}

	// 3. 按 API 版本、资源类型、命名空间和名称建立模板索引，供服务资源匹配。
	templateByResource := make(map[string]*yaml.Node, len(templateDocs))
	for _, node := range templateDocs {
		key, err := deliveryImageResourceKey(node, namespace, sources)
		if err != nil {
			return "", err
		}
		templateByResource[key] = node
	}

	// 4. 逐个匹配服务资源，定位拆分字段；源值匹配且目标分量非空时，才按模块替换。
	// 同时标记拆分字段，防止后续完整镜像替换绕过校验。
	protected := make(map[*yaml.Node]bool)
	changed := false
	for _, document := range serviceDocs {
		key, err := deliveryImageResourceKey(document, namespace, nil)
		if err != nil {
			return "", err
		}
		tmpl := templateByResource[key]
		if tmpl == nil {
			continue
		}
		modules := collectDeliveryImageFields(tmpl, document, protected)
		replaced, err := replaceDeliveryImageFields(modules, images, sources)
		if err != nil {
			return "", err
		}
		changed = changed || replaced
	}

	// 5. 未定位到实际的拆分字段时，沿用原有的完整镜像替换逻辑并返回。
	if len(protected) == 0 {
		service := &commonmodels.DeliveryVersionService{YamlContent: serviceYaml, Images: images}
		err := updateDeliveryServiceImageInYaml(service, setting.DeliveryVersionSourceFromEnv)
		return service.YamlContent, err
	}

	// 6. 替换其他字段中的完整源镜像，跳过已标记的拆分字段。
	for _, document := range serviceDocs {
		visitDeliveryImageFields(document, document, func(_, node *yaml.Node) bool {
			if protected[node] {
				return false
			}
			value := node.Value
			for _, image := range images {
				if image != nil && image.SourceImage != "" && image.TargetImage != "" {
					value = strings.ReplaceAll(value, image.SourceImage, image.TargetImage)
				}
			}
			if value != node.Value {
				node.Value, node.Tag, changed = value, "!!str", true
			}
			return true
		})
	}

	// 7. 字段发生更新时，将资源节点重新序列化并合并为服务 YAML。
	if changed {
		manifests := make([]string, 0, len(serviceDocs))
		for _, document := range serviceDocs {
			data, err := yaml.Marshal(document)
			if err != nil {
				return "", err
			}
			manifests = append(manifests, string(data))
		}
		serviceYaml = util.JoinYamls(manifests)
	}

	// 8. 按容器名称更新工作负载中的镜像，保持原有工作负载替换行为。
	result, _, err := kube.ReplaceWorkloadImages(serviceYaml, targets)
	return result, err
}

// collectDeliveryImageFields 根据模板占位符，按模块收集实际资源中的拆分镜像字段。
// protected 记录这些字段，避免后续完整镜像替换绕过拆分字段的校验。
func collectDeliveryImageFields(template, actual *yaml.Node, protected map[*yaml.Node]bool) map[string]*deliveryImageFields {
	modules := make(map[string]*deliveryImageFields)
	visitDeliveryImageFields(template, actual, func(pattern, node *yaml.Node) bool {
		for _, match := range commonutil.ModuleImageRegex.FindAllStringSubmatch(pattern.Value, -1) {
			if match[2] == "" {
				continue
			}
			fields := modules[match[1]]
			if fields == nil {
				fields = &deliveryImageFields{patterns: make(map[*yaml.Node]string), parts: make(map[string]bool)}
				modules[match[1]] = fields
			}
			fields.parts[match[2]] = true
			fields.patterns[node] = pattern.Value
			if node != nil {
				protected[node] = true
			}
		}
		return true
	})
	return modules
}

// replaceDeliveryImageFields 在单个资源内按模块校验并替换拆分镜像字段。
// 返回值表示是否写入了替换字段；字段不匹配或所需目标分量为空时，跳过该模块。
func replaceDeliveryImageFields(modules map[string]*deliveryImageFields, images []*commonmodels.DeliveryVersionImage, sources []*commonmodels.Container) (bool, error) {
	targets := make([]*commonmodels.Container, 0, len(images))
	matchedFields := make(map[*yaml.Node]string)
	for _, image := range images {
		if image == nil || image.SourceImage == "" || image.TargetImage == "" || image.SourceImage == image.TargetImage {
			continue
		}
		fields := modules[image.ContainerName]
		if fields == nil {
			continue
		}
		matched := true
		for node, pattern := range fields.patterns {
			if node == nil || node.Kind != yaml.ScalarNode {
				matched = false
				break
			}
			expected, err := kube.ParseModuleImageKeys(pattern, sources, true)
			if err != nil {
				return false, err
			}
			if node.Value != expected {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		targetParts, err := commonutil.ParseModuleImageParts(image.TargetImage)
		if err != nil {
			return false, fmt.Errorf("module %q: %w", image.ContainerName, err)
		}
		for part := range fields.parts {
			if value, _ := targetParts.PartValue(part); value == "" {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		targets = append(targets, &commonmodels.Container{Name: image.ContainerName, Image: image.TargetImage})
		for node, pattern := range fields.patterns {
			matchedFields[node] = pattern
		}
	}
	replacements := append(targets, sources...)
	for node, pattern := range matchedFields {
		value, err := kube.ParseModuleImageKeys(pattern, replacements, true)
		if err != nil {
			return false, err
		}
		node.Value, node.Tag = value, "!!str"
	}
	return len(matchedFields) > 0, nil
}

// deliveryImageDocuments 将多文档 YAML 解析为资源节点列表，跳过空文档。
// 每个非空文档必须是映射结构，否则返回错误。
func deliveryImageDocuments(content string) ([]*yaml.Node, error) {
	var documents []*yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(content))
	for {
		node := new(yaml.Node)
		if err := decoder.Decode(node); err == io.EOF {
			return documents, nil
		} else if err != nil {
			return nil, err
		}
		if len(node.Content) == 0 || node.Content[0].Tag == "!!null" {
			continue
		}
		if node.Content[0].Kind != yaml.MappingNode {
			return nil, fmt.Errorf("expected a Kubernetes resource mapping")
		}
		documents = append(documents, node.Content[0])
	}
}

// deliveryImageResourceKey 用 API 版本、类型、命名空间和名称生成匹配键，用于模板与服务资源配对。
// 名称和命名空间中的镜像占位符按源镜像解析；命名空间为空时使用传入的 namespace。
func deliveryImageResourceKey(node *yaml.Node, namespace string, sources []*commonmodels.Container) (string, error) {
	var resource struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Metadata   struct{ Name, Namespace string }
	}
	if err := node.Decode(&resource); err != nil {
		return "", err
	}
	for _, field := range []*string{&resource.Metadata.Name, &resource.Metadata.Namespace} {
		if commonutil.IsModuleImagePlaceholder(*field) {
			resolved, err := kube.ParseModuleImageKeys(*field, sources, true)
			if err != nil {
				return "", err
			}
			*field = resolved
		}
	}
	if resource.Metadata.Namespace == "" {
		resource.Metadata.Namespace = namespace
	}
	return strings.Join([]string{resource.APIVersion, resource.Kind, resource.Metadata.Namespace, resource.Metadata.Name}, "\x00"), nil
}

// visitDeliveryImageFields 对照模板结构遍历实际资源，将对应节点交给回调；缺失节点以 nil 传入。
// 当 template 和 actual 是同一节点时，直接遍历整棵资源树，用于完整镜像替换。
func visitDeliveryImageFields(template, actual *yaml.Node, visit func(*yaml.Node, *yaml.Node) bool) {
	if !visit(template, actual) {
		return
	}
	if template == actual {
		for _, child := range template.Content {
			visitDeliveryImageFields(child, child, visit)
		}
		return
	}
	if template.Kind == yaml.MappingNode {
		actualFields := make(map[string]*yaml.Node)
		if actual != nil && actual.Kind == yaml.MappingNode {
			for i := 0; i < len(actual.Content); i += 2 {
				actualFields[actual.Content[i].Value] = actual.Content[i+1]
			}
		}
		for i := 0; i < len(template.Content); i += 2 {
			visitDeliveryImageFields(template.Content[i+1], actualFields[template.Content[i].Value], visit)
		}
	}
	if template.Kind == yaml.SequenceNode {
		for i, child := range template.Content {
			var actualChild *yaml.Node
			if actual != nil && actual.Kind == yaml.SequenceNode && i < len(actual.Content) {
				actualChild = actual.Content[i]
			}
			visitDeliveryImageFields(child, actualChild, visit)
		}
	}
}
