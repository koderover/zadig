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

package scheduler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/rfyiamcool/cronlib"

	"github.com/koderover/zadig/v2/pkg/microservice/cron/core/service"
	"github.com/koderover/zadig/v2/pkg/microservice/cron/core/service/client"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util"
)

const (
	InitializeThreshold = 5 * time.Minute
	PullInterval        = 3 * time.Second
)

type CronjobHandler struct {
	aslanCli  *client.Client
	Scheduler *cronlib.CronSchduler
}

func NewCronjobHandler(client *client.Client, scheduler *cronlib.CronSchduler) *CronjobHandler {
	InitExistedCronjob(client, scheduler)

	return &CronjobHandler{
		aslanCli:  client,
		Scheduler: scheduler,
	}
}

func InitExistedCronjob(client *client.Client, scheduler *cronlib.CronSchduler) {
	log.Infof("Initializing existing cronjob ....")

	initChan := make(chan []*service.Cronjob, 1)
	defer close(initChan)

	failsafeChan := make(chan []*service.Cronjob, 1)
	defer close(failsafeChan)

	var (
		jobList         []*service.Cronjob
		failsafeJobList []*service.Cronjob
	)
	listAPI := fmt.Sprintf("%s/cron/cronjob", client.APIBase)
	// failsafe function: get enabled workflow and register them
	failsafeAPI := fmt.Sprintf("%s/cron/cronjob/failsafe", client.APIBase)

	cl := httpclient.New(
		httpclient.SetRetryCount(100),
		httpclient.SetRetryWaitTime(PullInterval),
	)
	go func() {
		_, err := cl.Get(listAPI, httpclient.SetResult(&jobList))
		if err != nil {
			log.Errorf("Failed to get cronjob, err: %s", err)
			return
		}
		initChan <- jobList
	}()

	go func() {
		_, err := cl.Get(failsafeAPI, httpclient.SetResult(&failsafeJobList))
		if err != nil {
			log.Errorf("Failed to get failsafe cronjob, err: %s", err)
			return
		}
		failsafeChan <- failsafeJobList
	}()

	timeout := time.After(InitializeThreshold)

	select {
	case jobList = <-initChan:
		for _, job := range jobList {
			err := registerCronjob(job, client, scheduler)
			if err != nil {
				fmt.Printf("Failed to init job with id: %s, err: %s\n", job.ID, err)
			}
		}
	case <-timeout:
		log.Fatalf("Failed to get aslan response after 5min, exiting...")
	}

	select {
	case jobList = <-failsafeChan:
		for _, job := range jobList {
			err := registerCronjob(job, client, scheduler)
			if err != nil {
				fmt.Printf("Failed to init job with id: %s, err: %s\n", job.ID, err)
			}
		}
	case <-timeout:
		log.Fatalf("Failed to get aslan response after 5min, exiting...")
	}

	log.Infof("cronjob initialization complete ...")
}

// HandleMessage ...
func (h *CronjobHandler) HandleMessage(msgs []*service.CronjobPayload) error {
	for _, msg := range msgs {
		switch msg.Action {
		case setting.TypeEnableCronjob:
			err := h.updateCronjob(msg.Name, msg.ProductName, msg.JobType, msg.JobList, msg.DeleteList)
			if err != nil {
				log.Errorf("Failed to update cronjob, the error is: %v", err)
				return err
			}
		case setting.TypeDisableCronjob:
			err := h.stopCronjob(msg.Name, msg.JobType)
			if err != nil {
				log.Errorf("Failed to stop all cron job, the error is: %v", err)
				return err
			}
		default:
			log.Errorf("unsupported cronjob action: NOT RECONSUMING")
		}
	}

	return nil
}

