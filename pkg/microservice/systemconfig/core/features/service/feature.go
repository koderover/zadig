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

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/features/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/features/repository/mongodb"
)

func FeatureEnabled(f string, log *zap.SugaredLogger) (bool, error) {
	features, err := mongodb.NewFeatureColl().ListFeatures()
	if err != nil {
		log.Errorf("failed to get feature:%s from the db, error: %s", f, err)
		return false, err
	}

	for _, feature := range features {
		if feature.Name == f {
			return feature.Enabled, nil
		}
	}

	return false, nil
}

type FeatureReq struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

func UpdateOrCreateFeature(req *FeatureReq, log *zap.SugaredLogger) error {
	err := mongodb.NewFeatureColl().UpdateOrCreateFeature(context.TODO(), &models.Feature{
		Name:    req.Name,
		Enabled: req.Enabled,
	})

	if err != nil {
		log.Errorf("failed to update feature gate: %s, err: %s", req.Name, err)
	}

	return err
}
