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
	"context"

	commonconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/jenkinsplugin/core/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

func Execute() error {
	log.Init(&log.Config{
		Level:       commonconfig.LogLevel(),
		NoCaller:    true,
		Development: commonconfig.Mode() != setting.ReleaseMode,
	})

	log.Info("jenkins build start")

	r, err := service.NewJenkinsPlugin()
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err = r.Exec(ctx); err != nil {
		return err
	}

	return nil
}