func (h *CronjobHandler) updateCronjob(name, productName, jobType string, jobList []*service.Schedule, deleteList []string) error {
	//首先根据deleteList停止不需要的cronjob
	for _, deleteID := range deleteList {
		jobID := deleteID
		log.Infof("stopping Job of ID: %s", jobID)
		h.Scheduler.StopService(jobID)
	}
	// 根据job内容来在scheduler中新增cronjob
	for _, job := range jobList {
		log.Debugf("job: %+v", job)
		var cron string
		if job.Type == setting.FixedGapCronjob || job.Type == setting.FixedDayTimeCronjob {
			cronString, err := convertFixedTimeToCron(job)
			if err != nil {
				return err
			}
			cron = cronString
		} else {
			cron = fmt.Sprintf("%s%s", "0 ", job.Cron)
		}
		switch jobType {
		case setting.WorkflowCronjob:
			err := h.registerWorkFlowJob(name, cron, job)
			if err != nil {
				return err
			}
		case setting.TestingCronjob:
			err := h.registerTestJob(name, productName, cron, job)
			if err != nil {
				return err
			}
		case setting.WorkflowV4Cronjob:
			err := h.registerWorkFlowV4Job(name, cron, job)
			if err != nil {
				return err
			}
		case setting.EnvAnalysisCronjob:
			err := h.registerEnvAnalysisJob(name, cron, job)
			if err != nil {
				return err
			}
		case setting.EnvSleepCronjob:
			err := h.registerEnvSleepJob(name, cron, job)
			if err != nil {
				return err
			}
		case setting.ReleasePlanCronjob:
			log.Debugf("registering release plan job, name: %v, cron: %v, jobID: %v", name, cron, job.ID)
			err := h.registerReleasePlanJob(name, cron, job)
			if err != nil {
				return err
			}
		default:
			log.Errorf("unrecognized cron job type for job id: %s", job.ID)
		}
	}
	return nil
}

func convertFixedTimeToCron(job *service.Schedule) (string, error) {
	return convertCronString(string(job.Type), job.Time, job.Frequency, job.Number)
}

func convertCronString(jobType, time, frequency string, number uint64) (string, error) {
	var buf bytes.Buffer
	// 无秒级支持
	buf.WriteString("0 ")
	if jobType == setting.FixedDayTimeCronjob {
		timeString := strings.Split(time, ":")
		if len(timeString) != 2 {
			log.Errorf("Failed to format the time string")
			return "", errors.New("time string format error")
		}
		timeCron := fmt.Sprintf("%s %s ", timeString[1], timeString[0])
		buf.WriteString(timeCron)
	}

	switch frequency {
	case setting.FrequencyDay:
		buf.WriteString("*/1 * *")
	case setting.FrequencyMondy:
		buf.WriteString("* * 1")
	case setting.FrequencyTuesday:
		buf.WriteString("* * 2")
	case setting.FrequencyWednesday:
		buf.WriteString("* * 3")
	case setting.FrequencyThursday:
		buf.WriteString("* * 4")
	case setting.FrequencyFriday:
		buf.WriteString("* * 5")
	case setting.FrequencySaturday:
		buf.WriteString("* * 6")
	case setting.FrequencySunday:
		buf.WriteString("* * 0")
	case setting.FrequencyMinutes:
		gapCron := fmt.Sprintf("*/%d * * * *", number)
		buf.WriteString(gapCron)
	case setting.FrequencyHours:
		gapCron := fmt.Sprintf("0 */%d * * *", number)
		buf.WriteString(gapCron)
	}

	return buf.String(), nil
}

