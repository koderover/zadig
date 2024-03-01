/*
Copyright 2023 The KodeRover Authors.

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

package register

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"

	agentconfig "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/network"
	fileutil "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/file"
	osutil "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/os"
)

func RegisterAgent() {
	agentConfig := agentconfig.NewAgentConfig()
	args := &network.RegisterAgentParameters{}

	parameters, err := osutil.GetPlatformParameters()
	if err != nil {
		panic(fmt.Errorf("failed to get platform parameters: %v", err))
	}
	err = osutil.IToi(parameters, args)
	if err != nil {
		panic(fmt.Errorf("failed to convert platform parameters to register agent parameters: %v", err))
	}

	args.AgentVersion = agentconfig.BuildAgentVersion
	if agentConfig.Token == "" {
		log.Panicf("agent token is empty, please use zadig-agent --token=xxx to start agent.")
	}
	if agentConfig.ServerURL == "" {
		log.Panicf("zadig-server url is empty, please use zadig-agent --server-url=xxx to start agent.")
	}
	if agentConfig.WorkDirectory != "" {
		args.WorkDir = agentConfig.WorkDirectory
	}

	config := &network.AgentConfig{
		URL:   agentConfig.ServerURL,
		Token: agentConfig.Token,
	}

	if config.URL == "" || config.Token == "" {
		return
	}
	// register agent to zadig-server
	resp, err := network.RegisterAgent(config, args)
	if err != nil {
		log.Panicf("failed to register agent: %v", err)
	}
	log.Infof("agent registered successfully!!!")

	// write agent token and status to config file
	afterRegister(resp, agentConfig)
}

func afterRegister(resp *network.RegisterAgentResponse, config *agentconfig.AgentConfig) {
	log.Infof("agent to write agent config file to local disk.")
	// load all Environment variables
	config.Status = common.AGENT_STATUS_REGISTERED

	// 更新配置值
	config.Token = resp.Token
	config.Description = resp.Description
	config.VmName = resp.VmName
	config.Concurrency = resp.Concurrency
	config.ScheduleWorkflow = resp.ScheduleWorkflow
	config.ZadigVersion = resp.ZadigVersion

	if config.Concurrency == 0 {
		config.Concurrency = common.DefaultAgentConcurrency
	}
	if resp.WorkDir != "" {
		config.WorkDirectory = resp.WorkDir
	}

	config.ZadigVersion = resp.ZadigVersion
	config.AgentPlatform = osutil.GetOSName()
	config.AgentArchitecture = osutil.GetOSArch()

	// 序列化配置结构体为YAML格式
	yamlData, err := yaml.Marshal(config)
	if err != nil {
		log.Fatalf("unable to serialize configuration into YAML format: %v", err)
	}

	path, err := agentconfig.GetAgentConfigFilePath()
	if err != nil {
		log.Panicf("failed to get agent config file path: %v", err)
	}

	if exists, err := fileutil.FileExists(path); err != nil {
		log.Panicf("failed to check if agent config file exists: %v", err)
	} else if exists {
		// 如果文件存在，删除它
		err = os.Remove(path)
		if err != nil {
			log.Panicf("failed to remove existing agent config file: %v", err)
		}
	}

	agentFile, err := os.Create(path)
	if err != nil {
		log.Panicf("failed to create agent config file: %v", err)
	}
	defer agentFile.Close()

	// 写入YAML数据到文件
	_, err = agentFile.Write(yamlData)
	if err != nil {
		log.Panicf("failed to write agent config file: %v", err)
	}

	agentconfig.SetAgentConfig(config)
	log.Infof("agent config file written local disk successfully!!!")
}
