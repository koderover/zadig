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
	"go.uber.org/zap"

	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/webhook"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func CreateScanningModule(username string, args *Scanning, log *zap.SugaredLogger) error {
	if len(args.Name) == 0 {
		return e.ErrCreateScanningModule.AddDesc("empty Name")
	}

	err := commonservice.ProcessWebhook(args.AdvancedSetting.HookCtl.Items, nil, webhook.ScannerPrefix+args.Name, log)
	if err != nil {
		return e.ErrCreateScanningModule.AddErr(err)
	}

	scanningModule := ConvertToDBScanningModule(args)
	scanningModule.UpdatedBy = username

	err = commonrepo.NewScanningColl().Create(scanningModule)

	if err != nil {
		log.Errorf("Create scanning module %s error: %s", args.Name, err)
		return e.ErrCreateScanningModule.AddErr(err)
	}

	return nil
}

func UpdateScanningModule(id, username string, args *Scanning, log *zap.SugaredLogger) error {
	if len(args.Name) == 0 {
		return e.ErrCreateScanningModule.AddDesc("empty Name")
	}

	scanning, err := commonrepo.NewScanningColl().GetByID(id)
	if err != nil {
		log.Errorf("failed to get scanning information to update webhook, err: %s", err)
		return err
	}

	if scanning.AdvancedSetting.HookCtl.Enabled {
		err = commonservice.ProcessWebhook(args.AdvancedSetting.HookCtl.Items, scanning.AdvancedSetting.HookCtl.Items, webhook.ScannerPrefix+args.Name, log)
		if err != nil {
			log.Errorf("failed to process webhook for scanning: %s, the error is: %s", args.Name, err)
			return e.ErrUpdateScanningModule.AddErr(err)
		}
	} else {
		err = commonservice.ProcessWebhook(args.AdvancedSetting.HookCtl.Items, nil, webhook.ScannerPrefix+args.Name, log)
		if err != nil {
			log.Errorf("failed to process webhook for scanning: %s, the error is: %s", args.Name, err)
			return e.ErrUpdateScanningModule.AddErr(err)
		}
	}

	scanningModule := ConvertToDBScanningModule(args)
	scanningModule.UpdatedBy = username

	err = commonrepo.NewScanningColl().Update(id, scanningModule)

	if err != nil {
		log.Errorf("update scanning module %s error: %s", args.Name, err)
		return e.ErrUpdateScanningModule.AddErr(err)
	}

	return nil
}

func ListScanningModule(projectName string, log *zap.SugaredLogger) ([]*Scanning, int64, error) {
	scanningList, total, err := commonrepo.NewScanningColl().List(&commonrepo.ScanningListOption{ProjectName: projectName}, 0, 0)
	if err != nil {
		log.Errorf("failed to list scanning list from mongodb, the error is: %s", err)
		return nil, 0, err
	}
	resp := make([]*Scanning, 0)
	for _, scanning := range scanningList {
		resp = append(resp, ConvertDBScanningModule(scanning))
	}
	return resp, total, nil
}

func GetScanningModuleByID(id string, log *zap.SugaredLogger) (*Scanning, error) {
	scanning, err := commonrepo.NewScanningColl().GetByID(id)
	if err != nil {
		log.Errorf("failed to get scanning from mongodb, the error is: %s", err)
		return nil, err
	}
	return ConvertDBScanningModule(scanning), nil
}

func DeleteScanningModuleByID(id string, log *zap.SugaredLogger) error {
	scanning, err := commonrepo.NewScanningColl().GetByID(id)
	if err != nil {
		log.Errorf("failed to get scanning from mongodb, the error is: %s", err)
		return err
	}

	err = commonservice.ProcessWebhook(nil, scanning.AdvancedSetting.HookCtl.Items, webhook.ScannerPrefix+scanning.Name, log)
	if err != nil {
		log.Errorf("failed to process webhook for scanning module: %s, the error is: %s", id, err)
		return err
	}

	err = commonrepo.NewScanningColl().DeleteByID(id)
	if err != nil {
		log.Errorf("failed to delete scanning from mongodb, the error is: %s", err)
	}
	return err
}