func (h *CronjobHandler) registerWorkFlowJob(name, schedule string, job *service.Schedule) error {
	args := &service.WorkflowTaskArgs{
		WorkflowName:       name,
		WorklowTaskCreator: setting.CronTaskCreator,
	}
	if job.WorkflowArgs != nil {
		args.Description = job.WorkflowArgs.Description
		args.ProductTmplName = job.WorkflowArgs.ProductTmplName
		args.Target = job.WorkflowArgs.Target
		args.Namespace = job.WorkflowArgs.Namespace
		args.Tests = job.WorkflowArgs.Tests
		args.DistributeEnabled = job.WorkflowArgs.DistributeEnabled
	}
	scheduleJob, err := cronlib.NewJobModel(schedule, func() {
		if err := h.aslanCli.ScheduleCall(path.Join("workflow/workflowtask", args.WorkflowName), args, log.SugaredLogger()); err != nil {
			log.Errorf("[%s]RunScheduledTask err: %v", name, err)
		}
	})
	if err != nil {
		log.Errorf("Failed to create job of ID: %s, the error is: %v", job.ID.Hex(), err)
		return err
	}

	log.Infof("registering jobID: %s with cron: %s", job.ID.Hex(), schedule)
	err = h.Scheduler.UpdateJobModel(job.ID.Hex(), scheduleJob)
	if err != nil {
		log.Errorf("Failed to register job of ID: %s to scheduler, the error is: %v", job.ID, err)
		return err
	}
	return nil
}

func (h *CronjobHandler) registerWorkFlowV4Job(name, schedule string, job *service.Schedule) error {
	if job.WorkflowV4Args == nil {
		return nil
	}
	scheduleJob, err := cronlib.NewJobModel(schedule, func() {
		if err := h.aslanCli.ScheduleCall(fmt.Sprintf("workflow/v4/workflowtask/trigger?triggerName=%s", setting.CronTaskCreator), job.WorkflowV4Args, log.SugaredLogger()); err != nil {
			log.Errorf("[%s]RunScheduledTask err: %v", name, err)
		}
	})
	if err != nil {
		log.Errorf("Failed to create job of ID: %s, the error is: %v", job.ID.Hex(), err)
		return err
	}

	log.Infof("registering jobID: %s with cron: %s", job.ID.Hex(), schedule)
	err = h.Scheduler.UpdateJobModel(job.ID.Hex(), scheduleJob)
	if err != nil {
		log.Errorf("Failed to register job of ID: %s to scheduler, the error is: %v", job.ID, err)
		return err
	}
	return nil
}

func (h *CronjobHandler) registerTestJob(name, productName, schedule string, job *service.Schedule) error {
	args := &service.TestTaskArgs{
		TestName:        name,
		ProductName:     productName,
		TestTaskCreator: setting.CronTaskCreator,
	}
	scheduleJob, err := cronlib.NewJobModel(schedule, func() {
		if err := h.aslanCli.ScheduleCall("testing/testtask", args, log.SugaredLogger()); err != nil {
			log.Errorf("[%s]RunScheduledTask err: %v", name, err)
		}
	})
	if err != nil {
		log.Errorf("Failed to create job of ID: %s, the error is: %v", job.ID.Hex(), err)
		return err
	}

	log.Infof("registering jobID: %s with cron: %s", job.ID.Hex(), schedule)
	err = h.Scheduler.UpdateJobModel(job.ID.Hex(), scheduleJob)
	if err != nil {
		log.Errorf("Failed to register job of ID: %s to scheduler, the error is: %v", job.ID, err)
		return err
	}
	return nil
}

// FIXME
// UNDER CURRENT SERVICE STRUCTURE, STOPPING CRONJOB SERVICE AND UPDATING DB RECORD
// ARE NOT ATOMIC, THIS WILL CAUSE SERIOUS PROBLEM IF UPDATE FAILED
func (h *CronjobHandler) stopCronjob(name, ptype string) error {
	var jobList []*service.Cronjob
	listAPI := fmt.Sprintf("%s/cron/cronjob/type/%s/name/%s", h.aslanCli.APIBase, ptype, name)
	header := http.Header{}
	resp, err := util.SendRequest(listAPI, "GET", header, nil)
	if err != nil {
		log.Errorf("Failed to get job list, the error is: %v, reconsuming", err)
		return err
	}
	err = json.Unmarshal(resp, &jobList)
	if err != nil {
		log.Errorf("Failed to unmarshal list cronjob response, the error is: %v", err)
		return err
	}

	for _, job := range jobList {
		log.Infof("stopping cronjob of ID: %s", job.ID)
		h.Scheduler.StopService(job.ID)
	}

	disableAPI := fmt.Sprintf("%s/cron/cronjob/disable", h.aslanCli.APIBase)
	req, err := json.Marshal(service.DisableCronjobReq{
		Name: name,
		Type: ptype,
	})
	if err != nil {
		log.Errorf("marshal json args error: %v", err)
		return err
	}
	_, err = util.SendRequest(disableAPI, "POST", header, req)
	if err != nil {
		log.Errorf("Failed to disable cron job of service name: %s, the error is: %v", name, err)
	}
	return nil
}

