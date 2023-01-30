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
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/podexec"
	"github.com/koderover/zadig/pkg/types"
)

var cleanCacheLock sync.Mutex

const (
	CleanStatusUnStart  = "unStart"
	CleanStatusSuccess  = "success"
	CleanStatusCleaning = "cleaning"
	CleanStatusFailed   = "failed"
)

// SetCron set the docker clean cron
func SetCron(cron string, cronEnabled bool, logger *zap.SugaredLogger) error {
	dindCleans, err := commonrepo.NewDindCleanColl().List()
	if err != nil {
		logger.Errorf("list dind cleans err:%s", err)
		return err
	}
	switch len(dindCleans) {
	case 0:
		dindClean := &commonmodels.DindClean{
			Status:         CleanStatusUnStart,
			DindCleanInfos: []*commonmodels.DindCleanInfo{},
			Cron:           cron,
			CronEnabled:    cronEnabled,
		}

		if err := commonrepo.NewDindCleanColl().Upsert(dindClean); err != nil {
			return e.ErrCreateDindClean.AddErr(err)
		}
	case 1:
		dindClean := dindCleans[0]
		dindClean.Status = CleanStatusSuccess
		dindClean.DindCleanInfos = dindCleans[0].DindCleanInfos
		dindClean.Cron = cron
		dindClean.CronEnabled = cronEnabled
		if err := commonrepo.NewDindCleanColl().Upsert(dindClean); err != nil {
			return e.ErrUpdateDindClean.AddErr(err)
		}
	}
	return nil
}

func CleanImageCache(logger *zap.SugaredLogger) error {
	// TODO: We should return immediately instead of waiting for the lock if a cleanup task is performed.
	//       Since `golang-1.18`, `sync.Mutex` provides a `TryLock()` method. For now, we can continue with the previous
	//       logic and replace it with `TryLock()` after upgrading to `golang-1.18+` to return immediately.
	cleanCacheLock.Lock()
	defer cleanCacheLock.Unlock()

	dindCleans, err := commonrepo.NewDindCleanColl().List()
	if err != nil {
		return fmt.Errorf("failed to list data in `dind_clean` table: %s", err)
	}

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
		}
		dindClean.Status = CleanStatusCleaning
		dindClean.DindCleanInfos = []*commonmodels.DindCleanInfo{}
		if err := commonrepo.NewDindCleanColl().UpdateStatusInfo(dindClean); err != nil {
			return e.ErrUpdateDindClean.AddErr(err)
		}
	}

	dindPods, err := getDindPods()
	if err != nil {
		logger.Errorf("Failed to list dind pods: %s", err)
		return commonrepo.NewDindCleanColl().UpdateStatusInfo(&commonmodels.DindClean{
			Status:         CleanStatusFailed,
			DindCleanInfos: []*commonmodels.DindCleanInfo{},
		})
	}
	logger.Infof("Total dind Pods to be cleaned up: %d", len(dindPods))

	// Note: Since the total number of dind instances of Zadig users will not exceed `50` within one or two years
	// (at this time, the resource amount may be `200C400GiB`, and the resource cost is too high), concurrency can be
	// left out of consideration.
	timeout, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	res := make(chan *commonmodels.DindClean)
	go func(ch chan *commonmodels.DindClean) {
		var (
			status = CleanStatusSuccess
		)

		dindInfos := make([]*commonmodels.DindCleanInfo, 0, len(dindPods))
		var wg sync.WaitGroup
		for _, dindPod := range dindPods {
			if !wrapper.Pod(dindPod.Pod).Ready() {
				continue
			}

			logger.Infof("Begin to clean up cache of dind %q in ns %q of cluster %q.", dindPod.Pod.Name, dindPod.Pod.Namespace, dindPod.ClusterID)
			wg.Add(1)
			go func(podInfo types.DindPod) {
				defer wg.Done()
				dindInfo := &commonmodels.DindCleanInfo{
					PodName:   fmt.Sprintf("%s:%s", podInfo.ClusterName, podInfo.Pod.Name),
					StartTime: time.Now().Unix(),
				}
				output, err := dockerPrune(podInfo.ClusterID, podInfo.Pod.Namespace, podInfo.Pod.Name, logger)
				if err != nil {
					logger.Warnf("Failed to clean up cache of dind %q in ns %q of cluster %q: %s", podInfo.Pod.Name, podInfo.Pod.Namespace, podInfo.ClusterID, err)
					dindInfo.ErrorMessage = err.Error()
				}
				logger.Infof("Finish cleaning up cache of dind %q in ns %q of cluster %q.", podInfo.Pod.Name, podInfo.Pod.Namespace, podInfo.ClusterID)

				dindInfo.EndTime = time.Now().Unix()
				dindInfo.CleanInfo = output
				dindInfos = append(dindInfos, dindInfo)
			}(dindPod)
		}

		wg.Wait()
		for _, dindInfo := range dindInfos {
			if dindInfo.ErrorMessage != "" {
				status = CleanStatusFailed
				break
			}
		}
		res <- &commonmodels.DindClean{
			Status:         status,
			DindCleanInfos: dindInfos,
		}

	}(res)

	select {
	case <-timeout.Done():
		err = commonrepo.NewDindCleanColl().UpdateStatusInfo(&commonmodels.DindClean{
			Status:         CleanStatusFailed,
			DindCleanInfos: []*commonmodels.DindCleanInfo{},
		})
		if err != nil {
			logger.Errorf("failed to update dind clean info, err: %s", err.Error())
		}
	case info := <-res:
		err = commonrepo.NewDindCleanColl().UpdateStatusInfo(info)
		if err != nil {
			logger.Errorf("failed to update dind clean info, err: %s", err.Error())
		}
	}

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
		Cron:           dindCleans[0].Cron,
		CronEnabled:    dindCleans[0].CronEnabled,
	}
	return dindClean, nil
}

