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

package server

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/koderover/zadig/v2/pkg/config"
	config2 "github.com/koderover/zadig/v2/pkg/microservice/hubagent/config"
	"github.com/koderover/zadig/v2/pkg/microservice/hubagent/core/service"
	"github.com/koderover/zadig/v2/pkg/microservice/hubagent/server/rest"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/login"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/aslan"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	registrytool "github.com/koderover/zadig/v2/pkg/tool/registries"
	"github.com/spf13/viper"
)

func init() {
	log.Init(&log.Config{
		Level:       config.LogLevel(),
		Filename:    config.LogFile(),
		SendToFile:  config.SendLogToFile(),
		Development: config.Mode() != setting.ReleaseMode,
	})
}

func Serve(ctx context.Context) error {
	log.Info("Start Hub-Agent service.")

	engine := rest.NewEngine()
	server := &http.Server{Addr: ":80", Handler: engine}

	// need to get cluster config to init k8s resource
	initResource()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-ctx.Done()

		ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Errorf("Failed to stop server, error: %s", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Errorf("Failed to start http server, error: %s", err)
			return
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := service.Init(ctx); err != nil {
			log.Errorf("Failed to init service, error: %s", err)
			return
		}
	}()

	wg.Wait()

	return nil
}

func initResource() {
	client := aslan.NewExternal(config2.AslanBaseAddr(), "")

	scheduleWorkflow := config2.ScheduleWorkflow()
	if scheduleWorkflow == "" {
		log.Infof("failed to get scheduleWorkflow from env")
		scheduleWorkflow = "true"
	}
	schedule, err := strconv.ParseBool(scheduleWorkflow)
	if err != nil {
		log.Errorf("failed to parse scheduleWorkflow, err: %s", err)
		schedule = true
	}

	if schedule {
		token, err := login.GetInternalToken("hub-agent")
		if err != nil {
			log.Fatalf("failed to get internal token, err: %s", err)
		}
		log.Infof("token: %s", token)

		ls, err := client.ListRegistries(token)
		if err != nil {
			log.Fatalf("failed to list registries from zadig server, error: %s", err)
		}

		regList := make([]*registrytool.RegistryInfoForDinDUpdate, 0)
		for _, reg := range ls {
			regItem := &registrytool.RegistryInfoForDinDUpdate{
				ID:      reg.ID,
				RegAddr: reg.RegAddr,
			}
			if reg.AdvancedSetting != nil {
				regItem.AdvancedSetting = &registrytool.RegistryAdvancedSetting{
					TLSEnabled: reg.AdvancedSetting.TLSEnabled,
					TLSCert:    reg.AdvancedSetting.TLSCert,
				}
			}
			regList = append(regList, regItem)
		}

		clientSet, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(setting.LocalClusterID)
		if err != nil {
			log.Fatalf("failed to create dynamic kubernetes clientset for clusterID: %s, the error is: %s", setting.LocalClusterID, err)
		}

		// Get storage driver from cluster config
		storageDriver := ""
		clusterInfo, err := client.GetClusterInfo(viper.GetString("CLUSTER_ID"))
		if err == nil && clusterInfo.DindCfg != nil {
			storageDriver = clusterInfo.DindCfg.StorageDriver
		}

		err = registrytool.PrepareDinD(clientSet, "koderover-agent", regList, storageDriver)
		if err != nil {
			log.Fatalf("failed to update dind, the error is: %s", err)
		}
	}
}
