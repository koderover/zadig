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

package scheduler

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jasonlvhit/gocron"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/cron/core/service"
	"github.com/koderover/zadig/v2/pkg/microservice/cron/core/service/client"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/types"
)

// UpsertEnvServiceScheduler ...
func (c *CronClient) UpsertEnvServiceScheduler(log *zap.SugaredLogger) {
	envs, err := c.AslanCli.ListEnvs(log, &client.EvnListOption{DeployType: []string{setting.PMDeployType}})
	if err != nil {
		log.Error(err)
		return
	}
	//当前的环境数据和上次做比较，如果环境有删除或者环境中的服务有删除，要清理掉定时器
	c.comparePMProductRevision(envs, log)

	log.Info("[vm] start init env scheduler..")
	for _, env := range envs {
		envObj, err := c.AslanCli.GetEnvService(env.ProductName, env.EnvName, log)
		if err != nil {
			log.Errorf("[vm] GetEnvService productName: %s envName: %s err: %v", env.ProductName, env.EnvName, err)
			continue
		}
		envServiceNames := sets.String{}
		for _, serviceGroup := range envObj.Services {
			for _, svc := range serviceGroup {
				envServiceNames.Insert(svc.ServiceName)
			}
		}

		for _, serviceRevision := range env.ServiceRevisions {
			if serviceRevision.Type != setting.PMDeployType {
				continue
			}

			// delete scheduler if service is not exist or healthChecks or envConfigs is empty
			svc, err := c.AslanCli.GetService(serviceRevision.ServiceName, env.ProductName, setting.PMDeployType, serviceRevision.CurrentRevision, log)
			if err != nil {
				log.Errorf("[vm] GetService %s/%s/%d err: %v", env.ProductName, serviceRevision.ServiceName, serviceRevision.CurrentRevision, err)
			}
			if svc == nil || len(svc.HealthChecks) == 0 || len(svc.EnvConfigs) == 0 || !envServiceNames.Has(serviceRevision.ServiceName) {
				key := "service-" + serviceRevision.ServiceName + "-" + env.ProductName + "-" + setting.PMDeployType + "-" + env.EnvName

				c.SchedulersRWMutex.Lock()
				for scheduleKey := range c.Schedulers {
					if strings.Contains(scheduleKey, key) {
						c.Schedulers[scheduleKey].Clear()
						delete(c.Schedulers, scheduleKey)
					}
				}
				c.SchedulersRWMutex.Unlock()

				c.lastSchedulersRWMutex.Lock()
				for lastScheduleKey := range c.lastSchedulers {
					if strings.Contains(lastScheduleKey, key) {
						delete(c.lastSchedulers, lastScheduleKey)
					}
				}
				c.lastSchedulersRWMutex.Unlock()
				// log.Infof("[vm] [%s] deleted service scheduler..", key)
				continue
			}

			// add scheduler if service is exist and healthChecks is not empty
			for _, envStatus := range svc.EnvStatuses {
				if envStatus.EnvName != env.EnvName {
					continue
				}

				for _, healthCheck := range svc.HealthChecks {
					key := "service-" + serviceRevision.ServiceName + "-" + env.ProductName + "-" + setting.PMDeployType + "-" +
						env.EnvName + "-" + envStatus.HostID + "-" + healthCheck.Protocol + "-" + strconv.Itoa(healthCheck.Port) + "-" + healthCheck.Path

					c.lastServiceSchedulersRWMutex.Lock()
					c.lastServiceSchedulers[key] = serviceRevision
					c.lastServiceSchedulersRWMutex.Unlock()

					c.SchedulerControllerRWMutex.Lock()
					sc, ok := c.SchedulerController[key]
					c.SchedulerControllerRWMutex.Unlock()
					if ok {
						sc <- true
					}

					c.SchedulersRWMutex.Lock()
					if scheduler, ok := c.Schedulers[key]; ok {
						scheduler.Clear()
						delete(c.Schedulers, key)
					}
					c.SchedulersRWMutex.Unlock()

					newScheduler := gocron.NewScheduler()
					err = BuildScheduledEnvJob(newScheduler, healthCheck).Do(c.RunScheduledVMServiceProbe, env.ProductName, serviceRevision.ServiceName, serviceRevision.CurrentRevision, healthCheck, envStatus.Address, env.EnvName, envStatus.HostID, log)
					if err != nil {
						log.Errorf("[vm] BuildScheduledEnvJob Do key: %s, error: %v", key, err)
					}
					c.SchedulersRWMutex.Lock()
					c.Schedulers[key] = newScheduler
					c.SchedulersRWMutex.Unlock()

					c.SchedulerControllerRWMutex.Lock()
					c.SchedulerController[key] = c.Schedulers[key].Start()
					c.SchedulerControllerRWMutex.Unlock()

					// log.Infof("[vm] [%s] added service scheduler..", key)
				}
			}
			break
		}
	}
}

