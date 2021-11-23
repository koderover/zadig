package service

import (
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
)

func HandleCallback(req *commonmodels.CallbackRequest) error {
	return commonrepo.NewCallbackRequestColl().Create(req)
}
