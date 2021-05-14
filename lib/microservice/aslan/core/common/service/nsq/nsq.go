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

package nsq

import (
	"log"
	"os"
	"time"

	"github.com/nsqio/go-nsq"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	internallog "github.com/koderover/zadig/lib/microservice/aslan/internal/log"
	"github.com/koderover/zadig/lib/tool/nsqcli"
)

type agent struct {
	sender        *nsq.Producer
	defaultConfig *nsq.Config
	addresses     []string
}

var defaultAgent *agent

func Init(agentID string, addresses []string) {
	cfg := nsq.NewConfig()
	cfg.MaxAttempts = 10
	cfg.LookupdPollInterval = 1 * time.Second
	cfg.UserAgent = agentID
	// nsqd 和服务起在一起
	nsqdAddr := "127.0.0.1:4150"
	nsqClient := nsqcli.NewNsqClient(addresses, "127.0.0.1:4151")
	sender, err := nsq.NewProducer(nsqdAddr, cfg)
	if err != nil {
		log.Fatalf("Failed to init producer for nsq service")
	}
	sender.SetLogger(log.New(os.Stdout, "nsq producer:", 0), nsq.LogLevelError)
	err = nsqClient.EnsureNsqdTopics([]string{config.TopicCronjob, config.TopicCancel, config.TopicProcess})
	if err != nil {
		log.Fatalf("cannot ensure cronjob topic in nsq")
	}

	defaultAgent = &agent{
		sender:        sender,
		defaultConfig: cfg,
		addresses:     addresses,
	}
}

func (s *agent) Publish(topic string, payload []byte) error {
	return s.sender.Publish(topic, payload)
}

func (s *agent) SubScribeSimple(topic, channel string, handler nsq.Handler) error {
	return s.SubScribe(topic, channel, 1, s.defaultConfig, handler)
}

func (s *agent) SubScribe(topic, channel string, concurrency int, cfg *nsq.Config, handler nsq.Handler) error {
	consumer, err := nsq.NewConsumer(topic, channel, cfg)
	if err != nil {
		return err
	}
	consumer.AddConcurrentHandlers(handler, concurrency)
	if err := consumer.ConnectToNSQLookupds(s.addresses); err != nil {
		return err
	}
	consumer.SetLogger(log.New(os.Stdout, "nsq consumer:", 0), nsq.LogLevelError)
	internallog.Infof("Successfully subscribe to topic: %s with channel: %s", topic, channel)
	return nil
}

func Config() *nsq.Config {
	if defaultAgent == nil {
		panic("nsq service is not initialized yet")
	}

	return defaultAgent.defaultConfig
}

func Publish(topic string, payload []byte) error {
	if defaultAgent == nil {
		panic("nsq service is not initialized yet")
	}

	return defaultAgent.Publish(topic, payload)
}

func SubScribeSimple(topic, channel string, handler nsq.Handler) error {
	if defaultAgent == nil {
		panic("nsq service is not initialized yet")
	}

	return defaultAgent.SubScribeSimple(topic, channel, handler)
}

func SubScribe(topic, channel string, concurrency int, cfg *nsq.Config, handler nsq.Handler) error {
	if defaultAgent == nil {
		panic("nsq service is not initialized yet")
	}

	return defaultAgent.SubScribe(topic, channel, concurrency, cfg, handler)
}
