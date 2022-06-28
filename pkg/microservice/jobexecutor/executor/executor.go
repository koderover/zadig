/*
Copyright 2022 The KodeRover Authors.

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
	"fmt"
	"io/ioutil"
	"time"

	commonconfig "github.com/koderover/zadig/pkg/config"
	job "github.com/koderover/zadig/pkg/microservice/jobexecutor/core/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

// const (
// 	ZadigContextDir    = "/zadig/"
// 	ZadigLogFile       = ZadigContextDir + "zadig.log"
// 	ZadigLifeCycleFile = ZadigContextDir + "lifecycle"
// )

func Execute(ctx context.Context) error {
	// f, err := os.OpenFile(ZadigLogFile, os.O_WRONLY|os.O_CREATE|os.O_SYNC|os.O_APPEND, 0755)
	// if err != nil {
	// 	fmt.Println("open log file error")
	// 	return err
	// }
	// os.Stdout = f
	// os.Stderr = f

	log.Init(&log.Config{
		Level:       commonconfig.LogLevel(),
		NoCaller:    true,
		NoLogLevel:  true,
		Development: commonconfig.Mode() != setting.ReleaseMode,
		// SendToFile:  true,
		// Filename:    ZadigLogFile,
	})

	start := time.Now()

	excutor := "job-excutor"
	var err error
	defer func() {
		// os.Remove(ZadigLifeCycleFile)
		resultMsg := types.JobSuccess
		if err != nil {
			resultMsg = types.JobFail
			fmt.Printf("Failed to run: %s.\n", err)
		}
		fmt.Printf("Job Status: %s\n", resultMsg)
		dogFoodErr := ioutil.WriteFile(setting.DogFood, []byte(resultMsg), 0644)
		if dogFoodErr != nil {
			log.Errorf("Failed to create dog food: %s.", dogFoodErr)
		}

		fmt.Printf("====================== %s End. Duration: %.2f seconds ======================\n", excutor, time.Since(start).Seconds())
		time.Sleep(30 * time.Second)
	}()

	var j *job.Job
	j, err = job.NewJob()
	if err != nil {
		return err
	}
	fmt.Printf("====================== %s Start ======================\n", excutor)
	if err = j.Run(ctx); err != nil {
		return err
	}
	if err = j.AfterRun(ctx); err != nil {
		return err
	}
	return nil
}
