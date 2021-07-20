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

package executor

import (
	commonconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/predator/core/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

func Execute() error {
	log.Init(&log.Config{
		Level:       commonconfig.LogLevel(),
		NoCaller:    true,
		NoLogLevel:  true,
		Development: commonconfig.Mode() != setting.ReleaseMode,
	})

	pred, err := service.NewPredator()
	if err != nil {
		log.Errorf("Failed to start predator, error: %s", err)
		return err
	}

	if err := pred.BeforeExec(); err != nil {
		log.Errorf("Failed to run before exec step, error: %s", err)
		return err
	}

	if err := pred.Exec(); err != nil {
		log.Errorf("Failed to run exec step, error: %s", err)
		return err
	}
	if err := pred.AfterExec(); err != nil {
		log.Errorf("Failed to run after exec step, error: %s", err)
		return err
	}

	return nil
}
