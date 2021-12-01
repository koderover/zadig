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

package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/features/repository/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/features/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/log"
)

type Feature string

const (
	ModernWorkflow             Feature = "ModernWorkflow"
	CommunityProjectRepository Feature = "CommunityProjectRepository"
	UserRegistration           Feature = "UserRegistration"
)

type FeatureGates map[Feature]bool

var Features FeatureGates = map[Feature]bool{
	ModernWorkflow:             false,
	CommunityProjectRepository: false,
	UserRegistration:           true,
}

func (fg FeatureGates) EnabledFeatures() []Feature {
	var res []Feature

	for k, v := range fg {
		if v {
			res = append(res, k)
		}
	}

	return res
}

func (fg FeatureGates) FeatureEnabled(f Feature) bool {
	return fg[f]
}

// MergeFeatureGates merge feature config from different source
// latter feature configs will be overridden by former ones
func (fg FeatureGates) MergeFeatureGates(fs ...FeatureGates) {
	for _, v := range fs {
		for k, vv := range v {
			fg[k] = vv
		}
	}
}

func DBToFeatureGates() (FeatureGates, error) {
	fg := make(FeatureGates)
	fs, err := mongodb.NewFeatureColl().ListFeatures()
	if err != nil {
		log.Errorf("list features err:%s", err)
		return nil, err
	}

	for _, v := range fs {
		fg[Feature(v.Name)] = v.Enabled
	}
	return fg, nil
}

func FlagToFeatureGates(s string) (FeatureGates, error) {
	res := make(FeatureGates)

	fs := strings.Split(s, ",")
	for _, f := range fs {
		kv := strings.Split(f, "=")
		if len(kv) != 2 {
			continue
		}
		boolValue, err := strconv.ParseBool(kv[1])
		if err != nil {
			log.Errorf("invalid value of %s=%s, err: %v", kv[0], kv[1], err)
			return nil, err
		}
		res[Feature(kv[0])] = boolValue
	}

	return res, nil
}

type FeatureReq struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

func UpdateOrCreateFeature(req *FeatureReq) error {
	if err := mongodb.NewFeatureColl().UpdateOrCreateFeature(context.TODO(), &models.Feature{
		Name:    req.Name,
		Enabled: req.Enabled,
	}); err != nil {
		return err
	}
	fg := FeatureGates{
		Feature(req.Name): req.Enabled,
	}
	Features.MergeFeatureGates(fg)
	return nil
}
