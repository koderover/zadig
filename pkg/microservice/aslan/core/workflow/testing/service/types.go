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

package service

import (
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/types"
)

type Scanning struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	ProjectName string              `json:"project_name"`
	Description string              `json:"description"`
	ScannerType string              `json:"scanner_type"`
	ImageID     string              `json:"image_id"`
	SonarID     string              `json:"sonar_id"`
	Repos       []*types.Repository `json:"repos"`
	// Parameter is for sonarQube type only
	Parameter string `json:"parameter"`
	// Script is for other type only
	Script          string                         `json:"script"`
	AdvancedSetting *types.ScanningAdvancedSetting `json:"advanced_settings"`
}

type ListScanningRespItem struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Statistics *ScanningStatistic `json:"statistics"`
	CreatedAt  int64              `json:"created_at"`
	UpdatedAt  int64              `json:"updated_at"`
}

type ScanningStatistic struct {
	TimesRun       int64 `json:"times_run"`
	AverageRuntime int64 `json:"run_time_average"`
}

func ConvertToDBScanningModule(args *Scanning) *commonmodels.Scanning {
	// ID is omitted since they are of different type and there will be no use of it
	return &commonmodels.Scanning{
		Name:            args.Name,
		ProjectName:     args.ProjectName,
		Description:     args.Description,
		ScannerType:     args.ScannerType,
		ImageID:         args.ImageID,
		SonarID:         args.SonarID,
		Repos:           args.Repos,
		Parameter:       args.Parameter,
		Script:          args.Script,
		AdvancedSetting: args.AdvancedSetting,
	}
}

func ConvertDBScanningModule(scanning *commonmodels.Scanning) *Scanning {
	return &Scanning{
		ID:              scanning.ID.Hex(),
		Name:            scanning.Name,
		ProjectName:     scanning.ProjectName,
		Description:     scanning.ProjectName,
		ScannerType:     scanning.ScannerType,
		ImageID:         scanning.ImageID,
		SonarID:         scanning.SonarID,
		Repos:           scanning.Repos,
		Parameter:       scanning.Parameter,
		Script:          scanning.Script,
		AdvancedSetting: scanning.AdvancedSetting,
	}
}
