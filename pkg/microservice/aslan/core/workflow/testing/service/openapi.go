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
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	openapitool "github.com/koderover/zadig/pkg/tool/openapi"
	"github.com/koderover/zadig/pkg/types"
)

func OpenAPICreateScanningModule(username string, args *OpenAPICreateScanningReq, log *zap.SugaredLogger) error {
	scanning, err := generateScanningModuleFromOpenAPIInput(args, log)
	if err != nil {
		log.Errorf("failed to generate scanning module from input, err: %s", err)
		return err
	}

	return CreateScanningModule(username, scanning, log)
}

func generateScanningModuleFromOpenAPIInput(req *OpenAPICreateScanningReq, log *zap.SugaredLogger) (*Scanning, error) {
	ret := &Scanning{
		Name:             req.Name,
		ProjectName:      req.ProjectName,
		Description:      req.Description,
		ScannerType:      req.ScannerType,
		Installs:         req.Addons,
		PreScript:        req.PrelaunchScript,
		Parameter:        req.SonarParameter,
		Script:           req.Script,
		CheckQualityGate: req.EnableQualityGate,
	}
	// since only one sonar system can be integrated, use that as the sonarID
	if req.ScannerType == "sonarQube" {
		sonarInfo, total, err := mongodb.NewSonarIntegrationColl().List(context.TODO(), 1, 20)
		if err != nil {
			log.Errorf("failed to list sonar integration, err is: %s", err)
			return nil, fmt.Errorf("didn't find the sonar integration to fill in")
		}
		if total != 1 {
			return nil, fmt.Errorf("there are more than 1 sonar integration in this system, which we didn't allow.")
		}
		ret.SonarID = sonarInfo[0].ID.Hex()
	}

	// find the correct image info to fill in
	imageInfo, err := mongodb.NewBasicImageColl().FindByImageName(req.ImageName)
	if err != nil {
		log.Errorf("failed to find the image name by tag")
	}
	ret.ImageID = imageInfo.ID.Hex()

	// advanced setting conversion
	scanningAdvancedSetting, err := openapitool.ToScanningAdvancedSetting(req.AdvancedSetting)
	if err != nil {
		log.Errorf("failed to convert openAPI scanning hook info to system hook info, err: %s", err)
		return nil, err
	}
	ret.AdvancedSetting = scanningAdvancedSetting

	//repo info conversion
	repoList := make([]*types.Repository, 0)
	for _, repo := range req.RepoInfo {
		systemRepoInfo, err := openapitool.ToScanningRepository(repo)
		if err != nil {
			log.Errorf("failed to convert user repository input info into system repository, codehostName: [%s], repoName[%s], err: %s", repo.CodeHostName, repo.RepoName, err)
			return nil, fmt.Errorf("failed to convert user repository input info into system repository, codehostName: [%s], repoName[%s], err: %s", repo.CodeHostName, repo.RepoName, err)
		}
		repoList = append(repoList, systemRepoInfo)
	}

	ret.Repos = repoList
	return ret, nil
}
