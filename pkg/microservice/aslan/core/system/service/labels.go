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
		log.Errorf("Create service label err:%v", err)
		return err
	}

	return nil
}

func ListServiceLabels(log *zap.SugaredLogger) ([]*commonmodels.Label, error) {
	labels, err := commonrepo.NewLabelColl().List()
	if err != nil {
		log.Errorf("List service label err:%v", err)
		return nil, err
	}

	return labels, nil
}

func UpdateServiceLabel(id string, args *commonmodels.Label, log *zap.SugaredLogger) error {
	err := commonrepo.NewLabelColl().Update(id, args)
	if err != nil {
		log.Errorf("update service label err:%v", err)
		return err
	}

	return nil
}

func DeleteServiceLabel(id string, log *zap.SugaredLogger) error {
	err := commonrepo.NewLabelColl().Delete(id)
	if err != nil {
		log.Errorf("delete service label err:%v", err)
		return err
	}

	return nil
}
