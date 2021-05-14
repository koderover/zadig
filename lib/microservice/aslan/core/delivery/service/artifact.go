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
	"time"

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

type DeliveryArtifactInfo struct {
	*commonmodels.DeliveryArtifact
	DeliveryActivities    []*commonmodels.DeliveryActivity            `json:"activities"`
	DeliveryActivitiesMap map[string][]*commonmodels.DeliveryActivity `json:"sortedActivities,omitempty"`
}

func ListDeliveryArtifacts(deliveryArtifactArgs *commonrepo.DeliveryArtifactArgs, log *xlog.Logger) ([]*DeliveryArtifactInfo, int, error) {
	var (
		total              int = 0
		err                error
		deliveryArtifacts  []*commonmodels.DeliveryArtifact
		deliveryActivities []*commonmodels.DeliveryActivity
	)
	deliveryArtifactMapInfos := make([]*DeliveryArtifactInfo, 0)
	deliveryArtifactArgs.IsFuzzyQuery = true

	if deliveryArtifactArgs.RepoName == "" && deliveryArtifactArgs.Branch == "" {
		deliveryArtifacts, total, err = commonrepo.NewDeliveryArtifactColl().List(deliveryArtifactArgs)
		if err != nil {
			log.Errorf("list deliveryArtifact error: %v", err)
			return deliveryArtifactMapInfos, 0, e.ErrFindArtifacts
		}
		for _, deliveryArtifact := range deliveryArtifacts {
			deliveryArtifactMapInfo := new(DeliveryArtifactInfo)
			deliveryArtifactMapInfo.DeliveryArtifact = deliveryArtifact
			deliveryActivities, _, err := commonrepo.NewDeliveryActivityColl().List(&commonrepo.DeliveryActivityArgs{ArtifactID: deliveryArtifact.ID.Hex()})
			if err != nil {
				log.Errorf("list deliveryActivities error: %v", err)
				return deliveryArtifactMapInfos, 0, e.ErrFindActivities
			}
			deliveryArtifactMapInfo.DeliveryActivities = deliveryActivities
			deliveryArtifactMapInfo.DeliveryActivitiesMap = getDeliveryActivitiesMap(deliveryActivities)
			deliveryArtifactMapInfos = append(deliveryArtifactMapInfos, deliveryArtifactMapInfo)
		}
	} else {
		deliveryActivities, total, err = commonrepo.NewDeliveryActivityColl().List(&commonrepo.DeliveryActivityArgs{RepoName: deliveryArtifactArgs.RepoName, Branch: deliveryArtifactArgs.Branch, Page: deliveryArtifactArgs.Page, PerPage: deliveryArtifactArgs.PerPage})
		if err != nil {
			log.Errorf("list deliveryActivities error: %v", err)
			return nil, 0, e.ErrFindActivities
		}
		for _, deliveryActivity := range deliveryActivities {
			deliveryArtifact, err := commonrepo.NewDeliveryArtifactColl().Get(&commonrepo.DeliveryArtifactArgs{ID: deliveryActivity.ArtifactID.Hex()})
			if err != nil {
				log.Errorf("get deliveryArtifact error: %v", err)
				return nil, 0, e.ErrFindArtifact
			}
			deliveryArtifactMapInfo := new(DeliveryArtifactInfo)
			deliveryArtifactMapInfo.DeliveryArtifact = deliveryArtifact

			deliveryActivities, _, err := commonrepo.NewDeliveryActivityColl().List(&commonrepo.DeliveryActivityArgs{ArtifactID: deliveryActivity.ArtifactID.Hex()})
			if err != nil {
				log.Errorf("get deliveryActivities error: %v", err)
				return nil, 0, e.ErrFindActivities
			}
			deliveryArtifactMapInfo.DeliveryActivities = deliveryActivities
			deliveryArtifactMapInfo.DeliveryActivitiesMap = getDeliveryActivitiesMap(deliveryActivities)
			deliveryArtifactMapInfos = append(deliveryArtifactMapInfos, deliveryArtifactMapInfo)
		}
	}

	return deliveryArtifactMapInfos, total, nil
}