func registerCronjob(job *service.Cronjob, client *client.Client, scheduler *cronlib.CronSchduler) error {
	switch job.Type {
	case setting.WorkflowCronjob:
		args := &service.WorkflowTaskArgs{
			WorkflowName:       job.Name,
			WorklowTaskCreator: setting.CronTaskCreator,
		}
		if job.WorkflowArgs != nil {
			args.Description = job.WorkflowArgs.Description
			args.ProductTmplName = job.WorkflowArgs.ProductTmplName
			args.Target = job.WorkflowArgs.Target
			args.Namespace = job.WorkflowArgs.Namespace
			args.Tests = job.WorkflowArgs.Tests
			args.DistributeEnabled = job.WorkflowArgs.DistributeEnabled
		}
		var cron string
		if job.JobType == setting.CrontabCronjob {
			cron = fmt.Sprintf("%s%s", "0 ", job.Cron)
		} else {
			cron, _ = convertCronString(job.JobType, job.Time, job.Frequency, job.Number)
		}
		scheduleJob, err := cronlib.NewJobModel(cron, func() {
			if err := client.ScheduleCall(path.Join("workflow/workflowtask", job.WorkflowArgs.WorkflowName), args, log.SugaredLogger()); err != nil {
				log.Errorf("[%s]RunScheduledTask err: %v", job.Name, err)
			}
		})
		if err != nil {
			log.Errorf("Failed to generate job of ID: %s to scheduler, the error is: %v", job.ID, err)
			return err
		}
		log.Infof("registering jobID: %s with cron: %s", job.ID, cron)
		err = scheduler.UpdateJobModel(job.ID, scheduleJob)
		if err != nil {
			log.Errorf("Failed to register job of ID: %s to scheduler, the error is: %v", job.ID, err)
			return err
		}
	case setting.WorkflowV4Cronjob:
		if job.WorkflowV4Args == nil {
			return fmt.Errorf("workflow args is nil")
		}
		var cron string
		if job.JobType == setting.CrontabCronjob {
			cron = fmt.Sprintf("%s%s", "0 ", job.Cron)
		} else {
			cron, _ = convertCronString(job.JobType, job.Time, job.Frequency, job.Number)
		}
		scheduleJob, err := cronlib.NewJobModel(cron, func() {
			if err := client.ScheduleCall(fmt.Sprintf("workflow/v4/workflowtask/trigger?triggerName=%s", setting.CronTaskCreator), job.WorkflowV4Args, log.SugaredLogger()); err != nil {
				log.Errorf("[%s]RunScheduledTask err: %v", job.Name, err)
			}
		})
		if err != nil {
			log.Errorf("Failed to generate job of ID: %s to scheduler, the error is: %v", job.ID, err)
			return err
		}
		log.Infof("registering jobID: %s with cron: %s", job.ID, cron)
		err = scheduler.UpdateJobModel(job.ID, scheduleJob)
		if err != nil {
			log.Errorf("Failed to register job of ID: %s to scheduler, the error is: %v", job.ID, err)
			return err
		}
	case setting.TestingCronjob:
		args := &service.TestTaskArgs{
			TestName:        job.Name,
			ProductName:     job.ProductName,
			TestTaskCreator: setting.CronTaskCreator,
		}
		var cron string
		if job.JobType == setting.CrontabCronjob {
			cron = fmt.Sprintf("%s%s", "0 ", job.Cron)
		} else {
			cron, _ = convertCronString(job.JobType, job.Time, job.Frequency, job.Number)
		}
		scheduleJob, err := cronlib.NewJobModel(cron, func() {
			if err := client.ScheduleCall("testing/testtask", args, log.SugaredLogger()); err != nil {
				log.Errorf("[%s]RunScheduledTask err: %v", job.Name, err)
			}
		})
		if err != nil {
			log.Errorf("Failed to generate job of ID: %s to scheduler, the error is: %v", job.ID, err)
			return err
		}
		log.Infof("registering jobID: %s with cron: %s", job.ID, cron)
		err = scheduler.UpdateJobModel(job.ID, scheduleJob)
		if err != nil {
			log.Errorf("Failed to register job of ID: %s to scheduler, the error is: %v", job.ID, err)
			return err
		}
	case setting.EnvAnalysisCronjob:
		if job.EnvAnalysisArgs == nil {
			return nil
		}

		var cron string
		if job.JobType == "" || job.JobType == setting.CrontabCronjob {
			cron = fmt.Sprintf("%s%s", "0 ", job.Cron)
		} else {
			cron, _ = convertCronString(job.JobType, job.Time, job.Frequency, job.Number)
		}

		scheduleJob, err := cronlib.NewJobModel(cron, func() {
			base := "environment/environments/"
			production := "false"
			if job.EnvAnalysisArgs.Production {
				production = "true"
			}

			url := base + fmt.Sprintf("%s/analysis?projectName=%s&triggerName=%s&userName=%s&production=%s", job.EnvAnalysisArgs.EnvName, job.EnvAnalysisArgs.ProductName, setting.CronTaskCreator, setting.CronTaskCreator, production)

			if err := client.ScheduleCall(url, nil, log.SugaredLogger()); err != nil {
				log.Errorf("[%s]RunScheduledTask err: %v", job.Name, err)
			}
		})
		if err != nil {
			log.Errorf("Failed to create job of ID: %s, the error is: %v", job.ID, err)
			return err
		}

		log.Infof("registering jobID: %s with cron: %s", job.ID, cron)
		err = scheduler.UpdateJobModel(job.ID, scheduleJob)
		if err != nil {
			log.Errorf("Failed to register job of ID: %s to scheduler, the error is: %v", job.ID, err)
			return err
		}
	case setting.EnvSleepCronjob:
		if job.EnvArgs == nil {
			return nil
		}
		var cron string
		if job.JobType == "" || job.JobType == setting.CrontabCronjob {
			cron = fmt.Sprintf("%s%s", "0 ", job.Cron)
		} else {
			cron, _ = convertCronString(job.JobType, job.Time, job.Frequency, job.Number)
		}
		scheduleJob, err := cronlib.NewJobModel(cron, func() {
			base := "environment/environments/"
			production := "false"
			if job.EnvArgs.Production {
				production = "true"
			}

			url := ""
			if job.EnvArgs.Name == util.GetEnvSleepCronName(job.EnvArgs.ProductName, job.EnvArgs.EnvName, true) {
				url = base + fmt.Sprintf("%s/sleep?projectName=%s&action=enable&production=%s", job.EnvArgs.EnvName, job.EnvArgs.ProductName, production)
			} else if job.EnvArgs.Name == util.GetEnvSleepCronName(job.EnvArgs.ProductName, job.EnvArgs.EnvName, false) {
				url = base + fmt.Sprintf("%s/sleep?projectName=%s&action=disable&production=%s", job.EnvArgs.EnvName, job.EnvArgs.ProductName, production)
			}

			if err := client.ScheduleCall(url, nil, log.SugaredLogger()); err != nil {
				log.Errorf("[%s]RunScheduledTask err: %v", job.Name, err)
			}
		})
		if err != nil {
			log.Errorf("Failed to create job of ID: %s, the error is: %v", job.ID, err)
			return err
		}

		log.Infof("registering jobID: %s with cron: %s", job.ID, cron)
		err = scheduler.UpdateJobModel(job.ID, scheduleJob)
		if err != nil {
			log.Errorf("Failed to register job of ID: %s to scheduler, the error is: %v", job.ID, err)
			return err
		}
	case setting.ReleasePlanCronjob:
		var cron string
		if job.JobType == "" || job.JobType == setting.CrontabCronjob {
			cron = fmt.Sprintf("%s%s", "0 ", job.Cron)
		} else {
			cron, _ = convertCronString(job.JobType, job.Time, job.Frequency, job.Number)
		}

		if job.ReleasePlanArgs == nil {
			log.Errorf("ReleasePlanArgs is nil, name: %v, schedule: %v, jobID: %v", job.Name, cron, job.ID)
			return nil
		}
		scheduleJob, err := cronlib.NewJobModel(cron, func() {
			base := "release_plan/v1"
			url := base + fmt.Sprintf("/%s/schedule_execute", job.ReleasePlanArgs.ID)
			if err := client.ScheduleCall(url, nil, log.SugaredLogger()); err != nil {
				log.Errorf("[%s]RunScheduledTask err: %v", job.Name, err)
			}

			scheduler.StopService(job.ID)

			log.Infof("schedule executed release plan, jobID: %v, cron: %v; release plan ID: %v, index: %v, name: %v", job.ID, job.Cron, job.ReleasePlanArgs.ID, job.ReleasePlanArgs.Index, job.ReleasePlanArgs.Name)
		})
		if err != nil {
			log.Errorf("Failed to create jobID: %s, jobName: %v, cron: %v; release plan ID: %v, index: %v, name: %v, error: %v", job.ID, job.Name, cron, job.ReleasePlanArgs.ID, job.ReleasePlanArgs.Index, job.ReleasePlanArgs.Name, err)
			return err
		}

		log.Infof("registering jobID: %s with name: %v, cron: %v; release plan ID: %v, index: %v, name: %v", job.ID, job.Name, cron, job.ReleasePlanArgs.ID, job.ReleasePlanArgs.Index, job.ReleasePlanArgs.Name)
		err = scheduler.UpdateJobModel(job.ID, scheduleJob)
		if err != nil {
			log.Errorf("Failed to register job of ID: %s to scheduler, the error is: %v", job.ID, err)
			return err
		}
	default:
		fmt.Printf("Not supported type of service: %s\n", job.Type)
		return errors.New("not supported service type")
	}
	return nil
}

