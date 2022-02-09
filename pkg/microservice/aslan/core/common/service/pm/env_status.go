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

package pm

import (
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
)

func GenerateEnvStatus(envConfigs []*commonmodels.EnvConfig, log *zap.SugaredLogger) ([]*commonmodels.EnvStatus, error) {
	changeEnvStatus := []*commonmodels.EnvStatus{}
	for _, envConfig := range envConfigs {
		tmpPrivateKeys, err := commonrepo.NewPrivateKeyColl().ListHostIPByArgs(&commonrepo.ListHostIPArgs{IDs: envConfig.HostIDs})
		if err != nil {
			log.Errorf("ListNameByArgs ids err:%s", err)
			return nil, err
		}

		privateKeysByLabels, err := commonrepo.NewPrivateKeyColl().ListHostIPByArgs(&commonrepo.ListHostIPArgs{Labels: envConfig.Labels})
		if err != nil {
			log.Errorf("ListNameByArgs labels err:%s", err)
			return nil, err
		}
		tmpPrivateKeys = append(tmpPrivateKeys, privateKeysByLabels...)
		privateKeysSet := sets.NewString()
		for _, v := range tmpPrivateKeys {
			tmp := commonmodels.EnvStatus{
				HostID:  v.ID.Hex(),
				EnvName: envConfig.EnvName,
				Address: v.IP,
			}
			if !privateKeysSet.Has(tmp.HostID) {
				changeEnvStatus = append(changeEnvStatus, &tmp)
				privateKeysSet.Insert(tmp.HostID)
			}
		}
	}
	return changeEnvStatus, nil
}
