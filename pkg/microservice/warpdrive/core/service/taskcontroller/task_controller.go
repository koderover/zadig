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
	"sync"
	"time"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/klock"
	"github.com/koderover/zadig/v2/pkg/tool/kube/client"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

// filter duplicate cancel message
var (
	cancelMsgMap     = make(map[string]string)
	cancelMsgMapLock = sync.RWMutex{}
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
	initMongoDB()

	time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
	_ = klock.Init(config.WarpDriveNamespace())
	log.Debugf("init lock successfully, ns: %s", config.WarpDriveNamespace())

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
					resp, err := mongodb.NewMsgQueuePipelineTaskColl().List(&mongodb.ListMsgQueuePipelineTaskOption{
						QueueType: setting.TopicProcess,
					})
					// unlock process lock at first
					if err := klock.UnlockWithRetry(config.ProcessLock, 3); err != nil {
						log.Errorf("unlock process lock error: %v", err)
					}
					// check list error
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
					if err := commonmodels.IToi(resp[0].Task, &pipelineTask); err != nil {
						log.Errorf("convert interface to struct error: %v", err)
						return
					}
					log.Infof("receiving pipeline task %s:%d message", pipelineTask.PipelineName, pipelineTask.TaskID)
					h := &ExecHandler{
						AckID: 0,
					}
					initTaskPlugins(h)
					h.PipelineTaskHandler()
				}()

			}
		}
	}()

	// handle cancel pipeline task message
	go func() {
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
						if _, ok := cancelMsgMap[fmt.Sprintf("%s:%d", msg.PipelineName, msg.TaskID)]; ok {
							continue
						}
						log.Infof("receiving cancel task %s:%d message", msg.PipelineName, msg.TaskID)
						cancelMsgMapLock.Lock()
						cancelMsgMap[fmt.Sprintf("%s:%d", msg.PipelineName, msg.TaskID)] = msg.Revoker
						cancelMsgMapLock.Unlock()

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

	// cancel pipeline task
	// blocked task maybe sent to warpdrive after receving cancel msg
	go func() {
		for {
			time.Sleep(1 * time.Second)
			select {
			case <-ctx.Done():
				return
			default:
				cancelMsgMapLock.RLock()
				if pipelineTask != nil {
					if revoker, ok := cancelMsgMap[fmt.Sprintf("%s:%d", pipelineTask.PipelineName, pipelineTask.TaskID)]; ok {
						log.Infof("cancel task %s %d", pipelineTask.PipelineName, pipelineTask.TaskID)
						pipelineTask.TaskRevoker = revoker
						//取消pipelineTask
						cancel()
					}
				}
				cancelMsgMapLock.RUnlock()
			}
		}
	}()

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