func (h *CronjobHandler) registerEnvAnalysisJob(name, schedule string, job *service.Schedule) error {
	if job.EnvAnalysisArgs == nil {
		return nil
	}
	scheduleJob, err := cronlib.NewJobModel(schedule, func() {
		base := "environment/environments/"
		production := "false"
		if job.EnvAnalysisArgs.Production {
			production = "true"
		}

		url := base + fmt.Sprintf("%s/analysis?projectName=%s&triggerName=%s&userName=%s&production=%s", job.EnvAnalysisArgs.EnvName, job.EnvAnalysisArgs.ProductName, setting.CronTaskCreator, setting.CronTaskCreator, production)

		if err := h.aslanCli.ScheduleCall(url, nil, log.SugaredLogger()); err != nil {
			log.Errorf("[%s]RunScheduledTask err: %v", name, err)
		}
	})
	if err != nil {
		log.Errorf("Failed to create job of ID: %s, the error is: %v", job.ID.Hex(), err)
		return err
	}

	log.Infof("registering jobID: %s with cron: %s", job.ID.Hex(), schedule)
	err = h.Scheduler.UpdateJobModel(job.ID.Hex(), scheduleJob)
	if err != nil {
		log.Errorf("Failed to register job of ID: %s to scheduler, the error is: %v", job.ID, err)
		return err
	}
	return nil
}

