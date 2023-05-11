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
	"os"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

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
	log.Init(&log.Config{
		Level:       commonconfig.LogLevel(),
		NoCaller:    true,
		NoLogLevel:  true,
		Development: commonconfig.Mode() != setting.ReleaseMode,
		// SendToFile:  true,
		// Filename:    ZadigLogFile,
	})

	start := time.Now()

	excutor := "job-executor"

	var err error
	var j *job.Job
	j, err = job.NewJob()
	if err != nil {
		return err
	}

	ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		log.Errorf("Failed to get namespace, err: %v", err)
		return errors.Wrap(err, "get namespace")
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Errorf("failed to get InClusterConfig, err: %v", err)
		return errors.Wrap(err, "get InClusterConfig")
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Errorf("failed to get ClientSet, err: %v", err)
		return errors.Wrap(err, "get ClientSet")
	}
	configMap, err := clientset.CoreV1().ConfigMaps(string(ns)).Get(context.Background(), j.Ctx.ConfigMapName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("failed to get ConfigMap, err: %v", err)
		return errors.Wrap(err, "get configMap")
	}

	defer func() {
		resultMsg := types.JobSuccess
		if err != nil {
			resultMsg = types.JobFail
			fmt.Printf("Failed to run: %s.\n", err)
		}
		fmt.Printf("Job Status: %s\n", resultMsg)

		// set job status and outputs to job context configMap
		configMap.Data[types.JobResultKey] = string(resultMsg)
		configMap.Data[types.JobOutputsKey] = string(j.OutputsJsonBytes)
		for i := 0; i < 3; i++ {
			_, err = clientset.CoreV1().ConfigMaps(string(ns)).Update(context.Background(), configMap, metav1.UpdateOptions{})
			if err == nil {
				log.Infof("Job result ConfigMap is updated successfully")
				fmt.Printf("====================== %s End. Duration: %.2f seconds ======================\n", excutor, time.Since(start).Seconds())
				return
			} else {
				log.Errorf("failed to update job context ConfigMap: %v, retry", err)
				time.Sleep(time.Second * 3)
			}
		}
		log.Errorf("failed to update job context ConfigMap: %v", err)
	}()

	fmt.Printf("====================== %s Start ======================\n", excutor)
	if err = j.Run(ctx); err != nil {
		return err
	}
	if err = j.AfterRun(ctx); err != nil {
		return err
	}
	return nil
}