func dockerPrune(clusterID, namespace, podName string, logger *zap.SugaredLogger) (string, error) {
	kclient, err := kubeclient.GetClientset(config.HubServerAddress(), clusterID)
	if err != nil {
		return "", fmt.Errorf("failed to get clientset for cluster %q: %s", clusterID, err)
	}

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
	if err != nil {
		return "", fmt.Errorf("failed to get rest config for cluster %q: %s", clusterID, err)
	}

	cleanInfo, errString, _, err := podexec.KubeExec(kclient, restConfig, podexec.ExecOptions{
		Command:   []string{"docker", "system", "prune", "--volumes", "-a", "-f"},
		Namespace: namespace,
		PodName:   podName,
	})
	if err != nil {
		logger.Errorf("Failed to execute docker prune: %s, err: %s", errString, err)
		cleanInfo = errString
	}

	cleanInfoArr := strings.Split(cleanInfo, "\n\n")
	if len(cleanInfoArr) >= 2 {
		cleanInfo = cleanInfoArr[1]
	}
	cleanInfo = strings.Replace(cleanInfo, "\n", "", -1)

	return cleanInfo, err
}

func getDindPods() ([]types.DindPod, error) {
	activeClusters, err := commonrepo.NewK8SClusterColl().FindActiveClusters()
	if err != nil {
		return nil, fmt.Errorf("failed to get active cluster: %s", err)
	}

	dindPods := []types.DindPod{}
	for _, cluster := range activeClusters {
		clusterID := cluster.ID.Hex()

		var ns string
		switch clusterID {
		case setting.LocalClusterID:
			ns = config.Namespace()
		default:
			ns = setting.AttachedClusterNamespace
		}

		pods, err := getDindPodsInCluster(clusterID, ns)
		if err != nil {
			return nil, fmt.Errorf("failed to get dind pods in ns %q of cluster %q: %s", ns, clusterID, err)
		}

		for _, pod := range pods {
			dindPods = append(dindPods, types.DindPod{
				ClusterID:   clusterID,
				ClusterName: cluster.Name,
				Pod:         pod,
			})
		}
	}

	return dindPods, nil
}

func getDindPodsInCluster(clusterID, ns string) ([]*corev1.Pod, error) {
	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get kube client for cluster %q: %s", clusterID, err)
	}

	dindSelector := labels.Set{setting.ComponentLabel: "dind"}.AsSelector()
	return getter.ListPods(ns, dindSelector, kclient)
}