func (c *CronClient) RunScheduledVMServiceProbe(projectName, serviceName string, currentRevision int64, healthCheck *service.PmHealthCheck, address, envName, hostID string, log *zap.SugaredLogger) {
	key := "service-" + serviceName + "-" + fmt.Sprintf("%d", currentRevision) + "-" + projectName + "-" + setting.PMDeployType + "-" +
		envName + "-" + hostID + "-" + healthCheck.Protocol + "-" + strconv.Itoa(healthCheck.Port) + "-" + healthCheck.Path
	mutexKey := "service-" + serviceName + "-" + fmt.Sprintf("%d", currentRevision) + "-" + projectName + "-" + setting.PMDeployType
	redisMutex := cache.NewRedisLock(mutexKey)
	redisMutex.Lock()
	defer redisMutex.Unlock()

	var (
		message   string
		err       error
		envStatus = new(service.EnvStatus)
	)

	svc, err := c.AslanCli.GetService(serviceName, projectName, setting.PMDeployType, currentRevision, log)
	if err != nil {
		log.Errorf("[vm] [%s] GetService err: %v", key, err)
		return
	}

	// set env status
	for _, svcEnvStatus := range svc.EnvStatuses {
		tmp := svcEnvStatus.PmHealthCheck
		svcEnvStatus.PmHealthCheck = nil
		svcEnvStatus.PmHealthCheck = tmp

		if svcEnvStatus.EnvName == envName && svcEnvStatus.HostID == hostID {
			envStatus = svcEnvStatus
			break
		}
	}

	// set health check
	if envStatus.PmHealthCheck != nil {
		healthCheck = envStatus.PmHealthCheck
	} else {
		for _, tmpHealthCheck := range svc.HealthChecks {
			if tmpHealthCheck.Protocol == healthCheck.Protocol && tmpHealthCheck.Port == healthCheck.Port && tmpHealthCheck.Path == healthCheck.Path {
				healthCheck = tmpHealthCheck
				break
			}
		}
	}

	for i := 0; i < MaxProbeRetries; i++ {
		message, err = runProbe(healthCheck, address, log)
		if err != nil {
			log.Errorf("[vm] [%s] address %s runProbe failed, message:[%s]", key, address, message)
		} else {
			break
		}
	}

	switch message {
	case Success:
		healthCheck.CurrentHealthyNum++
		healthCheck.CurrentUnhealthyNum = 0
	case Failure:
		healthCheck.CurrentUnhealthyNum++
		healthCheck.CurrentHealthyNum = 0
	}

	envStatus.EnvName = envName
	envStatus.Address = address
	envStatus.HostID = hostID
	envStatus.PmHealthCheck = healthCheck
	if healthCheck.CurrentHealthyNum >= healthCheck.HealthyThreshold && healthCheck.CurrentHealthyNum > 0 {
		healthCheck.CurrentHealthyNum = 0
		healthCheck.CurrentUnhealthyNum = 0
		envStatus.Status = setting.PodRunning
	}

	if healthCheck.CurrentUnhealthyNum >= healthCheck.UnhealthyThreshold && healthCheck.CurrentUnhealthyNum > 0 {
		healthCheck.CurrentHealthyNum = 0
		healthCheck.CurrentUnhealthyNum = 0
		envStatus.Status = setting.PodError
	}
	envStatus.PmHealthCheck = healthCheck

	if len(svc.EnvStatuses) == 0 {
		svc.EnvStatuses = []*service.EnvStatus{envStatus}
	} else {
		envStatusKeys := sets.String{}
		for _, tmpEnvStatus := range svc.EnvStatuses {
			if tmpEnvStatus.HostID != hostID {
				continue
			}
			key := fmt.Sprintf("%s-%s-%d-%s-%s", healthCheck.Protocol, tmpEnvStatus.Address, healthCheck.Port, healthCheck.Path, tmpEnvStatus.EnvName)
			envStatusKeys.Insert(key)
			if tmpEnvStatus.Address == "" {
				tmpEnvStatus.Address = envStatus.Address
			}

			tmpEnvStatus.Status = envStatus.Status
		}
		currentEnvStatusKey := fmt.Sprintf("%s-%s-%d-%s-%s", healthCheck.Protocol, envStatus.Address, healthCheck.Port, healthCheck.Path, envStatus.EnvName)
		if !envStatusKeys.Has(currentEnvStatusKey) {
			svc.EnvStatuses = append(svc.EnvStatuses, envStatus)
		}
	}

	if err = c.AslanCli.UpdateService(&service.ServiceTmplObject{
		ProductName: svc.ProductName,
		ServiceName: svc.ServiceName,
		Revision:    svc.Revision,
		Type:        setting.PMDeployType,
		EnvStatuses: svc.EnvStatuses,
		Username:    "system",
	}, log); err != nil {
		log.Errorf("[vm] UpdateService key %s, err: %v", key, err)
	} else {
		// log.Infof("[vm] ready to UpdateService projectName:%s, serviceName:%s, revision:%d, envName:%s, address:%s, status:%s", svc.ProductName, svc.ServiceName, svc.Revision, envStatus.EnvName, envStatus.Address, envStatus.Status)
		time.Sleep(100 * time.Millisecond)
	}
}

