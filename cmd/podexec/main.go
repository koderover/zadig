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

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/koderover/zadig/lib/microservice/podexec/server"
)

func main() {
	stopCh := make(chan struct{})

	signals := make(chan os.Signal, 1)
	defer close(signals)

	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-signals
		signal.Stop(signals)
		close(stopCh)
	}()

	if err := server.Serve(stopCh); err != nil {
		log.Fatal(err)
	}
}