func GetDeliveryArtifact(deliveryArtifactArgs *commonrepo.DeliveryArtifactArgs, log *xlog.Logger) (*DeliveryArtifactInfo, error) {
	deliveryArtifactMapInfo := new(DeliveryArtifactInfo)
	deliveryArtifact, err := commonrepo.NewDeliveryArtifactColl().Get(deliveryArtifactArgs)
	if err != nil {
		log.Errorf("get deliveryArtifact error: %v", err)
		return nil, e.ErrFindArtifact
	}
	deliveryArtifactMapInfo.DeliveryArtifact = deliveryArtifact
	deliveryActivities, _, err := commonrepo.NewDeliveryActivityColl().List(&commonrepo.DeliveryActivityArgs{ArtifactID: deliveryArtifactArgs.ID})
	if err != nil {
		log.Errorf("get deliveryActivities error: %v", err)
		return nil, e.ErrFindActivities
	}
	deliveryArtifactMapInfo.DeliveryActivities = deliveryActivities
	deliveryArtifactMapInfo.DeliveryActivitiesMap = getDeliveryActivitiesMap(deliveryActivities)
	return deliveryArtifactMapInfo, nil
}

func InsertDeliveryArtifact(args *DeliveryArtifactInfo, log *xlog.Logger) (string, error) {
	deliveryArtifact := args.DeliveryArtifact
	deliveryArtifact.CreatedTime = time.Now().Unix()

	if deliveryArtifact.ImageTag != "" {
		deliveryArtifacts, _, _ := commonrepo.NewDeliveryArtifactColl().List(&commonrepo.DeliveryArtifactArgs{Name: deliveryArtifact.Name, Type: deliveryArtifact.Type, ImageTag: deliveryArtifact.ImageTag})
		if len(deliveryArtifacts) > 0 {
			log.Errorf("artifact name:%s type:%s tag:%s already exist", deliveryArtifact.Name, deliveryArtifact.Type, deliveryArtifact.ImageTag)
			return "", e.ErrCreateArtifactFailed
		}
	}
	err := commonrepo.NewDeliveryArtifactColl().Insert(deliveryArtifact)
	if err != nil {
		log.Errorf("insert deliveryArtifact error: %v", err)
		return "", e.ErrCreateArtifact
	}
	for _, deliveryActivity := range args.DeliveryActivities {
		deliveryActivity.ArtifactID = deliveryArtifact.ID
		deliveryActivity.CreatedTime = deliveryArtifact.CreatedTime
		deliveryActivity.CreatedBy = deliveryArtifact.CreatedBy
		err = commonrepo.NewDeliveryActivityColl().Insert(deliveryActivity)
		if err != nil {
			log.Errorf("insert deliveryActivity error: %v", err)
			return "", e.ErrCreateActivity
		}
	}
	return deliveryArtifact.ID.Hex(), nil
}

func UpdateDeliveryArtifact(args *commonrepo.DeliveryArtifactArgs, log *xlog.Logger) error {
	return commonrepo.NewDeliveryArtifactColl().Update(args)
}

func InsertDeliveryActivities(args *commonmodels.DeliveryActivity, deliveryArtifactID string, log *xlog.Logger) error {
	artifact, _ := commonrepo.NewDeliveryArtifactColl().Get(&commonrepo.DeliveryArtifactArgs{ID: deliveryArtifactID})
	if artifact == nil {
		log.Errorf("artifact not exist!")
		return e.ErrFindArtifact
	}
	args.CreatedTime = time.Now().Unix()
	err := commonrepo.NewDeliveryActivityColl().InsertWithId(deliveryArtifactID, args)
	if err != nil {
		log.Errorf("insert deliveryActivity error: %v", err)
		return e.ErrCreateActivity
	}
	return nil
}

func getDeliveryActivitiesMap(deliveryActivities []*commonmodels.DeliveryActivity) map[string][]*commonmodels.DeliveryActivity {
	artifactMapInfo := make(map[string][]*commonmodels.DeliveryActivity)
	for _, deliveryActivity := range deliveryActivities {
		if _, exist := artifactMapInfo[deliveryActivity.Type]; exist {
			artifactMapInfo[deliveryActivity.Type] = append(artifactMapInfo[deliveryActivity.Type], deliveryActivity)
		} else {
			buildActivities := make([]*commonmodels.DeliveryActivity, 0)
			buildActivities = append(buildActivities, deliveryActivity)
			artifactMapInfo[deliveryActivity.Type] = buildActivities
		}
	}
	return artifactMapInfo
}
