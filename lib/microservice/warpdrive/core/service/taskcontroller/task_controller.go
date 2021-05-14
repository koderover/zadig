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
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nsqio/go-nsq"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/kube/client"
	"github.com/koderover/zadig/lib/tool/nsqcli"
)

func InitTaskController(ctx context.Context) error {
	go func() {
		if err := client.Start(ctx); err != nil {
			panic(err)
		}
	}()

	//初始化nsq
	cfg := nsq.NewConfig()
	// 注意 WD_POD_NAME 必须使用 Downward API 配置环境变量
	cfg.UserAgent = config.WarpDrivePodName()
	// FIXME: Min: FIX MAGIC NUMBER
	cfg.MaxAttempts = 50
	cfg.LookupdPollInterval = 1 * time.Second
	//nsqd 和服务起在一起 FIXME: Min: FIX MAGIC ADDR
	nsqClient := nsqcli.NewNsqClient(config.NSQLookupAddrs(), "127.0.0.1:4151")

	//Process topic
	processor, err := nsq.NewConsumer(setting.TopicProcess, "process", cfg)
	if err != nil {
		return fmt.Errorf("init nsq processor error: %v", err)
	}
	processor.SetLogger(log.New(os.Stdout, "nsq consumer:", 0), nsq.LogLevelError)

	//Cancel topic
	//监听不同的channel,确保取消消息到每一个wd
	canceller, err := nsq.NewConsumer(setting.TopicCancel, cfg.UserAgent, cfg)
	if err != nil {
		return fmt.Errorf("init nsq canceller error: %v", err)
	}
	canceller.SetLogger(log.New(os.Stdout, "nsq consumer:", 0), nsq.LogLevelError)

	//Sender
	nsqdAddr := "127.0.0.1:4150"
	sender, err := nsq.NewProducer(nsqdAddr, cfg)
	if err != nil {
		return fmt.Errorf("init nsq sender error: %v", err)
	}
	sender.SetLogger(log.New(os.Stdout, "nsq producer:", 0), nsq.LogLevelError)

	// 初始化nsq topic
	err = nsqClient.EnsureNsqdTopics([]string{setting.TopicAck, setting.TopicItReport, setting.TopicNotification})
	if err != nil {
		return fmt.Errorf("ensure nsq topic error: %v", err)
	}

	execHandler := &ExecHandler{
		Sender: sender,
	}

	processor.AddHandler(execHandler)

	//Add task plugin initiators to exec Handler
	initTaskPlugins(execHandler)

	cancelHandler := &CancelHandler{}

	canceller.AddHandler(cancelHandler)

	if err := processor.ConnectToNSQLookupds(config.NSQLookupAddrs()); err != nil {
		return fmt.Errorf("processor could not connect to %v", config.NSQLookupAddrs())
	}

	if err := canceller.ConnectToNSQLookupds(config.NSQLookupAddrs()); err != nil {
		return fmt.Errorf("canceller could not connect to %v", config.NSQLookupAddrs())
	}
	return nil
}
