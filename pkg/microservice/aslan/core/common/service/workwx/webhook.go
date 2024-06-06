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

package workwx

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
)

func workWXApprovalCacheKey(instanceID string) string {
	return fmt.Sprint("workwx_approval_cache_", instanceID)
}

func RemoveWorkWXApprovalManager(instanceID string) {
	cache.NewRedisCache(config.RedisCommonCacheTokenDB()).Delete(workWXApprovalCacheKey(instanceID))
}

//func EventHandler(id string, body []byte, signature, ts, nonce string) (interface{}, error) {
//	log := log.SugaredLogger().With("func", "WorkWXEventHandler").With("ID", id)
//
//	log.Info("New workwx event received")
//	info, err := mongodb.NewIMAppColl().GetByID(context.Background(), id)
//	if err != nil {
//		log.Errorf("get workwx info error: %v", err)
//		return nil, errors.Wrap(err, "get workwx info error")
//	}
//}
