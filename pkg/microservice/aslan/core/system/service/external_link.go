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
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListExternalLinks(log *zap.SugaredLogger) ([]*commonmodels.ExternalLink, error) {
	resp, err := commonrepo.NewExternalLinkColl().List()
	if err != nil {
		log.Errorf("ExternalLink.List error: %s", err)
		return resp, e.ErrListExternalLink.AddErr(err)
	}
	return resp, nil
}

func CreateExternalLink(args *commonmodels.ExternalLink, log *zap.SugaredLogger) error {
	err := commonrepo.NewExternalLinkColl().Create(args)
	if err != nil {
		log.Errorf("ExternalLink.Create error: %s", err)
		return e.ErrCreateExternalLink.AddErr(err)
	}
	return nil
}

func UpdateExternalLink(id string, args *commonmodels.ExternalLink, log *zap.SugaredLogger) error {
	err := commonrepo.NewExternalLinkColl().Update(id, args)
	if err != nil {
		log.Errorf("ExternalLink.Update %s error: %s", id, err)
		return e.ErrUpdateExternalLink.AddErr(err)
	}
	return nil
}

func DeleteExternalLink(id string, log *zap.SugaredLogger) error {
	err := commonrepo.NewExternalLinkColl().Delete(id)
	if err != nil {
		log.Errorf("ExternalLink.Delete %s error: %s", id, err)
		return e.ErrDeleteExternalLink.AddErr(err)
	}
	return nil
}
