package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func CreateAnnouncement(creater string, ctx *models.Announcement, log *zap.SugaredLogger) error {
	err := mongodb.NewAnnouncementColl().Create(ctx)
	if err != nil {
		log.Errorf("create announcement failed, creater: %s, error: %s", creater, err)
		return e.ErrCreateNotify
	}

	return nil
}

func UpdateAnnouncement(user string, notifyID string, ctx *models.Announcement, log *zap.SugaredLogger) error {
	err := mongodb.NewAnnouncementColl().Update(notifyID, ctx)
	if err != nil {
		log.Errorf("create announcement failed, user: %s, error: %s", user, err)
		return e.ErrUpdateNotify
	}

	return nil

}

func PullAllAnnouncement(user string, log *zap.SugaredLogger) ([]*models.Announcement, error) {
	resp, err := mongodb.NewAnnouncementColl().List("*")
	if err != nil {
		log.Errorf("list announcement failed, user: %s, error: %s", user, err)
		return nil, e.ErrPullAllAnnouncement
	}

	return resp, nil
}

func PullNotifyAnnouncement(user string, log *zap.SugaredLogger) ([]*models.Announcement, error) {
	resp, err := mongodb.NewAnnouncementColl().ListValidAnnouncements("*")
	if err != nil {
		log.Errorf("list announcement failed, user: %s, error: %s", user, err)
		return nil, e.ErrPullNotifyAnnouncement
	}

	return resp, nil
}
