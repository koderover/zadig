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
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/koderover/zadig/lib/microservice/podexec/core/service"
)

func Serve(stopCh <-chan struct{}) error {
	log.Printf("App podexec Started at %s\n", time.Now())

	router := mux.NewRouter()
	router.HandleFunc("/api/podexec/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "success"})
	})
	router.HandleFunc("/api/podexec/{productName}/{namespace}/{podName}/{containerName}/podExec", service.ServeWs)
	router.Use(service.AuthMiddleware)
	router.Use(service.PermissionMiddleware)

	srv := &http.Server{
		Addr:         "0.0.0.0:27000",
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 5 * 60,
		Handler:      router,
	}

	go func() {
		<-stopCh

		ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Failed to stop server, error: %s\n", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Failed to start http server, error: %s\n", err)
		return err
	}

	return nil
}
