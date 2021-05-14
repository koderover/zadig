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
	"log"
	"net/http"
	"time"

	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/taskcontroller"
)

func Serve(ctx context.Context) error {
	log.Print("Warpdrive service start ... \n")

	if err := taskcontroller.InitTaskController(ctx); err != nil {
		log.Fatalf("NewTaskController error: %v", err)
	}

	http.HandleFunc("/ping", ping)
	server := &http.Server{Addr: ":25001", Handler: nil}

	go func() {
		<-ctx.Done()

		ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Failed to stop server, error: %s\n", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Failed to start http server, error: %s\n", err)
		return err
	}

	return nil
}

func ping(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("success"))
}
