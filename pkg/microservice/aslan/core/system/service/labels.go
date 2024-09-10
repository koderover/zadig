package service

import (
	"fmt"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"go.uber.org/zap"
)

func CreateServiceLabel(args *commonmodels.Label, log *zap.SugaredLogger) error {
	if args == nil {
		return fmt.Errorf("nil label")
	}

	if err := commonrepo.NewLabelColl().Create(args); err != nil {
		log.Errorf("CreateDBInstance err:%v", err)
		return err
	}

	return nil
}
