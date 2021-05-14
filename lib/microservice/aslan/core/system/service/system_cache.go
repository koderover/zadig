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
	"sort"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/koderover/zadig/lib/internal/kube/wrapper"
	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	kubetool "github.com/koderover/zadig/lib/tool/kube/client"
	"github.com/koderover/zadig/lib/tool/kube/getter"
	"github.com/koderover/zadig/lib/tool/kube/podexec"
	"github.com/koderover/zadig/lib/tool/xlog"
)

const (
	CleanStatusUnStart  = "unStart"
	CleanStatusSuccess  = "success"
	CleanStatusCleaning = "cleaning"
	CleanStatusFailed   = "failed"
)

// CleanImageCache 清理镜像缓存
func CleanImageCache(logger *xlog.Logger) error {
	//Get pod list by label and namespace
	//判断当前的状态，cleaning状态下不做清理
	var mu sync.Mutex
	mu.Lock()
	defer func() {
		mu.Unlock()
	}()

	dindCleans, _ := commonrepo.NewDindCleanColl().List()
	switch len(dindCleans) {
	case 0:
		dindClean := &commonmodels.DindClean{
			Status:         CleanStatusCleaning,
			DindCleanInfos: []*commonmodels.DindCleanInfo{},
		}

		if err := commonrepo.NewDindCleanColl().Upsert(dindClean); err != nil {
			return e.ErrCreateDindClean.AddErr(err)
		}
	case 1:
		dindClean := dindCleans[0]
		if dindClean.Status == CleanStatusCleaning {
			return e.ErrDindClean.AddDesc("")
		} else {
			dindClean.Status = CleanStatusCleaning
			dindClean.DindCleanInfos = []*commonmodels.DindCleanInfo{}
			if err := commonrepo.NewDindCleanColl().Upsert(dindClean); err != nil {
				return e.ErrUpdateDindClean.AddErr(err)
			}
		}
	}

	go func() {
		var (
			namespace = config.Namespace()
			status    = CleanStatusSuccess
		)

		selector := labels.Set{setting.ServiceNameLabel: "dind"}.AsSelector()
		pods, err := getter.ListPods(namespace, selector, kubetool.Client())

		if err != nil {
			logger.Errorf("[%s]list dind pods error: %v", namespace, err)
			commonrepo.NewDindCleanColl().Upsert(&commonmodels.DindClean{
				Status:         CleanStatusFailed,
				DindCleanInfos: []*commonmodels.DindCleanInfo{},
			})
			return
		}

		dindInfos := make([]*commonmodels.DindCleanInfo, 0, len(pods))
		var wg sync.WaitGroup
		for _, pod := range pods {
			if wrapper.Pod(pod).Ready() {
				logger.Infof("[%s] pod [%s] dind cache cleaning", namespace, pod.Name)
				wg.Add(1)
				go func(podName string) {
					defer wg.Done()
					dindInfo := &commonmodels.DindCleanInfo{
						PodName:   podName,
						StartTime: time.Now().Unix(),
					}
					output, err := dockerPrune(namespace, podName, logger)
					if err != nil {
						logger.Errorf("[%s] pod [%s] dind cache clean error: %v", namespace, podName, err)
						dindInfo.ErrorMessage = err.Error()
					}
					logger.Infof("[%s] pod [%s] dind cache finish", namespace, podName)
					dindInfo.EndTime = time.Now().Unix()
					dindInfo.CleanInfo = output
					dindInfos = append(dindInfos, dindInfo)
				}(pod.Name)
			}
		}
		wg.Wait()

		for _, dindInfo := range dindInfos {
			if dindInfo.ErrorMessage != "" {
				status = CleanStatusFailed
				break
			}
		}
		commonrepo.NewDindCleanColl().Upsert(&commonmodels.DindClean{
			Status:         status,
			DindCleanInfos: dindInfos,
		})
	}()

	return nil
}

// GetOrCreateCleanCacheState 获取清理镜像缓存状态，如果数据库中没有数据返回一个临时对象
func GetOrCreateCleanCacheState() (*commonmodels.DindClean, error) {
	var dindClean *commonmodels.DindClean
	dindCleans, _ := commonrepo.NewDindCleanColl().List()
	if len(dindCleans) == 0 {
		dindClean = &commonmodels.DindClean{
			Status:         CleanStatusUnStart,
			DindCleanInfos: []*commonmodels.DindCleanInfo{},
			UpdateTime:     time.Now().Unix(),
		}
		return dindClean, nil
	}

	sort.SliceStable(dindCleans[0].DindCleanInfos, func(i, j int) bool {
		return dindCleans[0].DindCleanInfos[i].PodName < dindCleans[0].DindCleanInfos[j].PodName
	})

	dindClean = &commonmodels.DindClean{
		ID:             dindCleans[0].ID,
		Status:         dindCleans[0].Status,
		UpdateTime:     dindCleans[0].UpdateTime,
		DindCleanInfos: dindCleans[0].DindCleanInfos,
	}
	return dindClean, nil
}

func dockerPrune(namespace, podName string, logger *xlog.Logger) (string, error) {
	cleanInfo, errString, _, err := podexec.ExecWithOptions(podexec.ExecOptions{
		Command:   []string{"docker", "system", "prune", "--volumes", "-a", "-f"},
		Namespace: namespace,
		PodName:   podName,
	})

	if err != nil {
		logger.Infof("Failed to execute docker prune: %s, err: %s", errString, err)
		cleanInfo = errString
	}

	cleanInfoArr := strings.Split(cleanInfo, "\n\n")
	if len(cleanInfoArr) >= 2 {
		cleanInfo = cleanInfoArr[1]
	}
	cleanInfo = strings.Replace(cleanInfo, "\n", "", -1)

	return cleanInfo, err
}
