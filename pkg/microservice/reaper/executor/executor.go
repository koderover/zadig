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
	"fmt"
	"io/ioutil"
	"time"

	commonconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/reaper/core/service/reaper"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

func Execute() error {
	log.Init(&log.Config{
		Level:       commonconfig.LogLevel(),
		NoCaller:    true,
		NoLogLevel:  true,
		Development: commonconfig.Mode() != setting.ReleaseMode,
	})

	start := time.Now()

	var err error
	var reaperType types.ReaperType
	defer func() {
		// Create dog food file to tell wd that task has finished.
		resultMsg := types.JobSuccess
		if err != nil {
			resultMsg = types.JobFail
			log.Errorf("Failed to run: %s.", err)
		}
		log.Infof("Job Status: %s", resultMsg)

		dogFoodErr := ioutil.WriteFile(setting.DogFood, []byte(resultMsg), 0644)
		if dogFoodErr != nil {
			log.Errorf("Failed to create dog food: %s.", dogFoodErr)
		}

		log.Infof("====================== %s End. Duration: %.2f seconds ======================", reaperType, time.Since(start).Seconds())

		// Note: Mark the task has been completed through the dogfood file, indirectly notify wd to do follow-up
		//       operations, and wait for a fixed time.
		//       Since `wd` will automatically delete the job after detecting the dogfile, this time has little
		//       effect on the overall construction time.
		time.Sleep(30 * time.Second)
	}()

	var r *reaper.Reaper
	r, err = reaper.NewReaper()
	if err != nil {
		return fmt.Errorf("failed to new reaper: %s", err)
	}
	
	reaperType = r.Type
	log.Infof("====================== %s Start ======================", reaperType)

	if err = r.BeforeExec(); err != nil {
		return fmt.Errorf("failed to prepare before building: %s", err)
	}

	if r.Ctx.ArtifactInfo == nil {
		if err = r.Exec(); err != nil {
			return fmt.Errorf("failed to build: %s", err)
		}
	}

	if err = r.AfterExec(); err != nil {
		return fmt.Errorf("failed to work after building: %s", err)
	}

	return nil
}
