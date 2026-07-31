/*
Copyright 2025 The KodeRover Authors.

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
	"strings"
	"sync"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
)

func ValidateReviewRules(rules []*commonmodels.ReviewRule) error {
	paths := make(map[string]struct{}, len(rules))
	for i, rule := range rules {
		if rule == nil {
			return fmt.Errorf("review rule at index %d cannot be nil", i)
		}
		path := strings.TrimSpace(rule.Path)
		if path == "" {
			return fmt.Errorf("review rule path at index %d cannot be empty", i)
		}
		if _, ok := paths[path]; ok {
			return fmt.Errorf("duplicate review rule path %q", path)
		}
		rule.Path = path
		paths[path] = struct{}{}
	}
	return nil
}

func MergeReviewRules(scanningRules, systemRules []*commonmodels.ReviewRule) []*commonmodels.ReviewRule {
	result := make([]*commonmodels.ReviewRule, 0, len(scanningRules)+len(systemRules))
	overridden := make(map[string]struct{}, len(scanningRules))
	for _, rule := range scanningRules {
		if rule == nil {
			continue
		}
		result = append(result, rule)
		overridden[rule.Path] = struct{}{}
	}
	for _, rule := range systemRules {
		if rule == nil {
			continue
		}
		if _, ok := overridden[rule.Path]; ok {
			continue
		}
		result = append(result, rule)
	}
	return result
}

type ScanService struct {
	IDScanMap       sync.Map
	NamedScanMap    sync.Map
	ScanTemplateMap sync.Map
}

func NewScanningService() *ScanService {
	return &ScanService{
		IDScanMap:       sync.Map{},
		NamedScanMap:    sync.Map{},
		ScanTemplateMap: sync.Map{},
	}
}

func (c *ScanService) GetByID(id string) (*commonmodels.Scanning, error) {
	var err error
	scanInfo := new(commonmodels.Scanning)
	scanMapValue, ok := c.IDScanMap.Load(id)
	if !ok {
		scanInfo, err = commonrepo.NewScanningColl().GetByID(id)
		if err != nil {
			c.IDScanMap.Store(id, nil)
			return nil, fmt.Errorf("find scan: %s error: %v", id, err)
		}
		c.IDScanMap.Store(id, scanInfo)
	} else {
		if scanMapValue == nil {
			return nil, fmt.Errorf("failed to find scanning: %s", id)
		}
		scanInfo = scanMapValue.(*commonmodels.Scanning)
	}

	if err := FillScanningDetail(scanInfo, &c.ScanTemplateMap); err != nil {
		return nil, err
	}
	return scanInfo, nil
}

func (c *ScanService) GetByName(projectName, name string) (*commonmodels.Scanning, error) {
	var err error
	scanInfo := new(commonmodels.Scanning)
	key := fmt.Sprintf("%s++%s", projectName, name)
	buildMapValue, ok := c.NamedScanMap.Load(key)
	if !ok {
		scanInfo, err = commonrepo.NewScanningColl().Find(projectName, name)
		if err != nil {
			c.NamedScanMap.Store(key, nil)
			return nil, fmt.Errorf("find scan: %s error: %v", key, err)
		}
		c.NamedScanMap.Store(key, scanInfo)
	} else {
		if buildMapValue == nil {
			return nil, fmt.Errorf("failed to find scanning: %s", key)
		}
		scanInfo = buildMapValue.(*commonmodels.Scanning)
	}

	if err := FillScanningDetail(scanInfo, &c.ScanTemplateMap); err != nil {
		return nil, err
	}
	return scanInfo, nil
}

func FillScanningDetail(moduleScanning *commonmodels.Scanning, cacheMap *sync.Map) error {
	if moduleScanning.TemplateID == "" {
		return nil
	}

	var err error
	var templateInfo *commonmodels.ScanningTemplate

	if cacheMap == nil {
		templateInfo, err = commonrepo.NewScanningTemplateColl().Find(&commonrepo.ScanningTemplateQueryOption{
			ID: moduleScanning.TemplateID,
		})
		if err != nil {
			return fmt.Errorf("failed to find scanning template with id: %s, err: %s", moduleScanning.TemplateID, err)
		}
	} else {
		buildTemplateMapValue, ok := cacheMap.Load(moduleScanning.TemplateID)
		if !ok {
			templateInfo, err = commonrepo.NewScanningTemplateColl().Find(&commonrepo.ScanningTemplateQueryOption{
				ID: moduleScanning.TemplateID,
			})
			if err != nil {
				return fmt.Errorf("failed to find scanning template with id: %s, err: %s", moduleScanning.TemplateID, err)
			}
			cacheMap.Store(moduleScanning.TemplateID, templateInfo)
		} else {
			templateInfo = buildTemplateMapValue.(*commonmodels.ScanningTemplate)
		}
	}

	moduleScanning.Infrastructure = templateInfo.Infrastructure
	moduleScanning.VMLabels = templateInfo.VMLabels
	moduleScanning.ScannerType = templateInfo.ScannerType
	moduleScanning.EnableScanner = templateInfo.EnableScanner
	moduleScanning.ImageID = templateInfo.ImageID
	moduleScanning.SonarID = templateInfo.SonarID
	moduleScanning.Installs = templateInfo.Installs
	moduleScanning.Parameter = templateInfo.Parameter
	moduleScanning.Envs = MergeBuildEnvs(templateInfo.Envs.ToRuntimeList(), moduleScanning.Envs.ToRuntimeList()).ToKVList()
	moduleScanning.ScriptType = templateInfo.ScriptType
	moduleScanning.Script = templateInfo.Script
	moduleScanning.AdvancedSetting = templateInfo.AdvancedSetting
	moduleScanning.CheckQualityGate = templateInfo.CheckQualityGate

	return nil
}
