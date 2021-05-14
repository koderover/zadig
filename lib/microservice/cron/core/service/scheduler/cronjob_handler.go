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
	"strings"
	"time"

	"github.com/nsqio/go-nsq"
	"github.com/rfyiamcool/cronlib"

	"github.com/koderover/zadig/lib/microservice/cron/core/service"
	"github.com/koderover/zadig/lib/microservice/cron/core/service/client"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/util"
)

var (
	xl *xlog.Logger
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
	xl = xlog.NewDummy()
	InitExistedCronjob(client, scheduler)

	return &CronjobHandler{
		aslanCli:  client,
		Scheduler: scheduler,
	}
}

func InitExistedCronjob(client *client.Client, scheduler *cronlib.CronSchduler) {
	xl.Infof("Initializing existing cronjob ....")
	initChan := make(chan []*service.Cronjob, 1)
	failsafeChan := make(chan []*service.Cronjob, 1)
	var (
		jobList         []*service.Cronjob
		failsafeJobList []*service.Cronjob
		resp            []byte
		err             error
	)
	listAPI := fmt.Sprintf("%s/cron/cronjob", client.ApiBase)
	header := http.Header{}
	header.Set("Authorization", fmt.Sprintf("%s %s", setting.TIMERAPIKEY, client.Token))
	// failsafe function: get enabled workflow and register them
	failsafeAPI := fmt.Sprintf("%s/cron/cronjob/failsafe", client.ApiBase)
	go func() {
		for {
			resp, err = util.SendRequest(listAPI, "GET", header, nil)
			if err != nil {
				time.Sleep(PullInterval)
				continue
			}
			err = json.Unmarshal(resp, &jobList)
			if err == nil {
				initChan <- jobList
				return
			}
			time.Sleep(PullInterval)
		}
	}()

	go func() {
		for {
			resp, err = util.SendRequest(failsafeAPI, "GET", header, nil)
			if err != nil {
				time.Sleep(PullInterval)
				continue
			}
			err = json.Unmarshal(resp, &failsafeJobList)
			if err == nil {
				failsafeChan <- failsafeJobList
				return
			}
			time.Sleep(PullInterval)
		}
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
		xl.Fatalf("Failed to get aslan response after 5min, exiting...")
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
		xl.Fatalf("Failed to get aslan response after 5min, exiting...")
	}

	xl.Infof("cronjob initialization complete ...")
}

// HandleMessage ...
func (h *CronjobHandler) HandleMessage(message *nsq.Message) error {
	fmt.Printf("MESSAGE IN: %s, ATTEMPT: %d\n", message.Body, message.Attempts)

	var msg *service.CronjobPayload
	if err := json.Unmarshal(message.Body, &msg); err != nil {
		xl.Errorf("unmarshal CancelMessage error: %v", err)
		return nil
	}

	switch msg.Action {
	case setting.TypeEnableCronjob:
		err := h.updateCronjob(msg.Name, msg.ProductName, msg.JobType, msg.JobList, msg.DeleteList)
		if err != nil {
			xl.Errorf("Failed to update cronjob, the error is: %v", err)
			return err
		}
	case setting.TypeDisableCronjob:
		err := h.stopCronjob(msg.Name, msg.JobType)
		if err != nil {
			xl.Errorf("Failed to stop all cron job, the error is: %v", err)
			return err
		}
	default:
		xl.Errorf("unsupported cronjob action: NOT RECONSUMING")
	}

	return nil
}

func (h *CronjobHandler) updateCronjob(name, productName, jobType string, jobList []*service.Schedule, deleteList []string) error {
	//首先根据deleteList停止不需要的cronjob
	for _, deleteId := range deleteList {
		jobId := deleteId
		xl.Infof("stopping Job of ID: %s", jobId)
		h.Scheduler.StopService(jobId)
	}
	// 根据job内容来在scheduler中新增cronjob
	for _, job := range jobList {
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
		default:
			xl.Errorf("unrecognized cron job type for job id: %s", job.ID)
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
			xl.Errorf("Failed to format the time string")
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
		ReqID:              xl.ReqID(),
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
		if err := h.aslanCli.ScheduleCall("workflow/workflowtask", args, xl); err != nil {
			xl.Errorf("[%s]RunScheduledTask err: %v", name, err)
		}
	})
	if err != nil {
		xl.Errorf("Failed to create job of ID: %s, the error is: %v", job.ID.Hex(), err)
		return err
	}

	xl.Infof("registering jobID: %s with cron: %s", job.ID.Hex(), schedule)
	err = h.Scheduler.UpdateJobModel(job.ID.Hex(), scheduleJob)
	if err != nil {
		xl.Errorf("Failed to register job of ID: %s to scheduler, the error is: %v", job.ID, err)
		return err
	}
	return nil
}