func (h *CronjobHandler) registerEnvSleepJob(name, schedule string, job *service.Schedule) error {
	if job.EnvArgs == nil {
		return nil
	}
	scheduleJob, err := cronlib.NewJobModel(schedule, func() {
		base := "environment/environments/"
		production := "false"
		if job.EnvArgs.Production {
			production = "true"
		}

		url := ""
		if job.EnvArgs.Name == util.GetEnvSleepCronName(job.EnvArgs.ProductName, job.EnvArgs.EnvName, true) {
			url = base + fmt.Sprintf("%s/sleep?projectName=%s&action=enable&production=%s", job.EnvArgs.EnvName, job.EnvArgs.ProductName, production)
		} else if job.EnvArgs.Name == util.GetEnvSleepCronName(job.EnvArgs.ProductName, job.EnvArgs.EnvName, false) {
			url = base + fmt.Sprintf("%s/sleep?projectName=%s&action=disable&production=%s", job.EnvArgs.EnvName, job.EnvArgs.ProductName, production)
		}

		if err := h.aslanCli.ScheduleCall(url, nil, log.SugaredLogger()); err != nil {
			log.Errorf("[%s]RunScheduledTask err: %v", name, err)
		}
	})
	if err != nil {
		log.Errorf("Failed to create job of ID: %s, the error is: %v", job.ID.Hex(), err)
		return err
	}

	log.Infof("registering jobID: %s with cron: %s", job.ID.Hex(), schedule)
	err = h.Scheduler.UpdateJobModel(job.ID.Hex(), scheduleJob)
	if err != nil {
		log.Errorf("Failed to register job of ID: %s to scheduler, the error is: %v", job.ID, err)
		return err
	}
	return nil
}

