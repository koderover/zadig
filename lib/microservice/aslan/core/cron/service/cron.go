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
	"errors"
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/setting"
	krkubeclient "github.com/koderover/zadig/lib/tool/kube/client"
	"github.com/koderover/zadig/lib/tool/kube/getter"
	"github.com/koderover/zadig/lib/tool/kube/updater"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func CleanJobCronJob(log *xlog.Logger) {
	kubeClient := krkubeclient.Client()
	log.Infof("start clean job...")
	workflowSelector := labels.Set{"p-type": "workflow"}.AsSelector()
	singleSelector := labels.Set{"p-type": "single"}.AsSelector()
	testSelector := labels.Set{"p-type": "test"}.AsSelector()
	cleanJob(config.Namespace(), workflowSelector, kubeClient, log)
	cleanJob(config.Namespace(), singleSelector, kubeClient, log)
	cleanJob(config.Namespace(), testSelector, kubeClient, log)
	cleanServiceJob(kubeClient, log)
	log.Infof("finnish clean job...")
}

func CleanConfigmapCronJob(log *xlog.Logger) {
	kubeClient := krkubeclient.Client()
	log.Infof("start clean configmap...")
	workflowSelector := labels.Set{"p-type": "workflow"}.AsSelector()
	singleSelector := labels.Set{"p-type": "single"}.AsSelector()
	testSelector := labels.Set{"p-type": "test"}.AsSelector()
	cleanConfigmap(config.Namespace(), workflowSelector, kubeClient, log)
	cleanConfigmap(config.Namespace(), singleSelector, kubeClient, log)
	cleanConfigmap(config.Namespace(), testSelector, kubeClient, log)
	cleanServiceConfigmap(kubeClient, log)
	log.Infof("finnish clean configmap...")
}

func cleanJob(namespace string, selector labels.Selector, client client.Client, log *xlog.Logger) {
	jobList, err := getter.ListJobs(namespace, selector, client)
	if err != nil {
		log.Infof("[%s]list jobs error: %v", namespace, err)
		return
	}
	for _, job := range jobList {
		if job.CreationTimestamp.Unix() < time.Now().Unix()-60*60*24 {
			name := job.Name
			log.Infof("[%s][%s]deleting job", namespace, name)
			err := ensureDeleteJob(namespace, name, client, log)
			if err != nil {
				log.Infof("[%s][%s]delete job error: %v", namespace, name, err)
			} else {
				log.Infof("[%s][%s]deleted job", namespace, name)
			}
		}
	}
}

func cleanConfigmap(namespace string, selector labels.Selector, client client.Client, log *xlog.Logger) {
	configmaps, err := getter.ListConfigMaps(namespace, selector, client)
	if err != nil {
		log.Infof("[%s]list configmaps error: %v", namespace, err)
		return
	}

	for _, configmap := range configmaps {
		// configmap 保留七天
		if configmap.CreationTimestamp.Unix() < time.Now().Unix()-60*60*24*7 {
			name := configmap.Name
			log.Infof("[%s][%s]deleting configmap", namespace, name)
			err := ensureDeleteConfigmap(namespace, name, client, log)
			if err != nil {
				log.Infof("[%s][%s]delete configmap error: %v", namespace, name, err)
			} else {
				log.Infof("[%s][%s]delete configmap", namespace, name)
			}
		}
	}
}

func cleanServiceJob(client client.Client, log *xlog.Logger) {
	taskServiceMap, err := commonservice.GetServiceTasks(log)
	if err != nil {
		log.Infof("cleanServiceJob getServiceTasks error: %v", err)
		return
	}
	for namespace, serviceNames := range taskServiceMap {
		for _, serviceName := range serviceNames {
			selector := labels.Set{setting.ServiceLabel: serviceName}.AsSelector()
			log.Infof("[%s][%+v] cleanServiceJob", namespace, selector)
			jobList, err := getter.ListJobs(namespace, selector, client)
			if err != nil {
				log.Infof("[%s][%s]list jobs error: %v", namespace, serviceName, err)
				continue
			}
			sort.SliceStable(jobList, func(i, j int) bool {
				return jobList[i].CreationTimestamp.Unix() > jobList[j].CreationTimestamp.Unix()
			})

			for index, job := range jobList {
				if index > 0 && job.CreationTimestamp.Unix() < time.Now().Unix()-60*60*24 {
					name := job.Name
					log.Infof("[%s][%s]deleting job", namespace, name)
					err := ensureDeleteJob(namespace, name, client, log)
					if err != nil {
						log.Infof("[%s][%s]delete job error: %v", namespace, name, err)
					} else {
						log.Infof("[%s][%s]deleted job", namespace, name)
					}
				}
			}
		}
	}
}

func cleanServiceConfigmap(client client.Client, log *xlog.Logger) {
	taskServiceMap, err := commonservice.GetServiceTasks(log)
	if err != nil {
		log.Infof("cleanServiceConfigmap getServiceTasks error: %v", err)
		return
	}
	for namespace, serviceNames := range taskServiceMap {
		for _, serviceName := range serviceNames {
			selector := labels.Set{setting.ServiceLabel: serviceName}.AsSelector()
			log.Infof("[%s][%v] cleanServiceJob", namespace, selector)
			configmaps, err := getter.ListConfigMaps(namespace, selector, client)
			if err != nil {
				log.Infof("[%s][%s]list configmaps error: %v", namespace, serviceName, err)
				continue
			}
			sort.SliceStable(configmaps, func(i, j int) bool {
				return configmaps[i].CreationTimestamp.Unix() > configmaps[j].CreationTimestamp.Unix()
			})

			for index, configmap := range configmaps {
				// configmap 保留七天
				if index > 0 && configmap.CreationTimestamp.Unix() < time.Now().Unix()-60*60*24*7 {
					name := configmap.Name
					log.Infof("[%s][%s]deleting configmap", namespace, name)
					err := ensureDeleteConfigmap(namespace, name, client, log)
					if err != nil {
						log.Infof("[%s][%s]delete configmap error: %v", namespace, name, err)
					} else {
						log.Infof("[%s][%s]delete configmap", namespace, name)
					}
				}
			}
		}
	}
}

func ensureDeleteJob(namespace, jobName string, client client.Client, log *xlog.Logger) error {
	return updater.DeleteJobAndWait(namespace, jobName, client)
}

func ensureDeleteConfigmap(namespace, configmapName string, client client.Client, log *xlog.Logger) error {

	_, found, err := getter.GetConfigMap(namespace, configmapName, client)
	if err == nil && found {
		updater.DeleteConfigMap(namespace, configmapName, client)
	}

	timeout := false

	go func() {
		<-time.After(time.Duration(20) * time.Second)
		timeout = true
	}()

	for {
		if timeout {
			return errors.New("wait job to be delete timeout")
		}

		_, found, err := getter.GetConfigMap(namespace, configmapName, client)
		if err != nil {
			log.Errorf("[ensureDeleteConfigmap] error: %v", err)
		} else if !found {
			return nil
		}

		time.Sleep(time.Second)
	}
}