// BuildScheduledEnvJob ...
func BuildScheduledEnvJob(scheduler *gocron.Scheduler, healthCheck *service.PmHealthCheck) *gocron.Job {
	interval := healthCheck.Interval
	if interval < 2 {
		interval = 2
	}
	return scheduler.Every(interval).Seconds()
}

func runProbe(healthCheck *service.PmHealthCheck, address string, log *zap.SugaredLogger) (string, error) {
	var (
		message string
		err     error
	)
	timeout := time.Duration(healthCheck.TimeOut) * time.Second
	switch healthCheck.Protocol {
	case setting.ProtocolHTTP, setting.ProtocolHTTPS:
		if message, err = doHTTPProbe(healthCheck.Protocol, address, healthCheck.Path, healthCheck.Port, []*types.HTTPHeader{}, timeout, "", log); err != nil {
			log.Errorf("doHttpProbe err:%v", err)
			return Failure, err
		}
	case setting.ProtocolTCP:
		if message, err = doTCPProbe(address, healthCheck.Port, timeout, log); err != nil {
			log.Errorf("doTCPProbe err:%v", err)
			return Failure, err
		}
	}

	return message, nil
}
func doTCPProbe(addr string, port int, timeout time.Duration, log *zap.SugaredLogger) (string, error) {
	var (
		conn net.Conn
		err  error
	)
	if port == 0 {
		conn, err = net.DialTimeout("tcp", addr, timeout)
	} else {
		conn, err = net.DialTimeout("tcp", fmt.Sprintf("%s:%d", addr, port), timeout)
	}

	if err != nil {
		return Failure, err
	}
	err = conn.Close()
	if err != nil {
		log.Errorf("Unexpected error closing TCP socket: %v (%#v)", err, err)
		return Failure, err
	}
	return Success, nil
}

