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
package cmd

import (
	"net/http"
	"syscall"

	"github.com/koderover/zadig/pkg/shared/client/policy"
	"github.com/koderover/zadig/pkg/shared/client/user"
	"github.com/koderover/zadig/pkg/tool/log"
	"k8s.io/apiserver/pkg/server/healthz"
)

// InitHealthChecker
var InitHealthChecker healthz.HealthChecker = initCheck{}

type initCheck struct{}

func (initCheck) Name() string {
	return "initHealthChecker"
}

func (initCheck) Check(req *http.Request) error {
	if err := checkUserServiceHealth(); err != nil {
		log.Error("checkUserServiceHealth error:", err.Error())
		return err
	}
	if err := checkPolicyServiceHealth(); err != nil {
		log.Error("checkPolicyServiceHealth error:", err.Error())
		return err
	}

	once.Do(func() {
		go func() {
			defer syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
			err := Run()
			if err != nil {
				log.Errorf("once do init Run err:%s", err)
				return
			}
			log.Info("zadig init success")
		}()
	})
	return nil
}

func checkUserServiceHealth() error {
	return user.New().Healthz()
}

func checkPolicyServiceHealth() error {
	return policy.NewDefault().Healthz()
}
