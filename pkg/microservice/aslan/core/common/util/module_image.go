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

package util

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/distribution/reference"
)

// ModuleImagePartVariables defines the supported image parts and their display order.
var ModuleImagePartVariables = []struct {
	Part        string
	Description string
}{
	{"registry", "镜像仓库地址，包含端口"},
	{"namespace", "镜像多级路径"},
	{"name", "镜像名"},
	{"tag", "镜像标签"},
}

// ModuleImageRegex captures the module name and optional image part.
var ModuleImageRegex = regexp.MustCompile(`\$([A-Za-z0-9_-]+)-image(?:-(registry|namespace|name|tag))?\$`)

// IsModuleImagePlaceholder also recognizes unresolved variables inside a composed image.
func IsModuleImagePlaceholder(image string) bool {
	return ModuleImageRegex.MatchString(image)
}

// ModuleImageParts is derived from a complete image at render time and is not persisted.
type ModuleImageParts struct {
	Registry  string
	Namespace string
	Name      string
	Tag       string
}

// ParseModuleImageParts preserves explicitly supplied registry and path components.
func ParseModuleImageParts(image string) (*ModuleImageParts, error) {
	// Parse, unlike ParseNormalizedNamed, does not add docker.io or library.
	ref, err := reference.Parse(image)
	if err != nil {
		return nil, fmt.Errorf("invalid image %q: %w", image, err)
	}
	named, ok := ref.(reference.Named)
	if !ok {
		return nil, fmt.Errorf("image %q has no name", image)
	}
	parts := &ModuleImageParts{}
	if tagged, ok := ref.(reference.Tagged); ok {
		parts.Tag = tagged.Tag()
	}

	path := named.Name()
	if first, remainder, ok := strings.Cut(path, "/"); ok &&
		(first == "localhost" || strings.ContainsAny(first, ".:") || first != strings.ToLower(first)) {
		parts.Registry = first
		path = remainder
	}
	if index := strings.LastIndex(path, "/"); index >= 0 {
		parts.Namespace = path[:index]
		parts.Name = path[index+1:]
	} else {
		parts.Name = path
	}
	return parts, nil
}
