/*
Copyright 2024 The KodeRover Authors.

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

package git

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

func UpdateSubmoduleURLs(repoPath string, submoduleURLs map[string]string) error {
	modulesConfigPath := filepath.Join(repoPath, ".gitmodules")
	cfg, err := ini.Load(modulesConfigPath)
	if err != nil {
		return fmt.Errorf("unable to read .gitmodules file: %v", err)
	}

	for name, newURL := range submoduleURLs {
		section := cfg.Section(fmt.Sprintf("submodule \"%s\"", name))
		if section != nil {
			section.Key("url").SetValue(newURL)
		}
	}

	err = cfg.SaveTo(modulesConfigPath)
	if err != nil {
		return fmt.Errorf("unable to save to .gitmodules file: %v", err)
	}

	return nil
}

func GetRepoUrl(repoPath string) (string, error) {
	gitConfigPath := filepath.Join(repoPath, ".git/config")
	cfg, err := ini.Load(gitConfigPath)
	if err != nil {
		return "", fmt.Errorf("unable to read .git/config file: %v", err)
	}

	section := cfg.Section("remote \"origin\"")
	if section == nil {
		return "", fmt.Errorf("unable to find remote \"origin\" configuration")
	}

	url := section.Key("url").String()
	if url == "" {
		return "", fmt.Errorf("unable to find URL for remote \"origin\"")
	}

	return url, nil
}

func GetSubmoduleURLs(repoPath string) (map[string]string, error) {
	modulesConfigPath := filepath.Join(repoPath, ".gitmodules")
	cfg, err := ini.Load(filepath.Join(modulesConfigPath))
	if err != nil {
		return nil, fmt.Errorf("unable to read .gitmodules file: %v", err)
	}

	sections := cfg.Sections()
	submoduleURLs := make(map[string]string)
	for _, section := range sections {
		if section.HasKey("url") {
			name := section.Name()
			if strings.HasPrefix(name, "submodule") {
				name = strings.TrimPrefix(name, "submodule \"")
				name = strings.TrimSuffix(name, "\"")
				submoduleURLs[name] = section.Key("url").String()
			}
		}
	}

	return submoduleURLs, nil
}