func doHTTPProbe(protocol, address, path string, port int, headerList []*types.HTTPHeader, timeout time.Duration, responseSuccessFlag string, log *zap.SugaredLogger) (string, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	transport := &http.Transport{
		TLSClientConfig:   tlsConfig,
		DisableKeepAlives: true,
		Proxy:             http.ProxyURL(nil),
	}
	client := &http.Client{
		Timeout:       timeout,
		Transport:     transport,
		CheckRedirect: redirectChecker(false),
	}
	url, err := formatURL(protocol, address, path, port)
	if err != nil {
		return Failure, err
	}
	headers := buildHeader(headerList)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Failure, err
	}
	req.Header = headers
	req.Host = headers.Get("Host")

	res, err := client.Do(req)
	if err != nil {
		return Failure, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return Failure, err
	}

	if res.StatusCode >= http.StatusOK && res.StatusCode < http.StatusBadRequest {
		if responseSuccessFlag != "" && !strings.Contains(string(body), responseSuccessFlag) {
			return Failure, fmt.Errorf("HTTP probe failed with response success flag: %s", responseSuccessFlag)
		}
		log.Infof("Probe succeeded for %s, Response: %v", url, *res)
		return Success, nil
	}
	log.Warnf("Probe failed for %s, response body: %v", url, string(body))
	return Failure, fmt.Errorf("HTTP probe failed with statuscode: %d", res.StatusCode)
}

