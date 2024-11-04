package service

import (
	"strconv"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func ListTars(id, kind string, serviceNames []string) ([]*commonmodels.TarInfo, error) {
	logger := log.SugaredLogger()
	var (
		wg         wait.Group
		mutex      sync.RWMutex
		tarInfos   = make([]*commonmodels.TarInfo, 0)
		store      *commonmodels.S3Storage
		defaultS3  s3.S3
		defaultURL string
		err        error
	)

	store, err = commonrepo.NewS3StorageColl().Find(id)
	if err != nil {
		logger.Errorf("can't find store by id:%s err:%s", id, err)
		return nil, err
	}
	defaultS3 = s3.S3{
		S3Storage: store,
	}
	defaultURL, err = defaultS3.GetEncrypted()
	if err != nil {
		logger.Errorf("defaultS3 GetEncrypted err:%s", err)
		return nil, err
	}

	for _, serviceName := range serviceNames {
		// Change the service name to underscore splicing
		newServiceName := serviceName
		wg.Start(func() {
			deliveryArtifactArgs := &commonrepo.DeliveryArtifactArgs{
				Name:              newServiceName,
				Type:              kind,
				Source:            string(config.WorkflowType),
				PackageStorageURI: store.Endpoint + "/" + store.Bucket,
			}
			deliveryArtifacts, err := commonrepo.NewDeliveryArtifactColl().ListTars(deliveryArtifactArgs)
			if err != nil {
				logger.Errorf("ListTars err:%s", err)
				return
			}
			deliveryArtifactArgs.Source = string(config.WorkflowTypeV4)
			deliveryArtifacts2, err := commonrepo.NewDeliveryArtifactColl().ListTars(deliveryArtifactArgs)
			if err != nil {
				logger.Errorf("ListTars err:%s", err)
				return
			}
			deliveryArtifacts = append(deliveryArtifacts, deliveryArtifacts2...)
			deliveryArtifactArgs.Name = newServiceName + "_" + newServiceName
			newDeliveryArtifacts, err := commonrepo.NewDeliveryArtifactColl().ListTars(deliveryArtifactArgs)
			if err != nil {
				logger.Errorf("ListTars err:%s", err)
				return
			}
			deliveryArtifacts = append(deliveryArtifacts, newDeliveryArtifacts...)
			for _, deliveryArtifact := range deliveryArtifacts {
				activities, _, err := commonrepo.NewDeliveryActivityColl().List(&commonrepo.DeliveryActivityArgs{ArtifactID: deliveryArtifact.ID.Hex()})
				if err != nil {
					logger.Errorf("deliveryActivity.list err:%s", err)
					return
				}

				taskID := 0
				urlArr := strings.Split(activities[0].URL, "/")
				workflowName := urlArr[len(urlArr)-2]
				taskIDStr := urlArr[len(urlArr)-1]
				if deliveryArtifact.Source == string(config.WorkflowType) {
					taskID, err = strconv.Atoi(taskIDStr)
					if err != nil {
						logger.Errorf("WorkflowType source string convert to int err:%s", err)
						continue
					}
				} else if deliveryArtifact.Source == string(config.WorkflowTypeV4) {
					taskIDStrArr := strings.Split(taskIDStr, "?")
					taskID, err = strconv.Atoi(taskIDStrArr[0])
					if err != nil {
						logger.Errorf("WorkflowTypeV4 source string convert to int err:%s", err)
						continue
					}
				} else {
					logger.Errorf("deliveryArtifact.Source err:%s", err)
					continue
				}

				mutex.Lock()
				tarInfos = append(tarInfos, &commonmodels.TarInfo{
					URL:          defaultURL,
					Name:         newServiceName,
					FileName:     deliveryArtifact.Image,
					WorkflowName: workflowName,
					WorkflowType: deliveryArtifact.Source,
					JobTaskName:  activities[0].JobTaskName,
					TaskID:       int64(taskID),
				})
				mutex.Unlock()
			}
		})
	}
	wg.Wait()
	return tarInfos, nil
}
