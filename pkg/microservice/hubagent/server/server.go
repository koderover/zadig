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
	"time"

	"github.com/koderover/zadig/pkg/config"
	config2 "github.com/koderover/zadig/pkg/microservice/hubagent/config"
	"github.com/koderover/zadig/pkg/microservice/hubagent/core/service"
	"github.com/koderover/zadig/pkg/microservice/hubagent/server/rest"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/aslan"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/log"
	registrytool "github.com/koderover/zadig/pkg/tool/registries"
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

	initDinD()

	go func() {
		<-ctx.Done()

		ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Errorf("Failed to stop server, error: %s", err)
		}
	}()

	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Errorf("Failed to start http server, error: %s", err)
			return
		}
	}()

	if err := service.Init(); err != nil {
		return err
	}

	return nil
}

func initDinD() {
	client := aslan.NewExternal(config2.AslanBaseAddr(), "")

	ls, err := client.ListRegistries()
	if err != nil {
		log.Fatalf("failed to get information from zadig server to set DinD, err: %s", err)
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

	dynamicClient, err := kubeclient.GetDynamicKubeClient(config2.AslanBaseAddr(), setting.LocalClusterID)
	if err != nil {
		log.Fatalf("failed to create dynamic kubernetes clientset for clusterID: %s, the error is: %s", setting.LocalClusterID, err)
	}

	err = registrytool.PrepareDinD(dynamicClient, "koderover-agent", regList)
	if err != nil {
		log.Fatalf("failed to update dind, the error is: %s", err)
	}
}
