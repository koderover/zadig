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

	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/nsqcli"
)

type controller struct {
	consumers []*nsq.Consumer
	producers []*nsq.Producer
}

func NewController() ControllerI {
	return &controller{
		consumers: []*nsq.Consumer{},
		producers: []*nsq.Producer{},
	}
}

func (c *controller) Init(ctx context.Context) error {
	go func() {
		if err := client.Start(ctx); err != nil {
			panic(err)
		}
	}()

	cfg := nsq.NewConfig()
	cfg.UserAgent = config.WarpDrivePodName()
	cfg.MaxAttempts = 50
	cfg.LookupdPollInterval = 1 * time.Second
	cfg.MsgTimeout = 1 * time.Minute
	nsqClient := nsqcli.NewNsqClient(config.NSQLookupAddrs(), "127.0.0.1:4151")

	processor, err := nsq.NewConsumer(setting.TopicProcess, "process", cfg)
	if err != nil {
		return fmt.Errorf("init nsq processor error: %v", err)
	}

	processor.SetLogger(log.New(os.Stdout, "nsq consumer:", 0), nsq.LogLevelError)
	c.consumers = append(c.consumers, processor)

	canceller, err := nsq.NewConsumer(setting.TopicCancel, cfg.UserAgent, cfg)
	if err != nil {
		return fmt.Errorf("init nsq canceller error: %v", err)
	}

	canceller.SetLogger(log.New(os.Stdout, "nsq consumer:", 0), nsq.LogLevelError)
	c.consumers = append(c.consumers, canceller)

	nsqdAddr := "127.0.0.1:4150"
	sender, err := nsq.NewProducer(nsqdAddr, cfg)
	if err != nil {
		return fmt.Errorf("init nsq sender error: %v", err)
	}

	sender.SetLogger(log.New(os.Stdout, "nsq producer:", 0), nsq.LogLevelError)
	c.producers = append(c.producers, sender)

	err = nsqClient.EnsureNsqdTopics([]string{setting.TopicAck, setting.TopicItReport, setting.TopicNotification})
	if err != nil {
		return fmt.Errorf("ensure nsq topic error: %v", err)
	}

	execHandler := &ExecHandler{
		Sender: sender,
	}
	processor.AddHandler(execHandler)
	// Add task plugin initiators to exec Handler.
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

func (c *controller) Stop(ctx context.Context) error {
	for _, consumer := range c.consumers {
		consumer.Stop()
	}

	for _, producer := range c.producers {
		producer.Stop()
	}

	return nil
}