func (h *CronjobHandler) registerTestJob(name, productName, schedule string, job *service.Schedule) error {
	args := &service.TestTaskArgs{
		TestName:        name,
		ProductName:     productName,
		TestTaskCreator: setting.CronTaskCreator,
		ReqID:           xl.ReqID(),
	}
	scheduleJob, err := cronlib.NewJobModel(schedule, func() {
		if err := h.aslanCli.ScheduleCall("testing/testtask", args, xl); err != nil {
			xl.Errorf("[%s]RunScheduledTask err: %v", name, err)
		}
	})
	if err != nil {
		xl.Errorf("Failed to create job of ID: %s, the error is: %v", job.ID.Hex(), err)
		return err
	}

	xl.Infof("registering jobID: %s with cron: %s", job.ID.Hex(), schedule)
	err = h.Scheduler.UpdateJobModel(job.ID.Hex(), scheduleJob)
	if err != nil {
		xl.Errorf("Failed to register job of ID: %s to scheduler, the error is: %v", job.ID, err)
		return err
	}
	return nil
}

// FIXME
// UNDER CURRENT SERVICE STRUCTURE, STOPPING CRONJOB SERVICE AND UPDATING DB RECORD
// ARE NOT ATOMIC, THIS WILL CAUSE SERIOUS PROBLEM IF UPDATE FAILED
func (h *CronjobHandler) stopCronjob(name, ptype string) error {
	var jobList []*service.Cronjob
	listAPI := fmt.Sprintf("%s/cron/cronjob/type/%s/name/%s", h.aslanCli.ApiBase, ptype, name)
	header := http.Header{}
	header.Set("Authorization", fmt.Sprintf("%s %s", setting.TIMERAPIKEY, h.aslanCli.Token))
	resp, err := util.SendRequest(listAPI, "GET", header, nil)
	if err != nil {
		xl.Errorf("Failed to get job list, the error is: %v, reconsuming", err)
		return err
	}
	err = json.Unmarshal(resp, &jobList)
	if err != nil {
		xl.Errorf("Failed to unmarshal list cronjob response, the error is: %v", err)
		return err
	}

	for _, job := range jobList {
		xl.Infof("stopping cronjob of ID:", job.ID)
		h.Scheduler.StopService(job.ID)
	}

	disableAPI := fmt.Sprintf("%s/cron/cronjob/disable", h.aslanCli.ApiBase)
	req, err := json.Marshal(service.DisableCronjobReq{
		Name: name,
		Type: ptype,
	})
	if err != nil {
		xl.Errorf("marshal json args error: %v", err)
		return err
	}
	_, err = util.SendRequest(disableAPI, "POST", header, req)
	if err != nil {
		xl.Errorf("Failed to disable cron job of service name: %s, the error is: %v", name, err)
	}
	return nil
}

func registerCronjob(job *service.Cronjob, client *client.Client, scheduler *cronlib.CronSchduler) error {
	switch job.Type {
	case setting.WorkflowCronjob:
		args := &service.WorkflowTaskArgs{
			WorkflowName:       job.Name,
			WorklowTaskCreator: setting.CronTaskCreator,
			ReqID:              xl.ReqID(),
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
			if err := client.ScheduleCall("workflow/workflowtask", args, xl); err != nil {
				xl.Errorf("[%s]RunScheduledTask err: %v", job.Name, err)
			}
		})
		if err != nil {
			xl.Errorf("Failed to generate job of ID: %s to scheduler, the error is: %v", job.ID, err)
			return err
		}
		xl.Infof("registering jobID: %s with cron: %s", job.ID, cron)
		err = scheduler.UpdateJobModel(job.ID, scheduleJob)
		if err != nil {
			xl.Errorf("Failed to register job of ID: %s to scheduler, the error is: %v", job.ID, err)
			return err
		}
	case setting.TestingCronjob:
		args := &service.TestTaskArgs{
			TestName:        job.Name,
			ProductName:     job.ProductName,
			TestTaskCreator: setting.CronTaskCreator,
			ReqID:           xl.ReqID(),
		}
		var cron string
		if job.JobType == setting.CrontabCronjob {
			cron = fmt.Sprintf("%s%s", "0 ", job.Cron)
		} else {
			cron, _ = convertCronString(job.JobType, job.Time, job.Frequency, job.Number)
		}
		scheduleJob, err := cronlib.NewJobModel(cron, func() {
			if err := client.ScheduleCall("testing/testtask", args, xl); err != nil {
				xl.Errorf("[%s]RunScheduledTask err: %v", job.Name, err)
			}
		})
		if err != nil {
			xl.Errorf("Failed to generate job of ID: %s to scheduler, the error is: %v", job.ID, err)
			return err
		}
		xl.Infof("registering jobID: %s with cron: %s", job.ID, cron)
		err = scheduler.UpdateJobModel(job.ID, scheduleJob)
		if err != nil {
			xl.Errorf("Failed to register job of ID: %s to scheduler, the error is: %v", job.ID, err)
			return err
		}
	default:
		fmt.Printf("Not supported type of service: %s\n", job.Type)
		return errors.New("not supported service type")
	}
	return nil
}
