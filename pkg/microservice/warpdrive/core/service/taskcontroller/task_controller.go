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

package taskcontroller

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/klock"
	"github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type controller struct {
}

func NewController() ControllerI {
	return &controller{}
}

func (c *controller) Init(ctx context.Context) error {
	go func() {
		if err := client.Start(ctx); err != nil {
			panic(err)
		}
	}()

	time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
	_ = klock.Init(config.WarpDriveNamespace())
	log.Debugf("init lock successfully, ns: %s", config.WarpDriveNamespace())

	initMongoDB()

	// handle pipeline task
	go func() {
		for {
			time.Sleep(2 * time.Second)
			select {
			case <-ctx.Done():
				return
			default:
				func() {
					klock.Lock(config.ProcessLock)
					defer func() {
						if err := klock.UnlockWithRetry(config.ProcessLock, 3); err != nil {
							log.Errorf("unlock process lock error: %v", err)
						}
					}()
					resp, err := mongodb.NewMsgQueuePipelineTaskColl().List(&mongodb.ListMsgQueuePipelineTaskOption{
						QueueType: setting.TopicProcess,
					})
					if err != nil {
						log.Warnf("list queue error: %v", err)
						return
					}
					if len(resp) == 0 {
						return
					}
					if err = mongodb.NewMsgQueuePipelineTaskColl().Delete(resp[0].ID); err != nil {
						log.Errorf("delete queue error: %v", err)
					}
					if err := commonmodels.IToi(resp[0].Task, pipelineTask); err != nil {
						log.Errorf("convert interface to struct error: %v", err)
						return
					}
					// debug
					fmt.Printf("nil: %v", pipelineTask == nil)
					log.Infof("receiving pipeline task %s:%d message", pipelineTask.PipelineName, pipelineTask.TaskID)
					h := &ExecHandler{
						AckID: 0,
					}
					h.PipelineTaskHandler()
				}()

			}
		}
	}()

	// handle cancel pipeline task
	go func() {
		cancelMsgMap := sets.NewString()
		for {
			time.Sleep(1 * time.Second)
			select {
			case <-ctx.Done():
				return
			default:
				func() {
					resp, err := mongodb.NewMsgQueueCommonColl().List(&mongodb.ListMsgQueueCommonOption{
						QueueType: setting.TopicCancel,
					})
					if err != nil {
						log.Warnf("list cancel queue error: %v", err)
						return
					}
					if len(resp) == 0 {
						return
					}
					for _, common := range resp {
						msg := new(CancelMessage)
						if err := json.Unmarshal([]byte(common.Payload), msg); err != nil {
							log.Errorf("convert interface to struct error: %v", err)
							return
						}
						if cancelMsgMap.Has(fmt.Sprintf("%s:%d", msg.PipelineName, msg.TaskID)) {
							continue
						}
						log.Infof("receiving cancel task %s:%d message", msg.PipelineName, msg.TaskID)
						cancelMsgMap.Insert(fmt.Sprintf("%s:%d", msg.PipelineName, msg.TaskID))

						// 如果存在处理的 PipelineTask 并且匹配 PipelineName, 则取消PipelineTask
						if pipelineTask != nil && pipelineTask.PipelineName == msg.PipelineName && pipelineTask.TaskID == msg.TaskID {
							log.Infof("cancelling message: %+v", msg)
							pipelineTask.TaskRevoker = msg.Revoker

							//取消pipelineTask
							cancel()
							if err = mongodb.NewMsgQueueCommonColl().Delete(resp[0].ID); err != nil {
								log.Errorf("delete cancel queue error: %v", err)
							}
							break
						}
					}
				}()
			}
		}
	}()
	//cfg := nsq.NewConfig()
	//cfg.UserAgent = config.WarpDrivePodName()
	//cfg.MaxAttempts = 50
	//cfg.LookupdPollInterval = 1 * time.Second
	//cfg.MsgTimeout = 1 * time.Minute
	//nsqClient := nsqcli.NewNsqClient(config.NSQLookupAddrs(), "127.0.0.1:4151")
	//
	//processor, err := nsq.NewConsumer(setting.TopicProcess, "process", cfg)
	//if err != nil {
	//	return fmt.Errorf("init nsq processor error: %v", err)
	//}
	//
	//processor.SetLogger(log.New(os.Stdout, "nsq consumer:", 0), nsq.LogLevelError)
	//c.consumers = append(c.consumers, processor)

	//canceller, err := nsq.NewConsumer(setting.TopicCancel, cfg.UserAgent, cfg)
	//if err != nil {
	//	return fmt.Errorf("init nsq canceller error: %v", err)
	//}
	//
	//canceller.SetLogger(log.New(os.Stdout, "nsq consumer:", 0), nsq.LogLevelError)
	//c.consumers = append(c.consumers, canceller)
	//
	//nsqdAddr := "127.0.0.1:4150"
	//sender, err := nsq.NewProducer(nsqdAddr, cfg)
	//if err != nil {
	//	return fmt.Errorf("init nsq sender error: %v", err)
	//}
	//
	//sender.SetLogger(log.New(os.Stdout, "nsq producer:", 0), nsq.LogLevelError)
	//c.producers = append(c.producers, sender)
	//
	//err = nsqClient.EnsureNsqdTopics([]string{setting.TopicAck, setting.TopicItReport, setting.TopicNotification})
	//if err != nil {
	//	return fmt.Errorf("ensure nsq topic error: %v", err)
	//}
	//

	//processor.AddHandler(execHandler)
	// Add task plugin initiators to exec Handler.
	//initTaskPlugins(execHandler)
	//
	//canceller.AddHandler(cancelHandler)
	//
	//if err := processor.ConnectToNSQLookupds(config.NSQLookupAddrs()); err != nil {
	//	return fmt.Errorf("processor could not connect to %v", config.NSQLookupAddrs())
	//}
	//if err := canceller.ConnectToNSQLookupds(config.NSQLookupAddrs()); err != nil {
	//	return fmt.Errorf("canceller could not connect to %v", config.NSQLookupAddrs())
	//}

	return nil
}

