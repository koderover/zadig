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
	"sync"
	"time"

	commonconfig "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/hubserver/core/service"
	"github.com/koderover/zadig/v2/pkg/microservice/hubserver/server/rest"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/remotedialer"
)

func Serve(ctx context.Context) error {
	log.Init(&log.Config{
		Level:       commonconfig.LogLevel(),
		Filename:    commonconfig.LogFile(),
		SendToFile:  commonconfig.SendLogToFile(),
		Development: commonconfig.Mode() != setting.ReleaseMode,
	})

	log.Info("hub server start...")
	service.Init()

	handler := remotedialer.New(service.Authorize, remotedialer.DefaultErrorWriter, service.DeleteClusterInfo)
	engine := rest.NewEngine(ctx, handler)
	server := &http.Server{Addr: ":26000", Handler: engine}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-ctx.Done()

		clusters, err := service.GetClustersByPodIP(commonconfig.PodIP())
		if err != nil {
			log.Errorf("Failed to get clusters for pod IP %s: %v", commonconfig.PodIP(), err)
			return
		}

		for _, cluster := range clusters {
			if err := service.DeleteClusterInfo(cluster.ID.Hex()); err != nil {
				log.Errorf("Failed to delete cluster info for cluster %s: %v", cluster.ID.Hex(), err)
			} else {
				log.Infof("Successfully deleted cluster info for cluster %s", cluster.ID.Hex())
			}
		}

		ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Errorf("Failed to shutdown server, error: %s", err)
		} else {
			log.Infof("Shutdown server successfully")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		service.Sync(handler, ctx.Done())
		service.Reset()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		service.CheckReplicas(ctx, handler)
	}()

	go func() {
		service.CheckConnectionStatus(ctx, handler)
	}()

	rest.SetReady(true)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Errorf("Failed to start http server, error: %s\n", err)
		return err
	}

	wg.Wait()

	return nil
}
