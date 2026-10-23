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

// The template locates fields; referenced parts must match the source and
// have nonempty values in the target image.
func replaceDeliveryServiceImagesWithTemplate(content, template, namespace string, images []*commonmodels.DeliveryVersionImage) (string, error) {
	templates, err := deliveryImageDocuments(template)
	if err != nil {
		return "", err
	}
	documents, err := deliveryImageDocuments(content)
	if err != nil {
		return "", err
	}
	sources := make([]*commonmodels.Container, 0, len(images))
	for _, image := range images {
		if image != nil {
			sources = append(sources, &commonmodels.Container{Name: image.ContainerName, Image: image.SourceImage})
		}
	}
	templateByResource := make(map[string]*yaml.Node, len(templates))
	for _, node := range templates {
		key, err := deliveryImageResourceKey(node, namespace, sources)
		if err != nil {
			return "", err
		}
		templateByResource[key] = node
	}
	groups := make([]map[string]*deliveryImageFields, 0, len(documents))
	protected := make(map[*yaml.Node]bool)
	for _, document := range documents {
		key, err := deliveryImageResourceKey(document, namespace, nil)
		if err != nil {
			return "", err
		}
		tmpl := templateByResource[key]
		if tmpl == nil {
			continue
		}
		modules := make(map[string]*deliveryImageFields)
		visitDeliveryImageFields(tmpl, document, func(pattern, actual *yaml.Node) bool {
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
				fields.patterns[actual] = pattern.Value
				if actual != nil {
					protected[actual] = true
				}
			}
			return true
		})
		groups = append(groups, modules)
	}
	if len(protected) == 0 {
		service := &commonmodels.DeliveryVersionService{YamlContent: content, Images: images}
		err := updateDeliveryServiceImageInYaml(service, setting.DeliveryVersionSourceFromEnv)
		return service.YamlContent, err
	}
	changed := false
	for _, modules := range groups {
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
					return "", err
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
				return "", fmt.Errorf("module %q: %w", image.ContainerName, err)
			}
			if (fields.parts["registry"] && targetParts.Registry == "") ||
				(fields.parts["namespace"] && targetParts.Namespace == "") ||
				(fields.parts["name"] && targetParts.Name == "") ||
				(fields.parts["tag"] && targetParts.Tag == "") {
				continue
			}
			targets = append(targets, &commonmodels.Container{Name: image.ContainerName, Image: image.TargetImage})
			for node, pattern := range fields.patterns {
				matchedFields[node] = pattern
			}
		}
		// Matching is complete before any field is changed, including fields
		// whose strings reference more than one component.
		replacements := append(targets, sources...)
		for node, pattern := range matchedFields {
			value, err := kube.ParseModuleImageKeys(pattern, replacements, true)
			if err != nil {
				return "", err
			}
			node.Value, node.Tag = value, "!!str"
			changed = true
		}
	}
	// Full-image replacement must not bypass the split-field matching rule.
	for _, document := range documents {
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
	if changed {
		manifests := make([]string, 0, len(documents))
		for _, document := range documents {
			data, err := yaml.Marshal(document)
			if err != nil {
				return "", err
			}
			manifests = append(manifests, string(data))
		}
		content = util.JoinYamls(manifests)
	}
	targets := make([]*commonmodels.Container, 0, len(images))
	for _, image := range images {
		if image != nil {
			targets = append(targets, &commonmodels.Container{Name: image.ContainerName, Image: image.TargetImage})
		}
	}
	result, _, err := kube.ReplaceWorkloadImages(content, targets)
	return result, err
}

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
