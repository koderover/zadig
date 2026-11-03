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

package meta

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

// CollectGitTokens gathers repository authentication secrets and their URL-escaped forms for log masking.
func CollectGitTokens(steps []*Step) ([]string, error) {
	var tokens []string
	for _, info := range steps {
		if info.StepType != "git" {
			continue
		}
		yamlBytes, err := yaml.Marshal(info.Spec)
		if err != nil {
			return nil, fmt.Errorf("marshal spec %+v failed", info.Spec)
		}
		var spec *step.StepGitSpec
		if err := yaml.Unmarshal(yamlBytes, &spec); err != nil {
			return nil, fmt.Errorf("unmarshal spec %s to git spec failed", yamlBytes)
		}
		if spec == nil {
			return nil, fmt.Errorf("git spec is empty")
		}
		hasSubmodules := false
		for _, repo := range spec.Repos {
			if repo != nil {
				tokens = append(tokens, repo.OauthToken)
				if repo.Source == types.ProviderGerrit {
					tokens = append(tokens, repo.Password)
					// Gerrit clone URLs contain the HTTP password decoded from the access token.
					if userpass, err := base64.StdEncoding.DecodeString(repo.OauthToken); err == nil {
						if _, password, ok := strings.Cut(string(userpass), ":"); ok {
							tokens = append(tokens, password)
						}
					}
				}
				hasSubmodules = hasSubmodules || repo.SubModules
			}
		}
		if !hasSubmodules {
			continue
		}
		for _, host := range spec.CodeHosts {
			if host != nil {
				tokens = append(tokens, host.AccessToken)
				if host.Type == types.ProviderGerrit {
					tokens = append(tokens, host.Password)
				}
			}
		}
	}
	var values []string
	for _, token := range tokens {
		if token != "" {
			values = append(values, token, strings.TrimPrefix(url.UserPassword("", token).String(), ":"))
		}
	}
	// Match longer credentials first when secrets share a prefix.
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	return values, nil
}