func redirectChecker(followNonLocalRedirects bool) func(*http.Request, []*http.Request) error {
	if followNonLocalRedirects {
		return nil // Use the default http client checker.
	}

	return func(req *http.Request, via []*http.Request) error {
		if req.URL.Hostname() != via[0].URL.Hostname() {
			return http.ErrUseLastResponse
		}
		// Default behavior: stop after 10 redirects.
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
}

func formatURL(protocol, address, path string, port int) (string, error) {
	if len(strings.Split(address, ":")) > 2 {
		return "", fmt.Errorf("illegal address")
	}
	if path == "" && port == 0 {
		return fmt.Sprintf("%s://%s", protocol, address), nil
	}

	path = strings.TrimPrefix(path, "/")

	if port == 0 {
		return fmt.Sprintf("%s://%s/%s", protocol, address, path), nil
	}
	return fmt.Sprintf("%s://%s:%d/%s", protocol, address, port, path), nil
}

func buildHeader(headerList []*types.HTTPHeader) http.Header {
	headers := make(http.Header)
	for _, header := range headerList {
		headers[header.Name] = append(headers[header.Name], header.Value)
	}
	return headers
}

func buildEnvNameKey(productRevision *service.ProductRevision) string {
	return "helm-values-sync-" + productRevision.ProductName + "-" + productRevision.EnvName
}

func (c *CronClient) compareHelmProductEnvRevision(currentProductRevisions []*service.ProductRevision, log *zap.SugaredLogger) {
	c.lastHelmProductRevisionsRWMutex.Lock()
	if len(c.lastHelmProductRevisions) == 0 {
		c.lastHelmProductRevisions = currentProductRevisions
		c.lastHelmProductRevisionsRWMutex.Unlock()
		return
	}
	deleteProductRevisions := make([]*service.ProductRevision, 0)
	curEnvSet := sets.NewString()
	for _, r := range currentProductRevisions {
		curEnvSet.Insert(buildEnvNameKey(r))
	}
	for _, lastProductRevision := range c.lastHelmProductRevisions {
		if !curEnvSet.Has(buildEnvNameKey(lastProductRevision)) {
			deleteProductRevisions = append(deleteProductRevisions, lastProductRevision)
		}
	}
	c.lastHelmProductRevisionsRWMutex.Unlock()
	// delete related schedulers when env is deleted
	for _, env := range deleteProductRevisions {
		envKey := buildEnvNameKey(env)

		c.SchedulerControllerRWMutex.Lock()
		sc, ok := c.SchedulerController[envKey]
		c.SchedulerControllerRWMutex.Unlock()
		if ok {
			sc <- true
		}

		c.SchedulersRWMutex.Lock()
		if _, ok := c.Schedulers[envKey]; ok {
			c.Schedulers[envKey].Clear()
			delete(c.Schedulers, envKey)
		}
		c.SchedulersRWMutex.Unlock()
	}
	c.lastHelmProductRevisionsRWMutex.Lock()
	c.lastHelmProductRevisions = currentProductRevisions
	c.lastHelmProductRevisionsRWMutex.Unlock()
}

// compare environments and then services
func (c *CronClient) comparePMProductRevision(currentProductRevisions []*service.ProductRevision, log *zap.SugaredLogger) {
	c.lastPMProductRevisionsRWMutex.Lock()
	if len(c.lastPMProductRevisions) == 0 {
		c.lastPMProductRevisions = currentProductRevisions
		c.lastPMProductRevisionsRWMutex.Unlock()
		return
	}
	deleteProductRevisions := make([]*service.ProductRevision, 0)
	lastProductSvcRevisionMap := make(map[string][]*service.SvcRevision)
	currentProductSvcRevisionMap := make(map[string][]*service.SvcRevision)
	for _, lastProductRevision := range c.lastPMProductRevisions {
		isContain := false
		for _, currentProductRevision := range currentProductRevisions {
			if currentProductRevision.ProductName == lastProductRevision.ProductName && currentProductRevision.EnvName == lastProductRevision.EnvName {
				currentProductSvcRevisionMap[currentProductRevision.ProductName+"-"+currentProductRevision.EnvName] = currentProductRevision.ServiceRevisions
				lastProductSvcRevisionMap[lastProductRevision.ProductName+"-"+lastProductRevision.EnvName] = lastProductRevision.ServiceRevisions
				isContain = true
				break
			}
		}
		if !isContain {
			deleteProductRevisions = append(deleteProductRevisions, lastProductRevision)
		}
	}
	c.lastPMProductRevisionsRWMutex.Unlock()
	//已经删除的环境，如果包含非k8s服务，则清除定时器
	for _, env := range deleteProductRevisions {
		for _, serviceRevision := range env.ServiceRevisions {
			if serviceRevision.Type != setting.PMDeployType {
				continue
			}
			key := "service-" + serviceRevision.ServiceName + "-" + env.ProductName + "-" + setting.PMDeployType + "-" +
				env.EnvName

			c.SchedulersRWMutex.Lock()
			for scheduleKey := range c.Schedulers {
				if strings.Contains(scheduleKey, key) {
					c.Schedulers[scheduleKey].Clear()
					delete(c.Schedulers, scheduleKey)
				}
			}
			c.SchedulersRWMutex.Unlock()

			c.lastSchedulersRWMutex.Lock()
			for lastScheduleKey := range c.lastSchedulers {
				if strings.Contains(lastScheduleKey, key) {
					delete(c.lastSchedulers, lastScheduleKey)
				}
			}
			c.lastSchedulersRWMutex.Unlock()

			c.lastServiceSchedulersRWMutex.Lock()
			for lastServiceSchedulerKey := range c.lastServiceSchedulers {
				if strings.Contains(lastServiceSchedulerKey, key) {
					delete(c.lastServiceSchedulers, lastServiceSchedulerKey)
				}
			}
			c.lastServiceSchedulersRWMutex.Unlock()
		}
	}
	// 已经删除的服务，如果是非k8s，则清除定时器
	deleteSvcRevisionsMap := make(map[string][]*service.SvcRevision)
	lastServiceMap := make(map[string]*service.SvcRevision)
	currentServicesMap := make(map[string]*service.SvcRevision)
	for key, lastSvcRevisions := range lastProductSvcRevisionMap {
		currentSvcRevisions := currentProductSvcRevisionMap[key]
		for _, lastSvcRevision := range lastSvcRevisions {
			isContain := false
			for _, currentSvcRevision := range currentSvcRevisions {
				if lastSvcRevision.ServiceName == currentSvcRevision.ServiceName {
					lastServiceMap[key+"-"+lastSvcRevision.ServiceName] = lastSvcRevision
					currentServicesMap[key+"-"+currentSvcRevision.ServiceName] = currentSvcRevision
					isContain = true
					break
				}
			}
			if !isContain {
				deleteSvcRevisionsMap[key] = append(deleteSvcRevisionsMap[key], lastSvcRevision)
			}
		}
	}

	for key, deleteSvcRevisions := range deleteSvcRevisionsMap {
		envName := strings.Split(key, "-")[1]
		productName := strings.Split(key, "-")[0]
		for _, deleteSvcRevision := range deleteSvcRevisions {
			if deleteSvcRevision.Type != setting.PMDeployType {
				continue
			}
			key := "service-" + deleteSvcRevision.ServiceName + "-" + productName + "-" + setting.PMDeployType + "-" +
				envName

			c.SchedulersRWMutex.Lock()
			for scheduleKey := range c.Schedulers {
				if strings.Contains(scheduleKey, key) {
					c.Schedulers[scheduleKey].Clear()
					delete(c.Schedulers, scheduleKey)
				}
			}
			c.SchedulersRWMutex.Unlock()

			c.lastSchedulersRWMutex.Lock()
			for lastScheduleKey := range c.lastSchedulers {
				if strings.Contains(lastScheduleKey, key) {
					delete(c.lastSchedulers, lastScheduleKey)
				}
			}
			c.lastSchedulersRWMutex.Unlock()

			c.lastServiceSchedulersRWMutex.Lock()
			for lastServiceSchedulerKey := range c.lastServiceSchedulers {
				if strings.Contains(lastServiceSchedulerKey, key) {
					delete(c.lastServiceSchedulers, lastServiceSchedulerKey)
				}
			}
			c.lastServiceSchedulersRWMutex.Unlock()
		}
	}

	//判断相同的服务，revision是否相同，如果revision相同在判断env_configs和health_checks是否相同
	//找出老的revision的service
	oldRevisionServices := make(map[string]*service.SvcRevision)
	for lastKey, lastServiceRevision := range lastServiceMap {
		currentRevision := currentServicesMap[lastKey]
		if lastServiceRevision.CurrentRevision < currentRevision.CurrentRevision {
			oldRevisionServices[lastKey] = lastServiceRevision
		}
	}
	//清理掉老版本的探活定时器
	for key, oldRevisionService := range oldRevisionServices {
		if oldRevisionService.Type != setting.PMDeployType {
			continue
		}
		envName := strings.Split(key, "-")[1]
		productName := strings.Split(key, "-")[0]
		key := "service-" + oldRevisionService.ServiceName + "-" + productName + "-" + setting.PMDeployType + "-" +
			envName

		c.SchedulersRWMutex.Lock()
		for scheduleKey := range c.Schedulers {
			if strings.Contains(scheduleKey, key) {
				c.Schedulers[scheduleKey].Clear()
				delete(c.Schedulers, scheduleKey)
			}
		}
		c.SchedulersRWMutex.Unlock()

		c.lastSchedulersRWMutex.Lock()
		for lastScheduleKey := range c.lastSchedulers {
			if strings.Contains(lastScheduleKey, key) {
				delete(c.lastSchedulers, lastScheduleKey)
			}
		}
		c.lastSchedulersRWMutex.Unlock()

		c.lastServiceSchedulersRWMutex.Lock()
		for lastServiceSchedulerKey := range c.lastServiceSchedulers {
			if strings.Contains(lastServiceSchedulerKey, key) {
				delete(c.lastServiceSchedulers, lastServiceSchedulerKey)
			}
		}
		c.lastServiceSchedulersRWMutex.Unlock()
	}

	c.lastPMProductRevisionsRWMutex.Lock()
	c.lastPMProductRevisions = currentProductRevisions
	c.lastPMProductRevisionsRWMutex.Unlock()
}

const (
	// Success Result
	Success string = "success"
	// Failure Result
	Failure string = "failure"
	// maxProbeRetries
	MaxProbeRetries = 3
)