func (c *controller) Stop(ctx context.Context) error {
	return nil
}

func initMongoDB() {
	mongotool.Init(ctx, config.MongoURI())
	if err := mongotool.Ping(ctx); err != nil {
		panic(fmt.Errorf("failed to connect to mongo, error: %s", err))
	}
}

//func ConvertQueueToTask(queueTask *commonmodels.Queue) *task.Task {
//	return &task.Task{
//		TaskID:                  queueTask.TaskID,
//		ProductName:             queueTask.ProductName,
//		PipelineName:            queueTask.PipelineName,
//		PipelineDisplayName:     queueTask.PipelineDisplayName,
//		Type:                    queueTask.Type,
//		Status:                  queueTask.Status,
//		Description:             queueTask.Description,
//		TaskCreator:             queueTask.TaskCreator,
//		TaskRevoker:             queueTask.TaskRevoker,
//		CreateTime:              queueTask.CreateTime,
//		StartTime:               queueTask.StartTime,
//		EndTime:                 queueTask.EndTime,
//		SubTasks:                queueTask.SubTasks,
//		Stages:                  queueTask.Stages,
//		ReqID:                   queueTask.ReqID,
//		AgentHost:               queueTask.AgentHost,
//		DockerHost:              queueTask.DockerHost,
//		TeamName:                queueTask.TeamName,
//		IsDeleted:               queueTask.IsDeleted,
//		IsArchived:              queueTask.IsArchived,
//		AgentID:                 queueTask.AgentID,
//		MultiRun:                queueTask.MultiRun,
//		Target:                  queueTask.Target,
//		BuildModuleVer:          queueTask.BuildModuleVer,
//		ServiceName:             queueTask.ServiceName,
//		TaskArgs:                queueTask.TaskArgs,
//		WorkflowArgs:            queueTask.WorkflowArgs,
//		TestArgs:                queueTask.TestArgs,
//		ServiceTaskArgs:         queueTask.ServiceTaskArgs,
//		ArtifactPackageTaskArgs: queueTask.ArtifactPackageTaskArgs,
//		ConfigPayload:           queueTask.ConfigPayload,
//		Error:                   queueTask.Error,
//		Services:                queueTask.Services,
//		Render:                  queueTask.Render,
//		StorageURI:              queueTask.StorageURI,
//		TestReports:             queueTask.TestReports,
//		RwLock:                  queueTask.RwLock,
//		ResetImage:              queueTask.ResetImage,
//		ResetImagePolicy:        queueTask.ResetImagePolicy,
//		TriggerBy:               queueTask.TriggerBy,
//		Features:                queueTask.Features,
//		IsRestart:               queueTask.IsRestart,
//		StorageEndpoint:         queueTask.StorageEndpoint,
//	}
//}
