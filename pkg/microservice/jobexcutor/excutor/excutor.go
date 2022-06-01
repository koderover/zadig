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

package excutor

import (
	"context"
	"time"

	commonconfig "github.com/koderover/zadig/pkg/config"
	job "github.com/koderover/zadig/pkg/microservice/jobexcutor/core/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

func Execute(ctx context.Context) error {
	log.Init(&log.Config{
		Level:       commonconfig.LogLevel(),
		NoCaller:    true,
		NoLogLevel:  true,
		Development: commonconfig.Mode() != setting.ReleaseMode,
	})

	start := time.Now()

	excutor := "job-excutor"
	var err error
	defer func() {
		resultMsg := types.JobSuccess
		if err != nil {
			resultMsg = types.JobFail
			log.Errorf("Failed to run: %s.", err)
		}
		log.Infof("Job Status: %s", resultMsg)

		log.Infof("====================== %s End. Duration: %.2f seconds ======================", excutor, time.Since(start).Seconds())
	}()

	var j *job.Job
	j, err = job.NewJob()
	if err != nil {
		return err
	}

	log.Infof("====================== %s Start ======================", excutor)
	if err = j.Run(ctx); err != nil {
		return err
	}
	return nil
}
