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

package service

import (
	"fmt"

	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/util"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func CreateApprovalTicket(req *commonmodels.ApprovalTicket, log *zap.SugaredLogger) error {
	_, err := templaterepo.NewProductColl().Find(req.ProjectKey)
	if err != nil {
		log.Errorf("failed to find project: %s, error: %s", req.ProjectKey, err)
		return e.ErrCreateApprovalTicket.AddErr(fmt.Errorf("failed to find project: %s, error: %s", req.ProjectKey, err))
	}

	if err := commonrepo.NewApprovalTicketColl().Create(req); err != nil {
		log.Errorf("create approval ticket %s error: %v", req.ApprovalID, err)
		return e.ErrCreateApprovalTicket.AddErr(err)
	}

	return nil
}

func ListApprovalTicket(projectKey, query, userID string, log *zap.SugaredLogger) ([]*commonmodels.ApprovalTicket, error) {
	userInfo, err := userclient.New().GetUserByID(userID)
	if err != nil {
		log.Errorf("list approval ticket error: failed to generate user info with ID: %s, error: %s", userID, err)
		return nil, e.ErrListApprovalTicket.AddErr(err)
	}

	resp, err := commonrepo.NewApprovalTicketColl().List(&commonrepo.ApprovalTicketListOption{
		UserEmail:  util.GetStrPointer(userInfo.Email),
		ProjectKey: util.GetStrPointer(projectKey),
		Query:      util.GetStrPointer(query),
		Status:     util.GetInt32Pointer(1),
	})

	if err != nil {
		log.Errorf("list approval ticket error: %s", err)
		return nil, e.ErrListApprovalTicket.AddErr(err)
	}

	return resp, nil
}
