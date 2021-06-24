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

	commonconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/taskcontroller"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

func Serve(ctx context.Context) error {
	log.Init(&log.Config{
		Level:       commonconfig.LogLevel(),
		Filename:    commonconfig.LogFile(),
		SendToFile:  commonconfig.SendLogToFile(),
		Development: commonconfig.Mode() != setting.ReleaseMode,
	})

	log.Info("Warpdrive service start ... ")

	if err := taskcontroller.InitTaskController(ctx); err != nil {
		log.Fatalf("NewTaskController error: %v", err)
	}

	http.HandleFunc("/ping", ping)
	server := &http.Server{Addr: ":25001", Handler: nil}

	stopChan := make(chan struct{})
	go func() {
		defer close(stopChan)

		<-ctx.Done()

		ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Errorf("Failed to stop server, error: %s", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Errorf("Failed to start http server, error: %s", err)
		return err
	}

	<-stopChan

	return nil
}

func ping(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("success"))
}