func (h *CronjobHandler) registerReleasePlanJob(name, schedule string, job *service.Schedule) error {
	if job.ReleasePlanArgs == nil {
		log.Errorf("ReleasePlanArgs is nil, name: %v, schedule: %v, jobID: %v", name, schedule, job.ID.Hex())
		return nil
	}
	scheduleJob, err := cronlib.NewJobModel(schedule, func() {
		base := "release_plan/v1"
		url := base + fmt.Sprintf("/%s/schedule_execute", job.ReleasePlanArgs.ID)
		if err := h.aslanCli.ScheduleCall(url, nil, log.SugaredLogger()); err != nil {
			log.Errorf("[%s]RunScheduledTask err: %v", name, err)
		}

		h.Scheduler.StopService(job.ID.Hex())

		log.Infof("schedule executed release plan, jobID: %v, cron: %v; release plan ID: %v, index: %v, name: %v", job.ID.Hex(), job.Cron, job.ReleasePlanArgs.ID, job.ReleasePlanArgs.Index, job.ReleasePlanArgs.Name)
	})
	if err != nil {
		log.Errorf("Failed to create jobID: %s, jobName: %v, cron: %v; release plan ID: %v, index: %v, name: %v, error: %v", job.ID.Hex(), name, schedule, job.ReleasePlanArgs.ID, job.ReleasePlanArgs.Index, job.ReleasePlanArgs.Name, err)
		return err
	}

	log.Infof("registering jobID: %s with name: %v, cron: %v; release plan ID: %v, index: %v, name: %v", job.ID.Hex(), name, schedule, job.ReleasePlanArgs.ID, job.ReleasePlanArgs.Index, job.ReleasePlanArgs.Name)
	err = h.Scheduler.UpdateJobModel(job.ID.Hex(), scheduleJob)
	if err != nil {
		log.Errorf("Failed to register job of ID: %s to scheduler, the error is: %v", job.ID, err)
		return err
	}
	return nil
}
