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
